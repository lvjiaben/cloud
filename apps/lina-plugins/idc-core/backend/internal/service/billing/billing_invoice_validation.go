// This file validates invoice lifecycle and allocation commands before any
// database access. It rejects ambiguous whitespace, unsafe identifiers, and
// caller-defined enum values.

package billing

import (
	"strings"
)

func normalizeInvoiceNo(invoiceNo string) (string, error) {
	if invoiceNo != strings.TrimSpace(invoiceNo) || !validBusinessReference(invoiceNo, 40) {
		return "", invoiceInvalidError()
	}
	return invoiceNo, nil
}

func normalizeInvoicePayment(input InvoicePaymentInput) (InvoicePaymentInput, error) {
	if !exactlyTrimmed(
		input.InvoiceNo, input.InvoiceItemNo, input.SourceReference, input.LedgerTransactionNo,
	) || !validBusinessReference(input.InvoiceNo, 40) ||
		(input.InvoiceItemNo != "" && !validBusinessReference(input.InvoiceItemNo, 40)) ||
		!validPaymentSource(input.SourceType) ||
		!validBusinessReference(input.SourceReference, 128) ||
		!validBusinessReference(input.LedgerTransactionNo, 40) || !validAmount(input.AmountMinor) {
		return InvoicePaymentInput{}, invoiceInvalidError()
	}
	return input, nil
}

func normalizeInvoiceBalancePayment(
	input InvoiceBalancePaymentInput,
) (InvoiceBalancePaymentInput, error) {
	if !exactlyTrimmed(input.InvoiceNo, input.SourceReference, input.SourceSnapshotDigest) ||
		!validBusinessReference(input.InvoiceNo, 40) ||
		!validBusinessReference(input.SourceReference, 128) ||
		!canonicalDigestPattern.MatchString(input.SourceSnapshotDigest) {
		return InvoiceBalancePaymentInput{}, invoiceInvalidError()
	}
	return input, nil
}

func normalizeInvoicePaymentReversal(
	input InvoicePaymentReversalInput,
) (InvoicePaymentReversalInput, error) {
	if !exactlyTrimmed(
		input.OriginalAllocationNo, input.SourceReference, input.LedgerTransactionNo,
	) || !validBusinessReference(input.OriginalAllocationNo, 40) ||
		!validBusinessReference(input.SourceReference, 128) ||
		!validBusinessReference(input.LedgerTransactionNo, 40) || !validAmount(input.AmountMinor) {
		return InvoicePaymentReversalInput{}, invoiceInvalidError()
	}
	return input, nil
}

func normalizeInvoiceRefund(input InvoiceRefundInput) (InvoiceRefundInput, error) {
	if !exactlyTrimmed(
		input.InvoiceNo, input.InvoiceItemNo, input.PaymentAllocationNo,
		input.SourceReference, input.LedgerTransactionNo,
	) || !validBusinessReference(input.InvoiceNo, 40) ||
		!validBusinessReference(input.InvoiceItemNo, 40) ||
		!validBusinessReference(input.PaymentAllocationNo, 40) ||
		!validRefundSource(input.SourceType) ||
		!validBusinessReference(input.SourceReference, 128) ||
		!validBusinessReference(input.LedgerTransactionNo, 40) || !validAmount(input.AmountMinor) {
		return InvoiceRefundInput{}, invoiceInvalidError()
	}
	return input, nil
}

func normalizeInvoiceRefundReversal(
	input InvoiceRefundReversalInput,
) (InvoiceRefundReversalInput, error) {
	if !exactlyTrimmed(
		input.OriginalRefundAllocationNo, input.SourceReference, input.LedgerTransactionNo,
	) || !validBusinessReference(input.OriginalRefundAllocationNo, 40) ||
		!validBusinessReference(input.SourceReference, 128) ||
		!validBusinessReference(input.LedgerTransactionNo, 40) || !validAmount(input.AmountMinor) {
		return InvoiceRefundReversalInput{}, invoiceInvalidError()
	}
	return input, nil
}

func normalizeInvoiceBadDebt(input InvoiceBadDebtInput) (InvoiceBadDebtInput, error) {
	if !exactlyTrimmed(input.InvoiceNo, input.LedgerTransactionNo) ||
		!validBusinessReference(input.InvoiceNo, 40) ||
		!validBusinessReference(input.LedgerTransactionNo, 40) {
		return InvoiceBadDebtInput{}, invoiceInvalidError()
	}
	return input, nil
}

func validPaymentSource(source PaymentSourceType) bool {
	return source == PaymentSourceChannel || source == PaymentSourceBalance ||
		source == PaymentSourceCredit || source == PaymentSourceManualReceipt ||
		source == PaymentSourceMigration
}

func validRefundSource(source RefundSourceType) bool {
	return source == RefundSourceChannel || source == RefundSourceBalanceCredit ||
		source == RefundSourceCreditRelease || source == RefundSourceManual ||
		source == RefundSourceMigration
}
