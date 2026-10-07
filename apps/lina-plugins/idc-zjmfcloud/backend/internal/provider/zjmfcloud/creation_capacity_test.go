// This file verifies bounded creation-capacity projection helpers.

package zjmfcloud

import "testing"

func TestAppendUniqueCreationCandidateEnforcesGlobalLimitBeforeAppend(t *testing.T) {
	t.Parallel()
	candidates := make([]creationCandidate, maximumCreationCandidateCount)
	seen := make(map[string]struct{})
	candidate := creationCandidate{reference: "overflow", data: []byte(`{"store_id":1}`)}

	if appendUniqueCreationCandidate(&candidates, seen, candidate) {
		t.Fatal("candidate beyond the global limit was accepted")
	}
	if len(candidates) != maximumCreationCandidateCount {
		t.Fatalf("candidate count=%d, want %d", len(candidates), maximumCreationCandidateCount)
	}
	if _, exists := seen["overflow"]; exists {
		t.Fatal("rejected candidate polluted the deduplication set")
	}
	for _, value := range candidate.data {
		if value != 0 {
			t.Fatal("rejected candidate data was not cleared")
		}
	}
}
