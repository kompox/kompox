package aks

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

func TestBuildZones(t *testing.T) {
	tests := []struct {
		name     string
		zone     string
		expected []*string
	}{
		{
			name:     "empty zone should return nil",
			zone:     "",
			expected: nil,
		},
		{
			name:     "whitespace zone should return nil",
			zone:     "  ",
			expected: nil,
		},
		{
			name:     "zone 1",
			zone:     "1",
			expected: []*string{to.Ptr("1")},
		},
		{
			name:     "zone 2",
			zone:     "2",
			expected: []*string{to.Ptr("2")},
		},
		{
			name:     "zone 3",
			zone:     "3",
			expected: []*string{to.Ptr("3")},
		},
		{
			name:     "zone with whitespace",
			zone:     " 2 ",
			expected: []*string{to.Ptr("2")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildZones(tt.zone)

			if tt.expected == nil && result != nil {
				t.Errorf("expected nil, got %v", result)
				return
			}

			if tt.expected != nil && result == nil {
				t.Errorf("expected %v, got nil", tt.expected)
				return
			}

			if tt.expected == nil && result == nil {
				return // Both nil, test passes
			}

			if len(result) != len(tt.expected) {
				t.Errorf("expected length %d, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] == nil || expected == nil {
					t.Errorf("unexpected nil pointer at index %d", i)
					return
				}
				if *result[i] != *expected {
					t.Errorf("expected %s, got %s", *expected, *result[i])
				}
			}
		})
	}
}
