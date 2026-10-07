// This file exercises customer, staff, and mixed-source refund workflows on PostgreSQL.

package commerce

import (
	"context"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

type refundManagementAuthorizerStub struct{ actor int }

type refundBusinessAuditStub struct{}

func (refundBusinessAuditStub) AppendBusinessTransition(
	context.Context,
	idccap.BusinessAuditTransition,
) error {
	return nil
}

func (refundBusinessAuditStub) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func (s refundManagementAuthorizerStub) AuthorizeRefundList(context.Context) (int, error) {
	return s.actor, nil
}
func (s refundManagementAuthorizerStub) AuthorizeRefundDetail(context.Context, string) (int, error) {
	return s.actor, nil
}
func (s refundManagementAuthorizerStub) AuthorizeRefundReview(context.Context, string) (int, error) {
	return s.actor, nil
}
func (s refundManagementAuthorizerStub) AuthorizeRefundPolicyWrite(context.Context) (int, error) {
	return s.actor, nil
}

type refundRecentAuthStub struct {
	binding reauth.Binding
	actor   int
}

func (s *refundRecentAuthStub) Issue(
	_ context.Context,
	binding reauth.Binding,
) (*reauth.ProofContext, error) {
	s.binding = binding
	return &reauth.ProofContext{Token: "fresh-refund-proof", ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func (s *refundRecentAuthStub) Consume(
	_ context.Context,
	token string,
	expected reauth.Binding,
) (*reauth.Proof, error) {
	if token != "fresh-refund-proof" || expected != s.binding {
		return nil, bizerr.NewCode(billingservice.CodeRefundConflict)
	}
	return &reauth.Proof{
		Purpose: expected.Purpose, TargetDigest: expected.TargetDigest,
		UserID: s.actor, TenantID: 0,
	}, nil
}

func TestOnlineRefundPostgreSQLSubmitsAutomaticPartialRefund(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "online-refund-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout online refund order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balance, err := billingservice.NewBalanceService(scenario.database)
	if err != nil {
		t.Fatalf("create online refund balance service: %v", err)
	}
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY",
		AmountMinor: checkout.TotalAmountMinor, SourceReference: "online-refund-recharge-0001",
		SourceSnapshotDigest: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatalf("fund online refund balance: %v", err)
	}
	invoices, err := billingservice.NewInvoiceService(scenario.database)
	if err != nil {
		t.Fatalf("create online refund invoice service: %v", err)
	}
	if _, err = invoices.PayFromBalance(scenario.ctx, billingservice.InvoiceBalancePaymentInput{
		InvoiceNo: checkout.InvoiceNo, SourceReference: "online-refund-payment-0001",
		SourceSnapshotDigest: strings.Repeat("b", 64),
	}); err != nil {
		t.Fatalf("pay online refund invoice: %v", err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_refund_policy_version
    (policy_no,currency_code,policy_version,high_amount_threshold_minor,
     automatic_approval_enabled,content_digest,configured_by)
VALUES ('refund-policy-cny-1','CNY',1,1000000,TRUE,repeat('c',64),9001)`); err != nil {
		t.Fatalf("seed online refund policy: %v", err)
	}
	var item struct {
		InvoiceItemNo string `orm:"invoice_item_no"`
		TotalMinor    int64  `orm:"total_minor"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &item, `
SELECT invoice_item_no,total_amount_minor AS total_minor
FROM plugin_idc_core_billing_invoice_item
WHERE invoice_no=? ORDER BY line_number LIMIT 1`, checkout.InvoiceNo); err != nil ||
		item.InvoiceItemNo == "" || item.TotalMinor < 2 {
		t.Fatalf("load online refund invoice item: item=%+v err=%v", item, err)
	}
	refunds, err := billingservice.NewAccountRefundService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 101, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1",
		}}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatalf("create online refund service: %v", err)
	}
	amount := item.TotalMinor / 2
	previewInput := billingservice.RefundPreviewInput{
		OrderNo: checkout.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: item.InvoiceItemNo,
			Quantity:      1,
			AmountMinor:   amount,
		}},
	}
	preview, err := refunds.PreviewOnlineRefund(scenario.ctx, previewInput)
	if err != nil || preview == nil || preview.AmountMinor != amount ||
		preview.ApprovalMode != "automatic" || preview.HighAmount ||
		preview.HasResourceImpact || preview.RequiresEntitlementTermination ||
		len(preview.Items) != 1 || preview.ImpactDigest == "" {
		t.Fatalf("preview automatic partial refund: preview=%+v err=%v", preview, err)
	}
	input := billingservice.RefundSubmissionInput{
		PreviewDigest:  preview.ImpactDigest,
		Reason:         billingservice.RefundReasonWrongConfiguration,
		IdempotencyKey: "online-refund-submit-0001",
		Preview:        previewInput,
	}
	created, err := refunds.SubmitOnlineRefund(scenario.ctx, input)
	if err != nil || created == nil || created.Status != "approved" ||
		created.ApprovalMode != "automatic" || created.AmountMinor != amount ||
		created.ApprovedAt == nil || created.Reused {
		t.Fatalf("submit automatic partial refund: refund=%+v err=%v", created, err)
	}
	replayed, err := refunds.SubmitOnlineRefund(scenario.ctx, input)
	if err != nil || replayed == nil || !replayed.Reused ||
		replayed.RefundNo != created.RefundNo || replayed.Status != "approved" {
		t.Fatalf("replay automatic partial refund: refund=%+v err=%v", replayed, err)
	}
	var state struct {
		Requests  int64 `orm:"requests"`
		Items     int64 `orm:"items"`
		Sources   int64 `orm:"sources"`
		Decisions int64 `orm:"decisions"`
		Commands  int64 `orm:"commands"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &state, `
SELECT
  (SELECT COUNT(*) FROM plugin_idc_core_refund_request WHERE refund_no=?) AS requests,
  (SELECT COUNT(*) FROM plugin_idc_core_refund_request_item WHERE refund_no=?) AS items,
  (SELECT COUNT(*) FROM plugin_idc_core_refund_source_allocation WHERE refund_no=?) AS sources,
  (SELECT COUNT(*) FROM plugin_idc_core_refund_decision WHERE refund_no=?) AS decisions,
  (SELECT COUNT(*) FROM plugin_idc_core_refund_execution_command WHERE refund_no=?) AS commands`,
		created.RefundNo, created.RefundNo, created.RefundNo, created.RefundNo, created.RefundNo); err != nil {
		t.Fatalf("load automatic refund state: %v", err)
	}
	if state.Requests != 1 || state.Items != 1 || state.Sources != 1 ||
		state.Decisions != 1 || state.Commands != 1 {
		t.Fatalf("unexpected automatic refund state: %+v", state)
	}
	if _, err = refunds.PreviewOnlineRefund(scenario.ctx, billingservice.RefundPreviewInput{
		OrderNo: checkout.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: item.InvoiceItemNo,
			Quantity:      1,
			AmountMinor:   item.TotalMinor,
		}},
	}); !bizerr.Is(err, billingservice.CodeRefundNotEligible) {
		t.Fatalf("reserved refund amount allowed over-refund: %v", err)
	}
	foreign, err := billingservice.NewAccountRefundService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 202, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-2",
		}}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = foreign.PreviewOnlineRefund(scenario.ctx, previewInput); !bizerr.Is(
		err, billingservice.CodeRefundNotAccessible,
	) {
		t.Fatalf("foreign customer refund preview error=%v", err)
	}
}

func TestOnlineRefundPostgreSQLCompletesCreditSettledOrder(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "credit-refund-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout credit refund order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	activateFinancialSettlementCredit(
		t, scenario, "customer-commerce-service-1", checkout.TotalAmountMinor+10_000,
		"online-refund",
	)
	settled, err := newFinancialSettlementTestService(t, scenario).SettleOrderWithCredit(
		scenario.ctx, billingservice.CreditOrderSettlementInput{
			OrderNo: checkout.OrderNo, ActorType: "customer", ActorID: 101,
		},
	)
	if err != nil || settled == nil ||
		settled.Basis != billingservice.SettlementBasisCredit {
		t.Fatalf("settle credit refund order: settlement=%+v err=%v", settled, err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_refund_policy_version
    (policy_no,currency_code,policy_version,high_amount_threshold_minor,
     automatic_approval_enabled,content_digest,configured_by)
VALUES ('refund-policy-credit-cny-1','CNY',1,1000000,TRUE,repeat('c',64),9001)`); err != nil {
		t.Fatalf("seed credit refund policy: %v", err)
	}
	var item struct {
		InvoiceItemNo string `orm:"invoice_item_no"`
		TotalMinor    int64  `orm:"total_minor"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &item, `
SELECT invoice_item_no,total_amount_minor AS total_minor
FROM plugin_idc_core_billing_invoice_item
WHERE invoice_no=? ORDER BY line_number LIMIT 1`, checkout.InvoiceNo); err != nil ||
		item.InvoiceItemNo == "" || item.TotalMinor != checkout.TotalAmountMinor {
		t.Fatalf("load credit refund item: item=%+v err=%v", item, err)
	}
	refunds, err := billingservice.NewAccountRefundService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 101, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1",
		}}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatalf("create credit online refund service: %v", err)
	}
	previewInput := billingservice.RefundPreviewInput{
		OrderNo: checkout.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: item.InvoiceItemNo,
			Quantity:      1,
			AmountMinor:   item.TotalMinor,
		}},
	}
	preview, err := refunds.PreviewOnlineRefund(scenario.ctx, previewInput)
	if err != nil || preview == nil || preview.AmountMinor != item.TotalMinor ||
		len(preview.Items) != 1 || len(preview.Items[0].Sources) != 1 ||
		preview.Items[0].Sources[0].CreditComponent != billingservice.CreditRefundUnbilled ||
		preview.Items[0].Sources[0].CreditExposureNo == "" ||
		preview.Items[0].Sources[0].PaymentAllocationNo != "" {
		t.Fatalf("preview credit online refund: preview=%+v err=%v", preview, err)
	}
	created, err := refunds.SubmitOnlineRefund(scenario.ctx, billingservice.RefundSubmissionInput{
		PreviewDigest: preview.ImpactDigest, Reason: billingservice.RefundReasonNoLongerNeeded,
		IdempotencyKey: "credit-refund-submit-0001", Preview: previewInput,
	})
	if err != nil || created == nil || created.Status != "approved" {
		t.Fatalf("submit credit online refund: refund=%+v err=%v", created, err)
	}
	worker, err := newLocalRefundWorkerTestService(t, scenario.database)
	if err != nil {
		t.Fatalf("create credit local refund worker: %v", err)
	}
	processed, err := worker.Run(scenario.ctx, 10)
	if err != nil || processed != 1 {
		t.Fatalf("execute credit online refund: processed=%d err=%v", processed, err)
	}
	var state struct {
		RefundStatus    string `orm:"refund_status"`
		ExposureStatus  string `orm:"exposure_status"`
		ReleasedMinor   int64  `orm:"released_minor"`
		UnbilledMinor   int64  `orm:"unbilled_minor"`
		AvailableMinor  int64  `orm:"available_minor"`
		RefundMovements int64  `orm:"refund_movements"`
		InvoiceRefunds  int64  `orm:"invoice_refunds"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &state, `
SELECT request.status_code AS refund_status,
       exposure.status_code AS exposure_status,
       exposure.released_principal_minor AS released_minor,
       account.unbilled_principal_minor AS unbilled_minor,
       account.available_limit_minor AS available_minor,
       (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement movement
         WHERE movement.credit_account_no=account.credit_account_no
           AND movement.movement_type_code='refund'
           AND movement.source_reference=?) AS refund_movements,
       (SELECT COUNT(*) FROM plugin_idc_core_billing_refund_allocation allocation
         WHERE allocation.invoice_no=?) AS invoice_refunds
FROM plugin_idc_core_refund_request request
JOIN plugin_idc_core_credit_exposure exposure
  ON exposure.invoice_no=request.invoice_no
JOIN plugin_idc_core_credit_account account
  ON account.credit_account_no=exposure.credit_account_no
WHERE request.refund_no=?`, "online_refund:"+created.RefundNo,
		checkout.InvoiceNo, created.RefundNo); err != nil {
		t.Fatalf("load completed credit online refund: %v", err)
	}
	if state.RefundStatus != "succeeded" || state.ExposureStatus != "released" ||
		state.ReleasedMinor != item.TotalMinor || state.UnbilledMinor != 0 ||
		state.AvailableMinor != checkout.TotalAmountMinor+10_000 ||
		state.RefundMovements != 1 || state.InvoiceRefunds != 0 {
		t.Fatalf("unexpected completed credit online refund state: %+v", state)
	}
	if _, err = refunds.PreviewOnlineRefund(scenario.ctx, previewInput); !bizerr.Is(
		err, billingservice.CodeRefundNotEligible,
	) {
		t.Fatalf("completed credit refund allowed a duplicate preview: %v", err)
	}
}

