package account

import (
	"sort"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/renewal"
)

func toAPIFinancialOverview(value *billing.AccountFinancialOverview) *v1.FinancialOverview {
	if value == nil {
		return nil
	}
	result := &v1.FinancialOverview{
		Balances:                make([]*v1.FinancialBalance, 0, len(value.Balances)),
		OpenReceivables:         make([]*v1.FinancialReceivableSummary, 0, len(value.OpenReceivableByCurrency)),
		OpenInvoiceCount:        value.OpenInvoiceCount,
		AutomaticRenewalEnabled: value.AutomaticRenewalEnabled,
		RechargeAvailable:       value.RechargeAvailable, RechargeReasonCode: value.RechargeReasonCode,
	}
	for _, item := range value.Balances {
		result.Balances = append(result.Balances, &v1.FinancialBalance{
			AccountTypeCode: item.AccountTypeCode, CurrencyCode: item.CurrencyCode,
			StatusCode: item.StatusCode, Withdrawable: item.Withdrawable,
			BalanceMinor: amount(item.BalanceMinor), HeldBalanceMinor: amount(item.HeldBalanceMinor),
			AvailableBalanceMinor: amount(item.AvailableBalanceMinor), UpdatedAt: item.UpdatedAt.UnixMilli(),
		})
	}
	currencies := make([]string, 0, len(value.OpenReceivableByCurrency))
	for currency := range value.OpenReceivableByCurrency {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	for _, currency := range currencies {
		result.OpenReceivables = append(result.OpenReceivables, &v1.FinancialReceivableSummary{
			CurrencyCode: currency, OutstandingMinor: amount(value.OpenReceivableByCurrency[currency]),
		})
	}
	return result
}

func toAPIFinancialBalancePayment(value *billing.AccountBalancePayment) *v1.FinancialBalancePayment {
	if value == nil {
		return nil
	}
	return &v1.FinancialBalancePayment{
		TransactionNo: value.TransactionNo, InvoiceNo: value.InvoiceNo,
		CurrencyCode: value.CurrencyCode, StatusCode: value.StatusCode,
		PaidAmountMinor:        amount(value.PaidAmountMinor),
		OutstandingAmountMinor: amount(value.OutstandingAmountMinor),
		BalanceMinor:           amount(value.BalanceMinor),
		AvailableBalanceMinor:  amount(value.AvailableBalanceMinor), Reused: value.Reused,
	}
}

func toAPIFinancialCreditSettlement(value *billing.AccountCreditPayment) *v1.FinancialCreditSettlement {
	if value == nil {
		return nil
	}
	return &v1.FinancialCreditSettlement{
		SettlementNo: value.SettlementNo, InvoiceNo: value.InvoiceNo,
		OrderNo: value.OrderNo, CurrencyCode: value.CurrencyCode,
		CreditAmountMinor: amount(value.CreditAmountMinor),
		StatusCode:        value.StatusCode, Reused: value.Reused,
	}
}

func toAPIFinancialInvoiceMerge(value *billing.AccountInvoiceMerge) *v1.FinancialInvoiceMerge {
	if value == nil {
		return nil
	}
	result := &v1.FinancialInvoiceMerge{
		TransformNo: value.TransformNo, CurrencyCode: value.CurrencyCode,
		SourceInvoiceNos:    append([]string(nil), value.SourceInvoiceNos...),
		DestinationInvoices: make([]*v1.FinancialInvoiceMergeDestination, 0, len(value.DestinationInvoices)),
		Reused:              value.Reused,
	}
	for _, invoice := range value.DestinationInvoices {
		result.DestinationInvoices = append(result.DestinationInvoices, &v1.FinancialInvoiceMergeDestination{
			InvoiceNo: invoice.InvoiceNo, StatusCode: invoice.StatusCode,
			TotalAmountMinor:       amount(invoice.TotalAmountMinor),
			OutstandingAmountMinor: amount(invoice.OutstandingAmountMinor),
			PaymentDueAt:           invoice.PaymentDueAt,
		})
	}
	return result
}

func toAPIFinancialRenewalPolicy(value *renewal.Policy) *v1.FinancialRenewalPolicy {
	if value == nil {
		return nil
	}
	return &v1.FinancialRenewalPolicy{
		PolicyNo: value.PolicyNo, EntitlementNo: value.EntitlementNo,
		PolicyVersion: value.PolicyVersion, EntitlementVersion: value.EntitlementVersion,
		AutoRenewEnabled: value.AutoRenewEnabled, RenewalCycles: value.RenewalCycles,
		LeadSeconds: value.LeadSeconds, MaximumAttempts: value.MaximumAttempts,
		RetryIntervalSeconds:       value.RetryIntervalSeconds,
		FailureNotificationEnabled: value.FailureNotificationEnabled, Reused: value.Reused,
	}
}

func toAPIFinancialInvoices(values []billing.InvoiceDirectoryItem) []*v1.FinancialInvoice {
	result := make([]*v1.FinancialInvoice, 0, len(values))
	for _, value := range values {
		result = append(result, toAPIFinancialInvoice(value))
	}
	return result
}

func toAPIFinancialInvoice(value billing.InvoiceDirectoryItem) *v1.FinancialInvoice {
	return &v1.FinancialInvoice{
		InvoiceNo: value.InvoiceNo, OrderNo: value.OrderNo, SourceCode: value.SourceCode,
		CurrencyCode: value.CurrencyCode, StatusCode: value.StatusCode, ItemCount: value.ItemCount,
		TotalAmountMinor: amount(value.TotalAmountMinor), PaidAmountMinor: amount(value.PaidAmountMinor),
		WaivedAmountMinor:      amount(value.WaivedAmountMinor),
		CreditAmountMinor:      amount(value.CreditAmountMinor),
		OutstandingAmountMinor: amount(value.OutstandingAmountMinor),
		RefundableAmountMinor:  amount(value.RefundableAmountMinor), RefundedAmountMinor: amount(value.RefundedAmountMinor),
		PaymentDueAt: value.PaymentDueAt.UnixMilli(), IssuedAt: unixMillis(value.IssuedAt),
		SettledAt: unixMillis(value.SettledAt), CreatedAt: value.CreatedAt.UnixMilli(),
		CanPayFromBalance: value.CanPayFromBalance, CanMerge: value.CanMerge,
	}
}

func toAPIFinancialInvoiceDetail(value *billing.InvoiceDirectoryDetail) *v1.FinancialInvoiceDetail {
	if value == nil {
		return nil
	}
	result := &v1.FinancialInvoiceDetail{
		Invoice:        toAPIFinancialInvoice(value.Invoice),
		Items:          make([]*v1.FinancialInvoiceItem, 0, len(value.Items)),
		NextItemCursor: value.NextCursor,
	}
	for _, item := range value.Items {
		result.Items = append(result.Items, &v1.FinancialInvoiceItem{
			InvoiceItemNo: item.InvoiceItemNo, LineNumber: item.LineNumber,
			ItemTypeCode: item.ItemTypeCode, SourceReference: item.SourceReference,
			DescriptionZhCN: item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS,
			SubtotalMinor: amount(item.SubtotalMinor), DiscountMinor: amount(item.DiscountMinor),
			NetAmountMinor: amount(item.NetAmountMinor), TaxAmountMinor: amount(item.TaxAmountMinor),
			TotalAmountMinor: amount(item.TotalAmountMinor),
		})
	}
	return result
}

func toAPIFinancialTransactions(values []billing.TransactionDirectoryItem) []*v1.FinancialTransaction {
	result := make([]*v1.FinancialTransaction, 0, len(values))
	for _, value := range values {
		result = append(result, &v1.FinancialTransaction{
			TransactionNo: value.TransactionNo, TransactionTypeCode: value.TransactionTypeCode,
			CurrencyCode: value.CurrencyCode, AccountTypeCode: value.AccountTypeCode,
			AmountMinor: amount(value.AmountMinor), ChangeMinor: amount(value.ChangeMinor),
			RelatedInvoiceNo: value.RelatedInvoiceNo, PostedAt: value.PostedAt.UnixMilli(),
		})
	}
	return result
}

func toAPIFinancialRenewalServices(values []billing.RenewalServiceDirectoryItem) []*v1.FinancialRenewalService {
	result := make([]*v1.FinancialRenewalService, 0, len(values))
	for _, value := range values {
		result = append(result, &v1.FinancialRenewalService{
			EntitlementNo: value.EntitlementNo, ProductNameZhCN: value.ProductNameZhCN,
			ProductNameEnUS: value.ProductNameEnUS,
			CurrencyCode:    value.CurrencyCode, EntitlementStatusCode: value.EntitlementStatusCode,
			PeriodUnitCode: value.PeriodUnitCode, PeriodCount: value.PeriodCount,
			EntitlementExpiresAt: value.EntitlementExpiresAt.UnixMilli(), EntitlementVersion: value.EntitlementVersion,
			PolicyNo: value.PolicyNo, PolicyVersion: value.PolicyVersion,
			AutoRenewEnabled: value.AutoRenewEnabled, RenewalCycles: value.RenewalCycles,
			LeadSeconds: value.LeadSeconds, MaximumAttempts: value.MaximumAttempts,
			RetryIntervalSeconds:       value.RetryIntervalSeconds,
			FailureNotificationEnabled: value.FailureNotificationEnabled,
			LatestAttemptOutcomeCode:   value.LatestAttemptOutcomeCode,
			LatestAttemptFailureCode:   value.LatestAttemptFailureCode,
			LatestAttemptAt:            unixMillis(value.LatestAttemptAt), NextAttemptAt: unixMillis(value.NextAttemptAt),
			CanConfigureAutomaticRenewal: value.CanConfigureAutomaticRenewal,
		})
	}
	return result
}
