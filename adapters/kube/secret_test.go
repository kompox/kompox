package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestComputePodSecretHash_OrderAndMissing(t *testing.T) {
	pull := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "app-app-web--pull", Annotations: map[string]string{AnnotationK4xComposeSecretHash: "aaaaaa"}}}
	base := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "app-app-web-base", Annotations: map[string]string{AnnotationK4xComposeSecretHash: "bbbbbb"}}}
	// override missing
	secrets := []*corev1.Secret{pull, base}

	pod := &corev1.PodSpec{
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: pull.Name}},
		Containers: []corev1.Container{{
			Name: "web",
			EnvFrom: []corev1.EnvFromSource{
				{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: base.Name}}},
				{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "app-app-web-override"}}},
			},
		}},
	}
	hash := computePodSecretHash(pod, secrets)
	if hash == "" {
		toJSON := func(s *corev1.Secret) string { return s.Name }
		_ = toJSON // placeholder for potential future debug
		// Provide minimal context
		for _, s := range secrets {
			if s.Annotations == nil {
				t.Logf("secret %s has no annotations", s.Name)
			}
		}
		// Fail
		t.Fatalf("expected non-empty hash")
	}
}
