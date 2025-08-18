package kube

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/yaegashi/kompoxops/domain/model"
	"github.com/yaegashi/kompoxops/internal/logging"

	yaml "gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ComposeAppToObjects converts App compose spec into Kubernetes objects per conversion draft.
// Output order: Namespace, PV, PVC, Deployment, Service (optional), Ingress (optional).
func ComposeAppToObjects(ctx context.Context, svc *model.Service, prv *model.Provider, cls *model.Cluster, a *model.App) ([]runtime.Object, []string, error) {
	proj, err := newComposeProject(ctx, a.Compose)
	if err != nil {
		return nil, nil, fmt.Errorf("compose project failed: %w", err)
	}

	// Hashes (inHASH cluster dependent, idHASH cluster independent)
	inBase := fmt.Sprintf("%s:%s:%s:%s", svc.Name, prv.Name, cls.Name, a.Name)
	idBase := fmt.Sprintf("%s:%s:%s", svc.Name, prv.Name, a.Name)
	inHASH := shortHash(inBase, 6)
	idHASH := shortHash(idBase, 6)

	nsName := fmt.Sprintf("kompox-%s-%s", a.Name, idHASH)

	commonLabels := map[string]string{
		"app":                          a.Name,
		"app.kubernetes.io/name":       a.Name,
		"app.kubernetes.io/instance":   fmt.Sprintf("%s-%s", a.Name, inHASH),
		"app.kubernetes.io/managed-by": "kompox",
		"kompox.dev/app-instance-hash": inHASH,
		"kompox.dev/app-id-hash":       idHASH,
	}

	// Build volume definitions map from a.Volumes for quick lookup
	volDefs := map[string]model.AppVolume{}
	for _, v := range a.Volumes {
		volDefs[v.Name] = v
	}

	// Compose services parsing & validation
	hostPortToContainer := map[int]int{}   // hostPort -> containerPort
	containerPortOwner := map[int]string{} // containerPort -> service name
	containerPortName := map[int]string{}  // containerPort -> chosen Service port name
	// subPathsPerVolume collects subpaths to create per volume name
	subPathsPerVolume := map[string]map[string]struct{}{}
	var containers []corev1.Container

	for _, s := range proj.Services { // deterministic order from compose-go
		c := corev1.Container{Name: s.Name, Image: s.Image}
		// environment
		for k, v := range s.Environment {
			if v != nil {
				c.Env = append(c.Env, corev1.EnvVar{Name: k, Value: *v})
			}
		}
		sort.Slice(c.Env, func(i, j int) bool { return c.Env[i].Name < c.Env[j].Name })

		// ports: only "host:container" numeric accepted
		for _, p := range s.Ports {
			if p.Published == "" || p.Target == 0 {
				return nil, nil, fmt.Errorf("ports must be 'host:container' style numeric; service %s", s.Name)
			}
			hp, err := strconv.Atoi(p.Published)
			if err != nil || hp <= 0 {
				return nil, nil, fmt.Errorf("invalid host port %q", p.Published)
			}
			cp := int(p.Target)
			if owner, ok := containerPortOwner[cp]; ok && owner != s.Name {
				return nil, nil, fmt.Errorf("containerPort %d used by multiple services (%s,%s)", cp, owner, s.Name)
			}
			containerPortOwner[cp] = s.Name
			// keep unique containerPort entry order stable
			found := false
			for _, exist := range c.Ports {
				if int(exist.ContainerPort) == cp {
					found = true
					break
				}
			}
			if !found {
				c.Ports = append(c.Ports, corev1.ContainerPort{ContainerPort: int32(cp)})
			}
			if prev, ok := hostPortToContainer[hp]; ok && prev != cp {
				return nil, nil, fmt.Errorf("hostPort %d mapped to multiple container ports (%d,%d)", hp, prev, cp)
			}
			hostPortToContainer[hp] = cp
		}

		// volumes: parse according to spec
		//  - ./sub/path: default volume (first entry in a.Volumes slice) required
		//  - name/sub/path: named volume must match volume definition
		//  Absolute paths are error
		for _, v := range s.Volumes {
			if v.Source == "" || v.Target == "" {
				return nil, nil, errors.New("volume with empty source/target not supported")
			}
			if strings.HasPrefix(v.Source, "/") { // absolute path not allowed
				return nil, nil, fmt.Errorf("absolute volume path not supported: %s", v.Source)
			}
			src := v.Source
			src = strings.TrimPrefix(src, "./") // may or may not have ./
			volName := ""
			subPathRaw := ""
			if strings.Contains(src, ":") { // colon shouldn't appear here (compose-go already split), but guard
				return nil, nil, fmt.Errorf("unexpected ':' in volume source: %s", v.Source)
			}
			// Determine form
			if strings.Contains(src, "/") {
				// Could be name/sub/path or sub/path for default. To distinguish, check full token before first slash exists in volDefs
				first, rest, _ := strings.Cut(src, "/")
				if _, ok := volDefs[first]; ok { // named volume
					volName = first
					subPathRaw = rest
				} else {
					// treat as default volume reference
					if len(a.Volumes) == 0 {
						return nil, nil, fmt.Errorf("relative bind volume '%s' requires at least one app volume (default) defined", v.Source)
					}
					volName = a.Volumes[0].Name
					subPathRaw = src
				}
			} else {
				// single segment is invalid because subPath after normalization must not be empty
				return nil, nil, fmt.Errorf("volume source must include sub path: %s", v.Source)
			}
			if volName == "" {
				return nil, nil, fmt.Errorf("failed to resolve volume for source %s", v.Source)
			}
			sp := normalizeSubPath(subPathRaw)
			if sp == "" || strings.Contains(sp, "..") {
				return nil, nil, fmt.Errorf("invalid subPath: %s", subPathRaw)
			}
			// record subPath per volume
			if subPathsPerVolume[volName] == nil {
				subPathsPerVolume[volName] = map[string]struct{}{}
			}
			subPathsPerVolume[volName][sp] = struct{}{}
			c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{Name: volName, MountPath: v.Target, SubPath: sp})
		}

		applyXKompoxResources(&c, s.Extensions["x-kompox"]) // resources/limits
		containers = append(containers, c)
	}

	// initContainer to create subPath directories across volumes
	var initContainers []corev1.Container
	if len(subPathsPerVolume) > 0 {
		var lines []string
		// stable order: volume names sorted, then subpaths sorted
		var volNames []string
		for vn := range subPathsPerVolume {
			volNames = append(volNames, vn)
		}
		sort.Strings(volNames)
		for _, vn := range volNames {
			var sps []string
			for sp := range subPathsPerVolume[vn] {
				sps = append(sps, sp)
			}
			sort.Strings(sps)
			for _, sp := range sps {
				lines = append(lines, fmt.Sprintf("mkdir -m 1777 -p /work/%s/%s", vn, sp))
			}
		}
		// mount each volume at /work/<volName>
		var vm []corev1.VolumeMount
		for _, vn := range volNames {
			vm = append(vm, corev1.VolumeMount{Name: vn, MountPath: fmt.Sprintf("/work/%s", vn)})
		}
		initContainers = append(initContainers, corev1.Container{
			Name: "init-volume-subpaths", Image: "busybox:1.36",
			Command:      []string{"sh", "-c", strings.Join(lines, "\n")},
			VolumeMounts: vm,
		})
	}

	// Namespace
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName, Labels: commonLabels, Annotations: map[string]string{
		"kompox.dev/app":             fmt.Sprintf("%s/%s/%s/%s", svc.Name, prv.Name, cls.Name, a.Name),
		"kompox.dev/provider-driver": prv.Driver,
		// volume-handle-current/previous: provider specific; left empty here until driver injection stage.
	}}}

	// Build PVs / PVCs for each declared volume
	var pvs []runtime.Object
	var podVolumes []corev1.Volume
	for _, av := range a.Volumes {
		volHandle := fmt.Sprintf("placeholder-%s-handle", av.Name) // TODO provider inject
		volHASH := shortHash(volHandle, 6)
		pvName := fmt.Sprintf("kompox-%s-%s-%s", av.Name, idHASH, volHASH)
		size := av.Size
		if size == "" {
			size = "32Gi"
		}
		pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: pvName, Labels: commonLabels}, Spec: corev1.PersistentVolumeSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain, Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}, StorageClassName: "managed-csi", VolumeMode: volumeModePtr(corev1.PersistentVolumeFilesystem), PersistentVolumeSource: corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: "disk.csi.azure.com", VolumeHandle: volHandle, VolumeAttributes: map[string]string{"fsType": "ext4"}}}}}
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: pvName, Namespace: nsName, Labels: commonLabels}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}}, VolumeName: pvName}}
		pvs = append(pvs, pv, pvc)
		podVolumes = append(podVolumes, corev1.Volume{Name: av.Name, VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvName}}})
	}
	// Deployment with all volumes
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a.Name, Namespace: nsName, Labels: commonLabels}, Spec: appsv1.DeploymentSpec{Replicas: int32Ptr(1), Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": a.Name}}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: commonLabels}, Spec: corev1.PodSpec{Containers: containers, InitContainers: initContainers, Volumes: podVolumes}}}}

	// Build Service from app.Ingress definitions (ordered) – spec mandates Service ports align with ingress entries.
	var warnings []string
	var svcObj *corev1.Service
	if len(a.Ingress) > 0 { // need service if ingress defined
		portSeen := map[int]struct{}{}
		hostSeen := map[string]string{} // host -> ruleName
		var servicePorts []corev1.ServicePort
		for i, r := range a.Ingress {
			// validate name regex (simplified): start with [a-z], max 15, only [a-z0-9-]
			if r.Name == "" || len(r.Name) > 15 || r.Name[0] < 'a' || r.Name[0] > 'z' {
				return nil, nil, fmt.Errorf("invalid ingress name: %s", r.Name)
			}
			for _, ch := range r.Name {
				if !(ch == '-' || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')) {
					return nil, nil, fmt.Errorf("invalid ingress name: %s", r.Name)
				}
			}
			if _, ok := portSeen[r.Port]; ok {
				return nil, nil, fmt.Errorf("duplicate ingress port %d", r.Port)
			}
			portSeen[r.Port] = struct{}{}
			cp, ok := hostPortToContainer[r.Port]
			if !ok {
				return nil, nil, fmt.Errorf("ingress port %d not defined in compose ports", r.Port)
			}
			// ensure unique containerPort -> service port name mapping
			if exist, ok := containerPortName[cp]; ok && exist != r.Name {
				return nil, nil, fmt.Errorf("containerPort %d referenced by multiple ingress entries with different names (%s,%s)", cp, exist, r.Name)
			}
			containerPortName[cp] = r.Name
			servicePorts = append(servicePorts, corev1.ServicePort{Name: r.Name, Port: int32(r.Port), TargetPort: intstr.FromInt(cp)})
			// host validations & ordering
			for _, host := range r.Hosts {
				if prev, dup := hostSeen[host]; dup {
					return nil, nil, fmt.Errorf("host %s duplicated across ingress entries (%s,%s)", host, prev, r.Name)
				}
				hostSeen[host] = r.Name
			}
			_ = i // index kept for stable order (already sequential)
		}
		svcObj = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: a.Name, Namespace: nsName, Labels: commonLabels}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": a.Name}, Ports: servicePorts}}
	} else if len(hostPortToContainer) > 0 {
		// If ingress not specified but ports exist, still create Service using ascending hostPort names p<port>
		var ports []corev1.ServicePort
		var hps []int
		for hp := range hostPortToContainer {
			hps = append(hps, hp)
		}
		sort.Ints(hps)
		for _, hp := range hps {
			ports = append(ports, corev1.ServicePort{Name: fmt.Sprintf("p%d", hp), Port: int32(hp), TargetPort: intstr.FromInt(hostPortToContainer[hp])})
		}
		svcObj = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: a.Name, Namespace: nsName, Labels: commonLabels}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": a.Name}, Ports: ports}}
	}

	// Ingress generation
	var ingObj *netv1.Ingress
	if len(a.Ingress) > 0 && svcObj != nil {
		var rules []netv1.IngressRule
		for _, r := range a.Ingress { // defined order
			cp := hostPortToContainer[r.Port]
			portName := containerPortName[cp]
			path := netv1.HTTPIngressPath{Path: "/", PathType: pathTypePtr(netv1.PathTypePrefix), Backend: netv1.IngressBackend{Service: &netv1.IngressServiceBackend{Name: svcObj.Name, Port: netv1.ServiceBackendPort{Name: portName}}}}
			for _, host := range r.Hosts { // order given
				rules = append(rules, netv1.IngressRule{Host: host, IngressRuleValue: netv1.IngressRuleValue{HTTP: &netv1.HTTPIngressRuleValue{Paths: []netv1.HTTPIngressPath{path}}}})
			}
		}
		ingObj = &netv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: a.Name, Namespace: nsName, Labels: commonLabels, Annotations: map[string]string{"traefik.ingress.kubernetes.io/router.entrypoints": "websecure"}}, Spec: netv1.IngressSpec{IngressClassName: strPtr("traefik"), Rules: rules}}
	}

	objs := []runtime.Object{ns}
	objs = append(objs, pvs...)
	objs = append(objs, dep)
	if svcObj != nil {
		objs = append(objs, svcObj)
	}
	if ingObj != nil {
		objs = append(objs, ingObj)
	}
	return objs, warnings, nil
}

