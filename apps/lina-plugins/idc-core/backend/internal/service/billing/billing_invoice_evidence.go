// This file validates immutable ledger and allocation evidence used by invoice
// transitions. All reads are bounded by stable business keys and execute while
// the owning invoice is locked.

package billing

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type invoiceLedgerTransaction struct {
	TransactionNo       string `orm:"transaction_no"`
	TransactionType     string `orm:"transaction_type_code"`
	CurrencyCode        string `orm:"currency_code"`
	OriginalTransaction string `orm:"original_transaction_no"`
	AmountMinor         int64  `orm:"debit_total_minor"`
}

type paymentAllocationRow struct {
	AllocationNo         string `orm:"allocation_no"`
	InvoiceNo            string `orm:"invoice_no"`
	InvoiceItemNo        string `orm:"invoice_item_no"`
	CustomerNo           string `orm:"customer_no"`
	CurrencyCode         string `orm:"currency_code"`
	SourceType           string `orm:"payment_source_type_code"`
	SourceReference      string `orm:"payment_source_reference"`
	LedgerTransactionNo  string `orm:"ledger_transaction_no"`
	Direction            string `orm:"direction_code"`
	OriginalAllocationNo string `orm:"original_allocation_no"`
	AmountMinor          int64  `orm:"amount_minor"`
	ContentDigest        string `orm:"content_digest"`
}

type refundAllocationRow struct {
	RefundAllocationNo         string `orm:"refund_allocation_no"`
	InvoiceNo                  string `orm:"invoice_no"`
	InvoiceItemNo              string `orm:"invoice_item_no"`
	CustomerNo                 string `orm:"customer_no"`
	CurrencyCode               string `orm:"currency_code"`
	PaymentAllocationNo        string `orm:"payment_allocation_no"`
	SourceType                 string `orm:"refund_source_type_code"`
	SourceReference            string `orm:"refund_source_reference"`
	LedgerTransactionNo        string `orm:"ledger_transaction_no"`
	Direction                  string `orm:"direction_code"`
	OriginalRefundAllocationNo string `orm:"original_refund_allocation_no"`
	AmountMinor                int64  `orm:"amount_minor"`
	ContentDigest              string `orm:"content_digest"`
}

type invoiceItemAmounts struct {
	TotalMinor         int64 `orm:"total_amount_minor"`
	RefundableCapMinor int64 `orm:"refundable_cap_minor"`
	PaymentMinor       int64 `orm:"payment_minor"`
	RefundMinor        int64 `orm:"refund_minor"`
}

type badDebtWriteoffAmounts struct {
	TransactionCount int64 `orm:"transaction_count"`
	ReceivableMinor  int64 `orm:"receivable_minor"`
	ExpenseMinor     int64 `orm:"expense_minor"`
}

func lockInvoiceLedgerTransaction(
	ctx context.Context,
	database gdb.DB,
	transactionNo string,
) (*invoiceLedgerTransaction, error) {
	var transaction *invoiceLedgerTransaction
	if err := database.GetScan(ctx, &transaction, `
SELECT transaction_no, transaction_type_code, currency_code,
       COALESCE(original_transaction_no, '') AS original_transaction_no,
       debit_total_minor
FROM plugin_idc_core_billing_ledger_transaction
WHERE tenant_id = 0 AND transaction_no = ?
FOR UPDATE`, transactionNo); err != nil {
		return nil, gerror.Wrap(err, "lock invoice ledger transaction")
	}
	if transaction == nil {
		return nil, bizerr.NewCode(CodeInvoiceConflict)
	}
	return transaction, nil
}

