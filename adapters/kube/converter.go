package kube

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/naming"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	Svc *model.Workspace
	Prv *model.Provider
	Cls *model.Cluster
	App *model.App

	// Normalized compose project
	Project *types.Project

	// Working directory for file resolution (from RefBase)
	WorkingDir string

	// Stable identifiers
	HashSP string // service/provider
	HashID string // service/provider/app (cluster independent)
	HashIN string // service/provider/cluster/app (cluster dependent)

	// Stable namespace/label/selector namings
	Namespace       string
	ComponentName   string
	ResourceName    string            // <appName>-<componentName> for Deployment/Service/Ingress
	BaseLabels      map[string]string // labels applied to ALL kinds (Namespace/NP/SA/Role/RB/PV/PVC/...)
	ComponentLabels map[string]string // BaseLabels + component labels (Deployment/Service/Ingress/Pod only)
	// HeadlessServiceLabels are ComponentLabels plus the headless marker label.
	HeadlessServiceLabels map[string]string
	// HeadlessServiceSelector is the label selector map used to identify headless services (for pruning etc.).
	// It contains the component app label and the headless marker label. NOTE: This is NOT currently used as the
	// Service spec.selector (pods do not carry the headless marker) – it is for list/delete operations only.
	HeadlessServiceSelector map[string]string
	// HeadlessServiceSelectorString is the string form of HeadlessServiceSelector for convenience (k8s ListOptions).
	HeadlessServiceSelectorString string
	Selector                      map[string]string
	SelectorString                string
	NodeSelector                  map[string]string

	// Provider-agnostic K8s pieces
	K8sNamespace        *corev1.Namespace
	K8sContainers       []corev1.Container
	K8sInitContainers   []corev1.Container
	K8sService          *corev1.Service   // for ingress
	K8sHeadlessServices []*corev1.Service // for intra-pod DNS aliasing
	K8sIngressDefault   *netv1.Ingress
	K8sIngressCustom    *netv1.Ingress
	K8sDeployment       *appsv1.Deployment  // built at Build() time
	K8sSecrets          []*corev1.Secret    // generated from compose env_file (service order)
	K8sConfigMaps       []*corev1.ConfigMap // generated from compose configs
	K8sConfigSecrets    []*corev1.Secret    // generated from compose secrets

	// Config/Secret mount metadata (collected during Convert, consumed by Build for volume definitions)
	configMapMounts    map[string]*configMapMount    // keyed by configName
	configSecretMounts map[string]*configSecretMount // keyed by secretName

	// Optional security and access resources
	K8sNetworkPolicy  *netv1.NetworkPolicy
	K8sServiceAccount *corev1.ServiceAccount
	K8sRole           *rbacv1.Role
	K8sRoleBinding    *rbacv1.RoleBinding

	// Bound storage state
	VolumeBindings []*ConverterVolumeBinding // input bindings (app.Volumes order), updated in-place with chosen resource names
	K8sPVs         []runtime.Object          // generated PVs
	K8sPVCs        []runtime.Object          // generated PVCs

	// Non-fatal notes during planning
	warnings []string
}

// targetMapping represents a target path mapping source.
type targetMapping struct {
	source   string // "config:<name>", "secret:<name>", or "volume:<name>"
	target   string // target path
	location string // for error messages (e.g., "service foo")
}

// configMapMount holds metadata for ConfigMap volume mounting.
type configMapMount struct {
	configName string  // top-level config name
	cmName     string  // K8s ConfigMap resource name
	key        string  // data key (filename)
	mode       *uint32 // optional file mode from service reference (10-base)
}

// configSecretMount holds metadata for Secret volume mounting.
type configSecretMount struct {
	secretName string  // top-level secret name
	secName    string  // K8s Secret resource name
	key        string  // data key (filename)
	mode       *uint32 // optional file mode from service reference (10-base)
}

// ConverterVolumeBinding represents a static binding for one logical volume.
//
// Usage contract:
//   - With BindVolumes: accepts any number of bindings. Name MUST be set and
//     must reference a defined App volume. Completeness (all App.Volumes bound)
//     is enforced at Build time.
//   - With BuildVolumeObjects: provide any subset in any order; Name must be set.
type ConverterVolumeBinding struct {
	// Logical volume name defined in App.Volumes.
	Name string
	// Kubernetes resource name for both PV and PVC. When empty, a stable name
	// is generated from hashes and the provider-specific handle.
	ResourceName string
	// Resolved physical disk information for static provisioning.
	// VolumeDisk.Handle must be a non-empty provider-specific identifier (CSI volumeHandle).
	// VolumeDisk.Size is optional; when zero, the App volume size is used.
	VolumeDisk *model.VolumeDisk
	// VolumeClass controls CSI driver, StorageClass, access modes, volume mode, and attributes.
	// The caller provides this explicitly to keep Converter free of provider lookups.
	VolumeClass *model.VolumeClass
}

