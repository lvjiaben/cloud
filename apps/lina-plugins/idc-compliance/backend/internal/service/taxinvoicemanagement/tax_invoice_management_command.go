// This file applies exact management transitions without contacting a supplier.
package taxinvoicemanagement

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type mutation struct {
	action          string
	permission      string
	taxInvoiceNo    string
	expectedVersion int64
	identity        actionIdentity
	invoiceType     string
	providerCode    string
	reasonCode      string
	internalNote    string
	capability      Capability
}

// Get returns one safe management projection after the exact query permission succeeds.
func (s *serviceImpl) Get(ctx context.Context, taxInvoiceNo string) (*Request, error) {
	if !businessNoPattern.MatchString(taxInvoiceNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorize(ctx, PermissionQuery, taxInvoiceNo); err != nil {
		return nil, err
	}
	capability := s.currentCapability(ctx)
	row, err := s.load(ctx, taxInvoiceNo, false)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return project(row, capability)
}

// Approve confirms the invoice type and moves a pending request to waiting submission.
func (s *serviceImpl) Approve(ctx context.Context, input ReviewInput) (*Result, error) {
	input, identity, err := normalizeReview(input)
	if err != nil {
		return nil, err
	}
	actor, err := s.authorize(ctx, PermissionReview, input.TaxInvoiceNo)
	if err != nil {
		return nil, err
	}
	capability := s.currentCapability(ctx)
	if !capabilitySupports(capability, input.InvoiceTypeCode) {
		return s.blocked(ctx, input.TaxInvoiceNo, input.ExpectedVersion, "approve", capability)
	}
	return s.mutate(ctx, actor, mutation{
		action: "approve", permission: PermissionReview, taxInvoiceNo: input.TaxInvoiceNo,
		expectedVersion: input.ExpectedVersion, identity: identity,
		invoiceType: input.InvoiceTypeCode, providerCode: capability.ProviderCode,
		capability: capability,
	})
}

// Reject closes a pending request and releases its reserved capacity.
func (s *serviceImpl) Reject(ctx context.Context, input CloseInput) (*Result, error) {
	input, identity, err := normalizeClose("reject", input)
	if err != nil {
		return nil, err
	}
	actor, err := s.authorize(ctx, PermissionReview, input.TaxInvoiceNo)
	if err != nil {
		return nil, err
	}
	return s.mutate(ctx, actor, mutation{
		action: "reject", permission: PermissionReview, taxInvoiceNo: input.TaxInvoiceNo,
		expectedVersion: input.ExpectedVersion, identity: identity,
		reasonCode: input.ReasonCode, internalNote: input.InternalNote,
		capability: s.currentCapability(ctx),
	})
}

// Submit queues an approved request locally after rechecking provider capability.
func (s *serviceImpl) Submit(ctx context.Context, input SubmitInput) (*Result, error) {
	input, identity, err := normalizeSubmit(input)
	if err != nil {
		return nil, err
	}
	actor, err := s.authorize(ctx, PermissionSubmit, input.TaxInvoiceNo)
	if err != nil {
		return nil, err
	}
	capability := s.currentCapability(ctx)
	row, loadErr := s.load(ctx, input.TaxInvoiceNo, false)
	if loadErr != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if !capabilitySupports(capability, row.InvoiceTypeCode) {
		return s.blocked(ctx, input.TaxInvoiceNo, input.ExpectedVersion, "submit", capability)
	}
	return s.mutate(ctx, actor, mutation{
		action: "submit", permission: PermissionSubmit, taxInvoiceNo: input.TaxInvoiceNo,
		expectedVersion: input.ExpectedVersion, identity: identity,
		invoiceType: row.InvoiceTypeCode, providerCode: capability.ProviderCode,
		capability: capability,
	})
}

// Cancel safely closes only a request that is known not to have been issued.
func (s *serviceImpl) Cancel(ctx context.Context, input CloseInput) (*Result, error) {
	input, identity, err := normalizeClose("cancel", input)
	if err != nil {
		return nil, err
	}
	actor, err := s.authorize(ctx, PermissionCancel, input.TaxInvoiceNo)
	if err != nil {
		return nil, err
	}
	return s.mutate(ctx, actor, mutation{
		action: "cancel", permission: PermissionCancel, taxInvoiceNo: input.TaxInvoiceNo,
		expectedVersion: input.ExpectedVersion, identity: identity,
		reasonCode: input.ReasonCode, internalNote: input.InternalNote,
		capability: s.currentCapability(ctx),
	})
}

func (s *serviceImpl) blocked(
	ctx context.Context,
	taxInvoiceNo string,
	expectedVersion int64,
	action string,
	capability Capability,
) (*Result, error) {
	row, err := s.load(ctx, taxInvoiceNo, false)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if row.Version != expectedVersion || !transitionAllowed(action, row) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	request, err := project(row, capability)
	if err != nil {
		return nil, err
	}
	return &Result{Request: request}, nil
}

func (s *serviceImpl) currentCapability(ctx context.Context) Capability {
	capability, err := s.gate.Current(ctx)
	if err == nil {
		if normalized, valid := normalizeCapability(capability); valid {
			return normalized
		}
	}
	return Capability{
		ProviderCode: ProviderNuoNuo, Ready: false, ReasonCode: CapabilityCheckUnavailable,
	}
}

func (s *serviceImpl) mutate(
	ctx context.Context,
	actor int,
	input mutation,
) (*Result, error) {
	now := s.now().UTC()
	var output *requestRow
	reused := false
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, lockErr := tx.Ctx(ctx).GetValue(
			"SELECT pg_advisory_xact_lock(hashtextextended(?, 0))",
			"tax-invoice-action:"+input.identity.ActionNo,
		); lockErr != nil {
			return lockErr
		}
		replay, loadErr := loadActionTX(
			ctx, tx, input.taxInvoiceNo, input.action, input.identity.IdempotencyDigest,
		)
		if loadErr != nil {
			return loadErr
		}
		row, loadErr := loadTX(ctx, tx, input.taxInvoiceNo)
		if loadErr != nil {
			return loadErr
		}
		if row == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if replay != nil {
			if replay.RequestFingerprint != input.identity.Fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			applyReplay(row, replay)
			output = row
			reused = true
			return nil
		}
		if row.Version != input.expectedVersion || !transitionAllowed(input.action, row) {
			return bizerr.NewCode(CodeConflict)
		}
		beforeStatus := row.StatusCode
		beforeVersion := row.Version
		afterStatus, updateErr := applyUpdate(ctx, tx, actor, now, input, row)
		if updateErr != nil {
			return updateErr
		}
		if _, updateErr = tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_tax_invoice_action
    (tenant_id, action_no, tax_invoice_no, action_code, idempotency_digest,
     request_fingerprint, actor_user_id, before_status_code, after_status_code,
     before_version, after_version, invoice_type_code, provider_code, reason_code,
     internal_note, capability_reason_code)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			idccap.DefaultTenantID, input.identity.ActionNo, input.taxInvoiceNo, input.action,
			input.identity.IdempotencyDigest, input.identity.Fingerprint, actor,
			beforeStatus, afterStatus, beforeVersion, beforeVersion+1,
			row.InvoiceTypeCode, stringValue(row.ProviderCode), input.reasonCode,
			input.internalNote, input.capability.ReasonCode); updateErr != nil {
			return updateErr
		}
		if updateErr = s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.tax-invoice." + input.action,
			IdempotencyKey:   input.identity.IdempotencyDigest,
			OperationCode:    "compliance.tax-invoice." + input.action,
			TargetType:       "tax_invoice", TargetNo: input.taxInvoiceNo,
			ResultCode: "succeeded", ReasonCode: auditReason(input.action, input.reasonCode),
			RiskCode: "high", RequiredPermission: input.permission,
			BeforeSnapshot: map[string]any{"status": beforeStatus, "version": beforeVersion},
			AfterSnapshot: map[string]any{
				"status": afterStatus, "version": beforeVersion + 1,
				"invoice_type": row.InvoiceTypeCode, "provider": stringValue(row.ProviderCode),
			},
		}); updateErr != nil {
			return updateErr
		}
		output = row
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	request, err := project(output, input.capability)
	if err != nil {
		return nil, err
	}
	return &Result{Request: request, Applied: true, Reused: reused}, nil
}

