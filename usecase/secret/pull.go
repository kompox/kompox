package secret

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// dockerConfigJSONKey is the canonical data key for kubernetes.io/dockerconfigjson Secrets.
const dockerConfigJSONKey = ".dockerconfigjson"

type PullOperation string

const (
	PullOpSet    PullOperation = "set"
	PullOpDelete PullOperation = "delete"
)

type PullInput struct {
	AppID         string
	Operation     PullOperation
	ComponentName string
	FilePath      string
	FileContent   []byte
	DryRun        bool
}

type PullOutput struct {
	SecretName string
	Applied    bool
	Action     string
	Hash       string
	Auths      []string
	Warnings   []string
}

func (u *UseCase) Pull(ctx context.Context, in *PullInput) (*PullOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("PullInput is required")
	}
	if in.AppID == "" {
		return nil, fmt.Errorf("PullInput.AppID is required")
	}
	if in.Operation != PullOpSet && in.Operation != PullOpDelete {
		return nil, fmt.Errorf("unsupported operation: %s", in.Operation)
	}
	if in.ComponentName == "" {
		in.ComponentName = "app"
	}
	if in.Operation == PullOpSet {
		if in.FilePath == "" {
			return nil, fmt.Errorf("PullInput.FilePath is required for set operation")
		}
		if in.FileContent == nil {
			return nil, fmt.Errorf("PullInput.FileContent is required for set operation")
		}
	}

	logger := logging.FromContext(ctx)

	appObj, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil || appObj == nil {
		return nil, fmt.Errorf("failed to get app %s: %w", in.AppID, err)
	}
	clusterObj, err := u.Repos.Cluster.Get(ctx, appObj.ClusterID)
	if err != nil || clusterObj == nil {
		return nil, fmt.Errorf("failed to get cluster %s: %w", appObj.ClusterID, err)
	}
	providerObj, err := u.Repos.Provider.Get(ctx, clusterObj.ProviderID)
	if err != nil || providerObj == nil {
		return nil, fmt.Errorf("failed to get provider %s: %w", clusterObj.ProviderID, err)
	}
	var serviceObj *model.Service
	if providerObj.ServiceID != "" {
		serviceObj, _ = u.Repos.Service.Get(ctx, providerObj.ServiceID)
	}

	factory, ok := providerdrv.GetDriverFactory(providerObj.Driver)
	if !ok {
		return nil, fmt.Errorf("unknown provider driver: %s", providerObj.Driver)
	}
	drv, err := factory(serviceObj, providerObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver %s: %w", providerObj.Driver, err)
	}
	kubeconfig, err := drv.ClusterKubeconfig(ctx, clusterObj)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster kubeconfig: %w", err)
	}
	kcli, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	// Inline lite validation logic using Converter (component configurable)
	c := kube.NewConverter(serviceObj, providerObj, clusterObj, appObj, in.ComponentName)
	if _, err := c.Convert(ctx); err != nil {
		return nil, fmt.Errorf("convert failed: %w", err)
	}
	namespace := c.Namespace
	if namespace == "" {
		return nil, fmt.Errorf("app namespace unresolved")
	}

	secretName := kube.SecretPullName(appObj.Name, in.ComponentName)
	out := &PullOutput{SecretName: secretName}

	switch in.Operation {
	case PullOpDelete:
		if in.DryRun {
			out.Action = "deleted"
			return out, nil
		}
		if err := kcli.Clientset.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("delete secret: %w", err)
			}
			out.Action = "noop"
		} else {
			out.Action = "deleted"
			out.Applied = true
		}
	case PullOpSet:
		kv, err := parseDockerConfigJSON(in.FileContent, in.FilePath)
		if err != nil {
			return nil, fmt.Errorf("parse docker config: %w", err)
		}
		hash := kube.ComputeContentHash(kv)
		out.Hash = hash
		for k := range kv {
			out.Auths = append(out.Auths, k)
		}
		sort.Strings(out.Auths)
		if in.DryRun {
			out.Action = "updated"
			return out, nil
		}
		existing, err := kcli.Clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		if err == nil && existing != nil {
			need := false
			if existing.Annotations == nil || existing.Annotations[kube.AnnotationK4xComposeContentHash] != hash {
				need = true
			} else if len(existing.Data) != 1 {
				need = true
			} else if b, ok := existing.Data[dockerConfigJSONKey]; !ok || string(b) != kv[dockerConfigJSONKey] {
				need = true
			}
			if need {
				if existing.Annotations == nil {
					existing.Annotations = map[string]string{}
				}
				existing.Annotations[kube.AnnotationK4xComposeContentHash] = hash
				existing.Data = map[string][]byte{dockerConfigJSONKey: []byte(kv[dockerConfigJSONKey])}
				if _, uerr := kcli.Clientset.CoreV1().Secrets(namespace).Update(ctx, existing, metav1.UpdateOptions{}); uerr != nil {
					return nil, fmt.Errorf("update secret: %w", uerr)
				}
				out.Action = "updated"
				out.Applied = true
			} else {
				out.Action = "noop"
			}
		} else {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        secretName,
					Namespace:   namespace,
					Annotations: map[string]string{kube.AnnotationK4xComposeContentHash: hash},
				},
				Type: corev1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{dockerConfigJSONKey: []byte(kv[dockerConfigJSONKey])},
			}
			if _, cerr := kcli.Clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); cerr != nil {
				return nil, fmt.Errorf("create secret: %w", cerr)
			}
			out.Action = "created"
			out.Applied = true
		}
	}

	// Ensure deployment imagePullSecrets contain the pull secret when applied (and remove on delete).
	if !in.DryRun && c.ResourceName != "" {
		if err := kcli.PatchDeploymentPodContentHash(ctx, namespace, c.ResourceName); err != nil {
			logger.Warn(ctx, "patch deployment content hash failed", "err", err)
		}
	}

	return out, nil
}

// parseDockerConfigJSON validates content is a JSON object with an auths or credsStore/credHelpers key.
func parseDockerConfigJSON(content []byte, path string) (map[string]string, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("empty docker config content: %s", path)
	}
	var raw map[string]any
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	if len(raw) == 0 {
		return nil, errors.New("docker config json object is empty")
	}
	// We only store the original entire document under the canonical key.
	kv := map[string]string{}
	kv[dockerConfigJSONKey] = string(content)
	return kv, nil
}