// NewConverter creates a converter bound to domain objects and precomputes
// identifiers that do not require Compose parsing (hashes, namespace, labels).
// Compose parsing and plan construction are performed by Convert.
func NewConverter(svc *model.Workspace, prv *model.Provider, cls *model.Cluster, a *model.App, component string) *Converter {
	c := &Converter{
		Svc:                svc,
		Prv:                prv,
		Cls:                cls,
		App:                a,
		configMapMounts:    make(map[string]*configMapMount),
		configSecretMounts: make(map[string]*configSecretMount),
	}
	if svc != nil && prv != nil && cls != nil && a != nil {
		hashes := naming.NewHashes(svc.Name, prv.Name, cls.Name, a.Name)
		c.HashSP = hashes.Provider
		c.HashID = hashes.AppID
		c.HashIN = hashes.AppInstance
		c.Namespace = hashes.Namespace
		c.ComponentName = component
		c.ResourceName = fmt.Sprintf("%s-%s", a.Name, component)
		// BaseLabels are applied to all resources (including Namespace/PV/PVC etc.)
		c.BaseLabels = map[string]string{
			LabelAppK8sName:         a.Name,
			LabelAppK8sInstance:     fmt.Sprintf("%s-%s", a.Name, c.HashIN),
			LabelAppK8sManagedBy:    "kompox",
			LabelK4xAppInstanceHash: c.HashIN,
			LabelK4xAppIDHash:       c.HashID,
		}
		// ComponentLabels are applied only to Deployment/Service/Ingress/Pod (component-scoped)
		c.ComponentLabels = maps.Clone(c.BaseLabels)
		c.ComponentLabels[LabelAppSelector] = fmt.Sprintf("%s-%s", a.Name, component)
		c.ComponentLabels[LabelAppK8sComponent] = component
		c.Selector = map[string]string{LabelAppSelector: c.ComponentLabels[LabelAppSelector]}
		c.SelectorString = labels.Set(c.Selector).String()
		// Precompute headless Service labels (ComponentLabels + marker) and selector used for pruning
		c.HeadlessServiceLabels = maps.Clone(c.ComponentLabels)
		c.HeadlessServiceLabels[LabelK4xComposeServiceHeadless] = "true"
		c.HeadlessServiceSelector = maps.Clone(c.Selector)
		c.HeadlessServiceSelector[LabelK4xComposeServiceHeadless] = "true"
		c.HeadlessServiceSelectorString = labels.Set(c.HeadlessServiceSelector).String()

		// Precompute NodeSelector from app deployment settings
		c.NodeSelector = map[string]string{}
		// Default pool is "user" if not specified
		pool := "user"
		if a.Deployment.Pool != "" {
			pool = a.Deployment.Pool
		}
		c.NodeSelector[LabelK4xNodePool] = pool
		// Zone is optional and only set if specified
		if a.Deployment.Zone != "" {
			c.NodeSelector[LabelK4xNodeZone] = a.Deployment.Zone
		}
	}
	return c
}

// checkTargetConflict validates that target paths do not conflict.
// It detects:
// - Duplicate targets within configs/secrets for the same service (error)
// - Conflicts between volumes and configs/secrets (warning: configs/secrets win, volumes ignored)
//
// Returns: errors slice (fatal conflicts), warnings slice (non-fatal conflicts)
func checkTargetConflict(serviceName string, targetMappings []targetMapping) ([]string, []string) {
	var errs []string
	var warns []string

	// Track targets by type
	targetSources := make(map[string][]targetMapping)
	for _, tm := range targetMappings {
		targetSources[tm.target] = append(targetSources[tm.target], tm)
	}

	// Check each target for conflicts
	for target, sources := range targetSources {
		if len(sources) == 1 {
			continue
		}

		// Separate by type
		var volumeSources []targetMapping
		var configSecretSources []targetMapping
		for _, src := range sources {
			if strings.HasPrefix(src.source, "volume:") {
				volumeSources = append(volumeSources, src)
			} else {
				configSecretSources = append(configSecretSources, src)
			}
		}

		// Multiple configs/secrets targeting same path → error
		if len(configSecretSources) > 1 {
			sourceNames := make([]string, len(configSecretSources))
			for i, cs := range configSecretSources {
				sourceNames[i] = cs.source
			}
			errs = append(errs, fmt.Sprintf(
				"service %s: target path %q has duplicate configs/secrets: %v",
				serviceName, target, sourceNames,
			))
		}

		// configs/secrets vs volumes conflict → warn and ignore volumes
		if len(configSecretSources) > 0 && len(volumeSources) > 0 {
			for _, vs := range volumeSources {
				warns = append(warns, fmt.Sprintf(
					"service %s: target path %q conflicts between %s and %s; ignoring volume (use configs/secrets for single files)",
					serviceName, target, configSecretSources[0].source, vs.source,
				))
			}
		}
	}

	return errs, warns
}

