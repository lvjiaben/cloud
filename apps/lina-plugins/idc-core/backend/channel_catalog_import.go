// This file attaches customer-effective whole-item rules to complete ordinary
// catalog facts. Negotiated prices are never split into fictitious components.

package backend

import (
	"context"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (runtime *channelCommerceRuntime) ReadCatalogImportSnapshot(ctx context.Context, customerNo, slug string) (*channelcommercecap.ImportSnapshot, error) {
	if _, err := runtime.requirePayer(ctx, customerNo); err != nil {
		return nil, err
	}
	reader, ok := runtime.catalog.(catalog.CatalogImportReader)
	if !ok || runtime.pricing == nil {
		return nil, bizerr.NewCode(catalog.CodePublicCatalogUnavailable)
	}
	if customerNo == "" {
		return nil, bizerr.NewCode(catalog.CodePublicCatalogUnavailable)
	}
	snapshot, err := reader.ReadImportSnapshot(ctx, slug)
	if err != nil {
		return nil, err
	}
	if snapshot == nil || snapshot.ProductNo == "" {
		return nil, bizerr.NewCode(catalog.CodePublicCatalogUnavailable)
	}
	rules, err := runtime.pricing.ResolvePriceRules(ctx, customerNo, snapshot.ProductNo, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	for index := range snapshot.Prices {
		price := &snapshot.Prices[index]
		for _, rule := range rules {
			if rule.CurrencyCode == price.CurrencyCode && rule.ChargeTypeCode == price.ChargeType && rule.PeriodUnitCode == price.PeriodUnit && rule.PeriodCount == price.PeriodCount {
				price.WholeItemRule = &channelcommercecap.ImportWholeItemRule{Prior: price.WholeItemRule, Kind: rule.Kind, FixedAmountMinor: rule.FixedAmountMinor, DiscountBasisPoints: rule.DiscountBasisPoints}
				break
			}
		}
	}
	if err = snapshot.Validate(); err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodePublicQuoteNotAvailable)
	}
	return snapshot, nil
}

var _ channelcommercecap.CatalogImportRuntime = (*channelCommerceRuntime)(nil)

// ReadImportCandidates obtains only owner-projected candidate tokens.
func (runtime *channelCommerceRuntime) ReadImportCandidates(ctx context.Context, input channelcommercecap.ImportCandidatesInput) (*channelcommercecap.ImportCandidates, error) {
	ctx, err := runtime.fileCustomerContext(ctx, input.CustomerNo)
	if err != nil {
		return nil, err
	}
	reader, ok := runtime.catalog.(catalog.CatalogImportReader)
	if !ok {
		return nil, bizerr.NewCode(catalog.CodePublicCatalogUnavailable)
	}
	return reader.ReadImportCandidates(ctx, input)
}
