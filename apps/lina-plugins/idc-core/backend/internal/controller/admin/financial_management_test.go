package admin

import (
	"context"
	"testing"
	"time"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

type financialManagementReaderRecordingStub struct {
	invoiceFilter billing.ManagementInvoiceDirectoryFilter
}

func (*financialManagementReaderRecordingStub) GetManagementFinancialOverview(context.Context) (*billing.ManagementFinancialOverview, error) {
	return &billing.ManagementFinancialOverview{
		OpenInvoiceCount: 4, OverdueInvoiceCount: 1, PendingAdjustmentCount: 2,
		Receivables: []billing.ManagementReceivableSummary{{CurrencyCode: "CNY", OutstandingMinor: 12_345}},
	}, nil
}
func (s *financialManagementReaderRecordingStub) ListManagementInvoices(_ context.Context, filter billing.ManagementInvoiceDirectoryFilter) (*billing.InvoiceDirectoryPage, error) {
	s.invoiceFilter = filter
	now := time.Unix(1_800_000_000, 123_000_000).UTC()
	return &billing.InvoiceDirectoryPage{Invoices: []billing.InvoiceDirectoryItem{{
		InvoiceNo: "invoice-controller", CustomerNo: "customer-controller", CurrencyCode: "CNY",
		StatusCode: "credit_settled", TotalAmountMinor: 12_345, CreditAmountMinor: 12_345,
		PaymentDueAt: now.Add(time.Hour), CreatedAt: now,
	}}, NextCursor: "invoice-next"}, nil
}
func (*financialManagementReaderRecordingStub) GetManagementInvoice(context.Context, string, billing.InvoiceItemDirectoryFilter) (*billing.InvoiceDirectoryDetail, error) {
	return nil, nil
}
func (*financialManagementReaderRecordingStub) ListManagementTransactions(context.Context, billing.ManagementTransactionDirectoryFilter) (*billing.ManagementTransactionDirectoryPage, error) {
	return &billing.ManagementTransactionDirectoryPage{Transactions: []billing.ManagementTransactionDirectoryItem{}}, nil
}
func (*financialManagementReaderRecordingStub) GetManagementTransaction(context.Context, string, billing.InvoiceItemDirectoryFilter) (*billing.ManagementTransactionDetail, error) {
	return nil, nil
}
func (*financialManagementReaderRecordingStub) ListManagementAdjustments(context.Context, billing.ManagementAdjustmentDirectoryFilter) (*billing.ManagementAdjustmentDirectoryPage, error) {
	return &billing.ManagementAdjustmentDirectoryPage{Adjustments: []billing.ManagementAdjustmentDirectoryItem{}}, nil
}
func (*financialManagementReaderRecordingStub) GetManagementAdjustment(context.Context, string) (*billing.ManagementAdjustmentDetail, error) {
	return nil, nil
}

type financialManagementCommandRecordingStub struct {
	receiptIntent billing.ManagementManualReceiptIntent
	proofToken    string
}

func (s *financialManagementCommandRecordingStub) IssueManualReceiptProof(_ context.Context, intent billing.ManagementManualReceiptIntent) (*reauth.ProofContext, error) {
	s.receiptIntent = intent
	return &reauth.ProofContext{Token: "opaque-proof", ExpiresAt: time.Unix(1_800_000_000, 0).UTC()}, nil
}
func (s *financialManagementCommandRecordingStub) CreateManualReceipt(_ context.Context, intent billing.ManagementManualReceiptIntent, proofToken string) (*billing.FinancialAdjustment, error) {
	s.receiptIntent = intent
	s.proofToken = proofToken
	return &billing.FinancialAdjustment{
		AdjustmentNo: "adjustment-controller", AdjustmentType: "manual_receipt",
		Status: billing.AdjustmentStatusPendingReview, CustomerNo: intent.CustomerNo,
		CurrencyCode: intent.CurrencyCode, AmountMinor: intent.AmountMinor, ApprovalRequired: true,
	}, nil
}
func (*financialManagementCommandRecordingStub) IssueInvoiceWaiverProof(context.Context, billing.ManagementInvoiceWaiverIntent) (*reauth.ProofContext, error) {
	return &reauth.ProofContext{}, nil
}
func (*financialManagementCommandRecordingStub) CreateInvoiceWaiver(context.Context, billing.ManagementInvoiceWaiverIntent, string) (*billing.FinancialAdjustment, error) {
	return nil, nil
}
func (*financialManagementCommandRecordingStub) IssueAdjustmentReviewProof(context.Context, billing.ManagementAdjustmentReviewIntent) (*reauth.ProofContext, error) {
	return &reauth.ProofContext{}, nil
}
func (*financialManagementCommandRecordingStub) ReviewAdjustment(context.Context, billing.ManagementAdjustmentReviewIntent, string) (*billing.FinancialAdjustment, error) {
	return nil, nil
}
func (*financialManagementCommandRecordingStub) IssueAdjustmentPolicyProof(context.Context, billing.ManagementAdjustmentPolicyIntent) (*reauth.ProofContext, error) {
	return &reauth.ProofContext{}, nil
}
func (*financialManagementCommandRecordingStub) ConfigureAdjustmentPolicy(context.Context, billing.ManagementAdjustmentPolicyIntent, string) (*billing.AdjustmentPolicy, error) {
	return nil, nil
}

func TestFinancialManagementControllerMapsBoundedReadsAndExactAmounts(t *testing.T) {
	t.Parallel()
	reader := &financialManagementReaderRecordingStub{}
	controller := &ControllerV1{finance: reader}
	overview, err := controller.FinancialManagementOverviewGet(
		context.Background(), &v1.FinancialManagementOverviewGetReq{},
	)
	if err != nil || overview.Overview.Receivables[0].OutstandingMinor != "12345" ||
		overview.Overview.PendingAdjustmentCount != 2 {
		t.Fatalf("overview=%+v err=%v", overview, err)
	}
	page, err := controller.FinancialManagementInvoiceList(
		context.Background(), &v1.FinancialManagementInvoiceListReq{
			CustomerNo: "customer-controller", CurrencyCode: "CNY", Cursor: "cursor", Limit: 17,
		},
	)
	if err != nil || len(page.Invoices) != 1 || page.Invoices[0].TotalAmountMinor != "12345" ||
		page.Invoices[0].CreditAmountMinor != "12345" || page.Invoices[0].PaidAmountMinor != "0" ||
		page.NextCursor != "invoice-next" || reader.invoiceFilter.CustomerNo != "customer-controller" ||
		reader.invoiceFilter.Limit != 17 || page.Invoices[0].CreatedAt != 1_800_000_000_123 {
		t.Fatalf("page=%+v filter=%+v err=%v", page, reader.invoiceFilter, err)
	}
}

func TestFinancialManagementControllerMapsProofAndRejectsNonCanonicalAmount(t *testing.T) {
	t.Parallel()
	commands := &financialManagementCommandRecordingStub{}
	controller := &ControllerV1{financeCommands: commands}
	intent := v1.FinancialManagementManualReceiptIntent{
		RequestNo: "controller-receipt-00001", CustomerNo: "customer-controller",
		CurrencyCode: "CNY", AmountMinor: "1200", ReasonCode: "offline_transfer",
		ProofReference: "private-proof-controller", ReceivingAccountReference: "merchant-account-controller",
		BusinessPurposeCode: "balance_recharge", EvidenceDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	proof, err := controller.FinancialManagementManualReceiptProof(
		context.Background(), &v1.FinancialManagementManualReceiptProofReq{
			FinancialManagementManualReceiptIntent: intent,
		},
	)
	if err != nil || proof.Proof.ProofToken != "opaque-proof" || proof.Proof.ExpiresAt != 1_800_000_000_000 ||
		commands.receiptIntent.AmountMinor != 1200 {
		t.Fatalf("proof=%+v intent=%+v err=%v", proof, commands.receiptIntent, err)
	}
	created, err := controller.FinancialManagementManualReceiptCreate(
		context.Background(), &v1.FinancialManagementManualReceiptCreateReq{
			FinancialManagementManualReceiptIntent: intent, ProofToken: "opaque-proof",
		},
	)
	if err != nil || created.Adjustment.AmountMinor != "1200" || !created.Adjustment.ApprovalRequired ||
		commands.proofToken != "opaque-proof" {
		t.Fatalf("created=%+v command=%+v err=%v", created, commands, err)
	}
	intent.AmountMinor = "01200"
	if _, err = controller.FinancialManagementManualReceiptProof(
		context.Background(), &v1.FinancialManagementManualReceiptProofReq{
			FinancialManagementManualReceiptIntent: intent,
		},
	); err == nil {
		t.Fatal("non-canonical minor-unit amount reached command service")
	}
}

func (s *financialManagementCommandRecordingStub) IssueBalanceAdjustmentProof(_ context.Context, intent billing.ManagementBalanceAdjustmentIntent) (*reauth.ProofContext, error) {
	return &reauth.ProofContext{Token: "opaque-proof", ExpiresAt: time.Unix(1_800_000_000, 0).UTC()}, nil
}

func (s *financialManagementCommandRecordingStub) CreateBalanceAdjustment(_ context.Context, intent billing.ManagementBalanceAdjustmentIntent, proofToken string) (*billing.FinancialAdjustment, error) {
	s.proofToken = proofToken
	return &billing.FinancialAdjustment{
		AdjustmentNo: "adjustment-controller", AdjustmentType: "manual_receipt",
		Status: billing.AdjustmentStatusPendingReview, CustomerNo: intent.CustomerNo,
		CurrencyCode: intent.CurrencyCode, AmountMinor: intent.AmountMinor, ApprovalRequired: true,
	}, nil
}
