package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const maximumLateFeeAccrualDays int64 = 10_000

type creditLateFeeAssessment struct {
	FeeMinor        int64
	SourceReference string
	LineNumber      int
	LineNo          string
	LineDigest      string
}

func (s *creditDelinquencyServiceImpl) AccrueCreditLateFee(
	ctx context.Context,
	caseNo string,
) (*CreditDelinquency, error) {
	if !exactlyTrimmed(caseNo) || !validBusinessReference(caseNo, 40) {
		return nil, bizerr.NewCode(CodeCreditInvalid)
	}
	var result *CreditDelinquency
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		identity, loadErr := loadCreditDelinquencyByCase(
			ctx, s.database, caseNo, false,
		)
		if loadErr != nil {
			return loadErr
		}
		if identity == nil {
			return bizerr.NewCode(CodeCreditNotFound)
		}
		account, loadErr := loadCreditAccountByNo(
			ctx, s.database, identity.CreditAccountNo, true,
		)
		if loadErr != nil {
			return loadErr
		}
		statement, loadErr := loadCreditStatementForUpdate(
			ctx, s.database, identity.StatementNo,
		)
		if loadErr != nil {
			return loadErr
		}
		caseRow, loadErr := loadCreditDelinquencyByCase(
			ctx, s.database, caseNo, true,
		)
		if loadErr != nil {
			return loadErr
		}
		if account == nil || statement == nil || caseRow == nil ||
			!validCreditAccountProjection(account) {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		_, reused, accrueErr := s.accrueCreditLateFeeLocked(
			ctx, account, statement, caseRow,
		)
		if accrueErr != nil {
			return accrueErr
		}
		result = projectCreditDelinquency(caseRow, account, "", reused)
		return nil
	})
	return creditDelinquencyResult(result, err)
}

func (s *serviceImpl) accrueCreditLateFeeLocked(
	ctx context.Context,
	account *creditAccountRow,
	statement *creditStatementRow,
	caseRow *creditDelinquencyRow,
) (int64, bool, error) {
	if !validCreditLateFeeTarget(account, statement, caseRow) {
		return 0, false, bizerr.NewCode(CodeCreditConflict)
	}
	terms, err := loadCreditTerms(
		ctx, s.database, account.CreditAccountNo, statement.TermsNo,
		statement.TermsVersion,
	)
	if err != nil {
		return 0, false, err
	}
	if terms == nil || terms.DailyLateFeeBasisPoints < 0 ||
		terms.MaximumLateFeeMinor < 0 {
		return 0, false, bizerr.NewCode(CodeCreditIntegrity)
	}
	if terms.DailyLateFeeBasisPoints == 0 && terms.MaximumLateFeeMinor == 0 {
		return 0, true, nil
	}
	allocation, err := loadCreditOutstandingAllocation(
		ctx, s.database, statement.StatementNo,
	)
	if err != nil {
		return 0, false, err
	}
	if statement.PrincipalMinor-allocation.PrincipalPaidMinor != caseRow.PrincipalMinor ||
		statement.FeeMinor-allocation.FeePaidMinor != caseRow.LateFeeMinor ||
		statement.FeeMinor > terms.MaximumLateFeeMinor {
		return 0, false, bizerr.NewCode(CodeCreditIntegrity)
	}
	throughDay := int64(account.Clock.Sub(statement.PaymentDueAt) / (24 * time.Hour))
	if throughDay <= 0 || statement.FeeMinor == terms.MaximumLateFeeMinor ||
		caseRow.PrincipalMinor == 0 {
		return 0, true, nil
	}
	if throughDay > maximumLateFeeAccrualDays {
		return 0, false, bizerr.NewCode(CodeCreditIntegrity)
	}
	lastDay, err := loadLastCreditLateFeeDay(ctx, s.database, statement.StatementNo)
	if err != nil {
		return 0, false, err
	}
	if lastDay < 0 || lastDay > throughDay {
		return 0, false, bizerr.NewCode(CodeCreditIntegrity)
	}
	if lastDay == throughDay {
		return 0, true, nil
	}
	dailyFee := calculateDailyCreditLateFee(
		caseRow.PrincipalMinor, terms.DailyLateFeeBasisPoints,
	)
	remainingCap := terms.MaximumLateFeeMinor - statement.FeeMinor
	feeMinor := multiplyCreditLateFeeWithinCap(dailyFee, throughDay-lastDay, remainingCap)
	if feeMinor <= 0 {
		return 0, true, nil
	}
	if !canApplyCreditLateFeeAmount(statement, caseRow, feeMinor) {
		return 0, false, bizerr.NewCode(CodeCreditIntegrity)
	}
	sourceReference := fmt.Sprintf("late-fee-day-%010d", throughDay)
	lineNumber := statement.LineCount + 1
	lineNo := deterministicCreditNo(
		"crl_", "late-fee", statement.StatementNo, sourceReference,
	)
	lineDigest, err := contentDigest(struct {
		StatementNo, LineNo, SourceReference string
		LineNumber                           int
		ThroughDay, FeeMinor                 int64
		BasisPoints                          int
	}{
		statement.StatementNo, lineNo, sourceReference, lineNumber,
		throughDay, feeMinor, terms.DailyLateFeeBasisPoints,
	})
	if err != nil {
		return 0, false, err
	}
	return s.persistCreditLateFee(ctx, account, statement, caseRow,
		creditLateFeeAssessment{
			FeeMinor: feeMinor, SourceReference: sourceReference,
			LineNumber: lineNumber, LineNo: lineNo, LineDigest: lineDigest,
		})
}

