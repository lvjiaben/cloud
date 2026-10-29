// These tests keep the reserved non-submission proof exact and task-bound.
package resourcecap

import (
	"testing"
	"time"
)

func TestProvisionNonSubmissionEvidenceRequiresExactTaskAndStatement(t *testing.T) {
	proof, err := NewProvisionNonSubmissionEvidence("task_owner_1", time.Now().UTC())
	if err != nil || ValidateProvisionNonSubmissionEvidence(proof, "task_owner_1") != nil {
		t.Fatal("valid proof rejected")
	}
	if ValidateProvisionNonSubmissionEvidence(proof, "task_owner_2") == nil {
		t.Fatal("another task reused proof")
	}
	for _, mutate := range []func(*Evidence){
		func(p *Evidence) { p.Reference = "" }, func(p *Evidence) { p.Kind = EvidenceKindCallDiagnostic }, func(p *Evidence) { p.Summary = "no response" }, func(p *Evidence) { p.ObservedAt = time.Time{} },
	} {
		changed := proof
		mutate(&changed)
		if ValidateProvisionNonSubmissionEvidence(changed, "task_owner_1") == nil {
			t.Fatal("partial or contradictory proof accepted")
		}
	}
}