// Helpers (single copy)
func applyXKompoxResources(c *corev1.Container, ext any) {
	if ext == nil {
		return
	}
	b, err := yaml.Marshal(ext)
	if err != nil {
		return
	}
	var x struct {
		Resources struct {
			CPU    string `yaml:"cpu"`
			Memory string `yaml:"memory"`
		} `yaml:"resources"`
		Limits struct {
			CPU    string `yaml:"cpu"`
			Memory string `yaml:"memory"`
		} `yaml:"limits"`
	}
	if err := yaml.Unmarshal(b, &x); err != nil {
		return
	}
	rr := corev1.ResourceRequirements{}
	if x.Resources.CPU != "" || x.Resources.Memory != "" {
		rr.Requests = corev1.ResourceList{}
	}
	if x.Resources.CPU != "" {
		if q, err := resource.ParseQuantity(x.Resources.CPU); err == nil {
			rr.Requests[corev1.ResourceCPU] = q
		}
	}
	if x.Resources.Memory != "" {
		if q, err := resource.ParseQuantity(x.Resources.Memory); err == nil {
			rr.Requests[corev1.ResourceMemory] = q
		}
	}
	if x.Limits.CPU != "" || x.Limits.Memory != "" {
		if rr.Limits == nil {
			rr.Limits = corev1.ResourceList{}
		}
	}
	if x.Limits.CPU != "" {
		if q, err := resource.ParseQuantity(x.Limits.CPU); err == nil {
			rr.Limits[corev1.ResourceCPU] = q
		}
	}
	if x.Limits.Memory != "" {
		if q, err := resource.ParseQuantity(x.Limits.Memory); err == nil {
			rr.Limits[corev1.ResourceMemory] = q
		}
	}
	if len(rr.Requests) > 0 || len(rr.Limits) > 0 {
		c.Resources = rr
	}
}

