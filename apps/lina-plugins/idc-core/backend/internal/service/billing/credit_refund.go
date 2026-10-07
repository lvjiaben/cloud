// This file owns credit-purchase refund planning and atomic posting. It keeps
// unpaid receivable reductions separate from real-money refunds and preserves
// the exact original repayment allocation for every returned amount.

package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const maximumCreditRefundSources = 1_000

type creditRefundFundingRow struct {
	AllocationNo           string `orm:"allocation_no"`
	RepaymentNo            string `orm:"repayment_no"`
	AllocationType         string `orm:"allocation_type_code"`
	StatementNo            string `orm:"statement_no"`
	FundingSourceType      string `orm:"funding_source_type_code"`
	FundingSourceReference string `orm:"funding_source_reference"`
	RemainingAmountMinor   int64  `orm:"remaining_amount_minor"`
}

type creditRefundMovementRow struct {
	ExposureNo     string `orm:"exposure_no"`
	AmountMinor    int64  `orm:"amount_minor"`
	EvidenceDigest string `orm:"evidence_digest"`
}

// PreviewCreditRefund derives one bounded current-state plan without changing
// the credit account or creating a cash fact.
func (s *serviceImpl) PreviewCreditRefund(
	ctx context.Context,
	exposureNo string,
	amountMinor int64,
) (*CreditRefundPlan, error) {
	exposureNo = strings.TrimSpace(exposureNo)
	if !validBusinessReference(exposureNo, 40) || amountMinor <= 0 ||
		amountMinor > maximumAmountMinor {
		return nil, bizerr.NewCode(CodeCreditInvalid)
	}
	exposure, err := loadCreditExposureByNo(ctx, s.database, exposureNo, false)
	if err != nil {
		return nil, creditRefundError(err)
	}
	if exposure == nil {
		return nil, bizerr.NewCode(CodeCreditNotFound)
	}
	plan, err := s.buildCreditRefundPlan(ctx, exposure, amountMinor, false)
	return plan, creditRefundError(err)
}

// ApplyConfirmedCreditRefund commits an unchanged plan after every real-money
// source in that plan has authoritative success evidence.
func (s *serviceImpl) ApplyConfirmedCreditRefund(
	ctx context.Context,
	input CreditRefundInput,
) (*CreditRefund, error) {
	normalized, err := normalizeCreditRefundInput(input)
	if err != nil {
		return nil, err
	}
	var result *CreditRefund
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		identity, loadErr := loadCreditExposureByNo(ctx, s.database, normalized.ExposureNo, false)
		if loadErr != nil {
			return loadErr
		}
		if identity == nil {
			return bizerr.NewCode(CodeCreditNotFound)
		}
		account, loadErr := loadCreditAccountByNo(ctx, s.database, identity.CreditAccountNo, true)
		if loadErr != nil {
			return loadErr
		}
		if account == nil || !validCreditAccountProjection(account) {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		exposure, loadErr := loadCreditExposureByNo(ctx, s.database, normalized.ExposureNo, true)
		if loadErr != nil {
			return loadErr
		}
		if exposure == nil || exposure.CreditAccountNo != account.CreditAccountNo {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		existing, loadErr := loadCreditRefundMovement(
			ctx, s.database, account.CreditAccountNo, normalized.SourceReference,
		)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.ExposureNo != exposure.ExposureNo ||
				existing.AmountMinor != normalized.AmountMinor ||
				existing.EvidenceDigest != normalized.EvidenceDigest {
				return bizerr.NewCode(CodeCreditConflict)
			}
			result = projectCreditRefund(
				exposure, account, "", "", normalized.AmountMinor, 0, 0, 0, true,
			)
			return nil
		}
		plan, planErr := s.buildCreditRefundPlan(
			ctx, exposure, normalized.AmountMinor, true,
		)
		if planErr != nil {
			return planErr
		}
		if plan.PlanDigest != normalized.PlanDigest {
			return bizerr.NewCode(CodeCreditConflict)
		}
		fundingResults, resultErr := matchCreditRefundFundingResults(
			plan.FundingSources, normalized.FundingResults,
		)
		if resultErr != nil {
			return resultErr
		}
		for _, source := range plan.FundingSources {
			funding := fundingResults[source.RepaymentAllocationNo]
			if postErr := s.postCreditRefundFunding(
				ctx, account, exposure, source, funding,
			); postErr != nil {
				return postErr
			}
		}
		statementStatus, applyErr := s.applyCreditRefundProjection(
			ctx, account, exposure, plan, normalized,
		)
		if applyErr != nil {
			return applyErr
		}
		result = projectCreditRefund(
			exposure, account, plan.StatementNo, statementStatus,
			plan.AmountMinor, plan.UnbilledReleaseMinor,
			plan.BilledReductionMinor, plan.RepaidRefundMinor, false,
		)
		return nil
	})
	return result, creditRefundError(err)
}

