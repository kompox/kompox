package volume

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/kompox/kompox/domain/model"
)

// DiskCreateBootstrapInput parameters for DiskCreateBootstrap use case.
// Creates exactly one assigned disk per defined app volume when and only when
// no volume currently has an assigned disk. If any volume already has one or
// more assigned disks the operation becomes either a no-op (all volumes have
// exactly one assigned) or an error (partial / inconsistent state).
type DiskCreateBootstrapInput struct {
	// AppID owning application identifier.
	AppID string `json:"app_id"`
	// Zone overrides app.deployment.zone when specified (applied to all creates).
	Zone string `json:"zone,omitempty"`
	// Options overrides/merges with app.volumes.options when specified (applied per volume).
	Options map[string]any `json:"options,omitempty"`
}

// DiskCreateBootstrapOutput result for DiskCreateBootstrap use case.
type DiskCreateBootstrapOutput struct {
	// Created holds newly created disks when bootstrap executed. Empty for no-op.
	Created []*DiskCreateOutput `json:"created"`
	// Skipped true when bootstrap conditions not met (already initialized).
	Skipped bool `json:"skipped"`
	// Reason provides short explanation for skip or error-like condition reported as normal result.
	Reason string `json:"reason,omitempty"`
	// Duration processing time for observability.
	Duration time.Duration `json:"duration"`
}

// DiskCreateBootstrap performs initial disk creation for all app volumes when safe.
// Rules:
//
//	Success-create: All volumes have Assigned=0 (and any existing disks are either 0 or unassigned) -> create one disk per volume.
//	Success-skip:   All volumes already have exactly one Assigned disk -> no-op (Skipped=true).
//	Error:          Any other combination (some volumes assigned, others not) or ambiguous state.
func (u *UseCase) DiskCreateBootstrap(ctx context.Context, in *DiskCreateBootstrapInput) (*DiskCreateBootstrapOutput, error) {
	start := time.Now()
	if in == nil || in.AppID == "" {
		return nil, fmt.Errorf("missing parameters")
	}
	app, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, fmt.Errorf("app not found: %s", in.AppID)
	}
	cluster, err := u.Repos.Cluster.Get(ctx, app.ClusterID)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster not found: %s", app.ClusterID)
	}
	if len(app.Volumes) == 0 {
		return &DiskCreateBootstrapOutput{Skipped: true, Reason: "no volumes defined", Duration: time.Since(start)}, nil
	}

	// Gather state per volume.
	type volState struct {
		name     string
		assigned int
		total    int
	}
	states := make([]volState, 0, len(app.Volumes))
	for _, av := range app.Volumes {
		disks, lerr := u.VolumePort.DiskList(ctx, cluster, app, av.Name)
		if lerr != nil {
			return nil, fmt.Errorf("volume disk lookup failed for %s: %w", av.Name, lerr)
		}
		st := volState{name: av.Name, total: len(disks)}
		for _, d := range disks {
			if d.Assigned {
				st.assigned++
			}
		}
		states = append(states, st)
	}

	// Sort for deterministic error messages.
	sort.Slice(states, func(i, j int) bool { return states[i].name < states[j].name })

	allZero := true
	allOne := true
	for _, s := range states {
		if s.assigned != 0 {
			allZero = false
		}
		if s.assigned != 1 {
			allOne = false
		}
	}

	out := &DiskCreateBootstrapOutput{Duration: time.Since(start)}
	switch {
	case allZero:
		// Create one disk per volume.
		var created []*DiskCreateOutput
		for _, av := range app.Volumes {
			// Re-check just before create for this volume (light TOCTOU mitigation).
			disks, lerr := u.VolumePort.DiskList(ctx, cluster, app, av.Name)
			if lerr != nil {
				return nil, fmt.Errorf("volume disk recheck failed for %s: %w", av.Name, lerr)
			}
			var assignedNow int
			for _, d := range disks {
				if d.Assigned {
					assignedNow++
				}
			}
			if assignedNow != 0 {
				return nil, fmt.Errorf("bootstrap aborted: concurrent assignment detected for volume %s", av.Name)
			}
			// Build create options.
			var opts []model.VolumeDiskCreateOption
			if in.Zone != "" {
				opts = append(opts, model.WithVolumeDiskCreateZone(in.Zone))
			}
			if in.Options != nil {
				opts = append(opts, model.WithVolumeDiskCreateOptions(in.Options))
			}
			disk, cerr := u.VolumePort.DiskCreate(ctx, cluster, app, av.Name, opts...)
			if cerr != nil {
				return nil, fmt.Errorf("disk create failed for volume %s: %w", av.Name, cerr)
			}
			// Assign immediately (assumption: driver returns Assigned=false by default; assignment enforces invariants)
			if err := u.VolumePort.DiskAssign(ctx, cluster, app, av.Name, disk.Name); err != nil {
				return nil, fmt.Errorf("disk assign failed for volume %s: %w", av.Name, err)
			}
			created = append(created, &DiskCreateOutput{Disk: disk})
		}
		out.Created = created
		out.Duration = time.Since(start)
		return out, nil
	case allOne:
		out.Skipped = true
		out.Reason = "already initialized"
		out.Duration = time.Since(start)
		return out, nil
	default:
		// Partial / inconsistent
		msg := "bootstrap invalid state:"
		for _, s := range states {
			msg += fmt.Sprintf(" %s(assigned=%d,total=%d)", s.name, s.assigned, s.total)
		}
		return nil, errors.New(msg)
	}
}
