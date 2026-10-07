// This file exposes billing-owned durable refund commands through the versioned capability.

package billing

import (
	"context"
	"crypto/rand"
	"database/sql"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/billingcap"
)

type refundExecutionOwnerRow struct {
	CommandNo         string
	RefundNo          string
	RefundSourceNo    string
	PaymentAttemptNo  string
	CurrencyCode      string
	AmountMinor       int64
	RequestDigest     string
	Phase             string
	Status            string
	AttemptCount      int
	LeaseOwner        string
	LeaseExpiresAt    *time.Time
	ResultCode        string
	ResultReference   string
	ResultEvidence    string
	PreflightCode     string
	PreflightEvidence string
	ExecutionOwner    string
	RefundStatus      string
	CustomerNo        string
	OrderNo           string
	Version           int64
}

func (s *channelPaymentOwnerService) ClaimRefundExecution(
	ctx context.Context,
	input billingcap.ClaimRefundExecution,
) (*billingcap.RefundExecutionWork, error) {
	if err := billingcap.ValidateClaimRefundExecution(input); err != nil {
		return nil, err
	}
	var work *billingcap.RefundExecutionWork
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var row *refundExecutionOwnerRow
		if loadErr := s.database.GetScan(ctx, &row, `
SELECT command.command_no,command.refund_no,command.refund_source_no,
       command.source_reference AS payment_attempt_no,command.currency_code,
       command.amount_minor,command.request_digest,command.phase_code AS phase,
       command.status_code AS status,command.attempt_count,
       request.status_code AS refund_status,request.customer_no,request.order_no,request.version
FROM plugin_idc_core_refund_execution_command command
JOIN plugin_idc_core_refund_source_allocation source
  ON source.refund_source_no=command.refund_source_no
JOIN plugin_idc_core_refund_request request
  ON request.refund_no=command.refund_no
WHERE command.tenant_id=0 AND source.execution_owner_code='payment'
  AND command.destination_code='original_channel'
  AND request.status_code IN ('approved','executing')
  AND (command.status_code='pending'
       OR (command.status_code='claimed' AND command.lease_expires_at<CURRENT_TIMESTAMP))
  AND NOT EXISTS (
      SELECT 1 FROM plugin_idc_core_refund_execution_command active
      WHERE active.refund_no=command.refund_no AND active.phase_code='execution'
        AND active.status_code='claimed'
  )
  AND (command.phase_code='preflight' OR NOT EXISTS (
      SELECT 1 FROM plugin_idc_core_refund_execution_command gate
      WHERE gate.refund_no=command.refund_no AND gate.phase_code='preflight'
        AND gate.status_code IN ('pending','claimed')
  ))
ORDER BY CASE command.phase_code WHEN 'preflight' THEN 0 ELSE 1 END,
         command.created_at,command.id
LIMIT 1 FOR UPDATE OF command,request SKIP LOCKED`); loadErr != nil {
			return loadErr
		}
		if row == nil {
			work = &billingcap.RefundExecutionWork{}
			return nil
		}
		now, nowErr := refundDatabaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		leaseExpires := now.Add(time.Duration(input.LeaseSeconds) * time.Second)
		updated, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_execution_command
SET status_code='claimed',lease_owner=?,lease_expires_at=?,
    attempt_count=attempt_count+1,updated_at=?
WHERE command_no=? AND (status_code='pending'
    OR (status_code='claimed' AND lease_expires_at<CURRENT_TIMESTAMP))`,
			input.WorkerNo, leaseExpires, now, row.CommandNo)
		if updateErr != nil {
			return updateErr
		}
		if affected, affectedErr := updated.RowsAffected(); affectedErr != nil || affected != 1 {
			if affectedErr != nil {
				return affectedErr
			}
			return bizerr.NewCode(CodeRefundConflict)
		}
		if row.Phase == string(billingcap.RefundExecutionPhaseExecute) && row.RefundStatus == "approved" {
			requestUpdate, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_request
SET status_code='executing',version=version+1,updated_at=?
WHERE refund_no=? AND status_code='approved' AND version=?`, now, row.RefundNo, row.Version)
			if updateErr != nil {
				return updateErr
			}
			if updateErr = requireSingleRefundExecutionUpdate(requestUpdate); updateErr != nil {
				return updateErr
			}
			writer := &onlineRefundService{database: s.database, random: rand.Reader}
			sequence, sequenceErr := writer.nextAccountRefundDecisionSequence(ctx, row.RefundNo)
			if sequenceErr != nil {
				return sequenceErr
			}
			if decisionErr := writer.insertRefundDecision(
				ctx, row.RefundNo, sequence, "execution_started", "approved", "executing",
				"system", 0, "payment_execution_claimed", "", row.RequestDigest, now,
			); decisionErr != nil {
				return decisionErr
			}
			row.RefundStatus = "executing"
		}
		work = &billingcap.RefundExecutionWork{
			HasWork: true, CommandNo: row.CommandNo, RefundNo: row.RefundNo,
			RefundSourceNo: row.RefundSourceNo, PaymentAttemptNo: row.PaymentAttemptNo,
			CurrencyCode: row.CurrencyCode, AmountMinor: row.AmountMinor,
			RequestDigest: row.RequestDigest, Phase: billingcap.RefundExecutionPhase(row.Phase),
			Attempt: row.AttemptCount + 1, LeaseExpiresAt: leaseExpires,
		}
		return nil
	})
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	if work == nil || billingcap.ValidateRefundExecutionWork(*work) != nil {
		return nil, bizerr.NewCode(CodeRefundIntegrity)
	}
	return work, nil
}