func TestOnlineRefundPostgreSQLPostsVerifiedChannelSuccess(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 2, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "online-refund-channel-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout channel refund order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	owner, err := newChannelPaymentOwnerTestService(t,
		scenario.database, newFinancialSettlementTestService(t, scenario),
	)
	if err != nil {
		t.Fatal(err)
	}
	paid, err := owner.ConfirmChannelPayment(scenario.ctx, billingcap.ConfirmedChannelPayment{
		PaymentAttemptNo: "PAT_refund_channel_0001", InvoiceNo: checkout.InvoiceNo,
		CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor,
		TotalAmountMinor: checkout.TotalAmountMinor, EvidenceDigest: strings.Repeat("a", 64),
	})
	if err != nil || paid == nil || paid.InvoiceStatus != "paid" {
		t.Fatalf("pay channel refund order=%+v err=%v", paid, err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_refund_policy_version
    (policy_no,currency_code,policy_version,high_amount_threshold_minor,
     automatic_approval_enabled,content_digest,configured_by)
VALUES ('refund-policy-channel-cny-1','CNY',1,1000000,TRUE,repeat('b',64),9001)`); err != nil {
		t.Fatalf("seed channel refund policy: %v", err)
	}
	var item struct {
		InvoiceItemNo string `orm:"invoice_item_no"`
		TotalMinor    int64  `orm:"total_minor"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &item, `
SELECT invoice_item_no,total_amount_minor AS total_minor
FROM plugin_idc_core_billing_invoice_item
WHERE invoice_no=? ORDER BY line_number LIMIT 1`, checkout.InvoiceNo); err != nil ||
		item.InvoiceItemNo == "" || item.TotalMinor != checkout.TotalAmountMinor {
		t.Fatalf("load channel refund item=%+v err=%v", item, err)
	}
	refunds, err := billingservice.NewAccountRefundService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 101, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1",
		}}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatal(err)
	}
	previewInput := billingservice.RefundPreviewInput{
		OrderNo: checkout.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: item.InvoiceItemNo,
			Quantity:      1, AmountMinor: item.TotalMinor / 2,
		}},
	}
	preview, err := refunds.PreviewOnlineRefund(scenario.ctx, previewInput)
	if err != nil || preview == nil || len(preview.Items) != 1 ||
		len(preview.Items[0].Sources) != 1 {
		t.Fatalf("preview channel refund=%+v err=%v", preview, err)
	}
	created, err := refunds.SubmitOnlineRefund(scenario.ctx, billingservice.RefundSubmissionInput{
		PreviewDigest: preview.ImpactDigest, Reason: billingservice.RefundReasonDuplicatePurchase,
		IdempotencyKey: "online-refund-channel-submit-0001", Preview: previewInput,
	})
	if err != nil || created == nil || created.Status != "approved" {
		t.Fatalf("submit channel refund=%+v err=%v", created, err)
	}
	preflight, err := owner.ClaimRefundExecution(scenario.ctx, billingcap.ClaimRefundExecution{
		WorkerNo: "refund-channel-test-worker", LeaseSeconds: 60,
	})
	if err != nil || preflight == nil || !preflight.HasWork ||
		preflight.Phase != billingcap.RefundExecutionPhasePreflight {
		t.Fatalf("claim channel refund preflight=%+v err=%v", preflight, err)
	}
	if _, err = owner.ReportRefundExecution(scenario.ctx, billingcap.ReportRefundExecution{
		WorkerNo: "refund-channel-test-worker", CommandNo: preflight.CommandNo,
		RequestDigest: preflight.RequestDigest, Phase: billingcap.RefundExecutionPhasePreflight,
		Status: billingcap.RefundExecutionResultReady, ResultCode: "preflight_ready",
		EvidenceDigest: strings.Repeat("c", 64),
	}); err != nil {
		t.Fatalf("report channel refund preflight: %v", err)
	}
	execution, err := owner.ClaimRefundExecution(scenario.ctx, billingcap.ClaimRefundExecution{
		WorkerNo: "refund-channel-test-worker", LeaseSeconds: 60,
	})
	if err != nil || execution == nil || !execution.HasWork ||
		execution.Phase != billingcap.RefundExecutionPhaseExecute {
		t.Fatalf("claim channel refund execution=%+v err=%v", execution, err)
	}
	reported, err := owner.ReportRefundExecution(scenario.ctx, billingcap.ReportRefundExecution{
		WorkerNo: "refund-channel-test-worker", CommandNo: execution.CommandNo,
		RequestDigest: execution.RequestDigest, Phase: billingcap.RefundExecutionPhaseExecute,
		Status: billingcap.RefundExecutionResultSucceeded, ResultCode: "channel_refund_succeeded",
		ResultReference: "provider-refund-reference-1", EvidenceDigest: strings.Repeat("d", 64),
	})
	if err != nil || reported == nil || reported.RefundStatusCode != "succeeded" {
		t.Fatalf("report channel refund success=%+v err=%v", reported, err)
	}
	replayed, err := owner.ReportRefundExecution(scenario.ctx, billingcap.ReportRefundExecution{
		WorkerNo: "refund-channel-test-worker", CommandNo: execution.CommandNo,
		RequestDigest: execution.RequestDigest, Phase: billingcap.RefundExecutionPhaseExecute,
		Status: billingcap.RefundExecutionResultSucceeded, ResultCode: "channel_refund_succeeded",
		ResultReference: "provider-refund-reference-1", EvidenceDigest: strings.Repeat("d", 64),
	})
	if err != nil || replayed == nil || !replayed.Reused || replayed.RefundStatusCode != "succeeded" {
		t.Fatalf("replay channel refund success=%+v err=%v", replayed, err)
	}
	var state struct {
		InvoiceStatus     string `orm:"invoice_status"`
		RefundedMinor     int64  `orm:"refunded_minor"`
		RefundAllocations int64  `orm:"refund_allocations"`
		LedgerEntries     int64  `orm:"ledger_entries"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &state, `
SELECT
  (SELECT status_code FROM plugin_idc_core_billing_invoice WHERE invoice_no=?) AS invoice_status,
  (SELECT refunded_amount_minor FROM plugin_idc_core_billing_invoice WHERE invoice_no=?) AS refunded_minor,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_refund_allocation
    WHERE refund_source_reference=? AND refund_source_type_code='channel_refund') AS refund_allocations,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_entry entry
    JOIN plugin_idc_core_billing_ledger_transaction transaction
      ON transaction.transaction_no=entry.transaction_no
    WHERE transaction.source_type_code='online_refund'
      AND transaction.source_reference=?) AS ledger_entries`,
		checkout.InvoiceNo, checkout.InvoiceNo, execution.CommandNo, execution.CommandNo); err != nil {
		t.Fatalf("load channel refund posting state: %v", err)
	}
	if state.InvoiceStatus != "partially_refunded" || state.RefundedMinor != item.TotalMinor/2 ||
		state.RefundAllocations != 1 || state.LedgerEntries != 2 {
		t.Fatalf("unexpected channel refund posting state: %+v", state)
	}

	unknownPreviewInput := billingservice.RefundPreviewInput{
		OrderNo: checkout.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: item.InvoiceItemNo,
			Quantity:      1,
			AmountMinor:   item.TotalMinor / 2,
		}},
	}
	unknownPreview, err := refunds.PreviewOnlineRefund(scenario.ctx, unknownPreviewInput)
	if err != nil || unknownPreview == nil {
		t.Fatalf("preview unknown-result refund=%+v err=%v", unknownPreview, err)
	}
	unknownRefund, err := refunds.SubmitOnlineRefund(
		scenario.ctx, billingservice.RefundSubmissionInput{
			PreviewDigest:  unknownPreview.ImpactDigest,
			Reason:         billingservice.RefundReasonServiceIssue,
			IdempotencyKey: "online-refund-channel-submit-unknown-0001",
			Preview:        unknownPreviewInput,
		},
	)
	if err != nil || unknownRefund == nil || unknownRefund.Status != "approved" {
		t.Fatalf("submit unknown-result refund=%+v err=%v", unknownRefund, err)
	}
	unknownPreflight, err := owner.ClaimRefundExecution(
		scenario.ctx, billingcap.ClaimRefundExecution{
			WorkerNo: "refund-channel-test-worker", LeaseSeconds: 60,
		},
	)
	if err != nil || unknownPreflight == nil || !unknownPreflight.HasWork ||
		unknownPreflight.RefundNo != unknownRefund.RefundNo ||
		unknownPreflight.Phase != billingcap.RefundExecutionPhasePreflight {
		t.Fatalf("claim unknown-result preflight=%+v err=%v", unknownPreflight, err)
	}
	if _, err = owner.ReportRefundExecution(scenario.ctx, billingcap.ReportRefundExecution{
		WorkerNo: "refund-channel-test-worker", CommandNo: unknownPreflight.CommandNo,
		RequestDigest: unknownPreflight.RequestDigest,
		Phase:         billingcap.RefundExecutionPhasePreflight,
		Status:        billingcap.RefundExecutionResultReady,
		ResultCode:    "preflight_ready", EvidenceDigest: strings.Repeat("e", 64),
	}); err != nil {
		t.Fatalf("report unknown-result preflight: %v", err)
	}
	unknownExecution, err := owner.ClaimRefundExecution(
		scenario.ctx, billingcap.ClaimRefundExecution{
			WorkerNo: "refund-channel-test-worker", LeaseSeconds: 60,
		},
	)
	if err != nil || unknownExecution == nil || !unknownExecution.HasWork ||
		unknownExecution.Phase != billingcap.RefundExecutionPhaseExecute {
		t.Fatalf("claim unknown-result execution=%+v err=%v", unknownExecution, err)
	}
	unknownReported, err := owner.ReportRefundExecution(
		scenario.ctx, billingcap.ReportRefundExecution{
			WorkerNo: "refund-channel-test-worker", CommandNo: unknownExecution.CommandNo,
			RequestDigest:  unknownExecution.RequestDigest,
			Phase:          billingcap.RefundExecutionPhaseExecute,
			Status:         billingcap.RefundExecutionResultUnknown,
			ResultCode:     "provider_result_unknown",
			EvidenceDigest: strings.Repeat("f", 64),
		},
	)
	if err != nil || unknownReported == nil ||
		unknownReported.RefundStatusCode != "result_unknown" {
		t.Fatalf("report unknown refund result=%+v err=%v", unknownReported, err)
	}
	var unknownState struct {
		Tasks             int64  `orm:"tasks"`
		TaskStatus        string `orm:"task_status"`
		Priority          int    `orm:"priority"`
		RefundAllocations int64  `orm:"refund_allocations"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &unknownState, `
SELECT
  (SELECT COUNT(*) FROM plugin_idc_core_async_task
    WHERE task_type='refund_result_reconciliation' AND subject_no=?) AS tasks,
  COALESCE((SELECT status_code FROM plugin_idc_core_async_task
    WHERE task_type='refund_result_reconciliation' AND subject_no=? LIMIT 1),'') AS task_status,
  COALESCE((SELECT priority FROM plugin_idc_core_async_task
    WHERE task_type='refund_result_reconciliation' AND subject_no=? LIMIT 1),0) AS priority,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_refund_allocation
    WHERE refund_source_reference=?) AS refund_allocations`,
		unknownExecution.CommandNo, unknownExecution.CommandNo,
		unknownExecution.CommandNo, unknownExecution.CommandNo); err != nil {
		t.Fatalf("load unknown refund reconciliation state: %v", err)
	}
	if unknownState.Tasks != 1 || unknownState.TaskStatus != "pending" ||
		unknownState.Priority != 100 || unknownState.RefundAllocations != 0 {
		t.Fatalf("unexpected unknown refund reconciliation state: %+v", unknownState)
	}
}

