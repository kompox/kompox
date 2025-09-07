package app

import (
	"context"
	"fmt"
	"strings"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
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
	Compose string
	// K8sObjects are generated Kubernetes manifests if conversion succeeds.
	K8sObjects []runtime.Object
}

// Validate checks the compose string stored in an App resource.
// It performs syntactic validation and best-effort conversion to Kubernetes objects.
func (u *UseCase) Validate(ctx context.Context, in *ValidateInput) (*ValidateOutput, error) {
	out := &ValidateOutput{}
	if in == nil || in.AppID == "" {
		return out, fmt.Errorf("missing app ID")
	}

	app, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil {
		return out, fmt.Errorf("failed to get app: %w", err)
	}
	if app == nil {
		return out, fmt.Errorf("app not found: %s", in.AppID)
	}

	pro, err := kube.NewComposeProject(ctx, app.Compose)
	if err != nil {
		// Treat compose parsing failures as validation errors, not transport errors.
		out.Errors = append(out.Errors, fmt.Sprintf("compose validation failed: %v", err))
		return out, nil
	}

	b, err := pro.MarshalYAML()
	if err != nil {
		// Normalization failure is also a validation error from the caller's perspective.
		out.Errors = append(out.Errors, fmt.Sprintf("compose normalization failed: %v", err))
		return out, nil
	}
	out.Compose = string(b)

	// Fetch related resources for hash & conversion
	cls, err := u.Repos.Cluster.Get(ctx, app.ClusterID)
	if err != nil {
		// Repository error means the use case cannot proceed reliably.
		return out, fmt.Errorf("failed to get cluster: %w", err)
	}
	if cls == nil {
		// Conversion is optional; warn and return validated compose result.
		out.Warnings = append(out.Warnings, "compose conversion skipped: cluster not found")
		return out, nil
	}
	prv, perr := u.Repos.Provider.Get(ctx, cls.ProviderID)
	if perr != nil {
		return out, fmt.Errorf("failed to get provider: %w", perr)
	}
	if prv == nil {
		out.Warnings = append(out.Warnings, "compose conversion skipped: provider not found")
		return out, nil
	}
	svc, serr := u.Repos.Service.Get(ctx, prv.ServiceID)
	if serr != nil {
		return out, fmt.Errorf("failed to get service: %w", serr)
	}
	if svc == nil {
		out.Warnings = append(out.Warnings, "compose conversion skipped: service not found")
		return out, nil
	}
	// Instantiate provider driver for conversion (volume class, etc.)
	var drv providerdrv.Driver
	if factory, ok := providerdrv.GetDriverFactory(prv.Driver); ok {
		if d, derr := factory(svc, prv); derr == nil {
			drv = d
		}
	}
	// Build volume bindings first and collect fatal errors.
	binds := make([]*kube.ConverterVolumeBinding, 0, len(app.Volumes))
	for _, av := range app.Volumes {
		var chosenDisk *model.VolumeDisk
		if u.VolumePort != nil {
			disks, lerr := u.VolumePort.DiskList(ctx, cls, app, av.Name)
			if lerr != nil {
				out.Errors = append(out.Errors, fmt.Sprintf("volume disk lookup failed for %s: %v", av.Name, lerr))
				continue
			}
			var assigned []*model.VolumeDisk
			for _, disk := range disks {
				if disk.Assigned {
					assigned = append(assigned, disk)
				}
			}
			if len(assigned) != 1 {
				out.Errors = append(out.Errors, fmt.Sprintf("volume assignment invalid for %s (count=%d)", av.Name, len(assigned)))
				continue
			}
			chosenDisk = assigned[0]
		}
		if chosenDisk == nil || strings.TrimSpace(chosenDisk.Handle) == "" {
			out.Errors = append(out.Errors, fmt.Sprintf("no assigned disk handle for volume %s", av.Name))
			continue
		}
		// Resolve provider-specific volume class (non-fatal on failure)
		var vc model.VolumeClass
		if drv != nil {
			if res, rerr := drv.VolumeClass(ctx, cls, app, av); rerr != nil {
				out.Warnings = append(out.Warnings, fmt.Sprintf("compose conversion failed: volume class resolve failed for %s: %v", av.Name, rerr))
			} else {
				vc = res
			}
		} else {
			out.Warnings = append(out.Warnings, "compose conversion failed: provider driver unavailable for volume class resolution")
		}
		binds = append(binds, &kube.ConverterVolumeBinding{
			Name:        av.Name,
			VolumeDisk:  chosenDisk,
			VolumeClass: &vc,
		})
	}

	// Only attempt conversion if no fatal errors have been collected.
	if len(out.Errors) > 0 {
		return out, nil
	}
	conv := kube.NewConverter(svc, prv, cls, app, "app")
	if conv == nil {
		out.Warnings = append(out.Warnings, "compose conversion failed: converter initialization failed")
		return out, nil
	}
	warns1, convErr := conv.Convert(ctx)
	if convErr != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("compose conversion failed: %v", convErr))
		return out, nil
	}
	if bindErr := conv.BindVolumes(ctx, binds); bindErr != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("compose conversion failed: %v", bindErr))
		return out, nil
	}
	objs, warns2, buildErr := conv.Build()
	if buildErr != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("compose conversion failed: %v", buildErr))
		return out, nil
	}
	out.K8sObjects = objs
	out.Warnings = append(out.Warnings, warns1...)
	out.Warnings = append(out.Warnings, warns2...)
	return out, nil
}
