// This file canonicalizes reviewed receipt, waiver, policy, and decision
// commands before database access. Raw idempotency keys are never persisted.

package billing

import (
	"regexp"
)

const (
	adjustmentTypeManualReceipt         = "manual_receipt"
	adjustmentTypeInvoiceWaiver         = "invoice_waiver"
	adjustmentTypeMarketingGrantPost    = "marketing_grant_post"
	adjustmentTypeMarketingGrantReverse = "marketing_grant_reverse"
)

var adjustmentReasonPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

type adjustmentCommand struct {
	AdjustmentType            string
	InvoiceNo                 string
	CreditStatementNo         string
	CustomerNo                string
	CurrencyCode              string
	AccountNo                 string
	AccountType               string
	AmountMinor               int64
	ReasonCode                string
	ProofReference            string
	ReceivingAccountReference string
	BusinessPurposeCode       string
	EvidenceDigest            string
	IdempotencyDigest         string
	RequestedBy               int
}

func normalizeMarketingGrantAdjustment(
	input MarketingGrantAdjustmentInput,
	adjustmentType string,
) (adjustmentCommand, error) {
	if !exactlyTrimmed(
		input.GrantNo, input.CustomerNo, input.CurrencyCode, input.ReasonCode,
		input.EvidenceDigest, input.IdempotencyKey,
	) || !validBusinessReference(input.GrantNo, 40) ||
		!validBusinessReference(input.CustomerNo, 40) ||
		!currencyPattern.MatchString(input.CurrencyCode) ||
		!validAmount(input.AmountMinor) || input.RequestedBy <= 0 ||
		!adjustmentReasonPattern.MatchString(input.ReasonCode) ||
		!canonicalDigestPattern.MatchString(input.EvidenceDigest) ||
		!validInvoiceTransformKey(input.IdempotencyKey) ||
		(adjustmentType != adjustmentTypeMarketingGrantPost &&
			adjustmentType != adjustmentTypeMarketingGrantReverse) {
		return adjustmentCommand{}, invoiceInvalidError()
	}
	return adjustmentCommand{
		AdjustmentType: adjustmentType,
		CustomerNo:     input.CustomerNo, CurrencyCode: input.CurrencyCode,
		AccountType: AccountTypeCustomerGift, AmountMinor: input.AmountMinor,
		ReasonCode: input.ReasonCode, ProofReference: input.GrantNo,
		EvidenceDigest:    input.EvidenceDigest,
		IdempotencyDigest: invoiceTransformIdempotencyDigest(input.IdempotencyKey),
		RequestedBy:       input.RequestedBy,
	}, nil
}

type adjustmentPolicySnapshot struct {
	PolicyNo             string
	Version              int64
	ReviewThresholdMinor int64
	ApprovalRequired     bool
}

func normalizeManualReceipt(input ManualReceiptInput) (adjustmentCommand, error) {
	if !exactlyTrimmed(
		input.InvoiceNo, input.CreditStatementNo, input.CustomerNo,
		input.CurrencyCode, input.ReasonCode,
		input.ProofReference, input.ReceivingAccountReference, input.BusinessPurposeCode,
		input.EvidenceDigest, input.IdempotencyKey,
	) || !validAmount(input.AmountMinor) || input.RequestedBy <= 0 ||
		!adjustmentReasonPattern.MatchString(input.ReasonCode) ||
		!validBusinessReference(input.ProofReference, 128) ||
		!validBusinessReference(input.ReceivingAccountReference, 128) ||
		!adjustmentReasonPattern.MatchString(input.BusinessPurposeCode) ||
		!canonicalDigestPattern.MatchString(input.EvidenceDigest) ||
		!validInvoiceTransformKey(input.IdempotencyKey) {
		return adjustmentCommand{}, invoiceInvalidError()
	}
	invoiceTarget := input.InvoiceNo != ""
	creditTarget := input.CreditStatementNo != ""
	balanceTarget := input.CustomerNo != "" || input.CurrencyCode != ""
	if boolCount(invoiceTarget, creditTarget, balanceTarget) != 1 {
		return adjustmentCommand{}, invoiceInvalidError()
	}
	if invoiceTarget {
		if !validBusinessReference(input.InvoiceNo, 40) || input.CustomerNo != "" ||
			input.CurrencyCode != "" || input.CreditStatementNo != "" {
			return adjustmentCommand{}, invoiceInvalidError()
		}
	} else if creditTarget {
		if !validBusinessReference(input.CreditStatementNo, 40) ||
			input.CustomerNo != "" || input.CurrencyCode != "" {
			return adjustmentCommand{}, invoiceInvalidError()
		}
	} else if !validBusinessReference(input.CustomerNo, 40) ||
		!currencyPattern.MatchString(input.CurrencyCode) {
		return adjustmentCommand{}, invoiceInvalidError()
	}
	return adjustmentCommand{
		AdjustmentType: adjustmentTypeManualReceipt, InvoiceNo: input.InvoiceNo,
		CreditStatementNo: input.CreditStatementNo,
		CustomerNo:        input.CustomerNo, CurrencyCode: input.CurrencyCode,
		AmountMinor: input.AmountMinor, ReasonCode: input.ReasonCode,
		ProofReference:            input.ProofReference,
		ReceivingAccountReference: input.ReceivingAccountReference,
		BusinessPurposeCode:       input.BusinessPurposeCode,
		EvidenceDigest:            input.EvidenceDigest,
		IdempotencyDigest:         invoiceTransformIdempotencyDigest(input.IdempotencyKey),
		RequestedBy:               input.RequestedBy,
	}, nil
}

