package box

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
)

type DeployInput struct {
	AppID      string   `json:"app_id"`
	Image      string   `json:"image"`
	Command    []string `json:"command"`
	Args       []string `json:"args"`
	Volumes    []string `json:"volumes"`
	AlwaysPull bool     `json:"always_pull"`
	SSHPubkey  []byte   `json:"ssh_pubkey"`
}

type DeployOutput struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (u *UseCase) Deploy(ctx context.Context, in *DeployInput) (*DeployOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("DeployInput.AppID is required")
	}
	if strings.TrimSpace(in.Image) == "" {
		return nil, fmt.Errorf("DeployInput.Image is required")
	}
	logger := logging.FromContext(ctx)

	appObj, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil || appObj == nil {
		return nil, fmt.Errorf("failed to get app %s: %w", in.AppID, err)
	}
	clusterObj, err := u.Repos.Cluster.Get(ctx, appObj.ClusterID)
	if err != nil || clusterObj == nil {
		return nil, fmt.Errorf("failed to get cluster %s: %w", appObj.ClusterID, err)
	}
	providerObj, err := u.Repos.Provider.Get(ctx, clusterObj.ProviderID)
	if err != nil || providerObj == nil {
		return nil, fmt.Errorf("failed to get provider %s: %w", clusterObj.ProviderID, err)
	}
	var serviceObj *model.Service
	if providerObj.ServiceID != "" {
		serviceObj, _ = u.Repos.Service.Get(ctx, providerObj.ServiceID)
	}

	factory, ok := providerdrv.GetDriverFactory(providerObj.Driver)
	if !ok {
		return nil, fmt.Errorf("unknown provider driver: %s", providerObj.Driver)
	}
	drv, err := factory(serviceObj, providerObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver %s: %w", providerObj.Driver, err)
	}
	kubeconfig, err := drv.ClusterKubeconfig(ctx, clusterObj)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster kubeconfig: %w", err)
	}
	kcli, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	c := kube.NewConverter(serviceObj, providerObj, clusterObj, appObj, "box")
	if _, err := c.Convert(ctx); err != nil {
		return nil, fmt.Errorf("convert app for box deploy failed: %w", err)
	}

	type mountSpec struct{ volName, diskName, mountPath string }
	var mounts []mountSpec
	for _, spec := range in.Volumes {
		parts := strings.SplitN(spec, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid volume spec %q (want volName:diskName:/mount/path)", spec)
		}
		vn := strings.TrimSpace(parts[0])
		dn := strings.TrimSpace(parts[1])
		mp := strings.TrimSpace(parts[2])
		if vn == "" || dn == "" || mp == "" || !strings.HasPrefix(mp, "/") {
			return nil, fmt.Errorf("invalid volume spec %q", spec)
		}
		mounts = append(mounts, mountSpec{volName: vn, diskName: dn, mountPath: mp})
	}

	var bindings []*kube.ConverterVolumeBinding
	for _, m := range mounts {
		var appVol *model.AppVolume
		for i := range appObj.Volumes {
			if appObj.Volumes[i].Name == m.volName {
				appVol = &appObj.Volumes[i]
				break
			}
		}
		if appVol == nil {
			return nil, fmt.Errorf("volume %s not defined in app", m.volName)
		}
		disks, err := u.VolumePort.DiskList(ctx, clusterObj, appObj, m.volName)
		if err != nil {
			return nil, fmt.Errorf("disk list failed for volume %s: %w", m.volName, err)
		}
		var d *model.VolumeDisk
		for _, x := range disks {
			if x != nil && x.Name == m.diskName {
				d = x
				break
			}
		}
		if d == nil {
			return nil, fmt.Errorf("disk %s not found under volume %s", m.diskName, m.volName)
		}
		vc, err := drv.VolumeClass(ctx, clusterObj, appObj, *appVol)
		if err != nil {
			return nil, fmt.Errorf("get VolumeClass for volume %s: %w", m.volName, err)
		}
		bindings = append(bindings, &kube.ConverterVolumeBinding{
			Name:        m.volName,
			VolumeDisk:  d,
			VolumeClass: &vc,
		})
	}

	var pvObjs, pvcObjs []runtime.Object
	if len(bindings) > 0 {
		var err error
		pvObjs, pvcObjs, err = c.BuildVolumeObjects(ctx, bindings)
		if err != nil {
			return nil, fmt.Errorf("build PV/PVC objects failed: %w", err)
		}
	}

	var podVolumes []corev1.Volume
	for i, b := range bindings {
		claim := strings.TrimSpace(b.ResourceName)
		if claim == "" {
			return nil, fmt.Errorf("internal: empty claim name for binding %d", i)
		}
		// Use PV/PVC resource name as pod volume name to avoid conflicts when same logical volume is mounted multiple times
		podVolumes = append(podVolumes, corev1.Volume{
			Name:         claim,
			VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim}},
		})
	}
	var vm []corev1.VolumeMount
	for i, m := range mounts {
		if i >= len(bindings) {
			return nil, fmt.Errorf("internal: mount index %d out of bounds", i)
		}
		claim := strings.TrimSpace(bindings[i].ResourceName)
		if claim == "" {
			return nil, fmt.Errorf("internal: empty claim name for mount %d", i)
		}
		vm = append(vm, corev1.VolumeMount{Name: claim, MountPath: m.mountPath})
	}

	// Create SSH Secret if SSHPubkey is provided
	var secretObj *corev1.Secret
	if len(in.SSHPubkey) > 0 {
		secretObj = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      c.ResourceName,
				Namespace: c.Namespace,
				Labels:    c.Labels,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"authorized_keys": in.SSHPubkey,
			},
		}
	}

	pullPolicy := corev1.PullIfNotPresent
	if in.AlwaysPull {
		pullPolicy = corev1.PullAlways
	}

	// Add SSH Secret volume if exists
	if secretObj != nil {
		podVolumes = append(podVolumes, corev1.Volume{
			Name: "authorized-keys",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: c.ResourceName,
				},
			},
		})
		vm = append(vm, corev1.VolumeMount{
			Name:      "authorized-keys",
			MountPath: "/etc/ssh/authorized_keys",
			SubPath:   "authorized_keys",
		})
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: c.ResourceName, Namespace: c.Namespace, Labels: c.Labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: c.Labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: c.Labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            BoxContainerName,
						Image:           in.Image,
						ImagePullPolicy: pullPolicy,
						Command:         in.Command,
						Args:            in.Args,
						VolumeMounts:    vm,
					}},
					Volumes:      podVolumes,
					NodeSelector: c.NodeSelector,
				},
			},
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
		},
	}

	// Add restart annotation if SSH secret is provided to force pod restart
	if secretObj != nil {
		if dep.Spec.Template.ObjectMeta.Annotations == nil {
			dep.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}
		// Calculate SHA256 hash of the SSH public key to trigger restart only when content changes
		hash := sha256.Sum256(in.SSHPubkey)
		dep.Spec.Template.ObjectMeta.Annotations[AnnotationBoxSSHPubkeyHash] = fmt.Sprintf("%x", hash)
	}

	sch := runtime.NewScheme()
	utilruntime.Must(appsv1.AddToScheme(sch))
	utilruntime.Must(corev1.AddToScheme(sch))
	if gvk, _, err := sch.ObjectKinds(c.K8sNamespace); err == nil && len(gvk) > 0 {
		c.K8sNamespace.GetObjectKind().SetGroupVersionKind(gvk[0])
	}
	if err := kcli.ApplyObjects(ctx, []runtime.Object{c.K8sNamespace}, &kube.ApplyOptions{FieldManager: "kompoxops"}); err != nil {
		return nil, fmt.Errorf("apply Namespace failed: %w", err)
	}
	if len(pvObjs) > 0 {
		for i := range pvObjs {
			if gvk, _, err := sch.ObjectKinds(pvObjs[i]); err == nil && len(gvk) > 0 {
				pvObjs[i].GetObjectKind().SetGroupVersionKind(gvk[0])
			}
		}
		if err := kcli.ApplyObjects(ctx, pvObjs, &kube.ApplyOptions{FieldManager: "kompoxops"}); err != nil {
			return nil, fmt.Errorf("apply PVs failed: %w", err)
		}
	}
	if len(pvcObjs) > 0 {
		for i := range pvcObjs {
			if gvk, _, err := sch.ObjectKinds(pvcObjs[i]); err == nil && len(gvk) > 0 {
				pvcObjs[i].GetObjectKind().SetGroupVersionKind(gvk[0])
			}
		}
		if err := kcli.ApplyObjects(ctx, pvcObjs, &kube.ApplyOptions{FieldManager: "kompoxops"}); err != nil {
			return nil, fmt.Errorf("apply PVCs failed: %w", err)
		}
	}
	if secretObj != nil {
		if gvk, _, err := sch.ObjectKinds(secretObj); err == nil && len(gvk) > 0 {
			secretObj.GetObjectKind().SetGroupVersionKind(gvk[0])
		}
		if err := kcli.ApplyObjects(ctx, []runtime.Object{secretObj}, &kube.ApplyOptions{FieldManager: "kompoxops", ForceConflicts: true}); err != nil {
			return nil, fmt.Errorf("apply Secret failed: %w", err)
		}
	}
	if gvk, _, err := sch.ObjectKinds(dep); err == nil && len(gvk) > 0 {
		dep.GetObjectKind().SetGroupVersionKind(gvk[0])
	}
	if err := kcli.ApplyObjects(ctx, []runtime.Object{dep}, &kube.ApplyOptions{FieldManager: "kompoxops", ForceConflicts: true}); err != nil {
		return nil, fmt.Errorf("apply Deployment failed: %w", err)
	}

	logger.Info(ctx, "box runner deployed", "namespace", c.Namespace, "name", c.ResourceName, "image", in.Image, "command", in.Command, "args", in.Args)
	return &DeployOutput{Namespace: c.Namespace, Name: c.ResourceName}, nil
}
