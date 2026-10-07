//go:build legacy_server_catalog_tests

// This file is retained as migration evidence for the removed fixed server
// catalog controller contract.

package admin

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/customermanagement"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

func TestControllerDoesNotExposeDependencyBindingAsHTTPMethod(t *testing.T) {
	if _, exposed := reflect.TypeOf((*ControllerV1)(nil)).MethodByName("BindContractPolicies"); exposed {
		t.Fatal("dependency binding must not be exposed as a controller method")
	}
}

func intTestPointer(value int) *int { return &value }

type controllerManagementService struct {
	page             *customermanagement.Page
	overview         *customermanagement.Overview
	search           customermanagement.SearchInput
	segmentSearch    customermanagement.SegmentSearchInput
	assignmentSearch customermanagement.AssignmentSearchInput
	getNo            string
}

type controllerMutationService struct {
	input  customermanagement.TagBatchInput
	result *customermanagement.TagBatchResult
}

type marketingOverviewReaderStub struct {
	overview *marketing.ManagementOverview
	calls    int
}

type marketingDirectoryReaderStub struct {
	segmentInput   marketing.SegmentManagementSearchInput
	segmentNo      string
	segmentPage    *marketing.SegmentManagementPage
	segmentDetail  *marketing.SegmentManagementDetail
	campaignInput  marketing.CampaignManagementSearchInput
	campaignNo     string
	campaignPage   *marketing.CampaignManagementPage
	campaignDetail *marketing.CampaignManagementDetail
}

type marketingAssetDirectoryReaderStub struct {
	assetInput   marketing.PromotionAssetSearchInput
	assetNo      string
	assetPage    *marketing.PromotionAssetPage
	assetDetail  *marketing.PromotionAssetDetail
	pointsInput  marketing.PointsAccountSearchInput
	customerNo   string
	pointsPage   *marketing.PointsAccountPage
	pointsDetail *marketing.PointsAccountDetail
	grantInput   marketing.ValueGrantSearchInput
	grantNo      string
	grantPage    *marketing.ValueGrantPage
	grantDetail  *marketing.ValueGrantDetail
}

type marketingActionServiceStub struct {
	issueInput        marketing.AssetIssueInput
	issueResult       *marketing.ManagedAssetMutation
	revokeInput       marketing.AssetRevokeInput
	revokeResult      *marketing.ManagedAssetMutation
	creditInput       marketing.PointsCreditInput
	creditResult      *marketing.ManagedPointsMutation
	reverseInput      marketing.PointsReverseManagementInput
	reverseResult     *marketing.ManagedPointsMutation
	grantRequestInput marketing.ValueGrantFinancialRequestInput
	grantReviewInput  marketing.ValueGrantFinancialReviewInput
	grantResult       *marketing.ManagedValueGrantFinancialMutation
	grantPage         *marketing.ManagedValueGrantFinancialAdjustmentPage
	grantListInput    marketing.ValueGrantFinancialAdjustmentSearchInput
}

func (s *marketingActionServiceStub) RequestValueGrantPosting(_ context.Context, input marketing.ValueGrantFinancialRequestInput) (*marketing.ManagedValueGrantFinancialMutation, error) {
	s.grantRequestInput = input
	return s.grantResult, nil
}

func (s *marketingActionServiceStub) RequestValueGrantReversal(_ context.Context, input marketing.ValueGrantFinancialRequestInput) (*marketing.ManagedValueGrantFinancialMutation, error) {
	s.grantRequestInput = input
	return s.grantResult, nil
}

func (s *marketingActionServiceStub) ReviewValueGrantAdjustment(_ context.Context, input marketing.ValueGrantFinancialReviewInput) (*marketing.ManagedValueGrantFinancialMutation, error) {
	s.grantReviewInput = input
	return s.grantResult, nil
}

func (s *marketingActionServiceStub) ListValueGrantAdjustments(_ context.Context, input marketing.ValueGrantFinancialAdjustmentSearchInput) (*marketing.ManagedValueGrantFinancialAdjustmentPage, error) {
	s.grantListInput = input
	if s.grantPage == nil {
		return &marketing.ManagedValueGrantFinancialAdjustmentPage{
			Adjustments: []marketing.ManagedValueGrantFinancialAdjustment{},
		}, nil
	}
	return s.grantPage, nil
}

type commerceRiskReviewServiceStub struct {
	input  commerce.OrderRiskReviewDecisionInput
	result *commerce.OrderRiskReviewDecision
	err    error
}

type managementOrderReaderStub struct{}

type assistedQuoteServiceStub struct{}

type managementFinanceReaderStub struct{}

type managementFinanceCommandServiceStub struct{}

type managementCreditReaderStub struct{}

func (managementCreditReaderStub) ListManagementCreditEvidence(context.Context, string) ([]billing.CreditEvidenceItem, error) {
	return []billing.CreditEvidenceItem{}, nil
}
func (managementCreditReaderStub) OpenManagementCreditEvidence(context.Context, string, string) (*billing.CreditEvidenceOpen, error) {
	return nil, errors.New("credit evidence is not configured in this controller test")
}

type managementCreditCommandServiceStub struct{}

func (managementCreditReaderStub) ListManagementCreditApplications(context.Context, billing.CreditApplicationDirectoryFilter) (*billing.CreditApplicationDirectoryPage, error) {
	return &billing.CreditApplicationDirectoryPage{}, nil
}
func (managementCreditReaderStub) GetManagementCreditApplication(context.Context, string) (*billing.CreditApplicationDirectoryDetail, error) {
	return nil, nil
}
func (managementCreditReaderStub) ListManagementCreditAccounts(context.Context, billing.CreditAccountDirectoryFilter) (*billing.CreditAccountDirectoryPage, error) {
	return &billing.CreditAccountDirectoryPage{}, nil
}
func (managementCreditReaderStub) GetManagementCreditAccount(context.Context, string) (*billing.CreditAccountDirectoryItem, error) {
	return nil, nil
}
func (managementCreditReaderStub) ListManagementCreditStatements(context.Context, billing.CreditStatementDirectoryFilter) (*billing.CreditStatementDirectoryPage, error) {
	return &billing.CreditStatementDirectoryPage{}, nil
}
func (managementCreditReaderStub) GetManagementCreditStatement(context.Context, string, billing.CreditStatementLineDirectoryFilter) (*billing.CreditStatementDirectoryDetail, error) {
	return nil, nil
}
func (managementCreditCommandServiceStub) IssueCreditPolicyProof(context.Context, billing.ManagementCreditPolicyIntent) (*reauth.ProofContext, error) {
	return nil, nil
}
func (managementCreditCommandServiceStub) ConfigureCreditPolicy(context.Context, billing.ManagementCreditPolicyIntent, string) (*billing.CreditPolicy, error) {
	return nil, nil
}
func (managementCreditCommandServiceStub) IssueCreditApplicationReviewProof(context.Context, billing.ManagementCreditReviewIntent) (*reauth.ProofContext, error) {
	return nil, nil
}
func (managementCreditCommandServiceStub) ReviewCreditApplication(context.Context, billing.ManagementCreditReviewIntent, string) (*billing.CreditApplication, error) {
	return nil, nil
}

func (managementFinanceCommandServiceStub) IssueManualReceiptProof(context.Context, billing.ManagementManualReceiptIntent) (*reauth.ProofContext, error) {
	return &reauth.ProofContext{}, nil
}
func (managementFinanceCommandServiceStub) CreateManualReceipt(context.Context, billing.ManagementManualReceiptIntent, string) (*billing.FinancialAdjustment, error) {
	return nil, nil
}
func (managementFinanceCommandServiceStub) IssueInvoiceWaiverProof(context.Context, billing.ManagementInvoiceWaiverIntent) (*reauth.ProofContext, error) {
	return &reauth.ProofContext{}, nil
}
func (managementFinanceCommandServiceStub) CreateInvoiceWaiver(context.Context, billing.ManagementInvoiceWaiverIntent, string) (*billing.FinancialAdjustment, error) {
	return nil, nil
}
func (managementFinanceCommandServiceStub) IssueAdjustmentReviewProof(context.Context, billing.ManagementAdjustmentReviewIntent) (*reauth.ProofContext, error) {
	return &reauth.ProofContext{}, nil
}
func (managementFinanceCommandServiceStub) ReviewAdjustment(context.Context, billing.ManagementAdjustmentReviewIntent, string) (*billing.FinancialAdjustment, error) {
	return nil, nil
}
func (managementFinanceCommandServiceStub) IssueAdjustmentPolicyProof(context.Context, billing.ManagementAdjustmentPolicyIntent) (*reauth.ProofContext, error) {
	return &reauth.ProofContext{}, nil
}
func (managementFinanceCommandServiceStub) ConfigureAdjustmentPolicy(context.Context, billing.ManagementAdjustmentPolicyIntent, string) (*billing.AdjustmentPolicy, error) {
	return nil, nil
}

