package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"

	"github.com/yaegashi/kompoxops/domain/model"
)

// Installer provides common in-cluster install/uninstall operations.
// It is intended to be called from provider drivers' ClusterInstall/ClusterUninstall.
type Installer struct {
	Client *Client
}

// NewInstaller constructs an Installer from a kube Client.
func NewInstaller(c *Client) *Installer {
	return &Installer{Client: c}
}

// IngressNamespace resolves the namespace to use for ingress from the cluster spec.
// Falls back to "default" when not specified.
func IngressNamespace(cluster *model.Cluster) string {
	ns := "default"
	if cluster != nil && cluster.Ingress != nil {
		if v, ok := cluster.Ingress["namespace"].(string); ok && v != "" {
			ns = v
		}
	}
	return ns
}

// EnsureIngressNamespace ensures the ingress namespace exists.
func (i *Installer) EnsureIngressNamespace(ctx context.Context, cluster *model.Cluster) error {
	ns := IngressNamespace(cluster)
	return i.EnsureNamespace(ctx, ns)
}

// DeleteIngressNamespace deletes the ingress namespace if it exists.
func (i *Installer) DeleteIngressNamespace(ctx context.Context, cluster *model.Cluster) error {
	ns := IngressNamespace(cluster)
	return i.DeleteNamespace(ctx, ns)
}

// EnsureNamespace creates a namespace if it does not exist (idempotent).
func (i *Installer) EnsureNamespace(ctx context.Context, name string) error {
	if i == nil || i.Client == nil || i.Client.Clientset == nil {
		return fmt.Errorf("kube installer is not initialized")
	}
	if name == "" {
		return fmt.Errorf("namespace name is empty")
	}

	_, err := i.Client.Clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get namespace %s: %w", name, err)
	}

	_, err = i.Client.Clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
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
func (i *Installer) DeleteNamespace(ctx context.Context, name string) error {
	if i == nil || i.Client == nil || i.Client.Clientset == nil {
		return fmt.Errorf("kube installer is not initialized")
	}
	if name == "" {
		return fmt.Errorf("namespace name is empty")
	}

	err := i.Client.Clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete namespace %s: %w", name, err)
	}
	return nil
}

// ApplyYAML applies a YAML stream (possibly multi-document) using server-side apply.
// defaultNamespace is used when a namespaced resource has no namespace.
func (i *Installer) ApplyYAML(ctx context.Context, yamlStream []byte, defaultNamespace string) error {
	if i == nil || i.Client == nil || i.Client.RESTConfig == nil {
		return fmt.Errorf("kube installer is not initialized")
	}
	// Create clients once per stream
	cfg := i.Client.RESTConfig
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create discovery client: %w", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	dy, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlStream), 4096)
	for {
		var raw map[string]interface{}
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("decode yaml: %w", err)
		}
		if len(raw) == 0 {
			continue
		}
		obj := &unstructured.Unstructured{Object: raw}
		// Marshal back to JSON for Patch body
		body, err := json.Marshal(raw)
		if err != nil {
			return fmt.Errorf("marshal object to json: %w", err)
		}
		if err := i.applyUnstructured(ctx, obj, body, defaultNamespace, dy, mapper); err != nil {
			return err
		}
	}
	return nil
}

// ApplyManifests applies a list of YAML documents.
func (i *Installer) ApplyManifests(ctx context.Context, manifests [][]byte, defaultNamespace string) error {
	if i == nil || i.Client == nil || i.Client.RESTConfig == nil {
		return fmt.Errorf("kube installer is not initialized")
	}
	cfg := i.Client.RESTConfig
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create discovery client: %w", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	dy, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}
	for _, m := range manifests {
		dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(m), 4096)
		for {
			var raw map[string]interface{}
			if err := dec.Decode(&raw); err != nil {
				if err == io.EOF {
					break
				}
				return fmt.Errorf("decode yaml: %w", err)
			}
			if len(raw) == 0 {
				continue
			}
			obj := &unstructured.Unstructured{Object: raw}
			body, err := json.Marshal(raw)
			if err != nil {
				return fmt.Errorf("marshal object to json: %w", err)
			}
			if err := i.applyUnstructured(ctx, obj, body, defaultNamespace, dy, mapper); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyUnstructured performs server-side apply for a single object.
func (i *Installer) applyUnstructured(ctx context.Context, obj *unstructured.Unstructured, body []byte, defaultNamespace string, dy dynamic.Interface, mapper meta.RESTMapper) error {
	if obj.GetKind() == "" || obj.GetAPIVersion() == "" {
		// Skip docs without essential metadata
		return nil
	}
	gvk := schema.FromAPIVersionAndKind(obj.GetAPIVersion(), obj.GetKind())
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("find REST mapping for %s: %w", gvk.String(), err)
	}

	ns := obj.GetNamespace()
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace && ns == "" {
		if defaultNamespace == "" {
			defaultNamespace = "default"
		}
		obj.SetNamespace(defaultNamespace)
		ns = defaultNamespace
	}
	name := obj.GetName()
	if name == "" {
		return fmt.Errorf("object %s requires metadata.name for server-side apply", gvk.String())
	}

	ri := resourceInterfaceFor(dy, mapping.Resource, ns)
	force := true
	if _, err := ri.Patch(ctx, name, types.ApplyPatchType, body, metav1.PatchOptions{FieldManager: "kompoxops", Force: &force}); err != nil {
		return fmt.Errorf("apply %s %s: %w", obj.GetKind(), name, err)
	}
	return nil
}

// resourceInterfaceFor returns a ResourceInterface for the given GVR and namespace.
func resourceInterfaceFor(dy dynamic.Interface, gvr schema.GroupVersionResource, namespace string) dynamic.ResourceInterface {
	if namespace == "" {
		return dy.Resource(gvr)
	}
	return dy.Resource(gvr).Namespace(namespace)
}
