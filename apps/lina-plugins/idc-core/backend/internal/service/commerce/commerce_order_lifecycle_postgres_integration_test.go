package commerce

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func TestOrderCancellationAtomicAndReplaySafePostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 2, true, 0)
	enableOrderLifecycleMarketingReversal(t, scenario.service, scenario.database)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-before-cancel-0001",
	})
	if err != nil {
		t.Fatalf("checkout before cancellation: %v", err)
	}
	seedConsumedMarketingQuotaAndAsset(t, scenario, checkout.OrderNo)

	request := OrderCancellationRequest{
		OrderNo: checkout.OrderNo, IdempotencyKey: "cancel-concurrent-0001",
	}
	const workers = 8
	start := make(chan struct{})
	results := make([]*OrderCancellation, workers)
	errorsFound := make([]error, workers)
	var wait sync.WaitGroup
	for index := range workers {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errorsFound[index] = scenario.service.CancelAccountOrder(scenario.ctx, request)
		}(index)
	}
	close(start)
	wait.Wait()
	cancellationNo := ""
	for index := range workers {
		if errorsFound[index] != nil || results[index] == nil ||
			results[index].Status != OrderCancellationStatusCompleted {
			t.Fatalf("concurrent cancellation %d result=%+v err=%v", index, results[index], errorsFound[index])
		}
		if cancellationNo == "" {
			cancellationNo = results[index].CancellationNo
		} else if results[index].CancellationNo != cancellationNo {
			t.Fatalf("cancellation replay diverged: %q != %q", results[index].CancellationNo, cancellationNo)
		}
	}
	if _, err = scenario.service.CancelAccountOrder(scenario.ctx, OrderCancellationRequest{
		OrderNo: checkout.OrderNo, IdempotencyKey: "cancel-different-0002",
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("different cancellation key returned %v", err)
	}

	var state struct {
		OrderStatus       string `orm:"order_status"`
		InvoiceStatus     string `orm:"invoice_status"`
		InventoryStatus   string `orm:"inventory_status"`
		FulfillmentCount  int    `orm:"fulfillment_count"`
		MarketingStatus   string `orm:"marketing_status"`
		CancellationCount int    `orm:"cancellation_count"`
		OutboxCount       int    `orm:"outbox_count"`
		GlobalConsumed    int    `orm:"global_consumed"`
		CustomerConsumed  int    `orm:"customer_consumed"`
		AssetConsumed     int    `orm:"asset_consumed"`
		AssetStatus       string `orm:"asset_status"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &state, `
SELECT o.status_code AS order_status, i.status_code AS invoice_status,
       r.status_code AS inventory_status,
       (SELECT COUNT(*) FROM plugin_idc_core_product_fulfillment_intent f
          WHERE f.order_no = o.order_no)::INT AS fulfillment_count,
       m.status_code AS marketing_status,
       (SELECT COUNT(*) FROM plugin_idc_core_commerce_order_cancellation c
          WHERE c.order_no = o.order_no)::INT AS cancellation_count,
       (SELECT COUNT(*) FROM plugin_idc_core_outbox_event e
          WHERE e.aggregate_no = o.order_no)::INT AS outbox_count,
       campaign.global_consumed_count AS global_consumed,
       quota.consumed_count AS customer_consumed,
       asset.consumed_uses AS asset_consumed, asset.status_code AS asset_status
FROM plugin_idc_core_commerce_order o
JOIN plugin_idc_core_billing_invoice i ON i.order_no = o.order_no
JOIN plugin_idc_core_commerce_inventory_reservation r ON r.order_no = o.order_no
JOIN plugin_idc_core_marketing_campaign_reservation m ON m.quote_reference = o.quote_no
JOIN plugin_idc_core_marketing_campaign campaign ON campaign.campaign_no = m.campaign_no
JOIN plugin_idc_core_marketing_customer_quota quota
  ON quota.campaign_no = m.campaign_no AND quota.customer_no = o.customer_no
JOIN plugin_idc_core_marketing_promotion_asset asset
  ON asset.asset_no = 'asset-cancellation-fixture'
WHERE o.order_no = ?`, checkout.OrderNo); err != nil {
		t.Fatalf("load cancellation state: %v", err)
	}
	if state.OrderStatus != "cancelled" || state.InvoiceStatus != "cancelled" ||
		state.InventoryStatus != "released" || state.FulfillmentCount != 0 ||
		state.MarketingStatus != marketing.ReservationStatusReversed ||
		state.CancellationCount != 1 || state.OutboxCount != 2 ||
		state.GlobalConsumed != 0 || state.CustomerConsumed != 0 ||
		state.AssetConsumed != 0 || state.AssetStatus != "active" {
		t.Fatalf("atomic cancellation state=%+v", state)
	}
	plainKey, err := scenario.database.GetValue(scenario.ctx, `
SELECT COUNT(*) FROM plugin_idc_core_commerce_order_cancellation
WHERE request_digest = ?`, request.IdempotencyKey)
	if err != nil || plainKey.Int() != 0 {
		t.Fatalf("raw cancellation key count=%d err=%v", plainKey.Int(), err)
	}
}

func TestOrderCancellationReleasesGenericProductInventoryPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenarioWithSetup(
		t, "customer-commerce-service-1", 101, 2, false, 0,
		func(ctx context.Context, database gdb.DB) {
			if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory
SET tracking_enabled=TRUE,sellable_quantity=5,version=1
WHERE version_no='version-commerce-service-1'`); err != nil {
				t.Fatal(err)
			}
		},
	)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-generic-before-cancel-0001",
	})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := scenario.service.CancelAccountOrder(scenario.ctx, OrderCancellationRequest{
		OrderNo: checkout.OrderNo, IdempotencyKey: "cancel-generic-inventory-0001",
	})
	if err != nil || cancelled == nil || cancelled.Status != OrderCancellationStatusCompleted {
		t.Fatalf("cancel generic order=%+v err=%v", cancelled, err)
	}
	var evidence struct {
		OrderStatus, ReservationStatus string
		ReleasedQuantity               int
		FulfillmentIntents             int
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT
orders.status_code order_status,reservation.status_code reservation_status,
reservation.released_quantity,
(SELECT count(*) FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=orders.order_no) fulfillment_intents
FROM plugin_idc_core_commerce_order orders
JOIN plugin_idc_core_commerce_inventory_reservation reservation ON reservation.order_no=orders.order_no
WHERE orders.order_no=?`, checkout.OrderNo); err != nil {
		t.Fatal(err)
	}
	if evidence.OrderStatus != "cancelled" || evidence.ReservationStatus != "released" ||
		evidence.ReleasedQuantity != 2 || evidence.FulfillmentIntents != 0 {
		t.Fatalf("generic cancellation evidence=%+v", evidence)
	}
}

func seedConsumedMarketingQuotaAndAsset(
	t *testing.T,
	scenario checkoutTestScenario,
	orderNo string,
) {
	t.Helper()
	if _, err := scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_marketing_campaign
SET global_quota_limit = 10, per_customer_quota_limit = 2,
    global_consumed_count = 1
WHERE campaign_no = 'campaign-commerce-service'`); err != nil {
		t.Fatalf("seed cancellation campaign counters: %v", err)
	}
	if _, err := scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_marketing_customer_quota (
  campaign_no, customer_no, quota_limit, reserved_count, consumed_count
) VALUES ('campaign-commerce-service', 'customer-commerce-service-1', 2, 0, 1)`); err != nil {
		t.Fatalf("seed cancellation customer quota: %v", err)
	}
	if _, err := scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_marketing_promotion_asset (
  asset_no, campaign_no, benefit_no, benefit_type_code, asset_type_code,
  customer_no, benefit_snapshot_digest, currency_code, face_amount_minor,
  usage_limit, reserved_uses, consumed_uses, valid_from, valid_until,
  status_code, claimed_at
) SELECT 'asset-cancellation-fixture', r.campaign_no, r.benefit_no,
         'fixed_discount', 'coupon', r.customer_no, r.benefit_snapshot_digest,
         r.currency_code, r.quoted_value_minor, 1, 0, 1,
         CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP + INTERVAL '1 day',
         'exhausted', CURRENT_TIMESTAMP - INTERVAL '1 hour'
FROM plugin_idc_core_marketing_campaign_reservation r
JOIN plugin_idc_core_commerce_order o ON o.quote_no = r.quote_reference
WHERE o.order_no = ?`, orderNo); err != nil {
		t.Fatalf("seed consumed cancellation asset: %v", err)
	}
	if _, err := scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_marketing_asset_reservation (
  reservation_no, asset_no, customer_no, quote_reference, currency_code,
  reserved_uses, discount_amount_minor, status_code, expires_at, consumed_at
) SELECT r.reservation_no, 'asset-cancellation-fixture', r.customer_no,
         r.quote_reference, r.currency_code, 1, r.quoted_value_minor,
         'consumed', r.expires_at, CURRENT_TIMESTAMP
