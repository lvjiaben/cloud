package commerce

import (
	"sync"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestPaymentPreparationConcurrentIdentityRecoveryPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-before-identity-0001",
	})
	if err != nil {
		t.Fatalf("checkout before identity preparation: %v", err)
	}
	service, err := NewPaymentPreparationService(scenario.database, scenario.service.principal)
	if err != nil {
		t.Fatalf("create payment preparation service: %v", err)
	}

	const workers = 8
	start := make(chan struct{})
	results := make([]*PaymentPreparation, workers)
	errorsFound := make([]error, workers)
	var wait sync.WaitGroup
	for index := range workers {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errorsFound[index] = service.PrepareAccountOrderPayment(
				scenario.ctx, checkout.OrderNo,
			)
		}(index)
	}
	close(start)
	wait.Wait()
	contextNo := ""
	for index := range workers {
		if errorsFound[index] != nil || results[index] == nil ||
			results[index].StatusCode != PaymentPreparationAwaitingIdentity ||
			results[index].ReasonCode != PaymentPreparationIdentityUnverified || results[index].Ready {
			t.Fatalf("concurrent payment preparation %d result=%+v err=%v", index, results[index], errorsFound[index])
		}
		if contextNo == "" {
			contextNo = results[index].ContextNo
		} else if results[index].ContextNo != contextNo {
			t.Fatalf("payment context diverged: %q != %q", results[index].ContextNo, contextNo)
		}
	}
	count, err := scenario.database.GetValue(scenario.ctx, `
SELECT COUNT(*) FROM plugin_idc_core_commerce_payment_context
WHERE order_no = ?`, checkout.OrderNo)
	if err != nil || count.Int64() != 1 {
		t.Fatalf("payment context count=%d err=%v", count.Int64(), err)
	}

	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	ready, err := service.ResumeAccountOrderPayment(scenario.ctx, contextNo)
	if err != nil || ready == nil || !ready.Ready || ready.StatusCode != PaymentPreparationReady ||
		ready.ReasonCode != PaymentPreparationPaymentReady || ready.OrderNo != checkout.OrderNo ||
		ready.InvoiceNo != checkout.InvoiceNo || ready.CurrencyCode != "CNY" ||
		ready.AmountDueMinor != checkout.TotalAmountMinor || ready.RequiresContract {
		t.Fatalf("recovered payment preparation=%+v err=%v", ready, err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_customer_individual_profile
SET identity_status_code = 'revoked', version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE customer_no = 'customer-commerce-service-1'`); err != nil {
		t.Fatalf("revoke individual payment identity: %v", err)
	}
	revoked, err := service.ResumeAccountOrderPayment(scenario.ctx, contextNo)
	if err != nil || revoked == nil || revoked.Ready ||
		revoked.StatusCode != PaymentPreparationAwaitingIdentity ||
		revoked.ReasonCode != PaymentPreparationIdentityRevoked {
		t.Fatalf("revoked payment preparation=%+v err=%v", revoked, err)
	}
	readyAt, err := scenario.database.GetValue(scenario.ctx, `
SELECT ready_at IS NULL FROM plugin_idc_core_commerce_payment_context
WHERE context_no = ?`, contextNo)
	if err != nil || !readyAt.Bool() {
		t.Fatalf("revoked payment context ready_at null=%v err=%v", readyAt.Bool(), err)
	}

	otherCustomerService, err := NewPaymentPreparationService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 202, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-2",
		}},
	)
	if err != nil {
		t.Fatalf("create other-customer payment service: %v", err)
	}
	if _, err = otherCustomerService.ResumeAccountOrderPayment(scenario.ctx, contextNo); !bizerr.Is(err, CodePaymentPreparationNotAccessible) {
		t.Fatalf("cross-customer payment context returned %v", err)
	}
}

func TestPaymentPreparationContractAndOrderReviewAreIndependentPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	if _, err := scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_catalog_product_version
	SET requires_contract = TRUE,
	    contract_policy_code = 'required',
	    contract_template_no = ?
	WHERE version_no = 'version-commerce-service-1'`, checkoutContractTemplateNo); err != nil {
		t.Fatalf("enable contract requirement fixture: %v", err)
	}
	cart, err := scenario.service.GetAccountCart(scenario.ctx)
	if err != nil {
		t.Fatalf("reload account cart: %v", err)
	}
	requote, err := scenario.service.QuoteAccountCart(
		scenario.ctx, QuoteRequest{ExpectedCartVersion: cart.Version},
	)
	if err != nil {
		t.Fatalf("requote contract order: %v", err)
	}
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: requote.QuoteNo, IdempotencyKey: "checkout-contract-identity-0001",
	})
	if err != nil || !checkout.RequiresContract {
		t.Fatalf("checkout contract order=%+v err=%v", checkout, err)
	}
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	service, err := NewPaymentPreparationService(scenario.database, scenario.service.principal)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := service.PrepareAccountOrderPayment(scenario.ctx, checkout.OrderNo)
	if err != nil || ready == nil || !ready.Ready || !ready.RequiresContract {
		t.Fatalf("contract must not block payment preparation=%+v err=%v", ready, err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_commerce_order
SET status_code = 'manual_review', risk_decision_code = 'manual_review',
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE order_no = ?`, checkout.OrderNo); err != nil {
		t.Fatalf("move order to independent review fixture: %v", err)
	}
	blocked, err := service.ResumeAccountOrderPayment(scenario.ctx, ready.ContextNo)
	if err != nil || blocked == nil || blocked.Ready ||
		blocked.StatusCode != PaymentPreparationAwaitingOrderReview ||
		blocked.ReasonCode != PaymentPreparationOrderReview {
		t.Fatalf("review-blocked payment preparation=%+v err=%v", blocked, err)
	}
}

func TestPaymentPreparationEnterpriseAndCancellationPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	if _, err := scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_customer_subject
SET customer_type_code = 'enterprise', version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE customer_no = 'customer-commerce-service-2'`); err != nil {
		t.Fatalf("prepare enterprise subject: %v", err)
	}
	enterpriseScenario := newCheckoutCustomerOnDatabase(
		t, scenario, "customer-commerce-service-2", 202, 1,
	)
	checkout, err := enterpriseScenario.service.CheckoutAccountQuote(
		enterpriseScenario.ctx,
		CheckoutRequest{QuoteNo: enterpriseScenario.quote.QuoteNo, IdempotencyKey: "checkout-enterprise-identity-0001"},
	)
	if err != nil {
		t.Fatalf("checkout enterprise order: %v", err)
	}
	insertVerifiedEnterprisePaymentIdentity(t, enterpriseScenario, "customer-commerce-service-2")
	service, err := NewPaymentPreparationService(
		enterpriseScenario.database, enterpriseScenario.service.principal,
	)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := service.PrepareAccountOrderPayment(enterpriseScenario.ctx, checkout.OrderNo)
	if err != nil || ready == nil || !ready.Ready || ready.CustomerTypeCode != "enterprise" {
		t.Fatalf("enterprise payment preparation=%+v err=%v", ready, err)
	}
	if _, err = enterpriseScenario.service.CancelAccountOrder(
		enterpriseScenario.ctx,
		OrderCancellationRequest{OrderNo: checkout.OrderNo, IdempotencyKey: "cancel-enterprise-before-pay-0001"},
	); err != nil {
		t.Fatalf("cancel enterprise order: %v", err)
	}
	cancelled, err := service.ResumeAccountOrderPayment(enterpriseScenario.ctx, ready.ContextNo)
	if err != nil || cancelled == nil || cancelled.StatusCode != PaymentPreparationCancelled ||
		cancelled.ReasonCode != PaymentPreparationOrderCancelled || cancelled.Ready {
		t.Fatalf("cancelled payment preparation=%+v err=%v", cancelled, err)
	}
}

