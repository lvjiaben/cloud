// This file verifies credit-purchase refunds against real PostgreSQL state,
// including unpaid reductions, original-funding returns, and exact replay.

package billing

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

func TestCreditRefundPostgreSQLCoversUnbilledBilledAndRepaidStates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		repaidMinor        int64
		billed             bool
		fundingDestination CreditRefundDestination
	}{
		{name: "unbilled"},
		{name: "billed unpaid", billed: true},
		{name: "partially repaid", billed: true, repaidMinor: 400,
			fundingDestination: CreditRefundDestinationOriginalSource},
		{name: "fully repaid to consented balance", billed: true, repaidMinor: 1_000,
			fundingDestination: CreditRefundDestinationBalance},
	}
	for index, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			database := configureBillingServiceIntegrationDB(t, ctx)
			seedBillingService(t, ctx, database)
			suffix := fmt.Sprintf("refund-%02d", index+1)
			account := activateCreditAccountForUsageTest(
				t, ctx, database, "customer-ledger", 8_000, suffix,
			)
			service, err := NewCreditUsageService(database)
			if err != nil {
				t.Fatalf("create credit refund service: %v", err)
			}
			exposure, err := service.ReserveCredit(ctx, CreditReserveInput{
				CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1_000,
				SourceType: "manual", SourceReference: "credit-" + suffix + "-exposure-0001",
				SourceSnapshotDigest: strings.Repeat("1", 64),
				ActorType:            "system", ActorID: 0,
			})
			if err != nil {
				t.Fatalf("reserve refund exposure: %v", err)
			}
			statementNo := ""
			if test.billed {
				statementNo = seedCreditRefundStatement(
					t, ctx, database, account.CreditAccountNo, exposure.ExposureNo, suffix,
				)
			}
			if test.repaidMinor > 0 {
				fundingReference := "credit-" + suffix + "-repayment-0001"
				transactionNo := postCreditRepaymentLedger(
					t, ctx, database, test.repaidMinor, fundingReference,
				)
				if _, err = service.ApplyConfirmedCreditStatementRepayment(
					ctx, CreditStatementRepaymentInput{
						StatementNo: statementNo, FundingSourceType: "online_payment",
						FundingSourceReference: fundingReference,
						LedgerTransactionNo:    transactionNo, AmountMinor: test.repaidMinor,
						SourceSnapshotDigest:       strings.Repeat("2", 64),
						ConfirmationEvidenceDigest: strings.Repeat("3", 64),
					},
				); err != nil {
					t.Fatalf("repay credit before refund: %v", err)
				}
			}

			plan, err := service.PreviewCreditRefund(ctx, exposure.ExposureNo, 1_000)
			if err != nil {
				t.Fatalf("preview credit refund: %v", err)
			}
			if plan.UnbilledReleaseMinor != boolAmount(!test.billed, 1_000) ||
				plan.BilledReductionMinor != boolAmount(test.billed, 1_000-test.repaidMinor) ||
				plan.RepaidRefundMinor != test.repaidMinor {
				t.Fatalf("unexpected credit refund plan: %+v", plan)
			}
			fundingResults := make([]CreditRefundFundingResult, 0, len(plan.FundingSources))
			for sourceIndex, source := range plan.FundingSources {
				fundingResults = append(fundingResults, CreditRefundFundingResult{
					RepaymentAllocationNo: source.RepaymentAllocationNo,
					AmountMinor:           source.AmountMinor,
					RefundSourceReference: fmt.Sprintf(
						"credit-%s-refund-funding-%04d", suffix, sourceIndex+1,
					),
					EvidenceDigest: strings.Repeat("4", 64),
					Destination:    test.fundingDestination,
				})
			}
			input := CreditRefundInput{
				ExposureNo: exposure.ExposureNo, AmountMinor: 1_000,
				PlanDigest:      plan.PlanDigest,
				SourceReference: "credit-" + suffix + "-refund-0001",
				EvidenceDigest:  strings.Repeat("5", 64),
				ActorType:       "staff", ActorID: 701, FundingResults: fundingResults,
			}
			refunded, err := service.ApplyConfirmedCreditRefund(ctx, input)
			if err != nil {
				t.Fatalf("apply credit refund: %v", err)
			}
			if refunded.ExposureStatusCode != "released" ||
				refunded.AccountAvailableMinor != 8_000 ||
				refunded.AccountUnbilledMinor != 0 || refunded.AccountBilledMinor != 0 {
				t.Fatalf("unexpected credit refund projection: %+v", refunded)
			}
			if test.billed && refunded.StatementStatusCode != "refunded" {
				t.Fatalf("credit statement did not become refunded: %+v", refunded)
			}
			replayed, err := service.ApplyConfirmedCreditRefund(ctx, input)
			if err != nil || !replayed.Reused {
				t.Fatalf("credit refund replay=%+v err=%v", replayed, err)
			}
			assertCreditRefundFacts(
				t, ctx, database, exposure.ExposureNo, statementNo,
				test.repaidMinor, len(fundingResults),
			)
			if test.fundingDestination == CreditRefundDestinationBalance {
				assertCreditRefundBalanceDestination(t, ctx, database, test.repaidMinor)
			}
		})
	}
}