FROM plugin_idc_core_marketing_campaign_reservation r
JOIN plugin_idc_core_commerce_order o ON o.quote_no = r.quote_reference
WHERE o.order_no = ?`, orderNo); err != nil {
		t.Fatalf("seed consumed cancellation asset reservation: %v", err)
	}
}

func TestOrderCancellationOutboxFailureRollsBackPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, true, 0)
	enableOrderLifecycleMarketingReversal(t, scenario.service, scenario.database)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-cancel-rollback-0001",
	})
	if err != nil {
		t.Fatalf("checkout before rollback cancellation: %v", err)
	}
	scenario.service.outbox = failingCheckoutEventWriter{}
	_, err = scenario.service.CancelAccountOrder(scenario.ctx, OrderCancellationRequest{
		OrderNo: checkout.OrderNo, IdempotencyKey: "cancel-rollback-0001",
	})
	if !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("injected cancellation outbox error=%v", err)
	}
	var state struct {
		OrderStatus     string `orm:"order_status"`
		InvoiceStatus   string `orm:"invoice_status"`
		InventoryStatus string `orm:"inventory_status"`
		MarketingStatus string `orm:"marketing_status"`
		Cancellations   int    `orm:"cancellations"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &state, `
SELECT o.status_code AS order_status, i.status_code AS invoice_status,
       r.status_code AS inventory_status, m.status_code AS marketing_status,
       (SELECT COUNT(*) FROM plugin_idc_core_commerce_order_cancellation c
          WHERE c.order_no = o.order_no)::INT AS cancellations
FROM plugin_idc_core_commerce_order o
JOIN plugin_idc_core_billing_invoice i ON i.order_no = o.order_no
JOIN plugin_idc_core_commerce_inventory_reservation r ON r.order_no = o.order_no
JOIN plugin_idc_core_marketing_campaign_reservation m ON m.quote_reference = o.quote_no
WHERE o.order_no = ?`, checkout.OrderNo); err != nil {
		t.Fatalf("load rolled-back cancellation state: %v", err)
	}
	if state.OrderStatus != "pending_payment" || state.InvoiceStatus != "pending_payment" ||
		state.InventoryStatus != "held" || state.MarketingStatus != marketing.ReservationStatusConsumed ||
		state.Cancellations != 0 {
		t.Fatalf("cancellation escaped rollback: %+v", state)
	}
}

