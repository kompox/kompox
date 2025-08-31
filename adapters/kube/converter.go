package kube

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/yaegashi/kompoxops/domain/model"
	"github.com/yaegashi/kompoxops/internal/naming"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// Converter holds a provider-agnostic plan of Kubernetes objects derived from a Kompox App.
//
// Design:
// - NewConverter constructs the provider-agnostic plan from an App (Parse/Plan phase).
// - BindVolumes resolves storage and stores PV/PVC artifacts and claim names (Bind phase).
// - Build assembles final Kubernetes objects using the plan and stored binding artifacts (Assemble phase).
type Converter struct {
	// Domain references
	Svc *model.Service
	Prv *model.Provider
	Cls *model.Cluster
	App *model.App

	// Normalized compose project
	Project *types.Project

	// Stable identifiers
	HashID string // service/provider/app (cluster independent)
	HashIN string // service/provider/cluster/app (cluster dependent)

	// Naming/labels
	NSName       string
	CommonLabels map[string]string

	// Provider-agnostic K8s pieces
	Namespace      *corev1.Namespace
	Containers     []corev1.Container
	InitContainers []corev1.Container
	Service        *corev1.Service
	IngressDefault *netv1.Ingress
	IngressCustom  *netv1.Ingress

	// Bound storage state
	VolumeBindings []ConverterVolumeBinding // input bindings (app.Volumes order), updated with chosen resource names
	PVObjects      []runtime.Object         // generated PVs
	PVCObjects     []runtime.Object         // generated PVCs

	// Non-fatal notes during planning
	warnings []string
}

// ConverterVolumeBinding is an input item to BindVolumes. The slice must follow app.volumes order.
type ConverterVolumeBinding struct {
	// Logical volume name (must match app.Volumes[i].Name). Optional if caller aligns by order only.
	Name string
	// Physical volume handle (provider-specific resource id for static PV). Required.
	Handle string
	// Size in bytes to request/capacity for PV/PVC. If zero, uses app.Volumes[i].Size.
	Size int64
	// Provider-specific volume class parameters.
	VolumeClass model.VolumeClass
	// ResourceName is the Kubernetes resource name for PV and PVC. If empty, BindVolumes generates a stable default.
	ResourceName string
}

// NewConverter creates a converter bound to domain objects and precomputes
// identifiers that do not require Compose parsing (hashes, namespace, labels).
// Compose parsing and plan construction are performed by Convert.
func NewConverter(svc *model.Service, prv *model.Provider, cls *model.Cluster, a *model.App) *Converter {
	c := &Converter{Svc: svc, Prv: prv, Cls: cls, App: a}
	if svc != nil && prv != nil && cls != nil && a != nil {
		hashes := naming.NewHashes(svc.Name, prv.Name, cls.Name, a.Name)
		c.HashID = hashes.AppID
		c.HashIN = hashes.AppInstance
		c.NSName = fmt.Sprintf("kompox-%s-%s", a.Name, c.HashID)
		c.CommonLabels = map[string]string{
			"app":                          a.Name,
			"app.kubernetes.io/name":       a.Name,
			"app.kubernetes.io/instance":   fmt.Sprintf("%s-%s", a.Name, c.HashIN),
			"app.kubernetes.io/managed-by": "kompox",
			"kompox.dev/app-instance-hash": c.HashIN,
			"kompox.dev/app-id-hash":       c.HashID,
		}
	}
	return c
}

