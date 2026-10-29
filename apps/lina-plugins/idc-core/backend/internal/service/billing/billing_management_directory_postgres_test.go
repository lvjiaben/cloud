package billing

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
)

type managementFinanceAuthorizerStub struct{}

func (managementFinanceAuthorizerStub) AuthorizeBalanceAdjustmentCreate(context.Context) (int, error) {
	return 9001, nil
}
func (managementFinanceAuthorizerStub) AuthorizeFinancialOverview(context.Context) (int, error) {
	return 9001, nil
}
func (managementFinanceAuthorizerStub) AuthorizeInvoiceList(context.Context) (int, error) {
	return 9001, nil
}
func (managementFinanceAuthorizerStub) AuthorizeInvoiceDetail(context.Context, string) (int, error) {
	return 9001, nil
}
func (managementFinanceAuthorizerStub) AuthorizeTransactionList(context.Context) (int, error) {
	return 9001, nil
}
func (managementFinanceAuthorizerStub) AuthorizeTransactionDetail(context.Context, string) (int, error) {
	return 9001, nil
}
func (managementFinanceAuthorizerStub) AuthorizeAdjustmentList(context.Context) (int, error) {
	return 9001, nil
}
func (managementFinanceAuthorizerStub) AuthorizeAdjustmentDetail(context.Context, string) (int, error) {
	return 9001, nil
}
func (managementFinanceAuthorizerStub) AuthorizeManualReceiptCreate(context.Context) (int, error) {
	return 9001, nil
}
func (managementFinanceAuthorizerStub) AuthorizeInvoiceWaiverCreate(context.Context) (int, error) {
	return 9001, nil
}
func (managementFinanceAuthorizerStub) AuthorizeAdjustmentReview(context.Context, string) (int, error) {
	return 9001, nil
}
func (managementFinanceAuthorizerStub) AuthorizeAdjustmentPolicyWrite(context.Context) (int, error) {
	return 9001, nil
}

