package kube

import (
	"context"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
)

// TestNewComposeProjectVolumesNormalization verifies how compose-go normalizes
// service volumes for 4 patterns described in the converter spec:
//   - Abs path bind: /sub/path:/mount/path (should parse as bind with source "/sub/path")
//   - Rel path bind: ./sub/path:/mount/path (should parse as bind with source "sub/path")
//   - Root path volume: name:/mount/path (should parse as volume with source "name")
//   - Sub path volume: name/sub/path:/mount/path (should parse as volume with source "name/sub/path")
func TestNewComposeProjectVolumesNormalization(t *testing.T) {
	ctx := context.Background()

	compose := `
services:
  app:
    image: busybox:1.36
    volumes:
      - /abs/path:/mnt/string/abs
      - ./rel/path:/mnt/string/rel
      - data:/mnt/string/root
      - data/sub/path:/mnt/string/sub
      - type: bind
        source: /abs/path
        target: /mnt/struct/abs
      - type: bind
        source: ./rel/path
        target: /mnt/struct/rel
      - type: volume
        source: data
        target: /mnt/struct/root
      - type: volume
        source: data/sub/path
        target: /mnt/struct/sub
`

	proj, err := NewComposeProject(ctx, compose)
	if err != nil {
		t.Fatalf("NewComposeProject error: %v", err)
	}

	if len(proj.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(proj.Services))
	}

	var s types.ServiceConfig
	for _, sv := range proj.Services {
		s = sv
		break
	}
	if got, want := len(s.Volumes), 8; got != want {
		t.Fatalf("expected %d volumes, got %d", want, got)
	}

	// Helper to assert a single ServiceVolumeConfig
	assertVol := func(idx int, wantType, wantSource, wantTarget string) {
		v := s.Volumes[idx]
		t.Logf("LOG vol[%d] Type=%q Source=%q Target=%q", idx, v.Type, v.Source, v.Target)
		if v.Type != wantType {
			t.Errorf("ERR vol[%d] wants %q for Type", idx, wantType)
		}
		if v.Source != wantSource {
			t.Errorf("ERR vol[%d] wants %q for Source", idx, wantSource)
		}
		if v.Target != wantTarget {
			t.Errorf("ERR vol[%d] wants %q for Target", idx, wantTarget)
		}
	}

	assertVol(0, "bind", "/abs/path", "/mnt/string/abs")
	assertVol(1, "bind", "rel/path", "/mnt/string/rel")
	assertVol(2, "volume", "data", "/mnt/string/root")
	assertVol(3, "volume", "data/sub/path", "/mnt/string/sub")
	assertVol(4, "bind", "/abs/path", "/mnt/struct/abs")
	assertVol(5, "bind", "rel/path", "/mnt/struct/rel")
	assertVol(6, "volume", "data", "/mnt/struct/root")
	assertVol(7, "volume", "data/sub/path", "/mnt/struct/sub")
}