// Convert parses Compose and constructs the provider-agnostic plan into this Converter.
// It validates ports and volumes, and synthesizes Namespace/Service/Ingress and containers.
func (c *Converter) Convert(ctx context.Context) ([]string, error) {
	if c == nil || c.Svc == nil || c.Prv == nil || c.Cls == nil || c.App == nil {
		return nil, fmt.Errorf("converter requires svc/prv/cls/app")
	}

	proj, err := NewComposeProject(ctx, c.App.Compose)
	if err != nil {
		return nil, fmt.Errorf("compose project failed: %w", err)
	}

	// Hashes, namespace name and labels are precomputed by NewConverter.
	// Keep local aliases for readability.
	nsName := c.NSName
	commonLabels := c.CommonLabels

	// Build volume definition index
	volDefs := map[string]model.AppVolume{}
	for _, v := range c.App.Volumes {
		volDefs[v.Name] = v
	}

	// Compose services parsing & validation
	hostPortToContainer := map[int]int{}   // hostPort -> containerPort
	containerPortOwner := map[int]string{} // containerPort -> service name
	containerPortName := map[int]string{}  // containerPort -> chosen Service port name
	subPathsPerVolume := map[string]map[string]struct{}{}
	var containers []corev1.Container

	for _, s := range proj.Services { // deterministic order from compose-go
		ctn := corev1.Container{Name: s.Name, Image: s.Image}

		// environment
		for k, v := range s.Environment {
			if v != nil {
				ctn.Env = append(ctn.Env, corev1.EnvVar{Name: k, Value: *v})
			}
		}
		sort.Slice(ctn.Env, func(i, j int) bool { return ctn.Env[i].Name < ctn.Env[j].Name })

		// ports: only "host:container" numeric accepted
		for _, p := range s.Ports {
			if p.Published == "" || p.Target == 0 {
				return nil, fmt.Errorf("ports must be 'host:container' style numeric; service %s", s.Name)
			}
			hp, err := strconv.Atoi(p.Published)
			if err != nil || hp <= 0 {
				return nil, fmt.Errorf("invalid host port %q", p.Published)
			}
			cp := int(p.Target)
			if owner, ok := containerPortOwner[cp]; ok && owner != s.Name {
				return nil, fmt.Errorf("containerPort %d used by multiple services (%s,%s)", cp, owner, s.Name)
			}
			containerPortOwner[cp] = s.Name
			// keep unique containerPort entry order stable
			found := false
			for _, exist := range ctn.Ports {
				if int(exist.ContainerPort) == cp {
					found = true
					break
				}
			}
			if !found {
				ctn.Ports = append(ctn.Ports, corev1.ContainerPort{ContainerPort: int32(cp)})
			}
			if prev, ok := hostPortToContainer[hp]; ok && prev != cp {
				return nil, fmt.Errorf("hostPort %d mapped to multiple container ports (%d,%d)", hp, prev, cp)
			}
			hostPortToContainer[hp] = cp
		}

		// volumes: parse according to spec
		for _, v := range s.Volumes {
			if v.Source == "" || v.Target == "" {
				return nil, errors.New("volume with empty source/target not supported")
			}
			if strings.Contains(v.Source, ":") { // compose-go already split, but guard
				return nil, fmt.Errorf("unexpected ':' in volume source: %s", v.Source)
			}

			var volName string
			var subPath string // keep as-is; no extra normalization

			switch v.Type {
			case "bind":
				if strings.HasPrefix(v.Source, "/") {
					return nil, fmt.Errorf("absolute bind volume not supported: %s", v.Source)
				}
				if len(c.App.Volumes) == 0 {
					return nil, fmt.Errorf("relative bind volume '%s' requires at least one app volume (default) defined", v.Source)
				}
				volName = c.App.Volumes[0].Name
				subPath = v.Source

			case "volume":
				src := v.Source
				name := src
				rest := ""
				if i := strings.IndexByte(src, '/'); i >= 0 {
					name = src[:i]
					if i+1 < len(src) {
						rest = src[i+1:]
					} else {
						rest = ""
					}
				}
				if _, ok := volDefs[name]; !ok {
					return nil, fmt.Errorf("named volume '%s' referenced by '%s' is not defined in app volumes", name, v.Source)
				}
				volName = name
				subPath = rest

			default:
				return nil, fmt.Errorf("unsupported volume type: %s (source=%s target=%s)", v.Type, v.Source, v.Target)
			}

			if subPath != "" {
				if subPathsPerVolume[volName] == nil {
					subPathsPerVolume[volName] = map[string]struct{}{}
				}
				subPathsPerVolume[volName][subPath] = struct{}{}
				ctn.VolumeMounts = append(ctn.VolumeMounts, corev1.VolumeMount{Name: volName, MountPath: v.Target, SubPath: subPath})
			} else {
				ctn.VolumeMounts = append(ctn.VolumeMounts, corev1.VolumeMount{Name: volName, MountPath: v.Target})
			}
		}

		applyXKompoxResources(&ctn, s.Extensions["x-kompox"]) // resources/limits
		containers = append(containers, ctn)
	}

	// initContainer to create subPath directories across volumes
	var initContainers []corev1.Container
	if len(subPathsPerVolume) > 0 {
		var lines []string
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
		var vm []corev1.VolumeMount
		for _, vn := range volNames {
			vm = append(vm, corev1.VolumeMount{Name: vn, MountPath: fmt.Sprintf("/work/%s", vn)})
		}
		initContainers = append(initContainers, corev1.Container{
			Name:         "init-volume-subpaths",
			Image:        "busybox:1.36",
			Command:      []string{"sh", "-c", strings.Join(lines, "\n")},
			VolumeMounts: vm,
		})
	}

	// Namespace with annotations
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   nsName,
		Labels: commonLabels,
		Annotations: map[string]string{
			"kompox.dev/app":             fmt.Sprintf("%s/%s/%s/%s", c.Svc.Name, c.Prv.Name, c.Cls.Name, c.App.Name),
			"kompox.dev/provider-driver": c.Prv.Driver,
		},
	}}

	// Service from ingress rules or ports
	var warnings []string
	var svcObj *corev1.Service
	if len(c.App.Ingress.Rules) > 0 { // need service if ingress defined
		portSeen := map[int]struct{}{}
		hostSeen := map[string]string{}
		var servicePorts []corev1.ServicePort
		for _, r := range c.App.Ingress.Rules {
			if r.Name == "" || len(r.Name) > 15 || r.Name[0] < 'a' || r.Name[0] > 'z' {
				return nil, fmt.Errorf("invalid ingress name: %s", r.Name)
			}
			for _, ch := range r.Name {
				if !(ch == '-' || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')) {
					return nil, fmt.Errorf("invalid ingress name: %s", r.Name)
				}
			}
			if _, ok := portSeen[r.Port]; ok {
				return nil, fmt.Errorf("duplicate ingress port %d", r.Port)
			}
			portSeen[r.Port] = struct{}{}
			cp, ok := hostPortToContainer[r.Port]
			if !ok {
				return nil, fmt.Errorf("ingress port %d not defined in compose ports", r.Port)
			}
			if exist, ok := containerPortName[cp]; ok && exist != r.Name {
				return nil, fmt.Errorf("containerPort %d referenced by multiple ingress entries with different names (%s,%s)", cp, exist, r.Name)
			}
			containerPortName[cp] = r.Name
			servicePorts = append(servicePorts, corev1.ServicePort{Name: r.Name, Port: int32(r.Port), TargetPort: intstr.FromInt(cp)})
			for _, host := range r.Hosts {
				if prev, dup := hostSeen[host]; dup {
					return nil, fmt.Errorf("host %s duplicated across ingress entries (%s,%s)", host, prev, r.Name)
				}
				hostSeen[host] = r.Name
			}
		}
		svcObj = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: c.App.Name, Namespace: nsName, Labels: commonLabels}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": c.App.Name}, Ports: servicePorts}}
	} else if len(hostPortToContainer) > 0 {
		var ports []corev1.ServicePort
		var hps []int
		for hp := range hostPortToContainer {
			hps = append(hps, hp)
		}
		sort.Ints(hps)
		for _, hp := range hps {
			ports = append(ports, corev1.ServicePort{Name: fmt.Sprintf("p%d", hp), Port: int32(hp), TargetPort: intstr.FromInt(hostPortToContainer[hp])})
		}
		svcObj = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: c.App.Name, Namespace: nsName, Labels: commonLabels}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": c.App.Name}, Ports: ports}}
	}

	// Ingress generation (Traefik)
	var ingDefault, ingCustom *netv1.Ingress
	if len(c.App.Ingress.Rules) > 0 && svcObj != nil {
		// Build Custom-domain Ingress (hosts explicitly provided)
		var customRules []netv1.IngressRule
		customHostSeen := map[string]struct{}{}
		for _, r := range c.App.Ingress.Rules {
			cp := hostPortToContainer[r.Port]
			portName := containerPortName[cp]
			path := netv1.HTTPIngressPath{Path: "/", PathType: ptr.To(netv1.PathTypePrefix), Backend: netv1.IngressBackend{Service: &netv1.IngressServiceBackend{Name: svcObj.Name, Port: netv1.ServiceBackendPort{Name: portName}}}}
			for _, host := range r.Hosts {
				if _, dup := customHostSeen[host]; dup {
					return nil, fmt.Errorf("host %s duplicated across ingress entries", host)
				}
				customHostSeen[host] = struct{}{}
				customRules = append(customRules, netv1.IngressRule{Host: host, IngressRuleValue: netv1.IngressRuleValue{HTTP: &netv1.HTTPIngressRuleValue{Paths: []netv1.HTTPIngressPath{path}}}})
			}
		}
		if len(customRules) > 0 {
			ann := map[string]string{
				"traefik.ingress.kubernetes.io/router.entrypoints": "websecure",
				"traefik.ingress.kubernetes.io/router.tls":         "true",
			}
			certResolver := ""
			if c.App.Ingress.CertResolver != "" {
				certResolver = c.App.Ingress.CertResolver
			} else if c.Cls != nil && c.Cls.Ingress != nil && c.Cls.Ingress.CertResolver != "" {
				certResolver = c.Cls.Ingress.CertResolver
			}
			if certResolver != "" {
				ann["traefik.ingress.kubernetes.io/router.tls.certresolver"] = certResolver
			}
			ingCustom = &netv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-custom", c.App.Name), Namespace: nsName, Labels: commonLabels, Annotations: ann}, Spec: netv1.IngressSpec{IngressClassName: ptr.To("traefik"), Rules: customRules}}
		}

		// Build Default-domain Ingress (one host per rule based on hostPort)
		if c.Cls != nil && strings.TrimSpace(c.Cls.Domain) != "" {
			var defaultRules []netv1.IngressRule
			defaultHostSeen := map[string]struct{}{}
			for _, r := range c.App.Ingress.Rules {
				cp := hostPortToContainer[r.Port]
				portName := containerPortName[cp]
				path := netv1.HTTPIngressPath{Path: "/", PathType: ptr.To(netv1.PathTypePrefix), Backend: netv1.IngressBackend{Service: &netv1.IngressServiceBackend{Name: svcObj.Name, Port: netv1.ServiceBackendPort{Name: portName}}}}
				host := fmt.Sprintf("%s-%s-%d.%s", c.App.Name, c.HashID, r.Port, c.Cls.Domain)
				if _, dup := defaultHostSeen[host]; dup {
					return nil, fmt.Errorf("generated default host %s duplicated", host)
				}
				if ingCustom != nil { // ensure no collision with custom hosts
					if _, exists := customHostSeen[host]; exists {
						return nil, fmt.Errorf("generated default host %s collides with custom hosts", host)
					}
				}
				defaultHostSeen[host] = struct{}{}
				defaultRules = append(defaultRules, netv1.IngressRule{Host: host, IngressRuleValue: netv1.IngressRuleValue{HTTP: &netv1.HTTPIngressRuleValue{Paths: []netv1.HTTPIngressPath{path}}}})
			}
			if len(defaultRules) > 0 {
				ann := map[string]string{
					"traefik.ingress.kubernetes.io/router.entrypoints": "websecure",
					"traefik.ingress.kubernetes.io/router.tls":         "true",
				}
				ingDefault = &netv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-default", c.App.Name), Namespace: nsName, Labels: commonLabels, Annotations: ann}, Spec: netv1.IngressSpec{IngressClassName: ptr.To("traefik"), Rules: defaultRules}}
			}
		}
	}

	c.Project = proj
	// HashID/HashIN/NSName/CommonLabels were set in NewConverter
	c.Namespace = ns
	c.Containers = containers
	c.InitContainers = initContainers
	c.Service = svcObj
	c.IngressDefault = ingDefault
	c.IngressCustom = ingCustom
	c.warnings = warnings

	return c.warnings, nil
}

