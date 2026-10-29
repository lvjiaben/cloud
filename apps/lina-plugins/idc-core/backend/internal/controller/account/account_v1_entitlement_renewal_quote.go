// This file delegates current-customer ordinary renewal quotes to the shared owner.
package account

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/renewal"
	"strconv"
)

func (c *ControllerV1) EntitlementRenewalQuote(ctx context.Context, req *v1.EntitlementRenewalQuoteReq) (*v1.EntitlementRenewalQuoteRes, error) {
	quote, err := c.renewals.QuoteCurrentCustomer(ctx, renewal.QuoteInput{EntitlementNo: req.EntitlementNo, RenewalCycles: req.RenewalCycles, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.EntitlementRenewalQuoteRes{Quote: &v1.EntitlementRenewalQuote{QuoteNo: quote.QuoteNo, CurrencyCode: quote.CurrencyCode, TotalAmountMinor: strconv.FormatInt(quote.TotalAmountMinor, 10), TaxAmountMinor: strconv.FormatInt(quote.TaxAmountMinor, 10), ProjectedExpiresAt: quote.ProjectedExpiresAt.UnixMilli(), ExpiresAt: quote.ExpiresAt.UnixMilli()}}, nil
}
