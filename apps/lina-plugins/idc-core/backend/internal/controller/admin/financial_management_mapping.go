package admin

import (
	"strconv"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

func financialAmount(value int64) string { return strconv.FormatInt(value, 10) }

func financialUnixMilliseconds(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UnixMilli()
}

func parsePositiveFinancialAmount(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != value {
		return 0, bizerr.NewCode(billing.CodeInvoiceInvalid)
	}
	return parsed, nil
}

func toAPIManagementActionProof(value *reauth.ProofContext) *v1.FinancialManagementActionProof {
	if value == nil {
		return nil
	}
	return &v1.FinancialManagementActionProof{ProofToken: value.Token, ExpiresAt: value.ExpiresAt.UnixMilli()}
}

func toAPIManagementAdjustmentMutation(value *billing.FinancialAdjustment) *v1.FinancialManagementAdjustmentMutation {
	if value == nil {
		return nil
	}
	return &v1.FinancialManagementAdjustmentMutation{
		AdjustmentNo: value.AdjustmentNo, AdjustmentTypeCode: value.AdjustmentType,
		StatusCode: string(value.Status), CustomerNo: value.CustomerNo, InvoiceNo: value.InvoiceNo,
		CreditStatementNo: value.CreditStatementNo,
		CurrencyCode:      value.CurrencyCode, AmountMinor: financialAmount(value.AmountMinor),
		ApprovalRequired: value.ApprovalRequired, LedgerTransactionNo: value.LedgerTransactionNo,
		Reused: value.Reused,
	}
}

func toAPIManagementAdjustmentPolicyMutation(value *billing.AdjustmentPolicy) *v1.FinancialManagementAdjustmentPolicyMutation {
	if value == nil {
		return nil
	}
	return &v1.FinancialManagementAdjustmentPolicyMutation{
		PolicyNo: value.PolicyNo, CurrencyCode: value.CurrencyCode,
		ReviewThresholdMinor: financialAmount(value.ReviewThresholdMinor),
		Version:              value.Version, Reused: value.Reused,
	}
}

func managementManualReceiptIntent(
	value v1.FinancialManagementManualReceiptIntent,
) (billing.ManagementManualReceiptIntent, error) {
	amountMinor, err := parsePositiveFinancialAmount(value.AmountMinor)
	if err != nil {
		return billing.ManagementManualReceiptIntent{}, err
	}
	return billing.ManagementManualReceiptIntent{
		RequestNo: value.RequestNo, InvoiceNo: value.InvoiceNo,
		CreditStatementNo: value.CreditStatementNo, CustomerNo: value.CustomerNo,
		CurrencyCode: value.CurrencyCode, AmountMinor: amountMinor, ReasonCode: value.ReasonCode,
		ProofReference: value.ProofReference, ReceivingAccountReference: value.ReceivingAccountReference,
		BusinessPurposeCode: value.BusinessPurposeCode, EvidenceDigest: value.EvidenceDigest,
	}, nil
}

func managementInvoiceWaiverIntent(
	value v1.FinancialManagementInvoiceWaiverIntent,
) (billing.ManagementInvoiceWaiverIntent, error) {
	amountMinor, err := parsePositiveFinancialAmount(value.AmountMinor)
	if err != nil {
		return billing.ManagementInvoiceWaiverIntent{}, err
	}
	return billing.ManagementInvoiceWaiverIntent{
		RequestNo: value.RequestNo, InvoiceNo: value.InvoiceNo, AmountMinor: amountMinor,
		ReasonCode: value.ReasonCode, EvidenceDigest: value.EvidenceDigest,
	}, nil
}

func managementAdjustmentReviewIntent(
	adjustmentNo string,
	value v1.FinancialManagementAdjustmentReviewIntent,
) billing.ManagementAdjustmentReviewIntent {
	return billing.ManagementAdjustmentReviewIntent{
		RequestNo: value.RequestNo, AdjustmentNo: adjustmentNo,
		Decision:      billing.AdjustmentDecision(value.DecisionCode),
		CommentDigest: value.CommentDigest, EvidenceDigest: value.EvidenceDigest,
	}
}

func managementAdjustmentPolicyIntent(
	value v1.FinancialManagementAdjustmentPolicyIntent,
) (billing.ManagementAdjustmentPolicyIntent, error) {
	threshold, err := parsePositiveFinancialAmount(value.ReviewThresholdMinor)
	if err != nil {
		return billing.ManagementAdjustmentPolicyIntent{}, err
	}
	return billing.ManagementAdjustmentPolicyIntent{
		RequestNo: value.RequestNo, CurrencyCode: value.CurrencyCode,
		ReviewThresholdMinor: threshold,
	}, nil
}

func toAPIFinancialManagementOverview(value *billing.ManagementFinancialOverview) *v1.FinancialManagementOverview {
	if value == nil {
		return nil
	}
	result := &v1.FinancialManagementOverview{
		OpenInvoiceCount: value.OpenInvoiceCount, OverdueInvoiceCount: value.OverdueInvoiceCount,
		PendingAdjustmentCount: value.PendingAdjustmentCount,
		Receivables:            make([]*v1.FinancialManagementReceivable, 0, len(value.Receivables)),
	}
	for _, item := range value.Receivables {
		result.Receivables = append(result.Receivables, &v1.FinancialManagementReceivable{
			CurrencyCode: item.CurrencyCode, OutstandingMinor: financialAmount(item.OutstandingMinor),
		})
	}
	return result
}

func toAPIFinancialManagementInvoices(values []billing.InvoiceDirectoryItem) []*v1.FinancialManagementInvoice {
	result := make([]*v1.FinancialManagementInvoice, 0, len(values))
	for _, value := range values {
		result = append(result, toAPIFinancialManagementInvoice(value))
	}
	return result
}

func toAPIFinancialManagementInvoice(value billing.InvoiceDirectoryItem) *v1.FinancialManagementInvoice {
	return &v1.FinancialManagementInvoice{
		InvoiceNo: value.InvoiceNo, CustomerNo: value.CustomerNo, OrderNo: value.OrderNo,
		CustomerCode: value.CustomerCode, CustomerDisplayName: value.CustomerDisplayName,
		CustomerPrimaryPhone: value.CustomerPrimaryPhone,
		SourceCode:           value.SourceCode, CurrencyCode: value.CurrencyCode, StatusCode: value.StatusCode,
		ItemCount: value.ItemCount, TotalAmountMinor: financialAmount(value.TotalAmountMinor),
		PaidAmountMinor:        financialAmount(value.PaidAmountMinor),
		WaivedAmountMinor:      financialAmount(value.WaivedAmountMinor),
		CreditAmountMinor:      financialAmount(value.CreditAmountMinor),
		OutstandingAmountMinor: financialAmount(value.OutstandingAmountMinor),
		RefundableAmountMinor:  financialAmount(value.RefundableAmountMinor),
		RefundedAmountMinor:    financialAmount(value.RefundedAmountMinor),
		PaymentDueAt:           value.PaymentDueAt.UnixMilli(), IssuedAt: financialUnixMilliseconds(value.IssuedAt),
		SettledAt: financialUnixMilliseconds(value.SettledAt), CreatedAt: value.CreatedAt.UnixMilli(),
	}
}

func toAPIFinancialManagementInvoiceDetail(value *billing.InvoiceDirectoryDetail) *v1.FinancialManagementInvoiceDetail {
	if value == nil {
		return nil
	}
	result := &v1.FinancialManagementInvoiceDetail{
		Invoice:        toAPIFinancialManagementInvoice(value.Invoice),
		Items:          make([]*v1.FinancialManagementInvoiceItem, 0, len(value.Items)),
		NextItemCursor: value.NextCursor,
	}
	for _, item := range value.Items {
		result.Items = append(result.Items, &v1.FinancialManagementInvoiceItem{
			InvoiceItemNo: item.InvoiceItemNo, LineNumber: item.LineNumber,
			ItemTypeCode: item.ItemTypeCode, SourceReference: item.SourceReference,
			DescriptionZhCN: item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS,
			SubtotalMinor: financialAmount(item.SubtotalMinor), DiscountMinor: financialAmount(item.DiscountMinor),
			NetAmountMinor: financialAmount(item.NetAmountMinor), TaxAmountMinor: financialAmount(item.TaxAmountMinor),
			TotalAmountMinor: financialAmount(item.TotalAmountMinor),
		})
	}
	return result
}

func toAPIFinancialManagementTransactions(values []billing.ManagementTransactionDirectoryItem) []*v1.FinancialManagementTransaction {
	result := make([]*v1.FinancialManagementTransaction, 0, len(values))
	for _, value := range values {
		result = append(result, &v1.FinancialManagementTransaction{
			TransactionNo: value.TransactionNo, TransactionTypeCode: value.TransactionTypeCode,
			CustomerNo: value.CustomerNo, CurrencyCode: value.CurrencyCode,
			CustomerCode: value.CustomerCode, CustomerDisplayName: value.CustomerDisplayName,
			CustomerPrimaryPhone: value.CustomerPrimaryPhone,
			AmountMinor:          financialAmount(value.AmountMinor), RelatedInvoiceNo: value.RelatedInvoiceNo,
			EntryCount: value.EntryCount, PostedAt: value.PostedAt.UnixMilli(),
		})
	}
	return result
}

func toAPIFinancialManagementTransactionDetail(value *billing.ManagementTransactionDetail) *v1.FinancialManagementTransactionDetail {
	if value == nil {
		return nil
	}
	result := &v1.FinancialManagementTransactionDetail{
		TransactionNo: value.TransactionNo, TransactionTypeCode: value.TransactionTypeCode,
		CurrencyCode: value.CurrencyCode, SourceTypeCode: value.SourceTypeCode,
		SourceReference: value.SourceReference, OriginalTransactionNo: value.OriginalTransactionNo,
		EntryCount: value.EntryCount, AmountMinor: financialAmount(value.AmountMinor),
		PostedBy: value.PostedBy, PostedAt: value.PostedAt.UnixMilli(),
		Entries:         make([]*v1.FinancialManagementTransactionEntry, 0, len(value.Entries)),
		NextEntryCursor: value.NextCursor,
	}
	for _, item := range value.Entries {
		result.Entries = append(result.Entries, &v1.FinancialManagementTransactionEntry{
			EntryNo: item.EntryNo, LineNumber: item.LineNumber, AccountNo: item.AccountNo,
			OwnerTypeCode: item.OwnerTypeCode, OwnerReference: item.OwnerReference,
			CustomerNo: item.CustomerNo, AccountTypeCode: item.AccountTypeCode,
			DirectionCode: item.DirectionCode, EntryRoleCode: item.EntryRoleCode,
			AmountMinor: financialAmount(item.AmountMinor), InvoiceNo: item.InvoiceNo,
			InvoiceItemNo: item.InvoiceItemNo, ReasonCode: item.ReasonCode,
		})
	}
	return result
}

func toAPIFinancialManagementAdjustments(values []billing.ManagementAdjustmentDirectoryItem) []*v1.FinancialManagementAdjustment {
	result := make([]*v1.FinancialManagementAdjustment, 0, len(values))
	for _, value := range values {
		result = append(result, toAPIFinancialManagementAdjustment(value))
	}
	return result
}

func toAPIFinancialManagementAdjustment(value billing.ManagementAdjustmentDirectoryItem) *v1.FinancialManagementAdjustment {
	return &v1.FinancialManagementAdjustment{
		AdjustmentNo: value.AdjustmentNo, AdjustmentTypeCode: value.AdjustmentTypeCode,
		StatusCode: value.StatusCode, CustomerNo: value.CustomerNo, InvoiceNo: value.InvoiceNo,
		CustomerCode: value.CustomerCode, CustomerDisplayName: value.CustomerDisplayName,
		CustomerPrimaryPhone: value.CustomerPrimaryPhone,
		CurrencyCode:         value.CurrencyCode, AmountMinor: financialAmount(value.AmountMinor),
		ReasonCode: value.ReasonCode, ApprovalRequired: value.ApprovalRequired,
		RequestedBy: value.RequestedBy, LedgerTransactionNo: value.LedgerTransactionNo,
		CreatedAt: value.CreatedAt.UnixMilli(), UpdatedAt: value.UpdatedAt.UnixMilli(),
	}
}

func toAPIFinancialManagementAdjustmentDetail(value *billing.ManagementAdjustmentDetail) *v1.FinancialManagementAdjustmentDetail {
	if value == nil {
		return nil
	}
	return &v1.FinancialManagementAdjustmentDetail{
		Adjustment:     toAPIFinancialManagementAdjustment(value.ManagementAdjustmentDirectoryItem),
		EvidenceDigest: value.EvidenceDigest, ProofReference: value.ProofReference,
		ReceivingAccountReference: value.ReceivingAccountReference,
		BusinessPurposeCode:       value.BusinessPurposeCode, PolicyNo: value.PolicyNo,
		PolicyVersion: value.PolicyVersion, Version: value.Version,
		ReviewerID: value.ReviewerID, DecisionCode: value.DecisionCode,
		CommentDigest: value.CommentDigest, ReviewEvidenceDigest: value.ReviewEvidenceDigest,
		DecidedAt: financialUnixMilliseconds(value.DecidedAt), PostedAt: financialUnixMilliseconds(value.PostedAt),
	}
}

// managementBalanceAdjustmentIntent parses exact decimal minor units without floats.
func managementBalanceAdjustmentIntent(value v1.FinancialManagementBalanceAdjustmentIntent) (billing.ManagementBalanceAdjustmentIntent, error) {
	amount, err := parsePositiveFinancialAmount(value.AmountMinor)
	if err != nil {
		return billing.ManagementBalanceAdjustmentIntent{}, err
	}
	return billing.ManagementBalanceAdjustmentIntent{RequestNo: value.RequestNo, CustomerNo: value.CustomerNo,
		CurrencyCode: value.CurrencyCode, AmountMinor: amount, Deduct: value.Deduct,
		ReasonCode: value.ReasonCode, EvidenceDigest: value.EvidenceDigest}, nil
}