func TestPaymentProcessingCancellationRequiresClosurePostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-payment-close-0001",
	})
	if err != nil {
		t.Fatalf("checkout before payment closure: %v", err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_commerce_order SET status_code = 'payment_processing'
WHERE order_no = ?`, checkout.OrderNo); err != nil {
		t.Fatalf("move order to payment processing: %v", err)
	}
	pending, err := scenario.service.CancelAccountOrder(scenario.ctx, OrderCancellationRequest{
		OrderNo: checkout.OrderNo, IdempotencyKey: "cancel-payment-close-0001",
	})
	if err != nil || pending == nil || pending.Status != OrderCancellationStatusPendingPaymentClosure {
		t.Fatalf("payment closure request=%+v err=%v", pending, err)
	}
	status, err := scenario.database.GetValue(scenario.ctx, `
SELECT status_code FROM plugin_idc_core_commerce_inventory_reservation WHERE order_no = ?`, checkout.OrderNo)
	if err != nil || status.String() != "held" {
		t.Fatalf("payment-processing inventory status=%q err=%v", status.String(), err)
	}
	lifecycle := &orderLifecycleServiceImpl{
		database: scenario.database, reservationReverser: noQuoteReservationReverser{},
		outbox: scenario.service.outbox,
	}
	confirmation := PaymentClosureConfirmation{
		CancellationNo: pending.CancellationNo, Outcome: PaymentClosureOutcomeNoAttempts,
		Reference: "payment-owner-close-0001", EvidenceDigest: strings.Repeat("b", 64),
	}
	completed, err := lifecycle.FinalizePaymentClosure(scenario.ctx, confirmation)
	if err != nil || completed == nil || completed.Status != OrderCancellationStatusCompleted {
		t.Fatalf("finalize payment closure=%+v err=%v", completed, err)
	}
	replayed, err := lifecycle.FinalizePaymentClosure(scenario.ctx, confirmation)
	if err != nil || replayed == nil || !replayed.Reused {
		t.Fatalf("replay payment closure=%+v err=%v", replayed, err)
	}
	changed := confirmation
	changed.Outcome = PaymentClosureOutcomeClosed
	if _, err = lifecycle.FinalizePaymentClosure(scenario.ctx, changed); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("changed payment closure outcome returned %v", err)
	}
}

func TestExpireDueOrdersIsBoundedPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-expiry-0001",
	})
	if err != nil {
		t.Fatalf("checkout before expiry: %v", err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_commerce_order
SET created_at = CURRENT_TIMESTAMP - INTERVAL '2 minutes',
    payment_due_at = CURRENT_TIMESTAMP - INTERVAL '1 minute'
WHERE order_no = ?`, checkout.OrderNo); err != nil {
		t.Fatalf("expire order fixture: %v", err)
	}
	lifecycle := &orderLifecycleServiceImpl{
		database: scenario.database, reservationReverser: noQuoteReservationReverser{},
		outbox: scenario.service.outbox,
	}
	batch, err := lifecycle.ExpireDueOrders(scenario.ctx, 1)
	if err != nil || batch.Scanned != 1 || batch.Cancelled != 1 || batch.PendingPaymentClosure != 0 {
		t.Fatalf("expiry batch=%+v err=%v", batch, err)
	}
	empty, err := lifecycle.ExpireDueOrders(scenario.ctx, 1)
	if err != nil || empty.Scanned != 0 {
		t.Fatalf("empty expiry replay=%+v err=%v", empty, err)
	}
}

