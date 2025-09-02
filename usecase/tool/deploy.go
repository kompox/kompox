package tool

import (
	"context"
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

// DeployInput contains parameters to deploy a maintenance runner for an App.
type DeployInput struct {
	// AppID is the target application id.
	AppID string `json:"app_id"`
	// Image is the container image to run for the runner.
	Image string `json:"image"`
	// Command is the container entrypoint (overrides image ENTRYPOINT).
	Command []string `json:"command"`
	// Args are arguments to the command (overrides image CMD).
	Args []string `json:"args"`
	// Volumes is a list of mount specifications: "volName:diskName:/mount/path".
	Volumes []string `json:"volumes"`
}

// DeployOutput returns metadata of the deployed runner.
type DeployOutput struct {
	// Namespace where the runner resources are created.
	Namespace string `json:"namespace"`
	// Name is the name of the created Deployment (or Pod) for the runner.
	Name string `json:"name"`
}

// Deploy applies the runner resources idempotently and waits until ready.
func (u *UseCase) Deploy(ctx context.Context, in *DeployInput) (*DeployOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("DeployInput.AppID is required")
	}
	if strings.TrimSpace(in.Image) == "" {
		return nil, fmt.Errorf("DeployInput.Image is required")
	}
	logger := logging.FromContext(ctx)

	// Load domain objects
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

	// Driver and kube client
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

	// Converter for volume PV/PVC generation and stable identifiers
	c := kube.NewConverter(serviceObj, providerObj, clusterObj, appObj)
	if _, err := c.Convert(ctx); err != nil {
		return nil, fmt.Errorf("convert app for tool deploy failed: %w", err)
	}

	// Parse -V specs and prepare bindings
	// Track unique volumes for claim/volume creation and remember mounts for container
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

	// Build bindings per unique volume
	// volumeName -> binding pointer
	bindingsByVol := map[string]*kube.ConverterVolumeBinding{}
	for _, m := range mounts {
		if _, exists := bindingsByVol[m.volName]; exists {
			continue
		}
		// Find App volume definition
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
		// Resolve disk handle via VolumePort
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
		// Resolve VolumeClass from driver
		vc, err := drv.VolumeClass(ctx, clusterObj, appObj, *appVol)
		if err != nil {
			return nil, fmt.Errorf("get VolumeClass for volume %s: %w", m.volName, err)
		}
		bindingsByVol[m.volName] = &kube.ConverterVolumeBinding{
			Name:        m.volName,
			VolumeDisk:  d,
			VolumeClass: &vc,
		}
	}
	var bindings []*kube.ConverterVolumeBinding
	for _, b := range bindingsByVol {
		bindings = append(bindings, b)
	}

	var pvObjs, pvcObjs []runtime.Object
	if len(bindings) > 0 {
		var err error
		pvObjs, pvcObjs, err = c.BuildVolumeObjects(ctx, bindings)
		if err != nil {
			return nil, fmt.Errorf("build PV/PVC objects failed: %w", err)
		}
	}

	// Build runner Deployment
	// Volumes from bindings, mounts from mounts slice
	var podVolumes []corev1.Volume
	for volName, b := range bindingsByVol {
		claim := strings.TrimSpace(b.ResourceName)
		if claim == "" {
			return nil, fmt.Errorf("internal: empty claim name for volume %s", volName)
		}
		podVolumes = append(podVolumes, corev1.Volume{
			Name:         volName,
			VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim}},
		})
	}
	var vm []corev1.VolumeMount
	for _, m := range mounts {
		vm = append(vm, corev1.VolumeMount{Name: m.volName, MountPath: m.mountPath})
	}
	labels := map[string]string{}
	for k, v := range c.CommonLabels {
		labels[k] = v
	}
	labels["app.kubernetes.io/component"] = "tool-runner"
	labels["kompox.dev/tool-runner"] = "true"
	depName := "tool-runner"

	// Determine container command/args
	containerCommand := in.Command
	containerArgs := in.Args
	switch {
	case len(containerCommand) == 0 && len(containerArgs) == 0:
		// Default long-running idle when both are unspecified
		containerCommand = []string{"sh", "-c"}
		containerArgs = []string{"sleep infinity"}
	case len(containerCommand) == 0 && len(containerArgs) > 0:
		// Only args specified: use a shell to execute the given args as a single command
		containerCommand = []string{"sh", "-c"}
		// containerArgs remains as provided
	case len(containerCommand) > 0 && len(containerArgs) == 0:
		// Only command specified: leave args empty (respect explicit command)
		// no-op
	default:
		// both provided: use as-is
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: depName, Namespace: c.NSName, Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name:            "runner",
					Image:           in.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         containerCommand,
					Args:            containerArgs,
					VolumeMounts:    vm,
				}}, Volumes: podVolumes},
			},
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
		},
	}

	// Ensure GVK for objects and apply resources (idempotent)
	// We create a small scheme to get default GVKs for typed objects.
	sch := runtime.NewScheme()
	utilruntime.Must(appsv1.AddToScheme(sch))
	utilruntime.Must(corev1.AddToScheme(sch))
	// Set GVK for namespace
	if gvk, _, err := sch.ObjectKinds(c.Namespace); err == nil && len(gvk) > 0 {
		c.Namespace.GetObjectKind().SetGroupVersionKind(gvk[0])
	}
	// Apply Namespace first (needed before PVCs)
	if err := kcli.ApplyObjects(ctx, []runtime.Object{c.Namespace}, &kube.ApplyOptions{FieldManager: "kompoxops"}); err != nil {
		return nil, fmt.Errorf("apply Namespace failed: %w", err)
	}

	// Set GVK for PV/PVC and apply them
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
	// Set GVK for Deployment
	if gvk, _, err := sch.ObjectKinds(dep); err == nil && len(gvk) > 0 {
		dep.GetObjectKind().SetGroupVersionKind(gvk[0])
	}
	if err := kcli.ApplyObjects(ctx, []runtime.Object{dep}, &kube.ApplyOptions{FieldManager: "kompoxops", ForceConflicts: true}); err != nil {
		return nil, fmt.Errorf("apply Deployment failed: %w", err)
	}

	// Return identifiers
	logger.Info(ctx, "tool runner deployed", "namespace", c.NSName, "name", depName, "image", in.Image, "command", containerCommand, "args", containerArgs)
	return &DeployOutput{Namespace: c.NSName, Name: depName}, nil
}
