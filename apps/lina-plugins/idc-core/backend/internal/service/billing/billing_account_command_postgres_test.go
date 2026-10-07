package billing

import (
	"context"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestAccountFinanceCommandPostgreSQLPaysOwnedInvoiceAndReplays(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceTransform(t, ctx, database)
	balance, err := NewBalanceService(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 10_000,
		SourceReference:      "account-command-recharge",
		SourceSnapshotDigest: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatalf("fund customer balance: %v", err)
	}
	service, err := NewAccountFinanceCommandService(database, financialPrincipalStub{audience: &principal.Audience{
		UserID: 77, Code: principal.AudienceCustomer,
		Status: principal.AudienceStatusActive, CustomerNo: "customer-ledger",
	}}, financialSettlementServiceStub{})
	if err != nil {
		t.Fatal(err)
	}
	input := AccountBalancePaymentInput{
		InvoiceNo: "merge-invoice-a", IdempotencyKey: "account-balance-payment-0001",
	}
	paid, err := service.PayAccountInvoiceFromBalance(ctx, input)
	if err != nil || paid == nil || paid.Reused || paid.StatusCode != "paid" ||
		paid.PaidAmountMinor != 900 || paid.OutstandingAmountMinor != 0 ||
		paid.BalanceMinor != 9_100 || paid.AvailableBalanceMinor != 9_100 {
		t.Fatalf("pay account invoice: result=%+v err=%v", paid, err)
	}
	replay, err := service.PayAccountInvoiceFromBalance(ctx, input)
	if err != nil || replay == nil || !replay.Reused || replay.TransactionNo != paid.TransactionNo {
		t.Fatalf("replay account invoice payment: result=%+v err=%v", replay, err)
	}
	if _, err = service.PayAccountInvoiceFromBalance(ctx, AccountBalancePaymentInput{
		InvoiceNo: "other-customer-invoice", IdempotencyKey: "foreign-balance-payment-001",
	}); !bizerr.Is(err, CodeDirectoryNotAccessible) {
		t.Fatalf("foreign invoice existence was exposed: %v", err)
	}
}

func TestAccountFinanceCommandPostgreSQLMergesOnlyOwnedInvoices(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceTransform(t, ctx, database)
	service, err := NewAccountFinanceCommandService(database, financialPrincipalStub{audience: &principal.Audience{
		UserID: 78, Code: principal.AudienceCustomer,
		Status: principal.AudienceStatusActive, CustomerNo: "customer-ledger",
	}}, financialSettlementServiceStub{})
	if err != nil {
		t.Fatal(err)
	}
	input := AccountInvoiceMergeInput{
		InvoiceNos:     []string{"merge-invoice-b", "merge-invoice-a"},
		IdempotencyKey: "account-invoice-merge-0001",
	}
	merged, err := service.MergeAccountInvoices(ctx, input)
	if err != nil || merged == nil || merged.Reused || merged.CurrencyCode != "CNY" ||
		len(merged.SourceInvoiceNos) != 2 || len(merged.DestinationInvoices) != 1 ||
		merged.DestinationInvoices[0].TotalAmountMinor != 1_450 ||
		merged.DestinationInvoices[0].OutstandingAmountMinor != 1_450 {
		t.Fatalf("merge owned invoices: result=%+v err=%v", merged, err)
	}
	replay, err := service.MergeAccountInvoices(ctx, input)
	if err != nil || replay == nil || !replay.Reused || replay.TransformNo != merged.TransformNo {
		t.Fatalf("replay owned invoice merge: result=%+v err=%v", replay, err)
	}
	if _, err = service.MergeAccountInvoices(ctx, AccountInvoiceMergeInput{
		InvoiceNos:     []string{"split-invoice", "other-customer-invoice"},
		IdempotencyKey: "account-foreign-merge-0001",
	}); !bizerr.Is(err, CodeDirectoryNotAccessible) {
		t.Fatalf("foreign merge source existence was exposed: %v", err)
	}
}

func TestAccountFinanceCommandRejectsNonCustomerPrincipal(t *testing.T) {
	service := &accountFinanceCommandImpl{principal: financialPrincipalStub{audience: &principal.Audience{
		UserID: 1, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}}}
	if _, err := service.PayAccountInvoiceFromBalance(context.Background(), AccountBalancePaymentInput{}); !bizerr.Is(err, CodeDirectoryNotAccessible) {
		t.Fatalf("staff principal entered account financial command: %v", err)
	}
}
