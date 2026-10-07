package billing

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/internal/service/asynctask"
)

func TestCreditLateFeeIntegerCalculationIsBounded(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		principal     int64
		basisPoints   int
		days          int64
		cap           int64
		expectedDaily int64
		expectedTotal int64
	}{
		{name: "round up one minor unit", principal: 1, basisPoints: 1, days: 1, cap: 10, expectedDaily: 1, expectedTotal: 1},
		{name: "exact percentage", principal: 1_000, basisPoints: 100, days: 5, cap: 1_000, expectedDaily: 10, expectedTotal: 50},
		{name: "cap cumulative fee", principal: 1_000, basisPoints: 100, days: 5, cap: 35, expectedDaily: 10, expectedTotal: 35},
		{name: "avoid multiplication overflow", principal: maximumAmountMinor, basisPoints: 10_000, days: maximumLateFeeAccrualDays, cap: maximumAmountMinor, expectedDaily: maximumAmountMinor, expectedTotal: maximumAmountMinor},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			daily := calculateDailyCreditLateFee(test.principal, test.basisPoints)
			total := multiplyCreditLateFeeWithinCap(daily, test.days, test.cap)
			if daily != test.expectedDaily || total != test.expectedTotal {
				t.Fatalf("daily=%d total=%d, want daily=%d total=%d", daily, total, test.expectedDaily, test.expectedTotal)
			}
		})
	}
}

func TestCreditDelinquencyServiceKeepsGraceAnchoredToPaymentDueAt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	fixture := prepareCreditDelinquencyFixture(t, ctx, database, 1, "grace")
	service := newCreditDelinquencyServiceForTest(t, database)

	opened, err := service.OpenDueCreditStatement(ctx, fixture.statementNo)
	if err != nil {
		t.Fatalf("open due credit statement: %v", err)
	}
	if opened.StatusCode != "grace_period" || !opened.OpenedAt.Equal(fixture.paymentDueAt) ||
		!opened.GraceUntil.Equal(fixture.paymentDueAt.AddDate(0, 0, 3)) ||
		opened.AccountStatusCode != "active" || opened.AccountDelinquentMinor != 1_000 {
		t.Fatalf("unexpected grace-period delinquency: %+v", opened)
	}
	if _, err = service.EscalateCreditDelinquency(ctx, opened.CaseNo); err == nil {
		t.Fatal("credit delinquency escalated before the due-anchored grace period ended")
	}

	var facts struct {
		AccountStatus  string `orm:"account_status"`
		CaseStatus     string `orm:"case_status"`
		TaskCount      int64  `orm:"task_count"`
		FreezeMovement int64  `orm:"freeze_movement"`
	}
	if err = database.GetScan(ctx, &facts, `
SELECT
    (SELECT status_code FROM plugin_idc_core_credit_account
     WHERE credit_account_no = ?) AS account_status,
    (SELECT status_code FROM plugin_idc_core_credit_delinquency_case
     WHERE case_no = ?) AS case_status,
    (SELECT COUNT(*) FROM plugin_idc_core_async_task
     WHERE subject_no = ?) AS task_count,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement
     WHERE credit_account_no = ? AND movement_type_code = 'freeze') AS freeze_movement`,
		fixture.accountNo, opened.CaseNo, opened.CaseNo, fixture.accountNo); err != nil {
		t.Fatalf("load grace-period facts: %v", err)
	}
	if facts.AccountStatus != "active" || facts.CaseStatus != "grace_period" ||
		facts.TaskCount != 0 || facts.FreezeMovement != 0 {
		t.Fatalf("grace period changed protected state: %+v", facts)
	}
}

