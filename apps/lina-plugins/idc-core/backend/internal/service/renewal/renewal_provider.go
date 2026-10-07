package renewal

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/renewalownercap"
)

const (
	providerRenewalActor   = "renewal-provider-worker"
	maximumProviderBatch   = 64
	providerCapabilityCode = "renewal.capability_unavailable"
)

// ProviderBatch is one bounded trusted-worker pass over provider-confirmed renewals.
type ProviderBatch struct {
	Selected     int
	Dispatched   int
	Reconciled   int
	ManualReview int
	Failures     int
}

type providerOperationRow struct {
	OperationNo                string     `orm:"operation_no"`
	QuoteNo                    string     `orm:"quote_no"`
	InvoiceNo                  string     `orm:"invoice_no"`
	CustomerNo                 string     `orm:"customer_no"`
	EntitlementNo              string     `orm:"entitlement_no"`
	CurrencyCode               string     `orm:"currency_code"`
	PriceSnapshotNo            string     `orm:"price_snapshot_no"`
	StatusCode                 string     `orm:"status_code"`
	ExecutionModeCode          string     `orm:"execution_mode_code"`
	OwnerID                    string     `orm:"owner_id"`
	PluginID                   string     `orm:"plugin_id"`
	PluginReleaseVersion       string     `orm:"plugin_release_version"`
	ContractVersion            string     `orm:"contract_version"`
	RenewalMethod              string     `orm:"renewal_method"`
	ProviderTaskNo             *string    `orm:"provider_task_no"`
	CapabilityEvidenceNo       string     `orm:"capability_evidence_no"`
	CapabilityDigest           string     `orm:"capability_evidence_digest"`
	ExpectedEntitlementVersion int64      `orm:"expected_entitlement_version"`
	OperationVersion           int64      `orm:"operation_version"`
	LastErrorCode              string     `orm:"last_error_code"`
	EntitlementVersion         int64      `orm:"entitlement_version"`
	EntitlementRenewalMode     string     `orm:"entitlement_renewal_mode"`
	EntitlementContractNo      *string    `orm:"entitlement_contract_no"`
	EntitlementContractDigest  *string    `orm:"entitlement_contract_digest"`
	EntitlementContractExpires *time.Time `orm:"entitlement_contract_expires_at"`
	CurrentExpiresAt           *time.Time `orm:"current_expires_at"`
	OriginalExpiresAt          time.Time  `orm:"original_expires_at"`
	ProjectedExpiresAt         time.Time  `orm:"projected_expires_at"`
	TotalAmountMinor           int64      `orm:"total_amount_minor"`
	PaymentDueAt               time.Time  `orm:"payment_due_at"`
	MaximumSequence            int        `orm:"maximum_sequence"`
}

