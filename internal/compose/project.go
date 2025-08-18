package compose

import (
	"context"
	"fmt"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/yaegashi/kompoxops/internal/logging"
)

func NewProject(ctx context.Context, composeContent string) (*types.Project, error) {
	logger := logging.FromContext(ctx)

	cdm := types.ConfigDetails{
		WorkingDir: ".",
		ConfigFiles: []types.ConfigFile{
			{Filename: "app.compose", Content: []byte(composeContent)},
		},
		Environment: map[string]string{},
	}

	model, err := loader.LoadModelWithContext(ctx, cdm, func(o *loader.Options) {
		o.SetProjectName("kompox-compose", false)
		o.SkipInclude = true
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load compose model: %w", err)
	}

	// Additional validations
	if _, ok := model["version"]; ok {
		logger.Warn(ctx, "compose: `version` is obsolete")
	}

	var proj *types.Project
	err = loader.Transform(model, &proj)
	if err != nil {
		return nil, fmt.Errorf("failed to transform compose model to project: %w", err)
	}

	return proj, nil
}
