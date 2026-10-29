// Completed refund evidence stays with the financial owner. Ordinary refunds
// require net posted allocations; credit refunds require the owner's atomic
// credit-refund movement after all effective funding commands succeeded.

package billing

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
)

// CompletedRefundScope identifies the original financial aggregate exactly.
type CompletedRefundScope struct{ OrderNo, InvoiceNo, CustomerNo, CurrencyCode string }

// CompletedRefundEvidence reports net completed money, never an approval to
// initiate a refund. HasRequest also fences partial, failed and unknown requests.
type CompletedRefundEvidence struct {
	HasRequest           bool
	CompletedAmountMinor int64
	EvidenceDigest       string
}

// CompletedRefundEvidenceReader reads already committed financial-owner facts.
type CompletedRefundEvidenceReader interface {
	// ReadCompletedRefund requires the caller to hold the original order and
	// invoice locks when its result will authorize a subsequent state change.
	ReadCompletedRefund(context.Context, CompletedRefundScope) (*CompletedRefundEvidence, error)
}

type completedRefundEvidenceReader struct{ database gdb.DB }

// NewCompletedRefundEvidenceReader constructs the stateless owner reader once
// at startup. It has no provider, payment command or mutation dependency.
func NewCompletedRefundEvidenceReader(database gdb.DB) (CompletedRefundEvidenceReader, error) {
	if isNilFinanceDirectoryDependency(database) {
		return nil, bizerr.NewCode(CodeRefundUnavailable)
	}
	return &completedRefundEvidenceReader{database: database}, nil
}

func (s *completedRefundEvidenceReader) ReadCompletedRefund(ctx context.Context, input CompletedRefundScope) (*CompletedRefundEvidence, error) {
	if ctx == nil || ctx.Err() != nil || !validBusinessReference(input.OrderNo, 40) || !validBusinessReference(input.InvoiceNo, 40) || !validBusinessReference(input.CustomerNo, 40) || !currencyPattern.MatchString(input.CurrencyCode) {
		return nil, bizerr.NewCode(CodeRefundInvalid)
	}
	var row struct {
		HasRequest           bool
		CompletedAmountMinor int64
		Proof                string
	}
	if err := s.database.GetScan(ctx, &row, completedRefundEvidenceSQL, input.OrderNo, input.InvoiceNo, input.CustomerNo, input.CurrencyCode); err != nil {
		return nil, err
	}
	digest, err := contentDigest(struct {
		Scope CompletedRefundScope
		Proof string
	}{input, row.Proof})
	if err != nil {
		return nil, err
	}
	return &CompletedRefundEvidence{HasRequest: row.HasRequest, CompletedAmountMinor: row.CompletedAmountMinor, EvidenceDigest: digest}, nil
}

