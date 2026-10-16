// This file handles owned trial conversion quote create using the startup renewal service.

package account

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/renewal"
	"strconv"
)

func (c *ControllerV1) TrialConversionQuoteCreate(ctx context.Context, req *v1.TrialConversionQuoteCreateReq) (res *v1.TrialConversionQuoteCreateRes, err error) {
	quote, err := c.renewals.QuoteCurrentCustomer(ctx, renewal.QuoteInput{EntitlementNo: req.EntitlementNo,
		FormalBillingRuleNo: req.BillingRuleNo, RenewalCycles: 1, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.TrialConversionQuoteCreateRes{Quote: &v1.TrialConversionQuote{QuoteNo: quote.QuoteNo, CurrencyCode: quote.CurrencyCode,
		NetAmountMinor: strconv.FormatInt(quote.NetAmountMinor, 10), TaxAmountMinor: strconv.FormatInt(quote.TaxAmountMinor, 10),
		TotalAmountMinor: strconv.FormatInt(quote.TotalAmountMinor, 10), ExpiresAt: quote.ExpiresAt.UnixMilli()}}, nil
}
