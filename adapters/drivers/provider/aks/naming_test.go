package aks

import (
	"strings"
	"testing"
)

func TestSafeTruncate(t *testing.T) {
	tests := []struct {
		name        string
		base        string
		hash        string
		expected    string
		expectError bool
	}{
		{
			name:     "base and hash fit within limit",
			base:     "short",
			hash:     "abc123",
			expected: "short_abc123",
		},
		{
			name:     "base needs truncation",
			base:     strings.Repeat("a", 70),             // 70 chars
			hash:     "abc123",                            // 6 chars
			expected: strings.Repeat("a", 65) + "_abc123", // 65 + 1 + 6 = 72 chars (maxResourceName)
		},
		{
			name:     "base is exactly at max length after truncation",
			base:     strings.Repeat("b", 70),             // 70 chars
			hash:     "def456",                            // 6 chars
			expected: strings.Repeat("b", 65) + "_def456", // 65 + 1 + 6 = 72 chars
		},
		{
			name:     "empty base",
			base:     "",
			hash:     "xyz789",
			expected: "_xyz789",
		},
		{
			name:     "base with one character",
			base:     "x",
			hash:     "hash123",
			expected: "x_hash123",
		},
		{
			name:     "very long hash",
			base:     "test",
			hash:     strings.Repeat("h", 50),                // 50 chars
			expected: "test" + "_" + strings.Repeat("h", 50), // Should not truncate hash
		},
		{
			name:     "base needs to be truncated to minimum length",
			base:     strings.Repeat("c", 100),           // Very long base
			hash:     "short",                            // Short hash
			expected: strings.Repeat("c", 66) + "_short", // Base truncated to fit within maxResourceName (72)
		},
		{
			name:        "hash too long",
			base:        "test",
			hash:        strings.Repeat("x", 72), // Hash exactly at maxResourceName
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := safeTruncate(tt.base, tt.hash)

			if tt.expectError {
				if err == nil {
					t.Errorf("safeTruncate(%q, %q) expected error but got none", tt.base, tt.hash)
				}
				return
			}

			if err != nil {
				t.Errorf("safeTruncate(%q, %q) unexpected error: %v", tt.base, tt.hash, err)
				return
			}

			// Verify the result matches expected
			if result != tt.expected {
				t.Errorf("safeTruncate(%q, %q) = %q, expected %q",
					tt.base, tt.hash, result, tt.expected)
			}

			// Verify the result doesn't exceed maxResourceName (72) unless hash is too long
			if len(result) > maxResourceName && len(tt.hash)+1 <= maxResourceName {
				t.Errorf("safeTruncate(%q, %q) result length %d exceeds maxResourceName %d",
					tt.base, tt.hash, len(result), maxResourceName)
			}

			// Verify the hash is preserved at the end
			if !strings.HasSuffix(result, "_"+tt.hash) {
				t.Errorf("safeTruncate(%q, %q) = %q, hash suffix not preserved",
					tt.base, tt.hash, result)
			}
		})
	}
}

func TestSafeTruncateEdgeCases(t *testing.T) {
	t.Run("hash longer than maxResourceName", func(t *testing.T) {
		base := "test"
		hash := strings.Repeat("h", 73) // Hash longer than maxResourceName
		_, err := safeTruncate(base, hash)

		// The function should return an error when hash is too long
		if err == nil {
			t.Errorf("Expected error for hash too long, but got none")
		}
	})

	t.Run("exactly at boundary", func(t *testing.T) {
		// Create base and hash that together equal exactly maxResourceName
		hash := "123456"                                         // 6 chars
		base := strings.Repeat("a", maxResourceName-len(hash)-1) // 72 - 6 - 1 = 65 chars
		expected := base + "_" + hash                            // Should be exactly 72 chars

		result, err := safeTruncate(base, hash)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
			return
		}

		if result != expected {
			t.Errorf("Expected %q (len=%d), got %q (len=%d)",
				expected, len(expected), result, len(result))
		}

		if len(result) != maxResourceName {
			t.Errorf("Expected result length to be exactly %d, got %d",
				maxResourceName, len(result))
		}
	})
}
