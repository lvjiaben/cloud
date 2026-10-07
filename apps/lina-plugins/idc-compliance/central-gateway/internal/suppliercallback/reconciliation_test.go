package suppliercallback

import (
	"fmt"
	"testing"
	"time"
)

func TestQueryBackoffIsBounded(t *testing.T) {
	wants := []time.Duration{
		time.Minute, 2 * time.Minute, 4 * time.Minute,
		8 * time.Minute, 16 * time.Minute, 30 * time.Minute,
		30 * time.Minute,
	}
	for index, want := range wants {
		if got := queryBackoff(index + 1); got != want {
			t.Fatalf("failure %d backoff=%s want=%s", index+1, got, want)
		}
	}
}

func TestTakeoverEvidenceBoundsAndReasons(t *testing.T) {
	for count := 1; count <= 16; count++ {
		values := make([]string, count)
		for index := range values {
			values[index] = fmt.Sprintf("lev_%032x", index+1)
		}
		if _, valid := canonicalEvidenceNos(values); !valid {
			t.Fatalf("evidence count %d was rejected", count)
		}
	}
	tooMany := make([]string, 17)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("lev_%032x", index+1)
	}
	if _, valid := canonicalEvidenceNos(tooMany); valid {
		t.Fatal("seventeen evidence records were accepted")
	}
	if _, valid := canonicalEvidenceNos([]string{
		"lev_00000000000000000000000000000001",
		"lev_00000000000000000000000000000001",
	}); valid {
		t.Fatal("duplicate evidence records were accepted")
	}
	for _, reason := range []TakeoverReason{
		TakeoverResultRemainsUnknown, TakeoverTerminalResultConflict,
		TakeoverQueryReferenceMissing, TakeoverSupplierQueryUnavailable,
		TakeoverComplianceSecurityReview,
	} {
		if !validTakeoverReason(reason) {
			t.Fatalf("fixed reason %q was rejected", reason)
		}
	}
	if validTakeoverReason("free_text") {
		t.Fatal("free-text reason was accepted")
	}
}