func (managementFinanceReaderStub) GetManagementFinancialOverview(context.Context) (*billing.ManagementFinancialOverview, error) {
	return &billing.ManagementFinancialOverview{Receivables: []billing.ManagementReceivableSummary{}}, nil
}

func (managementFinanceReaderStub) ListManagementInvoices(context.Context, billing.ManagementInvoiceDirectoryFilter) (*billing.InvoiceDirectoryPage, error) {
	return &billing.InvoiceDirectoryPage{Invoices: []billing.InvoiceDirectoryItem{}}, nil
}

func (managementFinanceReaderStub) GetManagementInvoice(context.Context, string, billing.InvoiceItemDirectoryFilter) (*billing.InvoiceDirectoryDetail, error) {
	return nil, nil
}

func (managementFinanceReaderStub) ListManagementTransactions(context.Context, billing.ManagementTransactionDirectoryFilter) (*billing.ManagementTransactionDirectoryPage, error) {
	return &billing.ManagementTransactionDirectoryPage{Transactions: []billing.ManagementTransactionDirectoryItem{}}, nil
}

func (managementFinanceReaderStub) GetManagementTransaction(context.Context, string, billing.InvoiceItemDirectoryFilter) (*billing.ManagementTransactionDetail, error) {
	return nil, nil
}

func (managementFinanceReaderStub) ListManagementAdjustments(context.Context, billing.ManagementAdjustmentDirectoryFilter) (*billing.ManagementAdjustmentDirectoryPage, error) {
	return &billing.ManagementAdjustmentDirectoryPage{Adjustments: []billing.ManagementAdjustmentDirectoryItem{}}, nil
}

func (managementFinanceReaderStub) GetManagementAdjustment(context.Context, string) (*billing.ManagementAdjustmentDetail, error) {
	return nil, nil
}

func (managementOrderReaderStub) ListManagementOrders(
	context.Context,
	commerce.OrderDirectoryFilter,
) (*commerce.OrderPage, error) {
	return &commerce.OrderPage{Orders: []commerce.OrderSummary{}}, nil
}

func (managementOrderReaderStub) GetManagementOrder(
	context.Context,
	string,
) (*commerce.OrderDetail, error) {
	return nil, nil
}

func (*assistedQuoteServiceStub) CreateAssistedQuote(
	context.Context, commerce.AssistedQuoteCreateInput,
) (*commerce.AssistedQuoteRequest, error) {
	return nil, nil
}

func (*assistedQuoteServiceStub) ListAssistedQuotes(
	context.Context, commerce.AssistedQuoteFilter,
) (*commerce.AssistedQuotePage, error) {
	return &commerce.AssistedQuotePage{Requests: []commerce.AssistedQuoteRequest{}}, nil
}

func (*assistedQuoteServiceStub) GetAssistedQuote(
	context.Context, string,
) (*commerce.AssistedQuoteRequest, error) {
	return nil, nil
}

func (*assistedQuoteServiceStub) DecideAssistedQuote(
	context.Context, commerce.AssistedQuoteDecisionInput,
) (*commerce.AssistedQuoteRequest, error) {
	return nil, nil
}

func (*assistedQuoteServiceStub) CheckoutAssistedQuote(
	context.Context, commerce.AssistedQuoteCheckoutInput,
) (*commerce.AssistedQuoteRequest, error) {
	return nil, nil
}

func (s *commerceRiskReviewServiceStub) DecideRiskReview(
	_ context.Context,
	input commerce.OrderRiskReviewDecisionInput,
) (*commerce.OrderRiskReviewDecision, error) {
	s.input = input
	return s.result, s.err
}

func (s *marketingActionServiceStub) IssueAsset(_ context.Context, input marketing.AssetIssueInput) (*marketing.ManagedAssetMutation, error) {
	s.issueInput = input
	return s.issueResult, nil
}

func (s *marketingActionServiceStub) RevokeAsset(_ context.Context, input marketing.AssetRevokeInput) (*marketing.ManagedAssetMutation, error) {
	s.revokeInput = input
	return s.revokeResult, nil
}

func (s *marketingActionServiceStub) CreditPoints(_ context.Context, input marketing.PointsCreditInput) (*marketing.ManagedPointsMutation, error) {
	s.creditInput = input
	return s.creditResult, nil
}

func (s *marketingActionServiceStub) ReversePointsAward(_ context.Context, input marketing.PointsReverseManagementInput) (*marketing.ManagedPointsMutation, error) {
	s.reverseInput = input
	return s.reverseResult, nil
}

// newAdminControllerForTest constructs a complete controller graph for mapping tests.
func newAdminControllerForTest(
	t *testing.T,
	customerManagement customermanagement.Service,
	customerMutation customermanagement.MutationService,
	catalogManagement catalog.ProductManagementReader,
	catalogWriter catalog.ManagementService,
	catalogPublication catalog.PublicationService,
	catalogPublicationPreview catalog.PublicationPreviewService,
	marketingOverview marketing.ManagementOverviewReader,
	marketingDirectory ...marketing.ManagementDirectoryReader,
) *ControllerV1 {
	t.Helper()
	if marketingOverview == nil {
		marketingOverview = &marketingOverviewReaderStub{}
	}
	directory := marketing.ManagementDirectoryReader(&marketingDirectoryReaderStub{})
	if len(marketingDirectory) > 0 && marketingDirectory[0] != nil {
		directory = marketingDirectory[0]
	}
	controller, err := NewV1(
		customerManagement,
		customerMutation,
		catalogManagement,
		catalogWriter,
		catalogPublication,
		catalogPublicationPreview,
		marketingOverview,
		directory,
		&marketingAssetDirectoryReaderStub{},
		&marketingActionServiceStub{},
		&commerceRiskReviewServiceStub{},
		managementOrderReaderStub{},
		&assistedQuoteServiceStub{},
		managementFinanceReaderStub{},
		managementFinanceCommandServiceStub{},
		managementCreditReaderStub{},
		managementCreditCommandServiceStub{},
	)
	if err != nil {
		t.Fatalf("create admin controller: %v", err)
	}
	return controller
}

func (s *marketingDirectoryReaderStub) SearchSegments(_ context.Context, input marketing.SegmentManagementSearchInput) (*marketing.SegmentManagementPage, error) {
	s.segmentInput = input
	return s.segmentPage, nil
}
func (s *marketingDirectoryReaderStub) GetSegment(_ context.Context, segmentNo string) (*marketing.SegmentManagementDetail, error) {
	s.segmentNo = segmentNo
	return s.segmentDetail, nil
}
func (s *marketingDirectoryReaderStub) SearchCampaigns(_ context.Context, input marketing.CampaignManagementSearchInput) (*marketing.CampaignManagementPage, error) {
	s.campaignInput = input
	return s.campaignPage, nil
}
func (s *marketingDirectoryReaderStub) GetCampaign(_ context.Context, campaignNo string) (*marketing.CampaignManagementDetail, error) {
	s.campaignNo = campaignNo
	return s.campaignDetail, nil
}

