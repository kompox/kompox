package kube

import (
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/kompox/kompox/internal/naming"
)

// computePodSecretHash returns the aggregate <podSecretHASH> per spec.
// Inputs:
//
//	podSpec: Pod specification whose referenced secrets are analyzed.
//	secrets: Slice of Secret objects (may be superset). Only name+annotation used.
//
// Behavior:
//   - Builds map name-><secretHASH> by reading AnnotationK4xComposeSecretHash.
//   - Ordering: imagePullSecrets order, then envFrom secrets (containers sorted by name; envFrom order preserved).
//   - Missing referenced secrets contribute empty string at their position.
//   - Returns empty string if no secret references found.
func computePodSecretHash(podSpec *corev1.PodSpec, secrets []*corev1.Secret) string {
	if podSpec == nil {
		return ""
	}
	m := map[string]string{}
	for _, s := range secrets {
		if s == nil || s.Name == "" || s.Annotations == nil {
			continue
		}
		if h, ok := s.Annotations[AnnotationK4xComposeSecretHash]; ok {
			m[s.Name] = h
		}
	}
	var segments []string
	for _, ips := range podSpec.ImagePullSecrets {
		if ips.Name == "" {
			continue
		}
		segments = append(segments, m[ips.Name])
	}
	ctns := append([]corev1.Container{}, podSpec.Containers...)
	sort.Slice(ctns, func(i, j int) bool { return ctns[i].Name < ctns[j].Name })
	for _, ctn := range ctns {
		for _, ef := range ctn.EnvFrom {
			if ef.SecretRef != nil && ef.SecretRef.Name != "" {
				segments = append(segments, m[ef.SecretRef.Name])
			}
		}
	}
	if len(segments) == 0 {
		return ""
	}
	base := strings.Join(segments, "")
	return naming.ShortHash(base, 6)
}

// SecretEnvBaseName returns `<appName>-<componentName>-<containerName>-base`.
// Used for Compose env_file aggregated environment Secret (optional).
func SecretEnvBaseName(appName, componentName, containerName string) string {
	return appName + "-" + componentName + "-" + containerName + "-base"
}

// SecretEnvOverrideName returns `<appName>-<componentName>-<containerName>-override`.
// Used for CLI override environment Secret (optional; created by `kompoxops app env`).
func SecretEnvOverrideName(appName, componentName, containerName string) string {
	return appName + "-" + componentName + "-" + containerName + "-override"
}

// SecretPullName returns `<appName>-<componentName>--pull`.
// Used for registry auth Secret (kubernetes.io/dockerconfigjson) created by CLI `kompoxops app pull`.
func SecretPullName(appName, componentName string) string {
	return appName + "-" + componentName + "--pull"
}
