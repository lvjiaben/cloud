// This file defines and validates the versioned refund execution capability owned by billing.

package billingcap

import (
	"context"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// RefundExecutionPhase separates source preflight from the external refund write.
type RefundExecutionPhase string

// RefundExecutionResultStatus is the closed result set accepted from an execution owner.
type RefundExecutionResultStatus string

// Refund execution constants form the versioned phase and result vocabulary.
const (
	RefundExecutionPhasePreflight RefundExecutionPhase = "preflight"
	RefundExecutionPhaseExecute   RefundExecutionPhase = "execution"

	RefundExecutionResultReady              RefundExecutionResultStatus = "ready"
	RefundExecutionResultSucceeded          RefundExecutionResultStatus = "succeeded"
	RefundExecutionResultFailed             RefundExecutionResultStatus = "failed"
	RefundExecutionResultUnknown            RefundExecutionResultStatus = "result_unknown"
	RefundExecutionResultManualConfirmation RefundExecutionResultStatus = "manual_confirmation"
)

var refundResultCodePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,47}$`)

// ClaimRefundExecution requests one bounded durable command lease.
type ClaimRefundExecution struct {
	WorkerNo     string `json:"workerNo"`
	LeaseSeconds int    `json:"leaseSeconds"`
}

// RefundExecutionWork is one leased source command with immutable request evidence.
type RefundExecutionWork struct {
	HasWork          bool                 `json:"hasWork"`
	CommandNo        string               `json:"commandNo,omitempty"`
	RefundNo         string               `json:"refundNo,omitempty"`
	RefundSourceNo   string               `json:"refundSourceNo,omitempty"`
	PaymentAttemptNo string               `json:"paymentAttemptNo,omitempty"`
	CurrencyCode     string               `json:"currencyCode,omitempty"`
	AmountMinor      int64                `json:"amountMinor,omitempty"`
	RequestDigest    string               `json:"requestDigest,omitempty"`
	Phase            RefundExecutionPhase `json:"phase,omitempty"`
	Attempt          int                  `json:"attempt,omitempty"`
	LeaseExpiresAt   time.Time            `json:"leaseExpiresAt,omitempty"`
}

// ReportRefundExecution reports one exact preflight or execution outcome.
type ReportRefundExecution struct {
	WorkerNo        string                      `json:"workerNo"`
	CommandNo       string                      `json:"commandNo"`
	RequestDigest   string                      `json:"requestDigest"`
	Phase           RefundExecutionPhase        `json:"phase"`
	Status          RefundExecutionResultStatus `json:"status"`
	ResultCode      string                      `json:"resultCode"`
	ResultReference string                      `json:"resultReference,omitempty"`
	EvidenceDigest  string                      `json:"evidenceDigest"`
}

// RefundExecutionReport returns the committed command and aggregate refund states.
type RefundExecutionReport struct {
	CommandNo         string `json:"commandNo"`
	RefundNo          string `json:"refundNo"`
	CommandStatusCode string `json:"commandStatusCode"`
	RefundStatusCode  string `json:"refundStatusCode"`
	Reused            bool   `json:"reused"`
}

// RefundExecutionService is the billing-owned versioned execution capability.
type RefundExecutionService interface {
	// ClaimRefundExecution leases one bounded durable refund command for an execution owner.
	ClaimRefundExecution(context.Context, ClaimRefundExecution) (*RefundExecutionWork, error)
	// ReportRefundExecution commits one preflight or execution result with immutable evidence.
	ReportRefundExecution(context.Context, ReportRefundExecution) (*RefundExecutionReport, error)
}

// ValidateClaimRefundExecution rejects malformed or unbounded lease requests.
func ValidateClaimRefundExecution(input ClaimRefundExecution) error {
	if !businessReferencePattern.MatchString(input.WorkerNo) ||
		input.LeaseSeconds < 5 || input.LeaseSeconds > 300 {
		return gerror.New("billingcap: refund execution claim is invalid")
	}
	return nil
}

// ValidateRefundExecutionWork validates an empty or fully populated leased command.
func ValidateRefundExecutionWork(value RefundExecutionWork) error {
	if !value.HasWork {
		if value.CommandNo != "" || value.RefundNo != "" || value.RefundSourceNo != "" ||
			value.PaymentAttemptNo != "" || value.CurrencyCode != "" || value.AmountMinor != 0 ||
			value.RequestDigest != "" || value.Phase != "" || value.Attempt != 0 ||
			!value.LeaseExpiresAt.IsZero() {
			return gerror.New("billingcap: empty refund execution work is invalid")
		}
		return nil
	}
	if !businessReferencePattern.MatchString(value.CommandNo) ||
		!businessReferencePattern.MatchString(value.RefundNo) ||
		!businessReferencePattern.MatchString(value.RefundSourceNo) ||
		!businessReferencePattern.MatchString(value.PaymentAttemptNo) ||
		!currencyCodePattern.MatchString(value.CurrencyCode) || value.AmountMinor <= 0 ||
		!digestPattern.MatchString(value.RequestDigest) ||
		(value.Phase != RefundExecutionPhasePreflight && value.Phase != RefundExecutionPhaseExecute) ||
		value.Attempt < 1 || value.LeaseExpiresAt.IsZero() {
		return gerror.New("billingcap: refund execution work is invalid")
	}
	return nil
}

// ValidateReportRefundExecution validates phase-specific result status and evidence.
func ValidateReportRefundExecution(input ReportRefundExecution) error {
	if !businessReferencePattern.MatchString(input.WorkerNo) ||
		!businessReferencePattern.MatchString(input.CommandNo) ||
		!digestPattern.MatchString(input.RequestDigest) ||
		!refundResultCodePattern.MatchString(input.ResultCode) ||
		!digestPattern.MatchString(input.EvidenceDigest) || len(input.ResultReference) > 128 ||
		(input.Phase != RefundExecutionPhasePreflight && input.Phase != RefundExecutionPhaseExecute) {
		return gerror.New("billingcap: refund execution report is invalid")
	}
	if input.Phase == RefundExecutionPhasePreflight {
		if input.Status != RefundExecutionResultReady && input.Status != RefundExecutionResultFailed &&
			input.Status != RefundExecutionResultManualConfirmation {
			return gerror.New("billingcap: refund preflight result is invalid")
		}
		if (input.Status == RefundExecutionResultReady && input.ResultCode != "preflight_ready") ||
			(input.Status == RefundExecutionResultFailed && input.ResultCode != "original_channel_unsupported") ||
			(input.Status == RefundExecutionResultManualConfirmation && input.ResultCode != "payment_context_mismatch") {
			return gerror.New("billingcap: refund preflight code is invalid")
		}
	} else if input.Status != RefundExecutionResultSucceeded &&
		input.Status != RefundExecutionResultFailed && input.Status != RefundExecutionResultUnknown &&
		input.Status != RefundExecutionResultManualConfirmation {
		return gerror.New("billingcap: refund execution result is invalid")
	}
	return nil
}

// ValidateRefundExecutionReport validates the billing owner's committed result projection.
func ValidateRefundExecutionReport(value RefundExecutionReport) error {
	if !businessReferencePattern.MatchString(value.CommandNo) ||
		!businessReferencePattern.MatchString(value.RefundNo) || value.CommandStatusCode == "" ||
		value.RefundStatusCode == "" {
		return gerror.New("billingcap: refund execution report result is invalid")
	}
	return nil
}