func TestManagementFinanceDirectoryPostgreSQLReadsBoundedFacts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceTransform(t, ctx, database)
	balance, err := NewBalanceService(database)
	if err != nil {
		t.Fatal(err)
	}
	posting, err := balance.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 2_000,
		SourceReference:      "management-directory-recharge",
		SourceSnapshotDigest: strings.Repeat("7", 64),
	})
	if err != nil {
		t.Fatalf("seed management transaction: %v", err)
	}
	adjustments, err := NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := seedLegacyReceipt(t, ctx, adjustments, ManualReceiptInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 800,
		ReasonCode: "offline_balance_credit", ProofReference: "private-proof-management-001",
		ReceivingAccountReference: "merchant-collection-cny-001",
		BusinessPurposeCode:       "balance_recharge", EvidenceDigest: strings.Repeat("8", 64),
		IdempotencyKey: "management-directory-adjustment-0001", RequestedBy: 9001,
	})
	if err != nil || pending.Status != AdjustmentStatusPendingReview {
		t.Fatalf("seed pending adjustment: result=%+v err=%v", pending, err)
	}
	reader, err := NewManagementFinanceReader(
		database, managementFinanceAuthorizerStub{}, []byte(strings.Repeat("m", 32)),
	)
	if err != nil {
		t.Fatal(err)
	}
	overview, err := reader.GetManagementFinancialOverview(ctx)
	if err != nil || overview.OpenInvoiceCount < 4 || overview.OverdueInvoiceCount < 1 ||
		overview.PendingAdjustmentCount != 1 || len(overview.Receivables) != 2 {
		t.Fatalf("management overview=%+v err=%v", overview, err)
	}
	invoicePage, err := reader.ListManagementInvoices(ctx, ManagementInvoiceDirectoryFilter{
		CustomerNo: "customer-ledger", Limit: 2,
	})
	if err != nil || len(invoicePage.Invoices) != 2 || invoicePage.NextCursor == "" ||
		invoicePage.Invoices[0].CustomerNo != "customer-ledger" {
		t.Fatalf("management invoice page=%+v err=%v", invoicePage, err)
	}
	invoiceNext, err := reader.ListManagementInvoices(ctx, ManagementInvoiceDirectoryFilter{
		CustomerNo: "customer-ledger", Limit: 2, Cursor: invoicePage.NextCursor,
	})
	if err != nil || len(invoiceNext.Invoices) == 0 {
		t.Fatalf("management invoice next page=%+v err=%v", invoiceNext, err)
	}
	invoiceDetail, err := reader.GetManagementInvoice(ctx, "other-customer-invoice", InvoiceItemDirectoryFilter{})
	if err != nil || invoiceDetail.Invoice.CustomerNo != "customer-ledger-other" || len(invoiceDetail.Items) != 1 {
		t.Fatalf("management invoice detail=%+v err=%v", invoiceDetail, err)
	}
	transactionPage, err := reader.ListManagementTransactions(ctx, ManagementTransactionDirectoryFilter{
		CustomerNo: "customer-ledger",
	})
	if err != nil || len(transactionPage.Transactions) != 1 ||
		transactionPage.Transactions[0].TransactionNo != posting.TransactionNo {
		t.Fatalf("management transaction page=%+v err=%v", transactionPage, err)
	}
	transactionDetail, err := reader.GetManagementTransaction(ctx, posting.TransactionNo, InvoiceItemDirectoryFilter{})
	if err != nil || transactionDetail.EntryCount != 2 || len(transactionDetail.Entries) != 2 ||
		transactionDetail.AmountMinor != 2_000 {
		t.Fatalf("management transaction detail=%+v err=%v", transactionDetail, err)
	}
	adjustmentPage, err := reader.ListManagementAdjustments(ctx, ManagementAdjustmentDirectoryFilter{
		StatusCode: "pending_review",
	})
	if err != nil || len(adjustmentPage.Adjustments) != 1 ||
		adjustmentPage.Adjustments[0].AdjustmentNo != pending.AdjustmentNo {
		t.Fatalf("management adjustment page=%+v err=%v", adjustmentPage, err)
	}
	adjustmentDetail, err := reader.GetManagementAdjustment(ctx, pending.AdjustmentNo)
	if err != nil || adjustmentDetail.ProofReference != "private-proof-management-001" ||
		adjustmentDetail.RequestedBy != 9001 || adjustmentDetail.StatusCode != "pending_review" {
		t.Fatalf("management adjustment detail=%+v err=%v", adjustmentDetail, err)
	}
	for index := 1; index < 25; index++ {
		if _, err = balance.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
			CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 2000,
			SourceReference: fmt.Sprintf("numbered-recharge-%02d", index), SourceSnapshotDigest: strings.Repeat("7", 64),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err = seedLegacyReceipt(t, ctx, adjustments, ManualReceiptInput{
			CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 800,
			ReasonCode: "offline_balance_credit", ProofReference: fmt.Sprintf("numbered-proof-%02d", index),
			ReceivingAccountReference: "merchant-collection-cny-001", BusinessPurposeCode: "balance_recharge",
			EvidenceDigest: strings.Repeat("8", 64), IdempotencyKey: fmt.Sprintf("numbered-adjustment-%02d", index), RequestedBy: 9001,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, scenario := range []struct{ page, size, count int }{{1, 10, 10}, {2, 10, 10}, {3, 10, 5}, {50, 10, 0}, {1, 100, 25}} {
		transactions, readErr := reader.ListManagementTransactions(ctx, ManagementTransactionDirectoryFilter{CustomerNo: "customer-ledger", Page: scenario.page, Limit: scenario.size})
		if readErr != nil || transactions.Total != 25 || len(transactions.Transactions) != scenario.count {
			t.Fatalf("numbered transactions %+v: %+v, %v", scenario, transactions, readErr)
		}
		requests, readErr := reader.ListManagementAdjustments(ctx, ManagementAdjustmentDirectoryFilter{CustomerNo: "customer-ledger", StatusCode: "pending_review", Page: scenario.page, Limit: scenario.size})
		if readErr != nil || requests.Total != 25 || len(requests.Adjustments) != scenario.count {
			t.Fatalf("numbered adjustments %+v: %+v, %v", scenario, requests, readErr)
		}
	}
	invoiceCount, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND customer_no='customer-ledger'`)
	if err != nil {
		t.Fatal(err)
	}
	for _, pageNumber := range []int{1, 2, 50} {
		invoices, readErr := reader.ListManagementInvoices(ctx, ManagementInvoiceDirectoryFilter{CustomerNo: "customer-ledger", Page: pageNumber, Limit: 2})
		expected := min(2, max(invoiceCount.Int()-(pageNumber-1)*2, 0))
		if readErr != nil || invoices.Total != invoiceCount.Int() || len(invoices.Invoices) != expected {
			t.Fatalf("numbered invoices %d: %+v, %v", pageNumber, invoices, readErr)
		}
	}
	for _, customer := range []string{"customer-ledger-other", "customer-missing"} {
		transactions, readErr := reader.ListManagementTransactions(ctx, ManagementTransactionDirectoryFilter{CustomerNo: customer, Page: 1, Limit: 10})
		if readErr != nil || transactions.Total != 0 || len(transactions.Transactions) != 0 {
			t.Fatalf("scoped empty transactions: %+v, %v", transactions, readErr)
		}
	}
}

func TestManagementFinancialNumberedPageBounds(t *testing.T) {
	for _, scenario := range []struct {
		page, size int
		cursor     string
	}{{-1, 10, ""}, {1, 0, ""}, {1, 101, ""}, {1, 10, "sealed"}, {math.MaxInt, 100, ""}} {
		if validManagementPage(scenario.page, scenario.size, scenario.cursor) {
			t.Fatalf("accepted invalid page: %+v", scenario)
		}
	}
	if !validManagementPage(3, 100, "") || !validManagementPage(0, 10, "sealed") {
		t.Fatal("rejected valid numeric or keyset page")
	}
}
