package volume

import (
	"context"
	"strings"
	"testing"
)

func TestUseCaseValidatesNames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	u := &UseCase{}

	validVolume := "volname"

	cases := []struct {
		name          string
		call          func() error
		expectedToken string
	}{
		{
			name: "disk create invalid volume",
			call: func() error {
				_, err := u.DiskCreate(ctx, &DiskCreateInput{AppID: "app", VolumeName: "Invalid"})
				return err
			},
			expectedToken: "validate volume name",
		},
		{
			name: "disk create invalid disk",
			call: func() error {
				_, err := u.DiskCreate(ctx, &DiskCreateInput{AppID: "app", VolumeName: validVolume, DiskName: "BadDisk"})
				return err
			},
			expectedToken: "validate disk name",
		},
		{
			name: "disk assign invalid disk",
			call: func() error {
				_, err := u.DiskAssign(ctx, &DiskAssignInput{AppID: "app", VolumeName: validVolume, DiskName: "INVALID"})
				return err
			},
			expectedToken: "validate disk name",
		},
		{
			name: "disk delete invalid disk",
			call: func() error {
				_, err := u.DiskDelete(ctx, &DiskDeleteInput{AppID: "app", VolumeName: validVolume, DiskName: "INVALID"})
				return err
			},
			expectedToken: "validate disk name",
		},
		{
			name: "disk list invalid volume",
			call: func() error {
				_, err := u.DiskList(ctx, &DiskListInput{AppID: "app", VolumeName: "Invalid"})
				return err
			},
			expectedToken: "validate volume name",
		},
		{
			name: "snapshot create invalid volume",
			call: func() error {
				_, err := u.SnapshotCreate(ctx, &SnapshotCreateInput{AppID: "app", VolumeName: "Invalid"})
				return err
			},
			expectedToken: "validate volume name",
		},
		{
			name: "snapshot create invalid snapshot",
			call: func() error {
				_, err := u.SnapshotCreate(ctx, &SnapshotCreateInput{AppID: "app", VolumeName: validVolume, SnapshotName: "INVALID"})
				return err
			},
			expectedToken: "validate snapshot name",
		},
		{
			name: "snapshot delete invalid snapshot",
			call: func() error {
				_, err := u.SnapshotDelete(ctx, &SnapshotDeleteInput{AppID: "app", VolumeName: validVolume, SnapshotName: "INVALID"})
				return err
			},
			expectedToken: "validate snapshot name",
		},
		{
			name: "snapshot list invalid volume",
			call: func() error {
				_, err := u.SnapshotList(ctx, &SnapshotListInput{AppID: "app", VolumeName: "Invalid"})
				return err
			},
			expectedToken: "validate volume name",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.call()
			if err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !strings.Contains(err.Error(), tc.expectedToken) {
				t.Fatalf("unexpected error message: %v", err)
			}
		})
	}
}
