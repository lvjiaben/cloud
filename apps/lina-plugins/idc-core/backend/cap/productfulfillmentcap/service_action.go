// This file defines the uniform JSON envelope used for governed actions on existing long-lived entitlements.

package productfulfillmentcap

import (
	"encoding/json"
	"regexp"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	// ManagedServiceActionVersion is the first owner-governed action protocol.
	ManagedServiceActionVersion = "v1"
	// ManagedServiceActionStatusProjected reports a safe read-only projection.
	ManagedServiceActionStatusProjected = "projected"
	// ManagedServiceActionStatusSucceeded reports one confirmed final mutation.
	ManagedServiceActionStatusSucceeded = "succeeded"
	// ManagedServiceActionStatusResultUnknown prevents blind mutation replay.
	ManagedServiceActionStatusResultUnknown     = "result_unknown"
	ManagedServiceQueryDecisionSubmitAllowed    = "submit_allowed"
	ManagedServiceQueryDecisionAlreadySucceeded = "already_succeeded"
	ManagedServiceQueryDecisionForbidden        = "forbidden"
)

var managedServiceReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,255}$`)

// ManagedServiceActionRequest carries frozen entitlement evidence without customer identity or credentials.
type ManagedServiceActionRequest struct {
	Version              string          `json:"version"`
	ActionKey            string          `json:"actionKey"`
	EntitlementNo        string          `json:"entitlementNo"`
	IdempotencyKey       string          `json:"idempotencyKey,omitempty"`
	ExternalReference    string          `json:"externalReference"`
	ProductConfiguration json.RawMessage `json:"productConfiguration"`
	Input                json.RawMessage `json:"input,omitempty"`
	RequestedActionKey   string          `json:"requestedActionKey,omitempty"`
}

// ManagedServiceActionResult is a bounded JSON projection; binary content is never an envelope member.
type ManagedServiceActionResult struct {
	Version           string          `json:"version"`
	ActionKey         string          `json:"actionKey"`
	Status            string          `json:"status"`
	ExternalReference string          `json:"externalReference"`
	Projection        json.RawMessage `json:"projection,omitempty"`
	Decision          string          `json:"decision,omitempty"`
}

// ValidateManagedServiceActionRequest validates the uniform action envelope before provider dispatch.
func ValidateManagedServiceActionRequest(request *ManagedServiceActionRequest, expectedAction string, mutation bool) error {
	if request == nil || request.Version != ManagedServiceActionVersion || request.ActionKey != expectedAction ||
		!managedServiceActionKeyPattern.MatchString(request.ActionKey) ||
		!managedServiceReferencePattern.MatchString(request.EntitlementNo) ||
		!managedServiceReferencePattern.MatchString(request.ExternalReference) ||
		!validJSONObject(request.ProductConfiguration) ||
		(len(request.Input) > 0 && !validJSONObject(request.Input)) {
		return gerror.New("managed service action request is invalid")
	}
	if mutation != managedServiceReferencePattern.MatchString(request.IdempotencyKey) {
		return gerror.New("managed service action idempotency evidence is invalid")
	}
	if request.RequestedActionKey != "" && (!managedServiceActionKeyPattern.MatchString(request.RequestedActionKey) || mutation) {
		return gerror.New("managed service action query target is invalid")
	}
	return nil
}

// ValidateManagedServiceActionResult rejects binary data, unknown status, and mismatched action evidence.
func ValidateManagedServiceActionResult(result *ManagedServiceActionResult, expectedAction string, projection bool) error {
	if result == nil || result.Version != ManagedServiceActionVersion || result.ActionKey != expectedAction ||
		!managedServiceReferencePattern.MatchString(result.ExternalReference) || len(result.Projection) > MaximumEvidenceBytes ||
		(len(result.Projection) > 0 && !validJSONObject(result.Projection)) {
		return gerror.New("managed service action result is invalid")
	}
	if projection {
		if result.Status != ManagedServiceActionStatusProjected || len(result.Projection) == 0 {
			return gerror.New("managed service action projection is invalid")
		}
		if result.Decision != "" && result.Decision != ManagedServiceQueryDecisionSubmitAllowed && result.Decision != ManagedServiceQueryDecisionAlreadySucceeded && result.Decision != ManagedServiceQueryDecisionForbidden {
			return gerror.New("managed service action query decision is invalid")
		}
		return nil
	}
	if result.Status != ManagedServiceActionStatusSucceeded && result.Status != ManagedServiceActionStatusResultUnknown {
		return gerror.New("managed service action mutation result is invalid")
	}
	return nil
}