type riskReviewAuthorizerStub struct {
	actor        int
	approveCalls int
	rejectCalls  int
}

func (s *riskReviewAuthorizerStub) AuthorizeRiskReviewApprove(context.Context, string) (int, error) {
	s.approveCalls++
	return s.actor, nil
}

func (s *riskReviewAuthorizerStub) AuthorizeRiskReviewReject(context.Context, string) (int, error) {
	s.rejectCalls++
	return s.actor, nil
}

type riskReviewAuditStub struct {
	transitions []idccap.BusinessAuditTransition
	err         error
}

func (s *riskReviewAuditStub) AppendBusinessTransition(
	_ context.Context,
	input idccap.BusinessAuditTransition,
) error {
	if s.err != nil {
		return s.err
	}
	s.transitions = append(s.transitions, input)
	return nil
}

func (*riskReviewAuditStub) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, errors.New("not used")
}

func TestOrderRiskReviewApprovalKeepsIndependentGatesPostgreSQL(t *testing.T) {
	for _, testCase := range []struct {
		name              string
		fulfillmentReview bool
		wantOrderStatus   string
	}{
		{name: "risk is the only gate", wantOrderStatus: "pending_payment"},
		{name: "manual fulfillment remains", fulfillmentReview: true, wantOrderStatus: "manual_review"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
			setCheckoutQuoteManualRisk(t, scenario, testCase.fulfillmentReview)
			checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
				QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-risk-approve-0001",
			})
			if err != nil || checkout.StatusCode != "manual_review" {
				t.Fatalf("manual-risk checkout=%+v err=%v", checkout, err)
			}
			reviewNo := loadOnlyRiskReviewNo(t, scenario, checkout.OrderNo)
			authorizer := &riskReviewAuthorizerStub{actor: 9001}
			audit := &riskReviewAuditStub{}
			service, err := newOrderRiskReviewServiceWithDependencies(
				scenario.database, audit, authorizer, nil, scenario.service.outbox,
			)
			if err != nil {
				t.Fatalf("create risk review service: %v", err)
			}
			request := OrderRiskReviewDecisionInput{
				ReviewNo: reviewNo, RequestNo: "risk-approve-request-0001",
				DecisionCode:   OrderRiskReviewDecisionApproved,
				EvidenceDigest: strings.Repeat("c", 64),
			}
			decision, err := service.DecideRiskReview(scenario.ctx, request)
			if err != nil || decision == nil || decision.StatusCode != "approved" ||
				decision.OrderStatus != testCase.wantOrderStatus || decision.Reused {
				t.Fatalf("risk approval=%+v err=%v", decision, err)
			}
			replay, err := service.DecideRiskReview(scenario.ctx, request)
			if err != nil || replay == nil || !replay.Reused {
				t.Fatalf("risk approval replay=%+v err=%v", replay, err)
			}
			if authorizer.approveCalls != 2 || authorizer.rejectCalls != 0 || len(audit.transitions) != 2 ||
				audit.transitions[0].RequiredPermission != permissionOrderRiskReviewApprove {
				t.Fatalf("risk approval authority=%+v audit=%+v", authorizer, audit.transitions)
			}
			var stored struct {
				OrderStatus  string `orm:"order_status"`
				RiskStatus   string `orm:"risk_status"`
				ReviewStatus string `orm:"review_status"`
			}
			if err = scenario.database.GetScan(scenario.ctx, &stored, `
SELECT o.status_code AS order_status, o.risk_decision_code AS risk_status,
       r.status_code AS review_status
FROM plugin_idc_core_commerce_order o
JOIN plugin_idc_core_commerce_order_review r ON r.order_no = o.order_no
WHERE o.order_no = ?`, checkout.OrderNo); err != nil ||
				stored.OrderStatus != testCase.wantOrderStatus || stored.RiskStatus != "approved" ||
				stored.ReviewStatus != "approved" {
				t.Fatalf("stored risk approval=%+v err=%v", stored, err)
			}
		})
	}
}