func (s *channelPaymentOwnerService) ReportRefundExecution(
	ctx context.Context,
	input billingcap.ReportRefundExecution,
) (*billingcap.RefundExecutionReport, error) {
	if err := billingcap.ValidateReportRefundExecution(input); err != nil {
		return nil, err
	}
	input.ResultReference = strings.TrimSpace(input.ResultReference)
	var output *billingcap.RefundExecutionReport
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, loadErr := s.loadRefundExecutionOwnerRow(ctx, input.CommandNo)
		if loadErr != nil {
			return loadErr
		}
		if row == nil || row.RequestDigest != input.RequestDigest || row.Phase != string(input.Phase) {
			return bizerr.NewCode(CodeRefundNotAccessible)
		}
		if refundExecutionReportReplayed(row, input) {
			output = refundExecutionReport(row, true)
			return nil
		}
		now, nowErr := refundDatabaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if row.Status != "claimed" || row.LeaseOwner != input.WorkerNo || row.LeaseExpiresAt == nil ||
			!row.LeaseExpiresAt.After(now) {
			return bizerr.NewCode(CodeRefundConflict)
		}
		if input.Phase == billingcap.RefundExecutionPhasePreflight {
			var reportErr error
			output, reportErr = s.reportRefundPreflight(ctx, row, input, now)
			return reportErr
		}

		status := string(input.Status)
		updated, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_execution_command
SET status_code=?,result_code=?,result_reference=?,result_evidence_digest=?,
    completed_at=?,lease_owner=NULL,lease_expires_at=NULL,updated_at=?
WHERE command_no=? AND status_code='claimed' AND phase_code='execution'`,
			status, input.ResultCode, refundNullableString(input.ResultReference),
			input.EvidenceDigest, now, now, row.CommandNo)
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireSingleRefundExecutionUpdate(updated); updateErr != nil {
			return updateErr
		}
		refundStatus := "executing"
		decisionType := "execution_succeeded"
		switch input.Status {
		case billingcap.RefundExecutionResultFailed:
			refundStatus, decisionType = "manual_confirmation", "manual_confirmation"
		case billingcap.RefundExecutionResultUnknown:
			refundStatus, decisionType = "result_unknown", "execution_unknown"
		case billingcap.RefundExecutionResultManualConfirmation:
			refundStatus, decisionType = "manual_confirmation", "manual_confirmation"
		}
		if input.Status == billingcap.RefundExecutionResultSucceeded {
			refundStatus, updateErr = s.postSuccessfulRefundSource(
				ctx, row, input.EvidenceDigest, now,
			)
			if updateErr != nil {
				return updateErr
			}
		} else if refundStatus != "executing" {
			requestUpdated, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_request
SET status_code=?,version=version+1,updated_at=?
WHERE refund_no=? AND status_code='executing'`, refundStatus, now, row.RefundNo)
			if updateErr != nil {
				return updateErr
			}
			if updateErr = requireSingleRefundExecutionUpdate(requestUpdated); updateErr != nil {
				return updateErr
			}
			if refundStatus == "result_unknown" || refundStatus == "manual_confirmation" {
				if updateErr = s.createRefundReconciliationTask(
					ctx, row, input.ResultCode, now,
				); updateErr != nil {
					return updateErr
				}
			}
		}
		writer := &onlineRefundService{database: s.database, random: rand.Reader}
		sequence, sequenceErr := writer.nextAccountRefundDecisionSequence(ctx, row.RefundNo)
		if sequenceErr != nil {
			return sequenceErr
		}
		if decisionErr := writer.insertRefundDecision(
			ctx, row.RefundNo, sequence, decisionType, row.RefundStatus, refundStatus,
			"system", 0, input.ResultCode, "", input.EvidenceDigest, now,
		); decisionErr != nil {
			return decisionErr
		}
		row.Status = status
		row.ResultCode = input.ResultCode
		row.ResultReference = input.ResultReference
		row.ResultEvidence = input.EvidenceDigest
		row.RefundStatus = refundStatus
		output = refundExecutionReport(row, false)
		return nil
	})
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	if output == nil || billingcap.ValidateRefundExecutionReport(*output) != nil {
		return nil, bizerr.NewCode(CodeRefundIntegrity)
	}
	return output, nil
}

