package kompoxopscfg

import (
	"fmt"

	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/naming"
)

// Validate performs semantic validation on the configuration tree.
func (r *Root) Validate() error {
	if err := r.App.validate(); err != nil {
		return fmt.Errorf("app: %w", err)
	}
	return nil
}

func (a *App) validate() error {
	if err := a.validateVolumes(); err != nil {
		return err
	}
	if err := a.validateDeployment(); err != nil {
		return err
	}
	return nil
}

func (a *App) validateDeployment() error {
	if a.Deployment.Pool != "" && len(a.Deployment.Pools) > 0 {
		return fmt.Errorf("deployment.pool and deployment.pools cannot be specified together")
	}
	if a.Deployment.Zone != "" && len(a.Deployment.Zones) > 0 {
		return fmt.Errorf("deployment.zone and deployment.zones cannot be specified together")
	}
	if len(a.Deployment.Selectors) > 0 {
		return fmt.Errorf("deployment.selectors is reserved and not supported yet")
	}
	return nil
}

func (a *App) validateVolumes() error {
	if len(a.Volumes) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(a.Volumes))
	for i, volume := range a.Volumes {
		if err := naming.ValidateVolumeName(volume.Name); err != nil {
			return fmt.Errorf("volumes[%d].name: %w", i, err)
		}
		if _, exists := seen[volume.Name]; exists {
			return fmt.Errorf("volumes[%d].name: duplicate volume name %q", i, volume.Name)
		}
		seen[volume.Name] = struct{}{}

		// Validate Type: must be empty, "disk", or "files"
		if volume.Type != "" && volume.Type != model.VolumeTypeDisk && volume.Type != model.VolumeTypeFiles {
			return fmt.Errorf("volumes[%d].type: invalid type %q, must be %q or %q", i, volume.Type, model.VolumeTypeDisk, model.VolumeTypeFiles)
		}
	}

	return nil
}
