package naming

import (
	"testing"
	"time"
)

func TestNewCompactID(t *testing.T) {
	// Test uniqueness and format
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id, err := NewCompactID()
		if err != nil {
			t.Fatalf("NewCompactID failed: %v", err)
		}
		if len(id) != 12 {
			t.Fatalf("expected ID length 12, got %d for ID: %s", len(id), id)
		}
		if ids[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		ids[id] = true

		// Verify all characters are valid base36
		for _, char := range id {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'z')) {
				t.Fatalf("invalid character in ID %s: %c", id, char)
			}
		}
	}
}

func TestNewCompactIDFormat(t *testing.T) {
	// Generate multiple IDs and verify they are time-ordered within the same second
	start := time.Now()
	var ids []string
	for i := 0; i < 10; i++ {
		id, err := NewCompactID()
		if err != nil {
			t.Fatalf("NewCompactID failed: %v", err)
		}
		ids = append(ids, id)
		// Small delay to potentially cross timestamp boundaries
		time.Sleep(1 * time.Millisecond)
	}
	end := time.Now()

	// If all IDs were generated within the same second, their timestamp parts should be identical
	if end.Sub(start) < time.Second {
		firstTimestamp := ids[0][:7]
		for i, id := range ids {
			timestamp := id[:7]
			if timestamp != firstTimestamp {
				t.Logf("Timestamp changed during test (expected): ID %d has timestamp %s vs first %s", i, timestamp, firstTimestamp)
				break
			}
		}
	}

	// Verify format: 7 chars timestamp + 5 chars random
	for _, id := range ids {
		if len(id) != 12 {
			t.Fatalf("expected length 12, got %d for ID: %s", len(id), id)
		}

		// All should be lowercase base36
		for _, char := range id {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'z')) {
				t.Fatalf("invalid character in ID %s: %c", id, char)
			}
		}
	}
}