func assertCreditRefundBalanceDestination(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	expectedMinor int64,
) {
	t.Helper()
	var balance struct {
		BalanceMinor int64 `orm:"balance_minor"`
	}
	if err := database.GetScan(ctx, &balance, `
SELECT snapshot.closing_balance_minor AS balance_minor
FROM plugin_idc_core_billing_ledger_account account
JOIN plugin_idc_core_billing_balance_snapshot snapshot
  ON snapshot.account_no=account.account_no
WHERE account.owner_type_code='customer'
  AND account.owner_reference='customer-ledger'
  AND account.account_type_code='customer_balance'
  AND account.currency_code='CNY'
ORDER BY snapshot.sequence_no DESC
LIMIT 1`); err != nil {
		t.Fatalf("load consented credit refund balance: %v", err)
	}
	if balance.BalanceMinor != expectedMinor {
		t.Fatalf("consented credit refund balance=%d want=%d",
			balance.BalanceMinor, expectedMinor)
	}
}

func TestCreditRefundPostgreSQLSupportsMultiplePartialRefunds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	activateCreditAccountForUsageTest(t, ctx, database, "customer-ledger", 8_000, "partial-refund")
	service, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatal(err)
	}
	exposure, err := service.ReserveCredit(ctx, CreditReserveInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1_000,
		SourceType: "manual", SourceReference: "credit-partial-refund-exposure-0001",
		SourceSnapshotDigest: strings.Repeat("6", 64), ActorType: "system", ActorID: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	for index, amount := range []int64{300, 200, 500} {
		plan, previewErr := service.PreviewCreditRefund(ctx, exposure.ExposureNo, amount)
		if previewErr != nil {
			t.Fatalf("preview partial refund %d: %v", index+1, previewErr)
		}
		result, applyErr := service.ApplyConfirmedCreditRefund(ctx, CreditRefundInput{
			ExposureNo: exposure.ExposureNo, AmountMinor: amount, PlanDigest: plan.PlanDigest,
			SourceReference: fmt.Sprintf("credit-partial-refund-%04d", index+1),
			EvidenceDigest:  strings.Repeat(string(rune('7'+index)), 64),
			ActorType:       "system", ActorID: 0,
		})
		if applyErr != nil {
			t.Fatalf("apply partial refund %d: %v", index+1, applyErr)
		}
		expectedAvailable := int64(7_000)
		for prior := 0; prior <= index; prior++ {
			expectedAvailable += []int64{300, 200, 500}[prior]
		}
		if result.AccountAvailableMinor != expectedAvailable {
			t.Fatalf("partial refund %d projection: %+v", index+1, result)
		}
	}
	if _, err = service.PreviewCreditRefund(ctx, exposure.ExposureNo, 1); err == nil {
		t.Fatal("fully refunded exposure remained refundable")
	}
}

