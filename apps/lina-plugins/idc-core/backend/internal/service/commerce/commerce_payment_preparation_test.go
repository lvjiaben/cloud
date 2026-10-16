package commerce

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type nilPaymentPreparationPrincipal struct{}

func (*nilPaymentPreparationPrincipal) Current(context.Context) (*principal.Audience, error) {
	return nil, nil
}

func TestPaymentPreparationRejectsTypedNilDependency(t *testing.T) {
	var reader *nilPaymentPreparationPrincipal
	if !isNilPaymentPreparationDependency(principal.Reader(reader)) {
		t.Fatal("typed nil principal reader must be rejected")
	}
}

func TestDecidePaymentPreparationTerminalStates(t *testing.T) {
	now := time.Now().UTC()
	verified := paymentIdentityProjection{
		StatusCode: "verified", CaseNo: "case-1", Version: 1, VerifiedAt: &now,
	}

	expiredAt := now.Add(-time.Second)
	order := &entity.CommerceOrder{
		StatusCode: "pending_payment", PaymentDueAt: &expiredAt, TotalAmountMinor: 100,
	}
	invoice := &entity.BillingInvoice{
		StatusCode: "pending_payment", PaymentDueAt: &expiredAt, TotalAmountMinor: 100,
	}
	decision := decidePaymentPreparation(order, invoice, verified, 0, now)
	if decision.status != PaymentPreparationExpired ||
		decision.reason != PaymentPreparationPaymentExpired {
		t.Fatalf("expired decision=%+v", decision)
	}

	futureAt := now.Add(time.Minute)
	order.StatusCode = "payment_processing"
	order.PaymentDueAt = &futureAt
	invoice.PaymentDueAt = &futureAt
	decision = decidePaymentPreparation(order, invoice, verified, 0, now)
	if decision.status != PaymentPreparationClosed ||
		decision.reason != PaymentPreparationOrderNotPayable {
		t.Fatalf("closed decision=%+v", decision)
	}
}

// Explicit staff approval satisfies the identity gate without fabricated case evidence.
func TestDecidePaymentPreparationManualVerification(t *testing.T) {
	now := time.Now().UTC()
	future := now.Add(time.Hour)
	order := &entity.CommerceOrder{StatusCode: "pending_payment", PaymentDueAt: &future, TotalAmountMinor: 100}
	invoice := &entity.BillingInvoice{StatusCode: "pending_payment", TotalAmountMinor: 100}
	identity := paymentIdentityProjection{StatusCode: "verified", Version: 2, VerifiedAt: &now, ManualVerification: true}
	decision := decidePaymentPreparation(order, invoice, identity, 0, now)
	if decision.status != PaymentPreparationReady {
		t.Fatalf("manual approval blocked: %+v", decision)
	}
	identity.ManualVerification = false
	if decision = decidePaymentPreparation(order, invoice, identity, 0, now); decision.status != PaymentPreparationAwaitingIdentity {
		t.Fatalf("missing supplier evidence accepted: %+v", decision)
	}
	identity.ManualVerification = true
	identity.StatusCode = "unverified"
	if decision = decidePaymentPreparation(order, invoice, identity, 0, now); decision.status != PaymentPreparationAwaitingIdentity {
		t.Fatalf("cleared identity accepted: %+v", decision)
	}
}