func TestCreditDelinquencyServiceEscalatesIdempotentlyAndRepaymentCuresCase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	fixture := prepareCreditDelinquencyFixture(t, ctx, database, 5, "escalate")
	delinquencyService := newCreditDelinquencyServiceForTest(t, database)
	usageService, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create credit usage service: %v", err)
	}

	opened, err := delinquencyService.OpenDueCreditStatement(ctx, fixture.statementNo)
	if err != nil {
		t.Fatalf("open overdue credit statement: %v", err)
	}
	replayedOpen, err := delinquencyService.OpenDueCreditStatement(ctx, fixture.statementNo)
	if err != nil || !replayedOpen.Reused || replayedOpen.CaseNo != opened.CaseNo ||
		replayedOpen.AccountVersion != opened.AccountVersion {
		t.Fatalf("open delinquency replay=%+v err=%v", replayedOpen, err)
	}
	escalated, err := delinquencyService.EscalateCreditDelinquency(ctx, opened.CaseNo)
	if err != nil {
		t.Fatalf("escalate credit delinquency: %v", err)
	}
	if escalated.Reused || escalated.StatusCode != "service_action_pending" ||
		escalated.AccountStatusCode != "frozen" || escalated.ServiceActionTaskNo == "" ||
		escalated.AccountDelinquentMinor != 1_000 {
		t.Fatalf("unexpected escalated delinquency: %+v", escalated)
	}
	replayedEscalation, err := delinquencyService.EscalateCreditDelinquency(ctx, opened.CaseNo)
	if err != nil || !replayedEscalation.Reused ||
		replayedEscalation.ServiceActionTaskNo != escalated.ServiceActionTaskNo {
		t.Fatalf("escalation replay=%+v err=%v", replayedEscalation, err)
	}
	assertCreditServiceActionTask(t, ctx, database, opened.CaseNo, escalated.ServiceActionTaskNo)

	firstReference := "credit-delinquency-repayment-0001"
	firstTransaction := postCreditRepaymentLedger(t, ctx, database, 400, firstReference)
	partial, err := usageService.ApplyConfirmedCreditStatementRepayment(
		ctx, CreditStatementRepaymentInput{
			StatementNo: fixture.statementNo, FundingSourceType: "online_payment",
			FundingSourceReference: firstReference, LedgerTransactionNo: firstTransaction,
			AmountMinor: 400, SourceSnapshotDigest: strings.Repeat("7", 64),
			ConfirmationEvidenceDigest: strings.Repeat("8", 64),
		},
	)
	if err != nil {
		t.Fatalf("apply partial overdue repayment: %v", err)
	}
	if partial.StatementStatusCode != "overdue" || partial.StatementOutstandingMinor != 600 ||
		partial.AccountDelinquentMinor != 600 || partial.AccountStatusCode != "frozen" {
		t.Fatalf("unexpected partial overdue repayment: %+v", partial)
	}
	assertCreditDelinquencyProjection(t, ctx, database, opened.CaseNo,
		"service_action_pending", 600, 600)

	secondReference := "credit-delinquency-repayment-0002"
	secondTransaction := postCreditRepaymentLedger(t, ctx, database, 600, secondReference)
	paid, err := usageService.ApplyConfirmedCreditStatementRepayment(
		ctx, CreditStatementRepaymentInput{
			StatementNo: fixture.statementNo, FundingSourceType: "online_payment",
			FundingSourceReference: secondReference, LedgerTransactionNo: secondTransaction,
			AmountMinor: 600, SourceSnapshotDigest: strings.Repeat("9", 64),
			ConfirmationEvidenceDigest: strings.Repeat("a", 64),
		},
	)
	if err != nil {
		t.Fatalf("settle overdue credit statement: %v", err)
	}
	if paid.StatementStatusCode != "paid" || paid.StatementOutstandingMinor != 0 ||
		paid.AccountDelinquentMinor != 0 || paid.AccountStatusCode != "frozen" {
		t.Fatalf("unexpected settled overdue repayment: %+v", paid)
	}
	assertCreditDelinquencyProjection(t, ctx, database, opened.CaseNo, "resolved", 0, 0)

	resolved, err := delinquencyService.EscalateCreditDelinquency(ctx, opened.CaseNo)
	if err != nil || !resolved.Reused || resolved.StatusCode != "resolved" ||
		resolved.ServiceActionTaskNo != "" || resolved.AccountStatusCode != "frozen" {
		t.Fatalf("resolved delinquency replay=%+v err=%v", resolved, err)
	}
}

