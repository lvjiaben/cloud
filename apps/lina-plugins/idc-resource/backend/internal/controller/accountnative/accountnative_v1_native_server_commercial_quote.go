package accountnative

import (
	"context"
	"strconv"

	"lina-plugin-idc-resource/backend/internal/service/customeroperations"

	"lina-plugin-idc-resource/backend/api/accountnative/v1"
)

func (c *ControllerV1) NativeServerCommercialQuote(ctx context.Context, req *v1.NativeServerCommercialQuoteReq) (res *v1.NativeServerCommercialQuoteRes, err error) {
	if pathErr := bindNativePath(ctx, "serverNo", &req.ServerNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	quote, err := c.commercial.Quote(ctx, customeroperations.CommercialQuoteCommand{ServerNo: req.ServerNo, Action: req.Action, PackageNo: req.PackageNo, TargetQuoteNo: req.TargetQuoteNo, IdempotencyKey: req.IdempotencyKey, Parameters: req.Parameters})
	if err != nil {
		return nil, publicCommercialError(err)
	}
	res = &v1.NativeServerCommercialQuoteRes{QuoteNo: quote.QuoteNo, Action: string(quote.Action), CurrencyCode: quote.CurrencyCode, TotalAmountMinor: strconv.FormatInt(quote.TotalAmountMinor, 10), TaxAmountMinor: strconv.FormatInt(quote.TaxAmountMinor, 10), ExpiresAt: quote.ExpiresAt.UnixMilli(), FreeCountPerServer: quote.FreeCountPerServer, ExistingObjectCount: quote.ExistingObjectCount, ChargedQuantity: quote.ChargedQuantity, TrafficGB: quote.TrafficGB, PeriodUnit: quote.PeriodUnit, PeriodCount: quote.PeriodCount, Direction: quote.Direction, CurrentRecurringMinor: strconv.FormatInt(quote.CurrentRecurringMinor, 10), TargetRecurringMinor: strconv.FormatInt(quote.TargetRecurringMinor, 10), Reused: quote.Reused}
	if !quote.EffectiveAt.IsZero() {
		stamp := quote.EffectiveAt.UnixMilli()
		res.EffectiveAt = &stamp
	}
	return res, nil
}