func requireUnreversedLedgerTransaction(
	ctx context.Context,
	database gdb.DB,
	transactionNo string,
) error {
	count, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_transaction
WHERE tenant_id = 0 AND original_transaction_no = ?`, transactionNo)
	if err != nil {
		return gerror.Wrap(err, "count invoice ledger reversals")
	}
	if count.Int64() != 0 {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	return nil
}

func loadPaymentAllocation(
	ctx context.Context,
	database gdb.DB,
	allocationNo string,
	lock bool,
) (*paymentAllocationRow, error) {
	query := `
SELECT allocation_no, invoice_no, COALESCE(invoice_item_no, '') AS invoice_item_no,
       customer_no, currency_code, payment_source_type_code,
       payment_source_reference, ledger_transaction_no, direction_code,
       COALESCE(original_allocation_no, '') AS original_allocation_no,
       amount_minor, content_digest
FROM plugin_idc_core_billing_payment_allocation
WHERE tenant_id = 0 AND allocation_no = ?`
	if lock {
		query += ` FOR UPDATE`
	}
	var allocation *paymentAllocationRow
	if err := database.GetScan(ctx, &allocation, query, allocationNo); err != nil {
		return nil, gerror.Wrap(err, "load invoice payment allocation")
	}
	return allocation, nil
}

func loadRefundAllocation(
	ctx context.Context,
	database gdb.DB,
	allocationNo string,
	lock bool,
) (*refundAllocationRow, error) {
	query := `
SELECT refund_allocation_no, invoice_no, invoice_item_no, customer_no,
       currency_code, payment_allocation_no, refund_source_type_code,
       refund_source_reference, ledger_transaction_no, direction_code,
       COALESCE(original_refund_allocation_no, '') AS original_refund_allocation_no,
       amount_minor, content_digest
FROM plugin_idc_core_billing_refund_allocation
WHERE tenant_id = 0 AND refund_allocation_no = ?`
	if lock {
		query += ` FOR UPDATE`
	}
	var allocation *refundAllocationRow
	if err := database.GetScan(ctx, &allocation, query, allocationNo); err != nil {
		return nil, gerror.Wrap(err, "load invoice refund allocation")
	}
	return allocation, nil
}

func loadPaymentReplay(
	ctx context.Context,
	database gdb.DB,
	invoiceNo string,
	itemNo string,
	sourceType PaymentSourceType,
	sourceReference string,
	direction string,
) (*paymentAllocationRow, error) {
	var allocation *paymentAllocationRow
	if err := database.GetScan(ctx, &allocation, `
SELECT allocation_no, invoice_no, COALESCE(invoice_item_no, '') AS invoice_item_no,
       customer_no, currency_code, payment_source_type_code,
       payment_source_reference, ledger_transaction_no, direction_code,
       COALESCE(original_allocation_no, '') AS original_allocation_no,
       amount_minor, content_digest
FROM plugin_idc_core_billing_payment_allocation
WHERE tenant_id = 0 AND invoice_no = ? AND COALESCE(invoice_item_no, '') = ?
  AND payment_source_type_code = ? AND payment_source_reference = ?
  AND direction_code = ?`, invoiceNo, itemNo, sourceType, sourceReference, direction); err != nil {
		return nil, gerror.Wrap(err, "load invoice payment replay")
	}
	return allocation, nil
}

func loadRefundReplay(
	ctx context.Context,
	database gdb.DB,
	invoiceNo string,
	itemNo string,
	sourceType RefundSourceType,
	sourceReference string,
	direction string,
) (*refundAllocationRow, error) {
	var allocation *refundAllocationRow
	if err := database.GetScan(ctx, &allocation, `
SELECT refund_allocation_no, invoice_no, invoice_item_no, customer_no,
       currency_code, payment_allocation_no, refund_source_type_code,
       refund_source_reference, ledger_transaction_no, direction_code,
       COALESCE(original_refund_allocation_no, '') AS original_refund_allocation_no,
       amount_minor, content_digest
FROM plugin_idc_core_billing_refund_allocation
WHERE tenant_id = 0 AND invoice_no = ? AND invoice_item_no = ?
  AND refund_source_type_code = ? AND refund_source_reference = ?
  AND direction_code = ?`, invoiceNo, itemNo, sourceType, sourceReference, direction); err != nil {
		return nil, gerror.Wrap(err, "load invoice refund replay")
	}
	return allocation, nil
}

func loadInvoiceItemAmounts(
	ctx context.Context,
	database gdb.DB,
	invoiceNo string,
	itemNo string,
) (*invoiceItemAmounts, error) {
	var amounts *invoiceItemAmounts
	if err := database.GetScan(ctx, &amounts, `
SELECT item.total_amount_minor, item.refundable_cap_minor,
       COALESCE((SELECT SUM(CASE direction_code WHEN 'apply' THEN amount_minor ELSE -amount_minor END)
         FROM plugin_idc_core_billing_payment_allocation payment
         WHERE payment.tenant_id = 0 AND payment.invoice_no = item.invoice_no
           AND payment.invoice_item_no = item.invoice_item_no), 0) AS payment_minor,
       COALESCE((SELECT SUM(CASE direction_code WHEN 'apply' THEN amount_minor ELSE -amount_minor END)
         FROM plugin_idc_core_billing_refund_allocation refund
         WHERE refund.tenant_id = 0 AND refund.invoice_no = item.invoice_no
           AND refund.invoice_item_no = item.invoice_item_no), 0) AS refund_minor
FROM plugin_idc_core_billing_invoice_item item
WHERE item.tenant_id = 0 AND item.invoice_no = ? AND item.invoice_item_no = ?
FOR SHARE`, invoiceNo, itemNo); err != nil {
		return nil, gerror.Wrap(err, "load invoice item allocation amounts")
	}
	return amounts, nil
}

func effectivePaymentAllocationMinor(
	ctx context.Context,
	database gdb.DB,
	allocationNo string,
) (int64, error) {
	value, err := database.GetValue(ctx, `
SELECT original.amount_minor - COALESCE(SUM(reversal.amount_minor), 0)
FROM plugin_idc_core_billing_payment_allocation original
LEFT JOIN plugin_idc_core_billing_payment_allocation reversal
  ON reversal.original_allocation_no = original.allocation_no
WHERE original.tenant_id = 0 AND original.allocation_no = ?
GROUP BY original.amount_minor`, allocationNo)
	if err != nil {
		return 0, gerror.Wrap(err, "load effective payment allocation")
	}
	return value.Int64(), nil
}

func effectiveRefundAgainstPaymentMinor(
	ctx context.Context,
	database gdb.DB,
	paymentAllocationNo string,
) (int64, error) {
	value, err := database.GetValue(ctx, `
SELECT COALESCE(SUM(CASE direction_code WHEN 'apply' THEN amount_minor ELSE -amount_minor END), 0)
FROM plugin_idc_core_billing_refund_allocation
WHERE tenant_id = 0 AND payment_allocation_no = ?`, paymentAllocationNo)
	if err != nil {
		return 0, gerror.Wrap(err, "load refunds against payment allocation")
	}
	return value.Int64(), nil
}

func effectiveRefundAllocationMinor(
	ctx context.Context,
	database gdb.DB,
	allocationNo string,
) (int64, error) {
	value, err := database.GetValue(ctx, `
SELECT original.amount_minor - COALESCE(SUM(reversal.amount_minor), 0)
FROM plugin_idc_core_billing_refund_allocation original
LEFT JOIN plugin_idc_core_billing_refund_allocation reversal
  ON reversal.original_refund_allocation_no = original.refund_allocation_no
WHERE original.tenant_id = 0 AND original.refund_allocation_no = ?
GROUP BY original.amount_minor`, allocationNo)
	if err != nil {
		return 0, gerror.Wrap(err, "load effective refund allocation")
	}
	return value.Int64(), nil
}

func usedLedgerAllocationMinor(
	ctx context.Context,
	database gdb.DB,
	transactionNo string,
) (int64, error) {
	value, err := database.GetValue(ctx, `
SELECT
  COALESCE((SELECT SUM(amount_minor)
    FROM plugin_idc_core_billing_payment_allocation
    WHERE tenant_id = 0 AND ledger_transaction_no = ?), 0)
  + COALESCE((SELECT SUM(amount_minor)
    FROM plugin_idc_core_billing_refund_allocation
    WHERE tenant_id = 0 AND ledger_transaction_no = ?), 0)`,
		transactionNo, transactionNo)
	if err != nil {
		return 0, gerror.Wrap(err, "load used ledger allocation amount")
	}
	return value.Int64(), nil
}

func validateBadDebtWriteoff(
	ctx context.Context,
	database gdb.DB,
	invoiceNo string,
	transaction *invoiceLedgerTransaction,
	outstandingMinor int64,
) error {
	if transaction == nil || transaction.TransactionType != "writeoff" ||
		transaction.OriginalTransaction != "" || transaction.AmountMinor != outstandingMinor ||
		outstandingMinor <= 0 {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	var amounts badDebtWriteoffAmounts
	if err := database.GetScan(ctx, &amounts, `
SELECT
  COUNT(DISTINCT candidate.transaction_no) AS transaction_count,
  COALESCE(SUM(CASE WHEN candidate.transaction_no = ?
    AND entry.direction_code = 'credit' AND entry.entry_role_code = 'receivable'
    THEN entry.amount_minor ELSE 0 END), 0) AS receivable_minor,
  COALESCE(SUM(CASE WHEN candidate.transaction_no = ?
    AND entry.direction_code = 'debit' AND entry.entry_role_code = 'expense'
    THEN entry.amount_minor ELSE 0 END), 0) AS expense_minor
FROM plugin_idc_core_billing_ledger_transaction candidate
JOIN plugin_idc_core_billing_ledger_entry entry
  ON entry.transaction_no = candidate.transaction_no
WHERE candidate.tenant_id = 0 AND candidate.transaction_type_code = 'writeoff'
  AND entry.invoice_no = ?`, transaction.TransactionNo, transaction.TransactionNo, invoiceNo); err != nil {
		return gerror.Wrap(err, "validate invoice bad debt writeoff")
	}
	if amounts.TransactionCount != 1 || amounts.ReceivableMinor != outstandingMinor ||
		amounts.ExpenseMinor != outstandingMinor {
		return bizerr.NewCode(CodeInvoiceConflict)
	}
	return requireUnreversedLedgerTransaction(ctx, database, transaction.TransactionNo)
}