func normalizeCreditRefundInput(input CreditRefundInput) (CreditRefundInput, error) {
	input.ExposureNo = strings.TrimSpace(input.ExposureNo)
	input.PlanDigest = strings.TrimSpace(input.PlanDigest)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	input.EvidenceDigest = strings.TrimSpace(input.EvidenceDigest)
	if !validBusinessReference(input.ExposureNo, 40) || input.AmountMinor <= 0 ||
		input.AmountMinor > maximumAmountMinor ||
		!canonicalDigestPattern.MatchString(input.PlanDigest) ||
		!canonicalDigestPattern.MatchString(input.EvidenceDigest) ||
		len(input.SourceReference) < 1 || len(input.SourceReference) > 128 ||
		(input.ActorType != "customer" && input.ActorType != "staff" &&
			input.ActorType != "system" && input.ActorType != "migration") ||
		(input.ActorType == "system" && input.ActorID != 0) ||
		(input.ActorType != "system" && input.ActorID <= 0) ||
		len(input.FundingResults) > maximumCreditRefundSources {
		return CreditRefundInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	seen := make(map[string]struct{}, len(input.FundingResults))
	for index := range input.FundingResults {
		result := &input.FundingResults[index]
		result.RepaymentAllocationNo = strings.TrimSpace(result.RepaymentAllocationNo)
		result.RefundSourceReference = strings.TrimSpace(result.RefundSourceReference)
		result.EvidenceDigest = strings.TrimSpace(result.EvidenceDigest)
		result.Destination = CreditRefundDestination(strings.TrimSpace(string(result.Destination)))
		if !validBusinessReference(result.RepaymentAllocationNo, 40) ||
			result.AmountMinor <= 0 || result.AmountMinor > maximumAmountMinor ||
			len(result.RefundSourceReference) < 1 || len(result.RefundSourceReference) > 128 ||
			!canonicalDigestPattern.MatchString(result.EvidenceDigest) ||
			(result.Destination != CreditRefundDestinationOriginalSource &&
				result.Destination != CreditRefundDestinationBalance) {
			return CreditRefundInput{}, bizerr.NewCode(CodeCreditInvalid)
		}
		if _, exists := seen[result.RepaymentAllocationNo]; exists {
			return CreditRefundInput{}, bizerr.NewCode(CodeCreditInvalid)
		}
		seen[result.RepaymentAllocationNo] = struct{}{}
	}
	return input, nil
}

func (s *serviceImpl) buildCreditRefundPlan(
	ctx context.Context,
	exposure *creditExposureRow,
	amountMinor int64,
	lock bool,
) (*CreditRefundPlan, error) {
	if exposure == nil || exposure.WrittenOffPrincipalMinor != 0 ||
		exposure.ReleasedPrincipalMinor < 0 || amountMinor <= 0 ||
		amountMinor > exposure.UnbilledPrincipalMinor+
			exposure.BilledPrincipalMinor+exposure.RepaidPrincipalMinor {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	remaining := amountMinor
	unbilled := minCreditAmount(remaining, exposure.UnbilledPrincipalMinor)
	remaining -= unbilled
	billed := minCreditAmount(remaining, exposure.BilledPrincipalMinor)
	remaining -= billed
	repaid := remaining
	fundingRows, err := s.loadCreditRefundFundingRows(ctx, exposure.ExposureNo, lock)
	if err != nil {
		return nil, err
	}
	netRepaid := int64(0)
	for _, row := range fundingRows {
		if row.RemainingAmountMinor <= 0 || netRepaid > maximumAmountMinor-row.RemainingAmountMinor {
			return nil, bizerr.NewCode(CodeCreditIntegrity)
		}
		netRepaid += row.RemainingAmountMinor
	}
	if netRepaid != exposure.RepaidPrincipalMinor || len(fundingRows) > maximumCreditRefundSources {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	statementNo, err := s.loadCreditRefundStatementNo(ctx, exposure)
	if err != nil {
		return nil, err
	}
	sources := make([]CreditRefundFundingSource, 0, len(fundingRows))
	remainingRepaid := repaid
	for _, row := range fundingRows {
		if remainingRepaid == 0 {
			break
		}
		amount := minCreditAmount(remainingRepaid, row.RemainingAmountMinor)
		sources = append(sources, CreditRefundFundingSource{
			RepaymentAllocationNo:  row.AllocationNo,
			RepaymentNo:            row.RepaymentNo,
			AllocationType:         row.AllocationType,
			FundingSourceType:      row.FundingSourceType,
			FundingSourceReference: row.FundingSourceReference,
			AmountMinor:            amount,
		})
		remainingRepaid -= amount
	}
	if remainingRepaid != 0 {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	plan := &CreditRefundPlan{
		ExposureNo: exposure.ExposureNo, CreditAccountNo: exposure.CreditAccountNo,
		CustomerNo: exposure.CustomerNo, CurrencyCode: exposure.CurrencyCode,
		StatementNo: statementNo, AmountMinor: amountMinor,
		UnbilledReleaseMinor: unbilled, BilledReductionMinor: billed,
		RepaidRefundMinor: repaid, FundingSources: sources,
	}
	plan.PlanDigest, err = contentDigest(struct {
		ExposureNo, CreditAccountNo, CustomerNo, CurrencyCode, StatementNo string
		AmountMinor, UnbilledMinor, BilledMinor, RepaidMinor               int64
		ExposureVersion                                                    int64
		Sources                                                            []CreditRefundFundingSource
	}{
		plan.ExposureNo, plan.CreditAccountNo, plan.CustomerNo,
		plan.CurrencyCode, plan.StatementNo, plan.AmountMinor,
		plan.UnbilledReleaseMinor, plan.BilledReductionMinor,
		plan.RepaidRefundMinor, exposure.Version, plan.FundingSources,
	})
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func (s *serviceImpl) loadCreditRefundFundingRows(
	ctx context.Context,
	exposureNo string,
	lock bool,
) ([]creditRefundFundingRow, error) {
	locking := ""
	if lock {
		locking = " FOR UPDATE OF allocation, repayment"
	}
	rows := make([]creditRefundFundingRow, 0, 16)
	err := s.database.GetScan(ctx, &rows, `
SELECT allocation.allocation_no,allocation.repayment_no,
       allocation.allocation_type_code,
       COALESCE(allocation.statement_no,'') AS statement_no,
       repayment.funding_source_type_code,
       repayment.funding_source_reference,
       allocation.amount_minor-COALESCE(reversed.amount_minor,0)
         AS remaining_amount_minor
FROM plugin_idc_core_credit_repayment_allocation allocation
JOIN plugin_idc_core_credit_repayment repayment
  ON repayment.repayment_no=allocation.repayment_no
LEFT JOIN LATERAL (
    SELECT SUM(reverse_allocation.amount_minor) AS amount_minor
    FROM plugin_idc_core_credit_repayment_allocation reverse_allocation
    WHERE reverse_allocation.original_allocation_no=allocation.allocation_no
      AND reverse_allocation.direction_code='reverse'
) reversed ON TRUE
WHERE allocation.tenant_id=0 AND allocation.exposure_no=?
  AND allocation.direction_code='apply'
  AND allocation.allocation_type_code IN ('unbilled_principal','statement_principal')
  AND allocation.amount_minor-COALESCE(reversed.amount_minor,0)>0
ORDER BY repayment.confirmed_at,repayment.id,allocation.id
LIMIT ?`+locking, exposureNo, maximumCreditRefundSources+1)
	if err != nil {
		return nil, gerror.Wrap(err, "load credit refund funding sources")
	}
	if len(rows) > maximumCreditRefundSources {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	return rows, nil
}

func (s *serviceImpl) loadCreditRefundStatementNo(
	ctx context.Context,
	exposure *creditExposureRow,
) (string, error) {
	if exposure == nil {
		return "", bizerr.NewCode(CodeCreditIntegrity)
	}
	rows := make([]struct {
		StatementNo string `orm:"statement_no"`
	}, 0, 2)
	if err := s.database.GetScan(ctx, &rows, `
SELECT DISTINCT statement_no
FROM plugin_idc_core_credit_statement_line
WHERE tenant_id=0 AND exposure_no=? AND line_type_code='principal'
ORDER BY statement_no
LIMIT 2`, exposure.ExposureNo); err != nil {
		return "", gerror.Wrap(err, "load credit refund statement")
	}
	if len(rows) > 1 || (exposure.BilledPrincipalMinor > 0 && len(rows) != 1) {
		return "", bizerr.NewCode(CodeCreditIntegrity)
	}
	if len(rows) == 1 {
		return rows[0].StatementNo, nil
	}
	return "", nil
}

func matchCreditRefundFundingResults(
	planned []CreditRefundFundingSource,
	results []CreditRefundFundingResult,
) (map[string]CreditRefundFundingResult, error) {
	if len(planned) != len(results) {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	byAllocation := make(map[string]CreditRefundFundingResult, len(results))
	for _, result := range results {
		byAllocation[result.RepaymentAllocationNo] = result
	}
	for _, source := range planned {
		result, exists := byAllocation[source.RepaymentAllocationNo]
		if !exists || result.AmountMinor != source.AmountMinor {
			return nil, bizerr.NewCode(CodeCreditConflict)
		}
	}
	return byAllocation, nil
}

func (s *serviceImpl) postCreditRefundFunding(
	ctx context.Context,
	account *creditAccountRow,
	exposure *creditExposureRow,
	source CreditRefundFundingSource,
	result CreditRefundFundingResult,
) error {
	if account == nil || exposure == nil || source.AmountMinor <= 0 {
		return bizerr.NewCode(CodeCreditIntegrity)
	}
	if err := s.lockSource(ctx, "credit_refund", result.RefundSourceReference); err != nil {
		return err
	}
	existing, err := loadTransactionBySource(
		ctx, s.database, "credit_refund", result.RefundSourceReference,
	)
	if err != nil {
		return err
	}
	merchantNo, err := s.loadMerchantNo(ctx, existing == nil)
	if err != nil {
		return err
	}
	revenueSpec := merchantRevenueSpec(merchantNo)
	destinationSpec, role, err := creditRefundDestinationSpec(
		account, source, result.Destination, merchantNo,
	)
	if err != nil {
		return err
	}
	if destinationSpec.CustomerNo != "" {
		if err = s.requireActiveCustomer(ctx, destinationSpec.CustomerNo); err != nil {
			return err
		}
	}
	accounts, err := s.ensureAndLockAccounts(
		ctx, account.CurrencyCode, existing == nil, revenueSpec, destinationSpec,
	)
	if err != nil {
		return err
	}
	transaction, _, err := s.postLocked(
		ctx, "refund", account.CurrencyCode, "credit_refund",
		result.RefundSourceReference, result.EvidenceDigest, "",
		[]entrySemantic{
			{AccountNo: deterministicAccountNo(
				revenueSpec.OwnerType, revenueSpec.OwnerReference,
				revenueSpec.AccountType, account.CurrencyCode,
			), Direction: "debit", Role: "refund", AmountMinor: source.AmountMinor,
				InvoiceNo: exposure.InvoiceNo, Reason: "confirmed_credit_refund"},
			{AccountNo: deterministicAccountNo(
				destinationSpec.OwnerType, destinationSpec.OwnerReference,
				destinationSpec.AccountType, account.CurrencyCode,
			), Direction: "credit", Role: role, AmountMinor: source.AmountMinor,
				InvoiceNo: exposure.InvoiceNo, Reason: "confirmed_credit_refund"},
		}, accounts,
	)
	if err != nil {
		return err
	}
	if transaction == nil || transaction.TransactionNo == "" {
		return bizerr.NewCode(CodeCreditIntegrity)
	}
	return nil
}

func creditRefundDestinationSpec(
	account *creditAccountRow,
	source CreditRefundFundingSource,
	destination CreditRefundDestination,
	merchantNo string,
) (accountSpec, string, error) {
	if account == nil {
		return accountSpec{}, "", bizerr.NewCode(CodeCreditIntegrity)
	}
	if destination == CreditRefundDestinationBalance {
		return customerBalanceSpec(account.CustomerNo), "balance", nil
	}
	if destination != CreditRefundDestinationOriginalSource {
		return accountSpec{}, "", bizerr.NewCode(CodeCreditIntegrity)
	}
	switch source.FundingSourceType {
	case "customer_balance":
		return customerBalanceSpec(account.CustomerNo), "balance", nil
	case "online_payment", "manual_receipt", "migration":
		return merchantCashSpec(merchantNo), "cash", nil
	default:
		return accountSpec{}, "", bizerr.NewCode(CodeCreditIntegrity)
	}
}

func (s *serviceImpl) applyCreditRefundProjection(
	ctx context.Context,
	account *creditAccountRow,
	exposure *creditExposureRow,
	plan *CreditRefundPlan,
	input CreditRefundInput,
) (string, error) {
	if account == nil || exposure == nil || plan == nil {
		return "", bizerr.NewCode(CodeCreditIntegrity)
	}
	statementRepaidMinor := int64(0)
	for _, source := range plan.FundingSources {
		if source.AllocationType == "statement_principal" {
			statementRepaidMinor += source.AmountMinor
		}
		if err := s.reverseCreditRepaymentAllocation(
			ctx, account, source, input.SourceReference,
		); err != nil {
			return "", err
		}
	}
	statementReduction := plan.BilledReductionMinor + statementRepaidMinor
	statementStatus := ""
	if statementReduction > 0 {
		var err error
		statementStatus, err = s.reduceCreditRefundStatement(
			ctx, account, plan.StatementNo, exposure.ExposureNo,
			plan.BilledReductionMinor, statementRepaidMinor,
			input.SourceReference, input.EvidenceDigest,
		)
		if err != nil {
			return "", err
		}
	}
	afterUnbilled := exposure.UnbilledPrincipalMinor - plan.UnbilledReleaseMinor
	afterBilled := exposure.BilledPrincipalMinor - plan.BilledReductionMinor
	afterRepaid := exposure.RepaidPrincipalMinor - plan.RepaidRefundMinor
	afterReleased := exposure.ReleasedPrincipalMinor + plan.AmountMinor
	afterExposureStatus := creditRefundExposureStatus(
		afterUnbilled, afterBilled, afterRepaid,
	)
	updatedExposure, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_exposure
SET unbilled_principal_minor=?,billed_principal_minor=?,
    repaid_principal_minor=?,released_principal_minor=?,status_code=?,
    version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND exposure_no=? AND version=?`,
		afterUnbilled, afterBilled, afterRepaid, afterReleased,
		afterExposureStatus, exposure.ExposureNo, exposure.Version)
	if err != nil {
		return "", gerror.Wrap(err, "apply credit refund exposure")
	}
	if err = requireSingleCreditUpdate(updatedExposure); err != nil {
		return "", err
	}
	afterAvailable := account.AvailableLimitMinor +
		plan.UnbilledReleaseMinor + plan.BilledReductionMinor
	afterAccountUnbilled := account.UnbilledPrincipalMinor - plan.UnbilledReleaseMinor
	afterAccountBilled := account.BilledPrincipalMinor - plan.BilledReductionMinor
	afterDelinquent := account.DelinquentPrincipalMinor
	if plan.BilledReductionMinor > 0 {
		afterDelinquent -= minCreditAmount(afterDelinquent, plan.BilledReductionMinor)
	}
	updatedAccount, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET available_limit_minor=?,unbilled_principal_minor=?,
    billed_principal_minor=?,delinquent_principal_minor=?,
    version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND credit_account_no=? AND version=?`,
		afterAvailable, afterAccountUnbilled, afterAccountBilled,
		afterDelinquent, account.CreditAccountNo, account.Version)
	if err != nil {
		return "", gerror.Wrap(err, "apply credit refund account")
	}
	if err = requireSingleCreditUpdate(updatedAccount); err != nil {
		return "", err
	}
	if err = s.appendCreditMovement(ctx, creditMovementInput{
		MovementNo: deterministicCreditNo(
			"crm_", "refund", account.CreditAccountNo, input.SourceReference,
		),
		Account: account, ExposureNo: exposure.ExposureNo,
		StatementNo: plan.StatementNo, MovementType: "refund",
		AmountMinor: plan.AmountMinor, StatusAfter: account.StatusCode,
		AvailableAfter: afterAvailable, UnbilledAfter: afterAccountUnbilled,
		BilledAfter: afterAccountBilled, DelinquentAfter: afterDelinquent,
		WrittenOffAfter: account.WrittenOffPrincipalMinor,
		SourceType:      "credit_refund", SourceReference: input.SourceReference,
		EvidenceDigest: input.EvidenceDigest, ActorType: input.ActorType,
		ActorID: input.ActorID, OccurredAt: account.Clock,
	}); err != nil {
		return "", err
	}
	exposure.UnbilledPrincipalMinor = afterUnbilled
	exposure.BilledPrincipalMinor = afterBilled
	exposure.RepaidPrincipalMinor = afterRepaid
	exposure.ReleasedPrincipalMinor = afterReleased
	exposure.StatusCode = afterExposureStatus
	exposure.Version++
	account.AvailableLimitMinor = afterAvailable
	account.UnbilledPrincipalMinor = afterAccountUnbilled
	account.BilledPrincipalMinor = afterAccountBilled
	account.DelinquentPrincipalMinor = afterDelinquent
	account.Version++
	return statementStatus, nil
}

func (s *serviceImpl) reverseCreditRepaymentAllocation(
	ctx context.Context,
	account *creditAccountRow,
	source CreditRefundFundingSource,
	refundReference string,
) error {
	reversalNo := deterministicCreditNo(
		"cal_", "refund_reverse", refundReference, source.RepaymentAllocationNo,
	)
	digest, err := contentDigest(struct {
		ReversalNo, OriginalAllocationNo, RepaymentNo, CreditAccountNo string
		AllocationType, RefundReference                                string
		AmountMinor                                                    int64
	}{
		reversalNo, source.RepaymentAllocationNo, source.RepaymentNo,
		account.CreditAccountNo, source.AllocationType, refundReference,
		source.AmountMinor,
	})
	if err != nil {
		return err
	}
	var original struct {
		StatementNo string `orm:"statement_no"`
		ExposureNo  string `orm:"exposure_no"`
	}
	if err = s.database.GetScan(ctx, &original, `
SELECT COALESCE(statement_no,'') AS statement_no,
       COALESCE(exposure_no,'') AS exposure_no
FROM plugin_idc_core_credit_repayment_allocation
WHERE tenant_id=0 AND allocation_no=? AND repayment_no=?
  AND direction_code='apply' FOR UPDATE`,
		source.RepaymentAllocationNo, source.RepaymentNo); err != nil {
		return gerror.Wrap(err, "lock original credit repayment allocation")
	}
	if original.ExposureNo == "" {
		return bizerr.NewCode(CodeCreditIntegrity)
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_repayment_allocation (
    tenant_id,allocation_no,repayment_no,credit_account_no,customer_no,
    currency_code,statement_no,exposure_no,allocation_type_code,direction_code,
    amount_minor,original_allocation_no,source_reference,content_digest
) VALUES (0,?,?,?,?,?,?,?,?,'reverse',?,?,?,?)`,
		reversalNo, source.RepaymentNo, account.CreditAccountNo,
		account.CustomerNo, account.CurrencyCode,
		nullableCreditReference(original.StatementNo), original.ExposureNo,
		source.AllocationType, source.AmountMinor, source.RepaymentAllocationNo,
		reversalNo, digest); err != nil {
		return gerror.Wrap(err, "append credit repayment refund reversal")
	}
	updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_repayment
SET allocated_amount_minor=allocated_amount_minor-?,
    refunded_amount_minor=refunded_amount_minor+?,status_code='refunded',
    refunded_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND repayment_no=? AND allocated_amount_minor>=?`,
		source.AmountMinor, source.AmountMinor, source.RepaymentNo, source.AmountMinor)
	if err != nil {
		return gerror.Wrap(err, "mark credit repayment refunded")
	}
	return requireSingleCreditUpdate(updated)
}

func (s *serviceImpl) reduceCreditRefundStatement(
	ctx context.Context,
	account *creditAccountRow,
	statementNo string,
	exposureNo string,
	billedReductionMinor int64,
	repaidReductionMinor int64,
	refundReference string,
	evidenceDigest string,
) (string, error) {
	if statementNo == "" || account == nil {
		return "", bizerr.NewCode(CodeCreditIntegrity)
	}
	statement, err := loadCreditStatementForUpdate(ctx, s.database, statementNo)
	if err != nil {
		return "", err
	}
	if statement == nil || statement.CreditAccountNo != account.CreditAccountNo ||
		statement.WrittenOffMinor != 0 ||
		statement.PrincipalMinor < billedReductionMinor+repaidReductionMinor ||
		statement.TotalAmountMinor < billedReductionMinor+repaidReductionMinor ||
		statement.OutstandingMinor < billedReductionMinor ||
		statement.PaidAmountMinor < repaidReductionMinor {
		return "", bizerr.NewCode(CodeCreditConflict)
	}
	lineAmount := billedReductionMinor + repaidReductionMinor
	var sequence struct {
		LineNumber int `orm:"line_number"`
	}
	if err = s.database.GetScan(ctx, &sequence, `
SELECT COALESCE(MAX(line_number),0)+1
       AS line_number
FROM plugin_idc_core_credit_statement_line
WHERE tenant_id=0 AND statement_no=?`, statementNo); err != nil {
		return "", gerror.Wrap(err, "derive credit refund statement line")
	}
	lineNumber := sequence.LineNumber
	lineNo := deterministicCreditNo(
		"crl_", "refund", statementNo, refundReference,
	)
	lineDigest, err := contentDigest(struct {
		StatementLineNo, StatementNo, ExposureNo, RefundReference, EvidenceDigest string
		LineNumber                                                                int
		AmountMinor                                                               int64
	}{
		lineNo, statementNo, exposureNo, refundReference, evidenceDigest,
		lineNumber, -lineAmount,
	})
	if err != nil {
		return "", err
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_statement_line (
    tenant_id,statement_line_no,statement_no,credit_account_no,customer_no,
    currency_code,line_number,line_type_code,exposure_no,source_type_code,
    source_reference,amount_minor,content_digest
) VALUES (0,?,?,?,?,?,?,'refund',?,'credit_refund',?, ?,?)`,
		lineNo, statementNo, account.CreditAccountNo, account.CustomerNo,
		account.CurrencyCode, lineNumber, exposureNo, refundReference,
		-lineAmount, lineDigest); err != nil {
		return "", gerror.Wrap(err, "append credit refund statement line")
	}
	afterTotal := statement.TotalAmountMinor - lineAmount
	afterPaid := statement.PaidAmountMinor - repaidReductionMinor
	afterOutstanding := statement.OutstandingMinor - billedReductionMinor
	afterStatus := creditRefundStatementStatus(
		statement.StatusCode, afterTotal, afterPaid, afterOutstanding,
	)
	updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_statement
SET line_count=line_count+1,adjustment_minor=adjustment_minor-?,total_amount_minor=?,
    paid_amount_minor=?,outstanding_amount_minor=?,status_code=?,
    settled_at=CASE WHEN ? IN ('paid','refunded')
                    THEN COALESCE(settled_at,CURRENT_TIMESTAMP) ELSE NULL END,
    version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND statement_no=? AND version=?`,
		lineAmount, afterTotal, afterPaid, afterOutstanding, afterStatus,
		afterStatus, statementNo, statement.Version)
	if err != nil {
		return "", gerror.Wrap(err, "reduce credit refund statement")
	}
	if err = requireSingleCreditUpdate(updated); err != nil {
		return "", err
	}
	if billedReductionMinor > 0 && statement.StatusCode == "overdue" {
		if err = s.reduceCreditRefundDelinquency(
			ctx, statementNo, billedReductionMinor, afterOutstanding, account.Clock,
		); err != nil {
			return "", err
		}
	}
	return afterStatus, nil
}

func (s *serviceImpl) reduceCreditRefundDelinquency(
	ctx context.Context,
	statementNo string,
	principalMinor int64,
	afterOutstanding int64,
	now time.Time,
) error {
	caseRow, err := loadCreditDelinquencyByStatement(ctx, s.database, statementNo, true)
	if err != nil {
		return err
	}
	if caseRow == nil || caseRow.PrincipalMinor < principalMinor ||
		caseRow.OutstandingAmountMinor < principalMinor {
		return bizerr.NewCode(CodeCreditIntegrity)
	}
	afterPrincipal := caseRow.PrincipalMinor - principalMinor
	afterCaseOutstanding := caseRow.OutstandingAmountMinor - principalMinor
	if afterCaseOutstanding != afterOutstanding {
		return bizerr.NewCode(CodeCreditIntegrity)
	}
	afterStatus := caseRow.StatusCode
	var resolvedAt any
	if afterCaseOutstanding == 0 {
		afterStatus = "resolved"
		resolvedAt = now
	}
	updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_delinquency_case
SET principal_minor=?,outstanding_amount_minor=?,status_code=?,resolved_at=?,
    version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND case_no=? AND version=?`,
		afterPrincipal, afterCaseOutstanding, afterStatus, resolvedAt,
		caseRow.CaseNo, caseRow.Version)
	if err != nil {
		return gerror.Wrap(err, "reduce credit refund delinquency")
	}
	return requireSingleCreditUpdate(updated)
}

func creditRefundExposureStatus(unbilled int64, billed int64, repaid int64) string {
	switch {
	case repaid > 0 && (unbilled > 0 || billed > 0):
		return "partially_repaid"
	case unbilled > 0 && billed > 0:
		return "partially_billed"
	case unbilled > 0:
		return "unbilled"
	case billed > 0:
		return "billed"
	case repaid > 0:
		return "settled"
	default:
		return "released"
	}
}

func creditRefundStatementStatus(
	current string,
	totalMinor int64,
	paidMinor int64,
	outstandingMinor int64,
) string {
	switch {
	case totalMinor == 0:
		return "refunded"
	case current == "overdue" && outstandingMinor > 0:
		return "overdue"
	case outstandingMinor == 0:
		return "paid"
	case paidMinor > 0:
		return "partially_paid"
	default:
		return "open"
	}
}

func loadCreditRefundMovement(
	ctx context.Context,
	database gdb.DB,
	creditAccountNo string,
	sourceReference string,
) (*creditRefundMovementRow, error) {
	var row *creditRefundMovementRow
	if err := database.GetScan(ctx, &row, `
SELECT COALESCE(exposure_no,'') AS exposure_no,amount_minor,evidence_digest
FROM plugin_idc_core_credit_account_movement
WHERE tenant_id=0 AND credit_account_no=?
  AND source_type_code='credit_refund' AND source_reference=?`,
		creditAccountNo, sourceReference); err != nil {
		return nil, gerror.Wrap(err, "load existing credit refund")
	}
	return row, nil
}

func projectCreditRefund(
	exposure *creditExposureRow,
	account *creditAccountRow,
	statementNo string,
	statementStatus string,
	amountMinor int64,
	unbilledMinor int64,
	billedMinor int64,
	repaidMinor int64,
	reused bool,
) *CreditRefund {
	if exposure == nil || account == nil {
		return nil
	}
	return &CreditRefund{
		ExposureNo: exposure.ExposureNo, CreditAccountNo: account.CreditAccountNo,
		CustomerNo: account.CustomerNo, CurrencyCode: account.CurrencyCode,
		StatementNo: statementNo, StatementStatusCode: statementStatus,
		AmountMinor: amountMinor, UnbilledReleaseMinor: unbilledMinor,
		BilledReductionMinor: billedMinor, RepaidRefundMinor: repaidMinor,
		ExposureStatusCode:    exposure.StatusCode,
		AccountAvailableMinor: account.AvailableLimitMinor,
		AccountUnbilledMinor:  account.UnbilledPrincipalMinor,
		AccountBilledMinor:    account.BilledPrincipalMinor,
		AccountVersion:        account.Version, Reused: reused,
	}
}

func creditRefundError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeCreditUnavailable)
}
