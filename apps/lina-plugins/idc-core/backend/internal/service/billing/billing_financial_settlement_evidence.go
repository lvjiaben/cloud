// This file validates bounded settlement-source evidence and persists its
// immutable header/source conservation chain.
package billing

import (
	"context"
	"sort"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// preparedFinancialSettlementSource is one validated source before immutable
// persistence and public projection.
type preparedFinancialSettlementSource struct {
	settlementSourceNo  string
	sourceType          FinancialSettlementSourceType
	sourceReference     string
	paymentAllocationNo string
	creditExposureNo    string
	amountMinor         int64
	evidenceDigest      string
	contentDigest       string
}

// preparedFinancialSettlement is the transaction-local canonical fact used to
// derive all source and header digests.
type preparedFinancialSettlement struct {
	settlementNo           string
	orderNo                string
	invoiceNo              string
	customerNo             string
	currencyCode           string
	basis                  FinancialSettlementBasis
	totalAmountMinor       int64
	paidAmountMinor        int64
	creditAmountMinor      int64
	identityEvidenceDigest string
	sourceSnapshotDigest   string
	contentDigest          string
	settledAt              time.Time
	sources                []preparedFinancialSettlementSource
}

// settlementAllocationEvidenceRow is the bounded set-based projection of a
// payment allocation, ledger transaction, entries, and optional manual review.
type settlementAllocationEvidenceRow struct {
	ID                           int64      `orm:"id"`
	AllocationNo                 string     `orm:"allocation_no"`
	InvoiceNo                    string     `orm:"invoice_no"`
	CustomerNo                   string     `orm:"customer_no"`
	CurrencyCode                 string     `orm:"currency_code"`
	SourceType                   string     `orm:"payment_source_type_code"`
	SourceReference              string     `orm:"payment_source_reference"`
	LedgerTransactionNo          string     `orm:"ledger_transaction_no"`
	LedgerTransactionType        string     `orm:"ledger_transaction_type_code"`
	Direction                    string     `orm:"direction_code"`
	OriginalAllocationNo         *string    `orm:"original_allocation_no"`
	AmountMinor                  int64      `orm:"amount_minor"`
	AllocationDigest             string     `orm:"allocation_digest"`
	TransactionSourceType        string     `orm:"transaction_source_type"`
	TransactionSourceReference   string     `orm:"transaction_source_reference"`
	TransactionSnapshotDigest    string     `orm:"transaction_snapshot_digest"`
	TransactionContentDigest     string     `orm:"transaction_content_digest"`
	TransactionOriginalNo        *string    `orm:"transaction_original_no"`
	TransactionEntryCount        int        `orm:"transaction_entry_count"`
	TransactionDebitMinor        int64      `orm:"transaction_debit_minor"`
	TransactionCreditMinor       int64      `orm:"transaction_credit_minor"`
	LedgerReversalCount          int64      `orm:"ledger_reversal_count"`
	BalanceDebitMinor            int64      `orm:"balance_debit_minor"`
	RevenueCreditMinor           int64      `orm:"revenue_credit_minor"`
	CashDebitMinor               int64      `orm:"cash_debit_minor"`
	ReceivableCreditMinor        int64      `orm:"receivable_credit_minor"`
	AdjustmentNo                 *string    `orm:"adjustment_no"`
	AdjustmentType               *string    `orm:"adjustment_type_code"`
	AdjustmentStatus             *string    `orm:"adjustment_status_code"`
	AdjustmentInvoiceNo          *string    `orm:"adjustment_invoice_no"`
	AdjustmentCustomerNo         *string    `orm:"adjustment_customer_no"`
	AdjustmentCurrencyCode       *string    `orm:"adjustment_currency_code"`
	AdjustmentAmountMinor        *int64     `orm:"adjustment_amount_minor"`
	AdjustmentEvidenceDigest     *string    `orm:"adjustment_evidence_digest"`
	AdjustmentRequestDigest      *string    `orm:"adjustment_request_digest"`
	AdjustmentApprovalRequired   *bool      `orm:"adjustment_approval_required"`
	AdjustmentRequestedBy        *int       `orm:"adjustment_requested_by"`
	AdjustmentLedgerTransaction  *string    `orm:"adjustment_ledger_transaction_no"`
	AdjustmentProofReference     *string    `orm:"adjustment_proof_reference"`
	AdjustmentReceivingReference *string    `orm:"adjustment_receiving_reference"`
	AdjustmentBusinessPurpose    *string    `orm:"adjustment_business_purpose"`
	AdjustmentPostedAt           *time.Time `orm:"adjustment_posted_at"`
	AdjustmentApprovalCount      int64      `orm:"adjustment_approval_count"`
}

// prepareFinancialSettlement selects one mutually exclusive basis and fully
// validates all local source evidence before any write.
func prepareFinancialSettlement(
	ctx context.Context,
	database gdb.DB,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	items settlementOrderItemSummary,
	identityDigest string,
	now time.Time,
) (preparedFinancialSettlement, error) {
	if items.TrialCount > 0 {
		return prepareTrialFinancialSettlement(
			ctx, database, order, invoice, items, identityDigest, now,
		)
	}
	prepared := preparedFinancialSettlement{
		settlementNo: deterministicFinancialSettlementNo(order.OrderNo),
		orderNo:      order.OrderNo, invoiceNo: invoice.InvoiceNo,
		customerNo: order.CustomerNo, currencyCode: order.CurrencyCode,
		totalAmountMinor:       order.TotalAmountMinor,
		identityEvidenceDigest: identityDigest,
	}
	if order.TotalAmountMinor == 0 {
		basis, err := prepareSyntheticFinancialSettlement(
			ctx, database, order, invoice, now,
		)
		if err != nil {
			return preparedFinancialSettlement{}, err
		}
		prepared.basis = basis
		prepared.settledAt = now
		prepared.sources = []preparedFinancialSettlementSource{{
			sourceType: FinancialSettlementSourceType(basis), sourceReference: order.OrderNo,
		}}
	} else {
		if invoice.StatusCode != string(InvoiceStatusPaid) || invoice.SettledAt == nil ||
			invoice.PaidAmountMinor != invoice.TotalAmountMinor ||
			invoice.WaivedAmountMinor != 0 || invoice.CreditAmountMinor != 0 ||
			invoice.RefundedAmountMinor != 0 {
			return preparedFinancialSettlement{}, bizerr.NewCode(CodeSettlementNotEligible)
		}
		sources, paidMinor, err := loadFundedFinancialSettlementSources(
			ctx, database, invoice,
		)
		if err != nil {
			return preparedFinancialSettlement{}, err
		}
		if paidMinor != order.TotalAmountMinor || len(sources) == 0 {
			return preparedFinancialSettlement{}, bizerr.NewCode(CodeSettlementIntegrity)
		}
		prepared.basis = SettlementBasisFundedInvoice
		prepared.paidAmountMinor = paidMinor
		prepared.settledAt = *invoice.SettledAt
		prepared.sources = sources
	}
	if len(prepared.sources) < 1 || len(prepared.sources) > maximumFinancialSettlementSources {
		return preparedFinancialSettlement{}, bizerr.NewCode(CodeSettlementIntegrity)
	}
	if err := finalizePreparedFinancialSettlement(&prepared); err != nil {
		return preparedFinancialSettlement{}, err
	}
	return prepared, nil
}

// prepareSyntheticFinancialSettlement distinguishes a legal zero price from a
// full discount without creating a fake payment transaction.
func prepareSyntheticFinancialSettlement(
	ctx context.Context,
	database gdb.DB,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	now time.Time,
) (FinancialSettlementBasis, error) {
	if order.StatusCode != "pending_payment" || order.PaymentDueAt == nil ||
		!order.PaymentDueAt.After(now) || invoice.StatusCode != string(InvoiceStatusPendingPayment) ||
		invoice.PaymentDueAt == nil || !invoice.PaymentDueAt.After(now) ||
		invoice.PaidAmountMinor != 0 || invoice.WaivedAmountMinor != 0 ||
		invoice.CreditAmountMinor != 0 ||
		invoice.RefundableAmountMinor != 0 || invoice.RefundedAmountMinor != 0 ||
		invoice.SettledAt != nil || invoice.CancelledAt != nil {
		return "", bizerr.NewCode(CodeSettlementNotEligible)
	}
	allocationCount, err := database.GetValue(ctx, `
SELECT COUNT(*)
FROM plugin_idc_core_billing_payment_allocation
WHERE tenant_id = 0 AND invoice_no = ?`, invoice.InvoiceNo)
	if err != nil {
		return "", gerror.Wrap(err, "count zero-amount settlement allocations")
	}
	if allocationCount.Int64() != 0 {
		return "", bizerr.NewCode(CodeSettlementIntegrity)
	}
	if order.SubtotalMinor == 0 && order.DiscountMinor == 0 &&
		order.NetAmountMinor == 0 && order.TaxAmountMinor == 0 {
		return SettlementBasisZeroAmount, nil
	}
	if order.SubtotalMinor > 0 && order.DiscountMinor == order.SubtotalMinor &&
		order.NetAmountMinor == 0 && order.TaxAmountMinor == 0 {
		return SettlementBasisFullDiscount, nil
	}
	return "", bizerr.NewCode(CodeSettlementNotEligible)
}

// loadFundedFinancialSettlementSources loads at most the configured evidence
// bound through set-based aggregates and rejects any overflow.
func loadFundedFinancialSettlementSources(
	ctx context.Context,
	database gdb.DB,
	invoice *entity.BillingInvoice,
) ([]preparedFinancialSettlementSource, int64, error) {
	var rows []settlementAllocationEvidenceRow
	if err := database.GetScan(ctx, &rows, `
WITH target_allocations AS (
    SELECT id, tenant_id, allocation_no, invoice_no, customer_no,
           currency_code, payment_source_type_code,
           payment_source_reference, ledger_transaction_no,
           ledger_transaction_type_code, direction_code,
           original_allocation_no, amount_minor, content_digest
    FROM plugin_idc_core_billing_payment_allocation
    WHERE tenant_id = 0 AND invoice_no = ?
    ORDER BY id
    LIMIT ?
), target_transactions AS (
    SELECT DISTINCT tenant_id, ledger_transaction_no, invoice_no,
                    customer_no, currency_code
    FROM target_allocations
), ledger_reversals AS (
    SELECT target.ledger_transaction_no, COUNT(reversal.id) AS reversal_count
    FROM target_transactions target
    LEFT JOIN plugin_idc_core_billing_ledger_transaction reversal
      ON reversal.tenant_id = target.tenant_id
     AND reversal.original_transaction_no = target.ledger_transaction_no
    GROUP BY target.ledger_transaction_no
), entry_totals AS (
    SELECT target.ledger_transaction_no,
           COALESCE(SUM(CASE WHEN
               entry.direction_code = 'debit' AND entry.entry_role_code = 'balance'
               AND account.owner_type_code = 'customer'
               AND account.customer_no = target.customer_no
               AND account.account_type_code = 'customer_balance'
               THEN entry.amount_minor ELSE 0 END), 0)
               AS balance_debit_minor,
           COALESCE(SUM(CASE WHEN
               entry.direction_code = 'credit' AND entry.entry_role_code = 'revenue'
               AND account.owner_type_code = 'merchant'
               AND account.account_type_code = 'revenue'
               THEN entry.amount_minor ELSE 0 END), 0)
               AS revenue_credit_minor,
           COALESCE(SUM(CASE WHEN
               entry.direction_code = 'debit' AND entry.entry_role_code = 'cash'
               AND account.owner_type_code = 'merchant'
               AND account.account_type_code = 'merchant_cash'
               THEN entry.amount_minor ELSE 0 END), 0)
               AS cash_debit_minor,
           COALESCE(SUM(CASE WHEN
               entry.direction_code = 'credit' AND entry.entry_role_code = 'receivable'
               AND account.owner_type_code = 'merchant'
               AND account.account_type_code = 'accounts_receivable'
               THEN entry.amount_minor ELSE 0 END), 0)
               AS receivable_credit_minor
    FROM target_transactions target
    LEFT JOIN plugin_idc_core_billing_ledger_entry entry
      ON entry.tenant_id = target.tenant_id
     AND entry.transaction_no = target.ledger_transaction_no
     AND entry.invoice_no = target.invoice_no
     AND entry.currency_code = target.currency_code
    LEFT JOIN plugin_idc_core_billing_ledger_account account
      ON account.tenant_id = entry.tenant_id
     AND account.account_no = entry.account_no
     AND account.currency_code = entry.currency_code
    GROUP BY target.ledger_transaction_no
), target_adjustments AS (
    SELECT DISTINCT tenant_id, payment_source_reference AS adjustment_no
    FROM target_allocations
    WHERE payment_source_type_code = 'manual_receipt'
), approval_totals AS (
    SELECT target.adjustment_no,
           COALESCE(SUM(CASE WHEN approval.decision_code = 'approved'
               THEN 1 ELSE 0 END), 0)
               AS approval_count
    FROM target_adjustments target
    LEFT JOIN plugin_idc_core_billing_adjustment_approval approval
      ON approval.tenant_id = target.tenant_id
     AND approval.adjustment_no = target.adjustment_no
    GROUP BY target.adjustment_no
)
SELECT allocation.id, allocation.allocation_no, allocation.invoice_no,
       allocation.customer_no, allocation.currency_code,
       allocation.payment_source_type_code,
       allocation.payment_source_reference, allocation.ledger_transaction_no,
       allocation.ledger_transaction_type_code, allocation.direction_code,
       allocation.original_allocation_no, allocation.amount_minor,
       allocation.content_digest AS allocation_digest,
       ledger.source_type_code AS transaction_source_type,
       ledger.source_reference AS transaction_source_reference,
       ledger.source_snapshot_digest AS transaction_snapshot_digest,
       ledger.content_digest AS transaction_content_digest,
       ledger.original_transaction_no AS transaction_original_no,
       ledger.entry_count AS transaction_entry_count,
       ledger.debit_total_minor AS transaction_debit_minor,
       ledger.credit_total_minor AS transaction_credit_minor,
       COALESCE(reversals.reversal_count, 0) AS ledger_reversal_count,
       COALESCE(entries.balance_debit_minor, 0) AS balance_debit_minor,
       COALESCE(entries.revenue_credit_minor, 0) AS revenue_credit_minor,
       COALESCE(entries.cash_debit_minor, 0) AS cash_debit_minor,
       COALESCE(entries.receivable_credit_minor, 0) AS receivable_credit_minor,
       adjustment.adjustment_no,
       adjustment.adjustment_type_code,
       adjustment.status_code AS adjustment_status_code,
       adjustment.invoice_no AS adjustment_invoice_no,
       adjustment.customer_no AS adjustment_customer_no,
       adjustment.currency_code AS adjustment_currency_code,
       adjustment.amount_minor AS adjustment_amount_minor,
       adjustment.evidence_digest AS adjustment_evidence_digest,
       adjustment.request_digest AS adjustment_request_digest,
       adjustment.approval_required AS adjustment_approval_required,
       adjustment.requested_by AS adjustment_requested_by,
       adjustment.ledger_transaction_no AS adjustment_ledger_transaction_no,
       adjustment.proof_reference AS adjustment_proof_reference,
       adjustment.receiving_account_reference AS adjustment_receiving_reference,
       adjustment.business_purpose_code AS adjustment_business_purpose,
       adjustment.posted_at AS adjustment_posted_at,
       COALESCE(approvals.approval_count, 0) AS adjustment_approval_count
FROM target_allocations allocation
JOIN plugin_idc_core_billing_ledger_transaction ledger
  ON ledger.tenant_id = allocation.tenant_id
 AND ledger.transaction_no = allocation.ledger_transaction_no
 AND ledger.currency_code = allocation.currency_code
LEFT JOIN ledger_reversals reversals
  ON reversals.ledger_transaction_no = allocation.ledger_transaction_no
LEFT JOIN entry_totals entries
  ON entries.ledger_transaction_no = allocation.ledger_transaction_no
LEFT JOIN plugin_idc_core_billing_adjustment_request adjustment
  ON adjustment.tenant_id = allocation.tenant_id
 AND adjustment.adjustment_no = allocation.payment_source_reference
 AND allocation.payment_source_type_code = 'manual_receipt'
LEFT JOIN approval_totals approvals
  ON approvals.adjustment_no = adjustment.adjustment_no
ORDER BY allocation.id`,
		invoice.InvoiceNo, maximumFinancialSettlementEvidenceRows+1,
	); err != nil {
		return nil, 0, gerror.Wrap(err, "load funded financial settlement evidence")
	}
	if len(rows) > maximumFinancialSettlementEvidenceRows {
		return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
	}
	return validateFundedFinancialSettlementRows(invoice, rows)
}

// validateFundedFinancialSettlementRows resolves immutable allocation and
// ledger reversals into the exact positive effective funding sources.
func validateFundedFinancialSettlementRows(
	invoice *entity.BillingInvoice,
	rows []settlementAllocationEvidenceRow,
) ([]preparedFinancialSettlementSource, int64, error) {
	applies := make(map[string]settlementAllocationEvidenceRow, len(rows))
	reversals := make(map[string][]settlementAllocationEvidenceRow)
	for _, row := range rows {
		if err := validateSettlementAllocationBase(invoice, row); err != nil {
			return nil, 0, err
		}
		switch row.Direction {
		case "apply":
			if row.OriginalAllocationNo != nil {
				return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
			}
			if row.SourceType == string(PaymentSourceCredit) ||
				row.SourceType == string(PaymentSourceMigration) {
				return nil, 0, bizerr.NewCode(CodeSettlementNotEligible)
			}
			if row.SourceType != string(PaymentSourceChannel) &&
				row.SourceType != string(PaymentSourceBalance) &&
				row.SourceType != string(PaymentSourceManualReceipt) {
				return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
			}
			if row.TransactionOriginalNo != nil ||
				row.LedgerTransactionType != paymentLedgerType(PaymentSourceType(row.SourceType)) {
				return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
			}
			if _, duplicate := applies[row.AllocationNo]; duplicate {
				return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
			}
			if err := validateSettlementApplySemantics(row); err != nil {
				return nil, 0, err
			}
			applies[row.AllocationNo] = row
		case "reverse":
			if row.OriginalAllocationNo == nil || row.LedgerTransactionType != "reversal" ||
				row.TransactionOriginalNo == nil {
				return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
			}
			reversals[*row.OriginalAllocationNo] = append(
				reversals[*row.OriginalAllocationNo], row,
			)
		default:
			return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
		}
	}
	if len(applies) > maximumFinancialSettlementSources {
		return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
	}
	applyNos := make([]string, 0, len(applies))
	for allocationNo := range applies {
		applyNos = append(applyNos, allocationNo)
	}
	sort.Strings(applyNos)
	sources := make([]preparedFinancialSettlementSource, 0, len(applyNos))
	var paidMinor int64
	for _, allocationNo := range applyNos {
		apply := applies[allocationNo]
		reversalRows := reversals[allocationNo]
		var reversedMinor int64
		reversalDigests := make([]string, 0, len(reversalRows))
		for _, reversal := range reversalRows {
			if reversal.SourceType != apply.SourceType ||
				*reversal.TransactionOriginalNo != apply.LedgerTransactionNo {
				return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
			}
			if reversal.AmountMinor > apply.AmountMinor-reversedMinor {
				return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
			}
			reversedMinor += reversal.AmountMinor
			reversalDigests = append(reversalDigests, reversal.AllocationDigest)
		}
		sort.Strings(reversalDigests)
		if (apply.LedgerReversalCount == 0 && reversedMinor != 0) ||
			(apply.LedgerReversalCount == 1 && reversedMinor != apply.AmountMinor) ||
			apply.LedgerReversalCount > 1 {
			return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
		}
		effectiveMinor := apply.AmountMinor - reversedMinor
		if effectiveMinor == 0 {
			delete(reversals, allocationNo)
			continue
		}
		evidenceDigest, err := financialSettlementPaymentEvidenceDigest(
			apply, reversedMinor, reversalDigests,
		)
		if err != nil {
			return nil, 0, err
		}
		sources = append(sources, preparedFinancialSettlementSource{
			sourceType:          FinancialSettlementSourceType(apply.SourceType),
			sourceReference:     apply.SourceReference,
			paymentAllocationNo: apply.AllocationNo, amountMinor: effectiveMinor,
			evidenceDigest: evidenceDigest,
		})
		if effectiveMinor > maximumAmountMinor-paidMinor {
			return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
		}
		paidMinor += effectiveMinor
		delete(reversals, allocationNo)
	}
	if len(reversals) != 0 {
		return nil, 0, bizerr.NewCode(CodeSettlementIntegrity)
	}
	return sources, paidMinor, nil
}

// validateSettlementAllocationBase checks shared allocation, ledger, digest,
// ownership, currency, and amount invariants.
func validateSettlementAllocationBase(
	invoice *entity.BillingInvoice,
	row settlementAllocationEvidenceRow,
) error {
	if !validBusinessReference(row.AllocationNo, 40) ||
		row.InvoiceNo != invoice.InvoiceNo || row.CustomerNo != invoice.CustomerNo ||
		row.CurrencyCode != invoice.CurrencyCode || row.AmountMinor <= 0 ||
		row.AmountMinor > maximumAmountMinor ||
		!validBusinessReference(row.SourceReference, 128) ||
		!validBusinessReference(row.LedgerTransactionNo, 40) ||
		!sourceTypePattern.MatchString(row.TransactionSourceType) ||
		row.TransactionSourceReference != row.SourceReference ||
		!canonicalDigestPattern.MatchString(row.AllocationDigest) ||
		!canonicalDigestPattern.MatchString(row.TransactionSnapshotDigest) ||
		!canonicalDigestPattern.MatchString(row.TransactionContentDigest) ||
		row.TransactionEntryCount < 2 || row.TransactionEntryCount > 64 ||
		row.TransactionDebitMinor <= 0 ||
		row.TransactionDebitMinor != row.TransactionCreditMinor ||
		row.AmountMinor > row.TransactionDebitMinor {
		return bizerr.NewCode(CodeSettlementIntegrity)
	}
	return nil
}

// validateSettlementApplySemantics dispatches the small fixed source matrix to
// source-specific local evidence checks.
func validateSettlementApplySemantics(row settlementAllocationEvidenceRow) error {
	switch PaymentSourceType(row.SourceType) {
	case PaymentSourceBalance:
		return validateBalanceSettlementApply(row)
	case PaymentSourceManualReceipt:
		return validateManualReceiptSettlementApply(row)
	case PaymentSourceChannel:
		// The payment owner already verified channel evidence before creating
		// the immutable payment ledger transaction. Core verifies that fact and
		// never reaches back into a payment plugin database from this service.
	default:
		return bizerr.NewCode(CodeSettlementIntegrity)
	}
	return nil
}

// validateBalanceSettlementApply requires the exact customer-balance debit and
// merchant-revenue credit for this invoice allocation.
func validateBalanceSettlementApply(row settlementAllocationEvidenceRow) error {
	if row.BalanceDebitMinor != row.AmountMinor ||
		row.RevenueCreditMinor != row.AmountMinor {
		return bizerr.NewCode(CodeSettlementIntegrity)
	}
	return nil
}

// validateManualReceiptSettlementApply requires both accounting and reviewed
// adjustment evidence for offline money.
func validateManualReceiptSettlementApply(row settlementAllocationEvidenceRow) error {
	if err := validateManualReceiptSettlementAccounting(row); err != nil {
		return err
	}
	return validateManualReceiptSettlementReview(row)
}

// validateManualReceiptSettlementAccounting binds cash and receivable entries
// to the posted manual receipt request and invoice allocation.
func validateManualReceiptSettlementAccounting(row settlementAllocationEvidenceRow) error {
	if row.CashDebitMinor != row.AmountMinor ||
		row.ReceivableCreditMinor != row.AmountMinor ||
		row.AdjustmentNo == nil || *row.AdjustmentNo != row.SourceReference ||
		row.AdjustmentType == nil || *row.AdjustmentType != "manual_receipt" ||
		row.AdjustmentStatus == nil || *row.AdjustmentStatus != "posted" ||
		row.AdjustmentInvoiceNo == nil || *row.AdjustmentInvoiceNo != row.InvoiceNo ||
		row.AdjustmentCustomerNo == nil || *row.AdjustmentCustomerNo != row.CustomerNo ||
		row.AdjustmentCurrencyCode == nil || *row.AdjustmentCurrencyCode != row.CurrencyCode ||
		row.AdjustmentAmountMinor == nil || *row.AdjustmentAmountMinor != row.AmountMinor ||
		row.AdjustmentLedgerTransaction == nil ||
		*row.AdjustmentLedgerTransaction != row.LedgerTransactionNo {
		return bizerr.NewCode(CodeSettlementIntegrity)
	}
	return nil
}

// validateManualReceiptSettlementReview verifies private evidence references,
// request digests, and the required second-person approval count.
func validateManualReceiptSettlementReview(row settlementAllocationEvidenceRow) error {
	if row.AdjustmentEvidenceDigest == nil ||
		!canonicalDigestPattern.MatchString(*row.AdjustmentEvidenceDigest) ||
		row.AdjustmentRequestDigest == nil ||
		!canonicalDigestPattern.MatchString(*row.AdjustmentRequestDigest) ||
		row.AdjustmentApprovalRequired == nil || row.AdjustmentRequestedBy == nil ||
		*row.AdjustmentRequestedBy < 1 || row.AdjustmentProofReference == nil ||
		!validBusinessReference(*row.AdjustmentProofReference, 128) ||
		row.AdjustmentReceivingReference == nil ||
		!validBusinessReference(*row.AdjustmentReceivingReference, 128) ||
		row.AdjustmentBusinessPurpose == nil ||
		!sourceTypePattern.MatchString(*row.AdjustmentBusinessPurpose) ||
		row.AdjustmentPostedAt == nil ||
		(*row.AdjustmentApprovalRequired && row.AdjustmentApprovalCount != 1) {
		return bizerr.NewCode(CodeSettlementIntegrity)
	}
	return nil
}

// financialSettlementPaymentEvidenceDigest freezes the verified allocation,
// ledger, reversal, and manual-review projection without private payloads.
func financialSettlementPaymentEvidenceDigest(
	apply settlementAllocationEvidenceRow,
	reversedMinor int64,
	reversalDigests []string,
) (string, error) {
	return contentDigest(struct {
		AllocationNo               string   `json:"allocationNo"`
		SourceType                 string   `json:"sourceType"`
		SourceReference            string   `json:"sourceReference"`
		LedgerTransactionNo        string   `json:"ledgerTransactionNo"`
		LedgerTransactionType      string   `json:"ledgerTransactionType"`
		ApplyAmountMinor           int64    `json:"applyAmountMinor"`
		ReversedAmountMinor        int64    `json:"reversedAmountMinor"`
		AllocationDigest           string   `json:"allocationDigest"`
		TransactionSourceType      string   `json:"transactionSourceType"`
		TransactionSourceReference string   `json:"transactionSourceReference"`
		TransactionSnapshotDigest  string   `json:"transactionSnapshotDigest"`
		TransactionContentDigest   string   `json:"transactionContentDigest"`
		ReversalDigests            []string `json:"reversalDigests"`
		AdjustmentEvidenceDigest   string   `json:"adjustmentEvidenceDigest"`
		AdjustmentRequestDigest    string   `json:"adjustmentRequestDigest"`
	}{
		apply.AllocationNo, apply.SourceType, apply.SourceReference,
		apply.LedgerTransactionNo, apply.LedgerTransactionType,
		apply.AmountMinor, reversedMinor, apply.AllocationDigest,
		apply.TransactionSourceType, apply.TransactionSourceReference,
		apply.TransactionSnapshotDigest, apply.TransactionContentDigest,
		reversalDigests, optionalString(apply.AdjustmentEvidenceDigest),
		optionalString(apply.AdjustmentRequestDigest),
	})
}

// optionalString projects a nullable generated entity field without exposing a
// pointer in the service contract.
func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// finalizePreparedFinancialSettlement derives stable source numbers, source
// digests, the ordered snapshot digest, and the final header digest.
func finalizePreparedFinancialSettlement(prepared *preparedFinancialSettlement) error {
	if prepared == nil {
		return bizerr.NewCode(CodeSettlementIntegrity)
	}
	for index := range prepared.sources {
		source := &prepared.sources[index]
		source.settlementSourceNo = deterministicFinancialSettlementSourceNo(
			prepared.settlementNo, source.sourceType, source.sourceReference,
			source.paymentAllocationNo, source.creditExposureNo,
		)
		if source.evidenceDigest == "" {
			digest, err := contentDigest(struct {
				Basis   FinancialSettlementSourceType `json:"basis"`
				OrderNo string                        `json:"orderNo"`
			}{source.sourceType, prepared.orderNo})
			if err != nil {
				return err
			}
			source.evidenceDigest = digest
		}
		digest, err := financialSettlementSourceContentDigest(*prepared, *source)
		if err != nil {
			return err
		}
		source.contentDigest = digest
	}
	sort.Slice(prepared.sources, func(left int, right int) bool {
		return prepared.sources[left].settlementSourceNo <
			prepared.sources[right].settlementSourceNo
	})
	sourceDigests := make([]string, 0, len(prepared.sources))
	for _, source := range prepared.sources {
		sourceDigests = append(sourceDigests, source.contentDigest)
	}
	snapshotDigest, err := contentDigest(sourceDigests)
	if err != nil {
		return err
	}
	prepared.sourceSnapshotDigest = snapshotDigest
	prepared.contentDigest, err = financialSettlementHeaderContentDigest(*prepared)
	return err
}

// financialSettlementSourceContentDigest binds one source to the exact header
// ownership and amount coordinates.
func financialSettlementSourceContentDigest(
	prepared preparedFinancialSettlement,
	source preparedFinancialSettlementSource,
) (string, error) {
	return contentDigest(struct {
		SettlementSourceNo  string                        `json:"settlementSourceNo"`
		SettlementNo        string                        `json:"settlementNo"`
		OrderNo             string                        `json:"orderNo"`
		InvoiceNo           string                        `json:"invoiceNo"`
		CustomerNo          string                        `json:"customerNo"`
		CurrencyCode        string                        `json:"currencyCode"`
		SourceType          FinancialSettlementSourceType `json:"sourceType"`
		SourceReference     string                        `json:"sourceReference"`
		PaymentAllocationNo string                        `json:"paymentAllocationNo"`
		CreditExposureNo    string                        `json:"creditExposureNo"`
		AmountMinor         int64                         `json:"amountMinor"`
		EvidenceDigest      string                        `json:"evidenceDigest"`
	}{
		source.settlementSourceNo, prepared.settlementNo, prepared.orderNo,
		prepared.invoiceNo, prepared.customerNo, prepared.currencyCode,
		source.sourceType, source.sourceReference, source.paymentAllocationNo,
		source.creditExposureNo, source.amountMinor, source.evidenceDigest,
	})
}

// financialSettlementHeaderContentDigest freezes the canonical settlement
// header and its ordered source snapshot.
func financialSettlementHeaderContentDigest(
	prepared preparedFinancialSettlement,
) (string, error) {
	return contentDigest(struct {
		SettlementNo           string                   `json:"settlementNo"`
		OrderNo                string                   `json:"orderNo"`
		InvoiceNo              string                   `json:"invoiceNo"`
		CustomerNo             string                   `json:"customerNo"`
		CurrencyCode           string                   `json:"currencyCode"`
		Basis                  FinancialSettlementBasis `json:"basis"`
		TotalAmountMinor       int64                    `json:"totalAmountMinor"`
		PaidAmountMinor        int64                    `json:"paidAmountMinor"`
		CreditAmountMinor      int64                    `json:"creditAmountMinor"`
		SourceCount            int                      `json:"sourceCount"`
		IdentityEvidenceDigest string                   `json:"identityEvidenceDigest"`
		SourceSnapshotDigest   string                   `json:"sourceSnapshotDigest"`
		SettledAtUnixMicro     int64                    `json:"settledAtUnixMicro"`
	}{
		prepared.settlementNo, prepared.orderNo, prepared.invoiceNo,
		prepared.customerNo, prepared.currencyCode, prepared.basis,
		prepared.totalAmountMinor, prepared.paidAmountMinor,
		prepared.creditAmountMinor, len(prepared.sources),
		prepared.identityEvidenceDigest, prepared.sourceSnapshotDigest,
		prepared.settledAt.UTC().UnixMicro(),
	})
}

// persistFinancialSettlement writes the optional zero-value invoice transition,
// immutable header, and all sources in one caller-owned transaction.
func persistFinancialSettlement(
	ctx context.Context,
	database gdb.DB,
	invoice *entity.BillingInvoice,
	prepared preparedFinancialSettlement,
) error {
	if prepared.basis == SettlementBasisZeroAmount ||
		prepared.basis == SettlementBasisTrial ||
		prepared.basis == SettlementBasisFullDiscount {
		updated, err := database.Model(dao.BillingInvoice.Table()).Ctx(ctx).
			Where(do.BillingInvoice{
				TenantId: idccap.DefaultTenantID, InvoiceNo: invoice.InvoiceNo,
				StatusCode: string(InvoiceStatusPendingPayment), Version: invoice.Version,
			}).Data(do.BillingInvoice{
			StatusCode: string(InvoiceStatusPaid), PaidAmountMinor: int64(0),
			WaivedAmountMinor: int64(0), CreditAmountMinor: int64(0),
			RefundableAmountMinor: int64(0),
			RefundedAmountMinor:   int64(0), SettledAt: &prepared.settledAt,
			Version: invoice.Version + 1,
		}).Update()
		if err != nil {
			return gerror.Wrap(err, "settle zero-amount invoice")
		}
		if err = requireFinancialSettlementAffected(updated, "settle zero-amount invoice"); err != nil {
			return err
		}
	} else if prepared.basis == SettlementBasisCredit {
		updated, err := database.Model(dao.BillingInvoice.Table()).Ctx(ctx).
			Where(do.BillingInvoice{
				TenantId: idccap.DefaultTenantID, InvoiceNo: invoice.InvoiceNo,
				StatusCode: string(InvoiceStatusPendingPayment), Version: invoice.Version,
				PaidAmountMinor: int64(0), WaivedAmountMinor: int64(0),
				CreditAmountMinor: int64(0), RefundableAmountMinor: int64(0),
				RefundedAmountMinor: int64(0),
			}).Data(do.BillingInvoice{
			StatusCode:        string(InvoiceStatusCreditSettled),
			CreditAmountMinor: prepared.creditAmountMinor,
			SettledAt:         &prepared.settledAt, Version: invoice.Version + 1,
		}).Update()
		if err != nil {
			return gerror.Wrap(err, "settle credit invoice")
		}
		if err = requireFinancialSettlementAffected(updated, "settle credit invoice"); err != nil {
			return err
		}
	}
	if _, err := database.Model(dao.FinancialSettlement.Table()).Ctx(ctx).
		Data(do.FinancialSettlement{
			TenantId:     idccap.DefaultTenantID,
			SettlementNo: prepared.settlementNo, OrderNo: prepared.orderNo,
			InvoiceNo: prepared.invoiceNo, CustomerNo: prepared.customerNo,
			CurrencyCode: prepared.currencyCode, BasisCode: prepared.basis,
			TotalAmountMinor:       prepared.totalAmountMinor,
			PaidAmountMinor:        prepared.paidAmountMinor,
			CreditAmountMinor:      prepared.creditAmountMinor,
			SourceCount:            len(prepared.sources),
			IdentityEvidenceDigest: prepared.identityEvidenceDigest,
			SourceSnapshotDigest:   prepared.sourceSnapshotDigest,
			ContentDigest:          prepared.contentDigest, SettledAt: &prepared.settledAt,
		}).Insert(); err != nil {
		return gerror.Wrap(err, "insert financial settlement")
	}
	sourceRows := make([]do.FinancialSettlementSource, 0, len(prepared.sources))
	for _, source := range prepared.sources {
		var paymentAllocationNo *string
		if source.paymentAllocationNo != "" {
			value := source.paymentAllocationNo
			paymentAllocationNo = &value
		}
		var creditExposureNo *string
		if source.creditExposureNo != "" {
			value := source.creditExposureNo
			creditExposureNo = &value
		}
		sourceRows = append(sourceRows, do.FinancialSettlementSource{
			TenantId:           idccap.DefaultTenantID,
			SettlementSourceNo: source.settlementSourceNo,
			SettlementNo:       prepared.settlementNo, OrderNo: prepared.orderNo,
			InvoiceNo: prepared.invoiceNo, CustomerNo: prepared.customerNo,
			CurrencyCode: prepared.currencyCode, SourceTypeCode: string(source.sourceType),
			SourceReference:     source.sourceReference,
			PaymentAllocationNo: paymentAllocationNo,
			CreditExposureNo:    creditExposureNo, AmountMinor: source.amountMinor,
			EvidenceDigest: source.evidenceDigest, ContentDigest: source.contentDigest,
		})
	}
	if _, err := database.Model(dao.FinancialSettlementSource.Table()).Ctx(ctx).
		Data(sourceRows).Insert(); err != nil {
		return gerror.Wrap(err, "insert financial settlement sources")
	}
	return nil
}

// projectPreparedFinancialSettlement returns the secret-free stable service
// projection for a newly created or safely reused fact.
func projectPreparedFinancialSettlement(
	prepared preparedFinancialSettlement,
	reused bool,
) *FinancialSettlement {
	sources := make([]FinancialSettlementSource, 0, len(prepared.sources))
	for _, source := range prepared.sources {
		sources = append(sources, FinancialSettlementSource{
			SettlementSourceNo: source.settlementSourceNo,
			SourceType:         source.sourceType, SourceReference: source.sourceReference,
			PaymentAllocationNo: source.paymentAllocationNo,
			CreditExposureNo:    source.creditExposureNo,
			AmountMinor:         source.amountMinor, EvidenceDigest: source.evidenceDigest,
		})
	}
	return &FinancialSettlement{
		SettlementNo: prepared.settlementNo, OrderNo: prepared.orderNo,
		InvoiceNo: prepared.invoiceNo, CustomerNo: prepared.customerNo,
		CurrencyCode: prepared.currencyCode, Basis: prepared.basis,
		TotalAmountMinor:       prepared.totalAmountMinor,
		PaidAmountMinor:        prepared.paidAmountMinor,
		CreditAmountMinor:      prepared.creditAmountMinor,
		IdentityEvidenceDigest: prepared.identityEvidenceDigest,
		SourceSnapshotDigest:   prepared.sourceSnapshotDigest,
		SettledAt:              prepared.settledAt.UTC(), Sources: sources, Reused: reused,
	}
}

// projectPersistedFinancialSettlement recomputes every persisted digest,
// conservation rule, and corresponding outbox cardinality before replay.
func projectPersistedFinancialSettlement(
	ctx context.Context,
	database gdb.DB,
	header *entity.FinancialSettlement,
	reused bool,
) (*FinancialSettlement, error) {
	if header == nil || header.SettledAt == nil ||
		!canonicalDigestPattern.MatchString(header.IdentityEvidenceDigest) ||
		!canonicalDigestPattern.MatchString(header.SourceSnapshotDigest) ||
		!canonicalDigestPattern.MatchString(header.ContentDigest) ||
		header.SourceCount < 1 || header.SourceCount > maximumFinancialSettlementSources {
		return nil, gerror.Wrap(
			bizerr.NewCode(CodeSettlementIntegrity), "invalid persisted settlement header",
		)
	}
	var sourceRows []entity.FinancialSettlementSource
	if err := database.Model(dao.FinancialSettlementSource.Table()).Ctx(ctx).
		Where(do.FinancialSettlementSource{
			TenantId: idccap.DefaultTenantID, SettlementNo: header.SettlementNo,
		}).OrderAsc(dao.FinancialSettlementSource.Columns().Id).
		Limit(maximumFinancialSettlementSources + 1).Scan(&sourceRows); err != nil {
		return nil, gerror.Wrap(err, "load persisted financial settlement sources")
	}
	if len(sourceRows) != header.SourceCount {
		return nil, gerror.Wrap(
			bizerr.NewCode(CodeSettlementIntegrity), "persisted settlement source count",
		)
	}
	prepared := preparedFinancialSettlement{
		settlementNo: header.SettlementNo, orderNo: header.OrderNo,
		invoiceNo: header.InvoiceNo, customerNo: header.CustomerNo,
		currencyCode: header.CurrencyCode, basis: FinancialSettlementBasis(header.BasisCode),
		totalAmountMinor:       header.TotalAmountMinor,
		paidAmountMinor:        header.PaidAmountMinor,
		creditAmountMinor:      header.CreditAmountMinor,
		identityEvidenceDigest: header.IdentityEvidenceDigest,
		sourceSnapshotDigest:   header.SourceSnapshotDigest,
		contentDigest:          header.ContentDigest, settledAt: *header.SettledAt,
		sources: make([]preparedFinancialSettlementSource, 0, len(sourceRows)),
	}
	var actualAmount int64
	for _, row := range sourceRows {
		if !canonicalDigestPattern.MatchString(row.EvidenceDigest) ||
			!canonicalDigestPattern.MatchString(row.ContentDigest) {
			return nil, gerror.Wrap(
				bizerr.NewCode(CodeSettlementIntegrity), "persisted settlement source digest shape",
			)
		}
		source := preparedFinancialSettlementSource{
			settlementSourceNo:  row.SettlementSourceNo,
			sourceType:          FinancialSettlementSourceType(row.SourceTypeCode),
			sourceReference:     row.SourceReference,
			paymentAllocationNo: optionalString(row.PaymentAllocationNo),
			creditExposureNo:    optionalString(row.CreditExposureNo),
			amountMinor:         row.AmountMinor, evidenceDigest: row.EvidenceDigest,
			contentDigest: row.ContentDigest,
		}
		expectedDigest, err := financialSettlementSourceContentDigest(prepared, source)
		if err != nil || expectedDigest != source.contentDigest {
			return nil, gerror.Wrap(
				bizerr.NewCode(CodeSettlementIntegrity), "persisted settlement source content digest",
			)
		}
		if source.amountMinor > maximumAmountMinor-actualAmount {
			return nil, gerror.Wrap(
				bizerr.NewCode(CodeSettlementIntegrity), "persisted settlement source amount overflow",
			)
		}
		actualAmount += source.amountMinor
		prepared.sources = append(prepared.sources, source)
	}
	sort.Slice(prepared.sources, func(left int, right int) bool {
		return prepared.sources[left].settlementSourceNo <
			prepared.sources[right].settlementSourceNo
	})
	sourceDigests := make([]string, 0, len(prepared.sources))
	for _, source := range prepared.sources {
		sourceDigests = append(sourceDigests, source.contentDigest)
	}
	expectedSnapshot, err := contentDigest(sourceDigests)
	if err != nil || expectedSnapshot != prepared.sourceSnapshotDigest ||
		actualAmount != prepared.paidAmountMinor+prepared.creditAmountMinor {
		return nil, gerror.Wrap(
			bizerr.NewCode(CodeSettlementIntegrity), "persisted settlement source conservation",
		)
	}
	expectedHeader, err := financialSettlementHeaderContentDigest(prepared)
	if err != nil || expectedHeader != prepared.contentDigest ||
		!financialSettlementBasisMatchesSources(prepared) {
		return nil, gerror.Wrap(
			bizerr.NewCode(CodeSettlementIntegrity), "persisted settlement header content digest",
		)
	}
	eventCount, err := database.GetValue(ctx, `
SELECT COUNT(*)
FROM plugin_idc_core_outbox_event
WHERE tenant_id = 0 AND event_type = ? AND aggregate_type = 'commerce_order'
  AND aggregate_no = ? AND correlation_no = ?`,
		financiallySettledEventType, prepared.orderNo, prepared.settlementNo,
	)
	if err != nil {
		return nil, gerror.Wrap(err, "count persisted financial settlement event")
	}
	if eventCount.Int64() != 1 {
		return nil, gerror.Wrap(
			bizerr.NewCode(CodeSettlementIntegrity), "persisted settlement event cardinality",
		)
	}
	return projectPreparedFinancialSettlement(prepared, reused), nil
}

// financialSettlementBasisMatchesSources enforces the mutually exclusive
// header/source matrix in memory in addition to database constraints.
func financialSettlementBasisMatchesSources(prepared preparedFinancialSettlement) bool {
	switch prepared.basis {
	case SettlementBasisFundedInvoice:
		if prepared.totalAmountMinor <= 0 ||
			prepared.paidAmountMinor != prepared.totalAmountMinor ||
			prepared.creditAmountMinor != 0 {
			return false
		}
		for _, source := range prepared.sources {
			if source.amountMinor <= 0 ||
				(source.sourceType != FinancialSettlementSourceChannelPayment &&
					source.sourceType != FinancialSettlementSourceBalance &&
					source.sourceType != FinancialSettlementSourceManualReceipt) {
				return false
			}
		}
		return true
	case SettlementBasisZeroAmount, SettlementBasisFullDiscount, SettlementBasisTrial:
		return prepared.totalAmountMinor == 0 && prepared.paidAmountMinor == 0 &&
			prepared.creditAmountMinor == 0 && len(prepared.sources) == 1 &&
			prepared.sources[0].sourceType == FinancialSettlementSourceType(prepared.basis) &&
			prepared.sources[0].amountMinor == 0
	case SettlementBasisCredit:
		return prepared.totalAmountMinor > 0 && prepared.paidAmountMinor == 0 &&
			prepared.creditAmountMinor == prepared.totalAmountMinor &&
			len(prepared.sources) == 1 &&
			prepared.sources[0].sourceType == FinancialSettlementSourceCredit &&
			prepared.sources[0].amountMinor == prepared.totalAmountMinor
	default:
		return false
	}
}
