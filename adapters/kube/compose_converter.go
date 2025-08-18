package kube

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sort"
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

// ComposeAppToObjects converts App (with embedded compose) into Kubernetes objects per draft spec.
// Deterministic object ordering: PVC, Deployment, Service (if ports), Ingress (if rules & service).
func ComposeAppToObjects(ctx context.Context, svc *model.Service, prv *model.Provider, cls *model.Cluster, a *model.App) ([]runtime.Object, []string, error) {
	proj, err := newComposeProject(ctx, a.Compose)
	if err != nil {
		return nil, nil, fmt.Errorf("compose project failed: %w", err)
	}
	base := fmt.Sprintf("%s:%s:%s:%s", svc.Name, prv.Name, cls.Name, a.Name)
	sum := sha1.Sum([]byte(base))
	hash := fmt.Sprintf("%x", sum)[:6]
	nsName := fmt.Sprintf("kompox-%s-%s", a.Name, hash)

	labels := map[string]string{
		"app":                          a.Name,
		"app.kubernetes.io/name":       a.Name,
		"app.kubernetes.io/instance":   fmt.Sprintf("%s-%s", a.Name, hash),
		"app.kubernetes.io/managed-by": "kompox",
	}

	hostPortToContainer := map[int]int{}
	containerPortOwner := map[int]string{}
	subPaths := map[string]struct{}{}
	var containers []corev1.Container

	for _, s := range proj.Services { // compose-go order deterministic
		c := corev1.Container{Name: s.Name, Image: s.Image}
		for k, v := range s.Environment {
			if v != nil {
				c.Env = append(c.Env, corev1.EnvVar{Name: k, Value: *v})
			}
		}
		sort.Slice(c.Env, func(i, j int) bool { return c.Env[i].Name < c.Env[j].Name })

		for _, p := range s.Ports {
			if p.Published == "" || p.Target == 0 { // only published:target pairs
				continue
			}
			var hp int
			if n, err := fmt.Sscanf(p.Published, "%d", &hp); n != 1 || err != nil { // non-numeric published
				continue
			}
			if owner, ok := containerPortOwner[int(p.Target)]; ok && owner != s.Name {
				return nil, nil, fmt.Errorf("containerPort %d used by multiple services (%s,%s)", p.Target, owner, s.Name)
			}
			containerPortOwner[int(p.Target)] = s.Name
			c.Ports = append(c.Ports, corev1.ContainerPort{ContainerPort: int32(p.Target)})
			if prev, ok := hostPortToContainer[hp]; ok && prev != int(p.Target) {
				return nil, nil, fmt.Errorf("hostPort %d mapped to multiple container ports (%d,%d)", hp, prev, p.Target)
			}
			hostPortToContainer[hp] = int(p.Target)
		}

		for _, v := range s.Volumes {
			if v.Source == "" || v.Target == "" {
				return nil, nil, fmt.Errorf("volume with empty source/target not supported")
			}
			src := strings.TrimPrefix(v.Source, "./")
			if strings.HasPrefix(src, "/") { // absolute path not allowed (singleton PVC strategy)
				return nil, nil, fmt.Errorf("absolute volume paths not supported: %s", v.Source)
			}
			sp := normalizeSubPath(src)
			if sp == "" || strings.Contains(sp, "..") {
				return nil, nil, fmt.Errorf("invalid subPath: %s", sp)
			}
			subPaths[sp] = struct{}{}
			c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{Name: "default", MountPath: v.Target, SubPath: sp})
		}

		applyXKompoxResources(&c, s.Extensions["x-kompox"]) // best-effort parsing
		containers = append(containers, c)
	}

	var initContainers []corev1.Container
	if len(subPaths) > 0 {
		var list []string
		for sp := range subPaths {
			list = append(list, sp)
		}
		sort.Strings(list)
		script := []string{"set -eu"}
		for _, sp := range list {
			script = append(script, fmt.Sprintf("mkdir -p /work/%s", sp))
		}
		initContainers = append(initContainers, corev1.Container{
			Name:         "init-volume-subpaths",
			Image:        "busybox:1.36",
			Command:      []string{"sh", "-c", strings.Join(script, "\n")},
			VolumeMounts: []corev1.VolumeMount{{Name: "default", MountPath: "/work"}},
		})
	}

	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: a.Name, Namespace: nsName}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("32Gi")}}, VolumeName: fmt.Sprintf("kompox-%s-%s", a.Name, hash)}}
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: a.Name, Namespace: nsName, Labels: labels}, Spec: appsv1.DeploymentSpec{Replicas: int32Ptr(1), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": a.Name}}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}, Spec: corev1.PodSpec{Containers: containers, InitContainers: initContainers, Volumes: []corev1.Volume{{Name: "default", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: a.Name}}}}}}}}

	var svcObj *corev1.Service
	if len(hostPortToContainer) > 0 {
		var hostPorts []int
		for hp := range hostPortToContainer {
			hostPorts = append(hostPorts, hp)
		}
		sort.Ints(hostPorts)
		var ports []corev1.ServicePort
		for _, hp := range hostPorts {
			ports = append(ports, corev1.ServicePort{Name: fmt.Sprintf("p%d", hp), Port: int32(hp), TargetPort: intstr.FromInt(hostPortToContainer[hp])})
		}
		svcObj = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: a.Name, Namespace: nsName, Labels: labels}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": a.Name}, Ports: ports}}
	}

	var warnings []string
	objs := []runtime.Object{pvc, dep}
	if svcObj != nil {
		objs = append(objs, svcObj)
	}

	if len(a.Ingress) > 0 && svcObj != nil {
		var ingRules []netv1.IngressRule
		for _, rule := range a.Ingress {
			if _, ok := hostPortToContainer[rule.Port]; !ok {
				warnings = append(warnings, fmt.Sprintf("ingress rule %s port %d not exposed; skipping", rule.Name, rule.Port))
				continue
			}
			paths := []netv1.HTTPIngressPath{{
				Path:     "/",
				PathType: pathTypePtr(netv1.PathTypePrefix),
				Backend:  netv1.IngressBackend{Service: &netv1.IngressServiceBackend{Name: svcObj.Name, Port: netv1.ServiceBackendPort{Number: int32(rule.Port)}}},
			}}
			hosts := append([]string{}, rule.Hosts...)
			sort.Strings(hosts)
			for _, h := range hosts {
				ingRules = append(ingRules, netv1.IngressRule{Host: h, IngressRuleValue: netv1.IngressRuleValue{HTTP: &netv1.HTTPIngressRuleValue{Paths: paths}}})
			}
		}
		if len(ingRules) > 0 {
			ing := &netv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: a.Name, Namespace: nsName, Labels: labels}, Spec: netv1.IngressSpec{Rules: ingRules}}
			objs = append(objs, ing)
		}
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