func TestCreditDelinquencyServiceAccruesCappedLateFeeAndRepaysFeeFirst(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	fixture := prepareCreditDelinquencyFixtureWithLateFee(
		t, ctx, database, 5, "late-fee", 100, 35,
	)
	delinquencyService := newCreditDelinquencyServiceForTest(t, database)
	usageService, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create late-fee credit usage service: %v", err)
	}
	opened, err := delinquencyService.OpenDueCreditStatement(ctx, fixture.statementNo)
	if err != nil {
		t.Fatalf("open late-fee delinquency: %v", err)
	}
	accrued, err := delinquencyService.AccrueCreditLateFee(ctx, opened.CaseNo)
	if err != nil {
		t.Fatalf("accrue credit late fee: %v", err)
	}
	if accrued.Reused || accrued.PrincipalMinor != 1_000 ||
		accrued.LateFeeMinor != 35 || accrued.OutstandingAmountMinor != 1_035 ||
		accrued.AccountVersion != 5 {
		t.Fatalf("unexpected accrued credit late fee: %+v", accrued)
	}
	replayed, err := delinquencyService.AccrueCreditLateFee(ctx, opened.CaseNo)
	if err != nil || !replayed.Reused || replayed.LateFeeMinor != 35 ||
		replayed.AccountVersion != accrued.AccountVersion {
		t.Fatalf("late fee replay=%+v err=%v", replayed, err)
	}

	reference := "credit-late-fee-repayment-0001"
	transactionNo := postCreditRepaymentLedger(t, ctx, database, 35, reference)
	repaid, err := usageService.ApplyConfirmedCreditStatementRepayment(
		ctx, CreditStatementRepaymentInput{
			StatementNo: fixture.statementNo, FundingSourceType: "online_payment",
			FundingSourceReference: reference, LedgerTransactionNo: transactionNo,
			AmountMinor: 35, SourceSnapshotDigest: strings.Repeat("a", 64),
			ConfirmationEvidenceDigest: strings.Repeat("b", 64),
		},
	)
	if err != nil {
		t.Fatalf("repay accrued credit late fee: %v", err)
	}
	if repaid.StatementStatusCode != "overdue" ||
		repaid.StatementOutstandingMinor != 1_000 || repaid.AccountAvailableMinor != 7_000 ||
		repaid.AccountBilledMinor != 1_000 || repaid.AccountDelinquentMinor != 1_000 ||
		repaid.AccountVersion != 5 {
		t.Fatalf("late fee repayment changed principal projection: %+v", repaid)
	}
	assertCreditDelinquencyProjection(t, ctx, database, opened.CaseNo,
		"grace_period", 1_000, 1_000)

	var facts struct {
		StatementFee     int64 `orm:"statement_fee"`
		StatementLines   int   `orm:"statement_lines"`
		LateFeeLines     int64 `orm:"late_fee_lines"`
		LateFeeMovements int64 `orm:"late_fee_movements"`
		FeeAllocations   int64 `orm:"fee_allocations"`
	}
	if err = database.GetScan(ctx, &facts, `
SELECT
    (SELECT fee_minor FROM plugin_idc_core_credit_statement
     WHERE statement_no = ?) AS statement_fee,
    (SELECT line_count FROM plugin_idc_core_credit_statement
     WHERE statement_no = ?) AS statement_lines,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_statement_line
     WHERE statement_no = ? AND line_type_code = 'late_fee'
       AND amount_minor = 35) AS late_fee_lines,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement
     WHERE statement_no = ? AND movement_type_code = 'late_fee'
       AND amount_minor = 35) AS late_fee_movements,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_repayment_allocation
     WHERE statement_no = ? AND allocation_type_code = 'statement_fee'
       AND amount_minor = 35) AS fee_allocations`, fixture.statementNo,
		fixture.statementNo, fixture.statementNo, fixture.statementNo,
		fixture.statementNo); err != nil {
		t.Fatalf("load credit late-fee facts: %v", err)
	}
	if facts.StatementFee != 35 || facts.StatementLines != 2 ||
		facts.LateFeeLines != 1 || facts.LateFeeMovements != 1 ||
		facts.FeeAllocations != 1 {
		t.Fatalf("unexpected persisted credit late-fee facts: %+v", facts)
	}

	expectCreditTransactionRejected(t, ctx, database,
		"credit statement fee without an immutable late-fee line", func(ctx context.Context) error {
			_, updateErr := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_statement
SET line_count = line_count + 1, fee_minor = fee_minor + 1,
    total_amount_minor = total_amount_minor + 1,
    outstanding_amount_minor = outstanding_amount_minor + 1,
    version = version + 1
WHERE statement_no = ?`, fixture.statementNo)
			return updateErr
		})
}

func TestCreditBadDebtServiceConsumesDualReviewAndLedgerEvidenceAtomically(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	fixture := prepareCreditDelinquencyFixture(t, ctx, database, 5, "writeoff")
	delinquencyService := newCreditDelinquencyServiceForTest(t, database)
	badDebtService, err := NewCreditBadDebtService(database)
	if err != nil {
		t.Fatalf("create credit bad debt service: %v", err)
	}
	opened, err := delinquencyService.OpenDueCreditStatement(ctx, fixture.statementNo)
	if err != nil {
		t.Fatalf("open write-off delinquency: %v", err)
	}
	if _, err = delinquencyService.EscalateCreditDelinquency(ctx, opened.CaseNo); err != nil {
		t.Fatalf("freeze write-off delinquency: %v", err)
	}

	if _, err = badDebtService.WriteOffCreditStatement(ctx, CreditWriteoffInput{
		WriteoffNo: "credit-writeoff-same-reviewer", StatementNo: fixture.statementNo,
		ExpectedOutstandingMinor: 1_000, LedgerTransactionNo: "ledger-placeholder",
		ReasonCode: "credit.bad_debt", RequestedBy: 901, ReviewedBy: 901,
		ApprovalEvidenceDigest: strings.Repeat("b", 64),
	}); err == nil {
		t.Fatal("credit write-off accepted the requester as reviewer")
	}
	wrongWriteoffNo := "credit-writeoff-wrong-0001"
	wrongLedger := postCreditWriteoffLedger(t, ctx, database, 999, wrongWriteoffNo)
	if _, err = badDebtService.WriteOffCreditStatement(ctx, CreditWriteoffInput{
		WriteoffNo: wrongWriteoffNo, StatementNo: fixture.statementNo,
		ExpectedOutstandingMinor: 1_000, LedgerTransactionNo: wrongLedger,
		ReasonCode: "credit.bad_debt", RequestedBy: 901, ReviewedBy: 902,
		ApprovalEvidenceDigest: strings.Repeat("b", 64),
	}); err == nil {
		t.Fatal("credit write-off accepted mismatched ledger amount")
	}

	writeoffNo := "credit-writeoff-service-0001"
	ledgerTransactionNo := postCreditWriteoffLedger(t, ctx, database, 1_000, writeoffNo)
	input := CreditWriteoffInput{
		WriteoffNo: writeoffNo, StatementNo: fixture.statementNo,
		ExpectedOutstandingMinor: 1_000, LedgerTransactionNo: ledgerTransactionNo,
		ReasonCode: "credit.bad_debt", RequestedBy: 901, ReviewedBy: 902,
		ApprovalEvidenceDigest: strings.Repeat("c", 64),
	}
	writtenOff, err := badDebtService.WriteOffCreditStatement(ctx, input)
	if err != nil {
		t.Fatalf("write off credit statement: %v", err)
	}
	if writtenOff.Reused || writtenOff.StatementStatusCode != "written_off" ||
		writtenOff.StatementOutstandingMinor != 0 ||
		writtenOff.AccountStatusCode != "bad_debt" ||
		writtenOff.AccountAvailableMinor != 7_000 || writtenOff.AccountBilledMinor != 0 ||
		writtenOff.AccountDelinquentMinor != 0 ||
		writtenOff.AccountWrittenOffMinor != 1_000 || writtenOff.AccountVersion != 6 {
		t.Fatalf("unexpected credit write-off: %+v", writtenOff)
	}
	replayed, err := badDebtService.WriteOffCreditStatement(ctx, input)
	if err != nil || !replayed.Reused || replayed.WriteoffNo != writtenOff.WriteoffNo ||
		replayed.AccountVersion != writtenOff.AccountVersion {
		t.Fatalf("credit write-off replay=%+v err=%v", replayed, err)
	}
	changed := input
	changed.ApprovalEvidenceDigest = strings.Repeat("d", 64)
	if _, err = badDebtService.WriteOffCreditStatement(ctx, changed); err == nil {
		t.Fatal("credit write-off replay accepted changed approval evidence")
	}

	var facts struct {
		CaseStatus       string `orm:"case_status"`
		ExposureStatus   string `orm:"exposure_status"`
		ExposureBilled   int64  `orm:"exposure_billed"`
		ExposureWriteoff int64  `orm:"exposure_writeoff"`
		WriteoffCount    int64  `orm:"writeoff_count"`
		MovementCount    int64  `orm:"movement_count"`
	}
	if err = database.GetScan(ctx, &facts, `
SELECT
    (SELECT status_code FROM plugin_idc_core_credit_delinquency_case
     WHERE case_no = ?) AS case_status,
    (SELECT status_code FROM plugin_idc_core_credit_exposure
     WHERE credit_account_no = ?) AS exposure_status,
    (SELECT billed_principal_minor FROM plugin_idc_core_credit_exposure
     WHERE credit_account_no = ?) AS exposure_billed,
    (SELECT written_off_principal_minor FROM plugin_idc_core_credit_exposure
     WHERE credit_account_no = ?) AS exposure_writeoff,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_writeoff
     WHERE writeoff_no = ?) AS writeoff_count,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement
     WHERE credit_account_no = ? AND movement_type_code = 'writeoff') AS movement_count`,
		opened.CaseNo, fixture.accountNo, fixture.accountNo, fixture.accountNo,
		writeoffNo, fixture.accountNo); err != nil {
		t.Fatalf("load credit write-off facts: %v", err)
	}
	if facts.CaseStatus != "written_off" || facts.ExposureStatus != "written_off" ||
		facts.ExposureBilled != 0 || facts.ExposureWriteoff != 1_000 ||
		facts.WriteoffCount != 1 || facts.MovementCount != 1 {
		t.Fatalf("unexpected persisted credit write-off facts: %+v", facts)
	}

	ordinaryReference := "credit-writeoff-ordinary-repayment"
	ordinaryTransaction := postCreditRepaymentLedger(t, ctx, database, 1, ordinaryReference)
	usageService, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create credit usage service for written-off statement: %v", err)
	}
	if _, err = usageService.ApplyConfirmedCreditStatementRepayment(
		ctx, CreditStatementRepaymentInput{
			StatementNo: fixture.statementNo, FundingSourceType: "online_payment",
			FundingSourceReference: ordinaryReference,
			LedgerTransactionNo:    ordinaryTransaction, AmountMinor: 1,
			SourceSnapshotDigest:       strings.Repeat("1", 64),
			ConfirmationEvidenceDigest: strings.Repeat("2", 64),
		},
	); err == nil {
		t.Fatal("ordinary credit repayment accepted a written-off statement")
	}

	firstRecoveryReference := "credit-bad-debt-recovery-0001"
	firstRecoveryTransaction := postCreditRecoveryLedger(
		t, ctx, database, 400, firstRecoveryReference,
	)
	firstRecoveryInput := CreditStatementRepaymentInput{
		StatementNo: fixture.statementNo, FundingSourceType: "online_payment",
		FundingSourceReference: firstRecoveryReference,
		LedgerTransactionNo:    firstRecoveryTransaction, AmountMinor: 400,
		SourceSnapshotDigest:       strings.Repeat("3", 64),
		ConfirmationEvidenceDigest: strings.Repeat("4", 64),
	}
	partialRecovery, err := badDebtService.ApplyConfirmedBadDebtRecovery(ctx, firstRecoveryInput)
	if err != nil {
		t.Fatalf("apply partial bad debt recovery: %v", err)
	}
	if partialRecovery.Reused || partialRecovery.RepaymentType != "bad_debt_recovery" ||
		partialRecovery.AccountStatusCode != "bad_debt" ||
		partialRecovery.AccountAvailableMinor != 7_400 ||
		partialRecovery.AccountWrittenOffMinor != 600 ||
		partialRecovery.AccountVersion != 7 ||
		partialRecovery.StatementStatusCode != "written_off" {
		t.Fatalf("unexpected partial bad debt recovery: %+v", partialRecovery)
	}
	replayedRecovery, err := badDebtService.ApplyConfirmedBadDebtRecovery(ctx, firstRecoveryInput)
	if err != nil || !replayedRecovery.Reused ||
		replayedRecovery.RepaymentNo != partialRecovery.RepaymentNo ||
		replayedRecovery.AccountVersion != partialRecovery.AccountVersion {
		t.Fatalf("bad debt recovery replay=%+v err=%v", replayedRecovery, err)
	}
	changedRecovery := firstRecoveryInput
	changedRecovery.ConfirmationEvidenceDigest = strings.Repeat("5", 64)
	if _, err = badDebtService.ApplyConfirmedBadDebtRecovery(ctx, changedRecovery); err == nil {
		t.Fatal("bad debt recovery replay accepted changed evidence")
	}
	assertCreditDelinquencyProjection(t, ctx, database, opened.CaseNo,
		"written_off", 600, 600)

	secondRecoveryReference := "credit-bad-debt-recovery-0002"
	secondRecoveryTransaction := postCreditRecoveryLedger(
		t, ctx, database, 600, secondRecoveryReference,
	)
	fullRecovery, err := badDebtService.ApplyConfirmedBadDebtRecovery(
		ctx, CreditStatementRepaymentInput{
			StatementNo: fixture.statementNo, FundingSourceType: "online_payment",
			FundingSourceReference: secondRecoveryReference,
			LedgerTransactionNo:    secondRecoveryTransaction, AmountMinor: 600,
			SourceSnapshotDigest:       strings.Repeat("6", 64),
			ConfirmationEvidenceDigest: strings.Repeat("7", 64),
		},
	)
	if err != nil {
		t.Fatalf("apply full bad debt recovery: %v", err)
	}
	if fullRecovery.AccountStatusCode != "bad_debt" ||
		fullRecovery.AccountAvailableMinor != 8_000 ||
		fullRecovery.AccountWrittenOffMinor != 0 || fullRecovery.AccountVersion != 8 ||
		fullRecovery.StatementStatusCode != "written_off" {
		t.Fatalf("unexpected full bad debt recovery: %+v", fullRecovery)
	}
	assertCreditDelinquencyProjection(t, ctx, database, opened.CaseNo, "resolved", 0, 0)

	var recoveryFacts struct {
		AccountStatus      string `orm:"account_status"`
		FreezeReason       string `orm:"freeze_reason"`
		ExposureStatus     string `orm:"exposure_status"`
		ExposureRepaid     int64  `orm:"exposure_repaid"`
		ExposureWrittenOff int64  `orm:"exposure_written_off"`
		RecoveryCount      int64  `orm:"recovery_count"`
		AllocationCount    int64  `orm:"allocation_count"`
		MovementCount      int64  `orm:"movement_count"`
		UnfreezeCount      int64  `orm:"unfreeze_count"`
	}
	if err = database.GetScan(ctx, &recoveryFacts, `
