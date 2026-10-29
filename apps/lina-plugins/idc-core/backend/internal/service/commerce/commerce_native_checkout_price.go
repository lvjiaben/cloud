// Checkout rechecks native supplier prices only after the quote and customer
// are authorized. Historical/local fixed quotes retain their existing behavior.
package commerce

import (
	"context"
	"crypto/subtle"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (s *serviceImpl) requireNativeCheckoutPrice(ctx context.Context, quote *entity.CommerceQuoteSnapshot, items []entity.CommerceQuoteItemSnapshot, quantities map[string]int) error {
	verifier, ok := s.quoteCatalog.(catalog.NativeCheckoutPriceVerifier)
	if !ok {
		return nil
	}
	input := catalog.PurchaseQuoteInput{CurrencyCode: quote.CurrencyCode, Lines: make([]catalog.PurchaseQuoteLineInput, 0, len(items))}
	for _, item := range items {
		input.Lines = append(input.Lines, catalog.PurchaseQuoteLineInput{ReferenceNo: item.QuoteItemNo, ProductNo: item.ProductNo, VersionNo: item.VersionNo, BillingRuleNo: item.BillingRuleNo, ConfigurationSnapshotNo: item.ConfigurationSnapshotNo, Quantity: quantities[item.QuoteItemNo], PurchaseCycles: item.PurchaseCycles})
	}
	current, err := verifier.VerifyNativeCheckoutPrices(ctx, input)
	if err != nil {
		return bizerr.WrapCode(err, CodeQuoteNotAvailable)
	}
	if len(current) == 0 {
		return nil
	}
	itemNos := make([]string, 0, len(current))
	for number := range current {
		itemNos = append(itemNos, number)
	}
	var charges []struct {
		QuoteItemNo, PriceSourceDigest                      string
		ChargeAmountMinor, SetupFeeMinor, PricedAmountMinor int64
	}
	if err = s.database.Model(do.CommerceQuoteChargeSnapshot{}).Ctx(ctx).Where(do.CommerceQuoteChargeSnapshot{TenantId: 0}).WhereIn("quote_item_no", itemNos).Fields("quote_item_no,price_source_digest,charge_amount_minor,setup_fee_minor,priced_amount_minor").OrderAsc("quote_item_no").Limit(len(itemNos) + 1).Scan(&charges); err != nil {
		return err
	}
	if len(charges) != len(current) {
		return bizerr.NewCode(CodeQuoteNotAvailable)
	}
	for _, saved := range charges {
		live, found := current[saved.QuoteItemNo]
		if !found || subtle.ConstantTimeCompare([]byte(saved.PriceSourceDigest), []byte(live.PriceSourceDigest)) != 1 || saved.ChargeAmountMinor != live.ChargeAmountMinor || saved.SetupFeeMinor != live.SetupFeeMinor || saved.PricedAmountMinor != live.PricedAmountMinor {
			return bizerr.NewCode(CodeQuoteNotAvailable)
		}
	}
	return nil
}
