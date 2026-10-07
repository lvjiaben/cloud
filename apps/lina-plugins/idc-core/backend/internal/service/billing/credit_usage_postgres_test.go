package billing

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestCreditUsageServiceReservesIdempotentlyAndReleasesUnbilledExposure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	activateCreditAccountForUsageTest(t, ctx, database, "customer-ledger", 8_000, "usage")
	service, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create credit usage service: %v", err)
	}

	reserveInput := CreditReserveInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1_000,
		SourceType: "manual", SourceReference: "credit-usage-reserve-0001",
		SourceSnapshotDigest: strings.Repeat("1", 64),
		ActorType:            "staff", ActorID: 501,
	}
	reserved, err := service.ReserveCredit(ctx, reserveInput)
	if err != nil {
		t.Fatalf("reserve credit: %v", err)
	}
	if reserved.Reused || reserved.StatusCode != "unbilled" ||
		reserved.UnbilledPrincipalMinor != 1_000 ||
		reserved.AccountAvailableMinor != 7_000 ||
		reserved.AccountUnbilledMinor != 1_000 || reserved.AccountVersion != 2 {
		t.Fatalf("unexpected reserved projection: %+v", reserved)
	}
	replayed, err := service.ReserveCredit(ctx, reserveInput)
	if err != nil || !replayed.Reused || replayed.AccountVersion != 2 {
		t.Fatalf("reserve replay=%+v err=%v", replayed, err)
	}
	changedReserve := reserveInput
	changedReserve.AmountMinor++
	if _, err = service.ReserveCredit(ctx, changedReserve); err == nil {
		t.Fatal("reserve replay accepted changed amount")
	}
	if _, err = service.ReserveCredit(ctx, CreditReserveInput{
		CustomerNo: "customer-ledger", CurrencyCode: "USD", AmountMinor: 1,
		SourceType: "manual", SourceReference: "credit-cross-currency-0001",
		SourceSnapshotDigest: strings.Repeat("2", 64),
		ActorType:            "staff", ActorID: 501,
	}); err == nil {
		t.Fatal("credit reserve crossed its account currency")
	}

	releaseInput := CreditReleaseInput{
		ExposureNo: reserved.ExposureNo, SourceReference: "credit-usage-release-0001",
		EvidenceDigest: strings.Repeat("3", 64), ActorType: "staff", ActorID: 502,
	}
	released, err := service.ReleaseUnbilledCredit(ctx, releaseInput)
	if err != nil {
		t.Fatalf("release unbilled credit: %v", err)
	}
	if released.Reused || released.StatusCode != "released" ||
		released.ReleasedPrincipalMinor != 1_000 || released.UnbilledPrincipalMinor != 0 ||
		released.AccountAvailableMinor != 8_000 || released.AccountUnbilledMinor != 0 ||
		released.AccountVersion != 3 {
		t.Fatalf("unexpected released projection: %+v", released)
	}
	replayedRelease, err := service.ReleaseUnbilledCredit(ctx, releaseInput)
	if err != nil || !replayedRelease.Reused || replayedRelease.AccountVersion != 3 {
		t.Fatalf("release replay=%+v err=%v", replayedRelease, err)
	}
	changedRelease := releaseInput
	changedRelease.EvidenceDigest = strings.Repeat("4", 64)
	if _, err = service.ReleaseUnbilledCredit(ctx, changedRelease); err == nil {
		t.Fatal("release replay accepted changed evidence")
	}

	var facts struct {
		MovementCount int64 `orm:"movement_count"`
		ExposureCount int64 `orm:"exposure_count"`
	}
	if err = database.GetScan(ctx, &facts, `
SELECT
    (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement
     WHERE credit_account_no = ?) AS movement_count,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_exposure
     WHERE exposure_no = ?) AS exposure_count`,
		released.CreditAccountNo, released.ExposureNo); err != nil {
		t.Fatalf("load credit usage facts: %v", err)
	}
	if facts.MovementCount != 3 || facts.ExposureCount != 1 {
		t.Fatalf("unexpected immutable fact counts: %+v", facts)
	}
}

