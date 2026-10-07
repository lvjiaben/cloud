// This file defines the bounded error metadata allowed across the Provider call boundary.

package resourcecap

import (
	"errors"

	"github.com/gogf/gf/v2/errors/gerror"
)

// ProviderCallErrorCode classifies a failed Provider call without carrying
// upstream response text, request material, or an underlying cause.
type ProviderCallErrorCode string

// Safe Provider call error codes.
const (
	ProviderCallErrorCodeAuthentication  ProviderCallErrorCode = "authentication"
	ProviderCallErrorCodeTransport       ProviderCallErrorCode = "transport"
	ProviderCallErrorCodeTimeout         ProviderCallErrorCode = "timeout"
	ProviderCallErrorCodeInvalidResponse ProviderCallErrorCode = "invalid_response"
	ProviderCallErrorCodeInternal        ProviderCallErrorCode = "internal"
	ProviderCallErrorCodeUnknown         ProviderCallErrorCode = "unknown"
)

// ProviderCallError carries only normalized retry and reconciliation policy.
// It deliberately has no message or cause field.
type ProviderCallError struct {
	// Code is one reviewed normalized classification.
	Code ProviderCallErrorCode
	// Retryable allows owner-policy retry only when the result is known.
	Retryable bool
	// ResultUnknown requires reconciliation before any repeated write.
	ResultUnknown bool
}

// NewProviderCallError constructs validated safe call-error metadata.
func NewProviderCallError(
	code ProviderCallErrorCode,
	retryable bool,
	resultUnknown bool,
) (*ProviderCallError, error) {
	callError := &ProviderCallError{Code: code, Retryable: retryable, ResultUnknown: resultUnknown}
	if err := callError.Validate(); err != nil {
		return nil, err
	}
	return callError, nil
}

// Error returns fixed text that cannot reflect upstream or secret material.
func (*ProviderCallError) Error() string {
	return "resource Provider call failed"
}

// Validate verifies the safe error classification and retry invariant.
func (e ProviderCallError) Validate() error {
	if err := validateMachineValue("call error code", string(e.Code)); err != nil {
		return err
	}
	switch e.Code {
	case ProviderCallErrorCodeAuthentication,
		ProviderCallErrorCodeTransport,
		ProviderCallErrorCodeTimeout,
		ProviderCallErrorCodeInvalidResponse,
		ProviderCallErrorCodeInternal:
		if e.ResultUnknown && e.Retryable {
			return gerror.New("resource Provider result-unknown call error cannot be retryable")
		}
		return nil
	case ProviderCallErrorCodeUnknown:
		if e.Retryable || !e.ResultUnknown {
			return gerror.New("resource Provider unknown call error must require non-retryable reconciliation")
		}
		return nil
	default:
		return gerror.New("resource Provider call error code is invalid")
	}
}

// NormalizeProviderError converts every raw or invalid error to a generic,
// non-retryable result-unknown classification. Callers must invoke it before
// logging, persistence, retry decisions, or returning across owner boundaries.
func NormalizeProviderError(err error) *ProviderCallError {
	if err == nil {
		return nil
	}
	var typed *ProviderCallError
	if errors.As(err, &typed) && typed != nil {
		candidate := *typed
		if candidate.Validate() == nil {
			return &candidate
		}
	}
	return &ProviderCallError{
		Code: ProviderCallErrorCodeUnknown, ResultUnknown: true,
	}
}