func normalizeSubPath(s string) string {
	parts := strings.Split(s, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "/")
}
func int32Ptr(i int32) *int32                      { return &i }
func pathTypePtr(p netv1.PathType) *netv1.PathType { return &p }
func reclaimPolicyPtr(p corev1.PersistentVolumeReclaimPolicy) *corev1.PersistentVolumeReclaimPolicy {
	return &p
}
func volumeModePtr(m corev1.PersistentVolumeMode) *corev1.PersistentVolumeMode { return &m }
func strPtr(s string) *string                                                  { return &s }

// shortHash returns a hex SHA1 prefix with automatic extension length logic placeholder.
func shortHash(s string, n int) string {
	sum := sha1.Sum([]byte(s))
	h := fmt.Sprintf("%x", sum)
	if n > len(h) {
		n = len(h)
	}
	return h[:n]
}

var _ = types.Project{}

// newComposeProject loads a compose project from raw YAML string (single file only, includes disabled).
func newComposeProject(ctx context.Context, composeContent string) (*types.Project, error) {
	logger := logging.FromContext(ctx)

	cdm := types.ConfigDetails{
		WorkingDir:  ".",
		ConfigFiles: []types.ConfigFile{{Filename: "app.compose", Content: []byte(composeContent)}},
		Environment: map[string]string{},
	}
	model, err := loader.LoadModelWithContext(ctx, cdm, func(o *loader.Options) {
		o.SetProjectName("kompox-compose", false)
		o.SkipInclude = true
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load compose model: %w", err)
	}
	if _, ok := model["version"]; ok {
		logger.Warn(ctx, "compose: `version` is obsolete")
	}
	var proj *types.Project
	if err := loader.Transform(model, &proj); err != nil {
		return nil, fmt.Errorf("failed to transform compose model to project: %w", err)
	}
	return proj, nil
}

// ComposeAppToProject returns the normalized compose project (exported for usecases needing validation output).
func ComposeAppToProject(ctx context.Context, composeContent string) (*types.Project, error) {
	return newComposeProject(ctx, composeContent)
}
