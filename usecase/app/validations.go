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

// Severity represents the level of a validation issue.
type Severity string

const (
	// SeverityInfo annotates non-blocking information.
	SeverityInfo Severity = "INFO"
	// SeverityWarn indicates a condition that blocks deploy but not validate.
	SeverityWarn Severity = "WARN"
	// SeverityError represents fatal validation failures.
	SeverityError Severity = "ERROR"
)

// Issue encapsulates a structured validation finding.
type Issue struct {
	Severity Severity
	Code     string
	Message  string
}

type validationResult struct {
	Issues     []Issue
	Compose    string
	Converter  *kube.Converter
	K8sObjects []runtime.Object

	app       *model.App
	cluster   *model.Cluster
	provider  *model.Provider
	workspace *model.Workspace
}

func newValidationResult(app *model.App) *validationResult {
	return &validationResult{app: app}
}

func (r *validationResult) addIssue(sev Severity, code, msg string) {
	r.Issues = append(r.Issues, Issue{Severity: sev, Code: code, Message: msg})
}

func (r *validationResult) addIssuesFromStrings(sev Severity, code string, messages []string) {
	for _, msg := range messages {
		r.addIssue(sev, code, msg)
	}
}

func severityWeight(sev Severity) int {
	switch sev {
	case SeverityError:
		return 2
	case SeverityWarn:
		return 1
	case SeverityInfo:
		return 0
	default:
		return 1
	}
}

func hasIssuesAtOrAbove(issues []Issue, threshold Severity) bool {
	limit := severityWeight(threshold)
	for _, issue := range issues {
		if severityWeight(issue.Severity) >= limit {
			return true
		}
	}
	return false
}

func countIssuesBySeverity(issues []Issue) (info, warn, err int) {
	for _, issue := range issues {
		switch issue.Severity {
		case SeverityError:
			err++
		case SeverityWarn:
			warn++
		default:
			info++
		}
	}
	return
}

func issueMessagesBySeverity(issues []Issue, target Severity) []string {
	var msgs []string
	for _, issue := range issues {
		if issue.Severity == target {
			msgs = append(msgs, issue.Message)
		}
	}
	return msgs
}

func (u *UseCase) validateApp(ctx context.Context, app *model.App) (*validationResult, error) {
	if app == nil {
		return nil, fmt.Errorf("app is nil")
	}
	res := newValidationResult(app)

	project, _, err := kube.NewComposeProject(ctx, app.Compose, app.RefBase)
	if err != nil {
		res.addIssue(SeverityError, "compose_validation_failed", fmt.Sprintf("compose validation failed: %v", err))
		return res, nil
	}
	normalized, err := project.MarshalYAML()
	if err != nil {
		res.addIssue(SeverityError, "compose_normalization_failed", fmt.Sprintf("compose normalization failed: %v", err))
		return res, nil
	}
	res.Compose = string(normalized)

	cluster, err := u.Repos.Cluster.Get(ctx, app.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}
	if cluster == nil {
		res.addIssue(SeverityWarn, "cluster_not_found", "compose conversion skipped: cluster not found")
		return res, nil
	}
	res.cluster = cluster

	provider, err := u.Repos.Provider.Get(ctx, cluster.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}
	if provider == nil {
		res.addIssue(SeverityWarn, "provider_not_found", "compose conversion skipped: provider not found")
		return res, nil
	}
	res.provider = provider

	workspace, err := u.Repos.Workspace.Get(ctx, provider.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}
	if workspace == nil {
		res.addIssue(SeverityWarn, "workspace_not_found", "compose conversion skipped: workspace not found")
		return res, nil
	}
	res.workspace = workspace

	factory, ok := providerdrv.GetDriverFactory(provider.Driver)
	if !ok {
		res.addIssue(SeverityWarn, "provider_driver_missing", "compose conversion failed: provider driver unavailable for volume class resolution")
		return res, nil
	}
	drv, err := factory(workspace, provider)
	if err != nil {
		res.addIssue(SeverityWarn, "provider_driver_init_failed", fmt.Sprintf("compose conversion failed: %v", err))
		return res, nil
	}

	conv := kube.NewConverter(workspace, provider, cluster, app, "app")
	if conv == nil {
		res.addIssue(SeverityWarn, "converter_init_failed", "compose conversion failed: converter initialization failed")
		return res, nil
	}
	warns, convErr := conv.Convert(ctx)
	if convErr != nil {
		res.addIssue(SeverityWarn, "compose_conversion_failed", fmt.Sprintf("compose conversion failed: %v", convErr))
		return res, nil
	}
	res.addIssuesFromStrings(SeverityInfo, "compose_conversion_warning", warns)

	bindings, bindingIssues, complete := u.validateAppVolumes(ctx, cluster, app, drv)
	res.Issues = append(res.Issues, bindingIssues...)
	if hasIssuesAtOrAbove(bindingIssues, SeverityError) || !complete {
		return res, nil
	}

	if bindErr := conv.BindVolumes(ctx, bindings); bindErr != nil {
		res.addIssue(SeverityWarn, "compose_conversion_failed", fmt.Sprintf("compose conversion failed: %v", bindErr))
		return res, nil
	}
	warns2, buildErr := conv.Build()
	if buildErr != nil {
		res.addIssue(SeverityWarn, "compose_conversion_failed", fmt.Sprintf("compose conversion failed: %v", buildErr))
		return res, nil
	}
	res.addIssuesFromStrings(SeverityInfo, "compose_conversion_warning", warns2)
	res.Converter = conv
	res.K8sObjects = conv.AllObjects()
	return res, nil
}

