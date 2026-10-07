// This file turns authoritative posted refunds into ordered red-letter work.

package taxinvoiceredletter

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

// RecordPostedRefund creates correction work after the authoritative refund is posted.
func (s *serviceImpl) RecordPostedRefund(
	ctx context.Context,
	refundOperationNo string,
	idempotencyKey string,
) (*RecordResult, error) {
	refundOperationNo = strings.TrimSpace(refundOperationNo)
	if !businessNoPattern.MatchString(refundOperationNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	refund, err := s.source.GetPostedRefund(ctx, refundOperationNo)
	if err != nil || !validPostedRefund(refund, refundOperationNo) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	parts, itemNos, err := refundIdentityParts(refund.Allocations)
	if err != nil {
		return nil, err
	}
	taskNo, requestDigest := taskIdentity(refundOperationNo, parts)
	ident, err := actionIdentity("record_refund", idempotencyKey, requestDigest)
	if err != nil {
		return nil, err
	}
	var state recordPostedRefundState
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var transactionErr error
		state, transactionErr = s.recordPostedRefundTX(
			ctx, tx, refund, itemNos, taskNo, requestDigest, refundOperationNo, ident,
		)
		return transactionErr
	})
	if err != nil {
		recovered, recoveryErr := recoverPostedRefundReplay(
			ctx, s.database, taskNo, ident,
		)
		if recoveryErr == nil && recovered != nil {
			task, projectErr := projectTask(ctx, s.database, recovered)
			if projectErr != nil {
				return nil, bizerr.NewCode(CodeUnavailable)
			}
			return &RecordResult{Task: task, Reused: true}, nil
		}
		if bizerr.Is(recoveryErr, CodeConflict) {
			return nil, recoveryErr
		}
		if bizerr.Is(err, CodeConflict) {
			return nil, err
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if state.noCorrection {
		return &RecordResult{NoCorrectionRequired: true}, nil
	}
	task, err := projectTask(ctx, s.database, state.output)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &RecordResult{Task: task, Reused: state.reused}, nil
}

type recordPostedRefundState struct {
	output       *taskRow
	noCorrection bool
	reused       bool
}

type preparedRefundAllocation struct {
	refund taxinvoicecap.RefundAllocation
	rows   []sourceCandidate
	target int64
}

// validPostedRefund checks the bounded authoritative refund envelope.
func validPostedRefund(refund *taxinvoicecap.RefundOperation, operationNo string) bool {
	return refund != nil && refund.RefundOperationNo == operationNo &&
		businessNoPattern.MatchString(refund.CustomerNo) &&
		currencyPattern.MatchString(refund.CurrencyCode) && !refund.PostedAt.IsZero() &&
		len(refund.Allocations) > 0 && len(refund.Allocations) <= taxinvoicecap.MaximumItemsPerRequest
}

// refundIdentityParts validates allocations and returns stable digest inputs and item keys.
func refundIdentityParts(allocations []taxinvoicecap.RefundAllocation) ([]string, []string, error) {
	parts := make([]string, 0, len(allocations))
	itemNos := make([]string, 0, len(allocations))
	seenItems := make(map[string]struct{}, len(allocations))
	for _, allocation := range allocations {
		if allocation.AmountMinor <= 0 || !businessNoPattern.MatchString(allocation.RefundAllocationNo) ||
			!businessNoPattern.MatchString(allocation.BillingInvoiceNo) ||
			!businessNoPattern.MatchString(allocation.BillingInvoiceItemNo) ||
			!digestPattern.MatchString(allocation.ContentDigest) {
			return nil, nil, bizerr.NewCode(CodeUnavailable)
		}
		parts = append(parts, allocation.RefundAllocationNo+"\x00"+allocation.BillingInvoiceNo+"\x00"+
			allocation.BillingInvoiceItemNo+"\x00"+strconv.FormatInt(allocation.AmountMinor, 10)+"\x00"+allocation.ContentDigest)
		if _, exists := seenItems[allocation.BillingInvoiceItemNo]; !exists {
			seenItems[allocation.BillingInvoiceItemNo] = struct{}{}
			itemNos = append(itemNos, allocation.BillingInvoiceItemNo)
		}
	}
	return parts, itemNos, nil
}

// recordPostedRefundTX serializes a refund and reuses only an exact prior request.
func (s *serviceImpl) recordPostedRefundTX(
	ctx context.Context,
	tx gdb.TX,
	refund *taxinvoicecap.RefundOperation,
	itemNos []string,
	taskNo, requestDigest, refundOperationNo string,
	ident identity,
) (recordPostedRefundState, error) {
	existing, loadErr := loadTaskTX(ctx, tx, taskNo, true)
	if loadErr != nil {
		return recordPostedRefundState{}, loadErr
	}
	if existing != nil {
		replay, replayErr := loadReplayTX(ctx, tx, taskNo, "record_refund", ident.digest)
		if replayErr != nil {
			return recordPostedRefundState{}, replayErr
		}
		if replay == nil || replay.Fingerprint != ident.fingerprint {
			return recordPostedRefundState{}, bizerr.NewCode(CodeConflict)
		}
		return recordPostedRefundState{output: existing, reused: true}, nil
	}
	return s.createPostedRefundTaskTX(
		ctx, tx, refund, itemNos, taskNo, requestDigest, refundOperationNo, ident,
	)
}

// recoverPostedRefundReplay resolves a concurrent unique insert after the winner commits.
func recoverPostedRefundReplay(
	ctx context.Context,
	database gdb.DB,
	taskNo string,
	ident identity,
) (*taskRow, error) {
	existing, loadErr := loadTask(ctx, database, taskNo, false)
	if loadErr != nil || existing == nil {
		return nil, loadErr
	}
	replay, replayErr := loadReplay(ctx, database, taskNo, "record_refund", ident.digest)
	if replayErr != nil {
		return nil, replayErr
	}
	if replay == nil || replay.Fingerprint != ident.fingerprint {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return existing, nil
}

// createPostedRefundTaskTX resolves unique invoice relationships and persists immutable work.
func (s *serviceImpl) createPostedRefundTaskTX(
	ctx context.Context,
	tx gdb.TX,
	refund *taxinvoicecap.RefundOperation,
	itemNos []string,
	taskNo, requestDigest, refundOperationNo string,
	ident identity,
) (recordPostedRefundState, error) {
	candidates, loadErr := loadSourceCandidatesTX(ctx, tx, itemNos, refund.CustomerNo, refund.CurrencyCode)
	if loadErr != nil {
		return recordPostedRefundState{}, loadErr
	}
	byItem := make(map[string][]sourceCandidate, len(itemNos))
	for _, candidate := range candidates {
		if candidate.CustomerNo != refund.CustomerNo || candidate.CurrencyCode != refund.CurrencyCode {
			return recordPostedRefundState{}, errInconsistent
		}
		byItem[candidate.BillingInvoiceItemNo] = append(byItem[candidate.BillingInvoiceItemNo], candidate)
	}
	preparedRows, gross, resolved := prepareRefundAllocations(refund.Allocations, byItem)
	if gross <= 0 {
		return recordPostedRefundState{noCorrection: true}, nil
	}
	manualReason, primaryInvoice, net, tax := summarizePreparedRefund(preparedRows, resolved)
	now := s.now().UTC()
	_, insertErr := tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_red_letter_task
 (tenant_id, task_no, tax_invoice_no, request_sha256, refund_operation_no,
  reason_code, status_code, gross_amount_minor, net_amount_minor, tax_amount_minor,
  customer_confirmation_required, available_at, version, customer_no, currency_code,
  allocation_resolved, manual_reason_code)
VALUES (?, ?, ?, ?, ?, 'refund_completed', ?, ?, ?, ?, FALSE, ?, 1, ?, ?, ?, ?)`,
		idccap.DefaultTenantID, taskNo, primaryInvoice, requestDigest, refundOperationNo,
		StatusManualRequired, gross, net, tax, now, refund.CustomerNo, refund.CurrencyCode,
		resolved, manualReason)
	if insertErr != nil {
		return recordPostedRefundState{}, insertErr
	}
	for _, preparedRow := range preparedRows {
		for _, candidate := range preparedRow.rows {
			candidateNet := proportionalNet(candidate.AvailableGrossMinor, candidate.AvailableNetMinor, candidate.AvailableGrossMinor)
			if _, insertErr = tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_red_letter_candidate
 (tenant_id, candidate_no, task_no, refund_allocation_no, tax_invoice_no,
  tax_invoice_allocation_no, tax_invoice_item_no, requested_gross_minor,
  available_gross_minor, available_net_minor, available_tax_minor)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, idccap.DefaultTenantID,
				candidateNo(taskNo, preparedRow.refund.RefundAllocationNo, candidate.TaxInvoiceAllocationNo),
				taskNo, preparedRow.refund.RefundAllocationNo, candidate.TaxInvoiceNo,
				candidate.TaxInvoiceAllocationNo, candidate.TaxInvoiceItemNo, preparedRow.target,
				candidate.AvailableGrossMinor, candidateNet, candidate.AvailableGrossMinor-candidateNet); insertErr != nil {
				return recordPostedRefundState{}, insertErr
			}
		}
		if resolved {
			candidate := preparedRow.rows[0]
			if insertErr = insertRedAllocation(ctx, tx, taskNo, preparedRow.refund.RefundAllocationNo,
				candidate, preparedRow.target); insertErr != nil {
				return recordPostedRefundState{}, insertErr
			}
		}
	}
	if resolved {
		if updateErr := markInvoicesPending(ctx, tx, taskNo, now); updateErr != nil {
			return recordPostedRefundState{}, updateErr
		}
	}
	if insertErr = insertAction(ctx, tx, ident, taskNo, "record_refund", "system", nil,
		StatusManualRequired, StatusManualRequired, 1, 1, ""); insertErr != nil {
		return recordPostedRefundState{}, insertErr
	}
	if insertErr = s.systemAudit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "compliance.red-letter.record-refund", IdempotencyKey: ident.digest,
		OperationCode: "compliance.red-letter.record-refund", TargetType: "red_letter",
		TargetNo: taskNo, CustomerNo: refund.CustomerNo, ResultCode: "succeeded",
		ReasonCode: manualReason, RiskCode: "high", CorrelationNo: refundOperationNo,
		BeforeSnapshot: map[string]any{"status": "none"},
		AfterSnapshot:  map[string]any{"status": StatusManualRequired, "gross_amount_minor": gross, "allocation_resolved": resolved},
	}); insertErr != nil {
		return recordPostedRefundState{}, insertErr
	}
	output, loadErr := loadTaskTX(ctx, tx, taskNo, false)
	return recordPostedRefundState{output: output}, loadErr
}

// prepareRefundAllocations caps each refund allocation to still-correctable invoice capacity.
func prepareRefundAllocations(
	allocations []taxinvoicecap.RefundAllocation,
	byItem map[string][]sourceCandidate,
) ([]preparedRefundAllocation, int64, bool) {
	preparedRows := make([]preparedRefundAllocation, 0, len(allocations))
	var gross int64
	resolved := true
	for _, allocation := range allocations {
		rows := byItem[allocation.BillingInvoiceItemNo]
		var capacity int64
		for _, candidate := range rows {
			capacity += candidate.AvailableGrossMinor
		}
		target := min(allocation.AmountMinor, capacity)
		if target <= 0 {
			continue
		}
		if len(rows) != 1 {
			resolved = false
		}
		gross += target
		preparedRows = append(preparedRows, preparedRefundAllocation{refund: allocation, rows: rows, target: target})
	}
	return preparedRows, gross, resolved
}

// summarizePreparedRefund calculates immutable amounts and the optional unique invoice.
func summarizePreparedRefund(
	preparedRows []preparedRefundAllocation,
	resolved bool,
) (string, *string, int64, int64) {
	if !resolved {
		return ManualReasonAllocationSelection, nil, 0, 0
	}
	var net, tax int64
	invoiceNos := make(map[string]struct{})
	for _, preparedRow := range preparedRows {
		candidate := preparedRow.rows[0]
		lineNet := proportionalNet(preparedRow.target, candidate.AvailableNetMinor, candidate.AvailableGrossMinor)
		net += lineNet
		tax += preparedRow.target - lineNet
		invoiceNos[candidate.TaxInvoiceNo] = struct{}{}
	}
	var primaryInvoice *string
	if len(invoiceNos) == 1 {
		for invoiceNo := range invoiceNos {
			primaryInvoice = stringPointer(invoiceNo)
		}
	}
	return ManualReasonProviderUnavailable, primaryInvoice, net, tax
}

func insertRedAllocation(
	ctx context.Context,
	tx gdb.TX,
	taskNo, refundAllocationNo string,
	candidate sourceCandidate,
	gross int64,
) error {
	net := proportionalNet(gross, candidate.AvailableNetMinor, candidate.AvailableGrossMinor)
	_, err := tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_red_letter_allocation
 (tenant_id, allocation_no, task_no, tax_invoice_no, tax_invoice_allocation_no,
  tax_invoice_item_no, refund_allocation_no, gross_amount_minor, net_amount_minor, tax_amount_minor)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, idccap.DefaultTenantID,
		allocationNo(taskNo, refundAllocationNo, candidate.TaxInvoiceAllocationNo), taskNo,
		candidate.TaxInvoiceNo, candidate.TaxInvoiceAllocationNo, candidate.TaxInvoiceItemNo,
		refundAllocationNo, gross, net, gross-net)
	if err != nil {
		return err
	}
	result, err := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_tax_invoice_allocation
SET refunded_gross_minor = refunded_gross_minor + ?, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = ? AND allocation_no = ?
  AND refunded_gross_minor + ? <= allocated_gross_minor`,
		gross, idccap.DefaultTenantID, candidate.TaxInvoiceAllocationNo, gross)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func markInvoicesPending(ctx context.Context, tx gdb.TX, taskNo string, now time.Time) error {
	_, err := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_tax_invoice AS invoice
SET status_code = 'red_letter_pending', version = version + 1, updated_at = ?
WHERE invoice.tenant_id = ?
  AND invoice.status_code IN ('issued', 'delivered', 'partially_red_lettered')
  AND EXISTS (
      SELECT 1 FROM plugin_idc_compliance_red_letter_allocation allocation
      WHERE allocation.tenant_id = invoice.tenant_id AND allocation.task_no = ?
        AND allocation.tax_invoice_no = invoice.tax_invoice_no
  )`, now, idccap.DefaultTenantID, taskNo)
	return err
}

func insertAction(
	ctx context.Context,
	tx gdb.TX,
	ident identity,
	taskNo, action, actorType string,
	actorUserID *int,
	beforeStatus, afterStatus string,
	beforeVersion, afterVersion int64,
	evidenceNo string,
) error {
	_, err := tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_red_letter_action
 (tenant_id, action_no, task_no, action_code, idempotency_digest,
  request_fingerprint, actor_type_code, actor_user_id, before_status_code,
  after_status_code, before_version, after_version, evidence_no)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, idccap.DefaultTenantID,
		ident.actionNo, taskNo, action, ident.digest, ident.fingerprint, actorType,
		actorUserID, beforeStatus, afterStatus, beforeVersion, afterVersion, stringPointer(evidenceNo))
	return err
}
