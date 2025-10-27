package kube

import (
	"context"
	"errors"
	"fmt"

	"github.com/kompox/kompox/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// DeleteResourceTarget describes a collection of resources to delete.
// Keep this type small and explicit to avoid accidental broad deletions.
type DeleteResourceTarget struct {
	// GVR is the resource to delete, e.g. {Group: "apps", Version: "v1", Resource: "deployments"}.
	GVR schema.GroupVersionResource
	// Namespaced indicates whether the resource is namespaced.
	Namespaced bool
	// Kind is optional and used only for logs or error messages.
	Kind string
}

// DeleteBySelectorOptions controls deletion behavior for DeleteByLabelSelector.
type DeleteBySelectorOptions struct {
	// Propagation selects the deletion propagation policy. Defaults to Background.
	Propagation metav1.DeletionPropagation
	// IgnoreErrors continues deletion across resource kinds when errors occur.
	IgnoreErrors bool
}

func (o *DeleteBySelectorOptions) defaults() {
	if o.Propagation == "" {
		o.Propagation = metav1.DeletePropagationBackground
	}
}

// DeleteByLabelSelector deletes resources matching labelSelector across the provided targets.
// - If target.Namespaced is true, the given namespace is used; otherwise cluster-scoped.
// - Returns the count of successfully deleted resources and joined error if any.
func (c *Client) DeleteByLabelSelector(ctx context.Context, namespace string, targets []DeleteResourceTarget, labelSelector string, opts *DeleteBySelectorOptions) (int, error) {
	if c == nil || c.RESTConfig == nil {
		return 0, fmt.Errorf("kube client is not initialized")
	}
	if opts == nil {
		opts = &DeleteBySelectorOptions{IgnoreErrors: true}
	}
	opts.defaults()

	dy, err := dynamic.NewForConfig(c.RESTConfig)
	if err != nil {
		return 0, fmt.Errorf("create dynamic client: %w", err)
	}

	var deleted int
	var errs []error

	for _, t := range targets {
		var list *unstructured.UnstructuredList
		if t.Namespaced {
			list, err = dy.Resource(t.GVR).Namespace(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		} else {
			list, err = dy.Resource(t.GVR).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		}
		if err != nil {
			kind := t.Kind
			if kind == "" {
				kind = t.GVR.Resource
			}
			errs = append(errs, fmt.Errorf("list %s failed: %w", kind, err))
			if !opts.IgnoreErrors {
				return deleted, errors.Join(errs...)
			}
			continue
		}

		for _, it := range list.Items {
			name := it.GetName()
			delOpts := metav1.DeleteOptions{PropagationPolicy: &opts.Propagation}
			// Log before deletion for each item.
			kind := t.Kind
			if kind == "" {
				kind = t.GVR.Resource
			}
			ns := ""
			if t.Namespaced {
				ns = namespace
			}
			msgSym := "KubeClient:Delete"
			logger := logging.FromContext(ctx).With("ns", ns, "kind", kind, "name", name)
			if t.Namespaced {
				if err := dy.Resource(t.GVR).Namespace(namespace).Delete(ctx, name, delOpts); err != nil {
					logger.Info(ctx, msgSym+"/efail", "err", err)
					errs = append(errs, fmt.Errorf("delete %s %s/%s failed: %w", t.GVR.Resource, namespace, name, err))
					if !opts.IgnoreErrors {
						return deleted, errors.Join(errs...)
					}
					continue
				}
			} else {
				if err := dy.Resource(t.GVR).Delete(ctx, name, delOpts); err != nil {
					logger.Info(ctx, msgSym+"/efail", "err", err)
					errs = append(errs, fmt.Errorf("delete %s %s failed: %w", t.GVR.Resource, name, err))
					if !opts.IgnoreErrors {
						return deleted, errors.Join(errs...)
					}
					continue
				}
			}
			logger.Info(ctx, msgSym+"/eok")
			deleted++
		}
	}

	if len(errs) > 0 {
		return deleted, errors.Join(errs...)
	}
	return deleted, nil
}

// DefaultAppDeleteTargets returns the default set of resource kinds managed by kompox app deploys.
// This keeps use cases and CLI consistent for common cleanup operations.
func DefaultAppDeleteTargets() []DeleteResourceTarget {
	return []DeleteResourceTarget{
		{GVR: schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, Namespaced: true, Kind: "Ingress"},
		{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}, Namespaced: true, Kind: "Service"},
		{GVR: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, Namespaced: true, Kind: "Deployment"},
		{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}, Namespaced: true, Kind: "PVC"},
		{GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumes"}, Namespaced: false, Kind: "PV"},
	}
}