func boolCount(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}

func normalizeInvoiceWaiver(input InvoiceWaiverInput) (adjustmentCommand, error) {
	if !exactlyTrimmed(
		input.InvoiceNo, input.ReasonCode, input.EvidenceDigest, input.IdempotencyKey,
	) || !validBusinessReference(input.InvoiceNo, 40) || !validAmount(input.AmountMinor) ||
		!adjustmentReasonPattern.MatchString(input.ReasonCode) ||
		!canonicalDigestPattern.MatchString(input.EvidenceDigest) ||
		!validInvoiceTransformKey(input.IdempotencyKey) || input.RequestedBy <= 0 {
		return adjustmentCommand{}, invoiceInvalidError()
	}
	return adjustmentCommand{
		AdjustmentType: adjustmentTypeInvoiceWaiver, InvoiceNo: input.InvoiceNo,
		AmountMinor: input.AmountMinor, ReasonCode: input.ReasonCode,
		EvidenceDigest:    input.EvidenceDigest,
		IdempotencyDigest: invoiceTransformIdempotencyDigest(input.IdempotencyKey),
		RequestedBy:       input.RequestedBy,
	}, nil
}

func normalizeAdjustmentPolicy(input AdjustmentPolicyInput) (AdjustmentPolicyInput, error) {
	if !exactlyTrimmed(input.CurrencyCode) ||
		!currencyPattern.MatchString(input.CurrencyCode) ||
		!validAmount(input.ReviewThresholdMinor) || input.ConfiguredBy <= 0 {
		return AdjustmentPolicyInput{}, invoiceInvalidError()
	}
	return input, nil
}

func normalizeAdjustmentReview(input AdjustmentReviewInput) (AdjustmentReviewInput, error) {
	if !exactlyTrimmed(
		input.AdjustmentNo, input.CommentDigest, input.EvidenceDigest,
	) || !validBusinessReference(input.AdjustmentNo, 40) || input.ReviewerID <= 0 ||
		(input.Decision != AdjustmentDecisionApprove && input.Decision != AdjustmentDecisionReject) ||
		!canonicalDigestPattern.MatchString(input.CommentDigest) ||
		!canonicalDigestPattern.MatchString(input.EvidenceDigest) {
		return AdjustmentReviewInput{}, invoiceInvalidError()
	}
	return input, nil
}

func adjustmentRequestDigest(command adjustmentCommand) (string, error) {
	return contentDigest(struct {
		AdjustmentType            string `json:"adjustmentType"`
		InvoiceNo                 string `json:"invoiceNo,omitempty"`
		CreditStatementNo         string `json:"creditStatementNo,omitempty"`
		CustomerNo                string `json:"customerNo"`
		CurrencyCode              string `json:"currencyCode"`
		AccountNo                 string `json:"accountNo,omitempty"`
		AccountType               string `json:"accountType,omitempty"`
		AmountMinor               int64  `json:"amountMinor"`
		ReasonCode                string `json:"reasonCode"`
		ProofReference            string `json:"proofReference,omitempty"`
		ReceivingAccountReference string `json:"receivingAccountReference,omitempty"`
		BusinessPurposeCode       string `json:"businessPurposeCode,omitempty"`
		EvidenceDigest            string `json:"evidenceDigest"`
		RequestedBy               int    `json:"requestedBy"`
	}{
		command.AdjustmentType, command.InvoiceNo, command.CreditStatementNo,
		command.CustomerNo,
		command.CurrencyCode, command.AccountNo, command.AccountType, command.AmountMinor,
		command.ReasonCode, command.ProofReference,
		command.ReceivingAccountReference, command.BusinessPurposeCode,
		command.EvidenceDigest, command.RequestedBy,
	})
}