// BindVolumes binds logical volumes to static PV/PVCs.
// Inputs:
//   - drv: provider driver used only to resolve VolumeClass details if needed.
//   - vols: array of ConverterVolumeBinding in the same order as app.Volumes.
//
// Output: claim names by logical volume and generated PV/PVC objects.
func (c *Converter) BindVolumes(ctx context.Context, vols []ConverterVolumeBinding) error {
	if c.Project == nil || c.NSName == "" {
		return fmt.Errorf("convert must be called before binding")
	}

	var pvObjs []runtime.Object
	var pvcObjs []runtime.Object

	if len(vols) != len(c.App.Volumes) {
		return fmt.Errorf("vols length %d does not match app volumes %d", len(vols), len(c.App.Volumes))
	}

	for i, av := range c.App.Volumes {
		vb := vols[i]
		volHandle := strings.TrimSpace(vb.Handle)
		if volHandle == "" {
			return fmt.Errorf("volume %s has no handle in binding input", av.Name)
		}
		sizeBytes := vb.Size
		if sizeBytes <= 0 {
			sizeBytes = av.Size
		}
		volHASH := naming.VolumeHash(volHandle)
		resourceName := strings.TrimSpace(vb.ResourceName)
		if resourceName == "" {
			resourceName = fmt.Sprintf("kompox-%s-%s-%s", av.Name, c.HashID, volHASH)
		}
		sizeQty := bytesToQuantity(sizeBytes)

		// Use provided VolumeClass only.
		vc := vb.VolumeClass

		// AccessModes mapping with defaults
		accessModes := []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		if len(vc.AccessModes) > 0 {
			var am []corev1.PersistentVolumeAccessMode
			for _, m := range vc.AccessModes {
				switch m {
				case "ReadWriteOnce":
					am = append(am, corev1.ReadWriteOnce)
				case "ReadOnlyMany":
					am = append(am, corev1.ReadOnlyMany)
				case "ReadWriteMany":
					am = append(am, corev1.ReadWriteMany)
				}
			}
			if len(am) > 0 {
				accessModes = am
			}
		}
		reclaim := corev1.PersistentVolumeReclaimRetain
		if vc.ReclaimPolicy == "Delete" {
			reclaim = corev1.PersistentVolumeReclaimDelete
		}
		volMode := corev1.PersistentVolumeFilesystem
		if vc.VolumeMode == "Block" {
			volMode = corev1.PersistentVolumeBlock
		}
		csiDriver := vc.CSIDriver
		if csiDriver == "" {
			return fmt.Errorf("no CSIDriver for volume %s", av.Name)
		}
		// Collect CSI attributes (fsType fallback to ext4)
		attrs := map[string]string{}
		for k, v := range vc.Attributes {
			if v != "" {
				attrs[k] = v
			}
		}
		if vc.FSType != "" {
			attrs["fsType"] = vc.FSType
		}
		if _, ok := attrs["fsType"]; !ok {
			attrs["fsType"] = "ext4"
		}

		pvSpec := corev1.PersistentVolumeSpec{
			AccessModes:                   accessModes,
			PersistentVolumeReclaimPolicy: reclaim,
			Capacity:                      corev1.ResourceList{corev1.ResourceStorage: sizeQty},
			VolumeMode:                    ptr.To(volMode),
			PersistentVolumeSource:        corev1.PersistentVolumeSource{CSI: &corev1.CSIPersistentVolumeSource{Driver: csiDriver, VolumeHandle: volHandle, VolumeAttributes: attrs}},
		}
		if vc.StorageClassName != "" {
			pvSpec.StorageClassName = vc.StorageClassName
		}
		pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: resourceName, Labels: c.CommonLabels}, Spec: pvSpec}

		pvcSpec := corev1.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: sizeQty}},
			VolumeName:  resourceName,
			VolumeMode:  ptr.To(volMode),
		}
		if vc.StorageClassName != "" {
			pvcSpec.StorageClassName = ptr.To(vc.StorageClassName)
		}
		pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: c.NSName, Labels: c.CommonLabels}, Spec: pvcSpec}

		pvObjs = append(pvObjs, pv)
		pvcObjs = append(pvcObjs, pvc)
		// persist chosen resource name back to bindings
		vols[i].ResourceName = resourceName
	}

	c.VolumeBindings = vols
	c.PVObjects = pvObjs
	c.PVCObjects = pvcObjs
	return nil
}