// Convert parses Compose and constructs the provider-agnostic plan into this Converter.
// It validates ports and volumes, and synthesizes Namespace/Service/Ingress and containers.
func (c *Converter) Convert(ctx context.Context) ([]string, error) {
	if c == nil || c.Svc == nil || c.Prv == nil || c.Cls == nil || c.App == nil {
		return nil, fmt.Errorf("converter requires svc/prv/cls/app")
	}

	// Use RefBase-aware compose loading
	proj, workingDir, err := NewComposeProject(ctx, c.App.Compose, c.App.RefBase)
	if err != nil {
		return nil, fmt.Errorf("compose project failed: %w", err)
	}
	c.WorkingDir = workingDir

	// Hashes, namespace name and labels are precomputed by NewConverter.
	// Keep local aliases for readability.
	nsName := c.Namespace
	baseLabels := c.BaseLabels

	// Build volume definition index
	volDefs := map[string]model.AppVolume{}
	for _, v := range c.App.Volumes {
		volDefs[v.Name] = v
	}

	// Pre-build secrets (env_file) inline so envFrom can reference them. Mirrors buildComposeSecret logic.
	secrets := []*corev1.Secret{}
	for _, s := range proj.Services { // deterministic iteration from compose-go
		if len(s.EnvFiles) == 0 {
			continue
		}
		kv, _, err := mergeEnvFiles(c.WorkingDir, s.EnvFiles, c.App.RefBase)
		if err != nil {
			return nil, fmt.Errorf("env_file for service %s: %w", s.Name, err)
		}
		data := map[string][]byte{}
		for k, v := range kv {
			data[k] = []byte(v)
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        SecretEnvBaseName(c.App.Name, c.ComponentName, s.Name),
				Namespace:   nsName,
				Labels:      c.ComponentLabels,
				Annotations: map[string]string{AnnotationK4xComposeContentHash: ComputeContentHash(kv)},
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		secrets = append(secrets, secret)
	}

	// Process top-level configs and secrets
	configMaps := []*corev1.ConfigMap{}
	configSecrets := []*corev1.Secret{}

	// Build configs → ConfigMaps
	for name, cfg := range proj.Configs {
		if err := validateConfigSecretName(name); err != nil {
			return nil, fmt.Errorf("config name validation: %w", err)
		}
		key, content, isValidUTF8Text, err := resolveConfigOrSecretFile(c.WorkingDir, name, types.FileObjectConfig(cfg), true, c.App.RefBase)
		if err != nil {
			return nil, fmt.Errorf("resolve config %q: %w", name, err)
		}
		if content == nil {
			// External or name-only passthrough: skip generation (K8s resource must pre-exist)
			continue
		}
		if !isValidUTF8Text {
			return nil, fmt.Errorf("config %q: content must be valid UTF-8 without BOM and no NUL bytes", name)
		}
		cmName := ConfigMapName(c.App.Name, c.ComponentName, name)
		data := map[string]string{key: string(content)}
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        cmName,
				Namespace:   nsName,
				Labels:      c.ComponentLabels,
				Annotations: map[string]string{AnnotationK4xComposeContentHash: ComputeContentHash(data)},
			},
			Data: data,
		}
		configMaps = append(configMaps, cm)
	}

	// Build secrets → Secrets
	for name, sec := range proj.Secrets {
		if err := validateConfigSecretName(name); err != nil {
			return nil, fmt.Errorf("secret name validation: %w", err)
		}
		key, content, _, err := resolveConfigOrSecretFile(c.WorkingDir, name, types.FileObjectConfig(sec), false, c.App.RefBase)
		if err != nil {
			return nil, fmt.Errorf("resolve secret %q: %w", name, err)
		}
		if content == nil {
			// External or name-only passthrough: skip generation
			continue
		}
		secName := ConfigSecretName(c.App.Name, c.ComponentName, name)
		data := map[string][]byte{key: content}
		contentHash := ComputeContentHash(map[string]string{key: string(content)})
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        secName,
				Namespace:   nsName,
				Labels:      c.ComponentLabels,
				Annotations: map[string]string{AnnotationK4xComposeContentHash: contentHash},
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		configSecrets = append(configSecrets, secret)
	}

	// Compose services parsing & validation
	hostPortToContainer := map[int]int{}   // hostPort -> containerPort
	containerPortOwner := map[int]string{} // containerPort -> service name
	containerPortName := map[int]string{}  // containerPort -> chosen Service port name
	subPathsPerVolume := map[string]map[string]struct{}{}
	var containers []corev1.Container

	for _, s := range proj.Services { // deterministic order from compose-go
		ctn := corev1.Container{Name: s.Name, Image: s.Image}

		// entrypoint → command
		if len(s.Entrypoint) > 0 {
			ctn.Command = []string(s.Entrypoint)
		}

		// command → args
		if len(s.Command) > 0 {
			ctn.Args = []string(s.Command)
		}

		// Collect target mappings for conflict detection
		var targetMappings []targetMapping

		// environment
		for k, v := range s.Environment {
			if v != nil {
				ctn.Env = append(ctn.Env, corev1.EnvVar{Name: k, Value: *v})
			}
		}
		sort.Slice(ctn.Env, func(i, j int) bool { return ctn.Env[i].Name < ctn.Env[j].Name })

		// Always attach envFrom for base and override secrets (optional=true) per spec.
		baseSecretName := fmt.Sprintf("%s-%s-%s-base", c.App.Name, c.ComponentName, s.Name)
		overrideSecretName := fmt.Sprintf("%s-%s-%s-override", c.App.Name, c.ComponentName, s.Name)
		ctn.EnvFrom = append(ctn.EnvFrom,
			corev1.EnvFromSource{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: baseSecretName}, Optional: ptr.To(true)}},
			corev1.EnvFromSource{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: overrideSecretName}, Optional: ptr.To(true)}},
		)

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

		// volumes
		for _, v := range s.Volumes {
			if v.Source == "" || v.Target == "" {
				return nil, errors.New("volume with empty source/target not supported")
			}
			if strings.Contains(v.Source, ":") {
				return nil, fmt.Errorf("unexpected ':' in volume source: %s", v.Source)
			}
			var volName string
			var subPath string
			switch v.Type {
			case "bind":
				// Bind mounts require RefBase with file:// scheme
				if !strings.HasPrefix(c.App.RefBase, "file://") {
					return nil, fmt.Errorf("bind volume not allowed (RefBase: %q): %s", c.App.RefBase, v.Source)
				}
				if strings.HasPrefix(v.Source, "/") {
					return nil, fmt.Errorf("absolute bind volume not supported: %s", v.Source)
				}
				// Check if bind source is a single file (not a directory).
				// If the path exists and is a file, reject it (use configs/secrets instead).
				// If the path doesn't exist, allow it (will be auto-created as directory).
				if info, err := os.Stat(v.Source); err == nil {
					if !info.IsDir() {
						return nil, fmt.Errorf("bind volume source must be a directory, not a single file: %s (use configs/secrets for single-file mounts)", v.Source)
					}
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
			// Track target mapping for conflict detection
			targetMappings = append(targetMappings, targetMapping{
				source:   fmt.Sprintf("volume:%s", v.Source),
				target:   v.Target,
				location: fmt.Sprintf("service %s", s.Name),
			})
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

		// configs: mount as single files via ConfigMap volumes
		for _, cfgRef := range s.Configs {
			if cfgRef.Source == "" {
				return nil, fmt.Errorf("service %s: config reference must have source", s.Name)
			}
			// Apply default target if not specified: /<configName>
			target := cfgRef.Target
			if target == "" {
				target = "/" + cfgRef.Source
			}
			// Resolve config definition
			cfgDef, ok := proj.Configs[cfgRef.Source]
			if !ok {
				return nil, fmt.Errorf("service %s: config %q not defined in top-level configs", s.Name, cfgRef.Source)
			}
			// Skip external/name-only (assumed to exist in cluster)
			if cfgDef.External || (cfgDef.Name != "" && cfgDef.File == "" && cfgDef.Content == "") {
				continue
			}
			// Track target mapping for conflict detection
			targetMappings = append(targetMappings, targetMapping{
				source:   fmt.Sprintf("config:%s", cfgRef.Source),
				target:   target,
				location: fmt.Sprintf("service %s", s.Name),
			})
			// Generate volume name for this ConfigMap
			volName := ConfigMapVolumeName(cfgRef.Source)
			// Determine key (filename) for items
			key := filepath.Base(cfgDef.File)
			if cfgDef.File == "" && cfgDef.Content != "" {
				key = cfgRef.Source
			}
			// Store mount metadata for Build()
			if _, exists := c.configMapMounts[cfgRef.Source]; !exists {
				cmName := ConfigMapName(c.App.Name, c.ComponentName, cfgRef.Source)
				var mode *uint32
				if cfgRef.Mode != nil {
					m := uint32(*cfgRef.Mode)
					mode = &m
				}
				c.configMapMounts[cfgRef.Source] = &configMapMount{
					configName: cfgRef.Source,
					cmName:     cmName,
					key:        key,
					mode:       mode,
				}
			}
			// Add volumeMount (volume will be added in Build())
			ctn.VolumeMounts = append(ctn.VolumeMounts, corev1.VolumeMount{
				Name:      volName,
				MountPath: target,
				SubPath:   key,
				ReadOnly:  true,
			})
		}

		// secrets: mount as single files via Secret volumes
		for _, secRef := range s.Secrets {
			if secRef.Source == "" {
				return nil, fmt.Errorf("service %s: secret reference must have source", s.Name)
			}
			// Apply default target if not specified: /run/secrets/<secretName>
			target := secRef.Target
			if target == "" {
				target = "/run/secrets/" + secRef.Source
			}
			// Resolve secret definition
			secDef, ok := proj.Secrets[secRef.Source]
			if !ok {
				return nil, fmt.Errorf("service %s: secret %q not defined in top-level secrets", s.Name, secRef.Source)
			}
			// Skip external/name-only
			if secDef.External || (secDef.Name != "" && secDef.File == "" && secDef.Content == "") {
				continue
			}
			// Track target mapping for conflict detection
			targetMappings = append(targetMappings, targetMapping{
				source:   fmt.Sprintf("secret:%s", secRef.Source),
				target:   target,
				location: fmt.Sprintf("service %s", s.Name),
			})
			// Generate volume name for this Secret
			volName := ConfigSecretVolumeName(secRef.Source)
			// Determine key
			key := filepath.Base(secDef.File)
			if secDef.File == "" && secDef.Content != "" {
				key = secRef.Source
			}
			// Store mount metadata for Build()
			if _, exists := c.configSecretMounts[secRef.Source]; !exists {
				secName := ConfigSecretName(c.App.Name, c.ComponentName, secRef.Source)
				var mode *uint32
				if secRef.Mode != nil {
					m := uint32(*secRef.Mode)
					mode = &m
				}
				c.configSecretMounts[secRef.Source] = &configSecretMount{
					secretName: secRef.Source,
					secName:    secName,
					key:        key,
					mode:       mode,
				}
			}
			// Add volumeMount (volume will be added in Build())
			ctn.VolumeMounts = append(ctn.VolumeMounts, corev1.VolumeMount{
				Name:      volName,
				MountPath: target,
				SubPath:   key,
				ReadOnly:  true,
			})
		}

		// Check for target conflicts within this service
		conflictErrs, conflictWarns := checkTargetConflict(s.Name, targetMappings)
		if len(conflictErrs) > 0 {
			return nil, fmt.Errorf("target conflicts in service %s: %s", s.Name, strings.Join(conflictErrs, "; "))
		}
		c.warnings = append(c.warnings, conflictWarns...)

		// Filter out volume mounts that conflict with configs/secrets
		// (warnings were already recorded above)
		if len(conflictWarns) > 0 {
			// Build a set of conflicting volume targets
			conflictingVolTargets := make(map[string]bool)
			for _, tm := range targetMappings {
				if !strings.HasPrefix(tm.source, "volume:") {
					continue
				}
				// Check if this volume target conflicts with any config/secret
				for _, other := range targetMappings {
					if strings.HasPrefix(other.source, "config:") || strings.HasPrefix(other.source, "secret:") {
						if tm.target == other.target {
							conflictingVolTargets[tm.target] = true
							break
						}
					}
				}
			}
			// Remove conflicting volume mounts from container
			if len(conflictingVolTargets) > 0 {
				filteredMounts := []corev1.VolumeMount{}
				for _, vm := range ctn.VolumeMounts {
					// Keep mount if it's not a conflicting volume mount
					// (config/secret mounts don't have volume names in our tracking)
					isVolumeMount := false
					for volName := range volDefs {
						if vm.Name == volName || vm.Name == c.App.Volumes[0].Name {
							isVolumeMount = true
							break
						}
					}
					if isVolumeMount && conflictingVolTargets[vm.MountPath] {
						// Skip this mount (already warned)
						continue
					}
					filteredMounts = append(filteredMounts, vm)
				}
				ctn.VolumeMounts = filteredMounts
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
		Labels: baseLabels,
		Annotations: map[string]string{
			AnnotationK4xApp:            fmt.Sprintf("%s/%s/%s/%s", c.Svc.Name, c.Prv.Name, c.Cls.Name, c.App.Name),
			AnnotationK4xProviderDriver: c.Prv.Driver,
		},
	}}

	// Security and access resources per spec
	// - NetworkPolicy: Allow ingress from same-namespace pods, kube-system, and ingress controller namespace; egress unrestricted
	// - ServiceAccount/Role/RoleBinding: Human user access for diagnostics and operations
	// Determine ingress controller namespace via helper
	ingressNs := strings.TrimSpace(IngressNamespace(c.Cls))

	// Build NetworkPolicy peers
	var fromPeers []netv1.NetworkPolicyPeer
	// Same-namespace communication
	fromPeers = append(fromPeers, netv1.NetworkPolicyPeer{PodSelector: &metav1.LabelSelector{}})
	// Namespace selectors for kube-system and optional ingress namespace
	var nsValues []string
	seenNs := map[string]struct{}{}
	for _, n := range []string{"kube-system", ingressNs} {
		if n == "" {
			continue
		}
		if _, ok := seenNs[n]; ok {
			continue
		}
		seenNs[n] = struct{}{}
		nsValues = append(nsValues, n)
	}
	if len(nsValues) > 0 {
		fromPeers = append(fromPeers, netv1.NetworkPolicyPeer{
			NamespaceSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "kubernetes.io/metadata.name", Operator: metav1.LabelSelectorOpIn, Values: nsValues},
				},
			},
		})
	}

	// Build ingress rules list
	ingressRules := []netv1.NetworkPolicyIngressRule{{From: fromPeers}}

	// Add user-defined ingress rules from App.NetworkPolicy
	for _, rule := range c.App.NetworkPolicy.IngressRules {
		npRule := netv1.NetworkPolicyIngressRule{}

		// Convert From peers
		if len(rule.From) > 0 {
			npRule.From = make([]netv1.NetworkPolicyPeer, 0, len(rule.From))
			for _, from := range rule.From {
				peer := netv1.NetworkPolicyPeer{}
				if from.NamespaceSelector != nil {
					peer.NamespaceSelector = from.NamespaceSelector
				}
				npRule.From = append(npRule.From, peer)
			}
		}

		// Convert Ports
		if len(rule.Ports) > 0 {
			npRule.Ports = make([]netv1.NetworkPolicyPort, 0, len(rule.Ports))
			for _, port := range rule.Ports {
				protocol := corev1.Protocol(port.Protocol)
				if protocol == "" {
					protocol = corev1.ProtocolTCP
				}
				npRule.Ports = append(npRule.Ports, netv1.NetworkPolicyPort{
					Protocol: &protocol,
					Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: int32(port.Port)},
				})
			}
		}

		ingressRules = append(ingressRules, npRule)
	}

	var npObj *netv1.NetworkPolicy
	{
		npObj = &netv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: c.App.Name, Namespace: nsName, Labels: baseLabels},
			Spec: netv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{},
				PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeIngress},
				Ingress:     ingressRules,
			},
		}
	}

	// Build ServiceAccount/Role/RoleBinding for human users (diagnostics)
	saName := c.App.Name
	roleName := fmt.Sprintf("%s-access", c.App.Name)
	var saObj *corev1.ServiceAccount
	var roleObj *rbacv1.Role
	var rbObj *rbacv1.RoleBinding

	saObj = &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: saName, Namespace: nsName, Labels: baseLabels}}
	roleRules := []rbacv1.PolicyRule{
		// pods view
		{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list", "watch"}},
		// pods/log get, watch
		{APIGroups: []string{""}, Resources: []string{"pods/log"}, Verbs: []string{"get", "watch"}},
		// exec/portforward/attach create
		{APIGroups: []string{""}, Resources: []string{"pods/exec", "pods/portforward", "pods/attach"}, Verbs: []string{"create"}},
		// events/services/endpoints view
		{APIGroups: []string{""}, Resources: []string{"events", "services", "endpoints"}, Verbs: []string{"get", "list", "watch"}},
		// deployments/replicasets view
		{APIGroups: []string{"apps"}, Resources: []string{"deployments", "replicasets"}, Verbs: []string{"get", "list", "watch"}},
		// ephemeralcontainers update (kubectl debug)
		{APIGroups: []string{""}, Resources: []string{"pods/ephemeralcontainers"}, Verbs: []string{"update"}},
	}
	roleObj = &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: nsName, Labels: baseLabels}, Rules: roleRules}
	rbObj = &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: nsName, Labels: baseLabels},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: saName, Namespace: nsName}},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: roleName},
	}

	// Determine cluster ingress domain for validations.
	clusterDomain := ""
	clusterDomainLower := ""
	if c.Cls != nil && c.Cls.Ingress != nil {
		clusterDomain = strings.TrimSpace(c.Cls.Ingress.Domain)
		if clusterDomain != "" {
			clusterDomainLower = strings.ToLower(clusterDomain)
		}
	}

	// Service from ingress rules or ports
	var warnings []string
	var service *corev1.Service
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
			for _, rawHost := range r.Hosts {
				host := strings.TrimSpace(rawHost)
				if clusterDomainLower != "" {
					hostLower := strings.ToLower(host)
					if hostLower == clusterDomainLower || strings.HasSuffix(hostLower, "."+clusterDomainLower) {
						return nil, fmt.Errorf("ingress host %s must not be under cluster ingress domain %s", host, clusterDomain)
					}
				}
				if prev, dup := hostSeen[host]; dup {
					return nil, fmt.Errorf("host %s duplicated across ingress entries (%s,%s)", host, prev, r.Name)
				}
				hostSeen[host] = r.Name
			}
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: c.ResourceName, Namespace: nsName, Labels: c.ComponentLabels},
			Spec:       corev1.ServiceSpec{Selector: c.Selector, Ports: servicePorts},
		}
	} else if len(hostPortToContainer) > 0 {
		var ports []corev1.ServicePort
		var hps []int
		for hp := range hostPortToContainer {
			hps = append(hps, hp)
		}
		sort.Ints(hps)
		for _, hp := range hps {
			port := corev1.ServicePort{
				Name: fmt.Sprintf("p%d", hp),
				Port: int32(hp), TargetPort: intstr.FromInt(hostPortToContainer[hp]),
			}
			ports = append(ports, port)
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: c.ResourceName, Namespace: nsName, Labels: c.ComponentLabels},
			Spec:       corev1.ServiceSpec{Selector: c.Selector, Ports: ports},
		}
	}

	// Build headless Services for each compose service (DNS A record only, no ports) per spec.
	var headlessServices []*corev1.Service
	for _, s := range proj.Services { // deterministic order
		name := s.Name
		// Validate naming collision with ingress service reserved prefixes
		if strings.HasPrefix(name, fmt.Sprintf("%s-app", c.App.Name)) || strings.HasPrefix(name, fmt.Sprintf("%s-box", c.App.Name)) {
			return nil, fmt.Errorf("compose service name '%s' conflicts with reserved ingress service name prefixes", name)
		}
		hs := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nsName, Labels: c.HeadlessServiceLabels},
			Spec:       corev1.ServiceSpec{ClusterIP: corev1.ClusterIPNone, Selector: c.Selector},
		}
		headlessServices = append(headlessServices, hs)
	}

	// Ingress generation (Traefik)
	var ingDefault, ingCustom *netv1.Ingress
	if len(c.App.Ingress.Rules) > 0 && service != nil {
		// Build Custom-domain Ingress (hosts explicitly provided)
		var customRules []netv1.IngressRule
		customHostSeen := map[string]struct{}{}
		for _, r := range c.App.Ingress.Rules {
			cp := hostPortToContainer[r.Port]
			portName := containerPortName[cp]
			path := netv1.HTTPIngressPath{Path: "/", PathType: ptr.To(netv1.PathTypePrefix), Backend: netv1.IngressBackend{Service: &netv1.IngressServiceBackend{Name: service.Name, Port: netv1.ServiceBackendPort{Name: portName}}}}
			for _, rawHost := range r.Hosts {
				host := strings.TrimSpace(rawHost)
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
			ingCustom = &netv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-custom", c.ResourceName), Namespace: nsName, Labels: c.ComponentLabels, Annotations: ann}, Spec: netv1.IngressSpec{IngressClassName: ptr.To("traefik"), Rules: customRules}}
		}

		// Build Default-domain Ingress (one host per rule based on hostPort)
		defaultDomain := clusterDomain
		if defaultDomain != "" {
			var defaultRules []netv1.IngressRule
			defaultHostSeen := map[string]struct{}{}
			for _, r := range c.App.Ingress.Rules {
				cp := hostPortToContainer[r.Port]
				portName := containerPortName[cp]
				path := netv1.HTTPIngressPath{Path: "/", PathType: ptr.To(netv1.PathTypePrefix), Backend: netv1.IngressBackend{Service: &netv1.IngressServiceBackend{Name: service.Name, Port: netv1.ServiceBackendPort{Name: portName}}}}
				host := fmt.Sprintf("%s-%s-%d.%s", c.App.Name, c.HashID, r.Port, defaultDomain)
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
				ingDefault = &netv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-default", c.ResourceName), Namespace: nsName, Labels: c.ComponentLabels, Annotations: ann}, Spec: netv1.IngressSpec{IngressClassName: ptr.To("traefik"), Rules: defaultRules}}
			}
		}
	}

	c.Project = proj
	// HashID/HashIN/NSName/CommonLabels were set in NewConverter
	c.K8sNamespace = ns
	c.K8sContainers = containers
	c.K8sInitContainers = initContainers
	c.K8sService = service
	c.K8sHeadlessServices = headlessServices
	c.K8sIngressDefault = ingDefault
	c.K8sIngressCustom = ingCustom
	c.K8sSecrets = secrets
	c.K8sConfigMaps = configMaps
	c.K8sConfigSecrets = configSecrets
	// Assign security resources
	c.K8sNetworkPolicy = npObj
	c.K8sServiceAccount = saObj
	c.K8sRole = roleObj
	c.K8sRoleBinding = rbObj
	c.warnings = warnings

	return c.warnings, nil
}