func (s *serviceImpl) RunProviderOperations(
	ctx context.Context,
	limit int,
) (*ProviderBatch, error) {
	if s == nil || s.database == nil || s.owners == nil || ctx == nil || ctx.Err() != nil ||
		limit <= 0 || limit > maximumProviderBatch {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	operationRows := make([]struct {
		OperationNo string `orm:"operation_no"`
	}, 0, limit)
	if err := s.database.GetScan(ctx, &operationRows, `
SELECT operation_no
FROM plugin_idc_core_business_operation
WHERE tenant_id = 0 AND operation_type_code = 'renewal'
  AND execution_mode_code = 'provider_confirmed'
  AND status_code IN ('ready', 'executing')
ORDER BY updated_at ASC, id ASC
LIMIT ?`, limit); err != nil {
		return nil, normalizeError(gerror.Wrap(err, "list Provider renewal work"))
	}
	report := &ProviderBatch{Selected: len(operationRows)}
	for _, operationRow := range operationRows {
		if ctx.Err() != nil {
			return report, normalizeError(ctx.Err())
		}
		row, err := loadProviderOperation(ctx, s.database, operationRow.OperationNo, false)
		if err != nil || row == nil {
			report.Failures++
			continue
		}
		var result *Operation
		if row.StatusCode == "ready" {
			result, err = s.DispatchProviderOperation(ctx, operationRow.OperationNo)
			if err == nil && result != nil && result.Status == "executing" {
				report.Dispatched++
			}
		} else {
			result, err = s.ReconcileProviderOperation(ctx, operationRow.OperationNo)
			if err == nil {
				report.Reconciled++
			}
		}
		if err != nil {
			report.Failures++
			continue
		}
		if result != nil && result.Status == "manual_review" {
			report.ManualReview++
		}
	}
	if report.Failures != 0 {
		return report, bizerr.NewCode(CodeUnavailable)
	}
	return report, nil
}

func (s *serviceImpl) DispatchProviderOperation(
	ctx context.Context,
	operationNo string,
) (*Operation, error) {
	if s == nil || s.database == nil || s.owners == nil || ctx == nil || ctx.Err() != nil ||
		operationNo != strings.TrimSpace(operationNo) || !validReference(operationNo, 40) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	row, err := loadProviderOperation(ctx, s.database, operationNo, false)
	if err != nil {
		return nil, normalizeError(err)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if row.StatusCode == "executing" {
		return projectProviderOperation(row, true), nil
	}
	if terminalProviderOperation(row.StatusCode) {
		return projectProviderOperation(row, true), nil
	}
	if row.StatusCode != "ready" || validateProviderOperationFacts(row) != nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	now, err := renewalDatabaseNow(ctx, s.database)
	if err != nil {
		return nil, normalizeError(err)
	}
	if row.EntitlementContractExpires == nil || !row.EntitlementContractExpires.After(now) {
		return s.moveProviderOperationToManualReview(ctx, row.OperationNo, providerCapabilityCode, "", true)
	}
	taskRequest := providerTaskRequest(row)
	owner, available := s.owners.Resolve(row.OwnerID)
	if !available {
		return s.moveProviderOperationToManualReview(ctx, row.OperationNo, providerCapabilityCode, "", true)
	}
	task, err := owner.CreateTask(ctx, taskRequest)
	if err != nil {
		if errors.Is(err, renewalownercap.ErrOwnerUnavailable) ||
			errors.Is(err, renewalownercap.ErrTaskConflict) {
			return s.moveProviderOperationToManualReview(
				ctx, row.OperationNo, providerCapabilityCode, "", true,
			)
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if task == nil || renewalownercap.ValidateTask(*task) != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	attached, err := s.attachProviderTask(ctx, row.OperationNo, task)
	if err != nil {
		return nil, normalizeError(err)
	}
	if task.Status == renewalownercap.TaskStatusPending ||
		task.Status == renewalownercap.TaskStatusRunning {
		return attached, nil
	}
	return s.ReconcileProviderOperation(ctx, row.OperationNo)
}

func (s *serviceImpl) ReconcileProviderOperation(
	ctx context.Context,
	operationNo string,
) (*Operation, error) {
	if s == nil || s.database == nil || s.owners == nil || ctx == nil || ctx.Err() != nil ||
		operationNo != strings.TrimSpace(operationNo) || !validReference(operationNo, 40) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	row, err := loadProviderOperation(ctx, s.database, operationNo, false)
	if err != nil {
		return nil, normalizeError(err)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if terminalProviderOperation(row.StatusCode) {
		return projectProviderOperation(row, true), nil
	}
	if row.StatusCode != "executing" || row.ProviderTaskNo == nil ||
		!validReference(*row.ProviderTaskNo, 64) || validateProviderOperationFacts(row) != nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	owner, available := s.owners.Resolve(row.OwnerID)
	if !available {
		return s.moveProviderOperationToManualReview(ctx, row.OperationNo, providerCapabilityCode, "", false)
	}
	task, err := owner.ReadTask(ctx, renewalownercap.TaskReadRequest{
		TaskNo: *row.ProviderTaskNo, Request: providerTaskRequest(row),
	})
	if err != nil {
		if errors.Is(err, renewalownercap.ErrTaskConflict) ||
			errors.Is(err, renewalownercap.ErrTaskUnavailable) {
			return s.moveProviderOperationToManualReview(
				ctx, row.OperationNo, "provider.task_unavailable", "", false,
			)
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if task == nil || renewalownercap.ValidateTask(*task) != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	switch task.Status {
	case renewalownercap.TaskStatusPending, renewalownercap.TaskStatusRunning:
		return projectProviderOperation(row, true), nil
	case renewalownercap.TaskStatusSucceeded:
		return s.finishProviderOperation(ctx, row.OperationNo, "succeeded", "", task.EvidenceDigest)
	case renewalownercap.TaskStatusFailed:
		return s.finishProviderOperation(
			ctx, row.OperationNo, "failed", "provider.task_failed", task.EvidenceDigest,
		)
	case renewalownercap.TaskStatusResultUnknown:
		return s.moveProviderOperationToManualReview(
			ctx, row.OperationNo, "provider.result_unknown", task.EvidenceDigest, false,
		)
	case renewalownercap.TaskStatusManualReview:
		return s.moveProviderOperationToManualReview(
			ctx, row.OperationNo, "provider.manual_review", task.EvidenceDigest, false,
		)
	default:
		return nil, bizerr.NewCode(CodeConflict)
	}
}

func providerTaskRequest(row *providerOperationRow) renewalownercap.TaskRequest {
	return renewalownercap.TaskRequest{
		Version: renewalownercap.ContractVersion, OwnerID: row.OwnerID,
		OperationNo: row.OperationNo, EntitlementNo: row.EntitlementNo,
		PriceSnapshotNo: row.PriceSnapshotNo, PluginID: row.PluginID,
		PluginReleaseVersion: row.PluginReleaseVersion, ContractVersion: row.ContractVersion,
		DeclarationDigest: row.CapabilityDigest, RenewalMethod: row.RenewalMethod,
		ExpectedEntitlementVersion: row.ExpectedEntitlementVersion,
		OriginalExpiresAt:          row.OriginalExpiresAt.UTC(), ProjectedExpiresAt: row.ProjectedExpiresAt.UTC(),
	}
}

func (s *serviceImpl) attachProviderTask(
	ctx context.Context,
	operationNo string,
	task *renewalownercap.Task,
) (*Operation, error) {
	var result *Operation
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := loadProviderOperation(ctx, s.database, operationNo, true)
		if err != nil {
			return err
		}
		if row == nil {
			return bizerr.NewCode(CodeConflict)
		}
		if row.StatusCode == "executing" && row.ProviderTaskNo != nil && *row.ProviderTaskNo == task.TaskNo {
			result = projectProviderOperation(row, true)
			return nil
		}
		if row.StatusCode != "ready" || row.ProviderTaskNo != nil || validateProviderOperationFacts(row) != nil {
			return bizerr.NewCode(CodeConflict)
		}
		now, err := renewalDatabaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_business_operation
SET status_code = 'executing', provider_task_no = ?, execution_started_at = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND operation_no = ? AND status_code = 'ready'
  AND version = ?`, task.TaskNo, now, row.OperationNo, row.OperationVersion)
		if err != nil {
			return gerror.Wrap(err, "attach Provider renewal task")
		}
		affected, err := updated.RowsAffected()
		if err != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		eventRow := providerEventRow(row)
		if err = s.insertOperationEvent(
			ctx, eventRow, row.MaximumSequence+1, "ready", "executing",
			"renewal.provider_task_created", task.EvidenceDigest,
			"system", providerRenewalActor, now,
		); err != nil {
			return err
		}
		row.StatusCode = "executing"
		row.ProviderTaskNo = &task.TaskNo
		result = projectProviderOperation(row, false)
		return nil
	})
	return result, err
}

func (s *serviceImpl) finishProviderOperation(
	ctx context.Context,
	operationNo string,
	status string,
	errorCode string,
	evidence string,
) (*Operation, error) {
	if status != "succeeded" && status != "failed" {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var result *Operation
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := loadProviderOperation(ctx, s.database, operationNo, true)
		if err != nil {
			return err
		}
		if row == nil {
			return bizerr.NewCode(CodeConflict)
		}
		if row.StatusCode == status {
			result = projectProviderOperation(row, true)
			return nil
		}
		if row.StatusCode != "executing" || row.ProviderTaskNo == nil ||
			validateProviderOperationFacts(row) != nil {
			return bizerr.NewCode(CodeConflict)
		}
		now, err := renewalDatabaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_business_operation
SET status_code = ?, last_error_code = ?, completed_at = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND operation_no = ? AND status_code = 'executing'
  AND provider_task_no = ? AND version = ?`, status, errorCode, now,
			row.OperationNo, *row.ProviderTaskNo, row.OperationVersion)
		if err != nil {
			return gerror.Wrap(err, "finish Provider renewal operation")
		}
		affected, err := updated.RowsAffected()
		if err != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if status == "succeeded" {
			serviceUpdate, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_service_entitlement
SET status_code = 'active', expires_at = ?, current_price_snapshot_no = ?,
    last_renewal_operation_no = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND entitlement_no = ? AND customer_no = ? AND version = ?
  AND expires_at = ? AND expires_at < ?`, row.ProjectedExpiresAt, row.PriceSnapshotNo,
				row.OperationNo,
				row.EntitlementNo, row.CustomerNo, row.ExpectedEntitlementVersion,
				row.OriginalExpiresAt, row.ProjectedExpiresAt)
			if updateErr != nil {
				return gerror.Wrap(updateErr, "converge Provider renewal entitlement")
			}
			affected, updateErr = serviceUpdate.RowsAffected()
			if updateErr != nil || affected != 1 {
				return bizerr.NewCode(CodeConflict)
			}
		}
		reason := "renewal.provider_failed"
		if status == "succeeded" {
			reason = "renewal.provider_succeeded"
		}
		if err = s.insertOperationEvent(
			ctx, providerEventRow(row), row.MaximumSequence+1, "executing", status,
			reason, evidence, "system", providerRenewalActor, now,
		); err != nil {
			return err
		}
		row.StatusCode = status
		row.LastErrorCode = errorCode
		result = projectProviderOperation(row, false)
		return nil
	})
	return result, err
}

func (s *serviceImpl) moveProviderOperationToManualReview(
	ctx context.Context,
	operationNo string,
	errorCode string,
	evidence string,
	fromReady bool,
) (*Operation, error) {
	var result *Operation
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := loadProviderOperation(ctx, s.database, operationNo, true)
		if err != nil {
			return err
		}
		if row == nil {
			return bizerr.NewCode(CodeConflict)
		}
		if row.StatusCode == "manual_review" {
			result = projectProviderOperation(row, true)
			return nil
		}
		expectedStatus := "executing"
		if fromReady {
			expectedStatus = "ready"
		}
		if row.StatusCode != expectedStatus || validateProviderOperationFacts(row) != nil {
			return bizerr.NewCode(CodeConflict)
		}
		now, err := renewalDatabaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_business_operation
SET status_code = 'manual_review', last_error_code = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND operation_no = ? AND status_code = ?
  AND version = ?`, errorCode, row.OperationNo, expectedStatus, row.OperationVersion)
		if err != nil {
			return gerror.Wrap(err, "move Provider renewal to manual review")
		}
		affected, err := updated.RowsAffected()
		if err != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if evidence == "" {
			evidence, err = contentDigest(struct {
				OperationNo, ErrorCode string
				At                     time.Time
			}{row.OperationNo, errorCode, now})
			if err != nil {
				return err
			}
		}
		if err = s.insertOperationEvent(
			ctx, providerEventRow(row), row.MaximumSequence+1, expectedStatus, "manual_review",
			"renewal.provider_manual_review", evidence,
			"system", providerRenewalActor, now,
		); err != nil {
			return err
		}
		row.StatusCode = "manual_review"
		row.LastErrorCode = errorCode
		result = projectProviderOperation(row, false)
		return nil
	})
	return result, err
}

func loadProviderOperation(
	ctx context.Context,
	database gdb.DB,
	operationNo string,
	lock bool,
) (*providerOperationRow, error) {
	query := `
SELECT operation.operation_no, operation.quote_no, operation.invoice_no,
       operation.customer_no, operation.subject_no AS entitlement_no, operation.currency_code,
       operation.price_snapshot_no, operation.status_code,
       operation.execution_mode_code, operation.resource_owner_code AS owner_id,
	   service.plugin_id,
       COALESCE(operation.channel_no,'') AS plugin_release_version,
       operation.pool_no AS contract_version,
       operation.capability_evidence_no AS renewal_method,
       operation.provider_task_no,
       operation.capability_evidence_no, operation.capability_evidence_digest,
       operation.expected_subject_version AS expected_entitlement_version, operation.version AS operation_version,
       operation.last_error_code,
       service.version AS entitlement_version,
       service.renewal_mode_code AS entitlement_renewal_mode,
       service.plugin_id||'@'||service.plugin_release_version AS entitlement_contract_no,
       service.declaration_digest AS entitlement_contract_digest,
       invoice.payment_due_at AS entitlement_contract_expires_at,
       service.expires_at AS current_expires_at,
       price.original_expires_at, price.projected_expires_at,
       invoice.total_amount_minor, invoice.payment_due_at,
       COALESCE(event.maximum_sequence, 0) AS maximum_sequence
FROM plugin_idc_core_business_operation operation
INNER JOIN plugin_idc_core_service_entitlement service
    ON service.tenant_id = operation.tenant_id
   AND service.entitlement_no = operation.subject_no
   AND service.customer_no = operation.customer_no
INNER JOIN plugin_idc_core_service_price_snapshot price
    ON price.tenant_id = operation.tenant_id
   AND price.price_snapshot_no = operation.price_snapshot_no
   AND price.subject_no = operation.subject_no
INNER JOIN plugin_idc_core_billing_invoice invoice
    ON invoice.tenant_id = operation.tenant_id
   AND invoice.invoice_no = operation.invoice_no
LEFT JOIN LATERAL (
    SELECT MAX(event_sequence) AS maximum_sequence
    FROM plugin_idc_core_business_operation_event item
    WHERE item.tenant_id = operation.tenant_id
      AND item.operation_no = operation.operation_no
) event ON TRUE
WHERE operation.tenant_id = 0 AND operation.operation_type_code = 'renewal'
  AND operation.operation_no = ?`
	if lock {
		query += " FOR UPDATE OF operation, service"
	}
	var row *providerOperationRow
	if err := database.GetScan(ctx, &row, query, operationNo); err != nil {
		return nil, gerror.Wrap(err, "load Provider renewal operation")
	}
	return row, nil
}

func validateProviderOperationFacts(row *providerOperationRow) error {
	if row == nil || row.ExecutionModeCode != renewalownercap.ModeProviderConfirmed ||
		row.OwnerID == "" || row.PluginID == "" || row.PluginReleaseVersion == "" || row.ContractVersion == "" ||
		row.RenewalMethod == "" || row.EntitlementVersion != row.ExpectedEntitlementVersion ||
		row.EntitlementRenewalMode != row.ExecutionModeCode ||
		row.EntitlementContractDigest == nil || *row.EntitlementContractDigest != row.CapabilityDigest ||
		row.CurrentExpiresAt == nil || !row.CurrentExpiresAt.Equal(row.OriginalExpiresAt) ||
		!row.ProjectedExpiresAt.After(row.OriginalExpiresAt) ||
		renewalownercap.ValidateTaskRequest(providerTaskRequest(row)) != nil {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func providerEventRow(row *providerOperationRow) *convergenceRow {
	return &convergenceRow{
		OperationNo: row.OperationNo, EntitlementNo: row.EntitlementNo,
		MaximumSequence: row.MaximumSequence,
	}
}

func projectProviderOperation(row *providerOperationRow, reused bool) *Operation {
	return &Operation{
		OperationNo: row.OperationNo, QuoteNo: row.QuoteNo, InvoiceNo: row.InvoiceNo,
		EntitlementNo: row.EntitlementNo, CurrencyCode: row.CurrencyCode,
		Status: row.StatusCode, RenewalMode: row.ExecutionModeCode,
		TotalAmountMinor: row.TotalAmountMinor, PaymentDueAt: row.PaymentDueAt.UTC(),
		Reused: reused,
	}
}

func terminalProviderOperation(status string) bool {
	return status == "succeeded" || status == "failed" || status == "manual_review" ||
		status == "cancelled"
}