// Build composes the final list of Kubernetes objects using this plan and a given VolumeBinding.
// Output order: Namespace, PV/PVC (if any), Deployment, Service (optional), Ingress (optional).
func (c *Converter) Build() ([]runtime.Object, []string, error) {
	if c.Project == nil || c.NSName == "" {
		return nil, nil, fmt.Errorf("convert must be called before build")
	}
	if len(c.VolumeBindings) != len(c.App.Volumes) {
		return nil, nil, fmt.Errorf("bind must be called before build")
	}
	// Pod volumes using binding claim names
	var podVolumes []corev1.Volume
	for i, av := range c.App.Volumes {
		claimName := strings.TrimSpace(c.VolumeBindings[i].ResourceName)
		if claimName == "" {
			return nil, nil, fmt.Errorf("no claim name bound for volume %s", av.Name)
		}
		podVolumes = append(podVolumes, corev1.Volume{
			Name: av.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claimName},
			},
		})
	}

	// Deployment (single replica, Recreate)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: c.App.Name, Namespace: c.NSName, Labels: c.CommonLabels},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": c.App.Name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: c.CommonLabels},
				Spec:       corev1.PodSpec{Containers: c.Containers, InitContainers: c.InitContainers, Volumes: podVolumes},
			},
		},
	}

	var objs []runtime.Object
	objs = append(objs, c.Namespace)
	objs = append(objs, c.PVObjects...)
	objs = append(objs, c.PVCObjects...)
	objs = append(objs, dep)
	if c.Service != nil {
		objs = append(objs, c.Service)
	}
	if c.IngressDefault != nil {
		objs = append(objs, c.IngressDefault)
	}
	if c.IngressCustom != nil {
		objs = append(objs, c.IngressCustom)
	}
	return objs, c.warnings, nil
}
