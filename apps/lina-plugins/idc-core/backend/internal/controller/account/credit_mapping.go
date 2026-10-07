// This file maps customer credit requests and safe service projections at the HTTP boundary.

package account

import (
	"strconv"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

const (
	minimumCreditApplicationUploads = 1
	maximumCreditApplicationUploads = 10
)

func parsePositiveCreditAmount(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != value {
		return 0, bizerr.NewCode(billing.CodeCreditInvalid)
	}
	return parsed, nil
}

func toAPICreditProof(value *reauth.ProofContext) *v1.CreditActionProof {
	if value == nil {
		return nil
	}
	return &v1.CreditActionProof{ProofToken: value.Token, ExpiresAt: value.ExpiresAt.UnixMilli()}
}

func toAPICreditOverview(value *billing.AccountCreditOverview) *v1.CreditOverview {
	if value == nil {
		return nil
	}
	return &v1.CreditOverview{
		Accounts:                toAPICreditAccounts(value.Accounts),
		PendingApplicationCount: value.PendingApplicationCount,
		OpenStatementCount:      value.OpenStatementCount,
		OverdueStatementCount:   value.OverdueStatementCount,
	}
}

func toAPICreditAccounts(values []billing.CreditAccountDirectoryItem) []*v1.CreditAccount {
	result := make([]*v1.CreditAccount, 0, len(values))
	for _, value := range values {
		result = append(result, toAPICreditAccount(value))
	}
	return result
}

func toAPICreditAccount(value billing.CreditAccountDirectoryItem) *v1.CreditAccount {
	terms := value.Terms
	result := &v1.CreditAccount{
		CreditAccountNo: value.CreditAccountNo, CurrencyCode: value.CurrencyCode,
		StatusCode: value.StatusCode, TotalLimitMinor: amount(value.TotalLimitMinor),
		AvailableLimitMinor:      amount(value.AvailableLimitMinor),
		UnbilledPrincipalMinor:   amount(value.UnbilledPrincipalMinor),
		BilledPrincipalMinor:     amount(value.BilledPrincipalMinor),
		DelinquentPrincipalMinor: amount(value.DelinquentPrincipalMinor),
		WrittenOffPrincipalMinor: amount(value.WrittenOffPrincipalMinor),
		ValidFrom:                value.ValidFrom.UnixMilli(), ValidUntil: value.ValidUntil.UnixMilli(),
		FreezeReasonCode: value.FreezeReasonCode, FrozenAt: unixMillis(value.FrozenAt),
		Version: value.Version, UpdatedAt: value.UpdatedAt.UnixMilli(),
		Terms: &v1.CreditAccountTerms{
			TermsNo: terms.TermsNo, TermsVersion: terms.TermsVersion,
			TotalLimitMinor: amount(terms.TotalLimitMinor), BillingDay: terms.BillingDay,
			RepaymentPeriodDays: terms.RepaymentPeriodDays, GracePeriodDays: terms.GracePeriodDays,
			DailyLateFeeBasisPoints: terms.DailyLateFeeBasisPoints,
			MaximumLateFeeMinor:     amount(terms.MaximumLateFeeMinor),
			EffectiveFrom:           terms.EffectiveFrom.UnixMilli(), EffectiveUntil: terms.EffectiveUntil.UnixMilli(),
		},
	}
	if item := value.LatestDelinquency; item != nil {
		result.LatestDelinquency = &v1.CreditDelinquency{
			CaseNo: item.CaseNo, StatementNo: item.StatementNo, StatusCode: item.StatusCode,
			PrincipalMinor: amount(item.PrincipalMinor), LateFeeMinor: amount(item.LateFeeMinor),
			OutstandingAmountMinor: amount(item.OutstandingAmountMinor),
			OpenedAt:               item.OpenedAt.UnixMilli(), GraceUntil: item.GraceUntil.UnixMilli(),
			ResolvedAt: unixMillis(item.ResolvedAt),
		}
	}
	return result
}

func toAPICreditApplications(values []billing.CreditApplicationDirectoryItem) []*v1.CreditApplication {
	result := make([]*v1.CreditApplication, 0, len(values))
	for _, value := range values {
		result = append(result, toAPICreditApplicationDirectory(value))
	}
	return result
}

func toAPICreditApplicationDirectory(value billing.CreditApplicationDirectoryItem) *v1.CreditApplication {
	return &v1.CreditApplication{
		ApplicationNo: value.ApplicationNo, CurrencyCode: value.CurrencyCode,
		PolicyNo: value.PolicyNo, PolicyVersion: value.PolicyVersion,
		RequestedLimitMinor:   amount(value.RequestedLimitMinor),
		RequestedValidFrom:    value.RequestedValidFrom.UnixMilli(),
		RequestedValidUntil:   value.RequestedValidUntil.UnixMilli(),
		RequiredApprovalCount: value.RequiredApprovalCount, ReviewCount: value.ReviewCount,
		StatusCode: value.StatusCode, CreditAccountNo: value.CreditAccountNo,
		CreatedAt: value.CreatedAt.UnixMilli(), UpdatedAt: value.UpdatedAt.UnixMilli(),
	}
}

func toAPICreditApplication(value *billing.CreditApplication) *v1.CreditApplication {
	if value == nil {
		return nil
	}
	return &v1.CreditApplication{
		ApplicationNo: value.ApplicationNo, CurrencyCode: value.CurrencyCode,
		PolicyNo: value.PolicyNo, PolicyVersion: value.PolicyVersion,
		RequestedLimitMinor:   amount(value.RequestedLimitMinor),
		RequestedValidFrom:    value.RequestedValidFrom.UnixMilli(),
		RequestedValidUntil:   value.RequestedValidUntil.UnixMilli(),
		RequiredApprovalCount: value.RequiredApprovalCount, ReviewCount: value.ReviewCount,
		StatusCode: value.StatusCode, CreditAccountNo: value.CreditAccountNo,
		CreatedAt: value.CreatedAt.UnixMilli(), UpdatedAt: value.UpdatedAt.UnixMilli(), Reused: value.Reused,
	}
}

func toAPICreditStatements(values []billing.CreditStatementDirectoryItem) []*v1.CreditStatement {
	result := make([]*v1.CreditStatement, 0, len(values))
	for _, value := range values {
		result = append(result, toAPICreditStatement(value))
	}
	return result
}

func toAPICreditStatement(value billing.CreditStatementDirectoryItem) *v1.CreditStatement {
	return &v1.CreditStatement{
		StatementNo: value.StatementNo, CreditAccountNo: value.CreditAccountNo,
		CurrencyCode: value.CurrencyCode, PeriodStartedAt: value.PeriodStartedAt.UnixMilli(),
		PeriodEndedAt: value.PeriodEndedAt.UnixMilli(), LineCount: value.LineCount,
		PrincipalMinor: amount(value.PrincipalMinor), FeeMinor: amount(value.FeeMinor),
		AdjustmentMinor: amount(value.AdjustmentMinor), TotalAmountMinor: amount(value.TotalAmountMinor),
		PaidAmountMinor: amount(value.PaidAmountMinor), WrittenOffAmountMinor: amount(value.WrittenOffAmountMinor),
		OutstandingAmountMinor: amount(value.OutstandingAmountMinor), StatusCode: value.StatusCode,
		IssuedAt: value.IssuedAt.UnixMilli(), PaymentDueAt: value.PaymentDueAt.UnixMilli(),
		OverdueAt: unixMillis(value.OverdueAt), SettledAt: unixMillis(value.SettledAt),
		WrittenOffAt: unixMillis(value.WrittenOffAt), Version: value.Version,
		AvailableBalanceMinor: amount(value.AvailableBalanceMinor),
		CanRepayFromBalance:   value.CanRepayFromBalance,
		CreatedAt:             value.CreatedAt.UnixMilli(), UpdatedAt: value.UpdatedAt.UnixMilli(),
	}
}

func toAPICreditStatementDetail(value *billing.CreditStatementDirectoryDetail) *v1.CreditStatementDetail {
	if value == nil {
		return nil
	}
	result := &v1.CreditStatementDetail{
		Statement:           toAPICreditStatement(value.Statement),
		Lines:               make([]*v1.CreditStatementLine, 0, len(value.Lines)),
		NextLineCursor:      value.NextLineCursor,
		Repayments:          make([]*v1.CreditRepayment, 0, len(value.Repayments)),
		RepaymentsTruncated: value.RepaymentsTruncated,
	}
	for _, item := range value.Lines {
		result.Lines = append(result.Lines, &v1.CreditStatementLine{
			StatementLineNo: item.StatementLineNo, LineNumber: item.LineNumber,
			LineTypeCode: item.LineTypeCode, SourceTypeCode: item.SourceTypeCode,
			SourceReference: item.SourceReference, AmountMinor: amount(item.AmountMinor),
			CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	for _, item := range value.Repayments {
		result.Repayments = append(result.Repayments, &v1.CreditRepayment{
			RepaymentNo: item.RepaymentNo, RepaymentTypeCode: item.RepaymentTypeCode,
			FundingSourceTypeCode: item.FundingSourceTypeCode, AmountMinor: amount(item.AmountMinor),
			AllocatedAmountMinor:   amount(item.AllocatedAmountMinor),
			UnallocatedAmountMinor: amount(item.UnallocatedAmountMinor), StatusCode: item.StatusCode,
			ConfirmedAt: unixMillis(item.ConfirmedAt), CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	return result
}

func toAPICreditBalanceRepayment(value *billing.CreditBalanceRepayment) *v1.CreditBalanceRepaymentResult {
	if value == nil || value.Repayment == nil {
		return nil
	}
	repayment := value.Repayment
	return &v1.CreditBalanceRepaymentResult{
		Repayment: &v1.CreditBalanceRepayment{
			RepaymentNo: repayment.RepaymentNo, CreditAccountNo: repayment.CreditAccountNo,
			CurrencyCode: repayment.CurrencyCode, FundingSourceTypeCode: repayment.FundingSourceType,
			AmountMinor: amount(repayment.AmountMinor), AllocatedAmountMinor: amount(repayment.AllocatedAmountMinor),
			UnallocatedAmountMinor: amount(repayment.UnallocatedAmountMinor), StatusCode: repayment.StatusCode,
		},
		StatementStatusCode:       repayment.StatementStatusCode,
		StatementOutstandingMinor: amount(repayment.StatementOutstandingMinor),
		BalanceMinor:              amount(value.BalanceMinor), HeldBalanceMinor: amount(value.HeldBalanceMinor),
		AvailableBalanceMinor: amount(value.AvailableBalanceMinor), Reused: value.Reused,
	}
}

func creditApplicationIntent(req *v1.CreditApplicationSubmitReq) (billing.AccountCreditApplicationIntent, error) {
	if req == nil || len(req.UploadNos) < minimumCreditApplicationUploads || len(req.UploadNos) > maximumCreditApplicationUploads {
		return billing.AccountCreditApplicationIntent{}, bizerr.NewCode(billing.CodeCreditInvalid)
	}
	requestedLimit, err := parsePositiveCreditAmount(req.RequestedLimitMinor)
	if err != nil {
		return billing.AccountCreditApplicationIntent{}, err
	}
	return billing.AccountCreditApplicationIntent{
		CurrencyCode: req.CurrencyCode, RequestedLimitMinor: requestedLimit,
		RequestedValidFrom:  time.UnixMilli(req.RequestedValidFrom),
		RequestedValidUntil: time.UnixMilli(req.RequestedValidUntil),
		UploadNos:           req.UploadNos, IdempotencyKey: req.IdempotencyKey,
	}, nil
}

func creditBalanceRepaymentIntent(
	statementNo string,
	value v1.CreditBalanceRepaymentIntent,
) (billing.AccountCreditBalanceRepaymentIntent, error) {
	amountMinor, err := parsePositiveCreditAmount(value.AmountMinor)
	if err != nil {
		return billing.AccountCreditBalanceRepaymentIntent{}, err
	}
	return billing.AccountCreditBalanceRepaymentIntent{
		StatementNo: statementNo, AmountMinor: amountMinor, RequestNo: value.RequestNo,
	}, nil
}
