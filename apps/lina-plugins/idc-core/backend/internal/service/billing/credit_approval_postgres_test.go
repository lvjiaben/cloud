package billing

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCreditApprovalServicePersistsSequentialDualReviewAndActivatesAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	service, err := NewCreditApprovalService(database)
	if err != nil {
		t.Fatalf("create credit approval service: %v", err)
	}

	policyInput := CreditPolicyInput{
		CurrencyCode: "CNY", MaximumLimitMinor: 50_000,
		DualReviewThresholdMinor: 5_000, BillingDay: 15,
		RepaymentPeriodDays: 10, GracePeriodDays: 3,
		DailyLateFeeBasisPoints: 10, MaximumLateFeeMinor: 5_000,
		ConfiguredBy: 101, IdempotencyKey: "credit-policy-cny-0001",
	}
	policy, err := service.ConfigureCreditPolicy(ctx, policyInput)
	if err != nil {
		t.Fatalf("configure credit policy: %v", err)
	}
	if policy.Version != 1 || policy.Reused {
		t.Fatalf("unexpected initial policy: %+v", policy)
	}
	replayedPolicy, err := service.ConfigureCreditPolicy(ctx, policyInput)
	if err != nil || !replayedPolicy.Reused || replayedPolicy.PolicyNo != policy.PolicyNo {
		t.Fatalf("policy replay=%+v err=%v", replayedPolicy, err)
	}
	changedPolicy := policyInput
	changedPolicy.MaximumLimitMinor++
	if _, err = service.ConfigureCreditPolicy(ctx, changedPolicy); err == nil {
		t.Fatal("policy idempotency key accepted changed content")
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	applicationInput := CreditApplicationInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY",
		RequestedLimitMinor: 6_000, RequestedValidFrom: now.Add(-time.Minute),
		RequestedValidUntil:   now.AddDate(1, 0, 0),
		RiskEvidenceReference: "private:risk/customer-ledger/cny",
		RiskEvidenceDigest:    strings.Repeat("1", 64),
		RequestActorType:      "staff", RequestedBy: 150,
		IdempotencyKey: "credit-application-cny-0001",
	}
	application, err := service.SubmitCreditApplication(ctx, applicationInput)
	if err != nil {
		t.Fatalf("submit credit application: %v", err)
	}
	if application.StatusCode != "pending_review" || application.RequiredApprovalCount != 2 ||
		application.ReviewCount != 0 || application.CreditAccountNo != "" {
		t.Fatalf("unexpected submitted application: %+v", application)
	}
	replayedApplication, err := service.SubmitCreditApplication(ctx, applicationInput)
	if err != nil || !replayedApplication.Reused || replayedApplication.ApplicationNo != application.ApplicationNo {
		t.Fatalf("application replay=%+v err=%v", replayedApplication, err)
	}
	changedApplication := applicationInput
	changedApplication.RequestedLimitMinor--
	if _, err = service.SubmitCreditApplication(ctx, changedApplication); err == nil {
		t.Fatal("application idempotency key accepted changed content")
	}

	selfReview := CreditReviewInput{
		ApplicationNo: application.ApplicationNo, Decision: CreditDecisionApprove,
		ReviewerID: 150, CommentDigest: strings.Repeat("2", 64),
		EvidenceDigest: strings.Repeat("3", 64),
	}
	if _, err = service.ReviewCreditApplication(ctx, selfReview); err == nil {
		t.Fatal("staff applicant reviewed their own credit application")
	}
	firstReview := CreditReviewInput{
		ApplicationNo: application.ApplicationNo, Decision: CreditDecisionApprove,
		ReviewerID: 201, CommentDigest: strings.Repeat("4", 64),
		EvidenceDigest: strings.Repeat("5", 64),
	}
	afterFirst, err := service.ReviewCreditApplication(ctx, firstReview)
	if err != nil {
		t.Fatalf("persist first independent review: %v", err)
	}
	if afterFirst.StatusCode != "pending_review" || afterFirst.ReviewCount != 1 ||
		afterFirst.CreditAccountNo != "" {
		t.Fatalf("first review completed application too early: %+v", afterFirst)
	}
	accountCount, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_core_credit_account
