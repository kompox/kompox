package app

import (
	"context"
	"fmt"

	providerdrv "github.com/yaegashi/kompoxops/adapters/drivers/provider"
	"github.com/yaegashi/kompoxops/adapters/kube"
	"github.com/yaegashi/kompoxops/domain/model"
	"k8s.io/apimachinery/pkg/runtime"
)

// ValidateInput represents parameters to Validate.
// ValidateInput identifies the App whose compose specification should be validated.
type ValidateInput struct {
	// AppID is the application being validated.
	AppID string `json:"app_id"`
}

// ValidateOutput reports validation outcomes.
type ValidateOutput struct {
	// Errors are fatal validation failures.
	Errors []string
	// Warnings are non-fatal issues.
	Warnings []string
	// Compose is the normalized compose YAML when validation succeeds.
	Compose string // normalized compose YAML (if valid)
	// K8sObjects are generated Kubernetes manifests if conversion succeeds.
	K8sObjects []runtime.Object // converted Kubernetes objects (nil if conversion failed)
}

// Validate checks the compose string stored in an App resource.
// It performs syntactic validation and best-effort conversion to Kubernetes objects.
func (u *UseCase) Validate(ctx context.Context, in *ValidateInput) (*ValidateOutput, error) {
	out := &ValidateOutput{}
	if in == nil || in.AppID == "" {
		return out, fmt.Errorf("missing app ID")
	}

	a, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil {
		return out, fmt.Errorf("failed to get app: %w", err)
	}
	if a == nil {
		return out, fmt.Errorf("app not found: %s", in.AppID)
	}

	pro, err := kube.ComposeAppToProject(ctx, a.Compose)
	if err != nil {
		return out, fmt.Errorf("compose project failed: %w", err)
	}

	b, err := pro.MarshalYAML()
	if err != nil {
		return out, fmt.Errorf("failed to marshal project to YAML: %w", err)
	}
	out.Compose = string(b)

	// Fetch related resources for hash & conversion
	cls, err := u.Repos.Cluster.Get(ctx, a.ClusterID)
	if err == nil && cls != nil {
		prv, _ := u.Repos.Provider.Get(ctx, cls.ProviderID)
		var svc *model.Service
		if prv != nil {
			svc, _ = u.Repos.Service.Get(ctx, prv.ServiceID)
		}
		if svc != nil && prv != nil {
			// Instantiate provider driver for conversion (volume class, etc.)
			var drv providerdrv.Driver
			if factory, ok := providerdrv.GetDriverFactory(prv.Driver); ok {
				if d, derr := factory(svc, prv); derr == nil {
					drv = d
				}
			}
			// Collect assigned volume instances (one per logical volume) via VolumePort if available.
			vmap := map[string]kube.VolumeInstanceInfo{}
			if u.VolumePort != nil && len(a.Volumes) > 0 {
				for _, av := range a.Volumes {
					// list instances
					insts, lerr := u.VolumePort.VolumeInstanceList(ctx, cls, a, av.Name)
					if lerr != nil {
						continue // ignore errors; validation should still proceed
					}
					// choose newest assigned (Assigned true). If multiple, pick most recent CreatedAt.
					var chosen *model.AppVolumeInstance
					for _, inst := range insts {
						if inst.Assigned {
							if chosen == nil || inst.CreatedAt.After(chosen.CreatedAt) {
								chosen = inst
							}
						}
					}
					if chosen != nil {
						size := chosen.Size
						if size <= 0 && av.Size > 0 {
							size = av.Size
						}
						if size <= 0 {
							size = 32 * (1 << 30) // default
						}
						vmap[av.Name] = kube.VolumeInstanceInfo{Handle: chosen.Handle, Size: size}
					}
				}
			}
			objs, warns, convErr := kube.ComposeAppToObjects(ctx, svc, prv, cls, a, vmap, drv)
			if convErr != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("compose conversion failed: %v", convErr))
			} else {
				out.K8sObjects = objs
				out.Warnings = append(out.Warnings, warns...)
			}
		}
	}

	return out, nil
}
