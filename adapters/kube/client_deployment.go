package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kompox/kompox/internal/logging"
)

// PatchDeploymentPodContentHash updates the deployment's content hash annotation and imagePullSecrets list
// based on presence of pullSecretName. Only patches when something changes.
func (c *Client) PatchDeploymentPodContentHash(ctx context.Context, namespace, deploymentName string) error {
	if c == nil || c.Clientset == nil || namespace == "" || deploymentName == "" {
		return nil
	}

	logger := logging.FromContext(ctx)
	msgSym := "KubeClient:PatchDeploymentPodContentHash"

	dep, err := c.Clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}
	secList, err := c.Clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list secrets: %w", err)
	}
	cmList, err := c.Clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list configmaps: %w", err)
	}
	var secrets []*corev1.Secret
	for i := range secList.Items {
		secrets = append(secrets, &secList.Items[i])
	}
	var configMaps []*corev1.ConfigMap
	for i := range cmList.Items {
		configMaps = append(configMaps, &cmList.Items[i])
	}

	// Derive expected pull secret name from deploymentName convention (<app>-<component>) + "--pull".
	pullSecretName := deploymentName + "--pull"
	pullExists := false
	for _, s := range secrets {
		if s != nil && s.Name == pullSecretName {
			pullExists = true
			break
		}
	}

	desiredSpec := dep.Spec.Template.Spec.DeepCopy()
	if pullExists && pullSecretName != "" {
		desiredSpec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: pullSecretName}}
	} else {
		desiredSpec.ImagePullSecrets = nil
	}
	newHash := ComputePodContentHash(desiredSpec, secrets, configMaps)
	prev := ""
	if dep.Spec.Template.Annotations != nil {
		prev = dep.Spec.Template.Annotations[AnnotationK4xComposeContentHash]
	}
	// Detect imagePullSecrets change.
	imagePullSecretsChanged := false
	if pullExists {
		if len(dep.Spec.Template.Spec.ImagePullSecrets) != 1 || dep.Spec.Template.Spec.ImagePullSecrets[0].Name != pullSecretName {
			imagePullSecretsChanged = true
		}
	} else if len(dep.Spec.Template.Spec.ImagePullSecrets) != 0 {
		imagePullSecretsChanged = true
	}

	hashChanged := newHash != "" && newHash != prev
	if !hashChanged && !imagePullSecretsChanged {
		return nil
	}

	escape := func(s string) string {
		s = strings.ReplaceAll(s, "~", "~0")
		return strings.ReplaceAll(s, "/", "~1")
	}
	path := "/spec/template/metadata/annotations/" + escape(AnnotationK4xComposeContentHash)

	type op struct {
		Op    string `json:"op"`
		Path  string `json:"path"`
		Value any    `json:"value,omitempty"`
	}
	var patch []op

	if hashChanged {
		if dep.Spec.Template.Annotations == nil {
			patch = append(patch,
				op{Op: "add", Path: "/spec/template/metadata/annotations", Value: map[string]string{}},
				op{Op: "add", Path: path, Value: newHash},
			)
		} else if _, exists := dep.Spec.Template.Annotations[AnnotationK4xComposeContentHash]; exists {
			patch = append(patch, op{Op: "replace", Path: path, Value: newHash})
		} else {
			patch = append(patch, op{Op: "add", Path: path, Value: newHash})
		}
	}

	if imagePullSecretsChanged {
		pullPath := "/spec/template/spec/imagePullSecrets"
		if pullExists {
			if len(dep.Spec.Template.Spec.ImagePullSecrets) == 0 {
				patch = append(patch, op{Op: "add", Path: pullPath, Value: []corev1.LocalObjectReference{{Name: pullSecretName}}})
			} else {
				patch = append(patch, op{Op: "replace", Path: pullPath, Value: []corev1.LocalObjectReference{{Name: pullSecretName}}})
			}
		} else {
			if len(dep.Spec.Template.Spec.ImagePullSecrets) > 0 {
				patch = append(patch, op{Op: "replace", Path: pullPath, Value: []corev1.LocalObjectReference{}})
			}
		}
	}

	patchLogger := logger.With("deployment", deploymentName, "hashChanged", hashChanged, "imagePullSecretsChanged", imagePullSecretsChanged)
	patchLogger.Info(ctx, msgSym+"/s")
	if len(patch) == 0 {
		return nil
	}
	body, _ := json.Marshal(patch)
	_, err = c.Clientset.AppsV1().Deployments(namespace).Patch(ctx, deploymentName, types.JSONPatchType, body, metav1.PatchOptions{})
	if err != nil {
		// fallback merge patch
		mp := map[string]any{"spec": map[string]any{"template": map[string]any{}}}
		tpl := mp["spec"].(map[string]any)["template"].(map[string]any)
		if hashChanged {
			tpl["metadata"] = map[string]any{"annotations": map[string]string{AnnotationK4xComposeContentHash: newHash}}
		}
		if imagePullSecretsChanged {
			specMap := map[string]any{}
			if pullExists {
				specMap["imagePullSecrets"] = []map[string]string{{"name": pullSecretName}}
			} else {
				specMap["imagePullSecrets"] = []any{}
			}
			tpl["spec"] = specMap
		}
		mpBytes, _ := json.Marshal(mp)
		if _, err2 := c.Clientset.AppsV1().Deployments(namespace).Patch(ctx, deploymentName, types.MergePatchType, mpBytes, metav1.PatchOptions{}); err2 != nil {
			patchLogger.Info(ctx, msgSym+"/efail", "err", err2)
			return fmt.Errorf("patch deployment secret hash: %w (json patch failed: %v)", err2, err)
		}
	}
	return nil
}
