// This file records mixed-source balance consent while keeping the whole refund atomic.

package billing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

func (s *onlineRefundService) RespondRefundBalanceConsent(
	ctx context.Context,
	input RefundBalanceConsentInput,
) (*OnlineRefund, error) {
	audience, err := currentFinancialCustomer(ctx, s.principal)
	if err != nil {
		return nil, bizerr.NewCode(CodeRefundNotAccessible)
	}
	input.RefundNo = strings.TrimSpace(input.RefundNo)
	key := strings.TrimSpace(input.IdempotencyKey)
	if !validBusinessReference(input.RefundNo, 40) || input.ExpectedVersion < 1 ||
		len(key) < 8 || len(key) > 128 {
		return nil, bizerr.NewCode(CodeRefundInvalid)
	}
	keyDigest := sha256.Sum256([]byte("refund-balance-consent\x00" + key))
	evidence, err := contentDigest(struct {
		RefundNo        string `json:"refundNo"`
		CustomerNo      string `json:"customerNo"`
		ExpectedVersion int64  `json:"expectedVersion"`
		Consent         bool   `json:"consent"`
		KeyDigest       string `json:"keyDigest"`
	}{input.RefundNo, audience.CustomerNo, input.ExpectedVersion, input.Consent,
		hex.EncodeToString(keyDigest[:])})
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	var result *OnlineRefund
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		request, loadErr := s.loadAccountRefund(ctx, audience.CustomerNo, input.RefundNo, true)
		if loadErr != nil {
			return loadErr
		}
		if request == nil {
			return bizerr.NewCode(CodeRefundNotAccessible)
		}
		replayed, replayErr := s.loadBalanceConsentReplay(
			ctx, request, audience.UserID, input, evidence,
		)
		if replayErr != nil {
			return replayErr
		}
		if replayed != nil {
			result = replayed
			return nil
		}
		if request.Status != "awaiting_balance_consent" || request.Version != input.ExpectedVersion {
			return bizerr.NewCode(CodeRefundConflict)
		}
		unsupported, sourceErr := s.loadUnsupportedRefundSources(ctx, request.RefundNo)
		if sourceErr != nil {
			return sourceErr
		}
		if len(unsupported) == 0 {
			return bizerr.NewCode(CodeRefundIntegrity)
		}
		if unsafe, unsafeErr := s.hasUnsafeRefundCommandState(ctx, request.RefundNo); unsafeErr != nil {
			return unsafeErr
		} else if unsafe {
			return bizerr.NewCode(CodeRefundConflict)
		}
		now, nowErr := refundDatabaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		sequence, sequenceErr := s.nextAccountRefundDecisionSequence(ctx, request.RefundNo)
		if sequenceErr != nil {
			return sequenceErr
		}
		if !input.Consent {
			var refusalErr error
			result, refusalErr = s.refuseRefundBalance(
				ctx, request, audience.CustomerNo, audience.UserID,
				input.ExpectedVersion, sequence, evidence, now,
			)
			return refusalErr
		}

		totalSources, countErr := s.database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_core_refund_source_allocation
WHERE tenant_id=0 AND refund_no=?`, request.RefundNo)
		if countErr != nil || totalSources.Int() < len(unsupported) {
			if countErr != nil {
				return countErr
			}
			return bizerr.NewCode(CodeRefundIntegrity)
		}
		destination := "mixed"
		if totalSources.Int() == len(unsupported) {
			destination = "same_currency_balance"
		}
		update, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_request
SET status_code='approved',refund_destination_code=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND refund_no=? AND customer_no=?
  AND status_code='awaiting_balance_consent' AND version=?`,
			destination, now, request.RefundNo, audience.CustomerNo, input.ExpectedVersion)
		if updateErr != nil {
			return updateErr
		}
		if affected, affectedErr := update.RowsAffected(); affectedErr != nil || affected != 1 {
			if affectedErr != nil {
				return affectedErr
			}
			return bizerr.NewCode(CodeRefundConflict)
		}
		commands := make([]refundExecutionCommandInsert, 0, len(unsupported))
		for _, source := range unsupported {
			command, commandErr := s.newRefundExecutionCommand(
				request.RefundNo, source.RefundSourceNo, request.CurrencyCode,
				source.Allocation, "same_currency_balance", now,
			)
			if commandErr != nil {
				return commandErr
			}
			commands = append(commands, command)
		}
		if commandErr := s.insertRefundExecutionCommands(ctx, commands); commandErr != nil {
			return commandErr
		}
		if decisionErr := s.insertRefundDecision(
			ctx, request.RefundNo, sequence, "balance_consent",
			"awaiting_balance_consent", "approved", "customer", audience.UserID,
			"customer_consented_balance", "", evidence, now,
		); decisionErr != nil {
			return decisionErr
		}
		request.Status = "approved"
		request.Destination = destination
		request.Version++
		result = request.toOnlineRefund()
		return s.appendRefundAudit(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "billing.refund.balance_consent", IdempotencyKey: evidence,
			OperationCode: "billing.refund.balance_consent", TargetType: "refund_request",
			TargetNo: request.RefundNo, CustomerNo: audience.CustomerNo,
			ResultCode: "succeeded", ReasonCode: "customer_consented_balance",
			RiskCode: "critical", CorrelationNo: request.OrderNo,
			BeforeSnapshot: map[string]any{"state": "awaiting_balance_consent"},
			AfterSnapshot:  refundAuditSnapshot(result),
		})
	})
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	return result, nil
}

