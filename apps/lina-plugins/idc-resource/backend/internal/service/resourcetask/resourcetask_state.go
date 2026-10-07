// Package resourcetask owns durable Provider task state-transition rules.
package resourcetask

import "github.com/gogf/gf/v2/errors/gerror"

// Status is the resource-owner task state persisted in PostgreSQL.
type Status string

// Durable resource task states.
const (
	StatusPending        Status = "pending"
	StatusSubmitting     Status = "submitting"
	StatusAccepted       Status = "accepted"
	StatusRunning        Status = "running"
	StatusRetryWait      Status = "retry_wait"
	StatusSucceeded      Status = "succeeded"
	StatusFailed         Status = "failed"
	StatusResultUnknown  Status = "result_unknown"
	StatusManualTakeover Status = "manual_takeover"
	StatusCanceled       Status = "canceled"
)

// TransitionEvidence records only the proof category authorizing a state
// change. It never carries upstream text, payloads, credentials, or handles.
type TransitionEvidence struct {
	// AcceptanceProven means the upstream supplied validated acceptance identifiers.
	AcceptanceProven bool
	// NonAcceptanceProven means the write was conclusively rejected before acceptance.
	NonAcceptanceProven bool
	// ObservationProven means a validated task or resource query authoritatively resolved state.
	ObservationProven bool
	// ManualAuthorized means a separately authorized and audited operator accepted ownership.
	ManualAuthorized bool
}

// ValidateTransition fails closed for every transition not explicitly proven
// by the resource task matrix. Callers handle exact idempotent replays before
// invoking this function; same-state writes are not transitions.
func ValidateTransition(from Status, to Status, evidence TransitionEvidence) error {
	if !validStatus(from) || !validStatus(to) || from == to || multipleEvidenceKinds(evidence) {
		return gerror.New("resource task state transition is not permitted")
	}
	allowed := false
	switch from {
	case StatusPending:
		allowed = noEvidence(evidence) && (to == StatusSubmitting || to == StatusCanceled) ||
			to == StatusFailed && evidence.NonAcceptanceProven
	case StatusRetryWait:
		allowed = noEvidence(evidence) && (to == StatusSubmitting || to == StatusCanceled) ||
			to == StatusFailed && evidence.NonAcceptanceProven
	case StatusSubmitting:
		allowed = submittingTransitionAllowed(to, evidence)
	case StatusAccepted, StatusRunning:
		allowed = observedTransitionAllowed(to, evidence) ||
			(to == StatusManualTakeover && evidence.ManualAuthorized)
	case StatusResultUnknown:
		allowed = observedTransitionAllowed(to, evidence) ||
			(to == StatusManualTakeover && evidence.ManualAuthorized)
	case StatusSucceeded, StatusFailed, StatusManualTakeover, StatusCanceled:
		allowed = false
	}
	if !allowed {
		return gerror.New("resource task state transition is not permitted")
	}
	return nil
}

func submittingTransitionAllowed(to Status, evidence TransitionEvidence) bool {
	switch to {
	case StatusRetryWait:
		return evidence.NonAcceptanceProven
	case StatusFailed:
		return evidence.NonAcceptanceProven || evidence.ObservationProven
	case StatusCanceled:
		return evidence.ObservationProven
	case StatusAccepted, StatusRunning, StatusSucceeded:
		return evidence.AcceptanceProven || evidence.ObservationProven
	case StatusResultUnknown:
		return noEvidence(evidence)
	default:
		return false
	}
}

func observedTransitionAllowed(to Status, evidence TransitionEvidence) bool {
	if !evidence.ObservationProven {
		return false
	}
	switch to {
	case StatusAccepted, StatusRunning, StatusSucceeded, StatusFailed, StatusCanceled:
		return true
	default:
		return false
	}
}

func multipleEvidenceKinds(evidence TransitionEvidence) bool {
	count := 0
	for _, present := range []bool{
		evidence.AcceptanceProven,
		evidence.NonAcceptanceProven,
		evidence.ObservationProven,
		evidence.ManualAuthorized,
	} {
		if present {
			count++
		}
	}
	return count > 1
}

func noEvidence(evidence TransitionEvidence) bool {
	return !evidence.AcceptanceProven &&
		!evidence.NonAcceptanceProven &&
		!evidence.ObservationProven &&
		!evidence.ManualAuthorized
}

func validStatus(status Status) bool {
	switch status {
	case StatusPending,
		StatusSubmitting,
		StatusAccepted,
		StatusRunning,
		StatusRetryWait,
		StatusSucceeded,
		StatusFailed,
		StatusResultUnknown,
		StatusManualTakeover,
		StatusCanceled:
		return true
	default:
		return false
	}
}