// BindVolumes binds logical volumes to static PV/PVCs.
// Inputs:
//   - drv: provider driver used only to resolve VolumeClass details if needed.
//   - vols: array of ConverterVolumeBinding in the same order as app.Volumes.
//
// Output: claim names by logical volume and generated PV/PVC objects.
func (c *Converter) BindVolumes(ctx context.Context, vols []*ConverterVolumeBinding) error {
	if c.Project == nil || c.Namespace == "" {
		return fmt.Errorf("convert must be called before binding")
	}
	// Accept any number of bindings. For each provided binding, require Name
	// to be set and validate that the referenced volume exists in the app
	// definition. The completeness check (all app volumes are bound) is
	// deferred to Build().
	for i, vb := range vols {
		if vb == nil {
			return fmt.Errorf("binding at index %d is nil", i)
		}
		if strings.TrimSpace(vb.Name) == "" {
			return fmt.Errorf("binding at index %d has empty name", i)
		}
		// Ensure this name is a defined app volume
		found := false
		for _, av := range c.App.Volumes {
			if av.Name == vb.Name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("volume %s is not defined in app", vb.Name)
		}
	}

	// Build PV/PVC objects for the provided bindings and update in-place ResourceName
	var pvObjs []runtime.Object
	var pvcObjs []runtime.Object

	// Index app volume definitions for validation and default sizes.
	volDefs := map[string]model.AppVolume{}
	for _, v := range c.App.Volumes {
		volDefs[v.Name] = v
	}

	hashes := naming.NewHashes(c.Svc.Name, c.Prv.Name, c.Cls.Name, c.App.Name)

	for i, in := range vols {
		if in == nil {
			return fmt.Errorf("nil binding at index %d", i)
		}
		av, ok := volDefs[in.Name]
		if !ok {
			return fmt.Errorf("volume %s is not defined in app", in.Name)
		}

		if in.VolumeDisk == nil {
			return fmt.Errorf("volume %s has no disk in binding input", av.Name)
		}
		handle := strings.TrimSpace(in.VolumeDisk.Handle)
		if handle == "" {
			return fmt.Errorf("volume %s has no handle in binding input", av.Name)
		}
		sizeBytes := in.VolumeDisk.Size
		if sizeBytes <= 0 {
			sizeBytes = av.Size
		}
		resourceName := strings.TrimSpace(in.ResourceName)
		if resourceName == "" {
			resourceName = hashes.VolumeResourceName(av.Name, handle)
		}
		sizeQty := bytesToQuantity(sizeBytes)

		if in.VolumeClass == nil {
			return fmt.Errorf("no VolumeClass for volume %s", av.Name)
		}
		vc := in.VolumeClass
		// AccessModes with defaults.
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
				case "ReadWriteOncePod":
					am = append(am, corev1.ReadWriteOncePod)
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
		csiDriver := strings.TrimSpace(vc.CSIDriver)
		if csiDriver == "" {
			return fmt.Errorf("no CSIDriver for volume %s", av.Name)
		}

		// CSI attributes from VolumeClass
		attrs := map[string]string{}
		for k, v := range vc.Attributes {
			if v != "" {
				attrs[k] = v
			}
		}
		if vc.FSType != "" {
			attrs["fsType"] = vc.FSType
		}

		pvSpec := corev1.PersistentVolumeSpec{
			AccessModes:                   accessModes,
			PersistentVolumeReclaimPolicy: reclaim,
			Capacity:                      corev1.ResourceList{corev1.ResourceStorage: sizeQty},
			VolumeMode:                    ptr.To(volMode),
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           csiDriver,
					VolumeHandle:     handle,
					VolumeAttributes: attrs,
				},
			},
		}
		if vc.StorageClassName != "" {
			pvSpec.StorageClassName = vc.StorageClassName
		}
		pv := &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:   resourceName,
				Labels: c.BaseLabels,
			},
			Spec: pvSpec,
		}

		pvcSpec := corev1.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: sizeQty}},
			VolumeName:  resourceName,
			VolumeMode:  ptr.To(volMode),
		}
		if vc.StorageClassName != "" {
			pvcSpec.StorageClassName = ptr.To(vc.StorageClassName)
		}
		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: c.Namespace,
				Labels:    c.BaseLabels,
			},
			Spec: pvcSpec,
		}

		pvObjs = append(pvObjs, pv)
		pvcObjs = append(pvcObjs, pvc)
		// Update input binding in-place
		in.ResourceName = resourceName
	}

	// Set as-is per contract (the slice elements are already updated in-place)
	c.VolumeBindings = vols
	c.K8sPVs = pvObjs
	c.K8sPVCs = pvcObjs
	return nil
}

