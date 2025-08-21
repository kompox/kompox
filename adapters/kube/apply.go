package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

// ApplyOptions configures server-side apply operations.
type ApplyOptions struct {
	// DefaultNamespace is used when a namespaced resource omits metadata.namespace.
	DefaultNamespace string
	// FieldManager sets the field manager for SSA; defaults to "kompoxops".
	FieldManager string
	// ForceConflicts forces apply on conflicts when true (careful in multi-manager scenarios).
	ForceConflicts bool
}

func (o *ApplyOptions) defaults() {
	if o.FieldManager == "" {
		o.FieldManager = "kompoxops"
	}
}

// ServerSideApplyObjects performs server-side apply for a slice of typed runtime.Objects.
func (c *Client) ServerSideApplyObjects(ctx context.Context, objs []runtime.Object, opts *ApplyOptions) error {
	if c == nil || c.RESTConfig == nil {
		return fmt.Errorf("kube client is not initialized")
	}
	if opts == nil {
		opts = &ApplyOptions{}
	}
	opts.defaults()

	dc, err := discovery.NewDiscoveryClientForConfig(c.RESTConfig)
	if err != nil {
		return fmt.Errorf("create discovery client: %w", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	dy, err := dynamic.NewForConfig(c.RESTConfig)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	for _, obj := range objs {
		if obj == nil {
			continue
		}
		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return fmt.Errorf("to unstructured: %w", err)
		}
		u := &unstructured.Unstructured{Object: m}
		if err := c.serverSideApplyUnstructured(ctx, u, m, opts, dy, mapper); err != nil {
			return err
		}
	}
	return nil
}

// ServerSideApplyYAML performs server-side apply for a multi-document YAML/JSON byte stream.
func (c *Client) ServerSideApplyYAML(ctx context.Context, data []byte, opts *ApplyOptions) error {
	if c == nil || c.RESTConfig == nil {
		return fmt.Errorf("kube client is not initialized")
	}
	if opts == nil {
		opts = &ApplyOptions{}
	}
	opts.defaults()

	dc, err := discovery.NewDiscoveryClientForConfig(c.RESTConfig)
	if err != nil {
		return fmt.Errorf("create discovery client: %w", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	dy, err := dynamic.NewForConfig(c.RESTConfig)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("decode yaml: %w", err)
		}
		if len(raw) == 0 {
			continue
		}
		u := &unstructured.Unstructured{Object: raw}
		if err := c.serverSideApplyUnstructured(ctx, u, raw, opts, dy, mapper); err != nil {
			return err
		}
	}
	return nil
}

// serverSideApplyUnstructured performs SSA for one unstructured object.
func (c *Client) serverSideApplyUnstructured(ctx context.Context, u *unstructured.Unstructured, raw map[string]any, opts *ApplyOptions, dy dynamic.Interface, mapper meta.RESTMapper) error {
	if u.GetKind() == "" || u.GetAPIVersion() == "" {
		return nil
	}
	gvk := schema.FromAPIVersionAndKind(u.GetAPIVersion(), u.GetKind())
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("rest mapping %s: %w", gvk.String(), err)
	}

	// Fill namespace if needed
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace && u.GetNamespace() == "" {
		ns := opts.DefaultNamespace
		if ns == "" {
			ns = "default"
		}
		u.SetNamespace(ns)
	}
	if u.GetName() == "" {
		return fmt.Errorf("object %s missing metadata.name", gvk.String())
	}

	body, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal %s/%s: %w", u.GetKind(), u.GetName(), err)
	}
	ri := resourceInterfaceFor(dy, mapping.Resource, u.GetNamespace())
	force := opts.ForceConflicts
	if _, err := ri.Patch(ctx, u.GetName(), types.ApplyPatchType, body, metav1.PatchOptions{FieldManager: opts.FieldManager, Force: &force}); err != nil {
		return fmt.Errorf("apply %s %s: %w", u.GetKind(), u.GetName(), err)
	}
	return nil
}

// resourceInterfaceFor returns the dynamic resource interface for gvr/namespace.
func resourceInterfaceFor(dy dynamic.Interface, gvr schema.GroupVersionResource, namespace string) dynamic.ResourceInterface {
	if namespace == "" {
		return dy.Resource(gvr)
	}
	return dy.Resource(gvr).Namespace(namespace)
}
