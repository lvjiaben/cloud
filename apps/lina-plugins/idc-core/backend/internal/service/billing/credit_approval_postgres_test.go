package billing

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCreditApprovalServiceAllowsAuthorizedApplicantAndActivatesAccount(t *testing.T) {
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
	if application.StatusCode != "pending_review" || application.RequiredApprovalCount != 1 ||
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
	approved, err := service.ReviewCreditApplication(ctx, selfReview)
	if err != nil || approved.StatusCode != "approved" || approved.ReviewCount != 1 || approved.CreditAccountNo == "" {
		t.Fatalf("single authorized applicant decision: %+v %v", approved, err)
	}
	secondReview := selfReview
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

func TestCreditApprovalServiceAllowsSingleReviewerToReject(t *testing.T) {
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
	rejected, err := service.ReviewCreditApplication(ctx, CreditReviewInput{
		ApplicationNo: application.ApplicationNo, Decision: CreditDecisionReject,
		ReviewerID: 402, CommentDigest: strings.Repeat("c", 64),
		EvidenceDigest: strings.Repeat("d", 64),
	})
	if err != nil {
		t.Fatalf("persist second-person rejection: %v", err)
	}
	if rejected.StatusCode != "rejected" || rejected.ReviewCount != 1 || rejected.CreditAccountNo != "" {
		t.Fatalf("unexpected rejected application: %+v", rejected)
	}
}

// Legacy first approvals remain immutable and their author can complete the
// account without finding a second staff member or resubmitting old materials.
func TestCreditApprovalServiceCompletesLegacyFirstApprovalBySameOperator(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	service, err := NewCreditApprovalService(database)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ConfigureCreditPolicy(ctx, CreditPolicyInput{CurrencyCode: "CNY", MaximumLimitMinor: 20000, DualReviewThresholdMinor: 5000, BillingDay: 10, RepaymentPeriodDays: 7, ConfiguredBy: 101, IdempotencyKey: "legacy-credit-policy-0001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_credit_application
(application_no,customer_no,currency_code,policy_no,policy_version,requested_limit_minor,requested_valid_from,requested_valid_until,required_approval_count,risk_evidence_reference,risk_evidence_digest,request_digest,status_code,request_actor_type_code,requested_by)
VALUES ('legacy-first-approval','customer-ledger','CNY',(SELECT policy_no FROM plugin_idc_core_credit_policy_revision WHERE currency_code='CNY' LIMIT 1),1,6000,CURRENT_TIMESTAMP-INTERVAL '1 minute',CURRENT_TIMESTAMP+INTERVAL '1 year',2,'private:legacy',repeat('a',64),repeat('b',64),'pending_review','staff',201);
INSERT INTO plugin_idc_core_credit_application_review
(review_no,application_no,customer_no,currency_code,review_sequence,decision_code,reviewer_id,comment_digest,evidence_digest,content_digest)
VALUES ('legacy-first-review','legacy-first-approval','customer-ledger','CNY',1,'approved',201,repeat('c',64),repeat('d',64),repeat('e',64))`); err != nil {
		t.Fatal(err)
	}
	input := CreditReviewInput{ApplicationNo: "legacy-first-approval", Decision: CreditDecisionApprove, ReviewerID: 201, CommentDigest: strings.Repeat("f", 64), EvidenceDigest: strings.Repeat("f", 64)}
	result, err := service.ReviewCreditApplication(ctx, input)
	if err != nil || result.StatusCode != "approved" || result.ReviewCount != 1 || result.CreditAccountNo == "" {
		t.Fatalf("legacy completion: %+v %v", result, err)
	}
	replay, err := service.ReviewCreditApplication(ctx, input)
	if err != nil || !replay.Reused || replay.CreditAccountNo != result.CreditAccountNo {
		t.Fatalf("legacy replay: %+v %v", replay, err)
	}
}
