package account

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerResizeOptions(ctx context.Context, req *v1.ServerResizeOptionsReq) (*v1.ServerResizeOptionsRes, error) {
	value, err := c.accessoryPricing.ResizeOptionsCurrentCustomer(ctx, req.ServiceNo)
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	result := &v1.ServerResizeOptionsRes{PublicSlug: value.PublicSlug, CurrencyCode: value.CurrencyCode, Revision: value.Revision,
		Groups: make([]*v1.ServerResizeGroup, 0, len(value.Groups)), Rules: make([]*v1.ServerResizeRule, 0, len(value.Rules)),
		CurrentSelections: make([]*v1.ServerResizeSelection, 0, len(value.CurrentSelections))}
	for _, group := range value.Groups {
		mapped := &v1.ServerResizeGroup{GroupNo: group.GroupNo, SelectionMode: string(group.SelectionMode), NameZhCN: group.NameZhCN,
			NameEnUS: group.NameEnUS, MinimumSelected: group.MinimumSelected, MaximumSelected: group.MaximumSelected, Options: make([]*v1.ServerResizeOption, 0, len(group.Options))}
		for _, option := range group.Options {
			mapped.Options = append(mapped.Options, &v1.ServerResizeOption{OptionNo: option.OptionNo, ComponentType: option.ComponentType,
				NameZhCN: option.NameZhCN, NameEnUS: option.NameEnUS, MinimumQuantity: option.MinimumQuantity, MaximumQuantity: option.MaximumQuantity, DefaultQuantity: option.DefaultQuantity})
		}
		result.Groups = append(result.Groups, mapped)
	}
	for _, rule := range value.Rules {
		result.Rules = append(result.Rules, &v1.ServerResizeRule{RuleType: string(rule.RuleType), SourceOptionNo: rule.SourceOptionNo,
			TargetOptionNo: rule.TargetOptionNo, MinimumTargetQuantity: rule.MinimumTargetQuantity, MaximumTargetQuantity: rule.MaximumTargetQuantity})
	}
	for _, selection := range value.CurrentSelections {
		result.CurrentSelections = append(result.CurrentSelections, &v1.ServerResizeSelection{OptionNo: selection.OptionNo, Quantity: selection.Quantity})
	}
	return result, nil
}

func (c *ControllerV1) ServerResizeQuoteCreate(ctx context.Context, req *v1.ServerResizeQuoteCreateReq) (*v1.ServerResizeQuoteCreateRes, error) {
	selections := make([]catalog.PublicConfigurationSelection, 0, len(req.Selections))
	for _, item := range req.Selections {
		if item != nil {
			selections = append(selections, catalog.PublicConfigurationSelection{OptionNo: item.OptionNo, Quantity: item.Quantity})
		}
	}
	quote, err := c.accessoryPricing.PreviewResizeCurrentCustomer(ctx, serveroperation.ResizeQuoteInput{ServiceNo: req.ServiceNo, Revision: req.Revision,
		TermNo: req.TermNo, CurrencyCode: req.CurrencyCode, IdempotencyKey: req.IdempotencyKey, Selections: selections})
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	return &v1.ServerResizeQuoteCreateRes{Quote: &v1.ServerResizeQuote{QuoteNo: quote.QuoteNo, ServiceNo: quote.ServiceNo, Direction: quote.Direction,
		CurrencyCode: quote.CurrencyCode, CurrentRecurringAmountMinor: quote.CurrentRecurringAmountMinor, TargetRecurringAmountMinor: quote.TargetRecurringAmountMinor,
		ChargeAmountMinor: quote.ChargeAmountMinor, EffectiveAt: quote.EffectiveAt.UnixMilli(), IssuedAt: quote.IssuedAt.UnixMilli(), ExpiresAt: quote.ExpiresAt.UnixMilli(), Reused: quote.Reused}}, nil
}

func (c *ControllerV1) ServerResizeCheckoutCreate(ctx context.Context, req *v1.ServerResizeCheckoutCreateReq) (*v1.ServerResizeCheckoutCreateRes, error) {
	value, err := c.accessoryPricing.CheckoutResizeCurrentCustomer(ctx, serveroperation.ResizeCheckoutInput{QuoteNo: req.QuoteNo, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	return &v1.ServerResizeCheckoutCreateRes{Checkout: &v1.ServerResizeCheckout{OperationNo: value.OperationNo, OrderNo: value.OrderNo, InvoiceNo: value.InvoiceNo,
		ServiceNo: value.ServiceNo, Direction: value.Direction, Status: value.Status, CurrencyCode: value.CurrencyCode, ChargeAmountMinor: value.ChargeAmountMinor,
		EffectiveAt: value.EffectiveAt.UnixMilli(), Reused: value.Reused}}, nil
}