func TestOnlineRefundPostgreSQLKeepsRejectedSourcePendingForManualReview(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "online-refund-rejected-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout rejected refund order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	owner, err := newChannelPaymentOwnerTestService(
		t, scenario.database, newFinancialSettlementTestService(t, scenario),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.ConfirmChannelPayment(scenario.ctx, billingcap.ConfirmedChannelPayment{
		PaymentAttemptNo: "PAT_refund_rejected_0001", InvoiceNo: checkout.InvoiceNo,
		CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor,
		TotalAmountMinor: checkout.TotalAmountMinor, EvidenceDigest: strings.Repeat("1", 64),
	}); err != nil {
		t.Fatalf("pay rejected refund order: %v", err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_refund_policy_version
    (policy_no,currency_code,policy_version,high_amount_threshold_minor,
     automatic_approval_enabled,content_digest,configured_by)
VALUES ('refund-policy-rejected-cny-1','CNY',1,1000000,TRUE,repeat('2',64),9001)`); err != nil {
		t.Fatalf("seed rejected refund policy: %v", err)
	}
	var item struct {
		InvoiceItemNo string `orm:"invoice_item_no"`
		TotalMinor    int64  `orm:"total_minor"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &item, `
SELECT invoice_item_no,total_amount_minor AS total_minor
FROM plugin_idc_core_billing_invoice_item
WHERE invoice_no=? ORDER BY line_number LIMIT 1`, checkout.InvoiceNo); err != nil {
		t.Fatalf("load rejected refund item: %v", err)
	}
	refunds, err := billingservice.NewAccountRefundService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 101, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1",
		}}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatal(err)
	}
	previewInput := billingservice.RefundPreviewInput{
		OrderNo: checkout.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: item.InvoiceItemNo,
			Quantity:      1, AmountMinor: item.TotalMinor,
		}},
	}
	preview, err := refunds.PreviewOnlineRefund(scenario.ctx, previewInput)
	if err != nil || preview == nil {
		t.Fatalf("preview rejected refund=%+v err=%v", preview, err)
	}
	created, err := refunds.SubmitOnlineRefund(scenario.ctx, billingservice.RefundSubmissionInput{
		PreviewDigest: preview.ImpactDigest, Reason: billingservice.RefundReasonServiceIssue,
		IdempotencyKey: "online-refund-rejected-submit-0001", Preview: previewInput,
	})
	if err != nil || created == nil || created.Status != "approved" {
		t.Fatalf("submit rejected refund=%+v err=%v", created, err)
	}
	preflight, err := owner.ClaimRefundExecution(scenario.ctx, billingcap.ClaimRefundExecution{
		WorkerNo: "refund-rejected-test-worker", LeaseSeconds: 60,
	})
	if err != nil || preflight == nil || !preflight.HasWork {
		t.Fatalf("claim rejected refund preflight=%+v err=%v", preflight, err)
	}
	if _, err = owner.ReportRefundExecution(scenario.ctx, billingcap.ReportRefundExecution{
		WorkerNo: "refund-rejected-test-worker", CommandNo: preflight.CommandNo,
		RequestDigest: preflight.RequestDigest, Phase: billingcap.RefundExecutionPhasePreflight,
		Status: billingcap.RefundExecutionResultReady, ResultCode: "preflight_ready",
		EvidenceDigest: strings.Repeat("3", 64),
	}); err != nil {
		t.Fatalf("report rejected refund preflight: %v", err)
	}
	execution, err := owner.ClaimRefundExecution(scenario.ctx, billingcap.ClaimRefundExecution{
		WorkerNo: "refund-rejected-test-worker", LeaseSeconds: 60,
	})
	if err != nil || execution == nil || !execution.HasWork {
		t.Fatalf("claim rejected refund execution=%+v err=%v", execution, err)
	}
	reported, err := owner.ReportRefundExecution(scenario.ctx, billingcap.ReportRefundExecution{
		WorkerNo: "refund-rejected-test-worker", CommandNo: execution.CommandNo,
		RequestDigest: execution.RequestDigest, Phase: billingcap.RefundExecutionPhaseExecute,
		Status: billingcap.RefundExecutionResultFailed, ResultCode: "provider_rejected",
		EvidenceDigest: strings.Repeat("4", 64),
	})
	if err != nil || reported == nil || reported.RefundStatusCode != "manual_confirmation" {
		t.Fatalf("report rejected refund=%+v err=%v", reported, err)
	}
	blocked, err := owner.ClaimRefundExecution(scenario.ctx, billingcap.ClaimRefundExecution{
		WorkerNo: "refund-rejected-test-worker", LeaseSeconds: 60,
	})
	if err != nil || blocked == nil || blocked.HasWork {
		t.Fatalf("rejected refund released more work=%+v err=%v", blocked, err)
	}
	var state struct {
		RefundStatus string `orm:"refund_status"`
		Tasks        int64  `orm:"tasks"`
		Priority     int    `orm:"priority"`
		Allocations  int64  `orm:"allocations"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &state, `
SELECT
  (SELECT status_code FROM plugin_idc_core_refund_request WHERE refund_no=?) AS refund_status,
  (SELECT COUNT(*) FROM plugin_idc_core_async_task
    WHERE task_type='refund_result_reconciliation' AND subject_no=?) AS tasks,
  COALESCE((SELECT priority FROM plugin_idc_core_async_task
    WHERE task_type='refund_result_reconciliation' AND subject_no=? LIMIT 1),0) AS priority,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_refund_allocation
    WHERE refund_source_reference=?) AS allocations`, created.RefundNo, execution.CommandNo,
		execution.CommandNo, execution.CommandNo); err != nil {
		t.Fatalf("load rejected refund state: %v", err)
	}
	if state.RefundStatus != "manual_confirmation" || state.Tasks != 1 ||
		state.Priority != 100 || state.Allocations != 0 {
		t.Fatalf("unexpected rejected refund state: %+v", state)
	}
}

func TestOnlineRefundPostgreSQLRequiresFreshLoginForHighAmountApproval(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 2, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "online-refund-review-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout reviewed refund order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balance, err := billingservice.NewBalanceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY",
		AmountMinor: checkout.TotalAmountMinor, SourceReference: "online-refund-review-recharge-0001",
		SourceSnapshotDigest: strings.Repeat("4", 64),
	}); err != nil {
		t.Fatalf("fund reviewed refund order: %v", err)
	}
	invoices, err := billingservice.NewInvoiceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = invoices.PayFromBalance(scenario.ctx, billingservice.InvoiceBalancePaymentInput{
		InvoiceNo: checkout.InvoiceNo, SourceReference: "online-refund-review-payment-0001",
		SourceSnapshotDigest: strings.Repeat("5", 64),
	}); err != nil {
		t.Fatalf("pay reviewed refund order: %v", err)
	}
	const reviewerID = 9002
	recentAuth := &refundRecentAuthStub{actor: reviewerID}
	management, err := billingservice.NewManagementRefundService(
		scenario.database, refundManagementAuthorizerStub{actor: reviewerID}, recentAuth,
		refundBusinessAuditStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := management.ConfigureRefundPolicy(scenario.ctx, billingservice.RefundPolicyInput{
		CurrencyCode: "CNY", HighAmountThresholdMinor: 1,
		AutomaticApprovalEnabled: false,
	}, 0)
	if err != nil || policy == nil || policy.Version != 1 || policy.ConfiguredBy != reviewerID {
		t.Fatalf("configure reviewed refund policy: policy=%+v err=%v", policy, err)
	}
	var item struct {
		InvoiceItemNo string `orm:"invoice_item_no"`
		TotalMinor    int64  `orm:"total_minor"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &item, `
SELECT invoice_item_no,total_amount_minor AS total_minor
FROM plugin_idc_core_billing_invoice_item
WHERE invoice_no=? ORDER BY line_number LIMIT 1`, checkout.InvoiceNo); err != nil ||
		item.InvoiceItemNo == "" || item.TotalMinor < 4 {
		t.Fatalf("load reviewed refund item: item=%+v err=%v", item, err)
	}
	refunds, err := billingservice.NewAccountRefundService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 101, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1",
		}}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatal(err)
	}
	firstPreviewInput := billingservice.RefundPreviewInput{
		OrderNo: checkout.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: item.InvoiceItemNo,
			Quantity:      1,
			AmountMinor:   item.TotalMinor / 2,
		}},
	}
	firstPreview, err := refunds.PreviewOnlineRefund(scenario.ctx, firstPreviewInput)
	if err != nil || firstPreview.ApprovalMode != "manual" || !firstPreview.HighAmount {
		t.Fatalf("preview reviewed refund: preview=%+v err=%v", firstPreview, err)
	}
	pending, err := refunds.SubmitOnlineRefund(scenario.ctx, billingservice.RefundSubmissionInput{
		PreviewDigest:  firstPreview.ImpactDigest,
		Reason:         billingservice.RefundReasonNoLongerNeeded,
		IdempotencyKey: "online-refund-review-submit-0001",
		Preview:        firstPreviewInput,
	})
	if err != nil || pending.Status != "pending_review" || pending.Version != 1 {
		t.Fatalf("submit reviewed refund: refund=%+v err=%v", pending, err)
	}
	approve := billingservice.ManagementRefundReviewIntent{
		RequestNo: "online-refund-approve-0001", RefundNo: pending.RefundNo,
		ExpectedVersion: 1, Decision: billingservice.RefundReviewApprove,
		EvidenceDigest: strings.Repeat("6", 64),
	}
	if _, err = management.ReviewRefund(scenario.ctx, approve, ""); !bizerr.Is(
		err, billingservice.CodeRefundInvalid,
	) {
		t.Fatalf("high amount refund approved without fresh login proof: %v", err)
	}
	proof, err := management.IssueRefundReviewProof(scenario.ctx, approve)
	if err != nil || proof == nil || proof.Token == "" ||
		recentAuth.binding.Purpose != reauth.PurposeHighValueFunds {
		t.Fatalf("issue high amount refund proof: proof=%+v err=%v", proof, err)
	}
	approved, err := management.ReviewRefund(scenario.ctx, approve, proof.Token)
	if err != nil || approved == nil || approved.Status != "approved" ||
		approved.Version != 2 || approved.ApprovedAt == nil {
		t.Fatalf("approve high amount refund: refund=%+v err=%v", approved, err)
	}
	replayed, err := management.ReviewRefund(scenario.ctx, approve, "")
	if err != nil || replayed == nil || !replayed.Reused || replayed.RefundNo != approved.RefundNo {
		t.Fatalf("replay high amount approval: refund=%+v err=%v", replayed, err)
	}

	secondPreviewInput := billingservice.RefundPreviewInput{
		OrderNo: checkout.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: item.InvoiceItemNo,
			Quantity:      1,
			AmountMinor:   item.TotalMinor / 2,
		}},
	}
	secondPreview, err := refunds.PreviewOnlineRefund(scenario.ctx, secondPreviewInput)
	if err != nil {
		t.Fatalf("preview refund for rejection: %v", err)
	}
	secondPending, err := refunds.SubmitOnlineRefund(scenario.ctx, billingservice.RefundSubmissionInput{
		PreviewDigest:  secondPreview.ImpactDigest,
		Reason:         billingservice.RefundReasonDuplicatePurchase,
		IdempotencyKey: "online-refund-review-submit-0002",
		Preview:        secondPreviewInput,
	})
	if err != nil || secondPending.Status != "pending_review" {
		t.Fatalf("submit refund for rejection: refund=%+v err=%v", secondPending, err)
	}
	rejected, err := management.ReviewRefund(scenario.ctx, billingservice.ManagementRefundReviewIntent{
		RequestNo: "online-refund-reject-0001", RefundNo: secondPending.RefundNo,
		ExpectedVersion: 1, Decision: billingservice.RefundReviewReject,
		CommentDigest: strings.Repeat("7", 64), EvidenceDigest: strings.Repeat("8", 64),
	}, "")
	if err != nil || rejected == nil || rejected.Status != "rejected" ||
		rejected.Version != 2 || rejected.CompletedAt == nil {
		t.Fatalf("reject reviewed refund: refund=%+v err=%v", rejected, err)
	}
	commandCount, err := scenario.database.GetValue(scenario.ctx, `
SELECT COUNT(*) FROM plugin_idc_core_refund_execution_command
WHERE refund_no IN (?,?)`, approved.RefundNo, rejected.RefundNo)
	if err != nil || commandCount.Int64() != 1 {
		t.Fatalf("approved/rejected refund command count=%d err=%v", commandCount.Int64(), err)
	}

	firstPage, err := refunds.ListOnlineRefunds(scenario.ctx, billingservice.RefundDirectoryFilter{Limit: 1})
	if err != nil || firstPage == nil || len(firstPage.Refunds) != 1 ||
		firstPage.Refunds[0].RefundNo != secondPending.RefundNo || firstPage.NextCursor == "" {
		t.Fatalf("list first customer refund page: page=%+v err=%v", firstPage, err)
	}
	secondPage, err := refunds.ListOnlineRefunds(scenario.ctx, billingservice.RefundDirectoryFilter{
		Cursor: firstPage.NextCursor, Limit: 1,
	})
	if err != nil || secondPage == nil || len(secondPage.Refunds) != 1 ||
		secondPage.Refunds[0].RefundNo != pending.RefundNo || secondPage.NextCursor != "" {
		t.Fatalf("list second customer refund page: page=%+v err=%v", secondPage, err)
	}
	foreignRefunds, err := billingservice.NewAccountRefundService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 202, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-2",
		}}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatal(err)
	}
	foreignPage, err := foreignRefunds.ListOnlineRefunds(
		scenario.ctx, billingservice.RefundDirectoryFilter{Limit: 20},
	)
	if err != nil || foreignPage == nil || len(foreignPage.Refunds) != 0 {
		t.Fatalf("foreign customer refund directory leaked rows: page=%+v err=%v", foreignPage, err)
	}

	adminPage, err := management.ListRefunds(scenario.ctx, billingservice.RefundDirectoryFilter{
		Status: "rejected", Limit: 20,
	})
	if err != nil || adminPage == nil || len(adminPage.Refunds) != 1 ||
		adminPage.Refunds[0].RefundNo != secondPending.RefundNo ||
		adminPage.Refunds[0].CustomerNo != "customer-commerce-service-1" {
		t.Fatalf("list staff rejected refunds: page=%+v err=%v", adminPage, err)
	}
	adminDetail, err := management.GetRefundDetail(scenario.ctx, secondPending.RefundNo)
	if err != nil || adminDetail == nil || adminDetail.Refund == nil ||
		adminDetail.Refund.RefundNo != secondPending.RefundNo || len(adminDetail.Items) != 1 ||
		adminDetail.Items[0].InvoiceItemNo != item.InvoiceItemNo ||
		adminDetail.Items[0].RefundItemNo == "" {
		t.Fatalf("load immutable staff refund detail: detail=%+v err=%v", adminDetail, err)
	}
}

