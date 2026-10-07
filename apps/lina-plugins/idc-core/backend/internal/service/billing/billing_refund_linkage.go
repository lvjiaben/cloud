// This file commits verified refund money and publishes the exact downstream
// facts required by service recovery, commission reversal, and tax invoicing.

package billing

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/outbox"
)

const (
	refundLedgerSourceType             = "online_refund"
	refundSourcePostedEventType        = "billing.refund.source_posted"
	refundSucceededEventType           = "billing.refund.succeeded"
	refundCommissionEventType          = "billing.refund.commission_reversal_requested"
	refundTaxEventType                 = "billing.refund.tax_adjustment_requested"
	refundEntitlementEventType         = "billing.refund.entitlement_termination_required"
	refundResultReconciliationTaskType = "refund_result_reconciliation"
	refundEntitlementTaskType          = "refund_entitlement_termination"
	refundEntitlementTaskMaxAttempts   = 3
	refundHighPriority                 = 100
	refundTaskMaximumAttempts          = 1
)

type refundPostingContext struct {
	RefundNo                    string
	CommandNo                   string
	RefundSourceNo              string
	RefundItemNo                string
	InvoiceNo                   string
	InvoiceItemNo               string
	PaymentAllocationNo         string
	CreditExposureNo            string
	CreditComponent             string
	CreditRepaymentAllocationNo string
	CreditPlanDigest            string
	PaymentSourceType           string
	ExecutionOwner              string
	Destination                 string
	CustomerNo                  string
	CurrencyCode                string
	AmountMinor                 int64
	RequiresTermination         bool
}

type refundCompletionSummary struct {
	EffectiveCommands int64 `orm:"effective_commands"`
	SucceededCommands int64 `orm:"succeeded_commands"`
	PostedCommands    int64 `orm:"posted_commands"`
}

type refundEntitlementRecoveryRow struct {
	ImpactNo            string
	EntitlementNo       string
	RequiresTermination bool
}

// postSuccessfulRefundSource posts the immutable ledger and invoice facts for
// one verified source and returns the authoritative whole-request state.
func (s *channelPaymentOwnerService) postSuccessfulRefundSource(
	ctx context.Context,
	row *refundExecutionOwnerRow,
	evidenceDigest string,
	now time.Time,
) (string, error) {
	posting, err := s.loadRefundPostingContext(ctx, row.CommandNo)
	if err != nil {
		return "", err
	}
	if posting == nil || posting.RefundNo != row.RefundNo ||
		posting.CurrencyCode != row.CurrencyCode || posting.AmountMinor != row.AmountMinor {
		return "", bizerr.NewCode(CodeRefundIntegrity)
	}
	if posting.CreditExposureNo != "" {
		return s.postSuccessfulCreditRefundSource(ctx, posting, now)
	}
	transactionNo, sourceType, err := s.postRefundLedgerLocked(ctx, posting, evidenceDigest)
	if err != nil {
		return "", err
	}
	if _, err = s.ledger.applyRefundLocked(ctx, InvoiceRefundInput{
		InvoiceNo: posting.InvoiceNo, InvoiceItemNo: posting.InvoiceItemNo,
		PaymentAllocationNo: posting.PaymentAllocationNo, SourceType: sourceType,
		SourceReference: posting.CommandNo, LedgerTransactionNo: transactionNo,
		AmountMinor: posting.AmountMinor,
	}); err != nil {
		return "", err
	}
	if err = s.enqueueRefundSourcePosted(ctx, posting, sourceType, now); err != nil {
		return "", err
	}
	return s.completeRefundRequestIfReady(ctx, posting, now)
}