// Build composes the final Deployment using this plan and current bindings, stores it, and returns warnings.
// To retrieve full object lists, use NamespaceObjects/VolumeObjects/DeploymentObjects/AllObjects.
func (c *Converter) Build() ([]string, error) {
	if c.Project == nil || c.Namespace == "" {
		return nil, fmt.Errorf("convert must be called before build")
	}
	if len(c.VolumeBindings) != len(c.App.Volumes) {
		return nil, fmt.Errorf("volume bindings count %d does not match app volumes %d", len(c.VolumeBindings), len(c.App.Volumes))
	}
	// Ensure order and names align strictly with App.Volumes
	for i, av := range c.App.Volumes {
		vb := c.VolumeBindings[i]
		if vb == nil {
			return nil, fmt.Errorf("no binding at index %d for volume %s", i, av.Name)
		}
		if strings.TrimSpace(vb.Name) != av.Name {
			return nil, fmt.Errorf("volume binding at index %d is for %s; expected %s", i, vb.Name, av.Name)
		}
	}

	// Pod volumes using binding claim names
	var podVolumes []corev1.Volume
	for i, av := range c.App.Volumes {
		claimName := strings.TrimSpace(c.VolumeBindings[i].ResourceName)
		if claimName == "" {
			return nil, fmt.Errorf("no claim name bound for volume %s", av.Name)
		}
		podVolumes = append(podVolumes, corev1.Volume{
			Name: av.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claimName},
			},
		})
	}

	// Add ConfigMap volumes
	for _, cm := range c.configMapMounts {
		volName := ConfigMapVolumeName(cm.configName)
		items := []corev1.KeyToPath{{Key: cm.key, Path: cm.key}}
		if cm.mode != nil {
			// Convert 10-base uint32 to octal int32 for Kubernetes
			mode := int32(*cm.mode)
			items[0].Mode = &mode
		}
		podVolumes = append(podVolumes, corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cm.cmName},
					Items:                items,
				},
			},
		})
	}

	// Add Secret volumes
	for _, sec := range c.configSecretMounts {
		volName := ConfigSecretVolumeName(sec.secretName)
		items := []corev1.KeyToPath{{Key: sec.key, Path: sec.key}}
		if sec.mode != nil {
			// Convert 10-base uint32 to octal int32 for Kubernetes
			mode := int32(*sec.mode)
			items[0].Mode = &mode
		}
		podVolumes = append(podVolumes, corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: sec.secName,
					Items:      items,
				},
			},
		})
	}

	// Use precomputed NodeSelector from NewConverter
	nodeSelector := c.NodeSelector

	// Deployment (single replica, Recreate)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: c.ResourceName, Namespace: c.Namespace, Labels: c.ComponentLabels},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: c.Selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: c.ComponentLabels},
				Spec:       corev1.PodSpec{Containers: c.K8sContainers, InitContainers: c.K8sInitContainers, Volumes: podVolumes, NodeSelector: nodeSelector},
			},
		},
	}

	// Store deployment
	c.K8sDeployment = dep
	return c.warnings, nil
}