func TestOrderRiskReviewRejectionReusesCancellationTransactionPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, true, 0)
	enableOrderLifecycleMarketingReversal(t, scenario.service, scenario.database)
	setCheckoutQuoteManualRisk(t, scenario, false)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-risk-reject-0001",
	})
	if err != nil {
		t.Fatalf("checkout before risk rejection: %v", err)
	}
	reviewNo := loadOnlyRiskReviewNo(t, scenario, checkout.OrderNo)
	authorizer := &riskReviewAuthorizerStub{actor: 9002}
	audit := &riskReviewAuditStub{}
	service, err := newOrderRiskReviewServiceWithDependencies(
		scenario.database, audit, authorizer, scenario.service.reservationReverser,
		scenario.service.outbox,
	)
	if err != nil {
		t.Fatalf("create rejecting risk review service: %v", err)
	}
	decision, err := service.DecideRiskReview(scenario.ctx, OrderRiskReviewDecisionInput{
		ReviewNo: reviewNo, RequestNo: "risk-reject-request-0001",
		DecisionCode:   OrderRiskReviewDecisionRejected,
		EvidenceDigest: strings.Repeat("d", 64),
	})
	if err != nil || decision == nil || decision.StatusCode != "rejected" ||
		decision.OrderStatus != "cancelled" {
		t.Fatalf("risk rejection=%+v err=%v", decision, err)
	}
	var stored struct {
		OrderStatus        string `orm:"order_status"`
		ReviewStatus       string `orm:"review_status"`
		CancellationReason string `orm:"cancellation_reason"`
		MarketingStatus    string `orm:"marketing_status"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &stored, `
SELECT o.status_code AS order_status, r.status_code AS review_status,
       c.reason_code AS cancellation_reason, m.status_code AS marketing_status
FROM plugin_idc_core_commerce_order o
JOIN plugin_idc_core_commerce_order_review r ON r.order_no = o.order_no
JOIN plugin_idc_core_commerce_order_cancellation c ON c.order_no = o.order_no
JOIN plugin_idc_core_marketing_campaign_reservation m ON m.quote_reference = o.quote_no
WHERE o.order_no = ?`, checkout.OrderNo); err != nil ||
		stored.OrderStatus != "cancelled" || stored.ReviewStatus != "rejected" ||
		stored.CancellationReason != string(OrderCancellationReasonRiskRejected) ||
		stored.MarketingStatus != marketing.ReservationStatusReversed {
		t.Fatalf("stored risk rejection=%+v err=%v", stored, err)
	}
	if authorizer.rejectCalls != 1 || authorizer.approveCalls != 0 || len(audit.transitions) != 1 ||
		audit.transitions[0].RequiredPermission != permissionOrderRiskReviewReject {
		t.Fatalf("risk rejection authority=%+v audit=%+v", authorizer, audit.transitions)
	}
}

func TestOrderRiskReviewAuditFailureRollsBackPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	setCheckoutQuoteManualRisk(t, scenario, false)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-risk-audit-0001",
	})
	if err != nil {
		t.Fatalf("checkout before risk audit failure: %v", err)
	}
	reviewNo := loadOnlyRiskReviewNo(t, scenario, checkout.OrderNo)
	service, err := newOrderRiskReviewServiceWithDependencies(
		scenario.database,
		&riskReviewAuditStub{err: errors.New("injected risk audit failure")},
		&riskReviewAuthorizerStub{actor: 9003}, nil, scenario.service.outbox,
	)
	if err != nil {
		t.Fatalf("create failing-audit risk review service: %v", err)
	}
	_, err = service.DecideRiskReview(scenario.ctx, OrderRiskReviewDecisionInput{
		ReviewNo: reviewNo, RequestNo: "risk-audit-failure-0001",
		DecisionCode:   OrderRiskReviewDecisionApproved,
		EvidenceDigest: strings.Repeat("e", 64),
	})
	if !bizerr.Is(err, CodeOrderRiskReviewUnavailable) {
		t.Fatalf("risk audit failure returned %v", err)
	}
	var stored struct {
		OrderStatus  string `orm:"order_status"`
		RiskStatus   string `orm:"risk_status"`
		ReviewStatus string `orm:"review_status"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &stored, `
SELECT o.status_code AS order_status, o.risk_decision_code AS risk_status,
       r.status_code AS review_status
FROM plugin_idc_core_commerce_order o
JOIN plugin_idc_core_commerce_order_review r ON r.order_no = o.order_no
WHERE o.order_no = ?`, checkout.OrderNo); err != nil ||
		stored.OrderStatus != "manual_review" || stored.RiskStatus != "manual_review" ||
		stored.ReviewStatus != "pending" {
		t.Fatalf("risk audit rollback state=%+v err=%v", stored, err)
	}
}

