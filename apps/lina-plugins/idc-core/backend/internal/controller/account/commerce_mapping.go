package account

import (
	"strconv"
	"time"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func amount(value int64) string { return strconv.FormatInt(value, 10) }

func unixMillis(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UnixMilli()
}

func toAPICommerceCart(cart *commerce.Cart) *v1.CommerceCart {
	if cart == nil {
		return nil
	}
	result := &v1.CommerceCart{
		CartNo: cart.CartNo, CurrencyCode: cart.CurrencyCode, Version: cart.Version,
		Items: make([]*v1.CommerceCartItem, 0, len(cart.Items)),
	}
	for _, item := range cart.Items {
		mapped := &v1.CommerceCartItem{
			CartItemNo: item.CartItemNo, ProductNameZhCN: item.ProductNameZhCN,
			ProductNameEnUS: item.ProductNameEnUS, PublicSlug: item.PublicSlug,
			ConfigurationRequired: item.ConfigurationRequired, ConfigurationFrozen: item.ConfigurationFrozen,
			ConfigurationRevision: item.ConfigurationRevision, Quantity: item.Quantity,
			PurchaseCycles: item.PurchaseCycles, StatusCode: item.StatusCode,
			IssueCode: item.IssueCode, RepairAction: item.RepairAction, Version: item.Version,
		}
		result.Items = append(result.Items, mapped)
	}
	result.LatestQuote = toAPICommerceQuote(cart.LatestQuote)
	return result
}

func toAPICommerceQuote(quote *commerce.Quote) *v1.CommerceQuote {
	if quote == nil {
		return nil
	}
	result := &v1.CommerceQuote{
		QuoteNo: quote.QuoteNo, CartNo: quote.CartNo, CurrencyCode: quote.CurrencyCode,
		CartVersion: quote.CartVersion, SubtotalMinor: amount(quote.SubtotalMinor),
		DiscountMinor: amount(quote.DiscountMinor), NetAmountMinor: amount(quote.NetAmountMinor),
		TaxAmountMinor: amount(quote.TaxAmountMinor), TotalAmountMinor: amount(quote.TotalAmountMinor),
		IssuedAt: quote.IssuedAt.UnixMilli(), ExpiresAt: quote.ExpiresAt.UnixMilli(),
		Items: make([]*v1.CommerceQuoteItem, 0, len(quote.Items)),
	}
	for _, item := range quote.Items {
		mapped := &v1.CommerceQuoteItem{
			QuoteItemNo: item.QuoteItemNo, CartItemNo: item.CartItemNo,
			ProductNameZhCN: item.ProductNameZhCN, ProductNameEnUS: item.ProductNameEnUS,
			PublicSlug: item.PublicSlug, ConfigurationRequired: item.ConfigurationRequired,
			ConfigurationFrozen: item.ConfigurationFrozen, ConfigurationRevision: item.ConfigurationRevision,
			Quantity: item.Quantity, PurchaseCycles: item.PurchaseCycles,
			FulfillmentModeCode: item.FulfillmentModeCode, RiskDecisionCode: item.RiskDecisionCode,
			InventoryDecisionCode: item.InventoryDecisionCode,
			SetupSubtotalMinor:    amount(item.SetupSubtotalMinor), PeriodSubtotalMinor: amount(item.PeriodSubtotalMinor),
			SubtotalMinor: amount(item.SubtotalMinor), DiscountMinor: amount(item.DiscountMinor),
			NetAmountMinor: amount(item.NetAmountMinor), TaxAmountMinor: amount(item.TaxAmountMinor),
			TotalAmountMinor: amount(item.TotalAmountMinor),
			Adjustments:      make([]*v1.CommerceQuoteAdjustment, 0, len(item.Adjustments)),
		}
		for _, adjustment := range item.Adjustments {
			mapped.Adjustments = append(mapped.Adjustments, &v1.CommerceQuoteAdjustment{
				TypeCode: adjustment.TypeCode, SourceReference: adjustment.SourceReference,
				AmountMinor: amount(adjustment.AmountMinor),
			})
		}
		result.Items = append(result.Items, mapped)
	}
	return result
}

func toAPICommerceCheckout(checkout *commerce.Checkout) *v1.CommerceCheckout {
	if checkout == nil {
		return nil
	}
	return &v1.CommerceCheckout{
		OrderNo: checkout.OrderNo, InvoiceNo: checkout.InvoiceNo, QuoteNo: checkout.QuoteNo,
		StatusCode: checkout.StatusCode, CurrencyCode: checkout.CurrencyCode, ItemCount: checkout.ItemCount,
		TotalAmountMinor: amount(checkout.TotalAmountMinor), RequiresContract: checkout.RequiresContract,
		PaymentDueAt: checkout.PaymentDueAt.UnixMilli(), Reused: checkout.Reused,
	}
}

func toAPICommercePaymentPreparation(value *commerce.PaymentPreparation) *v1.CommercePaymentPreparation {
	if value == nil {
		return nil
	}
	return &v1.CommercePaymentPreparation{
		ContextNo: value.ContextNo, OrderNo: value.OrderNo, InvoiceNo: value.InvoiceNo,
		CustomerTypeCode: value.CustomerTypeCode, StatusCode: value.StatusCode,
		ReasonCode: value.ReasonCode, IdentityStatusCode: value.IdentityStatusCode,
		CurrencyCode: value.CurrencyCode, AmountDueMinor: amount(value.AmountDueMinor),
		RequiresContract: value.RequiresContract, PaymentDueAt: value.PaymentDueAt.UnixMilli(),
		LastEvaluatedAt: value.LastEvaluatedAt.UnixMilli(), Ready: value.Ready, Reused: value.Reused,
	}
}

func toAPICommerceCancellation(value *commerce.OrderCancellation) *v1.CommerceOrderCancellation {
	if value == nil {
		return nil
	}
	return &v1.CommerceOrderCancellation{
		CancellationNo: value.CancellationNo, OrderNo: value.OrderNo,
		StatusCode: string(value.Status), ReasonCode: string(value.Reason),
		RequestedAt: value.RequestedAt.UnixMilli(), CompletedAt: unixMillis(value.CompletedAt), Reused: value.Reused,
	}
}

func toAPICommerceOrderSummaries(values []commerce.OrderSummary) []*v1.CommerceOrderSummary {
	result := make([]*v1.CommerceOrderSummary, 0, len(values))
	for _, value := range values {
		result = append(result, toAPICommerceOrderSummary(value))
	}
	return result
}

func toAPICommerceOrderSummary(value commerce.OrderSummary) *v1.CommerceOrderSummary {
	return &v1.CommerceOrderSummary{
		CustomerCode: value.CustomerCode, CustomerDisplayName: value.CustomerDisplayName,
		CustomerPrimaryPhone: value.CustomerPrimaryPhone,
		ProductSummary:       value.ProductSummary, IPAddress: value.IPAddress,
		PaymentMethodCode: value.PaymentMethodCode, Remark: value.Remark,
		OrderNo: value.OrderNo, CustomerNo: value.CustomerNo,
		CustomerTypeCode: value.CustomerTypeCode, StatusCode: value.StatusCode,
		RiskDecisionCode: value.RiskDecisionCode, CurrencyCode: value.CurrencyCode,
		ItemCount: value.ItemCount, TotalAmountMinor: amount(value.TotalAmountMinor),
		RequiresContract: value.RequiresContract, PaymentDueAt: unixMillis(value.PaymentDueAt),
		CreatedAt: value.CreatedAt.UnixMilli(), CanCancel: value.CanCancel,
		CanPreparePayment: value.CanPreparePayment, AssistedRequestNo: value.AssistedRequestNo,
		PaymentTermCode: value.PaymentTermCode,
	}
}

// ToAPICommerceOrderDetail is shared by the staff controller while preserving
// the service-derived customer number and action flags.
func ToAPICommerceOrderDetail(value *commerce.OrderDetail) *v1.CommerceOrderDetail {
	if value == nil {
		return nil
	}
	result := &v1.CommerceOrderDetail{
		Summary:             toAPICommerceOrderSummary(value.Summary),
		Items:               make([]*v1.CommerceOrderItem, 0, len(value.Items)),
		Deliveries:          make([]*v1.CommerceDelivery, 0, len(value.Deliveries)),
		ServiceEntitlements: make([]*v1.CommerceServiceEntitlement, 0, len(value.ServiceEntitlements)),
		Reviews:             make([]*v1.CommerceOrderReview, 0, len(value.Reviews)),
		CanApproveRisk:      value.CanApproveRisk, CanRejectRisk: value.CanRejectRisk,
	}
	if value.Invoice != nil {
		invoice := value.Invoice
		result.Invoice = &v1.CommerceOrderInvoice{
			InvoiceNo: invoice.InvoiceNo, StatusCode: invoice.StatusCode, CurrencyCode: invoice.CurrencyCode,
			SubtotalMinor: amount(invoice.SubtotalMinor), DiscountMinor: amount(invoice.DiscountMinor),
			NetAmountMinor: amount(invoice.NetAmountMinor), TaxAmountMinor: amount(invoice.TaxAmountMinor),
			TotalAmountMinor: amount(invoice.TotalAmountMinor), PaidAmountMinor: amount(invoice.PaidAmountMinor),
			RefundableAmountMinor: amount(invoice.RefundableAmountMinor), RefundedAmountMinor: amount(invoice.RefundedAmountMinor),
			PaymentDueAt: unixMillis(invoice.PaymentDueAt), IssuedAt: unixMillis(invoice.IssuedAt),
			SettledAt: unixMillis(invoice.SettledAt), CancelledAt: unixMillis(invoice.CancelledAt),
		}
	}
	for _, item := range value.Items {
		result.Items = append(result.Items, &v1.CommerceOrderItem{
			OrderItemNo: item.OrderItemNo, LineNumber: item.LineNumber,
			ProductNameZhCN: item.ProductNameZhCN, ProductNameEnUS: item.ProductNameEnUS,
			Quantity: item.Quantity, PurchaseCycles: item.PurchaseCycles,
			ChargeTypeCode: item.ChargeTypeCode, PeriodUnitCode: item.PeriodUnitCode, PeriodCount: item.PeriodCount,
			RequiresContract: item.RequiresContract, FulfillmentModeCode: item.FulfillmentModeCode,
			RiskDecisionCode: item.RiskDecisionCode, InventoryDecisionCode: item.InventoryDecisionCode,
			SetupSubtotalMinor: amount(item.SetupSubtotalMinor), PeriodSubtotalMinor: amount(item.PeriodSubtotalMinor),
			SubtotalMinor: amount(item.SubtotalMinor), DiscountMinor: amount(item.DiscountMinor),
			NetAmountMinor: amount(item.NetAmountMinor), TaxAmountMinor: amount(item.TaxAmountMinor),
			TotalAmountMinor: amount(item.TotalAmountMinor),
		})
	}
	productNames := make(map[string][2]string, len(value.Items))
	for _, item := range value.Items {
		productNames[item.OrderItemNo] = [2]string{item.ProductNameZhCN, item.ProductNameEnUS}
	}
	for _, delivery := range value.Deliveries {
		names := productNames[delivery.OrderItemNo]
		result.Deliveries = append(result.Deliveries, &v1.CommerceDelivery{
			ProductNameZhCN: names[0], ProductNameEnUS: names[1], Quantity: delivery.Quantity,
			StatusCode: safeAPIDeliveryStatus(delivery.StatusCode), OngoingService: delivery.OngoingManagement,
			StartedAt: unixMillis(delivery.StartedAt), CompletedAt: unixMillis(delivery.CompletedAt),
			UpdatedAt: delivery.UpdatedAt.UnixMilli(),
		})
	}
	for _, entitlement := range value.ServiceEntitlements {
		names := productNames[entitlement.OrderItemNo]
		result.ServiceEntitlements = append(result.ServiceEntitlements, &v1.CommerceServiceEntitlement{
			EntitlementNo: entitlement.EntitlementNo, OrderItemNo: entitlement.OrderItemNo,
			ProductNameZhCN: names[0], ProductNameEnUS: names[1], StatusCode: entitlement.StatusCode,
			StartedAt: entitlement.StartedAt.UnixMilli(), ExpiresAt: unixMillis(entitlement.ExpiresAt),
		})
	}
	if value.Cancellation != nil {
		cancellation := value.Cancellation
		result.Cancellation = &v1.CommerceOrderCancellationDetail{
			CancellationNo: cancellation.CancellationNo, SourceCode: cancellation.SourceCode,
			ReasonCode: cancellation.ReasonCode, StatusCode: cancellation.StatusCode,
			PaymentClosureOutcomeCode: cancellation.PaymentClosureOutcomeCode,
			RequestedAt:               unixMillis(cancellation.RequestedAt), CompletedAt: unixMillis(cancellation.CompletedAt),
		}
	}
	for _, review := range value.Reviews {
		result.Reviews = append(result.Reviews, &v1.CommerceOrderReview{
			ReviewNo: review.ReviewNo, ReviewTypeCode: review.ReviewTypeCode,
			StatusCode: review.StatusCode, ReasonCode: review.ReasonCode,
			RequestedAt: unixMillis(review.RequestedAt), ReviewedAt: unixMillis(review.ReviewedAt),
		})
	}
	if value.PaymentPreparation != nil {
		preparation := value.PaymentPreparation
		result.PaymentPreparation = &v1.CommerceOrderPaymentPreparation{
			ContextNo: preparation.ContextNo, StatusCode: preparation.StatusCode,
			ReasonCode: preparation.ReasonCode, IdentityStatusCode: preparation.IdentityStatusCode,
			LastEvaluatedAt: unixMillis(preparation.LastEvaluatedAt), ReadyAt: unixMillis(preparation.ReadyAt),
			ExpiresAt: unixMillis(preparation.ExpiresAt),
		}
	}
	return result
}

func safeAPIDeliveryStatus(value string) string {
	switch value {
	case "awaiting_payment", "awaiting_review", "completed", "not_completed", "cancelled":
		return value
	default:
		return "processing"
	}
}

// ToAPICommerceOrderSummaries is shared by the staff order queue.
func ToAPICommerceOrderSummaries(values []commerce.OrderSummary) []*v1.CommerceOrderSummary {
	return toAPICommerceOrderSummaries(values)
}
