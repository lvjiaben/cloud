// This file maps credit-management requests and staff projections at the HTTP boundary.

package admin

import (
	"strconv"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func parseCreditAmount(value string, positive bool) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 || (positive && parsed == 0) || strconv.FormatInt(parsed, 10) != value {
		return 0, bizerr.NewCode(billing.CodeCreditInvalid)
	}
	return parsed, nil
}

func toAPIManagementCreditApplications(
	values []billing.CreditApplicationDirectoryItem,
) []*v1.ManagementCreditApplication {
	result := make([]*v1.ManagementCreditApplication, 0, len(values))
	for _, value := range values {
		result = append(result, toAPIManagementCreditApplicationDirectory(value))
	}
	return result
}

func toAPIManagementCreditApplicationDirectory(
	value billing.CreditApplicationDirectoryItem,
) *v1.ManagementCreditApplication {
	return &v1.ManagementCreditApplication{
		ApplicationNo: value.ApplicationNo, CustomerNo: value.CustomerNo,
		CurrencyCode: value.CurrencyCode, PolicyNo: value.PolicyNo, PolicyVersion: value.PolicyVersion,
		RequestedLimitMinor:   financialAmount(value.RequestedLimitMinor),
		RequestedValidFrom:    value.RequestedValidFrom.UnixMilli(),
		RequestedValidUntil:   value.RequestedValidUntil.UnixMilli(),
		RequiredApprovalCount: value.RequiredApprovalCount, ReviewCount: value.ReviewCount,
		StatusCode: value.StatusCode, CreditAccountNo: value.CreditAccountNo,
		CreatedAt: value.CreatedAt.UnixMilli(), UpdatedAt: value.UpdatedAt.UnixMilli(),
	}
}

func toAPIManagementCreditApplication(value *billing.CreditApplication) *v1.ManagementCreditApplication {
	if value == nil {
		return nil
	}
	return &v1.ManagementCreditApplication{
		ApplicationNo: value.ApplicationNo, CustomerNo: value.CustomerNo,
		CurrencyCode: value.CurrencyCode, PolicyNo: value.PolicyNo, PolicyVersion: value.PolicyVersion,
		RequestedLimitMinor:   financialAmount(value.RequestedLimitMinor),
		RequestedValidFrom:    value.RequestedValidFrom.UnixMilli(),
		RequestedValidUntil:   value.RequestedValidUntil.UnixMilli(),
		RequiredApprovalCount: value.RequiredApprovalCount, ReviewCount: value.ReviewCount,
		StatusCode: value.StatusCode, CreditAccountNo: value.CreditAccountNo,
		CreatedAt: value.CreatedAt.UnixMilli(), UpdatedAt: value.UpdatedAt.UnixMilli(), Reused: value.Reused,
	}
}