func TestCreditRefundPostgreSQLRejectsChangedPlanAndMissingFundingEvidence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	account := activateCreditAccountForUsageTest(
		t, ctx, database, "customer-ledger", 8_000, "refund-rejection",
	)
	service, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatal(err)
	}
	exposure, err := service.ReserveCredit(ctx, CreditReserveInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1_000,
		SourceType: "manual", SourceReference: "credit-refund-rejection-exposure",
		SourceSnapshotDigest: strings.Repeat("a", 64), ActorType: "system", ActorID: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	statementNo := seedCreditRefundStatement(
		t, ctx, database, account.CreditAccountNo, exposure.ExposureNo, "reject",
	)
	fundingReference := "credit-refund-rejection-repayment"
	transactionNo := postCreditRepaymentLedger(t, ctx, database, 400, fundingReference)
	if _, err = service.ApplyConfirmedCreditStatementRepayment(
		ctx, CreditStatementRepaymentInput{
			StatementNo: statementNo, FundingSourceType: "online_payment",
			FundingSourceReference: fundingReference, LedgerTransactionNo: transactionNo,
			AmountMinor: 400, SourceSnapshotDigest: strings.Repeat("b", 64),
			ConfirmationEvidenceDigest: strings.Repeat("c", 64),
		},
	); err != nil {
		t.Fatal(err)
	}
	plan, err := service.PreviewCreditRefund(ctx, exposure.ExposureNo, 1_000)
	if err != nil || plan == nil || len(plan.FundingSources) != 1 {
		t.Fatalf("preview rejection plan: plan=%+v err=%v", plan, err)
	}
	base := CreditRefundInput{
		ExposureNo: exposure.ExposureNo, AmountMinor: 1_000,
		PlanDigest: strings.Repeat("d", 64), SourceReference: "credit-refund-rejection-apply",
		EvidenceDigest: strings.Repeat("e", 64), ActorType: "system", ActorID: 0,
	}
	if _, err = service.ApplyConfirmedCreditRefund(ctx, base); !bizerr.Is(err, CodeCreditConflict) {
		t.Fatalf("changed credit refund plan was accepted: %v", err)
	}
	base.PlanDigest = plan.PlanDigest
	if _, err = service.ApplyConfirmedCreditRefund(ctx, base); !bizerr.Is(err, CodeCreditConflict) {
		t.Fatalf("missing credit refund funding evidence was accepted: %v", err)
	}
	var unchanged struct {
		BilledMinor int64 `orm:"billed_principal_minor"`
		RepaidMinor int64 `orm:"repaid_principal_minor"`
		Released    int64 `orm:"released_principal_minor"`
		Movements   int64 `orm:"movements"`
	}
	if err = database.GetScan(ctx, &unchanged, `
SELECT exposure.billed_principal_minor,exposure.repaid_principal_minor,
       exposure.released_principal_minor,
       (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement movement
        WHERE movement.exposure_no=exposure.exposure_no
          AND movement.movement_type_code='refund') AS movements
FROM plugin_idc_core_credit_exposure exposure
WHERE exposure.exposure_no=?`, exposure.ExposureNo); err != nil {
		t.Fatal(err)
	}
	if unchanged.BilledMinor != 600 || unchanged.RepaidMinor != 400 ||
		unchanged.Released != 0 || unchanged.Movements != 0 {
		t.Fatalf("rejected credit refund changed state: %+v", unchanged)
	}
}