func (s *marketingAssetDirectoryReaderStub) SearchAssets(_ context.Context, input marketing.PromotionAssetSearchInput) (*marketing.PromotionAssetPage, error) {
	s.assetInput = input
	if s.assetPage == nil {
		return &marketing.PromotionAssetPage{Assets: []marketing.PromotionAssetSummary{}}, nil
	}
	return s.assetPage, nil
}
func (s *marketingAssetDirectoryReaderStub) GetAsset(_ context.Context, assetNo string) (*marketing.PromotionAssetDetail, error) {
	s.assetNo = assetNo
	if s.assetDetail == nil {
		return &marketing.PromotionAssetDetail{Reservations: []marketing.PromotionAssetReservationSummary{}}, nil
	}
	return s.assetDetail, nil
}
func (s *marketingAssetDirectoryReaderStub) SearchPointsAccounts(_ context.Context, input marketing.PointsAccountSearchInput) (*marketing.PointsAccountPage, error) {
	s.pointsInput = input
	if s.pointsPage == nil {
		return &marketing.PointsAccountPage{Accounts: []marketing.PointsAccountSummary{}}, nil
	}
	return s.pointsPage, nil
}
func (s *marketingAssetDirectoryReaderStub) GetPointsAccount(_ context.Context, customerNo string) (*marketing.PointsAccountDetail, error) {
	s.customerNo = customerNo
	if s.pointsDetail == nil {
		return &marketing.PointsAccountDetail{Lots: []marketing.PointsLotSummary{}, Entries: []marketing.PointsEntrySummary{}}, nil
	}
	return s.pointsDetail, nil
}
func (s *marketingAssetDirectoryReaderStub) SearchValueGrants(_ context.Context, input marketing.ValueGrantSearchInput) (*marketing.ValueGrantPage, error) {
	s.grantInput = input
	if s.grantPage == nil {
		return &marketing.ValueGrantPage{Grants: []marketing.ValueGrantSummary{}}, nil
	}
	return s.grantPage, nil
}
func (s *marketingAssetDirectoryReaderStub) GetValueGrant(_ context.Context, grantNo string) (*marketing.ValueGrantDetail, error) {
	s.grantNo = grantNo
	if s.grantDetail == nil {
		return &marketing.ValueGrantDetail{Entries: []marketing.ValueEntrySummary{}}, nil
	}
	return s.grantDetail, nil
}

func (s *marketingOverviewReaderStub) GetOverview(context.Context) (*marketing.ManagementOverview, error) {
	s.calls++
	return s.overview, nil
}

// TestNewV1RejectsMissingRuntimeDependency verifies startup fails for nil and typed-nil services.
func TestNewV1RejectsMissingRuntimeDependency(t *testing.T) {
	t.Parallel()
	arguments := func(marketingOverview marketing.ManagementOverviewReader) (*ControllerV1, error) {
		return NewV1(
			&controllerManagementService{},
			&controllerMutationService{},
			&catalogProductManagementReaderStub{},
			&catalogManagementServiceStub{},
			&catalogPublicationServiceStub{},
			&catalogPublicationPreviewServiceStub{},
			marketingOverview,
			&marketingDirectoryReaderStub{},
			&marketingAssetDirectoryReaderStub{},
			&marketingActionServiceStub{},
			&commerceRiskReviewServiceStub{},
			managementOrderReaderStub{},
			&assistedQuoteServiceStub{},
			managementFinanceReaderStub{},
			managementFinanceCommandServiceStub{},
			managementCreditReaderStub{},
			managementCreditCommandServiceStub{},
		)
	}
	if _, err := arguments(nil); err == nil {
		t.Fatal("nil marketing overview reader must be rejected")
	}
	var typedNil *marketingOverviewReaderStub
	if _, err := arguments(typedNil); err == nil {
		t.Fatal("typed nil marketing overview reader must be rejected")
	}
	directoryArguments := func(directory marketing.ManagementDirectoryReader) (*ControllerV1, error) {
		return NewV1(
			&controllerManagementService{}, &controllerMutationService{},
			&catalogProductManagementReaderStub{}, &catalogManagementServiceStub{},
			&catalogPublicationServiceStub{}, &catalogPublicationPreviewServiceStub{},
			&marketingOverviewReaderStub{}, directory,
			&marketingAssetDirectoryReaderStub{},
			&marketingActionServiceStub{},
			&commerceRiskReviewServiceStub{},
			managementOrderReaderStub{},
			&assistedQuoteServiceStub{},
			managementFinanceReaderStub{},
			managementFinanceCommandServiceStub{},
			managementCreditReaderStub{},
			managementCreditCommandServiceStub{},
		)
	}
	if _, err := directoryArguments(nil); err == nil {
		t.Fatal("nil marketing directory reader must be rejected")
	}
	var typedNilDirectory *marketingDirectoryReaderStub
	if _, err := directoryArguments(typedNilDirectory); err == nil {
		t.Fatal("typed nil marketing directory reader must be rejected")
	}
	actionArguments := func(actions marketing.ManagementActionService) (*ControllerV1, error) {
		return NewV1(
			&controllerManagementService{}, &controllerMutationService{},
			&catalogProductManagementReaderStub{}, &catalogManagementServiceStub{},
			&catalogPublicationServiceStub{}, &catalogPublicationPreviewServiceStub{},
			&marketingOverviewReaderStub{}, &marketingDirectoryReaderStub{},
			&marketingAssetDirectoryReaderStub{}, actions,
			&commerceRiskReviewServiceStub{},
			managementOrderReaderStub{},
			&assistedQuoteServiceStub{},
			managementFinanceReaderStub{},
			managementFinanceCommandServiceStub{},
			managementCreditReaderStub{},
			managementCreditCommandServiceStub{},
		)
	}
	if _, err := actionArguments(nil); err == nil {
		t.Fatal("nil marketing action service must be rejected")
	}
	var typedNilActions *marketingActionServiceStub
	if _, err := actionArguments(typedNilActions); err == nil {
		t.Fatal("typed nil marketing action service must be rejected")
	}
	riskArguments := func(risk commerce.OrderRiskReviewService) (*ControllerV1, error) {
		return NewV1(
			&controllerManagementService{}, &controllerMutationService{},
			&catalogProductManagementReaderStub{}, &catalogManagementServiceStub{},
			&catalogPublicationServiceStub{}, &catalogPublicationPreviewServiceStub{},
			&marketingOverviewReaderStub{}, &marketingDirectoryReaderStub{},
			&marketingAssetDirectoryReaderStub{}, &marketingActionServiceStub{}, risk,
			managementOrderReaderStub{},
			&assistedQuoteServiceStub{},
			managementFinanceReaderStub{},
			managementFinanceCommandServiceStub{},
			managementCreditReaderStub{},
			managementCreditCommandServiceStub{},
		)
	}
	if _, err := riskArguments(nil); err == nil {
		t.Fatal("nil commerce risk review service must be rejected")
	}
	var typedNilRisk *commerceRiskReviewServiceStub
	if _, err := riskArguments(typedNilRisk); err == nil {
		t.Fatal("typed nil commerce risk review service must be rejected")
	}
	orderArguments := func(orders commerce.ManagementOrderReader) (*ControllerV1, error) {
		return NewV1(
			&controllerManagementService{}, &controllerMutationService{},
			&catalogProductManagementReaderStub{}, &catalogManagementServiceStub{},
			&catalogPublicationServiceStub{}, &catalogPublicationPreviewServiceStub{},
			&marketingOverviewReaderStub{}, &marketingDirectoryReaderStub{},
			&marketingAssetDirectoryReaderStub{}, &marketingActionServiceStub{},
			&commerceRiskReviewServiceStub{}, orders,
			&assistedQuoteServiceStub{},
			managementFinanceReaderStub{},
			managementFinanceCommandServiceStub{},
			managementCreditReaderStub{},
			managementCreditCommandServiceStub{},
		)
	}
	if _, err := orderArguments(nil); err == nil {
		t.Fatal("nil commerce order reader must be rejected")
	}
	var typedNilOrders *managementOrderReaderStub
	if _, err := orderArguments(typedNilOrders); err == nil {
		t.Fatal("typed nil commerce order reader must be rejected")
	}
}