SELECT
    (SELECT status_code FROM plugin_idc_core_credit_account
     WHERE credit_account_no = ?) AS account_status,
    (SELECT COALESCE(freeze_reason_code, '')
     FROM plugin_idc_core_credit_account
     WHERE credit_account_no = ?) AS freeze_reason,
    (SELECT status_code FROM plugin_idc_core_credit_exposure
     WHERE credit_account_no = ?) AS exposure_status,
    (SELECT repaid_principal_minor FROM plugin_idc_core_credit_exposure
     WHERE credit_account_no = ?) AS exposure_repaid,
    (SELECT written_off_principal_minor FROM plugin_idc_core_credit_exposure
     WHERE credit_account_no = ?) AS exposure_written_off,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_repayment
     WHERE credit_account_no = ? AND repayment_type_code = 'bad_debt_recovery') AS recovery_count,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_repayment_allocation allocation
     JOIN plugin_idc_core_credit_repayment repayment
       ON repayment.repayment_no = allocation.repayment_no
     WHERE repayment.credit_account_no = ?
       AND allocation.allocation_type_code = 'written_off_recovery') AS allocation_count,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement
     WHERE credit_account_no = ? AND movement_type_code = 'recovery') AS movement_count,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement
     WHERE credit_account_no = ? AND movement_type_code = 'unfreeze') AS unfreeze_count`,
		fixture.accountNo, fixture.accountNo, fixture.accountNo, fixture.accountNo,
		fixture.accountNo, fixture.accountNo, fixture.accountNo, fixture.accountNo,
		fixture.accountNo); err != nil {
		t.Fatalf("load bad debt recovery facts: %v", err)
	}
	if recoveryFacts.AccountStatus != "bad_debt" ||
		recoveryFacts.FreezeReason != "credit.bad_debt" ||
		recoveryFacts.ExposureStatus != "settled" || recoveryFacts.ExposureRepaid != 1_000 ||
		recoveryFacts.ExposureWrittenOff != 0 || recoveryFacts.RecoveryCount != 2 ||
		recoveryFacts.AllocationCount != 2 || recoveryFacts.MovementCount != 2 ||
		recoveryFacts.UnfreezeCount != 0 {
		t.Fatalf("unexpected persisted bad debt recovery facts: %+v", recoveryFacts)
	}

	restorationInput := CreditRestorationInput{
		CreditAccountNo: fixture.accountNo, ExpectedVersion: 7,
		SourceReference: "credit-restoration-0001",
		ReasonCode:      "credit.recovery_reviewed",
		EvidenceDigest:  strings.Repeat("8", 64), ActorID: 903,
	}
	if _, err = badDebtService.RestoreCreditAccount(ctx, restorationInput); err == nil {
		t.Fatal("credit account restoration accepted a stale expected version")
	}
	restorationInput.ExpectedVersion = 8
	restored, err := badDebtService.RestoreCreditAccount(ctx, restorationInput)
	if err != nil {
		t.Fatalf("restore recovered credit account: %v", err)
	}
	if restored.Reused || restored.PreviousStatusCode != "bad_debt" ||
		restored.RestoredStatusCode != "active" || restored.CurrentStatusCode != "active" ||
		restored.AccountAvailableMinor != 8_000 || restored.AccountDelinquentMinor != 0 ||
		restored.AccountWrittenOffMinor != 0 || restored.AccountVersion != 9 {
		t.Fatalf("unexpected restored credit account: %+v", restored)
	}
	replayedRestoration, err := badDebtService.RestoreCreditAccount(ctx, restorationInput)
	if err != nil || !replayedRestoration.Reused ||
		replayedRestoration.RestoredStatusCode != "active" ||
		replayedRestoration.AccountVersion != restored.AccountVersion {
		t.Fatalf("credit restoration replay=%+v err=%v", replayedRestoration, err)
	}
	changedRestoration := restorationInput
	changedRestoration.ReasonCode = "credit.operator_override"
	if _, err = badDebtService.RestoreCreditAccount(ctx, changedRestoration); err == nil {
		t.Fatal("credit restoration replay accepted a changed reason")
	}

	var restoredFacts struct {
		StatusCode    string `orm:"status_code"`
		FreezeReason  string `orm:"freeze_reason"`
		Frozen        bool   `orm:"frozen"`
		UnfreezeCount int64  `orm:"unfreeze_count"`
	}
	if err = database.GetScan(ctx, &restoredFacts, `
