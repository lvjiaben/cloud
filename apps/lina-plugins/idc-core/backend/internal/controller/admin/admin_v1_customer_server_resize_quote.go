// This generated controller delegates staff product edits to the existing authorized owner workflow.
package admin

import (
	"context"
	account "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CustomerServerResizeQuote(ctx context.Context, req *v1.CustomerServerResizeQuoteReq) (res *v1.CustomerServerResizeQuoteRes, err error) {
	selections := make([]catalog.PublicConfigurationSelection, 0, len(req.Selections))
	for _, item := range req.Selections {
		if item != nil {
			selections = append(selections, catalog.PublicConfigurationSelection{OptionNo: item.OptionNo, Quantity: item.Quantity})
		}
	}
	quote, err := c.accessoryPricing.PreviewResizeForStaff(ctx, serveroperation.ResizeQuoteInput{ServiceNo: req.ServiceNo, Revision: req.Revision,
		CurrencyCode: req.CurrencyCode, IdempotencyKey: req.IdempotencyKey, Selections: selections})
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	return &account.ServerResizeQuoteCreateRes{Quote: &account.ServerResizeQuote{QuoteNo: quote.QuoteNo, ServiceNo: quote.ServiceNo, Direction: quote.Direction,
		CurrencyCode: quote.CurrencyCode, CurrentRecurringAmountMinor: quote.CurrentRecurringAmountMinor, TargetRecurringAmountMinor: quote.TargetRecurringAmountMinor,
		ChargeAmountMinor: quote.ChargeAmountMinor, EffectiveAt: quote.EffectiveAt.UnixMilli(), IssuedAt: quote.IssuedAt.UnixMilli(), ExpiresAt: quote.ExpiresAt.UnixMilli(), Reused: quote.Reused}}, nil
}