func insertVerifiedIndividualPaymentIdentity(
	t *testing.T,
	scenario checkoutTestScenario,
	customerNo string,
) {
	t.Helper()
	if _, err := scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_customer_individual_profile (
    customer_no, legal_name, legal_name_masked, identity_identifier_masked,
    identity_lookup_digest, identity_case_no, identity_status_code,
    identity_verified_at, identity_expires_at
) VALUES (?, 'Test Customer', 'T*** C*******', '110***********001X',
          repeat('a', 64), 'identity-case-individual-001', 'verified',
          CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '1 year')`, customerNo); err != nil {
		t.Fatalf("insert verified individual payment identity: %v", err)
	}
}

func insertVerifiedEnterprisePaymentIdentity(
	t *testing.T,
	scenario checkoutTestScenario,
	customerNo string,
) {
	t.Helper()
	if _, err := scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_customer_enterprise_profile (
    customer_no, legal_name, registration_identifier,
    registration_identifier_masked, registration_lookup_digest,
    verification_case_no, verification_status_code, verified_at, expires_at
) VALUES (?, 'Test Enterprise Ltd', '91350000TEST000001', '9135********0001',
          repeat('b', 64), 'identity-case-enterprise-001', 'verified',
          CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '1 year')`, customerNo); err != nil {
		t.Fatalf("insert verified enterprise payment identity: %v", err)
	}
}