func setCheckoutQuoteManualRisk(
	t *testing.T,
	scenario checkoutTestScenario,
	manualFulfillment bool,
) {
	t.Helper()
	fulfillment := "automatic"
	if manualFulfillment {
		fulfillment = "manual_review"
	}
	if _, err := scenario.database.Exec(scenario.ctx, `
DROP TRIGGER trg_idc_com_quote_item_immutable
ON plugin_idc_core_commerce_quote_item_snapshot`); err != nil {
		t.Fatalf("remove immutable quote trigger for fixture: %v", err)
	}
	if _, err := scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_commerce_quote_item_snapshot
SET risk_decision_code = 'manual_review', fulfillment_mode_code = ?
WHERE quote_no = ?`, fulfillment, scenario.quote.QuoteNo); err != nil {
		t.Fatalf("prepare manual-risk quote fixture: %v", err)
	}
	if _, err := scenario.database.Exec(scenario.ctx, `
CREATE TRIGGER trg_idc_com_quote_item_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_quote_item_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation()`); err != nil {
		t.Fatalf("restore immutable quote trigger after fixture: %v", err)
	}
}

func loadOnlyRiskReviewNo(t *testing.T, scenario checkoutTestScenario, orderNo string) string {
	t.Helper()
	value, err := scenario.database.GetValue(scenario.ctx, `
SELECT review_no FROM plugin_idc_core_commerce_order_review
WHERE order_no = ? AND review_type_code = 'risk'`, orderNo)
	if err != nil || value.String() == "" {
		t.Fatalf("load risk review number=%q err=%v", value.String(), err)
	}
	return value.String()
}

func enableOrderLifecycleMarketingReversal(t *testing.T, service *serviceImpl, database gdb.DB) {
	t.Helper()
	marketingService, err := marketing.New(database, marketing.Config{})
	if err != nil {
		t.Fatalf("create cancellation marketing service: %v", err)
	}
	evaluator, err := NewMarketingQuoteAdjustmentEvaluator(marketingService)
	if err != nil {
		t.Fatalf("create cancellation marketing evaluator: %v", err)
	}
	reverser, ok := evaluator.(QuoteReservationReverser)
	if !ok {
		t.Fatal("cancellation marketing evaluator does not support reversal")
	}
	service.reservationReverser = reverser
	service.hasReservationReverser = true
}