func TestCreditUsageServiceSerializesConcurrentOverspend(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	account := activateCreditAccountForUsageTest(
		t, ctx, database, "customer-ledger", 10_000, "concurrent",
	)
	service, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create credit usage service: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for index := 1; index <= 2; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, reserveErr := service.ReserveCredit(ctx, CreditReserveInput{
				CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 6_000,
				SourceType:           "manual",
				SourceReference:      fmt.Sprintf("credit-concurrent-reserve-%04d", index),
				SourceSnapshotDigest: strings.Repeat(string(rune('4'+index)), 64),
				ActorType:            "system", ActorID: 0,
			})
			results <- reserveErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	succeeded := 0
	failed := 0
	for reserveErr := range results {
		if reserveErr == nil {
			succeeded++
		} else {
			failed++
		}
	}
	if succeeded != 1 || failed != 1 {
		t.Fatalf("concurrent reserve succeeded=%d failed=%d", succeeded, failed)
	}

	projection, err := loadCreditAccountByNo(ctx, database, account.CreditAccountNo, false)
	if err != nil {
		t.Fatalf("load concurrent account projection: %v", err)
	}
	if !validCreditAccountProjection(projection) || projection.AvailableLimitMinor != 4_000 ||
		projection.UnbilledPrincipalMinor != 6_000 || projection.Version != 2 {
		t.Fatalf("unexpected concurrent account projection: %+v", projection)
	}
	exposureCount, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_core_credit_exposure
WHERE tenant_id = 0 AND credit_account_no = ?`, account.CreditAccountNo)
	if err != nil || exposureCount.Int64() != 1 {
		t.Fatalf("concurrent exposure count=%d err=%v", exposureCount.Int64(), err)
	}
}

func activateCreditAccountForUsageTest(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	limitMinor int64,
	suffix string,
) *CreditApplication {
	return activateCreditAccountForUsageTestWithTerms(
		t, ctx, database, customerNo, limitMinor, suffix,
		time.Now().UTC().Add(-time.Minute), 10, 3,
	)
}

func activateCreditAccountForUsageTestWithTerms(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	limitMinor int64,
	suffix string,
	validFrom time.Time,
	repaymentPeriodDays int,
	gracePeriodDays int,
) *CreditApplication {
	return activateCreditAccountForUsageTestWithPolicy(
		t, ctx, database, customerNo, limitMinor, suffix, validFrom,
		repaymentPeriodDays, gracePeriodDays, 0, 0,
	)
}

func activateCreditAccountForUsageTestWithPolicy(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	limitMinor int64,
	suffix string,
	validFrom time.Time,
	repaymentPeriodDays int,
	gracePeriodDays int,
	dailyLateFeeBasisPoints int,
	maximumLateFeeMinor int64,
) *CreditApplication {
	t.Helper()
	service, err := NewCreditApprovalService(database)
	if err != nil {
		t.Fatalf("create credit approval service: %v", err)
	}
	_, err = service.ConfigureCreditPolicy(ctx, CreditPolicyInput{
		CurrencyCode: "CNY", MaximumLimitMinor: 50_000,
		DualReviewThresholdMinor: 20_000, BillingDay: 15,
		RepaymentPeriodDays: repaymentPeriodDays, GracePeriodDays: gracePeriodDays,
		DailyLateFeeBasisPoints: dailyLateFeeBasisPoints,
		MaximumLateFeeMinor:     maximumLateFeeMinor,
		ConfiguredBy:            101, IdempotencyKey: "credit-policy-" + suffix + "-0001",
	})
	if err != nil {
		t.Fatalf("configure usage policy: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	application, err := service.SubmitCreditApplication(ctx, CreditApplicationInput{
		CustomerNo: customerNo, CurrencyCode: "CNY", RequestedLimitMinor: limitMinor,
		RequestedValidFrom:    validFrom.UTC().Truncate(time.Microsecond),
		RequestedValidUntil:   now.AddDate(1, 0, 0),
		RiskEvidenceReference: "private:risk/" + customerNo + "/" + suffix,
		RiskEvidenceDigest:    strings.Repeat("e", 64),
		RequestActorType:      "customer", RequestedBy: 601,
		IdempotencyKey: "credit-application-" + suffix + "-0001",
	})
	if err != nil {
		t.Fatalf("submit usage credit application: %v", err)
	}
	approved, err := service.ReviewCreditApplication(ctx, CreditReviewInput{
		ApplicationNo: application.ApplicationNo, Decision: CreditDecisionApprove,
		ReviewerID: 602, CommentDigest: strings.Repeat("f", 64),
		EvidenceDigest: strings.Repeat("0", 64),
	})
	if err != nil {
		t.Fatalf("approve usage credit application: %v", err)
	}
	return approved
}