func TestOnlineRefundPostgreSQLKeepsServiceLockedForHighPriorityRecovery(t *testing.T) {
	t.Skip("migration 087 removes server refund targets; generic entitlement coverage is exercised separately")
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo:        scenario.quote.QuoteNo,
		IdempotencyKey: "online-refund-service-recovery-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout service recovery refund order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balances, err := billingservice.NewBalanceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balances.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY",
		AmountMinor:          checkout.TotalAmountMinor,
		SourceReference:      "online-refund-service-recovery-recharge-0001",
		SourceSnapshotDigest: strings.Repeat("1", 64),
	}); err != nil {
		t.Fatalf("fund service recovery refund: %v", err)
	}
	invoices, err := billingservice.NewInvoiceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = invoices.PayFromBalance(scenario.ctx, billingservice.InvoiceBalancePaymentInput{
		InvoiceNo:            checkout.InvoiceNo,
		SourceReference:      "online-refund-service-recovery-payment-0001",
		SourceSnapshotDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatalf("pay service recovery refund order: %v", err)
	}
	const serviceNo = "service-refund-recovery-0001"
	if _, err = scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_service_instance (
    service_no, customer_no, source_code, source_fulfillment_no,
    order_no, order_item_no, product_no, version_no, billing_rule_no,
    configuration_snapshot_no, product_type_code, currency_code,
    binding_no, pool_no, entitlement_status_code, runtime_status_code,
    provider_task_status_code, renewal_mode_code, period_unit_code,
    period_count, entitlement_started_at, entitlement_expires_at
)
SELECT ?, order_row.customer_no, 'order', fulfillment.fulfillment_no,
       fulfillment.order_no, fulfillment.order_item_no,
       item.product_no, item.version_no, item.billing_rule_no,
       item.configuration_snapshot_no, item.product_type_code, item.currency_code,
       item.initial_binding_no, binding.pool_no, 'active', 'running',
       'succeeded', 'unresolved', item.period_unit_code, item.period_count,
       CURRENT_TIMESTAMP,
       CASE WHEN item.period_unit_code = 'once' THEN NULL
            ELSE CURRENT_TIMESTAMP + INTERVAL '1 year' END
FROM plugin_idc_core_commerce_fulfillment_unit fulfillment
JOIN plugin_idc_core_commerce_order_item item
  ON item.order_no=fulfillment.order_no AND item.order_item_no=fulfillment.order_item_no
JOIN plugin_idc_core_commerce_order order_row
  ON order_row.order_no=fulfillment.order_no
JOIN plugin_idc_core_catalog_resource_pool_binding binding
  ON binding.binding_no=item.initial_binding_no AND binding.version_no=item.version_no
WHERE fulfillment.order_no=?
ORDER BY fulfillment.unit_sequence LIMIT 1`, serviceNo, checkout.OrderNo); err != nil {
		t.Fatalf("seed active refunded service: %v", err)
	}
	const reviewerID = 9010
	management, err := billingservice.NewManagementRefundService(
		scenario.database, refundManagementAuthorizerStub{actor: reviewerID},
		&refundRecentAuthStub{actor: reviewerID}, refundBusinessAuditStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = management.ConfigureRefundPolicy(
		scenario.ctx, billingservice.RefundPolicyInput{
			CurrencyCode: "CNY", HighAmountThresholdMinor: checkout.TotalAmountMinor + 1,
			AutomaticApprovalEnabled: true,
		}, 0,
	); err != nil {
		t.Fatalf("configure service recovery refund policy: %v", err)
	}
	var item struct {
		InvoiceItemNo string `orm:"invoice_item_no"`
		TotalMinor    int64  `orm:"total_minor"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &item, `
SELECT invoice_item_no,total_amount_minor AS total_minor
FROM plugin_idc_core_billing_invoice_item
WHERE invoice_no=? ORDER BY line_number LIMIT 1`, checkout.InvoiceNo); err != nil {
		t.Fatalf("load service recovery refund item: %v", err)
	}
	refunds, err := billingservice.NewAccountRefundService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 101, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1",
		}}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatal(err)
	}
	previewInput := billingservice.RefundPreviewInput{
		OrderNo: checkout.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo:  item.InvoiceItemNo,
			EntitlementNos: []string{serviceNo},
			Quantity:       1, AmountMinor: item.TotalMinor,
		}},
	}
	preview, err := refunds.PreviewOnlineRefund(scenario.ctx, previewInput)
	if err != nil || preview == nil || preview.ApprovalMode != "manual" ||
		!preview.RequiresEntitlementTermination || !preview.HasResourceImpact {
		t.Fatalf("preview active service refund=%+v err=%v", preview, err)
	}
	pending, err := refunds.SubmitOnlineRefund(scenario.ctx, billingservice.RefundSubmissionInput{
		PreviewDigest: preview.ImpactDigest, Reason: billingservice.RefundReasonServiceIssue,
		IdempotencyKey: "online-refund-service-recovery-submit-0001", Preview: previewInput,
	})
	if err != nil || pending == nil || pending.Status != "pending_review" {
		t.Fatalf("submit active service refund=%+v err=%v", pending, err)
	}
	approved, err := management.ReviewRefund(scenario.ctx, billingservice.ManagementRefundReviewIntent{
		RequestNo: "online-refund-service-recovery-approve-0001",
		RefundNo:  pending.RefundNo, ExpectedVersion: pending.Version,
		Decision:       billingservice.RefundReviewApprove,
		EvidenceDigest: strings.Repeat("3", 64),
	}, "")
	if err != nil || approved == nil || approved.Status != "approved" {
		t.Fatalf("approve active service refund=%+v err=%v", approved, err)
	}
	worker, err := newLocalRefundWorkerTestService(t, scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	processed, err := worker.Run(scenario.ctx, 20)
	if err != nil || processed != 1 {
		t.Fatalf("post active service refund: processed=%d err=%v", processed, err)
	}
	var state struct {
		RefundStatus   string `orm:"refund_status"`
		RuntimeStatus  string `orm:"runtime_status"`
		ActiveLocks    int64  `orm:"active_locks"`
		RecoveryTasks  int64  `orm:"recovery_tasks"`
		Priority       int    `orm:"priority"`
		RecoveryEvents int64  `orm:"recovery_events"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &state, `
SELECT
  (SELECT status_code FROM plugin_idc_core_refund_request WHERE refund_no=?) AS refund_status,
  (SELECT runtime_status_code FROM plugin_idc_core_service_instance WHERE service_no=?) AS runtime_status,
  (SELECT COUNT(*) FROM plugin_idc_core_refund_service_lock
    WHERE refund_no=? AND service_no=? AND released_at IS NULL) AS active_locks,
  (SELECT COUNT(*) FROM plugin_idc_core_async_task
    WHERE task_type='refund_service_recovery' AND subject_no=?) AS recovery_tasks,
  COALESCE((SELECT priority FROM plugin_idc_core_async_task
    WHERE task_type='refund_service_recovery' AND subject_no=? LIMIT 1),0) AS priority,
  (SELECT COUNT(*) FROM plugin_idc_core_outbox_event
    WHERE event_type='billing.refund.service_recovery_required'
      AND aggregate_no=? AND correlation_no=?) AS recovery_events`,
		pending.RefundNo, serviceNo, pending.RefundNo, serviceNo,
		serviceNo, serviceNo, serviceNo, pending.RefundNo); err != nil {
		t.Fatalf("load service recovery linkage state: %v", err)
	}
	if state.RefundStatus != "succeeded" || state.RuntimeStatus != "running" ||
		state.ActiveLocks != 1 || state.RecoveryTasks != 1 ||
		state.Priority != 100 || state.RecoveryEvents != 1 {
		t.Fatalf("unexpected service recovery linkage state: %+v", state)
	}
}

func TestOnlineRefundPostgreSQLHoldsMixedFundingUntilBalanceConsent(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "online-refund-mixed-checkout-0001",
	})
	if err != nil || checkout.TotalAmountMinor < 2 {
		t.Fatalf("checkout mixed refund order=%+v err=%v", checkout, err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balances, err := billingservice.NewBalanceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	balanceMinor := checkout.TotalAmountMinor / 3
	if _, err = balances.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY",
		AmountMinor: balanceMinor, SourceReference: "online-refund-mixed-recharge-0001",
		SourceSnapshotDigest: strings.Repeat("9", 64),
	}); err != nil {
		t.Fatalf("fund mixed refund balance: %v", err)
	}
	hold, err := balances.CreateHold(scenario.ctx, billingservice.HoldCreateInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY",
		AmountMinor: balanceMinor, SourceType: "payment_attempt",
		SourceReference: "PAT_refund_mixed_0001", SourceSnapshotDigest: strings.Repeat("a", 64),
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("hold mixed refund balance: %v", err)
	}
	owner, err := newChannelPaymentOwnerTestService(t,
		scenario.database, newFinancialSettlementTestService(t, scenario),
	)
	if err != nil {
		t.Fatal(err)
	}
	channelMinor := checkout.TotalAmountMinor - balanceMinor
	paid, err := owner.ConfirmChannelPayment(scenario.ctx, billingcap.ConfirmedChannelPayment{
		PaymentAttemptNo: "PAT_refund_mixed_0001", InvoiceNo: checkout.InvoiceNo,
		CurrencyCode: "CNY", AmountMinor: channelMinor,
		TotalAmountMinor: checkout.TotalAmountMinor, BalanceHoldNo: hold.HoldNo,
		BalanceAmountMinor: balanceMinor, EvidenceDigest: strings.Repeat("b", 64),
	})
	if err != nil || paid == nil || paid.InvoiceStatus != "paid" ||
		paid.BalanceAllocationNo == "" || paid.AllocationNo == "" {
		t.Fatalf("pay mixed refund order=%+v err=%v", paid, err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_refund_policy_version
    (policy_no,currency_code,policy_version,high_amount_threshold_minor,
     automatic_approval_enabled,content_digest,configured_by)
VALUES ('refund-policy-mixed-cny-1','CNY',1,1000000,TRUE,repeat('c',64),9001)`); err != nil {
		t.Fatalf("seed mixed refund policy: %v", err)
	}
	var item struct {
		InvoiceItemNo string `orm:"invoice_item_no"`
		TotalMinor    int64  `orm:"total_minor"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &item, `
SELECT invoice_item_no,total_amount_minor AS total_minor
FROM plugin_idc_core_billing_invoice_item
WHERE invoice_no=? ORDER BY line_number LIMIT 1`, checkout.InvoiceNo); err != nil ||
		item.InvoiceItemNo == "" || item.TotalMinor != checkout.TotalAmountMinor {
		t.Fatalf("load mixed refund item=%+v err=%v", item, err)
	}
	refunds, err := billingservice.NewAccountRefundService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 101, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1",
		}}, refundBusinessAuditStub{}, newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatal(err)
	}
	previewInput := billingservice.RefundPreviewInput{
		OrderNo: checkout.OrderNo,
		Selections: []billingservice.RefundSelectionInput{{
			InvoiceItemNo: item.InvoiceItemNo,
			Quantity:      1, AmountMinor: item.TotalMinor,
		}},
	}
	preview, err := refunds.PreviewOnlineRefund(scenario.ctx, previewInput)
	if err != nil || preview == nil || len(preview.Items) != 1 ||
		len(preview.Items[0].Sources) != 2 {
		t.Fatalf("preview mixed refund=%+v err=%v", preview, err)
	}
	created, err := refunds.SubmitOnlineRefund(scenario.ctx, billingservice.RefundSubmissionInput{
		PreviewDigest: preview.ImpactDigest, Reason: billingservice.RefundReasonDuplicatePurchase,
		IdempotencyKey: "online-refund-mixed-submit-0001", Preview: previewInput,
	})
	if err != nil || created == nil || created.Status != "approved" {
		t.Fatalf("submit mixed refund=%+v err=%v", created, err)
	}
	localWorker, err := newLocalRefundWorkerTestService(t, scenario.database)
	if err != nil {
		t.Fatalf("construct local refund worker: %v", err)
	}
	processed, err := localWorker.Run(scenario.ctx, 20)
	if err != nil || processed != 0 {
		t.Fatalf("local source escaped before channel preflight: processed=%d err=%v", processed, err)
	}
	work, err := owner.ClaimRefundExecution(scenario.ctx, billingcap.ClaimRefundExecution{
		WorkerNo: "refund-mixed-test-worker", LeaseSeconds: 60,
	})
	if err != nil || work == nil || !work.HasWork ||
		work.RefundNo != created.RefundNo || work.Phase != billingcap.RefundExecutionPhasePreflight {
		t.Fatalf("claim mixed refund preflight=%+v err=%v", work, err)
	}
	reported, err := owner.ReportRefundExecution(scenario.ctx, billingcap.ReportRefundExecution{
		WorkerNo: "refund-mixed-test-worker", CommandNo: work.CommandNo,
		RequestDigest: work.RequestDigest, Phase: billingcap.RefundExecutionPhasePreflight,
		Status:          billingcap.RefundExecutionResultFailed,
		ResultCode:      "original_channel_unsupported",
		ResultReference: "unsupported-by-provider", EvidenceDigest: strings.Repeat("d", 64),
	})
	if err != nil || reported == nil || reported.RefundStatusCode != "awaiting_balance_consent" {
		t.Fatalf("report unsupported original channel=%+v err=%v", reported, err)
	}
	replayedReport, err := owner.ReportRefundExecution(scenario.ctx, billingcap.ReportRefundExecution{
		WorkerNo: "refund-mixed-test-worker", CommandNo: work.CommandNo,
		RequestDigest: work.RequestDigest, Phase: billingcap.RefundExecutionPhasePreflight,
		Status:          billingcap.RefundExecutionResultFailed,
		ResultCode:      "original_channel_unsupported",
		ResultReference: "unsupported-by-provider", EvidenceDigest: strings.Repeat("d", 64),
	})
	if err != nil || replayedReport == nil || !replayedReport.Reused ||
		replayedReport.RefundStatusCode != "awaiting_balance_consent" {
		t.Fatalf("replay unsupported original channel=%+v err=%v", replayedReport, err)
	}
	blockedWork, err := owner.ClaimRefundExecution(scenario.ctx, billingcap.ClaimRefundExecution{
		WorkerNo: "refund-mixed-test-worker", LeaseSeconds: 60,
	})
	if err != nil || blockedWork == nil || blockedWork.HasWork {
		t.Fatalf("mixed refund released before customer consent=%+v err=%v", blockedWork, err)
	}

	refused, err := refunds.RespondRefundBalanceConsent(scenario.ctx, billingservice.RefundBalanceConsentInput{
		RefundNo: created.RefundNo, ExpectedVersion: 2, Consent: false,
		IdempotencyKey: "online-refund-mixed-refuse-0001",
	})
	if err != nil || refused == nil || refused.Status != "awaiting_balance_consent" ||
		refused.Version != 3 {
		t.Fatalf("refuse mixed refund balance=%+v err=%v", refused, err)
	}
	var refusedState struct {
		Commands  int64 `orm:"commands"`
		Pending   int64 `orm:"pending"`
		Succeeded int64 `orm:"succeeded"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &refusedState, `
SELECT COUNT(*) AS commands,
       COUNT(*) FILTER (WHERE status_code='pending') AS pending,
       COUNT(*) FILTER (WHERE status_code='succeeded') AS succeeded
FROM plugin_idc_core_refund_execution_command WHERE refund_no=?`,
		created.RefundNo); err != nil {
		t.Fatalf("load refused mixed refund commands: %v", err)
	}
	if refusedState.Commands != 2 || refusedState.Pending != 1 || refusedState.Succeeded != 0 {
		t.Fatalf("mixed refund executed before consent: %+v", refusedState)
	}

	consented, err := refunds.RespondRefundBalanceConsent(scenario.ctx, billingservice.RefundBalanceConsentInput{
		RefundNo: created.RefundNo, ExpectedVersion: 3, Consent: true,
		IdempotencyKey: "online-refund-mixed-consent-0001",
	})
	if err != nil || consented == nil || consented.Status != "approved" ||
		consented.Destination != "mixed" || consented.Version != 4 {
		t.Fatalf("consent mixed refund balance=%+v err=%v", consented, err)
	}
	replayed, err := refunds.RespondRefundBalanceConsent(scenario.ctx, billingservice.RefundBalanceConsentInput{
		RefundNo: created.RefundNo, ExpectedVersion: 3, Consent: true,
		IdempotencyKey: "online-refund-mixed-consent-0001",
	})
	if err != nil || replayed == nil || !replayed.Reused || replayed.Version != 4 {
		t.Fatalf("replay mixed refund consent=%+v err=%v", replayed, err)
	}
	var consentedState struct {
		Commands               int64 `orm:"commands"`
		OriginalPending        int64 `orm:"original_pending"`
		UnsupportedOriginal    int64 `orm:"unsupported_original"`
		BalanceFallbackPending int64 `orm:"balance_fallback_pending"`
		Succeeded              int64 `orm:"succeeded"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &consentedState, `
SELECT COUNT(*) AS commands,
       COUNT(*) FILTER (WHERE destination_code='original_channel' AND status_code='pending') AS original_pending,
       COUNT(*) FILTER (WHERE destination_code='original_channel' AND status_code='failed'
                         AND result_code='original_channel_unsupported') AS unsupported_original,
       COUNT(*) FILTER (WHERE destination_code='same_currency_balance' AND status_code='pending') AS balance_fallback_pending,
       COUNT(*) FILTER (WHERE status_code='succeeded') AS succeeded
FROM plugin_idc_core_refund_execution_command WHERE refund_no=?`,
		created.RefundNo); err != nil {
		t.Fatalf("load consented mixed refund commands: %v", err)
	}
	if consentedState.Commands != 3 || consentedState.OriginalPending != 1 ||
		consentedState.UnsupportedOriginal != 1 || consentedState.BalanceFallbackPending != 1 ||
		consentedState.Succeeded != 0 {
		t.Fatalf("mixed refund consent did not release atomically: %+v", consentedState)
	}
	processed, err = localWorker.Run(scenario.ctx, 20)
	if err != nil || processed != 2 {
		t.Fatalf("post mixed local refund sources: processed=%d err=%v", processed, err)
	}
	processed, err = localWorker.Run(scenario.ctx, 20)
	if err != nil || processed != 0 {
		t.Fatalf("replayed local refund worker created duplicate work: processed=%d err=%v", processed, err)
	}
	var completedState struct {
		RefundStatus       string `orm:"refund_status"`
		InvoiceStatus      string `orm:"invoice_status"`
		RefundedMinor      int64  `orm:"refunded_minor"`
		LedgerTransactions int64  `orm:"ledger_transactions"`
		RefundAllocations  int64  `orm:"refund_allocations"`
		RefundEvents       int64  `orm:"refund_events"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &completedState, `
SELECT
  (SELECT status_code FROM plugin_idc_core_refund_request WHERE refund_no=?) AS refund_status,
  (SELECT status_code FROM plugin_idc_core_billing_invoice WHERE invoice_no=?) AS invoice_status,
  (SELECT refunded_amount_minor FROM plugin_idc_core_billing_invoice WHERE invoice_no=?) AS refunded_minor,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_transaction
    WHERE source_type_code='online_refund') AS ledger_transactions,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_refund_allocation allocation
    JOIN plugin_idc_core_refund_execution_command command
      ON command.command_no=allocation.refund_source_reference
    WHERE command.refund_no=? AND allocation.direction_code='apply') AS refund_allocations,
  (SELECT COUNT(*) FROM plugin_idc_core_outbox_event
    WHERE aggregate_no=? AND event_type IN (
      'billing.refund.source_posted','billing.refund.succeeded',
      'billing.refund.commission_reversal_requested','billing.refund.tax_adjustment_requested'
    )) AS refund_events`,
		created.RefundNo, checkout.InvoiceNo, checkout.InvoiceNo,
		created.RefundNo, created.RefundNo); err != nil {
		t.Fatalf("load completed mixed refund linkage: %v", err)
	}
	if completedState.RefundStatus != "succeeded" ||
		completedState.InvoiceStatus != "refunded" ||
		completedState.RefundedMinor != checkout.TotalAmountMinor ||
		completedState.LedgerTransactions != 2 ||
		completedState.RefundAllocations != 2 || completedState.RefundEvents != 5 {
		t.Fatalf("unexpected completed mixed refund linkage: %+v", completedState)
	}
}
