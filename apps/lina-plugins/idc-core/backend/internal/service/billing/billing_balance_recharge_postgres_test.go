package billing

import (
	"context"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type paymentClosureOwnerTestStub struct{}

func (paymentClosureOwnerTestStub) ClaimPaymentClosure(context.Context) (*billingcap.PaymentClosureWork, error) {
	return &billingcap.PaymentClosureWork{}, nil
}
func (paymentClosureOwnerTestStub) ReportPaymentClosure(_ context.Context, input billingcap.ReportPaymentClosure) (*billingcap.PaymentClosureReport, error) {
	return &billingcap.PaymentClosureReport{CancellationNo: input.CancellationNo, StatusCode: "completed", OutcomeCode: input.OutcomeCode}, nil
}

func newChannelPaymentOwnerTestService(
	t *testing.T,
	database gdb.DB,
	settlements FinancialSettlementService,
	readers ...principal.Reader,
) (billingcap.Service, error) {
	t.Helper()
	events, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return NewChannelPaymentOwnerService(database, settlements, events, tasks, paymentClosureOwnerTestStub{}, readers...)
}

func TestChannelPaymentOwnerPreparesAndConfirmsCurrentCustomerBalanceRecharge(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	reader := financialPrincipalStub{audience: &principal.Audience{UserID: 77, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "customer-ledger"}}
	owner, err := newChannelPaymentOwnerTestService(t, database, financialSettlementServiceStub{}, reader)
	if err != nil {
		t.Fatal(err)
	}
	input := billingcap.PrepareBalanceRecharge{PaymentAttemptNo: "PAT_recharge_0001", CurrencyCode: "CNY", AmountMinor: 12500}
	prepared, err := owner.PrepareBalanceRecharge(ctx, input)
	if err != nil || prepared.AmountMinor != input.AmountMinor || prepared.CurrencyCode != "CNY" || prepared.CustomerRefDigest == "customer-ledger" {
		t.Fatalf("prepared=%+v err=%v", prepared, err)
	}
	replayed, err := owner.PrepareBalanceRecharge(ctx, input)
	if err != nil || replayed.RechargeContextNo != prepared.RechargeContextNo {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	changed := input
	changed.AmountMinor++
	if result, changedErr := owner.PrepareBalanceRecharge(ctx, changed); result != nil || changedErr == nil {
		t.Fatalf("changed=%+v err=%v", result, changedErr)
	}
	confirmation := billingcap.ConfirmedBalanceRecharge{PaymentAttemptNo: input.PaymentAttemptNo, RechargeContextNo: prepared.RechargeContextNo, CurrencyCode: "CNY", AmountMinor: input.AmountMinor, EvidenceDigest: strings.Repeat("a", 64)}
	confirmed, err := owner.ConfirmBalanceRecharge(ctx, confirmation)
	if err != nil || confirmed.BalanceMinor != input.AmountMinor || confirmed.AvailableMinor != input.AmountMinor || confirmed.Reused {
		t.Fatalf("confirmed=%+v err=%v", confirmed, err)
	}
	confirmedAgain, err := owner.ConfirmBalanceRecharge(ctx, confirmation)
	if err != nil || !confirmedAgain.Reused || confirmedAgain.TransactionNo != confirmed.TransactionNo {
		t.Fatalf("confirmed replay=%+v err=%v", confirmedAgain, err)
	}
	reader.audience.Status = principal.AudienceStatusRevoked
	if result, revokedErr := owner.PrepareBalanceRecharge(ctx, billingcap.PrepareBalanceRecharge{PaymentAttemptNo: "PAT_recharge_0002", CurrencyCode: "CNY", AmountMinor: 100}); result != nil || revokedErr == nil {
		t.Fatalf("revoked result=%+v err=%v", result, revokedErr)
	}
}