func TestMarketingActionControllerMapsRequestsAndResults(t *testing.T) {
	t.Parallel()
	now := time.Unix(1767225600, 123000000).UTC()
	expiresAt := now.Add(24 * time.Hour).UnixMilli()
	actions := &marketingActionServiceStub{
		issueResult:   &marketing.ManagedAssetMutation{AssetNo: "mast-1", CustomerNo: "customer-1", Status: "active", UpdatedAt: now},
		revokeResult:  &marketing.ManagedAssetMutation{AssetNo: "mast-1", CustomerNo: "customer-1", Status: "revoked", UpdatedAt: now.Add(time.Second)},
		creditResult:  &marketing.ManagedPointsMutation{LotNo: "lot-credit", CustomerNo: "customer-1", AvailablePoints: 1200},
		reverseResult: &marketing.ManagedPointsMutation{LotNo: "lot-credit", CustomerNo: "customer-1", AvailablePoints: 700},
		grantResult: &marketing.ManagedValueGrantFinancialMutation{
			AdjustmentNo: "bar-1", GrantNo: "grant-1", AdjustmentType: "marketing_grant_post",
			Status: "pending_review", CustomerNo: "customer-1", CurrencyCode: "CNY",
			AmountMinor: 8800,
		},
		grantPage: &marketing.ManagedValueGrantFinancialAdjustmentPage{
			Adjustments: []marketing.ManagedValueGrantFinancialAdjustment{{
				ManagedValueGrantFinancialMutation: marketing.ManagedValueGrantFinancialMutation{
					AdjustmentNo: "bar-1", GrantNo: "grant-1", AdjustmentType: "marketing_grant_post",
					Status: "pending_review", CustomerNo: "customer-1", CurrencyCode: "CNY", AmountMinor: 8800,
				},
				RequestedBy: 17, CreatedAt: now,
			}},
			NextCursor: "bar-next",
		},
	}
	controller, err := NewV1(
		&controllerManagementService{}, &controllerMutationService{}, &catalogProductManagementReaderStub{},
		&catalogManagementServiceStub{}, &catalogPublicationServiceStub{}, &catalogPublicationPreviewServiceStub{},
		&marketingOverviewReaderStub{}, &marketingDirectoryReaderStub{}, &marketingAssetDirectoryReaderStub{}, actions,
		&commerceRiskReviewServiceStub{},
		managementOrderReaderStub{},
		&assistedQuoteServiceStub{},
		managementFinanceReaderStub{},
		managementFinanceCommandServiceStub{},
		managementCreditReaderStub{},
		managementCreditCommandServiceStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := controller.MarketingAssetIssue(context.Background(), &v1.MarketingAssetIssueReq{
		RequestNo: "issue-1", CampaignNo: "campaign-1", BenefitNo: "benefit-1", CustomerNo: "customer-1", ReasonCode: "customer_service",
	})
	if err != nil || issued.Mutation.AssetNo != "mast-1" || issued.Mutation.UpdatedAt != now.UnixMilli() || actions.issueInput.CampaignNo != "campaign-1" {
		t.Fatalf("map asset issue result=%+v input=%+v err=%v", issued, actions.issueInput, err)
	}
	revoked, err := controller.MarketingAssetRevoke(context.Background(), &v1.MarketingAssetRevokeReq{
		AssetNo: "mast-1", RequestNo: "revoke-1", ExpectedUpdatedAt: now.UnixMilli(), ReasonCode: "risk_control",
	})
	if err != nil || revoked.Mutation.Status != "revoked" || !actions.revokeInput.ExpectedUpdatedAt.Equal(time.UnixMilli(now.UnixMilli()).UTC()) {
		t.Fatalf("map asset revoke result=%+v input=%+v err=%v", revoked, actions.revokeInput, err)
	}
	credited, err := controller.MarketingPointsCredit(context.Background(), &v1.MarketingPointsCreditReq{
		CustomerNo: "customer-1", RequestNo: "credit-1", Points: 1200, ExpiresAt: &expiresAt, ReasonCode: "manual_correction",
	})
	if err != nil || credited.Mutation.AvailablePoints != 1200 || actions.creditInput.ExpiresAt == nil || actions.creditInput.ExpiresAt.UnixMilli() != expiresAt {
		t.Fatalf("map points credit result=%+v input=%+v err=%v", credited, actions.creditInput, err)
	}
	reversed, err := controller.MarketingPointsReverse(context.Background(), &v1.MarketingPointsReverseReq{
		CustomerNo: "customer-1", RequestNo: "reverse-1", AwardSourceType: "manual", AwardSourceReference: "credit-1", Points: 500, ReasonCode: "refund_correction",
	})
	if err != nil || reversed.Mutation.AvailablePoints != 700 || actions.reverseInput.AwardSourceReference != "credit-1" {
		t.Fatalf("map points reverse result=%+v input=%+v err=%v", reversed, actions.reverseInput, err)
	}
	posted, err := controller.MarketingValueGrantPostRequest(context.Background(), &v1.MarketingValueGrantPostRequestReq{
		GrantNo: "grant-1", RequestNo: "post-1", ReasonCode: "campaign_fulfillment",
	})
	if err != nil || posted.Mutation.AmountMinor != "8800" || actions.grantRequestInput.GrantNo != "grant-1" {
		t.Fatalf("map value-grant posting result=%+v input=%+v err=%v", posted, actions.grantRequestInput, err)
	}
	actions.grantResult.AdjustmentType = "marketing_grant_reverse"
	reversal, err := controller.MarketingValueGrantReverseRequest(context.Background(), &v1.MarketingValueGrantReverseRequestReq{
		GrantNo: "grant-1", RequestNo: "reverse-1", AmountMinor: "250", ReasonCode: "refund_correction",
	})
	if err != nil || reversal.Mutation.AdjustmentType != "marketing_grant_reverse" || actions.grantRequestInput.AmountMinor != 250 {
		t.Fatalf("map value-grant reversal result=%+v input=%+v err=%v", reversal, actions.grantRequestInput, err)
	}
	reviewed, err := controller.MarketingValueGrantAdjustmentReview(context.Background(), &v1.MarketingValueGrantAdjustmentReviewReq{
		AdjustmentNo: "bar-1", Decision: "approve", ReasonCode: "manual_correction",
	})
	if err != nil || reviewed.Mutation.AdjustmentNo != "bar-1" || actions.grantReviewInput.Decision != "approve" {
		t.Fatalf("map value-grant review result=%+v input=%+v err=%v", reviewed, actions.grantReviewInput, err)
	}
	queue, err := controller.MarketingValueGrantAdjustmentList(context.Background(), &v1.MarketingValueGrantAdjustmentListReq{
		Cursor: "bar-cursor", Limit: 25, Status: "pending_review",
	})
	if err != nil || len(queue.Adjustments) != 1 || queue.Adjustments[0].AmountMinor != "8800" ||
		queue.Adjustments[0].CreatedAt != now.UnixMilli() || queue.NextCursor != "bar-next" ||
		actions.grantListInput.Cursor != "bar-cursor" {
		t.Fatalf("map value-grant review queue result=%+v input=%+v err=%v", queue, actions.grantListInput, err)
	}
}

func TestCommerceRiskReviewControllerSeparatesDecisionsAndMapsUnixMilliseconds(t *testing.T) {
	t.Parallel()
	reviewedAt := time.Date(2026, 8, 10, 15, 30, 0, 123000000, time.UTC)
	risk := &commerceRiskReviewServiceStub{result: &commerce.OrderRiskReviewDecision{
		ReviewNo: "review-risk-001", OrderNo: "order-001", CustomerNo: "customer-001",
		StatusCode: commerce.OrderRiskReviewDecisionApproved, ReasonCode: "risk_approved",
		ReviewedBy: 1001, ReviewedAt: reviewedAt, OrderStatus: "pending_payment",
	}}
	controller, err := NewV1(
		&controllerManagementService{}, &controllerMutationService{}, &catalogProductManagementReaderStub{},
		&catalogManagementServiceStub{}, &catalogPublicationServiceStub{}, &catalogPublicationPreviewServiceStub{},
		&marketingOverviewReaderStub{}, &marketingDirectoryReaderStub{}, &marketingAssetDirectoryReaderStub{},
		&marketingActionServiceStub{}, risk, managementOrderReaderStub{},
		&assistedQuoteServiceStub{},
		managementFinanceReaderStub{},
		managementFinanceCommandServiceStub{},
		managementCreditReaderStub{},
		managementCreditCommandServiceStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	approved, err := controller.CommerceOrderRiskReviewApproval(
		context.Background(),
		&v1.CommerceOrderRiskReviewApprovalReq{
			ReviewNo: "review-risk-001", RequestNo: "risk-approve-001", EvidenceDigest: digest,
		},
	)
	if err != nil || approved.Decision.ReviewedAt != reviewedAt.UnixMilli() ||
		approved.Decision.StatusCode != commerce.OrderRiskReviewDecisionApproved ||
		risk.input.DecisionCode != commerce.OrderRiskReviewDecisionApproved ||
		risk.input.EvidenceDigest != digest {
		t.Fatalf("approval=%+v input=%+v err=%v", approved, risk.input, err)
	}
	risk.result.StatusCode = commerce.OrderRiskReviewDecisionRejected
	risk.result.ReasonCode = "risk_rejected"
	risk.result.OrderStatus = "cancelled"
	rejected, err := controller.CommerceOrderRiskReviewRejection(
		context.Background(),
		&v1.CommerceOrderRiskReviewRejectionReq{
			ReviewNo: "review-risk-001", RequestNo: "risk-reject-001", EvidenceDigest: digest,
		},
	)
	if err != nil || rejected.Decision.OrderStatus != "cancelled" ||
		risk.input.DecisionCode != commerce.OrderRiskReviewDecisionRejected ||
		risk.input.RequestNo != "risk-reject-001" {
		t.Fatalf("rejection=%+v input=%+v err=%v", rejected, risk.input, err)
	}
}

func TestMarketingDirectoryControllerMapsListsAndDetails(t *testing.T) {
	t.Parallel()
	now := time.Unix(1767225600, 0).UTC()
	directory := &marketingDirectoryReaderStub{
		segmentPage: &marketing.SegmentManagementPage{Segments: []marketing.SegmentManagementSummary{{
			SegmentNo: "segment-loyal", SegmentCode: "loyal", NameZhCN: "Loyal CN", NameEnUS: "Loyal",
			Mode: "dynamic", Status: "active", Version: 2, CreatedAt: now, UpdatedAt: now,
		}}, NextCursor: "segment-next"},
		campaignPage: &marketing.CampaignManagementPage{Campaigns: []marketing.CampaignManagementSummary{{
			CampaignNo: "campaign-summer", CampaignCode: "summer", NameZhCN: "Summer CN", NameEnUS: "Summer",
			Type: "cashback", Status: "active", StackPolicy: "exclusive", StartsAt: now, EndsAt: now.Add(time.Hour),
			CreatedAt: now, UpdatedAt: now,
		}}, NextCursor: "campaign-next"},
	}
	directory.segmentDetail = &marketing.SegmentManagementDetail{
		SegmentManagementSummary: directory.segmentPage.Segments[0], DescriptionZhCN: "desc-cn", DescriptionEnUS: "desc-en",
		Conditions: []marketing.SegmentCondition{{ConditionNo: "condition-1", OrGroupNo: 1, Fact: "customer_type", Operator: "eq", ValueKind: "text", ValueText: "enterprise", Values: []marketing.SegmentConditionValue{}}},
		Snapshots:  []marketing.SegmentSnapshotSummary{{SnapshotNo: "snapshot-1", SegmentVersion: 2, Status: "complete", MemberCount: 10, StartedAt: now}},
	}
	directory.campaignDetail = &marketing.CampaignManagementDetail{
		CampaignManagementSummary: directory.campaignPage.Campaigns[0], DescriptionZhCN: "desc-cn", DescriptionEnUS: "desc-en", StackGroup: "summer", Version: 3,
		Scopes:   []marketing.CampaignScope{{Kind: "product_type", MatchMode: "include", ReferenceCode: "cloud_server"}},
		Benefits: []marketing.CampaignBenefit{{BenefitNo: "benefit-1", Type: "cashback", CurrencyCode: "CNY", PurchasePhase: "both", CycleScope: "all", Status: "active", Version: 1}},
	}
	controller := newAdminControllerForTest(t, &controllerManagementService{}, &controllerMutationService{}, &catalogProductManagementReaderStub{}, &catalogManagementServiceStub{}, &catalogPublicationServiceStub{}, &catalogPublicationPreviewServiceStub{}, &marketingOverviewReaderStub{}, directory)
	segments, err := controller.MarketingSegmentList(context.Background(), &v1.MarketingSegmentListReq{NamePrefix: "loyal", Mode: "dynamic", Status: "active", Limit: 25})
	if err != nil || len(segments.Segments) != 1 || segments.NextCursor != "segment-next" || directory.segmentInput.NamePrefix != "loyal" {
		t.Fatalf("map segment list: result=%+v input=%+v err=%v", segments, directory.segmentInput, err)
	}
	segment, err := controller.MarketingSegmentDetail(context.Background(), &v1.MarketingSegmentDetailReq{SegmentNo: "segment-loyal"})
	if err != nil || segment.Segment.SegmentNo != "segment-loyal" || len(segment.Conditions) != 1 || len(segment.Snapshots) != 1 || directory.segmentNo != "segment-loyal" {
		t.Fatalf("map segment detail: result=%+v err=%v", segment, err)
	}
	campaigns, err := controller.MarketingCampaignList(context.Background(), &v1.MarketingCampaignListReq{Type: "cashback", Status: "active", Limit: 25})
	if err != nil || len(campaigns.Campaigns) != 1 || campaigns.NextCursor != "campaign-next" || directory.campaignInput.Type != "cashback" {
		t.Fatalf("map campaign list: result=%+v input=%+v err=%v", campaigns, directory.campaignInput, err)
	}
	campaign, err := controller.MarketingCampaignDetail(context.Background(), &v1.MarketingCampaignDetailReq{CampaignNo: "campaign-summer"})
	if err != nil || campaign.Campaign.CampaignNo != "campaign-summer" || len(campaign.Scopes) != 1 || len(campaign.Benefits) != 1 || campaign.Benefits[0].CurrencyCode != "CNY" || directory.campaignNo != "campaign-summer" {
		t.Fatalf("map campaign detail: result=%+v err=%v", campaign, err)
	}
}

func TestMarketingAssetDirectoryControllerMapsListsAndDetails(t *testing.T) {
	t.Parallel()
	now := time.Unix(1767225600, 0).UTC()
	face := int64(500)
	claimedAt := now.Add(time.Minute)
	expiresAt := now.Add(24 * time.Hour)
	assetReader := &marketingAssetDirectoryReaderStub{}
	assetReader.assetPage = &marketing.PromotionAssetPage{Assets: []marketing.PromotionAssetSummary{{
		AssetNo: "asset-1", CampaignNo: "campaign-1", BenefitNo: "benefit-1", BenefitType: "voucher", Type: "voucher",
		CustomerNo: "customer-1", CurrencyCode: "CNY", FaceAmountMinor: &face, UsageLimit: 1, Status: "active",
		ValidFrom: now, ValidUntil: expiresAt, ClaimedAt: &claimedAt, CreatedAt: now, UpdatedAt: claimedAt,
	}}, NextCursor: "asset-next"}
	assetReader.assetDetail = &marketing.PromotionAssetDetail{
		PromotionAssetSummary: assetReader.assetPage.Assets[0],
		Reservations:          []marketing.PromotionAssetReservationSummary{{ReservationNo: "reservation-1", ReservedUses: 1, DiscountAmountMinor: 500, Status: "active", ExpiresAt: expiresAt, CreatedAt: now}},
	}
	assetReader.pointsPage = &marketing.PointsAccountPage{Accounts: []marketing.PointsAccountSummary{{CustomerNo: "customer-1", AvailablePoints: 1200, Version: 2, LotCount: 1, EntryCount: 1, CreatedAt: now, UpdatedAt: now}}, NextCursor: "points-next"}
	assetReader.pointsDetail = &marketing.PointsAccountDetail{
		PointsAccountSummary: assetReader.pointsPage.Accounts[0],
		Lots:                 []marketing.PointsLotSummary{{LotNo: "lot-1", SourceType: "order", SourceReference: "order-1", OriginalPoints: 1200, AvailablePoints: 1200, Status: "available", CreatedAt: now}},
		Entries:              []marketing.PointsEntrySummary{{EntryNo: "points-entry-1", LotNo: "lot-1", Type: "earn", AvailableDelta: 1200, SourceType: "order", SourceReference: "order-1", CreatedAt: now}},
	}
	assetReader.grantPage = &marketing.ValueGrantPage{Grants: []marketing.ValueGrantSummary{{
		GrantNo: "grant-1", Type: "cashback", CampaignNo: "campaign-1", BenefitNo: "benefit-2", CustomerNo: "customer-1",
		CurrencyCode: "CNY", SourceReference: "payment-1", OriginalAmountMinor: 8800, AvailableAmountMinor: 8800,
		Status: "available", FinancialPostingReference: "ledger-1", CreatedAt: now, UpdatedAt: now,
	}}, NextCursor: "grant-next"}
	assetReader.grantDetail = &marketing.ValueGrantDetail{
		ValueGrantSummary: assetReader.grantPage.Grants[0],
		Entries:           []marketing.ValueEntrySummary{{EntryNo: "value-entry-1", Type: "post", PendingDeltaMinor: -8800, AvailableDeltaMinor: 8800, SourceType: "activity", SourceReference: "ledger-1", CreatedAt: now}},
	}
	controller, err := NewV1(
		&controllerManagementService{}, &controllerMutationService{}, &catalogProductManagementReaderStub{},
		&catalogManagementServiceStub{}, &catalogPublicationServiceStub{}, &catalogPublicationPreviewServiceStub{},
		&marketingOverviewReaderStub{}, &marketingDirectoryReaderStub{}, assetReader,
		&marketingActionServiceStub{},
		&commerceRiskReviewServiceStub{},
		managementOrderReaderStub{},
		&assistedQuoteServiceStub{},
		managementFinanceReaderStub{},
		managementFinanceCommandServiceStub{},
		managementCreditReaderStub{},
		managementCreditCommandServiceStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := controller.MarketingAssetList(context.Background(), &v1.MarketingAssetListReq{CustomerNo: "customer-1", Limit: 25})
	if err != nil || len(assets.Assets) != 1 || assets.Assets[0].FaceAmountMinor == nil || *assets.Assets[0].FaceAmountMinor != 500 || assets.NextCursor != "asset-next" || assetReader.assetInput.CustomerNo != "customer-1" {
		t.Fatalf("map asset list: result=%+v input=%+v err=%v", assets, assetReader.assetInput, err)
	}
	asset, err := controller.MarketingAssetDetail(context.Background(), &v1.MarketingAssetDetailReq{AssetNo: "asset-1"})
	if err != nil || asset.Asset.ClaimedAt == nil || len(asset.Reservations) != 1 || assetReader.assetNo != "asset-1" {
		t.Fatalf("map asset detail: result=%+v err=%v", asset, err)
	}
	points, err := controller.MarketingPointsAccountDetail(context.Background(), &v1.MarketingPointsAccountDetailReq{CustomerNo: "customer-1"})
	if err != nil || points.Account.AvailablePoints != 1200 || len(points.Lots) != 1 || len(points.Entries) != 1 || assetReader.customerNo != "customer-1" {
		t.Fatalf("map points detail: result=%+v err=%v", points, err)
	}
	grants, err := controller.MarketingValueGrantList(context.Background(), &v1.MarketingValueGrantListReq{Currency: "CNY", Limit: 25})
	if err != nil || len(grants.Grants) != 1 || grants.Grants[0].CurrencyCode != "CNY" || grants.NextCursor != "grant-next" || assetReader.grantInput.Currency != "CNY" {
		t.Fatalf("map grant list: result=%+v input=%+v err=%v", grants, assetReader.grantInput, err)
	}
	grant, err := controller.MarketingValueGrantDetail(context.Background(), &v1.MarketingValueGrantDetailReq{GrantNo: "grant-1"})
	if err != nil || grant.Grant.FinancialPostingReference != "ledger-1" || len(grant.Entries) != 1 || assetReader.grantNo != "grant-1" {
		t.Fatalf("map grant detail: result=%+v err=%v", grant, err)
	}
}

type catalogProductManagementReaderStub struct {
	categories []catalog.CategoryManagementSummary
	input      catalog.ProductManagementSearchInput
	detailNo   string
	detail     *catalog.ProductManagementDetail
	page       *catalog.ProductManagementPage
}

func (s *catalogProductManagementReaderStub) PreviewProductPauseImpact(
	_ context.Context, _ string,
) (*catalog.ProductPauseImpactPreview, error) {
	return nil, nil
}

type catalogManagementServiceStub struct {
	categoryInput       catalog.CategoryCreateInput
	category            *catalog.Category
	categoryUpdateInput catalog.CategoryUpdateInput
	categoryStateInput  catalog.CategoryStateInput
	productInput        catalog.ProductCreateInput
	product             *catalog.ProductDraft
	billingRuleInput    catalog.BillingRuleCreateInput
	billingRule         *catalog.BillingRule
	priceScheduleInput  catalog.PriceScheduleCreateInput
	priceSchedule       *catalog.PriceSchedule
	taxPolicyInput      catalog.TaxPolicyCreateInput
	taxPolicy           *catalog.TaxPolicy
}

func (s *catalogManagementServiceStub) CreateCategory(
	_ context.Context,
	input catalog.CategoryCreateInput,
) (*catalog.Category, error) {
	s.categoryInput = input
	return s.category, nil
}

func (s *catalogManagementServiceStub) UpdateCategory(
	_ context.Context,
	input catalog.CategoryUpdateInput,
) (*catalog.Category, error) {
	s.categoryUpdateInput = input
	return s.category, nil
}

func (s *catalogManagementServiceStub) DeleteCategory(
	_ context.Context,
	input catalog.CategoryStateInput,
) error {
	s.categoryStateInput = input
	return nil
}

func (s *catalogManagementServiceStub) RestoreCategory(
	_ context.Context,
	input catalog.CategoryStateInput,
) (*catalog.Category, error) {
	s.categoryStateInput = input
	return s.category, nil
}

func (s *catalogManagementServiceStub) CreateProductDraft(
	_ context.Context,
	input catalog.ProductCreateInput,
) (*catalog.ProductDraft, error) {
	s.productInput = input
	return s.product, nil
}

func (s *catalogManagementServiceStub) CreateBillingRule(
	_ context.Context,
	input catalog.BillingRuleCreateInput,
) (*catalog.BillingRule, error) {
	s.billingRuleInput = input
	return s.billingRule, nil
}

func (s *catalogManagementServiceStub) CreatePriceSchedule(
	_ context.Context,
	input catalog.PriceScheduleCreateInput,
) (*catalog.PriceSchedule, error) {
	s.priceScheduleInput = input
	return s.priceSchedule, nil
}

func (s *catalogManagementServiceStub) CreateTaxPolicy(
	_ context.Context,
	input catalog.TaxPolicyCreateInput,
) (*catalog.TaxPolicy, error) {
	s.taxPolicyInput = input
	return s.taxPolicy, nil
}

type catalogPublicationServiceStub struct {
	result    *catalog.PublicationResult
	versionNo string
}

type catalogPublicationPreviewServiceStub struct {
	preview   *catalog.PublicationPreview
	versionNo string
}

func (s *catalogPublicationPreviewServiceStub) PreviewVersion(
	_ context.Context,
	versionNo string,
) (*catalog.PublicationPreview, error) {
	s.versionNo = versionNo
	return s.preview, nil
}

func (s *catalogPublicationServiceStub) PublishVersion(
	_ context.Context,
	versionNo string,
) (*catalog.PublicationResult, error) {
	s.versionNo = versionNo
	return s.result, nil
}

func (s *catalogProductManagementReaderStub) SearchProducts(
	_ context.Context,
	input catalog.ProductManagementSearchInput,
) (*catalog.ProductManagementPage, error) {
	s.input = input
	if s.page == nil {
		return &catalog.ProductManagementPage{Products: []catalog.ProductManagementSummary{}}, nil
	}
	return s.page, nil
}

func (s *catalogProductManagementReaderStub) GetProductDetail(
	_ context.Context,
	productNo string,
) (*catalog.ProductManagementDetail, error) {
	s.detailNo = productNo
	return s.detail, nil
}

func (s *catalogProductManagementReaderStub) ListCategories(
	_ context.Context,
	_ string,
) ([]catalog.CategoryManagementSummary, error) {
	return s.categories, nil
}

func (s *controllerMutationService) BatchUpdateTags(
	_ context.Context,
	input customermanagement.TagBatchInput,
) (*customermanagement.TagBatchResult, error) {
	s.input = input
	return s.result, nil
}

func (*controllerMutationService) AddNote(context.Context, customermanagement.NoteInput) (*customermanagement.NoteResult, error) {
	return &customermanagement.NoteResult{}, nil
}

func (*controllerMutationService) PlanFollowUp(context.Context, customermanagement.FollowUpPlanInput) (*customermanagement.FollowUpResult, error) {
	return &customermanagement.FollowUpResult{}, nil
}

func (*controllerMutationService) TransitionFollowUp(context.Context, customermanagement.FollowUpTransitionInput) (*customermanagement.FollowUpResult, error) {
	return &customermanagement.FollowUpResult{}, nil
}

func (*controllerMutationService) CreateBlacklist(context.Context, customermanagement.BlacklistCreateInput) (*customermanagement.BlacklistResult, error) {
	return &customermanagement.BlacklistResult{}, nil
}

func (*controllerMutationService) DecideBlacklist(context.Context, customermanagement.BlacklistDecisionInput) (*customermanagement.BlacklistResult, error) {
	return &customermanagement.BlacklistResult{}, nil
}

func (s *controllerManagementService) Search(_ context.Context, input customermanagement.SearchInput) (*customermanagement.Page, error) {
	s.search = input
	return s.page, nil
}

func (s *controllerManagementService) GetOverview(_ context.Context, customerNo string) (*customermanagement.Overview, error) {
	s.getNo = customerNo
	return s.overview, nil
}

func (s *controllerManagementService) SearchSegments(
	_ context.Context,
	input customermanagement.SegmentSearchInput,
) (*customermanagement.SegmentPage, error) {
	s.segmentSearch = input
	return &customermanagement.SegmentPage{Items: []customermanagement.SegmentSummary{{
		Number: "TAG_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Code: "priority",
		NameZhCn: "优先客户", NameEnUs: "Priority", ColorToken: "warning", Status: "active",
	}}, NextCursor: "segment-next"}, nil
}

func (s *controllerManagementService) SearchAssignments(
	_ context.Context,
	input customermanagement.AssignmentSearchInput,
) (*customermanagement.AssignmentPage, error) {
	s.assignmentSearch = input
	return &customermanagement.AssignmentPage{Items: []customermanagement.SegmentSummary{{
		Number: "TAG_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Code: "priority",
		NameZhCn: "优先客户", NameEnUs: "Priority", ColorToken: "warning", Status: "active",
	}}, NextCursor: "assignment-next"}, nil
}

func (*controllerManagementService) SearchActivities(
	context.Context,
	customermanagement.ActivitySearchInput,
) (*customermanagement.ActivityPage, error) {
	return &customermanagement.ActivityPage{Items: []customermanagement.ActivityItem{}}, nil
}

func TestCustomerControllersMapOpaqueCursorAndMillisecondTimes(t *testing.T) {
	created := time.Date(2026, 8, 8, 12, 0, 0, 123000000, time.UTC)
	updated := created.Add(time.Minute)
	summary := customermanagement.Summary{
		CustomerNo: "CUS_0123456789abcdef0123456789abcdef",
		Type:       customer.TypeIndividual, Status: customer.StatusActive,
		Verification: customer.VerificationVerified, DisplayName: "Example",
		CountryCode: "CN", PreferredLocale: "zh-CN", TagCount: 2, Version: 3,
		CreatedAt: created, UpdatedAt: updated,
	}
	service := &controllerManagementService{
		page: &customermanagement.Page{Customers: []customermanagement.Summary{summary}, NextCursor: "opaque-next"},
		overview: &customermanagement.Overview{
			Customer: summary,
			Groups:   customermanagement.SegmentPreview{Items: []customermanagement.SegmentSummary{}},
			Tags:     customermanagement.SegmentPreview{Items: []customermanagement.SegmentSummary{}},
		},
	}
	mutation := &controllerMutationService{result: &customermanagement.TagBatchResult{
		AuditNo: "AUD_0123456789abcdef0123456789abcdef", CustomerCount: 1, TagCount: 1,
	}}
	controller := newAdminControllerForTest(t,
		service, mutation, &catalogProductManagementReaderStub{}, &catalogManagementServiceStub{},
		&catalogPublicationServiceStub{},
		&catalogPublicationPreviewServiceStub{},
		nil,
	)
	list, err := controller.CustomerList(context.Background(), &v1.CustomerListReq{
		NamePrefix: "example", Type: "individual", Status: "active",
		GroupNo: "GRP_11111111111111111111111111111111",
		TagNo:   "TAG_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Cursor:  "opaque-in", Limit: 25,
	})
	if err != nil || len(list.Customers) != 1 || list.NextCursor != "opaque-next" {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	if list.Customers[0].CreatedAt != created.UnixMilli() || list.Customers[0].UpdatedAt != updated.UnixMilli() {
		t.Fatalf("API times are not Unix milliseconds: %+v", list.Customers[0])
	}
	if service.search.Cursor != "opaque-in" || service.search.Limit != 25 ||
		service.search.GroupNo != "GRP_11111111111111111111111111111111" ||
		service.search.TagNo != "TAG_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("search input=%+v", service.search)
	}
	segments, err := controller.CustomerSegmentList(context.Background(), &v1.CustomerSegmentListReq{
		Kind: "tag", Locale: "en-US", NamePrefix: "Pri", Cursor: "segment-in", Limit: 20,
	})
	if err != nil || len(segments.Items) != 1 || segments.NextCursor != "segment-next" ||
		segments.Items[0].Number != "TAG_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
		service.segmentSearch.Kind != customermanagement.SegmentKindTag ||
		service.segmentSearch.Locale != customermanagement.SegmentLocaleEnUS ||
		service.segmentSearch.Cursor != "segment-in" {
		t.Fatalf("segments=%+v search=%+v err=%v", segments, service.segmentSearch, err)
	}
	overview, err := controller.CustomerOverviewGet(context.Background(), &v1.CustomerOverviewGetReq{CustomerNo: summary.CustomerNo})
	if err != nil || overview.Customer.CustomerNo != summary.CustomerNo || service.getNo != summary.CustomerNo {
		t.Fatalf("overview=%+v getNo=%q err=%v", overview, service.getNo, err)
	}
	batch, err := controller.CustomerTagBatch(context.Background(), &v1.CustomerTagBatchReq{
		Action:      "assign",
		CustomerNos: []string{"CUS_0123456789abcdef0123456789abcdef"},
		TagNos:      []string{"TAG_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		Reason:      "Quarterly account classification", IdempotencyKey: "batch-request-0001",
	})
	if err != nil || batch.AuditNo != mutation.result.AuditNo || batch.CustomerCount != 1 ||
		mutation.input.Action != customermanagement.TagBatchAssign || len(mutation.input.CustomerNos) != 1 {
		t.Fatalf("batch=%+v input=%+v err=%v", batch, mutation.input, err)
	}
}

func TestCatalogProductControllerMapsWorkbenchProjection(t *testing.T) {
	created := time.Date(2026, 8, 9, 9, 0, 0, 123000000, time.UTC)
	published := created.Add(time.Minute)
	reader := &catalogProductManagementReaderStub{page: &catalog.ProductManagementPage{
		Products: []catalog.ProductManagementSummary{{
			ProductNo: "product-basic", CategoryNo: "category-services",
			CategoryNameZhCN: "数字服务", CategoryNameEnUS: "Digital services",
			VersionNo: "version-basic-1", VersionNumber: 1,
			Status: "active", VersionStatus: "published", Source: "manual",
			PublicSlug: "managed-ssl", NameZhCN: "托管 SSL 证书", NameEnUS: "Managed SSL",
			BillingRuleCount: 2, CurrencyCount: 2, PublishedAt: &published,
			CreatedAt: created, UpdatedAt: published,
		}}, NextCursor: "catalog-next",
	}}
	controller := newAdminControllerForTest(t,
		&controllerManagementService{}, &controllerMutationService{}, reader,
		&catalogManagementServiceStub{},
		&catalogPublicationServiceStub{}, &catalogPublicationPreviewServiceStub{},
		nil,
	)
	result, err := controller.CatalogProductList(context.Background(), &v1.CatalogProductListReq{
		NamePrefix: "ssl", CategoryNo: "category-services", Status: "active", Source: "manual",
		Cursor: "catalog-in", Limit: 25,
	})
	if err != nil || len(result.Products) != 1 || result.NextCursor != "catalog-next" ||
		result.Products[0].PublishedAt == nil || *result.Products[0].PublishedAt != published.UnixMilli() {
		t.Fatalf("catalog product list=%+v err=%v", result, err)
	}
	if reader.input.Cursor != "catalog-in" || reader.input.Limit != 25 ||
		reader.input.CategoryNo != "category-services" || reader.input.Source != "manual" {
		t.Fatalf("catalog product input=%+v", reader.input)
	}
}

func TestCatalogProductControllerMapsLatestVersionDetail(t *testing.T) {
	created := time.Date(2026, 8, 9, 10, 0, 0, 123000000, time.UTC)
	renewalAmount := int64(10_900)
	reader := &catalogProductManagementReaderStub{detail: &catalog.ProductManagementDetail{
		Product: catalog.ProductManagementDetailProduct{
			ProductNo: "product-route", CategoryNo: "category-route",
			Status: "active", PublicSlug: "managed-ssl",
			NameZhCN: "托管 SSL 证书", NameEnUS: "Managed SSL", CreatedAt: created, UpdatedAt: created,
		},
		Version: catalog.ProductManagementDetailVersion{
			VersionNo: "version-route-1", VersionNumber: 1, Status: "published", Source: "manual",
			PublishedAt: &created, CreatedAt: created,
		},
		BillingRules: []catalog.ProductManagementDetailBillingRule{{
			BillingRuleNo: "billing-route-monthly", RuleCode: "monthly", ChargeType: "recurring",
			PeriodUnit: "month", PeriodCount: 1, PricePhase: "both", QuantityMode: "fixed",
			MinimumPurchaseCycles: 1, Status: "active",
		}},
		Prices: []catalog.ProductManagementDetailPrice{{
			PriceNo: "price-route-base", BillingRuleNo: "billing-route-monthly", CurrencyCode: "CNY",
			AmountMinor: 9900, RenewalAmountMinor: &renewalAmount, Status: "active",
		}},
		Inventory: catalog.ProductVersionInventory{TrackingEnabled: true, SellableQuantity: 7, Version: 1},
	}}
	controller := newAdminControllerForTest(t,
		&controllerManagementService{}, &controllerMutationService{}, reader,
		&catalogManagementServiceStub{},
		&catalogPublicationServiceStub{}, &catalogPublicationPreviewServiceStub{},
		nil,
	)
	result, err := controller.CatalogProductDetail(context.Background(), &v1.CatalogProductDetailReq{
		ProductNo: "product-route",
	})
	if err != nil || result == nil || result.Product.ProductNo != "product-route" ||
		result.Version.PublishedAt == nil || *result.Version.PublishedAt != created.UnixMilli() ||
		result.Inventory == nil || result.Inventory.SellableQuantity != 7 || len(result.Prices) != 1 {
		t.Fatalf("catalog product detail=%+v err=%v", result, err)
	}
	if reader.detailNo != "product-route" {
		t.Fatalf("catalog product detail request=%q", reader.detailNo)
	}
}

func TestCatalogProductPublishControllerMapsCommittedTransition(t *testing.T) {
	publishedAt := time.Date(2026, 8, 9, 16, 30, 0, 123000000, time.UTC)
	publication := &catalogPublicationServiceStub{result: &catalog.PublicationResult{
		ProductNo: "product-route", VersionNo: "version-route-2", Status: "published",
		SupersededVersionNo: "version-route-1", PublishedAt: publishedAt,
	}}
	controller := newAdminControllerForTest(t,
		&controllerManagementService{}, &controllerMutationService{},
		&catalogProductManagementReaderStub{}, &catalogManagementServiceStub{}, publication,
		&catalogPublicationPreviewServiceStub{},
		nil,
	)
	result, err := controller.CatalogProductPublish(
		context.Background(),
		&v1.CatalogProductPublishReq{VersionNo: "version-route-2"},
	)
	if err != nil || result == nil || result.ProductNo != "product-route" ||
		result.VersionNo != "version-route-2" || result.Status != "published" ||
		result.SupersededVersionNo != "version-route-1" ||
		result.PublishedAt != publishedAt.UnixMilli() ||
		publication.versionNo != "version-route-2" {
		t.Fatalf("catalog product publication=%+v request=%q err=%v", result, publication.versionNo, err)
	}
}

func TestCatalogProductPublicationPreviewControllerMapsStableChecks(t *testing.T) {
	preview := &catalogPublicationPreviewServiceStub{preview: &catalog.PublicationPreview{
		ProductNo: "product-route", VersionNo: "version-route-2", Ready: false,
		Checks: []catalog.PublicationReadinessCheck{
			{Code: catalog.PublicationCheckLifecycle, Passed: true},
			{Code: catalog.PublicationCheckPricing, Passed: false},
		},
	}}
	controller := newAdminControllerForTest(t,
		&controllerManagementService{}, &controllerMutationService{},
		&catalogProductManagementReaderStub{}, &catalogManagementServiceStub{},
		&catalogPublicationServiceStub{}, preview,
		nil,
	)
	result, err := controller.CatalogProductPublicationPreview(
		context.Background(),
		&v1.CatalogProductPublicationPreviewReq{VersionNo: "version-route-2"},
	)
	if err != nil || result == nil || result.Ready || result.ProductNo != "product-route" ||
		len(result.Checks) != 2 || result.Checks[0].Code != "lifecycle" ||
		!result.Checks[0].Passed || result.Checks[1].Code != "pricing" ||
		result.Checks[1].Passed || preview.versionNo != "version-route-2" {
		t.Fatalf("publication preview=%+v request=%q err=%v", result, preview.versionNo, err)
	}
}

func TestMarketingOverviewControllerMapsCurrencySafeSnapshot(t *testing.T) {
	generatedAt := time.Date(2026, 8, 10, 6, 30, 0, 123000000, time.UTC)
	expiresAt := generatedAt.Add(24 * time.Hour)
	reader := &marketingOverviewReaderStub{overview: &marketing.ManagementOverview{
		GeneratedAt: generatedAt,
		Summary: marketing.MarketingOverviewSummary{
			SegmentTotal: 3, SegmentActive: 2, CampaignTotal: 4, CampaignActive: 1,
			PointsAvailable: 900, ConsentOptedIn: 7,
		},
		Currencies: []marketing.MarketingCurrencySummary{{
			CurrencyCode: "CNY", GrantCount: 2, AvailableMinor: 8800, ConsumedMinor: 1200,
		}},
		RecentCampaigns: []marketing.MarketingCampaignSummary{{
			CampaignNo: "campaign-controller", NameZhCN: "首充赠送", NameEnUS: "First recharge",
			Type: "recharge_bonus", Status: "active", StartsAt: generatedAt.Add(-time.Hour),
			EndsAt: generatedAt.Add(time.Hour), UpdatedAt: generatedAt,
		}},
		RecentGrants: []marketing.MarketingGrantSummary{{
			GrantNo: "grant-controller", GrantType: "recharge_bonus", CampaignNo: "campaign-controller",
			CustomerNo: "customer-controller", CurrencyCode: "CNY", OriginalMinor: 10000,
			AvailableMinor: 8800, ConsumedMinor: 1200, Status: "available",
			ExpiresAt: &expiresAt, CreatedAt: generatedAt.Add(-time.Minute), UpdatedAt: generatedAt,
		}},
	}}
	controller := newAdminControllerForTest(t,
		&controllerManagementService{}, &controllerMutationService{},
		&catalogProductManagementReaderStub{}, &catalogManagementServiceStub{},
		&catalogPublicationServiceStub{}, &catalogPublicationPreviewServiceStub{}, reader,
	)
	result, err := controller.MarketingOverviewGet(context.Background(), &v1.MarketingOverviewGetReq{})
	if err != nil || result == nil || reader.calls != 1 || result.GeneratedAt != generatedAt.UnixMilli() ||
		result.Summary.SegmentTotal != 3 || result.Summary.PointsAvailable != 900 ||
		len(result.Currencies) != 1 || result.Currencies[0].AvailableMinor != 8800 ||
		len(result.RecentCampaigns) != 1 || result.RecentCampaigns[0].NameEnUs != "First recharge" ||
		len(result.RecentGrants) != 1 || result.RecentGrants[0].ExpiresAt == nil ||
		*result.RecentGrants[0].ExpiresAt != expiresAt.UnixMilli() {
		t.Fatalf("marketing overview mapping=%+v reader=%+v err=%v", result, reader, err)
	}
}
