package kube

import (
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/kompox/kompox/internal/naming"
)

// ComputeContentHash returns base36 SHA256 prefix length 6 using naming.ShortHash logic.
func ComputeContentHash(kv map[string]string) string {
	if len(kv) == 0 {
		return naming.ShortHash("", 6)
	}
	var keys []string
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(kv[k])
		b.WriteByte(0)
	}
	return naming.ShortHash(b.String(), 6)
}

// ComputePodContentHash returns the aggregate <podContentHASH> per spec.
// Inputs:
//
//	podSpec: Pod specification whose referenced secrets and configmaps are analyzed.
//	secrets: Slice of Secret objects (may be superset). Only name+annotation used.
//	configMaps: Slice of ConfigMap objects (may be superset). Only name+annotation used.
//
// Behavior:
//   - Builds map name-><contentHASH> by reading AnnotationK4xComposeContentHash from both Secrets and ConfigMaps.
//   - Ordering:
//     1. imagePullSecrets order
//     2. envFrom secrets/configmaps (containers sorted by name; envFrom order preserved)
//     3. volume-mounted secrets/configmaps (volumes order)
//   - Missing referenced resources contribute empty string at their position.
//   - Returns empty string if no resource references found.
func ComputePodContentHash(podSpec *corev1.PodSpec, secrets []*corev1.Secret, configMaps []*corev1.ConfigMap) string {
	if podSpec == nil {
		return ""
	}
	m := map[string]string{}
	// Collect content hashes from Secrets
	for _, s := range secrets {
		if s == nil || s.Name == "" || s.Annotations == nil {
			continue
		}
		if h, ok := s.Annotations[AnnotationK4xComposeContentHash]; ok {
			m[s.Name] = h
		}
	}
	// Collect content hashes from ConfigMaps
	for _, cm := range configMaps {
		if cm == nil || cm.Name == "" || cm.Annotations == nil {
			continue
		}
		if h, ok := cm.Annotations[AnnotationK4xComposeContentHash]; ok {
			m[cm.Name] = h
		}
	}
	var segments []string
	// 1. imagePullSecrets (Secrets only)
	for _, ips := range podSpec.ImagePullSecrets {
		if ips.Name == "" {
			continue
		}
		segments = append(segments, m[ips.Name])
	}
	// 2. envFrom references (Secrets and ConfigMaps, containers sorted by name)
	ctns := append([]corev1.Container{}, podSpec.Containers...)
	sort.Slice(ctns, func(i, j int) bool { return ctns[i].Name < ctns[j].Name })
	for _, ctn := range ctns {
		for _, ef := range ctn.EnvFrom {
			if ef.SecretRef != nil && ef.SecretRef.Name != "" {
				segments = append(segments, m[ef.SecretRef.Name])
			}
			if ef.ConfigMapRef != nil && ef.ConfigMapRef.Name != "" {
				segments = append(segments, m[ef.ConfigMapRef.Name])
			}
		}
	}
	// 3. Volume-mounted Secrets and ConfigMaps (volumes order)
	for _, vol := range podSpec.Volumes {
		if vol.Secret != nil && vol.Secret.SecretName != "" {
			segments = append(segments, m[vol.Secret.SecretName])
		}
		if vol.ConfigMap != nil && vol.ConfigMap.Name != "" {
			segments = append(segments, m[vol.ConfigMap.Name])
		}
	}
	if len(segments) == 0 {
		return ""
	}
	base := strings.Join(segments, "")
	return naming.ShortHash(base, 6)
}
