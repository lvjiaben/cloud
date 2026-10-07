// This file maps internal tax-invoice projections to public staff DTOs.
package admin

import (
	"time"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoicemanagement"
)

func taxInvoiceManagementItem(
	value *taxinvoicemanagement.Request,
) *v1.TaxInvoiceManagementItem {
	if value == nil {
		return nil
	}
	return &v1.TaxInvoiceManagementItem{
		TaxInvoiceNo: value.TaxInvoiceNo, CustomerNo: value.CustomerNo,
		StatusCode: value.StatusCode, InvoiceTypeCode: value.InvoiceTypeCode,
		ProviderCode: value.ProviderCode, DecisionReasonCode: value.DecisionReasonCode,
		DecisionInternalNote: value.DecisionInternalNote,
		GrossAmountMinor:     value.GrossAmountMinor, NetAmountMinor: value.NetAmountMinor,
		TaxAmountMinor: value.TaxAmountMinor, CurrencyCode: value.CurrencyCode,
		MaskedInvoiceNumber: value.MaskedInvoiceNumber,
		Version:             value.Version, SubmittedAt: value.SubmittedAt.UTC().UnixMilli(),
		ReviewedAt: milliseconds(value.ReviewedAt), CapabilityReady: value.CapabilityReady,
		CapabilityReasonCode:  value.CapabilityReasonCode,
		SupportedInvoiceTypes: append([]string(nil), value.SupportedInvoiceTypes...),
		IssuedAt:              millisecondsValue(value.IssuedAt), DeliveredAt: millisecondsValue(value.DeliveredAt),
		CanceledAt: millisecondsValue(value.CanceledAt), ProviderNotIssuedAt: millisecondsValue(value.ProviderNotIssuedAt),
	}
}

func millisecondsValue(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UTC().UnixMilli()
}

func taxInvoiceActionResult(
	value *taxinvoicemanagement.Result,
) *v1.TaxInvoiceManagementActionResult {
	if value == nil {
		return nil
	}
	return &v1.TaxInvoiceManagementActionResult{
		Item: taxInvoiceManagementItem(value.Request), Applied: value.Applied, Reused: value.Reused,
	}
}