func (s *channelPaymentOwnerService) loadRefundPostingContext(
	ctx context.Context,
	commandNo string,
) (*refundPostingContext, error) {
	var row *refundPostingContext
	err := s.database.GetScan(ctx, &row, `
SELECT request.refund_no,command.command_no,source.refund_source_no,
       source.refund_item_no,request.invoice_no,item.invoice_item_no,
       COALESCE(source.payment_allocation_no,'') AS payment_allocation_no,
       COALESCE(source.credit_exposure_no,'') AS credit_exposure_no,
       COALESCE(source.credit_component_code,'') AS credit_component,
       COALESCE(source.credit_repayment_allocation_no,'') AS credit_repayment_allocation_no,
       COALESCE(source.credit_plan_digest,'') AS credit_plan_digest,
       source.payment_source_type_code AS payment_source_type,
       source.execution_owner_code AS execution_owner,
       command.destination_code AS destination,request.customer_no,
       request.currency_code,command.amount_minor,
       request.requires_entitlement_termination AS requires_termination
FROM plugin_idc_core_refund_execution_command command
JOIN plugin_idc_core_refund_source_allocation source
  ON source.refund_source_no=command.refund_source_no
JOIN plugin_idc_core_refund_request request
  ON request.refund_no=command.refund_no
JOIN plugin_idc_core_refund_request_item item
  ON item.refund_item_no=source.refund_item_no
WHERE command.tenant_id=0 AND command.command_no=?
FOR SHARE OF source,item`, commandNo)
	return row, err
}

func (s *channelPaymentOwnerService) postRefundLedgerLocked(
	ctx context.Context,
	posting *refundPostingContext,
	evidenceDigest string,
) (string, RefundSourceType, error) {
	if posting == nil || posting.AmountMinor <= 0 {
		return "", "", bizerr.NewCode(CodeRefundIntegrity)
	}
	if err := s.ledger.lockSource(ctx, refundLedgerSourceType, posting.CommandNo); err != nil {
		return "", "", err
	}
	existing, err := loadTransactionBySource(
		ctx, s.database, refundLedgerSourceType, posting.CommandNo,
	)
	if err != nil {
		return "", "", err
	}
	merchantNo, err := s.ledger.loadMerchantNo(ctx, existing == nil)
	if err != nil {
		return "", "", err
	}
	revenueSpec := merchantRevenueSpec(merchantNo)
	creditSpec, sourceType, err := refundCreditAccount(posting, merchantNo)
	if err != nil {
		return "", "", err
	}
	if creditSpec.CustomerNo != "" {
		if err = s.ledger.requireActiveCustomer(ctx, creditSpec.CustomerNo); err != nil {
			return "", "", err
		}
	}
	accounts, err := s.ledger.ensureAndLockAccounts(
		ctx, posting.CurrencyCode, existing == nil, revenueSpec, creditSpec,
	)
	if err != nil {
		return "", "", err
	}
	revenueAccountNo := deterministicAccountNo(
		revenueSpec.OwnerType, revenueSpec.OwnerReference,
		revenueSpec.AccountType, posting.CurrencyCode,
	)
	creditAccountNo := deterministicAccountNo(
		creditSpec.OwnerType, creditSpec.OwnerReference,
		creditSpec.AccountType, posting.CurrencyCode,
	)
	transaction, _, err := s.ledger.postLocked(
		ctx, "refund", posting.CurrencyCode, refundLedgerSourceType,
		posting.CommandNo, evidenceDigest, "",
		[]entrySemantic{
			{AccountNo: revenueAccountNo, Direction: "debit", Role: "refund",
				AmountMinor: posting.AmountMinor, InvoiceNo: posting.InvoiceNo,
				InvoiceItemNo: posting.InvoiceItemNo, Reason: "confirmed_refund"},
			{AccountNo: creditAccountNo, Direction: "credit", Role: refundCreditRole(sourceType),
				AmountMinor: posting.AmountMinor, InvoiceNo: posting.InvoiceNo,
				InvoiceItemNo: posting.InvoiceItemNo, Reason: "confirmed_refund"},
		}, accounts,
	)
	if err != nil {
		return "", "", err
	}
	return transaction.TransactionNo, sourceType, nil
}

func refundCreditAccount(
	posting *refundPostingContext,
	merchantNo string,
) (accountSpec, RefundSourceType, error) {
	if posting.Destination == "same_currency_balance" || posting.PaymentSourceType == "balance" {
		return customerBalanceSpec(posting.CustomerNo), RefundSourceBalanceCredit, nil
	}
	switch posting.PaymentSourceType {
	case "channel_payment":
		return merchantCashSpec(merchantNo), RefundSourceChannel, nil
	case "credit":
		return merchantReceivableSpec(merchantNo), RefundSourceCreditRelease, nil
	case "migration":
		return merchantCashSpec(merchantNo), RefundSourceMigration, nil
	case "manual_receipt":
		return merchantCashSpec(merchantNo), RefundSourceManual, nil
	default:
		return accountSpec{}, "", bizerr.NewCode(CodeRefundIntegrity)
	}
}

