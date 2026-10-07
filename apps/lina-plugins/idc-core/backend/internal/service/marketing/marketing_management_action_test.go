package marketing

import (
	"context"
	"errors"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type managementActionAuthorizerStub struct {
	err    error
	calls  []string
	userID int
}

func (s *managementActionAuthorizerStub) authorize(action string) (int, error) {
	s.calls = append(s.calls, action)
	if s.err != nil {
		return 0, s.err
	}
	if s.userID <= 0 {
		return 7, nil
	}
	return s.userID, nil
}

func (s *managementActionAuthorizerStub) AuthorizeAssetIssue(context.Context) (int, error) {
	return s.authorize("asset_issue")
}
func (s *managementActionAuthorizerStub) AuthorizeCouponCodeGenerate(context.Context) (int, error) {
	return s.authorize("coupon_code_generate")
}
func (s *managementActionAuthorizerStub) AuthorizeAssetRevoke(context.Context) (int, error) {
	return s.authorize("asset_revoke")
}
func (s *managementActionAuthorizerStub) AuthorizePointsCredit(context.Context) (int, error) {
	return s.authorize("points_credit")
}
func (s *managementActionAuthorizerStub) AuthorizePointsReverse(context.Context) (int, error) {
	return s.authorize("points_reverse")
}
func (s *managementActionAuthorizerStub) AuthorizeGrantPost(context.Context) (int, error) {
	return s.authorize("grant_post")
}
func (s *managementActionAuthorizerStub) AuthorizeGrantReverse(context.Context) (int, error) {
	return s.authorize("grant_reverse")
}
func (s *managementActionAuthorizerStub) AuthorizeGrantReview(context.Context) (int, error) {
	return s.authorize("grant_review")
}

type managementActionAuditStub struct{}

func (*managementActionAuditStub) AppendBusinessTransition(context.Context, idccap.BusinessAuditTransition) error {
	return nil
}
func (*managementActionAuditStub) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func TestManagementActionsAuthorizeBeforeParsingUntrustedInput(t *testing.T) {
	t.Parallel()
	denied := errors.New("denied")
	authorizer := &managementActionAuthorizerStub{err: denied}
	service := &managementActionServiceImpl{authorizer: authorizer}
	cases := []struct {
		name string
		call func() error
	}{
		{name: "asset issue", call: func() error { _, err := service.IssueAsset(context.Background(), AssetIssueInput{}); return err }},
		{name: "asset revoke", call: func() error { _, err := service.RevokeAsset(context.Background(), AssetRevokeInput{}); return err }},
		{name: "points credit", call: func() error { _, err := service.CreditPoints(context.Background(), PointsCreditInput{}); return err }},
		{name: "points reverse", call: func() error {
			_, err := service.ReversePointsAward(context.Background(), PointsReverseManagementInput{})
			return err
		}},
		{name: "grant post", call: func() error {
			_, err := service.RequestValueGrantPosting(context.Background(), ValueGrantFinancialRequestInput{})
			return err
		}},
		{name: "grant reverse", call: func() error {
			_, err := service.RequestValueGrantReversal(context.Background(), ValueGrantFinancialRequestInput{})
			return err
		}},
		{name: "grant review", call: func() error {
			_, err := service.ReviewValueGrantAdjustment(context.Background(), ValueGrantFinancialReviewInput{})
			return err
		}},
		{name: "grant review list", call: func() error {
			_, err := service.ListValueGrantAdjustments(context.Background(), ValueGrantFinancialAdjustmentSearchInput{})
			return err
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, denied) {
				t.Fatalf("authorization error must be returned before validation, got %v", err)
			}
		})
	}
	if len(authorizer.calls) != len(cases) {
		t.Fatalf("expected one authorization call per action, got %v", authorizer.calls)
	}
}

func TestManagementActionNormalizationIsBounded(t *testing.T) {
	t.Parallel()
	if _, err := normalizeAssetIssue(AssetIssueInput{
		RequestNo: " request-001 ", CampaignNo: "campaign-001", BenefitNo: "benefit-001",
		CustomerNo: "customer-001", ReasonCode: "customer_service",
	}); err != nil {
		t.Fatalf("normalize valid asset issuance: %v", err)
	}
	if _, err := normalizeAssetIssue(AssetIssueInput{RequestNo: "request-001", CampaignNo: "campaign-001", BenefitNo: "benefit-001", CustomerNo: "customer-001", ReasonCode: "free text"}); !bizerr.Is(err, CodeManagementActionInvalid) {
		t.Fatalf("free-form reason must be rejected, got %v", err)
	}
	if _, err := normalizePointsCredit(PointsCreditInput{RequestNo: "request-001", CustomerNo: "customer-001", Points: maximumManagedPoints + 1, ReasonCode: "manual_correction"}); !bizerr.Is(err, CodeManagementActionInvalid) {
		t.Fatalf("unbounded points credit must be rejected, got %v", err)
	}
	if _, err := normalizePointsReverseManagement(PointsReverseManagementInput{RequestNo: "request-002", CustomerNo: "customer-001", AwardSourceType: "unknown", AwardSourceReference: "award-001", Points: 1, ReasonCode: "refund_correction"}); !bizerr.Is(err, CodeManagementActionInvalid) {
		t.Fatalf("unknown award source must be rejected, got %v", err)
	}
	if _, err := normalizeAssetRevoke(AssetRevokeInput{RequestNo: "request-003", AssetNo: "asset-001", ExpectedUpdatedAt: time.Time{}, ReasonCode: "risk_control"}); !bizerr.Is(err, CodeManagementActionInvalid) {
		t.Fatalf("missing optimistic concurrency timestamp must be rejected, got %v", err)
	}
	if deterministicManagedAssetNo("request-001") != deterministicManagedAssetNo("request-001") ||
		deterministicManagedAssetNo("request-001") == deterministicManagedAssetNo("request-002") {
		t.Fatal("managed asset identity must be stable and request-specific")
	}
}

func TestManagementActionConstructorFailsClosed(t *testing.T) {
	t.Parallel()
	authorizer := &managementActionAuthorizerStub{}
	audit := &managementActionAuditStub{}
	if _, err := NewManagementActionService(nil, audit, authorizer, nil, nil, nil); err == nil {
		t.Fatal("nil database must be rejected")
	}
	if _, err := NewManagementActionService(nil, nil, authorizer, nil, nil, nil); err == nil {
		t.Fatal("nil audit writer must be rejected")
	}
	var typedNilAudit *managementActionAuditStub
	if _, err := NewManagementActionService(nil, typedNilAudit, authorizer, nil, nil, nil); err == nil {
		t.Fatal("typed nil audit writer must be rejected")
	}
	var typedNilAuthorizer *managementActionAuthorizerStub
	if _, err := NewManagementActionService(nil, audit, typedNilAuthorizer, nil, nil, nil); err == nil {
		t.Fatal("typed nil authorizer must be rejected")
	}
}
