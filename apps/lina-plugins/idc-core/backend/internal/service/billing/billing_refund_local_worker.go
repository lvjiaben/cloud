// This file executes core-owned refund destinations without calling an
// external payment channel. It is registered as a bounded primary-node job.

package billing

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/outbox"
)

const maximumLocalRefundBatch = 100

// LocalRefundWorker posts balance credits and credit releases and sends
// manually owned sources to the high-priority reconciliation queue.
type LocalRefundWorker interface {
	Run(context.Context, int) (int, error)
}

type localRefundWorker struct {
	owner *channelPaymentOwnerService
}

var _ LocalRefundWorker = (*localRefundWorker)(nil)

// NewLocalRefundWorker constructs the core-owned refund execution worker.
func NewLocalRefundWorker(
	database gdb.DB,
	events outbox.Enqueuer,
	tasks asynctask.Service,
) (LocalRefundWorker, error) {
	if isNilFinanceDirectoryDependency(database) ||
		isNilFinanceDirectoryDependency(events) ||
		isNilFinanceDirectoryDependency(tasks) {
		return nil, gerror.New("local refund worker requires database, event, and task services")
	}
	ledger, err := newBalanceService(database, rand.Reader)
	if err != nil {
		return nil, err
	}
	return &localRefundWorker{owner: &channelPaymentOwnerService{
		database: database, ledger: ledger, events: events, tasks: tasks,
	}}, nil
}

// Run processes at most limit local commands. Each command owns a separate
// transaction so one malformed request cannot roll back already committed work.
func (w *localRefundWorker) Run(ctx context.Context, limit int) (int, error) {
	if w == nil || w.owner == nil || ctx == nil || ctx.Err() != nil ||
		limit < 1 || limit > maximumLocalRefundBatch {
		return 0, bizerr.NewCode(CodeRefundInvalid)
	}
	processed := 0
	for processed < limit {
		hadWork := false
		err := w.owner.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			row, loadErr := w.claimLocalRefundCommand(ctx)
			if loadErr != nil || row == nil {
				return loadErr
			}
			hadWork = true
			return w.executeLocalRefundCommand(ctx, row)
		})
		if err != nil {
			return processed, normalizeOnlineRefundError(err)
		}
		if !hadWork {
			break
		}
		processed++
	}
	return processed, nil
}

func (w *localRefundWorker) claimLocalRefundCommand(
	ctx context.Context,
) (*refundExecutionOwnerRow, error) {
	var row *refundExecutionOwnerRow
	err := w.owner.database.GetScan(ctx, &row, `
SELECT command.command_no,command.refund_no,command.refund_source_no,
       command.source_reference AS payment_attempt_no,command.currency_code,
       command.amount_minor,command.request_digest,command.phase_code AS phase,
       command.status_code AS status,command.attempt_count,
       request.status_code AS refund_status,request.customer_no,request.order_no,request.version,
	       source.execution_owner_code AS execution_owner
FROM plugin_idc_core_refund_execution_command command
JOIN plugin_idc_core_refund_source_allocation source
  ON source.refund_source_no=command.refund_source_no
JOIN plugin_idc_core_refund_request request
  ON request.refund_no=command.refund_no
WHERE command.tenant_id=0 AND command.status_code='pending'
  AND command.phase_code='execution'
  AND request.status_code IN ('approved','executing')
  AND (source.execution_owner_code IN ('core','manual')
       OR command.destination_code='same_currency_balance')
  AND NOT EXISTS (
      SELECT 1 FROM plugin_idc_core_refund_execution_command gate
      WHERE gate.refund_no=command.refund_no AND gate.phase_code='preflight'
        AND gate.status_code IN ('pending','claimed')
  )
  AND (command.destination_code='same_currency_balance' OR NOT EXISTS (
      SELECT 1 FROM plugin_idc_core_refund_execution_command replacement
      WHERE replacement.refund_source_no=command.refund_source_no
        AND replacement.destination_code='same_currency_balance'
  ))
  AND NOT EXISTS (
      SELECT 1
      FROM plugin_idc_core_refund_source_allocation payment_source
      JOIN plugin_idc_core_refund_execution_command payment_command
        ON payment_command.refund_source_no=payment_source.refund_source_no
      WHERE payment_source.refund_no=request.refund_no
        AND payment_source.execution_owner_code='payment'
        AND payment_command.destination_code='original_channel'
        AND payment_command.status_code<>'succeeded'
        AND NOT EXISTS (
            SELECT 1
            FROM plugin_idc_core_refund_execution_command replacement
            WHERE replacement.refund_source_no=payment_command.refund_source_no
              AND replacement.destination_code='same_currency_balance'
        )
  )
  AND NOT EXISTS (
      SELECT 1 FROM plugin_idc_core_refund_execution_command active
      WHERE active.refund_no=request.refund_no AND active.status_code='claimed'
  )
ORDER BY command.created_at,command.id
LIMIT 1 FOR UPDATE OF command,request SKIP LOCKED`)
	return row, err
}

