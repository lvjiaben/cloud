// This file publishes posted refund truth to the compliance owner without
// exposing core ledger tables or mutable refund state.
package taxinvoicesource

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

type refundRow struct {
	RefundAllocationNo   string    `json:"refund_allocation_no"`
	InvoiceNo            string    `json:"invoice_no"`
	InvoiceItemNo        string    `json:"invoice_item_no"`
	CustomerNo           string    `json:"customer_no"`
	CurrencyCode         string    `json:"currency_code"`
	RefundOperationNo    string    `json:"refund_operation_no"`
	RefundSourceTypeCode string    `json:"refund_source_type_code"`
	EffectiveAmountMinor int64     `json:"effective_amount_minor"`
	ContentDigest        string    `json:"content_digest"`
	PostedAt             time.Time `json:"posted_at"`
}

// GetPostedRefund returns one bounded, immutable, effective refund projection.
func (s *serviceImpl) GetPostedRefund(
	ctx context.Context,
	refundOperationNo string,
) (*taxinvoicecap.RefundOperation, error) {
	refundOperationNo = strings.TrimSpace(refundOperationNo)
	sourceType, sourceReference, parseErr := parsePublishedRefundOperationNo(refundOperationNo)
	if ctx == nil || ctx.Err() != nil || parseErr != nil {
		return nil, gerror.New("posted refund source is invalid")
	}
	var rows []refundRow
	err := s.database.GetScan(ctx, &rows, `
SELECT original.refund_allocation_no, original.invoice_no, original.invoice_item_no,
       original.customer_no, original.currency_code,
       original.refund_source_reference AS refund_operation_no,
       original.refund_source_type_code,
       original.amount_minor - COALESCE(SUM(reversal.amount_minor), 0) AS effective_amount_minor,
       original.content_digest, ledger_tx.posted_at
FROM plugin_idc_core_billing_refund_allocation AS original
JOIN plugin_idc_core_billing_ledger_transaction AS ledger_tx
  ON ledger_tx.tenant_id = original.tenant_id
 AND ledger_tx.transaction_no = original.ledger_transaction_no
 AND ledger_tx.currency_code = original.currency_code
 AND ledger_tx.transaction_type_code = original.ledger_transaction_type_code
LEFT JOIN plugin_idc_core_billing_refund_allocation AS reversal
  ON reversal.tenant_id = original.tenant_id
 AND reversal.original_refund_allocation_no = original.refund_allocation_no
 AND reversal.direction_code = 'reverse'
WHERE original.tenant_id = 0
  AND original.direction_code = 'apply'
  AND original.refund_source_type_code = ?
  AND original.refund_source_reference = ?
GROUP BY original.id, original.refund_allocation_no, original.invoice_no,
         original.invoice_item_no, original.customer_no, original.currency_code,
         original.refund_source_reference, original.refund_source_type_code,
         original.amount_minor,
         original.content_digest, ledger_tx.posted_at
HAVING original.amount_minor - COALESCE(SUM(reversal.amount_minor), 0) > 0
ORDER BY original.invoice_no, original.invoice_item_no, original.refund_allocation_no
LIMIT ?`, sourceType, sourceReference, taxinvoicecap.MaximumItemsPerRequest+1)
	if err != nil {
		return nil, gerror.Wrap(err, "load posted refund source")
	}
	if len(rows) == 0 || len(rows) > taxinvoicecap.MaximumItemsPerRequest {
		return nil, gerror.New("posted refund source is unavailable")
	}
	first := rows[0]
	if !businessNoPattern.MatchString(first.CustomerNo) || !currencyPattern.MatchString(first.CurrencyCode) ||
		!validRefundSourceType(first.RefundSourceTypeCode) ||
		first.PostedAt.IsZero() {
		return nil, gerror.New("posted refund source is inconsistent")
	}
	result := &taxinvoicecap.RefundOperation{
		RefundOperationNo: refundOperationNo, CustomerNo: first.CustomerNo,
		CurrencyCode: first.CurrencyCode, PostedAt: first.PostedAt.UTC(),
		Allocations: make([]taxinvoicecap.RefundAllocation, 0, len(rows)),
	}
	for _, row := range rows {
		if row.RefundOperationNo != sourceReference || row.CustomerNo != first.CustomerNo ||
			row.CurrencyCode != first.CurrencyCode || row.RefundSourceTypeCode != sourceType ||
			!businessNoPattern.MatchString(row.RefundAllocationNo) ||
			!businessNoPattern.MatchString(row.InvoiceNo) || !businessNoPattern.MatchString(row.InvoiceItemNo) ||
			row.EffectiveAmountMinor <= 0 || !digestPattern.MatchString(row.ContentDigest) {
			return nil, gerror.New("posted refund source is inconsistent")
		}
		result.Allocations = append(result.Allocations, taxinvoicecap.RefundAllocation{
			RefundAllocationNo: row.RefundAllocationNo, BillingInvoiceNo: row.InvoiceNo,
			BillingInvoiceItemNo: row.InvoiceItemNo, AmountMinor: row.EffectiveAmountMinor,
			ContentDigest: row.ContentDigest,
		})
	}
	return result, nil
}