func validCreditLateFeeTarget(
	account *creditAccountRow,
	statement *creditStatementRow,
	caseRow *creditDelinquencyRow,
) bool {
	if account == nil || statement == nil || caseRow == nil {
		return false
	}
	caseOpen := caseRow.StatusCode == "grace_period" ||
		caseRow.StatusCode == "frozen" || caseRow.StatusCode == "service_action_pending"
	return statement.CreditAccountNo == account.CreditAccountNo &&
		caseRow.CreditAccountNo == account.CreditAccountNo &&
		caseRow.StatementNo == statement.StatementNo && statement.StatusCode == "overdue" &&
		caseOpen && statement.OutstandingMinor == caseRow.OutstandingAmountMinor
}

func canApplyCreditLateFeeAmount(
	statement *creditStatementRow,
	caseRow *creditDelinquencyRow,
	feeMinor int64,
) bool {
	return statement.LineCount < maximumCreditStatementLines &&
		statement.TotalAmountMinor <= maximumAmountMinor-feeMinor &&
		statement.OutstandingMinor <= maximumAmountMinor-feeMinor &&
		caseRow.LateFeeMinor <= maximumAmountMinor-feeMinor &&
		caseRow.OutstandingAmountMinor <= maximumAmountMinor-feeMinor
}

func (s *serviceImpl) persistCreditLateFee(
	ctx context.Context,
	account *creditAccountRow,
	statement *creditStatementRow,
	caseRow *creditDelinquencyRow,
	assessment creditLateFeeAssessment,
) (int64, bool, error) {
	if _, err := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_statement_line (
    tenant_id, statement_line_no, statement_no, credit_account_no,
    customer_no, currency_code, line_number, line_type_code,
    source_type_code, source_reference, amount_minor, content_digest
) VALUES (0, ?, ?, ?, ?, ?, ?, 'late_fee', 'late_fee', ?, ?, ?)`,
		assessment.LineNo, statement.StatementNo, account.CreditAccountNo,
		account.CustomerNo, account.CurrencyCode, assessment.LineNumber,
		assessment.SourceReference, assessment.FeeMinor,
		assessment.LineDigest); err != nil {
		return 0, false, gerror.Wrap(err, "append credit late fee line")
	}
	updatedStatement, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_statement
SET line_count = line_count + 1, fee_minor = fee_minor + ?,
    total_amount_minor = total_amount_minor + ?,
    outstanding_amount_minor = outstanding_amount_minor + ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND statement_no = ? AND version = ?`,
		assessment.FeeMinor, assessment.FeeMinor, assessment.FeeMinor,
		statement.StatementNo, statement.Version)
	if err != nil {
		return 0, false, gerror.Wrap(err, "accrue credit late fee to statement")
	}
	if err = requireSingleCreditUpdate(updatedStatement); err != nil {
		return 0, false, err
	}
	updatedCase, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_delinquency_case
