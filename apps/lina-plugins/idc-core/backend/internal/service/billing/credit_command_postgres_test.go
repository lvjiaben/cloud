// This file verifies protected credit commands, proofs, audit, and transactions in PostgreSQL.

package billing

import (
	"context"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

type creditCommandAuthorizerStub struct {
	policyActor int
	reviewActor int
}

func (s *creditCommandAuthorizerStub) actor() int {
	if s.policyActor > 0 {
		return s.policyActor
	}
	return 9001
}
func (s *creditCommandAuthorizerStub) AuthorizeApplicationList(context.Context) (int, error) {
	return s.actor(), nil
}
func (s *creditCommandAuthorizerStub) AuthorizeApplicationDetail(context.Context, string) (int, error) {
	return s.actor(), nil
}
func (s *creditCommandAuthorizerStub) AuthorizeApplicationReview(context.Context, string) (int, error) {
	if s.reviewActor > 0 {
		return s.reviewActor, nil
	}
	return s.actor(), nil
}
func (s *creditCommandAuthorizerStub) AuthorizeAccountList(context.Context) (int, error) {
	return s.actor(), nil
}
func (s *creditCommandAuthorizerStub) AuthorizeAccountDetail(context.Context, string) (int, error) {
	return s.actor(), nil
}
func (s *creditCommandAuthorizerStub) AuthorizeStatementList(context.Context) (int, error) {
	return s.actor(), nil
}
func (s *creditCommandAuthorizerStub) AuthorizeStatementDetail(context.Context, string) (int, error) {
	return s.actor(), nil
}
func (s *creditCommandAuthorizerStub) AuthorizePolicyWrite(context.Context) (int, error) {
	return s.actor(), nil
}

func TestCreditCommandsPostgreSQLBindExactProofsDeriveCustomerAndAudit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	approval, err := NewCreditApprovalService(database)
	if err != nil {
		t.Fatal(err)
	}
	usage, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatal(err)
	}
	authorizer := &creditCommandAuthorizerStub{policyActor: 9101, reviewActor: 9102}
	recentAuth := &managementCommandRecentAuthStub{consumeActor: authorizer.policyActor}
	managementAudit := &managementCommandAuditStub{}
	management, err := NewManagementCreditCommandService(
		database, managementAudit, authorizer, recentAuth, approval,
	)
	if err != nil {
		t.Fatal(err)
	}
	policyIntent := ManagementCreditPolicyIntent{
		RequestNo: "credit-command-policy-0001", CurrencyCode: "CNY",
		MaximumLimitMinor: 50_000, DualReviewThresholdMinor: 5_000,
		BillingDay: 15, RepaymentPeriodDays: 10, GracePeriodDays: 3,
		DailyLateFeeBasisPoints: 5, MaximumLateFeeMinor: 5_000,
	}
	proof, err := management.IssueCreditPolicyProof(ctx, policyIntent)
	if err != nil || proof.Token == "" || recentAuth.issued.Purpose != reauth.PurposeHighValueFunds {
		t.Fatalf("issue credit policy proof=%+v binding=%+v err=%v", proof, recentAuth.issued, err)
	}
	policy, err := management.ConfigureCreditPolicy(ctx, policyIntent, proof.Token)
	if err != nil || policy.ConfiguredBy != authorizer.policyActor ||
		len(managementAudit.transitions) != 1 ||
		managementAudit.transitions[0].RequiredPermission != permissionCreditPolicyWrite {
		t.Fatalf("configure credit policy=%+v audit=%+v err=%v", policy, managementAudit.transitions, err)
	}

	accountAudit := &managementCommandAuditStub{}
	accountCommands, err := NewAccountCreditCommandService(
		database,
		financialPrincipalStub{audience: activeCreditCustomer(77, "customer-ledger")},
		accountAudit, recentAuth, approval, usage,
	)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	application, err := accountCommands.SubmitCreditApplication(ctx, AccountCreditApplicationIntent{
		CurrencyCode: "CNY", RequestedLimitMinor: 6_000,
		RequestedValidFrom: now.Add(-time.Minute), RequestedValidUntil: now.AddDate(1, 0, 0),
		RiskEvidenceReference: "private:risk/credit-command-customer",
		RiskEvidenceDigest:    strings.Repeat("a", 64), IdempotencyKey: "credit-command-application-0001",
	})
	if err != nil || application.CustomerNo != "customer-ledger" || application.RequestedBy != 77 ||
		len(accountAudit.transitions) != 1 || accountAudit.transitions[0].CustomerNo != "customer-ledger" {
		t.Fatalf("submit customer credit application=%+v audit=%+v err=%v", application, accountAudit.transitions, err)
	}

	recentAuth.consumeActor = authorizer.reviewActor
	reviewIntent := ManagementCreditReviewIntent{
		RequestNo: "credit-command-review-0001", ApplicationNo: application.ApplicationNo,
		Decision: CreditDecisionApprove, CommentDigest: strings.Repeat("b", 64),
		EvidenceDigest: strings.Repeat("c", 64),
	}
	proof, err = management.IssueCreditApplicationReviewProof(ctx, reviewIntent)
	if err != nil {
		t.Fatal(err)
	}
	reviewed, err := management.ReviewCreditApplication(ctx, reviewIntent, proof.Token)
	if err != nil || reviewed.ReviewCount != 1 || reviewed.StatusCode != "approved" ||
		len(managementAudit.transitions) != 2 ||
		managementAudit.transitions[1].RequiredPermission != permissionCreditApplicationReview {
		t.Fatalf("review credit application=%+v audit=%+v err=%v", reviewed, managementAudit.transitions, err)
	}
}

