package resourcetask

import "testing"

func TestResourceTaskTransitionMatrix(t *testing.T) {
	accepted := TransitionEvidence{AcceptanceProven: true}
	rejected := TransitionEvidence{NonAcceptanceProven: true}
	observed := TransitionEvidence{ObservationProven: true}
	manual := TransitionEvidence{ManualAuthorized: true}
	tests := []struct {
		name     string
		from     Status
		to       Status
		evidence TransitionEvidence
		allowed  bool
	}{
		{name: "claim pending", from: StatusPending, to: StatusSubmitting, allowed: true},
		{name: "cancel before claim", from: StatusPending, to: StatusCanceled, allowed: true},
		{name: "invalid before claim", from: StatusPending, to: StatusFailed, evidence: rejected, allowed: true},
		{name: "claim retry", from: StatusRetryWait, to: StatusSubmitting, allowed: true},
		{name: "accepted write", from: StatusSubmitting, to: StatusAccepted, evidence: accepted, allowed: true},
		{name: "synchronous success", from: StatusSubmitting, to: StatusSucceeded, evidence: accepted, allowed: true},
		{name: "proven retry", from: StatusSubmitting, to: StatusRetryWait, evidence: rejected, allowed: true},
		{name: "proven failure", from: StatusSubmitting, to: StatusFailed, evidence: rejected, allowed: true},
		{name: "observed synchronous failure", from: StatusSubmitting, to: StatusFailed, evidence: observed, allowed: true},
		{name: "observed synchronous cancellation", from: StatusSubmitting, to: StatusCanceled, evidence: observed, allowed: true},
		{name: "ambiguous write", from: StatusSubmitting, to: StatusResultUnknown, allowed: true},
		{name: "observed running", from: StatusAccepted, to: StatusRunning, evidence: observed, allowed: true},
		{name: "observed success", from: StatusRunning, to: StatusSucceeded, evidence: observed, allowed: true},
		{name: "accepted query outage keeps known acceptance", from: StatusAccepted, to: StatusResultUnknown, allowed: false},
		{name: "accepted manual takeover", from: StatusAccepted, to: StatusManualTakeover, evidence: manual, allowed: true},
		{name: "unknown reconciled", from: StatusResultUnknown, to: StatusSucceeded, evidence: observed, allowed: true},
		{name: "unknown manual takeover", from: StatusResultUnknown, to: StatusManualTakeover, evidence: manual, allowed: true},
		{name: "retry without proof", from: StatusSubmitting, to: StatusRetryWait, allowed: false},
		{name: "failure without rejection", from: StatusSubmitting, to: StatusFailed, allowed: false},
		{name: "acceptance without proof", from: StatusSubmitting, to: StatusAccepted, allowed: false},
		{name: "unknown guessed success", from: StatusResultUnknown, to: StatusSucceeded, allowed: false},
		{name: "unknown retry", from: StatusResultUnknown, to: StatusRetryWait, evidence: rejected, allowed: false},
		{name: "claim with irrelevant proof", from: StatusPending, to: StatusSubmitting, evidence: accepted, allowed: false},
		{name: "unproven pre-claim failure", from: StatusPending, to: StatusFailed, allowed: false},
		{name: "ambiguous write with claimed acceptance", from: StatusSubmitting, to: StatusResultUnknown, evidence: accepted, allowed: false},
		{name: "running automatic retry", from: StatusRunning, to: StatusRetryWait, evidence: rejected, allowed: false},
		{name: "terminal rollback", from: StatusSucceeded, to: StatusRunning, evidence: observed, allowed: false},
		{name: "same state write", from: StatusRunning, to: StatusRunning, evidence: observed, allowed: false},
		{name: "multiple proof kinds", from: StatusSubmitting, to: StatusAccepted, evidence: TransitionEvidence{
			AcceptanceProven: true, NonAcceptanceProven: true,
		}, allowed: false},
		{name: "unknown status", from: Status("mystery"), to: StatusFailed, evidence: observed, allowed: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateTransition(test.from, test.to, test.evidence)
			if (err == nil) != test.allowed {
				t.Fatalf("transition %s -> %s allowed=%v want=%v err=%v", test.from, test.to, err == nil, test.allowed, err)
			}
		})
	}
}

func TestEveryResourceTaskStatusHasExplicitTerminalPolicy(t *testing.T) {
	statuses := []Status{
		StatusPending, StatusSubmitting, StatusAccepted, StatusRunning, StatusRetryWait,
		StatusSucceeded, StatusFailed, StatusResultUnknown, StatusManualTakeover, StatusCanceled,
	}
	terminal := map[Status]bool{
		StatusSucceeded: true, StatusFailed: true, StatusManualTakeover: true, StatusCanceled: true,
	}
	for _, status := range statuses {
		if !validStatus(status) {
			t.Fatalf("declared status %q is missing from validation", status)
		}
		if terminal[status] {
			for _, target := range statuses {
				if ValidateTransition(status, target, TransitionEvidence{
					AcceptanceProven: true, ObservationProven: true,
				}) == nil {
					t.Fatalf("terminal status %q transitioned to %q", status, target)
				}
			}
		}
	}
}