WHERE tenant_id = 0 AND origin_application_no = ?`, application.ApplicationNo)
	if err != nil || accountCount.Int64() != 0 {
		t.Fatalf("first review created account count=%d err=%v", accountCount.Int64(), err)
	}

	secondReview := CreditReviewInput{
		ApplicationNo: application.ApplicationNo, Decision: CreditDecisionApprove,
		ReviewerID: 202, CommentDigest: strings.Repeat("6", 64),
		EvidenceDigest: strings.Repeat("7", 64),
	}
	approved, err := service.ReviewCreditApplication(ctx, secondReview)
	if err != nil {
		t.Fatalf("complete second independent review: %v", err)
	}
	if approved.StatusCode != "approved" || approved.ReviewCount != 2 || approved.CreditAccountNo == "" {
		t.Fatalf("unexpected approved application: %+v", approved)
	}
	replayedReview, err := service.ReviewCreditApplication(ctx, secondReview)
	if err != nil || !replayedReview.Reused || replayedReview.CreditAccountNo != approved.CreditAccountNo {
		t.Fatalf("review replay=%+v err=%v", replayedReview, err)
	}
	changedReview := secondReview
	changedReview.EvidenceDigest = strings.Repeat("8", 64)
	if _, err = service.ReviewCreditApplication(ctx, changedReview); err == nil {
		t.Fatal("review replay accepted changed evidence")
	}

	var account struct {
		StatusCode          string `orm:"status_code"`
		TotalLimitMinor     int64  `orm:"total_limit_minor"`
		AvailableLimitMinor int64  `orm:"available_limit_minor"`
		Version             int64  `orm:"version"`
		MovementCount       int64  `orm:"movement_count"`
	}
	if err = database.GetScan(ctx, &account, `
SELECT account.status_code, account.total_limit_minor,
       account.available_limit_minor, account.version,
       (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement movement
        WHERE movement.credit_account_no = account.credit_account_no) AS movement_count
FROM plugin_idc_core_credit_account account
WHERE account.tenant_id = 0 AND account.credit_account_no = ?`, approved.CreditAccountNo); err != nil {
		t.Fatalf("load activated credit account: %v", err)
	}
	if account.StatusCode != "active" || account.TotalLimitMinor != 6_000 ||
		account.AvailableLimitMinor != 6_000 || account.Version != 1 || account.MovementCount != 1 {
		t.Fatalf("unexpected activated account projection: %+v", account)
	}
}

func TestCreditApprovalServiceAllowsSecondReviewerToReject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	service, err := NewCreditApprovalService(database)
	if err != nil {
		t.Fatalf("create credit approval service: %v", err)
	}
	_, err = service.ConfigureCreditPolicy(ctx, CreditPolicyInput{
		CurrencyCode: "CNY", MaximumLimitMinor: 20_000,
		DualReviewThresholdMinor: 5_000, BillingDay: 10,
		RepaymentPeriodDays: 7, GracePeriodDays: 1,
		ConfiguredBy: 101, IdempotencyKey: "credit-policy-rejection-0001",
	})
	if err != nil {
		t.Fatalf("configure rejection policy: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	application, err := service.SubmitCreditApplication(ctx, CreditApplicationInput{
		CustomerNo: "customer-ledger-other", CurrencyCode: "CNY",
		RequestedLimitMinor: 8_000, RequestedValidFrom: now,
		RequestedValidUntil:   now.AddDate(1, 0, 0),
		RiskEvidenceReference: "private:risk/customer-ledger-other/cny",
		RiskEvidenceDigest:    strings.Repeat("9", 64),
		RequestActorType:      "customer", RequestedBy: 301,
		IdempotencyKey: "credit-application-rejection-0001",
	})
	if err != nil {
		t.Fatalf("submit rejection application: %v", err)
	}
	_, err = service.ReviewCreditApplication(ctx, CreditReviewInput{
		ApplicationNo: application.ApplicationNo, Decision: CreditDecisionApprove,
		ReviewerID: 401, CommentDigest: strings.Repeat("a", 64),
		EvidenceDigest: strings.Repeat("b", 64),
	})
	if err != nil {
		t.Fatalf("persist first approval before rejection: %v", err)
	}
	rejected, err := service.ReviewCreditApplication(ctx, CreditReviewInput{
		ApplicationNo: application.ApplicationNo, Decision: CreditDecisionReject,
		ReviewerID: 402, CommentDigest: strings.Repeat("c", 64),
		EvidenceDigest: strings.Repeat("d", 64),
	})
	if err != nil {
		t.Fatalf("persist second-person rejection: %v", err)
	}
	if rejected.StatusCode != "rejected" || rejected.ReviewCount != 2 || rejected.CreditAccountNo != "" {
		t.Fatalf("unexpected rejected application: %+v", rejected)
	}
}