SELECT account.status_code,
       COALESCE(account.freeze_reason_code, '') AS freeze_reason,
       account.frozen_at IS NOT NULL AS frozen,
       (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement movement
        WHERE movement.credit_account_no = account.credit_account_no
          AND movement.movement_type_code = 'unfreeze') AS unfreeze_count
FROM plugin_idc_core_credit_account account
WHERE account.credit_account_no = ?`, fixture.accountNo); err != nil {
		t.Fatalf("load restored credit account facts: %v", err)
	}
	if restoredFacts.StatusCode != "active" || restoredFacts.FreezeReason != "" ||
		restoredFacts.Frozen || restoredFacts.UnfreezeCount != 1 {
		t.Fatalf("unexpected persisted credit restoration facts: %+v", restoredFacts)
	}
}

type creditDelinquencyFixture struct {
	accountNo    string
	statementNo  string
	paymentDueAt time.Time
}

func prepareCreditDelinquencyFixture(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	dueDaysAgo int,
	suffix string,
) creditDelinquencyFixture {
	return prepareCreditDelinquencyFixtureWithLateFee(
		t, ctx, database, dueDaysAgo, suffix, 0, 0,
	)
}

func prepareCreditDelinquencyFixtureWithLateFee(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	dueDaysAgo int,
	suffix string,
	dailyLateFeeBasisPoints int,
	maximumLateFeeMinor int64,
) creditDelinquencyFixture {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	account := activateCreditAccountForUsageTestWithPolicy(
		t, ctx, database, "customer-ledger", 8_000, "delinquency-"+suffix,
		now.AddDate(0, 0, -90), 10, 3,
		dailyLateFeeBasisPoints, maximumLateFeeMinor,
	)
	usageService, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create fixture credit usage service: %v", err)
	}
	exposure, err := usageService.ReserveCredit(ctx, CreditReserveInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1_000,
		SourceType: "manual", SourceReference: "credit-delinquency-exposure-" + suffix,
		SourceSnapshotDigest: strings.Repeat("1", 64), ActorType: "system", ActorID: 0,
	})
	if err != nil {
		t.Fatalf("reserve delinquency fixture exposure: %v", err)
	}
	statementNo := "credit-delinquency-statement-" + suffix
	paymentDueAt := now.AddDate(0, 0, -dueDaysAgo)
	issuedAt := paymentDueAt.AddDate(0, 0, -10)
	periodEnd := issuedAt.Add(-time.Hour)
	periodStart := periodEnd.AddDate(0, -1, 0)
	mustCreditTransaction(t, ctx, database, "seed due credit statement", func(ctx context.Context) error {
		if _, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_statement (
    statement_no, credit_account_no, customer_no, currency_code, terms_no,
    terms_version, period_started_at, period_ended_at, line_count,
    principal_minor, total_amount_minor, outstanding_amount_minor,
    status_code, content_digest, issued_at, payment_due_at
) SELECT ?, account.credit_account_no, account.customer_no, account.currency_code,
         account.current_terms_no, account.current_terms_version, ?, ?, 1,
         1000, 1000, 1000, 'open', ?, ?, ?
  FROM plugin_idc_core_credit_account account
 WHERE account.credit_account_no = ?`, statementNo, periodStart, periodEnd,
			strings.Repeat("2", 64), issuedAt, paymentDueAt,
			account.CreditAccountNo); execErr != nil {
			return execErr
		}
		lineNo := deterministicCreditNo("crl_", "delinquency", statementNo)
		if _, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_statement_line (
    statement_line_no, statement_no, credit_account_no, customer_no,
    currency_code, line_number, line_type_code, exposure_no,
    source_type_code, source_reference, amount_minor, content_digest
) VALUES (?, ?, ?, 'customer-ledger', 'CNY', 1, 'principal', ?,
          'exposure', ?, 1000, ?)`, lineNo, statementNo, account.CreditAccountNo,
			exposure.ExposureNo, exposure.ExposureNo, strings.Repeat("3", 64)); execErr != nil {
			return execErr
		}
		if _, execErr := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_exposure
SET unbilled_principal_minor = 0, billed_principal_minor = 1000,
    status_code = 'billed', version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE exposure_no = ? AND version = 1`, exposure.ExposureNo); execErr != nil {
			return execErr
		}
		if _, execErr := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET unbilled_principal_minor = 0, billed_principal_minor = 1000,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE credit_account_no = ? AND version = 2`, account.CreditAccountNo); execErr != nil {
			return execErr
		}
		movementNo := deterministicCreditNo("crm_", "bill", statementNo)
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_account_movement (
    movement_no, credit_account_no, customer_no, currency_code, exposure_no,
    statement_no, movement_type_code, amount_minor, account_version_before,
    account_version_after, status_before_code, status_after_code,
    total_limit_before_minor, total_limit_after_minor, available_before_minor,
    available_after_minor, unbilled_before_minor, unbilled_after_minor,
    billed_before_minor, billed_after_minor, delinquent_before_minor,
    delinquent_after_minor, written_off_before_minor, written_off_after_minor,
    source_type_code, source_reference, idempotency_digest, evidence_digest,
    content_digest, actor_type_code, actor_id, occurred_at
) VALUES (?, ?, 'customer-ledger', 'CNY', ?, ?, 'bill', 1000, 2, 3,
          'active', 'active', 8000, 8000, 7000, 7000, 1000, 0,
          0, 1000, 0, 0, 0, 0, 'statement', ?, ?, ?, ?, 'system', 0, ?)`,
			movementNo, account.CreditAccountNo, exposure.ExposureNo, statementNo,
			statementNo, strings.Repeat("4", 64), strings.Repeat("5", 64),
			strings.Repeat("6", 64), issuedAt)
		return execErr
	})
	return creditDelinquencyFixture{
		accountNo: account.CreditAccountNo, statementNo: statementNo,
		paymentDueAt: paymentDueAt,
	}
}