SET late_fee_minor = late_fee_minor + ?,
    outstanding_amount_minor = outstanding_amount_minor + ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND case_no = ? AND version = ?`,
		assessment.FeeMinor, assessment.FeeMinor, caseRow.CaseNo, caseRow.Version)
	if err != nil {
		return 0, false, gerror.Wrap(err, "accrue credit late fee to delinquency")
	}
	if err = requireSingleCreditUpdate(updatedCase); err != nil {
		return 0, false, err
	}
	updatedAccount, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND credit_account_no = ? AND version = ?`,
		account.CreditAccountNo, account.Version)
	if err != nil {
		return 0, false, gerror.Wrap(err, "advance credit account for late fee")
	}
	if err = requireSingleCreditUpdate(updatedAccount); err != nil {
		return 0, false, err
	}
	if err = s.appendCreditMovement(ctx, creditMovementInput{
		MovementNo: deterministicCreditNo(
			"crm_", "late-fee", statement.StatementNo, assessment.SourceReference,
		),
		Account: account, StatementNo: statement.StatementNo,
		MovementType: "late_fee", AmountMinor: assessment.FeeMinor,
		StatusAfter: account.StatusCode, AvailableAfter: account.AvailableLimitMinor,
		UnbilledAfter:   account.UnbilledPrincipalMinor,
		BilledAfter:     account.BilledPrincipalMinor,
		DelinquentAfter: account.DelinquentPrincipalMinor,
		WrittenOffAfter: account.WrittenOffPrincipalMinor,
		SourceType:      "late_fee", SourceReference: assessment.SourceReference,
		EvidenceDigest: assessment.LineDigest, ActorType: "system", ActorID: 0,
		OccurredAt: account.Clock,
	}); err != nil {
		return 0, false, err
	}
	statement.LineCount++
	statement.FeeMinor += assessment.FeeMinor
	statement.TotalAmountMinor += assessment.FeeMinor
	statement.OutstandingMinor += assessment.FeeMinor
	statement.Version++
	caseRow.LateFeeMinor += assessment.FeeMinor
	caseRow.OutstandingAmountMinor += assessment.FeeMinor
	caseRow.Version++
	account.Version++
	return assessment.FeeMinor, false, nil
}

func loadLastCreditLateFeeDay(
	ctx context.Context,
	database gdb.DB,
	statementNo string,
) (int64, error) {
	value, err := database.GetValue(ctx, `
SELECT COALESCE(MAX(substring(source_reference FROM '[0-9]{10}$')::BIGINT), 0)
FROM plugin_idc_core_credit_statement_line
WHERE tenant_id = 0 AND statement_no = ? AND line_type_code = 'late_fee'
  AND source_type_code = 'late_fee'
  AND source_reference ~ '^late-fee-day-[0-9]{10}$'`, statementNo)
	if err != nil {
		return 0, gerror.Wrap(err, "load last credit late fee day")
	}
	return value.Int64(), nil
}

func calculateDailyCreditLateFee(principalMinor int64, basisPoints int) int64 {
	if principalMinor <= 0 || basisPoints <= 0 {
		return 0
	}
	whole := (principalMinor / 10_000) * int64(basisPoints)
	remainder := (principalMinor % 10_000) * int64(basisPoints)
	return whole + (remainder+9_999)/10_000
}

func multiplyCreditLateFeeWithinCap(dailyFee int64, days int64, capMinor int64) int64 {
	if dailyFee <= 0 || days <= 0 || capMinor <= 0 {
		return 0
	}
	if dailyFee > capMinor/days {
		return capMinor
	}
	fee := dailyFee * days
	if fee > capMinor {
		return capMinor
	}
	return fee
}