// NamespaceObjects returns namespace-scoped foundational objects:
// Namespace, ServiceAccount, Role, RoleBinding, NetworkPolicy (in this order)
func (c *Converter) NamespaceObjects() []runtime.Object {
	var objs []runtime.Object
	if c.K8sNamespace != nil {
		objs = append(objs, c.K8sNamespace)
	}
	if c.K8sServiceAccount != nil {
		objs = append(objs, c.K8sServiceAccount)
	}
	if c.K8sRole != nil {
		objs = append(objs, c.K8sRole)
	}
	if c.K8sRoleBinding != nil {
		objs = append(objs, c.K8sRoleBinding)
	}
	if c.K8sNetworkPolicy != nil {
		objs = append(objs, c.K8sNetworkPolicy)
	}
	return objs
}

// VolumeObjects returns statically-provisioned PVs and PVCs.
func (c *Converter) VolumeObjects() []runtime.Object {
	var objs []runtime.Object
	objs = append(objs, c.K8sPVs...)
	objs = append(objs, c.K8sPVCs...)
	return objs
}

// DeploymentObjects returns the Deployment, Service, and Ingress resources.
func (c *Converter) DeploymentObjects() []runtime.Object {
	var objs []runtime.Object
	for _, cm := range c.K8sConfigMaps {
		objs = append(objs, cm)
	}
	for _, sec := range c.K8sConfigSecrets {
		objs = append(objs, sec)
	}
	for _, sec := range c.K8sSecrets { // secrets first so Deployment can refer via envFrom in future
		objs = append(objs, sec)
	}
	if c.K8sDeployment != nil {
		objs = append(objs, c.K8sDeployment)
	}
	if c.K8sService != nil {
		objs = append(objs, c.K8sService)
	}
	for _, hs := range c.K8sHeadlessServices {
		objs = append(objs, hs)
	}
	if c.K8sIngressDefault != nil {
		objs = append(objs, c.K8sIngressDefault)
	}
	if c.K8sIngressCustom != nil {
		objs = append(objs, c.K8sIngressCustom)
	}
	return objs
}

// AllObjects returns all objects in the recommended apply order.
func (c *Converter) AllObjects() []runtime.Object {
	var objs []runtime.Object
	objs = append(objs, c.NamespaceObjects()...)
	objs = append(objs, c.VolumeObjects()...)
	objs = append(objs, c.DeploymentObjects()...)
	return objs
}