func (s *onlineRefundService) refuseRefundBalance(
	ctx context.Context,
	request *managementRefundRow,
	customerNo string,
	userID int,
	expectedVersion int64,
	sequence int,
	evidence string,
	now time.Time,
) (*OnlineRefund, error) {
	update, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_request
SET version=version+1,updated_at=?
WHERE tenant_id=0 AND refund_no=? AND customer_no=?
  AND status_code='awaiting_balance_consent' AND version=?`,
		now, request.RefundNo, customerNo, expectedVersion)
	if err != nil {
		return nil, err
	}
	affected, err := update.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, bizerr.NewCode(CodeRefundConflict)
	}
	if err = s.insertRefundDecision(
		ctx, request.RefundNo, sequence, "balance_refusal",
		"awaiting_balance_consent", "awaiting_balance_consent",
		"customer", userID, "customer_refused_balance", "", evidence, now,
	); err != nil {
		return nil, err
	}
	request.Version++
	result := request.toOnlineRefund()
	err = s.appendRefundAudit(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "billing.refund.balance_consent", IdempotencyKey: evidence,
		OperationCode: "billing.refund.balance_consent", TargetType: "refund_request",
		TargetNo: request.RefundNo, CustomerNo: customerNo,
		ResultCode: "succeeded", ReasonCode: "customer_refused_balance",
		RiskCode: "high", CorrelationNo: request.OrderNo,
		BeforeSnapshot: map[string]any{"state": "awaiting_balance_consent"},
		AfterSnapshot:  refundAuditSnapshot(result),
	})
	return result, err
}

type unsupportedRefundSource struct {
	RefundSourceNo string
	Allocation     RefundSourceAllocation
}

func (s *onlineRefundService) loadUnsupportedRefundSources(
	ctx context.Context,
	refundNo string,
) ([]unsupportedRefundSource, error) {
	var rows []struct {
		RefundSourceNo              string `orm:"refund_source_no"`
		PaymentAllocationNo         string `orm:"payment_allocation_no"`
		PaymentSourceType           string `orm:"payment_source_type_code"`
		PaymentSourceReference      string `orm:"payment_source_reference"`
		ExecutionOwner              string `orm:"execution_owner_code"`
		AmountMinor                 int64  `orm:"amount_minor"`
		CreditExposureNo            string `orm:"credit_exposure_no"`
		CreditComponent             string `orm:"credit_component_code"`
		CreditRepaymentAllocationNo string `orm:"credit_repayment_allocation_no"`
		CreditPlanDigest            string `orm:"credit_plan_digest"`
		ContentDigest               string `orm:"content_digest"`
	}
	err := s.database.GetScan(ctx, &rows, `
SELECT source.refund_source_no,COALESCE(source.payment_allocation_no,'') AS payment_allocation_no,
       source.payment_source_type_code,source.payment_source_reference,
       source.execution_owner_code,source.amount_minor,
       COALESCE(source.credit_exposure_no,'') AS credit_exposure_no,
       COALESCE(source.credit_component_code,'') AS credit_component_code,
       COALESCE(source.credit_repayment_allocation_no,'') AS credit_repayment_allocation_no,
       COALESCE(source.credit_plan_digest,'') AS credit_plan_digest,
       source.content_digest
FROM plugin_idc_core_refund_source_allocation source
JOIN plugin_idc_core_refund_execution_command command
  ON command.refund_source_no=source.refund_source_no
 AND command.destination_code='original_channel'
WHERE source.tenant_id=0 AND source.refund_no=?
  AND command.status_code='failed'
  AND command.result_code='original_channel_unsupported'
ORDER BY source.id`, refundNo)
	if err != nil {
		return nil, err
	}
	result := make([]unsupportedRefundSource, 0, len(rows))
	for _, row := range rows {
		result = append(result, unsupportedRefundSource{
			RefundSourceNo: row.RefundSourceNo,
			Allocation: RefundSourceAllocation{
				PaymentAllocationNo:    row.PaymentAllocationNo,
				PaymentSourceType:      row.PaymentSourceType,
				PaymentSourceReference: row.PaymentSourceReference,
				ExecutionOwner:         row.ExecutionOwner, AmountMinor: row.AmountMinor,
				CreditExposureNo:            row.CreditExposureNo,
				CreditComponent:             CreditRefundComponent(row.CreditComponent),
				CreditRepaymentAllocationNo: row.CreditRepaymentAllocationNo,
				CreditPlanDigest:            row.CreditPlanDigest, ContentDigest: row.ContentDigest,
			},
		})
	}
	return result, nil
}

func (s *onlineRefundService) hasUnsafeRefundCommandState(
	ctx context.Context,
	refundNo string,
) (bool, error) {
	value, err := s.database.GetValue(ctx, `
SELECT COUNT(*)
FROM plugin_idc_core_refund_execution_command
WHERE tenant_id=0 AND refund_no=? AND (
    status_code IN ('claimed','succeeded','result_unknown','manual_confirmation')
    OR (status_code='failed' AND result_code<>'original_channel_unsupported')
)`, refundNo)
	return value.Int64() > 0, err
}

func (s *onlineRefundService) loadAccountRefund(
	ctx context.Context,
	customerNo string,
	refundNo string,
	lock bool,
) (*managementRefundRow, error) {
	query := `
SELECT refund_no,customer_no,order_no,invoice_no,currency_code,status_code AS status,
       reason_code AS reason,reason_detail,requested_amount_minor AS amount_minor,
       approval_mode_code AS approval_mode,refund_destination_code AS destination,
       requires_entitlement_termination,has_resource_impact,has_dangerous_operation,
       has_fund_anomaly,high_amount,version,impact_digest,request_digest,
       submitted_at,approved_at,completed_at
FROM plugin_idc_core_refund_request
WHERE tenant_id=0 AND refund_no=? AND customer_no=?`
	if lock {
		query += " FOR UPDATE"
	}
	var row *managementRefundRow
	err := s.database.GetScan(ctx, &row, query, refundNo, customerNo)
	return row, err
}

func (s *onlineRefundService) loadBalanceConsentReplay(
	ctx context.Context,
	request *managementRefundRow,
	actor int,
	input RefundBalanceConsentInput,
	evidence string,
) (*OnlineRefund, error) {
	if request == nil || request.Version != input.ExpectedVersion+1 {
		return nil, nil
	}
	decisionType := "balance_refusal"
	wantStatus := "awaiting_balance_consent"
	if input.Consent {
		decisionType = "balance_consent"
		wantStatus = "approved"
	}
	if request.Status != wantStatus {
		return nil, nil
	}
	value, err := s.database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_core_refund_decision
WHERE refund_no=? AND decision_type_code=? AND actor_type_code='customer'
  AND actor_id=? AND evidence_digest=?`, request.RefundNo, decisionType, actor, evidence)
	if err != nil || value.Int64() != 1 {
		return nil, err
	}
	result := request.toOnlineRefund()
	result.Reused = true
	return result, nil
}

func (s *onlineRefundService) nextAccountRefundDecisionSequence(
	ctx context.Context,
	refundNo string,
) (int, error) {
	value, err := s.database.GetValue(ctx, `
SELECT COALESCE(MAX(sequence_no),0)+1
FROM plugin_idc_core_refund_decision WHERE refund_no=?`, refundNo)
	if err != nil || value.Int() < 1 {
		return 0, err
	}
	return value.Int(), nil
}
