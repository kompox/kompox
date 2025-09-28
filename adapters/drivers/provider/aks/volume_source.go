package aks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/kompox/kompox/domain/model"
)

// resolveSourceResourceID resolves a source string to an Azure resource ID.
// - "" (empty) -> error
// - "snapshot:name" -> Kompox managed snapshot
// - "disk:name" -> Kompox managed disk
// - "/subscriptions/..." -> Azure resource ID (snapshot or disk)
// - "arm:..." -> Azure resource ID with arm: prefix
// - "resourceId:..." -> Azure resource ID with resourceId: prefix
// - Others -> returns empty string with no error
func (d *driver) resolveSourceResourceID(ctx context.Context, app *model.App, volName string, source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("source cannot be empty")
	}

	// Convert to lowercase once for case-insensitive comparisons
	lowerSource := strings.ToLower(source)

	// Handle explicit Azure resource IDs (with various prefixes)
	if strings.HasPrefix(lowerSource, "/subscriptions/") {
		// Direct Azure resource ID
		return source, nil
	}
	if strings.HasPrefix(lowerSource, "arm:") {
		// Strip "arm:" prefix (case-insensitive)
		return source[4:], nil
	}
	if strings.HasPrefix(lowerSource, "resourceid:") {
		// Strip "resourceId:" prefix (case-insensitive)
		return source[11:], nil
	}

	// Handle Kompox managed resources
	if strings.HasPrefix(lowerSource, "disk:") {
		// Explicit disk reference (case-insensitive)
		return d.resolveKompoxDiskResourceID(ctx, app, volName, source[5:])
	}
	if strings.HasPrefix(lowerSource, "snapshot:") {
		// Explicit snapshot reference (case-insensitive)
		return d.resolveKompoxSnapshotResourceID(ctx, app, volName, source[9:])
	}

	// Return empty with no error if no known pattern matched
	return "", nil
}

// resolveSourceDiskResourceID resolves a source string to an Azure Disk resource ID (default to "disk:source" if unknown).
func (d *driver) resolveSourceDiskResourceID(ctx context.Context, app *model.App, volName string, source string) (string, error) {
	src, err := d.resolveSourceResourceID(ctx, app, volName, source)
	if src == "" && err == nil {
		src, err = d.resolveKompoxDiskResourceID(ctx, app, volName, source)
	}
	return src, err
}

// resolveSourceSnapshotResourceID resolves a source string to an Azure Snapshot resource ID (default to "snapshot:source" if unknown).
func (d *driver) resolveSourceSnapshotResourceID(ctx context.Context, app *model.App, volName string, source string) (string, error) {
	src, err := d.resolveSourceResourceID(ctx, app, volName, source)
	if src == "" && err == nil {
		src, err = d.resolveKompoxSnapshotResourceID(ctx, app, volName, source)
	}
	return src, err
}

// resolveKompoxDiskResourceID resolves a Kompox managed disk name to its Azure resource ID.
func (d *driver) resolveKompoxDiskResourceID(ctx context.Context, app *model.App, volName string, diskName string) (string, error) {
	if diskName == "" {
		return "", fmt.Errorf("disk name cannot be empty")
	}

	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return "", fmt.Errorf("app RG: %w", err)
	}

	diskResourceName, err := d.appDiskName(app, volName, diskName)
	if err != nil {
		return "", fmt.Errorf("generate disk resource name: %w", err)
	}

	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return "", fmt.Errorf("new disks client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	diskRes, err := disksClient.Get(ctx, rg, diskResourceName, nil)
	if err != nil {
		return "", fmt.Errorf("get disk %q: %w", diskName, err)
	}

	if diskRes.ID == nil {
		return "", fmt.Errorf("disk %q has no resource ID", diskName)
	}

	return *diskRes.ID, nil
}

// resolveKompoxSnapshotResourceID resolves a Kompox managed snapshot name to its Azure resource ID.
func (d *driver) resolveKompoxSnapshotResourceID(ctx context.Context, app *model.App, volName string, snapName string) (string, error) {
	if snapName == "" {
		return "", fmt.Errorf("snapshot name cannot be empty")
	}

	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return "", fmt.Errorf("app RG: %w", err)
	}

	snapResourceName, err := d.appSnapshotName(app, volName, snapName)
	if err != nil {
		return "", fmt.Errorf("generate snapshot resource name: %w", err)
	}

	snapsClient, err := armcompute.NewSnapshotsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return "", fmt.Errorf("new snapshots client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	snapRes, err := snapsClient.Get(ctx, rg, snapResourceName, nil)
	if err != nil {
		return "", fmt.Errorf("get snapshot %q: %w", snapName, err)
	}

	if snapRes.ID == nil {
		return "", fmt.Errorf("snapshot %q has no resource ID", snapName)
	}

	return *snapRes.ID, nil
}