func newCreditDelinquencyServiceForTest(
	t *testing.T,
	database gdb.DB,
) CreditDelinquencyService {
	t.Helper()
	tasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		t.Fatalf("create asynchronous task service: %v", err)
	}
	service, err := NewCreditDelinquencyService(database, tasks)
	if err != nil {
		t.Fatalf("create credit delinquency service: %v", err)
	}
	return service
}

func assertCreditServiceActionTask(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	caseNo string,
	taskNo string,
) {
	t.Helper()
	var facts struct {
		TaskCount      int64  `orm:"task_count"`
		TaskType       string `orm:"task_type"`
		TaskStatus     string `orm:"task_status"`
		Payload        string `orm:"payload"`
		OperationCount int64  `orm:"operation_count"`
	}
	if err := database.GetScan(ctx, &facts, `
SELECT
    (SELECT COUNT(*) FROM plugin_idc_core_async_task
     WHERE task_no = ?) AS task_count,
    (SELECT task_type FROM plugin_idc_core_async_task
     WHERE task_no = ?) AS task_type,
    (SELECT status_code FROM plugin_idc_core_async_task
     WHERE task_no = ?) AS task_status,
    (SELECT payload::text FROM plugin_idc_core_async_task
     WHERE task_no = ?) AS payload,
    (SELECT COUNT(*) FROM plugin_idc_core_business_operation) AS operation_count`,
		taskNo, taskNo, taskNo, taskNo); err != nil {
		t.Fatalf("load credit service action task: %v", err)
	}
	if facts.TaskCount != 1 || facts.TaskType != creditServiceActionTaskType ||
		facts.TaskStatus != "pending" || facts.OperationCount != 0 {
		t.Fatalf("unexpected credit service action task facts: %+v", facts)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(facts.Payload), &payload); err != nil {
		t.Fatalf("decode credit service action payload: %v", err)
	}
	if len(payload) != 5 || payload["CaseNo"] != caseNo ||
		payload["StatementNo"] == "" || payload["CreditAccountNo"] == "" ||
		payload["CustomerNo"] != "customer-ledger" || payload["CurrencyCode"] != "CNY" {
		t.Fatalf("unexpected redacted service action payload: %v", payload)
	}
}

func assertCreditDelinquencyProjection(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	caseNo string,
	status string,
	principalMinor int64,
	outstandingMinor int64,
) {
	t.Helper()
	var facts struct {
		StatusCode       string `orm:"status_code"`
		PrincipalMinor   int64  `orm:"principal_minor"`
		LateFeeMinor     int64  `orm:"late_fee_minor"`
		OutstandingMinor int64  `orm:"outstanding_amount_minor"`
		Resolved         bool   `orm:"resolved"`
	}
	if err := database.GetScan(ctx, &facts, `
SELECT status_code, principal_minor, late_fee_minor, outstanding_amount_minor,
       resolved_at IS NOT NULL AS resolved
FROM plugin_idc_core_credit_delinquency_case
WHERE case_no = ?`, caseNo); err != nil {
		t.Fatalf("load credit delinquency projection: %v", err)
	}
	if facts.StatusCode != status || facts.PrincipalMinor != principalMinor ||
		facts.LateFeeMinor != 0 || facts.OutstandingMinor != outstandingMinor ||
		facts.Resolved != (status == "resolved") {
		t.Fatalf("unexpected credit delinquency projection: %+v", facts)
	}
}