func validRefundSourceType(value string) bool {
	switch value {
	case "channel_refund", "balance_credit", "credit_release", "manual", "migration":
		return true
	default:
		return false
	}
}

type refundReferenceRow struct {
	RefundOperationNo string `json:"refund_operation_no"`
	RefundSourceType  string `json:"refund_source_type"`
	FirstID           uint64 `json:"first_id"`
}

// ListPostedRefundOperations returns a bounded, stable page for the primary-node
// compliance worker. The cursor advances only after the consumer commits.
func (s *serviceImpl) ListPostedRefundOperations(
	ctx context.Context,
	afterCursor string,
	limit int,
) ([]taxinvoicecap.PostedRefundReference, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > taxinvoicecap.MaximumRefundOperations {
		return nil, gerror.New("posted refund feed request is invalid")
	}
	afterID, err := parseRefundCursor(strings.TrimSpace(afterCursor))
	if err != nil {
		return nil, err
	}
	var rows []refundReferenceRow
	err = s.database.GetScan(ctx, &rows, `
SELECT original.refund_source_reference AS refund_operation_no,
       original.refund_source_type_code AS refund_source_type,
       MIN(original.id) AS first_id
FROM plugin_idc_core_billing_refund_allocation AS original
JOIN plugin_idc_core_billing_ledger_transaction AS ledger_tx
  ON ledger_tx.tenant_id = original.tenant_id
 AND ledger_tx.transaction_no = original.ledger_transaction_no
 AND ledger_tx.currency_code = original.currency_code
 AND ledger_tx.transaction_type_code = original.ledger_transaction_type_code
WHERE original.tenant_id = 0 AND original.direction_code = 'apply'
GROUP BY original.refund_source_type_code, original.refund_source_reference
HAVING MIN(original.id) > ?
   AND SUM(
       original.amount_minor - COALESCE((
           SELECT SUM(reversal.amount_minor)
           FROM plugin_idc_core_billing_refund_allocation AS reversal
           WHERE reversal.tenant_id = original.tenant_id
             AND reversal.original_refund_allocation_no = original.refund_allocation_no
             AND reversal.direction_code = 'reverse'
       ), 0)
   ) > 0
ORDER BY MIN(original.id)
LIMIT ?`, afterID, limit)
	if err != nil {
		return nil, gerror.Wrap(err, "list posted refund operations")
	}
	result := make([]taxinvoicecap.PostedRefundReference, 0, len(rows))
	for _, row := range rows {
		operationNo, formatErr := publishedRefundOperationNo(row.RefundSourceType, row.RefundOperationNo)
		if row.FirstID == 0 || formatErr != nil {
			return nil, gerror.New("posted refund feed is inconsistent")
		}
		result = append(result, taxinvoicecap.PostedRefundReference{
			RefundOperationNo: operationNo,
			Cursor:            fmt.Sprintf("rfc_%016x", row.FirstID),
		})
	}
	return result, nil
}

// publishedRefundOperationNo namespaces a source reference by its ledger source type.
func publishedRefundOperationNo(sourceType, sourceReference string) (string, error) {
	if !validRefundSourceType(sourceType) || !refundOperationPattern.MatchString(sourceReference) {
		return "", gerror.New("posted refund operation identity is invalid")
	}
	return sourceType + ":" + sourceReference, nil
}

// parsePublishedRefundOperationNo decodes the source namespace without exposing database keys.
func parsePublishedRefundOperationNo(operationNo string) (string, string, error) {
	sourceType, sourceReference, found := strings.Cut(operationNo, ":")
	if !found {
		return "", "", gerror.New("posted refund operation identity is invalid")
	}
	normalized, err := publishedRefundOperationNo(sourceType, sourceReference)
	if err != nil || normalized != operationNo {
		return "", "", gerror.New("posted refund operation identity is invalid")
	}
	return sourceType, sourceReference, nil
}

func parseRefundCursor(cursor string) (uint64, error) {
	if cursor == "" {
		return 0, nil
	}
	if len(cursor) != 20 || !strings.HasPrefix(cursor, "rfc_") {
		return 0, gerror.New("posted refund feed cursor is invalid")
	}
	value, err := strconv.ParseUint(cursor[4:], 16, 64)
	if err != nil || value == 0 {
		return 0, gerror.New("posted refund feed cursor is invalid")
	}
	return value, nil
}