func refundCreditRole(sourceType RefundSourceType) string {
	if sourceType == RefundSourceBalanceCredit {
		return "balance"
	}
	if sourceType == RefundSourceCreditRelease {
		return "receivable"
	}
	return "cash"
}

func (s *channelPaymentOwnerService) completeRefundRequestIfReady(
	ctx context.Context,
	posting *refundPostingContext,
	now time.Time,
) (string, error) {
	var summary refundCompletionSummary
	if err := s.database.GetScan(ctx, &summary, `
SELECT COUNT(*) AS effective_commands,
       COUNT(*) FILTER (WHERE status_code='succeeded') AS succeeded_commands,
       COUNT(*) FILTER (WHERE status_code='succeeded' AND EXISTS (
           SELECT 1
           FROM plugin_idc_core_billing_refund_allocation allocation
           WHERE allocation.tenant_id=0 AND allocation.direction_code='apply'
             AND allocation.refund_source_reference=effective.command_no
       )) AS posted_commands
FROM (
    SELECT DISTINCT ON (source.refund_source_no)
           command.command_no,command.status_code
    FROM plugin_idc_core_refund_source_allocation source
    JOIN plugin_idc_core_refund_execution_command command
      ON command.refund_source_no=source.refund_source_no
    WHERE source.tenant_id=0 AND source.refund_no=?
    ORDER BY source.refund_source_no,
             CASE command.destination_code WHEN 'same_currency_balance' THEN 0 ELSE 1 END,
             command.id DESC
) effective`, posting.RefundNo); err != nil {
		return "", err
	}
	if summary.EffectiveCommands == 0 ||
		summary.SucceededCommands != summary.EffectiveCommands ||
		summary.PostedCommands != summary.EffectiveCommands {
		return "executing", nil
	}
	updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_request
SET status_code='succeeded',completed_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND refund_no=? AND status_code='executing'`,
		now, now, posting.RefundNo)
	if err != nil {
		return "", err
	}
	if err = requireSingleRefundExecutionUpdate(updated); err != nil {
		return "", err
	}
	if err = s.enqueueCompletedRefundEvents(ctx, posting, now); err != nil {
		return "", err
	}
	return "succeeded", nil
}

func (s *channelPaymentOwnerService) enqueueRefundSourcePosted(
	ctx context.Context,
	posting *refundPostingContext,
	sourceType RefundSourceType,
	now time.Time,
) error {
	payload, err := json.Marshal(struct {
		RefundNo       string `json:"refundNo"`
		CommandNo      string `json:"commandNo"`
		RefundSourceNo string `json:"refundSourceNo"`
		InvoiceNo      string `json:"invoiceNo"`
		InvoiceItemNo  string `json:"invoiceItemNo"`
		CurrencyCode   string `json:"currencyCode"`
		AmountMinor    int64  `json:"amountMinor"`
		SourceTypeCode string `json:"sourceTypeCode"`
	}{
		posting.RefundNo, posting.CommandNo, posting.RefundSourceNo,
		posting.InvoiceNo, posting.InvoiceItemNo, posting.CurrencyCode,
		posting.AmountMinor, string(sourceType),
	})
	if err != nil {
		return gerror.Wrap(err, "encode posted refund source event")
	}
	_, err = s.events.Enqueue(ctx, outbox.EnqueueInput{
		EventType: refundSourcePostedEventType, EventVersion: 1,
		Topic: refundSourcePostedEventType, AggregateType: "refund_request",
		AggregateNo: posting.RefundNo, CorrelationNo: posting.CommandNo,
		DeduplicationKey: refundSourcePostedEventType + ":" + posting.CommandNo,
		Payload:          payload, Metadata: json.RawMessage(`{}`), AvailableAt: now,
	})
	return err
}

func (s *channelPaymentOwnerService) enqueueCompletedRefundEvents(
	ctx context.Context,
	posting *refundPostingContext,
	now time.Time,
) error {
	payload, err := json.Marshal(struct {
		RefundNo     string `json:"refundNo"`
		InvoiceNo    string `json:"invoiceNo"`
		CustomerNo   string `json:"customerNo"`
		CurrencyCode string `json:"currencyCode"`
	}{posting.RefundNo, posting.InvoiceNo, posting.CustomerNo, posting.CurrencyCode})
	if err != nil {
		return gerror.Wrap(err, "encode completed refund event")
	}
	for _, eventType := range []string{
		refundSucceededEventType, refundCommissionEventType, refundTaxEventType,
	} {
		if _, err = s.events.Enqueue(ctx, outbox.EnqueueInput{
			EventType: eventType, EventVersion: 1, Topic: eventType,
			AggregateType: "refund_request", AggregateNo: posting.RefundNo,
			CorrelationNo:    posting.RefundNo,
			DeduplicationKey: eventType + ":" + posting.RefundNo,
			Payload:          payload, Metadata: json.RawMessage(`{}`), AvailableAt: now,
		}); err != nil {
			return err
		}
	}
	return s.createRefundEntitlementTasks(ctx, posting, now)
}

func (s *channelPaymentOwnerService) createRefundEntitlementTasks(
	ctx context.Context,
	posting *refundPostingContext,
	now time.Time,
) error {
	rows := make([]refundEntitlementRecoveryRow, 0, 16)
	if err := s.database.GetScan(ctx, &rows, `
SELECT impact_no,entitlement_no,requires_termination
FROM plugin_idc_core_refund_entitlement_impact
WHERE tenant_id=0 AND refund_no=?
ORDER BY id
LIMIT ?`, posting.RefundNo, maximumFinancialSettlementSources+1); err != nil {
		return err
	}
	if len(rows) > maximumFinancialSettlementSources {
		return bizerr.NewCode(CodeRefundIntegrity)
	}
	for _, row := range rows {
		if !row.RequiresTermination {
			continue
		}
		payload, err := json.Marshal(struct {
			RefundNo      string `json:"refundNo"`
			ImpactNo      string `json:"impactNo"`
			EntitlementNo string `json:"entitlementNo"`
		}{posting.RefundNo, row.ImpactNo, row.EntitlementNo})
		if err != nil {
			return err
		}
		if _, err = s.tasks.CreateInDatabase(ctx, s.database, asynctask.CreateInput{
			TaskType: refundEntitlementTaskType, SubjectType: "service_entitlement",
			SubjectNo: row.EntitlementNo, CustomerNo: posting.CustomerNo,
			IdempotencyKey: refundEntitlementTaskType + ":" + posting.RefundNo + ":" + row.ImpactNo,
			Priority:       refundHighPriority, Payload: payload, AvailableAt: now,
			MaxAttempts: refundEntitlementTaskMaxAttempts,
		}); err != nil {
			return err
		}
		if _, err = s.events.Enqueue(ctx, outbox.EnqueueInput{
			EventType: refundEntitlementEventType, EventVersion: 1,
			Topic: refundEntitlementEventType, AggregateType: "service_entitlement",
			AggregateNo: row.EntitlementNo, CorrelationNo: posting.RefundNo,
			DeduplicationKey: refundEntitlementEventType + ":" + posting.RefundNo + ":" + row.ImpactNo,
			Payload:          payload, Metadata: json.RawMessage(`{}`), AvailableAt: now,
		}); err != nil {
			return err
		}
	}
	if len(rows) == 0 || !posting.RequiresTermination {
		_, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_entitlement_lock
SET released_at=?,release_reason_code='succeeded'
WHERE tenant_id=0 AND refund_no=? AND released_at IS NULL`, now, posting.RefundNo)
		return err
	}
	return nil
}

func (s *channelPaymentOwnerService) createRefundReconciliationTask(
	ctx context.Context,
	row *refundExecutionOwnerRow,
	reasonCode string,
	now time.Time,
) error {
	payload, err := json.Marshal(struct {
		RefundNo   string `json:"refundNo"`
		CommandNo  string `json:"commandNo"`
		ReasonCode string `json:"reasonCode"`
	}{row.RefundNo, row.CommandNo, reasonCode})
	if err != nil {
		return err
	}
	_, err = s.tasks.CreateInDatabase(ctx, s.database, asynctask.CreateInput{
		TaskType: refundResultReconciliationTaskType, SubjectType: "refund_command",
		SubjectNo: row.CommandNo, CustomerNo: row.CustomerNo,
		IdempotencyKey: refundResultReconciliationTaskType + ":" + row.CommandNo,
		Priority:       refundHighPriority, Payload: payload, AvailableAt: now,
		MaxAttempts: refundTaskMaximumAttempts,
	})
	return err
}
