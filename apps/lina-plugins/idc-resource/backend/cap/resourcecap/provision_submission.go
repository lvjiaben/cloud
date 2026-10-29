// This file defines the immutable evidence shape for resource-owner confirmed
// absence of a provisioning submission. Only the owner may establish this fact;
// a connector assertion or missing response cannot establish non-submission.
package resourcecap

import (
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// EvidenceKindRequestNotSent is reserved for trusted resource-owner observations.
const EvidenceKindRequestNotSent = "owner_provision_request_not_sent"

const provisionRequestNotSentSummary = "The resource owner confirmed no business HTTP request was attempted"

// NewProvisionNonSubmissionEvidence binds owner-confirmed non-submission to one
// exact task request. Callers must prove no business HTTP attempt in the entire
// invocation; this constructor does not make that determination for the caller.
func NewProvisionNonSubmissionEvidence(requestID string, observedAt time.Time) (Evidence, error) {
	value := Evidence{Kind: EvidenceKindRequestNotSent, Reference: requestID, Summary: provisionRequestNotSentSummary, ObservedAt: observedAt}
	if err := ValidateProvisionNonSubmissionEvidence(value, requestID); err != nil {
		return Evidence{}, err
	}
	return value, nil
}

// ValidateProvisionNonSubmissionEvidence checks the exact reserved category,
// fixed owner statement and task binding, rejecting fabricated or partial shapes.
// Origin must additionally be enforced by the owner before persistence.
func ValidateProvisionNonSubmissionEvidence(value Evidence, requestID string) error {
	if requestID == "" || value.Kind != EvidenceKindRequestNotSent || value.Reference != requestID || value.Summary != provisionRequestNotSentSummary || value.Validate() != nil {
		return gerror.New("resource owner non-submission evidence is invalid")
	}
	return nil
}
