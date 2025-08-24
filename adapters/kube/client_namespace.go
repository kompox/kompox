package kube

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateNamespace creates a namespace if it does not exist (idempotent).
func (c *Client) CreateNamespace(ctx context.Context, name string) error {
	if c == nil || c.Clientset == nil {
		return fmt.Errorf("kube client is not initialized")
	}
	if name == "" {
		return fmt.Errorf("namespace name is empty")
	}

	_, err := c.Clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get namespace %s: %w", name, err)
	}

	_, err = c.Clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create namespace %s: %w", name, err)
	}
	return nil
}

// DeleteNamespace deletes a namespace if it exists (idempotent best-effort).
func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	if c == nil || c.Clientset == nil {
		return fmt.Errorf("kube client is not initialized")
	}
	if name == "" {
		return fmt.Errorf("namespace name is empty")
	}

	err := c.Clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete namespace %s: %w", name, err)
	}
	return nil
}
