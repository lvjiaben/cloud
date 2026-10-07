// This file implements separately authorized staff actions for red-letter tasks.

package taxinvoiceredletter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

// GetAdmin returns one bounded task after the independent query permission.
func (s *serviceImpl) GetAdmin(ctx context.Context, taskNo string) (*Task, error) {
	if !businessNoPattern.MatchString(taskNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.management.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionQuery, TargetNo: taskNo,
	}); err != nil {
		return nil, err
	}
	row, err := loadTask(ctx, s.database, taskNo, false)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	task, err := projectTask(ctx, s.database, row)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return task, nil
}

// ResolveAllocation records the full explicit plan for an ambiguous refund.
func (s *serviceImpl) ResolveAllocation(ctx context.Context, input ResolveInput) (*Task, error) {
	input, ident, err := normalizeSelections(input)
	if err != nil {
		return nil, err
	}
	actor, err := s.management.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionHandle, TargetNo: input.TaskNo,
	})
	if err != nil {
		return nil, err
	}
	var output *taskRow
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var transactionErr error
		output, transactionErr = s.resolveAllocationTX(ctx, tx, input, ident, actor)
		return transactionErr
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return projectTask(ctx, s.database, output)
}

// resolveAllocationTX validates and persists one explicit allocation plan under a task lock.
func (s *serviceImpl) resolveAllocationTX(
	ctx context.Context,
	tx gdb.TX,
	input ResolveInput,
	ident identity,
	actor int,
) (*taskRow, error) {
	row, loadErr := loadTaskTX(ctx, tx, input.TaskNo, true)
	if loadErr != nil {
		return nil, loadErr
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	replay, replayErr := loadReplayTX(ctx, tx, input.TaskNo, "resolve_allocation", ident.digest)
	if replayErr != nil {
		return nil, replayErr
	}
	if replay != nil {
		if replay.Fingerprint != ident.fingerprint {
			return nil, bizerr.NewCode(CodeConflict)
		}
		return row, nil
	}
	if row.Version != input.ExpectedVersion || row.StatusCode != StatusManualRequired ||
		row.AllocationResolved || row.ManualReasonCode != ManualReasonAllocationSelection {
		return nil, bizerr.NewCode(CodeConflict)
	}
	candidates, loadErr := loadCandidatesTX(ctx, tx, input.TaskNo)
	if loadErr != nil || len(candidates) == 0 {
		return nil, errInconsistent
	}
	candidateMap := make(map[string]candidateRow, len(candidates))
	required := make(map[string]int64)
	for _, candidate := range candidates {
		key := candidate.RefundAllocationNo + "\x00" + candidate.TaxInvoiceAllocationNo
		candidateMap[key] = candidate
		required[candidate.RefundAllocationNo] = candidate.RequestedGrossMinor
	}
	selected := make(map[string]int64)
	invoiceNos := make(map[string]struct{})
	var gross, net, tax int64
	for _, selection := range input.Selections {
		key := selection.RefundAllocationNo + "\x00" + selection.TaxInvoiceAllocationNo
		candidate, exists := candidateMap[key]
		if !exists || selection.GrossAmountMinor > candidate.AvailableGrossMinor {
			return nil, bizerr.NewCode(CodeConflict)
		}
		selected[selection.RefundAllocationNo] += selection.GrossAmountMinor
		lineNet := proportionalNet(selection.GrossAmountMinor, candidate.AvailableNetMinor, candidate.AvailableGrossMinor)
		gross += selection.GrossAmountMinor
		net += lineNet
		tax += selection.GrossAmountMinor - lineNet
		invoiceNos[candidate.TaxInvoiceNo] = struct{}{}
	}
	for refundNo, amount := range required {
		if selected[refundNo] != amount {
			return nil, bizerr.NewCode(CodeConflict)
		}
	}
	if gross != row.GrossAmountMinor || gross != net+tax {
		return nil, bizerr.NewCode(CodeConflict)
	}
	for _, selection := range input.Selections {
		candidate := candidateMap[selection.RefundAllocationNo+"\x00"+selection.TaxInvoiceAllocationNo]
		if insertErr := insertRedAllocation(ctx, tx, input.TaskNo, selection.RefundAllocationNo,
			sourceCandidate{
				TaxInvoiceNo:           candidate.TaxInvoiceNo,
				TaxInvoiceAllocationNo: candidate.TaxInvoiceAllocationNo,
				TaxInvoiceItemNo:       candidate.TaxInvoiceItemNo,
				AvailableGrossMinor:    candidate.AvailableGrossMinor,
				AvailableNetMinor:      candidate.AvailableNetMinor,
				AvailableTaxMinor:      candidate.AvailableTaxMinor,
			}, selection.GrossAmountMinor); insertErr != nil {
			return nil, insertErr
		}
	}
	var primary *string
	if len(invoiceNos) == 1 {
		for invoiceNo := range invoiceNos {
			primary = stringPointer(invoiceNo)
		}
	}
	now := s.now().UTC()
	result, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_red_letter_task
SET tax_invoice_no = ?, allocation_resolved = TRUE,
    net_amount_minor = ?, tax_amount_minor = ?,
    manual_reason_code = ?, version = version + 1, updated_at = ?
WHERE tenant_id = ? AND task_no = ? AND version = ?
  AND status_code = ? AND allocation_resolved = FALSE
  AND manual_reason_code = ?`, primary, net, tax, ManualReasonProviderUnavailable, now,
		idccap.DefaultTenantID, input.TaskNo, input.ExpectedVersion,
		StatusManualRequired, ManualReasonAllocationSelection)
	if updateErr != nil {
		return nil, updateErr
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if updateErr = markInvoicesPending(ctx, tx, input.TaskNo, now); updateErr != nil {
		return nil, updateErr
	}
	actorID := actor
	if updateErr = insertAction(ctx, tx, ident, input.TaskNo, "resolve_allocation", "staff", &actorID,
		StatusManualRequired, StatusManualRequired, row.Version, row.Version+1, ""); updateErr != nil {
		return nil, updateErr
	}
	if updateErr = s.management.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "compliance.red-letter.resolve-allocation", IdempotencyKey: ident.digest,
		OperationCode: "compliance.red-letter.resolve-allocation", TargetType: "red_letter",
		TargetNo: input.TaskNo, CustomerNo: *row.CustomerNo, ResultCode: "succeeded",
		ReasonCode: ManualReasonAllocationSelection, RiskCode: "high", RequiredPermission: PermissionHandle,
		BeforeSnapshot: map[string]any{"allocation_resolved": false, "version": row.Version},
		AfterSnapshot:  map[string]any{"allocation_resolved": true, "version": row.Version + 1, "gross_amount_minor": gross},
	}); updateErr != nil {
		return nil, updateErr
	}
	return loadTaskTX(ctx, tx, input.TaskNo, false)
}

// CompleteManual closes a resolved task only with immutable real-world evidence.
func (s *serviceImpl) CompleteManual(ctx context.Context, input ManualCompletionInput) (*Task, error) {
	input, ident, err := normalizeCompletion(input)
	if err != nil {
		return nil, err
	}
	actor, err := s.management.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionHandle, TargetNo: input.TaskNo,
	})
	if err != nil {
		return nil, err
	}
	var output *taskRow
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, loadErr := loadTaskTX(ctx, tx, input.TaskNo, true)
		if loadErr != nil {
			return loadErr
		}
		if row == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		replay, replayErr := loadReplayTX(ctx, tx, input.TaskNo, "manual_complete", ident.digest)
		if replayErr != nil {
			return replayErr
		}
		if replay != nil {
			if replay.Fingerprint != ident.fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			output = row
			return nil
		}
		if row.Version != input.ExpectedVersion || row.StatusCode != StatusManualRequired || !row.AllocationResolved {
			return bizerr.NewCode(CodeConflict)
		}
		earlierValue, earlierErr := tx.Ctx(ctx).GetValue(`
SELECT COUNT(*)
FROM plugin_idc_compliance_red_letter_task earlier
JOIN plugin_idc_compliance_red_letter_allocation earlier_allocation
  ON earlier_allocation.tenant_id = earlier.tenant_id AND earlier_allocation.task_no = earlier.task_no
JOIN plugin_idc_compliance_red_letter_allocation current_allocation
  ON current_allocation.tenant_id = earlier_allocation.tenant_id
 AND current_allocation.task_no = ?
 AND current_allocation.tax_invoice_no = earlier_allocation.tax_invoice_no
WHERE earlier.tenant_id = ? AND earlier.id < (
    SELECT id FROM plugin_idc_compliance_red_letter_task
    WHERE tenant_id = ? AND task_no = ?
) AND earlier.status_code NOT IN ('completed', 'canceled', 'failed')`,
			input.TaskNo, idccap.DefaultTenantID, idccap.DefaultTenantID, input.TaskNo)
		if earlierErr != nil {
			return earlierErr
		}
		if earlierValue.Int() > 0 {
			return bizerr.NewCode(CodeConflict)
		}
		now := s.now().UTC()
		evidenceDigest := sha256.Sum256([]byte(input.TaskNo + "\x00" + input.EvidenceReference + "\x00" + input.EvidenceSHA256))
		evidenceNo := "rle_" + hex.EncodeToString(evidenceDigest[:20])
		var objectRef, mediaType any
		var byteSize any
		if input.UploadedObjectRef != "" {
			objectRef, mediaType, byteSize = input.UploadedObjectRef, input.MediaType, input.ByteSize
		}
		if _, insertErr := tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_external_evidence
 (tenant_id, evidence_no, aggregate_type_code, aggregate_no,
  source_system_code, source_event_ref, evidence_type_code, payload_sha256,
  private_object_ref, media_type, byte_size, disposition_code, occurred_at)
VALUES (?, ?, 'red_letter', ?, 'manual_tax_red_letter', ?, 'manual_red_letter_completion', ?, ?, ?, ?, 'accepted', ?)`,
			idccap.DefaultTenantID, evidenceNo, input.TaskNo, input.TaskNo+":"+input.EvidenceReference,
			input.EvidenceSHA256, objectRef, mediaType, byteSize, input.OccurredAt.UTC()); insertErr != nil {
			return insertErr
		}
		var allocations []struct {
			TaxInvoiceNo           string `json:"tax_invoice_no"`
			TaxInvoiceAllocationNo string `json:"tax_invoice_allocation_no"`
			GrossAmountMinor       int64  `json:"gross_amount_minor"`
		}
		if loadErr = tx.Ctx(ctx).GetScan(&allocations, `
SELECT tax_invoice_no, tax_invoice_allocation_no, SUM(gross_amount_minor) AS gross_amount_minor
FROM plugin_idc_compliance_red_letter_allocation
WHERE tenant_id = ? AND task_no = ?
GROUP BY tax_invoice_no, tax_invoice_allocation_no
ORDER BY tax_invoice_no, tax_invoice_allocation_no`, idccap.DefaultTenantID, input.TaskNo); loadErr != nil || len(allocations) == 0 {
			return errInconsistent
		}
		invoiceNos := make(map[string]struct{})
		for _, allocation := range allocations {
			result, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_tax_invoice_allocation
SET red_lettered_gross_minor = red_lettered_gross_minor + ?, updated_at = ?
WHERE tenant_id = ? AND allocation_no = ?
  AND red_lettered_gross_minor + ? <= refunded_gross_minor`,
				allocation.GrossAmountMinor, now, idccap.DefaultTenantID,
				allocation.TaxInvoiceAllocationNo, allocation.GrossAmountMinor)
			if updateErr != nil {
				return updateErr
			}
			if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
				return bizerr.NewCode(CodeConflict)
			}
			invoiceNos[allocation.TaxInvoiceNo] = struct{}{}
		}
		sortedInvoices := make([]string, 0, len(invoiceNos))
		for invoiceNo := range invoiceNos {
			sortedInvoices = append(sortedInvoices, invoiceNo)
		}
		sort.Strings(sortedInvoices)
		for _, invoiceNo := range sortedInvoices {
			if _, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_tax_invoice AS invoice
SET status_code = CASE
      WHEN totals.red_lettered_minor = invoice.gross_amount_minor THEN 'red_lettered'
      ELSE 'partially_red_lettered'
    END,
    version = version + 1, updated_at = ?
FROM (
    SELECT tax_invoice_no, SUM(red_lettered_gross_minor) AS red_lettered_minor
    FROM plugin_idc_compliance_tax_invoice_allocation
    WHERE tenant_id = ? AND tax_invoice_no = ? GROUP BY tax_invoice_no
) totals
WHERE invoice.tenant_id = ? AND invoice.tax_invoice_no = totals.tax_invoice_no
  AND totals.red_lettered_minor > 0 AND totals.red_lettered_minor <= invoice.gross_amount_minor`,
				now, idccap.DefaultTenantID, invoiceNo, idccap.DefaultTenantID); updateErr != nil {
				return updateErr
			}
		}
		result, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_red_letter_task