func transitionAllowed(action string, row *requestRow) bool {
	if row == nil {
		return false
	}
	switch action {
	case "approve":
		return row.StatusCode == StatusPendingReview &&
			row.InvoiceTypeCode == InvoiceTypePendingConfirmation
	case "reject":
		return row.StatusCode == StatusPendingReview
	case "submit":
		return row.StatusCode == StatusApprovedPendingSubmission &&
			row.IssuedAt == nil && row.ExternalInvoiceRef == nil
	case "cancel":
		if row.IssuedAt != nil {
			return false
		}
		if row.StatusCode == StatusFailed {
			return row.NotIssuedConfirmedAt != nil
		}
		return (row.StatusCode == StatusPendingReview ||
			row.StatusCode == StatusApprovedPendingSubmission) && row.ExternalInvoiceRef == nil
	default:
		return false
	}
}

func applyUpdate(
	ctx context.Context,
	tx gdb.TX,
	actor int,
	now time.Time,
	input mutation,
	row *requestRow,
) (string, error) {
	var result sql.Result
	var err error
	switch input.action {
	case "approve":
		result, err = tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_tax_invoice
SET status_code = ?, invoice_type_code = ?, provider_code = ?, reviewed_by_user_id = ?,
    reviewed_at = ?, decision_reason_code = '', decision_internal_note = '',
    version = version + 1, updated_at = ?
WHERE tenant_id = ? AND tax_invoice_no = ? AND version = ? AND status_code = ?`,
			StatusApprovedPendingSubmission, input.invoiceType, input.providerCode, actor, now, now,
			idccap.DefaultTenantID, input.taxInvoiceNo, input.expectedVersion, StatusPendingReview)
		row.StatusCode = StatusApprovedPendingSubmission
		row.InvoiceTypeCode = input.invoiceType
		row.ProviderCode = &input.providerCode
		row.ReviewedAt = &now
	case "reject":
		result, err = tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_tax_invoice
SET status_code = ?, decision_reason_code = ?, decision_internal_note = ?,
    reviewed_by_user_id = ?, reviewed_at = ?, version = version + 1, updated_at = ?
WHERE tenant_id = ? AND tax_invoice_no = ? AND version = ? AND status_code = ?`,
			StatusRejected, input.reasonCode, input.internalNote, actor, now, now,
			idccap.DefaultTenantID, input.taxInvoiceNo, input.expectedVersion, StatusPendingReview)
		row.StatusCode = StatusRejected
		row.DecisionReasonCode = input.reasonCode
		row.DecisionInternalNote = input.internalNote
		row.ReviewedAt = &now
	case "submit":
		result, err = tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_tax_invoice
SET status_code = ?, provider_code = ?, version = version + 1, updated_at = ?
WHERE tenant_id = ? AND tax_invoice_no = ? AND version = ? AND status_code = ?
  AND issued_at IS NULL AND external_invoice_ref IS NULL`,
			StatusQueued, input.providerCode, now, idccap.DefaultTenantID,
			input.taxInvoiceNo, input.expectedVersion, StatusApprovedPendingSubmission)
		row.StatusCode = StatusQueued
		row.ProviderCode = &input.providerCode
	case "cancel":
		result, err = tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_tax_invoice
SET status_code = ?, decision_reason_code = ?, decision_internal_note = ?,
    canceled_at = ?, version = version + 1, updated_at = ?
WHERE tenant_id = ? AND tax_invoice_no = ? AND version = ?
  AND issued_at IS NULL
  AND ((status_code IN (?, ?) AND external_invoice_ref IS NULL)
       OR (status_code = ? AND provider_not_issued_confirmed_at IS NOT NULL))`,
			StatusCanceled, input.reasonCode, input.internalNote, now, now,
			idccap.DefaultTenantID, input.taxInvoiceNo, input.expectedVersion,
			StatusPendingReview, StatusApprovedPendingSubmission, StatusFailed)
		row.StatusCode = StatusCanceled
		row.DecisionReasonCode = input.reasonCode
		row.DecisionInternalNote = input.internalNote
	default:
		return "", bizerr.NewCode(CodeInvalid)
	}
	if err != nil {
		return "", err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return "", bizerr.NewCode(CodeConflict)
	}
	row.Version++
	return row.StatusCode, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func auditReason(action, reason string) string {
	if reason != "" {
		return reason
	}
	return "tax_invoice_" + action + "_succeeded"
}
