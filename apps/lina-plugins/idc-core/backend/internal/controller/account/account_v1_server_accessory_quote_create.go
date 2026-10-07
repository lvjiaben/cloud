package account

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerAccessoryQuoteCreate(ctx context.Context,
	req *v1.ServerAccessoryQuoteCreateReq) (*v1.ServerAccessoryQuoteCreateRes, error) {
	quote, err := c.accessoryPricing.PreviewCurrentCustomer(ctx, serveroperation.AccessoryQuoteInput{
		ServiceNo: req.ServiceNo, Action: serveroperationresourcecap.Action(req.Action),
		Parameters: req.Parameters, IdempotencyKey: req.IdempotencyKey, PackageNo: req.PackageNo,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ServerAccessoryQuoteCreateRes{Quote: toAPIAccessoryQuote(quote)}, nil
}

func toAPIAccessoryQuote(value *serveroperation.AccessoryQuote) *v1.ServerAccessoryQuote {
	if value == nil {
		return nil
	}
	return &v1.ServerAccessoryQuote{QuoteNo: value.QuoteNo, Action: string(value.Action), Kind: string(value.Kind),
		TrafficGB: value.TrafficGB, PackageNameZhCN: value.PackageNameZhCN, PackageNameEnUS: value.PackageNameEnUS,
		CurrencyCode: value.CurrencyCode, RequestedQuantity: value.RequestedQuantity,
		ExistingObjectCount: value.ExistingObjectCount, ReservedCreateCount: value.ReservedCreateCount,
		FreeCountPerServer: value.FreeCountPerServer, ChargedQuantity: value.ChargedQuantity,
		UnitAmountMinor: value.UnitAmountMinor, SubtotalMinor: value.SubtotalMinor,
		NetAmountMinor: value.NetAmountMinor, TaxAmountMinor: value.TaxAmountMinor,
		TotalAmountMinor: value.TotalAmountMinor, PriceIncludesTax: true,
		PeriodUnitCode: value.PeriodUnitCode, PeriodCount: value.PeriodCount,
		PolicyVersion: value.PolicyVersion, IssuedAt: value.IssuedAt.UnixMilli(), ExpiresAt: value.ExpiresAt.UnixMilli()}
}
