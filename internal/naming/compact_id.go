package naming

import (
	"crypto/rand"
	"fmt"
	"strconv"
	"time"
)

// NewCompactID returns a time-ordered compact ID (12 chars, base36) for resource naming.
// Format: 7-char timestamp (base36) + 5-char random (base36)
// Supports timestamps until year ~4454 with second-level ordering.
// Returns lowercase characters only.
func NewCompactID() (string, error) {
	// Get current Unix timestamp (seconds since epoch)
	timestamp := time.Now().UTC().Unix()

	// Convert timestamp to base36 and ensure it fits in 7 chars
	// Max timestamp for 7 chars: 36^7-1 = 78,364,164,095 (supports until year ~4454)
	if timestamp < 0 {
		return "", fmt.Errorf("negative timestamp not supported")
	}
	if timestamp >= 78364164096 { // 36^7
		return "", fmt.Errorf("timestamp too large for 7-char base36 encoding")
	}

	// Convert to base36 and pad to exactly 7 characters
	timeStr := strconv.FormatInt(timestamp, 36)
	timeStr = fmt.Sprintf("%07s", timeStr) // Zero-pad to 7 chars

	// Generate 5 random characters in base36
	randomBytes := make([]byte, 3) // 3 bytes gives us enough entropy for 5 base36 chars
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	// Convert random bytes to base36 (ensure it fits in 5 chars)
	var randomInt uint64
	for _, b := range randomBytes {
		randomInt = randomInt*256 + uint64(b)
	}
	randomInt = randomInt % (36 * 36 * 36 * 36 * 36) // Ensure 5 chars max (36^5 = 60,466,176)

	randomStr := strconv.FormatUint(randomInt, 36)
	randomStr = fmt.Sprintf("%05s", randomStr) // Zero-pad to 5 chars

	return timeStr + randomStr, nil
}