func (w *localRefundWorker) executeLocalRefundCommand(
	ctx context.Context,
	row *refundExecutionOwnerRow,
) error {
	now, err := refundDatabaseNow(ctx, w.owner.database)
	if err != nil {
		return err
	}
	if row.RefundStatus == "approved" {
		updated, updateErr := w.owner.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_request
SET status_code='executing',version=version+1,updated_at=?
WHERE tenant_id=0 AND refund_no=? AND status_code='approved' AND version=?`,
			now, row.RefundNo, row.Version)
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireSingleRefundExecutionUpdate(updated); updateErr != nil {
			return updateErr
		}
		writer := &onlineRefundService{database: w.owner.database, random: rand.Reader}
		sequence, updateErr := writer.nextAccountRefundDecisionSequence(ctx, row.RefundNo)
		if updateErr != nil {
			return updateErr
		}
		if updateErr = writer.insertRefundDecision(
			ctx, row.RefundNo, sequence, "execution_started", "approved", "executing",
			"system", 0, "core_execution_started", "", row.RequestDigest, now,
		); updateErr != nil {
			return updateErr
		}
		row.RefundStatus = "executing"
	}
	if row.ExecutionOwner == "manual" {
		return w.markLocalRefundManual(ctx, row, now)
	}
	updated, err := w.owner.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_execution_command
SET status_code='succeeded',result_code='local_refund_posted',
    result_evidence_digest=request_digest,completed_at=?,updated_at=?
WHERE tenant_id=0 AND command_no=? AND status_code='pending' AND phase_code='execution'`,
		now, now, row.CommandNo)
	if err != nil {
		return err
	}
	if err = requireSingleRefundExecutionUpdate(updated); err != nil {
		return err
	}
	refundStatus, err := w.owner.postSuccessfulRefundSource(ctx, row, row.RequestDigest, now)
	if err != nil {
		return err
	}
	writer := &onlineRefundService{database: w.owner.database, random: rand.Reader}
	sequence, err := writer.nextAccountRefundDecisionSequence(ctx, row.RefundNo)
	if err != nil {
		return err
	}
	return writer.insertRefundDecision(
		ctx, row.RefundNo, sequence, "execution_succeeded", row.RefundStatus,
		refundStatus, "system", 0, "local_refund_posted", "", row.RequestDigest, now,
	)
}

func (w *localRefundWorker) markLocalRefundManual(
	ctx context.Context,
	row *refundExecutionOwnerRow,
	now time.Time,
) error {
	updated, err := w.owner.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_execution_command
SET status_code='manual_confirmation',result_code='manual_source_requires_reconciliation',
    result_evidence_digest=request_digest,completed_at=?,updated_at=?
WHERE tenant_id=0 AND command_no=? AND status_code='pending'`, now, now, row.CommandNo)
	if err != nil {
		return err
	}
	if err = requireSingleRefundExecutionUpdate(updated); err != nil {
		return err
	}
	requestUpdated, err := w.owner.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_request
SET status_code='manual_confirmation',version=version+1,updated_at=?
WHERE tenant_id=0 AND refund_no=? AND status_code='executing'`, now, row.RefundNo)
	if err != nil {
		return err
	}
	if err = requireSingleRefundExecutionUpdate(requestUpdated); err != nil {
		return err
	}
	if err = w.owner.createRefundReconciliationTask(
		ctx, row, "manual_source_requires_reconciliation", now,
	); err != nil {
		return err
	}
	writer := &onlineRefundService{database: w.owner.database, random: rand.Reader}
	sequence, err := writer.nextAccountRefundDecisionSequence(ctx, row.RefundNo)
	if err != nil {
		return err
	}
	return writer.insertRefundDecision(
		ctx, row.RefundNo, sequence, "manual_confirmation", row.RefundStatus,
		"manual_confirmation", "system", 0,
		"manual_source_requires_reconciliation", "", row.RequestDigest, now,
	)
}