func (s *channelPaymentOwnerService) reportRefundPreflight(
	ctx context.Context,
	row *refundExecutionOwnerRow,
	input billingcap.ReportRefundExecution,
	now time.Time,
) (*billingcap.RefundExecutionReport, error) {
	if input.Status == billingcap.RefundExecutionResultReady {
		updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_execution_command
SET status_code='pending',phase_code='execution',lease_owner=NULL,lease_expires_at=NULL,
    preflight_result_code=?,preflight_evidence_digest=?,preflight_at=?,updated_at=?
WHERE command_no=? AND status_code='claimed' AND phase_code='preflight'`,
			input.ResultCode, input.EvidenceDigest, now, now, row.CommandNo)
		if err != nil {
			return nil, err
		}
		if err = requireSingleRefundExecutionUpdate(updated); err != nil {
			return nil, err
		}
		row.Status = "pending"
		row.Phase = "execution"
		row.PreflightCode = input.ResultCode
		row.PreflightEvidence = input.EvidenceDigest
		return refundExecutionReport(row, false), nil
	}

	var (
		commandStatus  = "failed"
		refundStatus   = "awaiting_balance_consent"
		decisionType   = "request_balance_consent"
		decisionReason = "original_channel_unsupported"
	)
	if input.Status == billingcap.RefundExecutionResultManualConfirmation {
		commandStatus = "manual_confirmation"
		refundStatus = "manual_confirmation"
		decisionType = "manual_confirmation"
		decisionReason = "payment_context_mismatch"
	}
	updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_execution_command
SET status_code=?,result_code=?,result_reference=?,result_evidence_digest=?,
    preflight_result_code=?,preflight_evidence_digest=?,preflight_at=?,completed_at=?,
    lease_owner=NULL,lease_expires_at=NULL,updated_at=?
WHERE command_no=? AND status_code='claimed' AND phase_code='preflight'`,
		commandStatus, input.ResultCode, refundNullableString(input.ResultReference), input.EvidenceDigest,
		input.ResultCode, input.EvidenceDigest, now, now, now, row.CommandNo)
	if err != nil {
		return nil, err
	}
	if err = requireSingleRefundExecutionUpdate(updated); err != nil {
		return nil, err
	}
	requestUpdated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_request
SET status_code=?,version=version+1,updated_at=?
WHERE refund_no=? AND status_code IN ('approved','executing')`, refundStatus, now, row.RefundNo)
	if err != nil {
		return nil, err
	}
	if err = requireSingleRefundExecutionUpdate(requestUpdated); err != nil {
		return nil, err
	}
	writer := &onlineRefundService{database: s.database, random: rand.Reader}
	sequence, err := writer.nextAccountRefundDecisionSequence(ctx, row.RefundNo)
	if err != nil {
		return nil, err
	}
	if err = writer.insertRefundDecision(
		ctx, row.RefundNo, sequence, decisionType, row.RefundStatus,
		refundStatus, "system", 0, decisionReason, "", input.EvidenceDigest, now,
	); err != nil {
		return nil, err
	}
	if refundStatus == "manual_confirmation" {
		if err = s.createRefundReconciliationTask(ctx, row, decisionReason, now); err != nil {
			return nil, err
		}
	}
	row.Status = commandStatus
	row.ResultCode = input.ResultCode
	row.ResultEvidence = input.EvidenceDigest
	row.RefundStatus = refundStatus
	return refundExecutionReport(row, false), nil
}

func requireSingleRefundExecutionUpdate(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return bizerr.NewCode(CodeRefundConflict)
	}
	return nil
}

func (s *channelPaymentOwnerService) loadRefundExecutionOwnerRow(
	ctx context.Context,
	commandNo string,
) (*refundExecutionOwnerRow, error) {
	var row *refundExecutionOwnerRow
	err := s.database.GetScan(ctx, &row, `
SELECT command.command_no,command.refund_no,command.refund_source_no,
       command.source_reference AS payment_attempt_no,command.currency_code,
       command.amount_minor,command.request_digest,command.phase_code AS phase,
       command.status_code AS status,command.attempt_count,command.lease_owner,
       command.lease_expires_at,command.result_code,command.result_reference,
       command.result_evidence_digest AS result_evidence,
       command.preflight_result_code AS preflight_code,
       command.preflight_evidence_digest AS preflight_evidence,
       request.status_code AS refund_status,request.customer_no,request.order_no,request.version
FROM plugin_idc_core_refund_execution_command command
JOIN plugin_idc_core_refund_source_allocation source
  ON source.refund_source_no=command.refund_source_no
JOIN plugin_idc_core_refund_request request ON request.refund_no=command.refund_no
WHERE command.tenant_id=0 AND command.command_no=?
  AND command.destination_code='original_channel'
  AND source.execution_owner_code='payment'
FOR UPDATE OF command,request`, commandNo)
	return row, err
}

func refundExecutionReportReplayed(
	row *refundExecutionOwnerRow,
	input billingcap.ReportRefundExecution,
) bool {
	if row == nil {
		return false
	}
	if input.Phase == billingcap.RefundExecutionPhasePreflight &&
		input.Status == billingcap.RefundExecutionResultReady {
		return row.Phase == "execution" && row.Status == "pending" &&
			row.PreflightCode == input.ResultCode && row.PreflightEvidence == input.EvidenceDigest
	}
	return row.Status == string(input.Status) && row.ResultCode == input.ResultCode &&
		row.ResultReference == input.ResultReference && row.ResultEvidence == input.EvidenceDigest
}

func refundExecutionReport(row *refundExecutionOwnerRow, replayed bool) *billingcap.RefundExecutionReport {
	if row == nil {
		return nil
	}
	return &billingcap.RefundExecutionReport{
		CommandNo: row.CommandNo, RefundNo: row.RefundNo,
		CommandStatusCode: row.Status, RefundStatusCode: row.RefundStatus, Reused: replayed,
	}
}
