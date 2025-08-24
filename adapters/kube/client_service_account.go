package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateServiceAccount ensures a namespaced ServiceAccount exists (idempotent) and applies provided annotations.
// If the ServiceAccount already exists, missing or different annotations will be merged/updated.
func (c *Client) CreateServiceAccount(ctx context.Context, namespace, name string, annotations map[string]string) error {
	if c == nil || c.Clientset == nil {
		return fmt.Errorf("kube client is not initialized")
	}
	if namespace == "" {
		return fmt.Errorf("namespace is empty")
	}
	if name == "" {
		return fmt.Errorf("serviceaccount name is empty")
	}

	// Check existence first
	sa, err := c.Clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		// Merge annotations if provided
		if len(annotations) == 0 {
			// Even if no annotations are provided, ensure automount is disabled.
			// Update only when the current value is not false.
			if sa.AutomountServiceAccountToken == nil || *sa.AutomountServiceAccountToken {
				f := false
				sa.AutomountServiceAccountToken = &f
				if _, err := c.Clientset.CoreV1().ServiceAccounts(namespace).Update(ctx, sa, metav1.UpdateOptions{}); err != nil {
					return fmt.Errorf("update serviceaccount %s/%s: %w", namespace, name, err)
				}
			}
			return nil
		}
		if sa.Annotations == nil {
			sa.Annotations = map[string]string{}
		}
		changed := false
		for k, v := range annotations {
			if ev, ok := sa.Annotations[k]; !ok || ev != v {
				sa.Annotations[k] = v
				changed = true
			}
		}
		// Ensure automountServiceAccountToken = false
		if sa.AutomountServiceAccountToken == nil || *sa.AutomountServiceAccountToken {
			f := false
			sa.AutomountServiceAccountToken = &f
			changed = true
		}
		if !changed {
			return nil
		}
		if _, err := c.Clientset.CoreV1().ServiceAccounts(namespace).Update(ctx, sa, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update serviceaccount %s/%s: %w", namespace, name, err)
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get serviceaccount %s/%s: %w", namespace, name, err)
	}

	// Create new ServiceAccount with annotations
	f := false
	sa = &corev1.ServiceAccount{
		ObjectMeta:                   metav1.ObjectMeta{Name: name},
		AutomountServiceAccountToken: &f,
	}
	if len(annotations) > 0 {
		sa.ObjectMeta.Annotations = annotations
	}
	if _, err := c.Clientset.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create serviceaccount %s/%s: %w", namespace, name, err)
	}
	return nil
}