func TestAccountCreditBalanceRepaymentCommandRejectsForeignOwnerBeforeProof(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	fixture := prepareCreditDelinquencyFixture(t, ctx, database, -10, "cmdowner")
	approval, err := NewCreditApprovalService(database)
	if err != nil {
		t.Fatal(err)
	}
	usage, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := NewBalanceService(database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 500,
		SourceReference:      "credit-command-owner-recharge-0001",
		SourceSnapshotDigest: strings.Repeat("d", 64),
	}); err != nil {
		t.Fatalf("seed owner balance: %v", err)
	}
	recentAuth := &managementCommandRecentAuthStub{consumeActor: 77}
	ownerAudit := &managementCommandAuditStub{}
	owner, err := NewAccountCreditCommandService(
		database,
		financialPrincipalStub{audience: activeCreditCustomer(77, "customer-ledger")},
		ownerAudit, recentAuth, approval, usage,
	)
	if err != nil {
		t.Fatal(err)
	}
	intent := AccountCreditBalanceRepaymentIntent{
		StatementNo: fixture.statementNo, AmountMinor: 100,
		RequestNo: "owner-credit-repayment-0001",
	}
	proof, err := owner.IssueCreditBalanceRepaymentProof(ctx, intent)
	if err != nil || proof.Token == "" {
		t.Fatalf("issue owner repayment proof=%+v err=%v", proof, err)
	}
	repaid, err := owner.RepayCreditStatementFromBalance(ctx, intent, proof.Token)
	if err != nil || repaid.Repayment == nil || repaid.Repayment.StatementOutstandingMinor != 900 ||
		len(ownerAudit.transitions) != 1 || ownerAudit.transitions[0].CustomerNo != "customer-ledger" {
		t.Fatalf("owner credit repayment=%+v audit=%+v err=%v", repaid, ownerAudit.transitions, err)
	}
	replayed, replayErr := owner.RepayCreditStatementFromBalance(ctx, intent, proof.Token)
	if replayErr != nil || replayed == nil || !replayed.Reused || len(ownerAudit.transitions) != 1 {
		t.Fatalf("repayment replay must reuse the committed audit: result=%+v audits=%d err=%v", replayed, len(ownerAudit.transitions), replayErr)
	}
	recentAuth.consumeActor = 78
	proofCallsBeforeForeign := recentAuth.issueCalls
	foreign, err := NewAccountCreditCommandService(
		database,
		financialPrincipalStub{audience: activeCreditCustomer(78, "customer-ledger-other")},
		&managementCommandAuditStub{}, recentAuth, approval, usage,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = foreign.IssueCreditBalanceRepaymentProof(ctx, AccountCreditBalanceRepaymentIntent{
		StatementNo: fixture.statementNo, AmountMinor: 100, RequestNo: "foreign-credit-repayment-0001",
	})
	if !bizerr.Is(err, CodeDirectoryNotAccessible) || recentAuth.issueCalls != proofCallsBeforeForeign {
		t.Fatalf("foreign statement reached proof issuance calls=%d err=%v", recentAuth.issueCalls, err)
	}
}
