package secret

import (
	"context"
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

type EnvOperation string

const (
	EnvOpSet    EnvOperation = "set"
	EnvOpDelete EnvOperation = "delete"
)

type EnvInput struct {
	AppID         string
	Operation     EnvOperation
	ComponentName string
	ContainerName string
	FilePath      string
	FileContent   []byte
	DryRun        bool
}
type EnvOutput struct {
	SecretName string
	Applied    bool
	Action     string
	Hash       string
	Keys       []string
	Warnings   []string
}

// Env manages override environment variables for a compose service container.
func (u *UseCase) Env(ctx context.Context, in *EnvInput) (*EnvOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("EnvInput is required")
	}
	if in.AppID == "" {
		return nil, fmt.Errorf("EnvInput.AppID is required")
	}
	if in.Operation != EnvOpSet && in.Operation != EnvOpDelete {
		return nil, fmt.Errorf("unsupported operation: %s", in.Operation)
	}
	if in.ComponentName == "" {
		in.ComponentName = "app"
	}
	if in.ContainerName == "" {
		return nil, fmt.Errorf("EnvInput.ContainerName is required")
	}
	if in.Operation == EnvOpSet {
		if in.FilePath == "" {
			return nil, fmt.Errorf("EnvInput.FilePath is required for set operation")
		}
		if in.FileContent == nil {
			return nil, fmt.Errorf("EnvInput.FileContent is required for set operation")
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

	secretName := kube.SecretEnvOverrideName(appObj.Name, in.ComponentName, in.ContainerName)
	out := &EnvOutput{SecretName: secretName}

	switch in.Operation {
	case EnvOpDelete:
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
	case EnvOpSet:
		kv, err := kube.ReadEnv(in.FileContent, in.FilePath)
		if err != nil {
			return nil, fmt.Errorf("parse env content: %w", err)
		}
		if err := kube.ValidateSecretData(kv); err != nil {
			return nil, fmt.Errorf("validate env: %w", err)
		}
		hash := kube.ComputeSecretHash(kv)
		out.Hash = hash
		for k := range kv {
			out.Keys = append(out.Keys, k)
		}
		sort.Strings(out.Keys)
		if in.DryRun {
			out.Action = "updated"
			return out, nil
		}
		existing, err := kcli.Clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		if err == nil && existing != nil {
			need := false
			if existing.Annotations == nil || existing.Annotations[kube.AnnotationK4xComposeSecretHash] != hash {
				need = true
			} else if len(existing.Data) != len(kv) {
				need = true
			} else {
				for k, v := range kv {
					if ev, ok := existing.Data[k]; !ok || string(ev) != v {
						need = true
						break
					}
				}
			}
			if need {
				if existing.Annotations == nil {
					existing.Annotations = map[string]string{}
				}
				existing.Annotations[kube.AnnotationK4xComposeSecretHash] = hash
				existing.Data = map[string][]byte{}
				for k, v := range kv {
					existing.Data[k] = []byte(v)
				}
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
					Annotations: map[string]string{kube.AnnotationK4xComposeSecretHash: hash},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{},
			}
			for k, v := range kv {
				secret.Data[k] = []byte(v)
			}
			if _, cerr := kcli.Clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{}); cerr != nil {
				return nil, fmt.Errorf("create secret: %w", cerr)
			}
			out.Action = "created"
			out.Applied = true
		}
	}

	if !in.DryRun && out.Applied && c.ResourceName != "" {
		if err := kcli.PatchDeploymentPodSecretHash(ctx, namespace, c.ResourceName); err != nil {
			logger.Warn(ctx, "patch deployment secret hash failed", "err", err)
		}
	}
	return out, nil
}
