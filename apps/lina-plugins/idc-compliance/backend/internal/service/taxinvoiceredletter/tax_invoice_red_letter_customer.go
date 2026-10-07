// This file implements ownership-only customer views and authoritative rechecks.

package taxinvoiceredletter

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// GetCustomer returns only the current customer's own safe task projection.
func (s *serviceImpl) GetCustomer(ctx context.Context, taskNo string) (*Task, error) {
	taskNo = strings.TrimSpace(taskNo)
	if !businessNoPattern.MatchString(taskNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !businessNoPattern.MatchString(subject.CustomerNo) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	row, err := loadTask(ctx, s.database, taskNo, false)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil || row.CustomerNo == nil || *row.CustomerNo != subject.CustomerNo {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	task, err := projectTask(ctx, s.database, row)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row.StatusCode == StatusAwaitingConfirmation && row.ProviderCode != nil && row.ExternalTaskRef != nil {
		if actionURL, actionErr := s.gateway.CustomerAction(ctx, *row.ProviderCode, *row.ExternalTaskRef); actionErr == nil {
			task.CustomerActionURL = actionURL
		}
	}
	return task, nil
}

// RecheckCustomer records the customer's request and advances only from an
// authoritative adapter result. A click or browser return never self-confirms.
func (s *serviceImpl) RecheckCustomer(ctx context.Context, input RecheckInput) (*Task, error) {
	input.TaskNo = strings.TrimSpace(input.TaskNo)
	if err := validateTaskInput(input.TaskNo, input.ExpectedVersion); err != nil {
		return nil, err
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !businessNoPattern.MatchString(subject.CustomerNo) || subject.ActorUserID <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	ident, err := actionIdentity("customer_recheck", input.IdempotencyKey, input.TaskNo)
	if err != nil {
		return nil, err
	}
	row, err := loadTask(ctx, s.database, input.TaskNo, false)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil || row.CustomerNo == nil || *row.CustomerNo != subject.CustomerNo {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	replay, replayErr := loadReplay(ctx, s.database, input.TaskNo, "customer_recheck", ident.digest)
	if replayErr != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if replay != nil {
		if replay.Fingerprint != ident.fingerprint {
			return nil, bizerr.NewCode(CodeConflict)
		}
		task, projectErr := projectTask(ctx, s.database, row)
		if projectErr != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		return task, nil
	}
	if row.Version != input.ExpectedVersion || row.StatusCode != StatusAwaitingConfirmation ||
		row.ProviderCode == nil || row.ExternalTaskRef == nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	result, err := s.gateway.Recheck(ctx, *row.ProviderCode, *row.ExternalTaskRef)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	var output *taskRow
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var transactionErr error
		output, transactionErr = s.recheckCustomerTX(
			ctx, tx, input, ident, subject.CustomerNo, subject.ActorUserID, result,
		)
		return transactionErr
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	task, err := projectTask(ctx, s.database, output)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	task.CustomerActionURL = result.CustomerActionURL
	return task, nil
}

// recheckCustomerTX records one authoritative supplier recheck atomically.
func (s *serviceImpl) recheckCustomerTX(
	ctx context.Context,
	tx gdb.TX,
	input RecheckInput,
	ident identity,
	customerNo string,
	actorID int,
	authoritative AuthoritativeResult,
) (*taskRow, error) {
	locked, loadErr := loadTaskTX(ctx, tx, input.TaskNo, true)
	if loadErr != nil {
		return nil, loadErr
	}
	if locked == nil || locked.CustomerNo == nil || *locked.CustomerNo != customerNo {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	replay, replayErr := loadReplayTX(ctx, tx, input.TaskNo, "customer_recheck", ident.digest)
	if replayErr != nil {
		return nil, replayErr
	}
	if replay != nil {
		if replay.Fingerprint != ident.fingerprint {
			return nil, bizerr.NewCode(CodeConflict)
		}
		return locked, nil
	}
	if locked.Version != input.ExpectedVersion || locked.StatusCode != StatusAwaitingConfirmation {
		return nil, bizerr.NewCode(CodeConflict)
	}
	var (
		now          = s.now().UTC()
		afterStatus  = locked.StatusCode
		manualReason = ""
		confirmedAt  any
	)
	if authoritative.Confirmed {
		confirmedAt = now
		afterStatus = "processing"
	}
	if authoritative.ResultUnknown {
		afterStatus = StatusManualRequired
		manualReason = "provider_result_unknown"
	}
	if authoritative.Completed {
		// Supplier completion must carry immutable evidence and is finalized by
		// the same allocation update path as manual evidence. Task 10.11.e wires it.
		if !digestPattern.MatchString(authoritative.EvidenceSHA256) || authoritative.EvidenceReference == "" {
			return nil, errInconsistent
		}
		afterStatus = "processing"
	}
	updateResult, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_red_letter_task
SET customer_recheck_requested_at = ?, customer_confirmed_at = COALESCE(customer_confirmed_at, ?),
    status_code = ?, manual_reason_code = ?, version = version + 1, updated_at = ?
WHERE tenant_id = ? AND task_no = ? AND version = ? AND status_code = ?`,
		now, confirmedAt, afterStatus, manualReason, now, idccap.DefaultTenantID,
		input.TaskNo, input.ExpectedVersion, StatusAwaitingConfirmation)
	if updateErr != nil {
		return nil, updateErr
	}
	if affected, affectedErr := updateResult.RowsAffected(); affectedErr != nil || affected != 1 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if updateErr = insertAction(ctx, tx, ident, input.TaskNo, "customer_recheck", "customer", &actorID,
		locked.StatusCode, afterStatus, locked.Version, locked.Version+1, ""); updateErr != nil {
		return nil, updateErr
	}
	if updateErr = s.customerAudit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "compliance.red-letter.customer-recheck", IdempotencyKey: ident.digest,
		OperationCode: "compliance.red-letter.customer-recheck", TargetType: "red_letter",
		TargetNo: input.TaskNo, CustomerNo: customerNo, ResultCode: "succeeded",
		ReasonCode: "authoritative_recheck", RiskCode: "medium",
		BeforeSnapshot: map[string]any{"status": locked.StatusCode, "version": locked.Version},
		AfterSnapshot:  map[string]any{"status": afterStatus, "version": locked.Version + 1, "confirmed": authoritative.Confirmed},
	}); updateErr != nil {
		return nil, updateErr
	}
	return loadTaskTX(ctx, tx, input.TaskNo, false)
}