func seedCreditRefundStatement(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	creditAccountNo string,
	exposureNo string,
	suffix string,
) string {
	t.Helper()
	statementNo := "credit-" + suffix + "-statement-0001"
	periodEnd := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Microsecond)
	periodStart := periodEnd.AddDate(0, -1, 0)
	mustCreditTransaction(t, ctx, database, "seed credit refund statement", func(ctx context.Context) error {
		if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_statement (
    statement_no,credit_account_no,customer_no,currency_code,terms_no,
    terms_version,period_started_at,period_ended_at,line_count,
    principal_minor,total_amount_minor,outstanding_amount_minor,status_code,
    content_digest,issued_at,payment_due_at
) SELECT ?,account.credit_account_no,account.customer_no,account.currency_code,
         account.current_terms_no,account.current_terms_version,?,?,1,
         1000,1000,1000,'open',?,CURRENT_TIMESTAMP,
         CURRENT_TIMESTAMP+make_interval(days=>terms.repayment_period_days)
  FROM plugin_idc_core_credit_account account
  JOIN plugin_idc_core_credit_account_terms terms
    ON terms.terms_no=account.current_terms_no
   AND terms.credit_account_no=account.credit_account_no
	 WHERE account.credit_account_no=?`,
			statementNo, periodStart, periodEnd, strings.Repeat("a", 64), creditAccountNo,
		); err != nil {
			return err
		}
		if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_statement_line (
    statement_line_no,statement_no,credit_account_no,customer_no,currency_code,
    line_number,line_type_code,exposure_no,source_type_code,source_reference,
    amount_minor,content_digest
) SELECT ?,?,?,customer_no,currency_code,1,'principal',?,'exposure',?,1000,?
	  FROM plugin_idc_core_credit_account WHERE credit_account_no=?`,
			"credit-"+suffix+"-statement-line-0001", statementNo, creditAccountNo,
			exposureNo, exposureNo, strings.Repeat("b", 64), creditAccountNo,
		); err != nil {
			return err
		}
		if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_exposure
SET unbilled_principal_minor=0,billed_principal_minor=1000,
    status_code='billed',version=version+1,updated_at=CURRENT_TIMESTAMP
	WHERE exposure_no=?`, exposureNo); err != nil {
			return err
		}
		if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET unbilled_principal_minor=0,billed_principal_minor=1000,
    version=version+1,updated_at=CURRENT_TIMESTAMP
	WHERE credit_account_no=?`, creditAccountNo); err != nil {
			return err
		}
		_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_account_movement (
    movement_no,credit_account_no,customer_no,currency_code,exposure_no,
    statement_no,movement_type_code,amount_minor,account_version_before,
    account_version_after,status_before_code,status_after_code,
    total_limit_before_minor,total_limit_after_minor,available_before_minor,
    available_after_minor,unbilled_before_minor,unbilled_after_minor,
    billed_before_minor,billed_after_minor,delinquent_before_minor,
    delinquent_after_minor,written_off_before_minor,written_off_after_minor,
    source_type_code,source_reference,idempotency_digest,evidence_digest,
    content_digest,actor_type_code,actor_id,occurred_at
) SELECT ?,credit_account_no,customer_no,currency_code,?,?,
         'bill',1000,version-1,version,status_code,status_code,
         total_limit_minor,total_limit_minor,available_limit_minor,
         available_limit_minor,1000,0,0,1000,delinquent_principal_minor,
         delinquent_principal_minor,written_off_principal_minor,
         written_off_principal_minor,'statement',?,?,?,?,'system',0,CURRENT_TIMESTAMP
	  FROM plugin_idc_core_credit_account WHERE credit_account_no=?`,
			"credit-"+suffix+"-bill-movement-0001", exposureNo, statementNo,
			statementNo, strings.Repeat("c", 64),
			strings.Repeat("d", 64), strings.Repeat("e", 64), creditAccountNo)
		return err
	})
	return statementNo
}

func assertCreditRefundFacts(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	exposureNo string,
	statementNo string,
	repaidMinor int64,
	expectedFundingCount int,
) {
	t.Helper()
	var facts struct {
		RefundMovements  int64  `orm:"refund_movements"`
		ExposureStatus   string `orm:"exposure_status"`
		StatementStatus  string `orm:"statement_status"`
		RefundLines      int64  `orm:"refund_lines"`
		ReversalCount    int64  `orm:"reversal_count"`
		RefundLedgerRows int64  `orm:"refund_ledger_rows"`
	}
	if err := database.GetScan(ctx, &facts, `
SELECT
  (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement
   WHERE exposure_no=? AND movement_type_code='refund') AS refund_movements,
  (SELECT status_code FROM plugin_idc_core_credit_exposure
   WHERE exposure_no=?) AS exposure_status,
  COALESCE((SELECT status_code FROM plugin_idc_core_credit_statement
            WHERE statement_no=?),'') AS statement_status,
  (SELECT COUNT(*) FROM plugin_idc_core_credit_statement_line
   WHERE statement_no=? AND line_type_code='refund') AS refund_lines,
  (SELECT COUNT(*) FROM plugin_idc_core_credit_repayment_allocation
   WHERE exposure_no=? AND direction_code='reverse') AS reversal_count,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_transaction
   WHERE source_type_code='credit_refund') AS refund_ledger_rows`,
		exposureNo, exposureNo, statementNo, statementNo, exposureNo); err != nil {
		t.Fatalf("load credit refund facts: %v", err)
	}
	if facts.RefundMovements != 1 || facts.ExposureStatus != "released" ||
		facts.ReversalCount != int64(expectedFundingCount) ||
		facts.RefundLedgerRows != int64(expectedFundingCount) {
		t.Fatalf("unexpected credit refund facts: %+v", facts)
	}
	if statementNo != "" && (facts.StatementStatus != "refunded" || facts.RefundLines != 1) {
		t.Fatalf("unexpected refunded statement facts: %+v", facts)
	}
	if repaidMinor == 0 && (facts.ReversalCount != 0 || facts.RefundLedgerRows != 0) {
		t.Fatalf("unpaid refund created cash facts: %+v", facts)
	}
}

func boolAmount(condition bool, amount int64) int64 {
	if condition {
		return amount
	}
	return 0
}
