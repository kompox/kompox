package naming

import (
	"strings"
	"testing"
)

func TestValidateVolumeName(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid short", value: "vol0", wantErr: false},
		{name: "valid max length", value: strings.Repeat("a", volumeNameMaxLength), wantErr: false},
		{name: "too long", value: strings.Repeat("a", volumeNameMaxLength+1), wantErr: true},
		{name: "contains uppercase", value: "Volname", wantErr: true},
		{name: "starts with hyphen", value: "-volname", wantErr: true},
		{name: "ends with hyphen", value: "volname-", wantErr: true},
		{name: "contains underscore", value: "vol_name", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateVolumeName(tc.value)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateDiskName(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid", value: "disk-123", wantErr: false},
		{name: "valid max length", value: strings.Repeat("a", diskNameMaxLength), wantErr: false},
		{name: "too long", value: strings.Repeat("a", diskNameMaxLength+1), wantErr: true},
		{name: "invalid char", value: "disk^name", wantErr: true},
		{name: "empty", value: "", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDiskName(tc.value)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateSnapshotName(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid", value: "snap1", wantErr: false},
		{name: "valid max length", value: strings.Repeat("a", snapshotNameMaxLength), wantErr: false},
		{name: "too long", value: strings.Repeat("a", snapshotNameMaxLength+1), wantErr: true},
		{name: "invalid hyphen placement", value: "-snap", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSnapshotName(tc.value)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