SET status_code = 'completed', manual_reason_code = '', completion_evidence_no = ?,
    completed_at = ?, version = version + 1, updated_at = ?
WHERE tenant_id = ? AND task_no = ? AND version = ?
  AND status_code = ? AND allocation_resolved = TRUE`, evidenceNo, now, now,
			idccap.DefaultTenantID, input.TaskNo, input.ExpectedVersion, StatusManualRequired)
		if updateErr != nil {
			return updateErr
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		actorID := actor
		if updateErr = insertAction(ctx, tx, ident, input.TaskNo, "manual_complete", "staff", &actorID,
			row.StatusCode, StatusCompleted, row.Version, row.Version+1, evidenceNo); updateErr != nil {
			return updateErr
		}
		if updateErr = s.management.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.red-letter.manual-complete", IdempotencyKey: ident.digest,
			OperationCode: "compliance.red-letter.manual-complete", TargetType: "red_letter",
			TargetNo: input.TaskNo, CustomerNo: *row.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "manual_evidence_confirmed", RiskCode: "high", RequiredPermission: PermissionHandle,
			BeforeSnapshot: map[string]any{"status": row.StatusCode, "version": row.Version},
			AfterSnapshot:  map[string]any{"status": StatusCompleted, "version": row.Version + 1, "evidence_no": evidenceNo},
		}); updateErr != nil {
			return updateErr
		}
		output, loadErr = loadTaskTX(ctx, tx, input.TaskNo, false)
		return loadErr
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return projectTask(ctx, s.database, output)
}
