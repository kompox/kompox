package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestComputeContentHash_Empty(t *testing.T) {
	hash := ComputeContentHash(map[string]string{})
	if hash == "" {
		t.Fatalf("expected non-empty hash even for empty map")
	}
}

func TestComputeContentHash_SingleKeyValue(t *testing.T) {
	kv := map[string]string{"key1": "value1"}
	hash := ComputeContentHash(kv)
	if hash == "" {
		t.Fatalf("expected non-empty hash")
	}
	if len(hash) != 6 {
		t.Errorf("expected hash length 6, got %d", len(hash))
	}
}

func TestComputeContentHash_MultipleKeyValues(t *testing.T) {
	kv := map[string]string{
		"DATABASE_URL": "postgres://localhost/mydb",
		"API_KEY":      "secret123",
		"PORT":         "8080",
	}
	hash := ComputeContentHash(kv)
	if hash == "" {
		t.Fatalf("expected non-empty hash")
	}
	if len(hash) != 6 {
		t.Errorf("expected hash length 6, got %d", len(hash))
	}
}

func TestComputeContentHash_Deterministic(t *testing.T) {
	kv1 := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	kv2 := map[string]string{
		"key3": "value3",
		"key1": "value1",
		"key2": "value2",
	}
	hash1 := ComputeContentHash(kv1)
	hash2 := ComputeContentHash(kv2)
	if hash1 != hash2 {
		t.Errorf("hash should be deterministic regardless of map iteration order: hash1=%s hash2=%s", hash1, hash2)
	}
}

func TestComputeContentHash_ContentSensitive(t *testing.T) {
	kv1 := map[string]string{"key": "value1"}
	kv2 := map[string]string{"key": "value2"}
	hash1 := ComputeContentHash(kv1)
	hash2 := ComputeContentHash(kv2)
	if hash1 == hash2 {
		t.Errorf("hash should differ when value changes: hash1=%s hash2=%s", hash1, hash2)
	}

	kv3 := map[string]string{"key1": "value"}
	kv4 := map[string]string{"key2": "value"}
	hash3 := ComputeContentHash(kv3)
	hash4 := ComputeContentHash(kv4)
	if hash3 == hash4 {
		t.Errorf("hash should differ when key changes: hash3=%s hash4=%s", hash3, hash4)
	}
}

func TestComputeContentHash_EmptyValues(t *testing.T) {
	kv1 := map[string]string{"key1": ""}
	kv2 := map[string]string{"key1": "value"}
	hash1 := ComputeContentHash(kv1)
	hash2 := ComputeContentHash(kv2)
	if hash1 == hash2 {
		t.Errorf("hash should differ between empty and non-empty value: hash1=%s hash2=%s", hash1, hash2)
	}
}

func TestComputePodContentHash_OrderAndMissing(t *testing.T) {
	pull := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "app-app-web--pull", Annotations: map[string]string{AnnotationK4xComposeContentHash: "aaaaaa"}}}
	base := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "app-app-web-base", Annotations: map[string]string{AnnotationK4xComposeContentHash: "bbbbbb"}}}
	// override missing
	secrets := []*corev1.Secret{pull, base}
	configMaps := []*corev1.ConfigMap{} // no ConfigMaps in this test

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
	hash := ComputePodContentHash(pod, secrets, configMaps)
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

func TestComputePodContentHash_WithVolumes(t *testing.T) {
	// Setup resources with content hashes
	pullSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "app-app-web--pull",
			Annotations: map[string]string{AnnotationK4xComposeContentHash: "aaaaaa"},
		},
	}
	envSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "app-app-web-base",
			Annotations: map[string]string{AnnotationK4xComposeContentHash: "bbbbbb"},
		},
	}
	volSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "app-app-web--sec-api-key",
			Annotations: map[string]string{AnnotationK4xComposeContentHash: "cccccc"},
		},
	}
	envConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "app-app-web-config",
			Annotations: map[string]string{AnnotationK4xComposeContentHash: "dddddd"},
		},
	}
	volConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "app-app-web--cfg-nginx-conf",
			Annotations: map[string]string{AnnotationK4xComposeContentHash: "eeeeee"},
		},
	}

	secrets := []*corev1.Secret{pullSecret, envSecret, volSecret}
	configMaps := []*corev1.ConfigMap{envConfigMap, volConfigMap}

	// Pod spec with all reference types
	pod := &corev1.PodSpec{
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: pullSecret.Name}},
		Containers: []corev1.Container{{
			Name: "web",
			EnvFrom: []corev1.EnvFromSource{
				{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: envSecret.Name}}},
				{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: envConfigMap.Name}}},
			},
		}},
		Volumes: []corev1.Volume{
			{
				Name: "cfg-nginx-conf",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: volConfigMap.Name},
					},
				},
			},
			{
				Name: "sec-api-key",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: volSecret.Name,
					},
				},
			},
		},
	}

	hash1 := ComputePodContentHash(pod, secrets, configMaps)
	if hash1 == "" {
		t.Fatalf("expected non-empty hash")
	}

	// Verify hash changes when volume-mounted ConfigMap changes
	volConfigMap.Annotations[AnnotationK4xComposeContentHash] = "ffffff"
	hash2 := ComputePodContentHash(pod, secrets, configMaps)
	if hash2 == "" {
		t.Fatalf("expected non-empty hash after ConfigMap change")
	}
	if hash1 == hash2 {
		t.Errorf("hash should change when volume-mounted ConfigMap content changes: hash1=%s hash2=%s", hash1, hash2)
	}

	// Verify hash changes when volume-mounted Secret changes
	volSecret.Annotations[AnnotationK4xComposeContentHash] = "gggggg"
	hash3 := ComputePodContentHash(pod, secrets, configMaps)
	if hash3 == "" {
		t.Fatalf("expected non-empty hash after Secret change")
	}
	if hash2 == hash3 {
		t.Errorf("hash should change when volume-mounted Secret content changes: hash2=%s hash3=%s", hash2, hash3)
	}

	// Verify all three hashes are different
	if hash1 == hash2 || hash2 == hash3 || hash1 == hash3 {
		t.Errorf("all hashes should be different: hash1=%s hash2=%s hash3=%s", hash1, hash2, hash3)
	}
}

func TestComputePodContentHash_VolumesOnly(t *testing.T) {
	// Test with only volume-mounted resources (no imagePullSecrets, no envFrom)
	volSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "app-app-web--sec-tls-cert",
			Annotations: map[string]string{AnnotationK4xComposeContentHash: "secret1"},
		},
	}
	volConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "app-app-web--cfg-config",
			Annotations: map[string]string{AnnotationK4xComposeContentHash: "config1"},
		},
	}

	secrets := []*corev1.Secret{volSecret}
	configMaps := []*corev1.ConfigMap{volConfigMap}

	pod := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "web"}},
		Volumes: []corev1.Volume{
			{
				Name: "cfg-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: volConfigMap.Name},
					},
				},
			},
			{
				Name: "sec-tls-cert",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: volSecret.Name,
					},
				},
			},
		},
	}

	hash := ComputePodContentHash(pod, secrets, configMaps)
	if hash == "" {
		t.Fatalf("expected non-empty hash with volume-only references")
	}
}
