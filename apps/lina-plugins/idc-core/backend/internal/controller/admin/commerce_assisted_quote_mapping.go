package admin

import (
	publiccontroller "lina-plugin-idc-core/backend/internal/controller/public"
	"strconv"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func toAPICommerceAssistedQuote(input *commerce.AssistedQuoteRequest) *v1.CommerceAssistedQuote {
	if input == nil {
		return nil
	}
	items := make([]*v1.CommerceAssistedQuoteItem, 0, len(input.Items))
	for _, item := range input.Items {
		items = append(items, &v1.CommerceAssistedQuoteItem{
			RequestItemNo: item.RequestItemNo, LineNumber: item.LineNumber,
			ProductNameZhCN: item.ProductNameZhCN, ProductNameEnUS: item.ProductNameEnUS,
			Quantity: item.Quantity, PurchaseCycles: item.PurchaseCycles,
		})
	}
	result := &v1.CommerceAssistedQuote{
		RequestNo: input.RequestNo, CustomerNo: input.CustomerNo, CurrencyCode: input.CurrencyCode,
		CustomerCode: input.CustomerCode, CustomerDisplayName: input.CustomerDisplayName,
		CustomerPrimaryPhone: input.CustomerPrimaryPhone,
		StatusCode:           string(input.StatusCode), ManualDiscountMinor: strconv.FormatInt(input.ManualDiscountMinor, 10),
		PaymentTermCode: string(input.PaymentTermCode), PaymentDueSeconds: input.PaymentDueSeconds,
		ContractPolicyCode: string(input.ContractPolicyCode), ContractTemplateNo: input.ContractTemplateNo,
		Reason: input.Reason, InternalNote: input.InternalNote, RequestedBy: input.RequestedBy,
		DecisionCode: string(input.DecisionCode), DecisionBy: input.DecisionBy,
		DecisionReason: input.DecisionReason, QuoteNo: input.QuoteNo, OrderNo: input.OrderNo,
		OrderStatusCode: input.OrderStatusCode, SubtotalMinor: strconv.FormatInt(input.SubtotalMinor, 10),
		DiscountMinor:    strconv.FormatInt(input.DiscountMinor, 10),
		TotalAmountMinor: strconv.FormatInt(input.TotalAmountMinor, 10),
		ExpiresAt:        input.ExpiresAt.UnixMilli(), CreatedAt: input.CreatedAt.UnixMilli(), Items: items,
		CanApprove: input.CanApprove, CanReject: input.CanReject, CanCheckout: input.CanCheckout,
		Reused: input.Reused,
	}
	if input.DecisionAt != nil {
		result.DecisionAt = input.DecisionAt.UnixMilli()
	}
	return result
}

func toCommerceAssistedQuoteItems(
	input []*v1.CommerceAssistedQuoteItemInput,
	currencyCode string,
) ([]commerce.AssistedQuoteItemInput, error) {
	result := make([]commerce.AssistedQuoteItemInput, 0, len(input))
	for _, item := range input {
		if item == nil {
			result = append(result, commerce.AssistedQuoteItemInput{})
			continue
		}
		values, err := publiccontroller.MapPurchaseConfigurationInput(item.Values)
		if err != nil {
			return nil, err
		}
		result = append(result, commerce.AssistedQuoteItemInput{
			Quantity: item.Quantity,
			Purchase: catalog.PurchaseConfigurationInput{
				PublicSlug: item.PublicSlug, ConfigurationRevision: item.ConfigurationRevision,
				BillingTermToken: item.BillingTermToken, CurrencyCode: currencyCode,
				PurchaseCycles: item.PurchaseCycles, Values: values,
			},
		})
	}
	return result, nil
}