func (u *UseCase) validateAppVolumes(ctx context.Context, cluster *model.Cluster, app *model.App, drv providerdrv.Driver) ([]*kube.ConverterVolumeBinding, []Issue, bool) {
	if len(app.Volumes) == 0 {
		return nil, nil, true
	}
	var issues []Issue
	bindings := make([]*kube.ConverterVolumeBinding, len(app.Volumes))
	ready := 0

	for i, av := range app.Volumes {
		if u.VolumePort == nil {
			issues = append(issues, Issue{Severity: SeverityError, Code: "volume_port_unavailable", Message: "volume operations unavailable"})
			continue
		}
		disks, err := u.VolumePort.DiskList(ctx, cluster, app, av.Name)
		if err != nil {
			issues = append(issues, Issue{Severity: SeverityError, Code: "volume_disk_lookup_failed", Message: fmt.Sprintf("volume disk lookup failed for %s: %v", av.Name, err)})
			continue
		}
		var assigned []*model.VolumeDisk
		for _, disk := range disks {
			if disk != nil && disk.Assigned {
				assigned = append(assigned, disk)
			}
		}
		switch len(assigned) {
		case 0:
			issues = append(issues, Issue{Severity: SeverityWarn, Code: "volume_assignment_missing", Message: fmt.Sprintf("volume assignment missing (count=0) volume=%s", av.Name)})
			continue
		case 1:
			// ok
		default:
			issues = append(issues, Issue{Severity: SeverityError, Code: "volume_assignment_invalid", Message: fmt.Sprintf("volume assignment invalid (count=%d) volume=%s", len(assigned), av.Name)})
			continue
		}
		disk := assigned[0]
		if strings.TrimSpace(disk.Handle) == "" {
			issues = append(issues, Issue{Severity: SeverityError, Code: "volume_disk_handle_missing", Message: fmt.Sprintf("no assigned disk handle for volume %s", av.Name)})
			continue
		}
		if drv == nil {
			issues = append(issues, Issue{Severity: SeverityWarn, Code: "volume_class_driver_missing", Message: "compose conversion failed: provider driver unavailable for volume class resolution"})
			continue
		}
		vc, err := drv.VolumeClass(ctx, cluster, app, av)
		if err != nil {
			issues = append(issues, Issue{Severity: SeverityWarn, Code: "volume_class_resolve_failed", Message: fmt.Sprintf("compose conversion failed: volume class resolve failed for %s: %v", av.Name, err)})
			continue
		}
		class := vc
		if strings.TrimSpace(class.CSIDriver) == "" {
			issues = append(issues, Issue{Severity: SeverityWarn, Code: "volume_class_missing_driver", Message: fmt.Sprintf("compose conversion failed: volume class missing CSI driver for %s", av.Name)})
			continue
		}
		bindings[i] = &kube.ConverterVolumeBinding{
			Name:        av.Name,
			VolumeDisk:  disk,
			VolumeClass: &class,
		}
		ready++
	}
	if ready != len(app.Volumes) {
		return nil, issues, false
	}
	return bindings, issues, true
}