func toAPIManagementCreditApplicationDetail(
	value *billing.CreditApplicationDirectoryDetail,
) *v1.ManagementCreditApplicationDetail {
	if value == nil {
		return nil
	}
	result := &v1.ManagementCreditApplicationDetail{
		Application: toAPIManagementCreditApplicationDirectory(value.Application),
		Reviews:     make([]*v1.ManagementCreditApplicationReview, 0, len(value.Reviews)),
	}
	for _, item := range value.Reviews {
		result.Reviews = append(result.Reviews, &v1.ManagementCreditApplicationReview{
			ReviewNo: item.ReviewNo, ReviewSequence: item.ReviewSequence,
			DecisionCode: item.DecisionCode, ReviewerID: item.ReviewerID,
			CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	return result
}

func toAPIManagementCreditAccounts(
	values []billing.CreditAccountDirectoryItem,
) []*v1.ManagementCreditAccount {
	result := make([]*v1.ManagementCreditAccount, 0, len(values))
	for _, value := range values {
		result = append(result, toAPIManagementCreditAccount(value))
	}
	return result
}

func toAPIManagementCreditAccount(value billing.CreditAccountDirectoryItem) *v1.ManagementCreditAccount {
	terms := value.Terms
	result := &v1.ManagementCreditAccount{
		CreditAccountNo: value.CreditAccountNo, CustomerNo: value.CustomerNo,
		CurrencyCode: value.CurrencyCode, StatusCode: value.StatusCode,
		TotalLimitMinor:          financialAmount(value.TotalLimitMinor),
		AvailableLimitMinor:      financialAmount(value.AvailableLimitMinor),
		UnbilledPrincipalMinor:   financialAmount(value.UnbilledPrincipalMinor),
		BilledPrincipalMinor:     financialAmount(value.BilledPrincipalMinor),
		DelinquentPrincipalMinor: financialAmount(value.DelinquentPrincipalMinor),
		WrittenOffPrincipalMinor: financialAmount(value.WrittenOffPrincipalMinor),
		ValidFrom:                value.ValidFrom.UnixMilli(), ValidUntil: value.ValidUntil.UnixMilli(),
		FreezeReasonCode: value.FreezeReasonCode, FrozenAt: financialUnixMilliseconds(value.FrozenAt),
		Version: value.Version, UpdatedAt: value.UpdatedAt.UnixMilli(),
		Terms: &v1.ManagementCreditAccountTerms{
			TermsNo: terms.TermsNo, TermsVersion: terms.TermsVersion,
			TotalLimitMinor: financialAmount(terms.TotalLimitMinor), BillingDay: terms.BillingDay,
			RepaymentPeriodDays: terms.RepaymentPeriodDays, GracePeriodDays: terms.GracePeriodDays,
			DailyLateFeeBasisPoints: terms.DailyLateFeeBasisPoints,
			MaximumLateFeeMinor:     financialAmount(terms.MaximumLateFeeMinor),
			EffectiveFrom:           terms.EffectiveFrom.UnixMilli(), EffectiveUntil: terms.EffectiveUntil.UnixMilli(),
		},
	}
	if item := value.LatestDelinquency; item != nil {
		result.LatestDelinquency = &v1.ManagementCreditDelinquency{
			CaseNo: item.CaseNo, StatementNo: item.StatementNo, StatusCode: item.StatusCode,
			PrincipalMinor:         financialAmount(item.PrincipalMinor),
			LateFeeMinor:           financialAmount(item.LateFeeMinor),
			OutstandingAmountMinor: financialAmount(item.OutstandingAmountMinor),
			OpenedAt:               item.OpenedAt.UnixMilli(), GraceUntil: item.GraceUntil.UnixMilli(),
			ResolvedAt: financialUnixMilliseconds(item.ResolvedAt),
		}
	}
	return result
}

func toAPIManagementCreditStatements(
	values []billing.CreditStatementDirectoryItem,
) []*v1.ManagementCreditStatement {
	result := make([]*v1.ManagementCreditStatement, 0, len(values))
	for _, value := range values {
		result = append(result, toAPIManagementCreditStatement(value))
	}
	return result
}

func toAPIManagementCreditStatement(
	value billing.CreditStatementDirectoryItem,
) *v1.ManagementCreditStatement {
	return &v1.ManagementCreditStatement{
		StatementNo: value.StatementNo, CreditAccountNo: value.CreditAccountNo,
		CustomerNo: value.CustomerNo, CurrencyCode: value.CurrencyCode,
		PeriodStartedAt: value.PeriodStartedAt.UnixMilli(), PeriodEndedAt: value.PeriodEndedAt.UnixMilli(),
		LineCount: value.LineCount, PrincipalMinor: financialAmount(value.PrincipalMinor),
		FeeMinor: financialAmount(value.FeeMinor), AdjustmentMinor: financialAmount(value.AdjustmentMinor),
		TotalAmountMinor:       financialAmount(value.TotalAmountMinor),
		PaidAmountMinor:        financialAmount(value.PaidAmountMinor),
		WrittenOffAmountMinor:  financialAmount(value.WrittenOffAmountMinor),
		OutstandingAmountMinor: financialAmount(value.OutstandingAmountMinor), StatusCode: value.StatusCode,
		IssuedAt: value.IssuedAt.UnixMilli(), PaymentDueAt: value.PaymentDueAt.UnixMilli(),
		OverdueAt:    financialUnixMilliseconds(value.OverdueAt),
		SettledAt:    financialUnixMilliseconds(value.SettledAt),
		WrittenOffAt: financialUnixMilliseconds(value.WrittenOffAt), Version: value.Version,
		CreatedAt: value.CreatedAt.UnixMilli(), UpdatedAt: value.UpdatedAt.UnixMilli(),
	}
}

func toAPIManagementCreditStatementDetail(
	value *billing.CreditStatementDirectoryDetail,
) *v1.ManagementCreditStatementDetail {
	if value == nil {
		return nil
	}
	result := &v1.ManagementCreditStatementDetail{
		Statement:           toAPIManagementCreditStatement(value.Statement),
		Lines:               make([]*v1.ManagementCreditStatementLine, 0, len(value.Lines)),
		NextLineCursor:      value.NextLineCursor,
		Repayments:          make([]*v1.ManagementCreditRepayment, 0, len(value.Repayments)),
		RepaymentsTruncated: value.RepaymentsTruncated,
	}
	for _, item := range value.Lines {
		result.Lines = append(result.Lines, &v1.ManagementCreditStatementLine{
			StatementLineNo: item.StatementLineNo, LineNumber: item.LineNumber,
			LineTypeCode: item.LineTypeCode, SourceTypeCode: item.SourceTypeCode,
			SourceReference: item.SourceReference, AmountMinor: financialAmount(item.AmountMinor),
			CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	for _, item := range value.Repayments {
		result.Repayments = append(result.Repayments, &v1.ManagementCreditRepayment{
			RepaymentNo: item.RepaymentNo, RepaymentTypeCode: item.RepaymentTypeCode,
			FundingSourceTypeCode:  item.FundingSourceTypeCode,
			AmountMinor:            financialAmount(item.AmountMinor),
			AllocatedAmountMinor:   financialAmount(item.AllocatedAmountMinor),
			UnallocatedAmountMinor: financialAmount(item.UnallocatedAmountMinor), StatusCode: item.StatusCode,
			ConfirmedAt: financialUnixMilliseconds(item.ConfirmedAt), CreatedAt: item.CreatedAt.UnixMilli(),
		})
	}
	return result
}

func managementCreditPolicyIntent(
	value v1.ManagementCreditPolicyIntent,
) (billing.ManagementCreditPolicyIntent, error) {
	maximumLimit, err := parseCreditAmount(value.MaximumLimitMinor, true)
	if err != nil {
		return billing.ManagementCreditPolicyIntent{}, err
	}
	if value.DualReviewThresholdMinor == "" {
		value.DualReviewThresholdMinor = value.MaximumLimitMinor
	}
	dualReviewThreshold, err := parseCreditAmount(value.DualReviewThresholdMinor, true)
	if err != nil {
		return billing.ManagementCreditPolicyIntent{}, err
	}
	maximumLateFee, err := parseCreditAmount(value.MaximumLateFeeMinor, false)
	if err != nil {
		return billing.ManagementCreditPolicyIntent{}, err
	}
	return billing.ManagementCreditPolicyIntent{
		RequestNo: value.RequestNo, CurrencyCode: value.CurrencyCode,
		MaximumLimitMinor: maximumLimit, DualReviewThresholdMinor: dualReviewThreshold,
		BillingDay: value.BillingDay, RepaymentPeriodDays: value.RepaymentPeriodDays,
		GracePeriodDays:         value.GracePeriodDays,
		DailyLateFeeBasisPoints: value.DailyLateFeeBasisPoints,
		MaximumLateFeeMinor:     maximumLateFee,
	}, nil
}

func managementCreditReviewIntent(
	applicationNo string,
	value v1.ManagementCreditApplicationReviewIntent,
) billing.ManagementCreditReviewIntent {
	return billing.ManagementCreditReviewIntent{
		RequestNo: value.RequestNo, ApplicationNo: applicationNo,
		Decision:      billing.CreditDecision(value.DecisionCode),
		CommentDigest: value.CommentDigest, EvidenceDigest: value.EvidenceDigest,
	}
}

func toAPIManagementCreditPolicy(value *billing.CreditPolicy) *v1.ManagementCreditPolicyMutation {
	if value == nil {
		return nil
	}
	return &v1.ManagementCreditPolicyMutation{
		PolicyNo: value.PolicyNo, Version: value.Version, CurrencyCode: value.CurrencyCode,
		MaximumLimitMinor:        financialAmount(value.MaximumLimitMinor),
		DualReviewThresholdMinor: financialAmount(value.DualReviewThresholdMinor),
		BillingDay:               value.BillingDay, RepaymentPeriodDays: value.RepaymentPeriodDays,
		GracePeriodDays:         value.GracePeriodDays,
		DailyLateFeeBasisPoints: value.DailyLateFeeBasisPoints,
		MaximumLateFeeMinor:     financialAmount(value.MaximumLateFeeMinor),
		CreatedAt:               value.CreatedAt.UnixMilli(), Reused: value.Reused,
	}
}