const completedRefundEvidenceSQL = `WITH requests AS (
 SELECT refund_no,invoice_no,customer_no,currency_code,status_code,completed_at,requested_amount_minor
 FROM plugin_idc_core_refund_request WHERE tenant_id=0 AND order_no=? AND invoice_no=? AND customer_no=? AND currency_code=? AND status_code<>'rejected'
), effective AS (
 SELECT DISTINCT ON (source.refund_source_no) source.refund_no,source.refund_source_no,source.credit_exposure_no,
  source.payment_allocation_no,item.invoice_item_no,source.amount_minor,command.command_no,command.status_code,command.completed_at
 FROM requests request JOIN plugin_idc_core_refund_source_allocation source ON source.tenant_id=0 AND source.refund_no=request.refund_no
 JOIN plugin_idc_core_refund_request_item item ON item.tenant_id=0 AND item.refund_no=request.refund_no AND item.refund_item_no=source.refund_item_no AND item.invoice_no=request.invoice_no AND item.customer_no=request.customer_no AND item.currency_code=request.currency_code
 JOIN plugin_idc_core_refund_execution_command command ON command.tenant_id=0 AND command.refund_no=request.refund_no AND command.refund_source_no=source.refund_source_no AND command.amount_minor=source.amount_minor AND command.currency_code=request.currency_code
 ORDER BY source.refund_source_no,CASE command.destination_code WHEN 'same_currency_balance' THEN 0 ELSE 1 END,command.id DESC
), posted AS (
 SELECT command.refund_no,COALESCE(SUM(allocation.amount_minor-COALESCE(reversed.amount_minor,0)),0) AS amount_minor
 FROM effective command JOIN requests request ON request.refund_no=command.refund_no
 JOIN plugin_idc_core_billing_refund_allocation allocation ON allocation.tenant_id=0 AND allocation.refund_source_reference=command.command_no
  AND allocation.direction_code='apply' AND allocation.invoice_no=request.invoice_no AND allocation.customer_no=request.customer_no AND allocation.currency_code=request.currency_code
  AND allocation.payment_allocation_no=command.payment_allocation_no AND allocation.invoice_item_no=command.invoice_item_no
 JOIN plugin_idc_core_billing_ledger_transaction ledger ON ledger.tenant_id=0 AND ledger.transaction_no=allocation.ledger_transaction_no
  AND ledger.currency_code=allocation.currency_code AND ledger.transaction_type_code='refund' AND ledger.posted_at IS NOT NULL
 LEFT JOIN LATERAL (
  SELECT SUM(reversal.amount_minor) AS amount_minor FROM plugin_idc_core_billing_refund_allocation reversal
  JOIN plugin_idc_core_billing_ledger_transaction tx ON tx.tenant_id=0 AND tx.transaction_no=reversal.ledger_transaction_no AND tx.transaction_type_code='reversal' AND tx.currency_code=reversal.currency_code AND tx.posted_at IS NOT NULL
  WHERE reversal.tenant_id=0 AND reversal.direction_code='reverse' AND reversal.original_refund_allocation_no=allocation.refund_allocation_no
 ) reversed ON TRUE
 WHERE command.status_code='succeeded' AND command.completed_at IS NOT NULL AND command.credit_exposure_no IS NULL
 GROUP BY command.refund_no
), credit AS (
 SELECT request.refund_no,movement.amount_minor
 FROM requests request JOIN plugin_idc_core_credit_exposure exposure ON exposure.tenant_id=0 AND exposure.invoice_no=request.invoice_no AND exposure.customer_no=request.customer_no AND exposure.currency_code=request.currency_code
 JOIN plugin_idc_core_credit_account_movement movement ON movement.tenant_id=0 AND movement.exposure_no=exposure.exposure_no
  AND movement.credit_account_no=exposure.credit_account_no AND movement.customer_no=request.customer_no AND movement.currency_code=request.currency_code
  AND movement.movement_type_code='refund' AND movement.source_type_code='credit_refund' AND movement.source_reference='online_refund:'||request.refund_no
 WHERE movement.amount_minor=request.requested_amount_minor
  AND EXISTS(SELECT 1 FROM effective source WHERE source.refund_no=request.refund_no AND source.credit_exposure_no=exposure.exposure_no)
), verified AS (
 SELECT request.*,CASE WHEN request.status_code='succeeded' AND request.completed_at IS NOT NULL
  AND EXISTS(SELECT 1 FROM effective source WHERE source.refund_no=request.refund_no)
  AND NOT EXISTS(SELECT 1 FROM effective source WHERE source.refund_no=request.refund_no AND (source.status_code<>'succeeded' OR source.completed_at IS NULL))
  AND COALESCE(credit.amount_minor,posted.amount_minor,0)=request.requested_amount_minor
 THEN request.requested_amount_minor ELSE 0 END AS verified_amount
 FROM requests request LEFT JOIN posted USING(refund_no) LEFT JOIN credit USING(refund_no)
)
SELECT count(*)>0 AS has_request,COALESCE(SUM(verified_amount),0) AS completed_amount_minor,
 COALESCE(string_agg(refund_no||':'||status_code||':'||verified_amount::TEXT,',' ORDER BY refund_no),'') AS proof FROM verified`
