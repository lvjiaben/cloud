// This file verifies the reviewed lifetime of management sensitive-access proofs.

package managementruntime

import (
	"testing"
	"time"
)

func TestSensitiveAccessProofTTLMatchesReviewedWindow(t *testing.T) {
	if sensitiveAccessProofTTL != 5*time.Minute {
		t.Fatalf("sensitive access proof TTL=%s, want=5m", sensitiveAccessProofTTL)
	}
}
