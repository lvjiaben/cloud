// This file exports one complete published version for same-system imports.
// Queries are bounded and independent of currency count. Customer-specific
// whole-item pricing is attached by the authenticated channel runtime.

package catalog

import (
	"context"
	"encoding/json"
	"strings"

	"golang.org/x/text/currency"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

// CatalogImportReader reads a bounded, version-fenced sale description. This
// internal composition interface is not an anonymous HTTP route; callers must
// authenticate the channel customer and attach their effective price rules.
type CatalogImportReader interface {
	ReadImportSnapshot(ctx context.Context, publicSlug string) (*channelcommercecap.ImportSnapshot, error)
	// ReadImportCandidates resolves bounded dynamic candidates for the exact published revision.
	ReadImportCandidates(ctx context.Context, input channelcommercecap.ImportCandidatesInput) (*channelcommercecap.ImportCandidates, error)
}

// ReadImportSnapshot preserves configuration and explicit prices from the same
// published version; missing or excessive evidence rejects the entire export.
func (s *publicCatalogReaderImpl) ReadImportSnapshot(ctx context.Context, publicSlug string) (*channelcommercecap.ImportSnapshot, error) {
	slug := strings.ToLower(strings.TrimSpace(publicSlug))
	snapshot, err := s.loadStructuredConfiguration(ctx, slug, "")
	if err != nil {
		return nil, err
	}
	product, err := s.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	projection, err := projectImportSchema(s.revision, snapshot)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	source, dimensions, err := s.loadImportPriceSource(ctx, snapshot.Target.VersionNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	wholePrices, err := s.importedWholePrices(ctx, []string{snapshot.Target.VersionNo}, "")
	if err != nil {
		return nil, err
	}
	prices, err := projectImportPrices(s.revision, slug, snapshot, projection, source, dimensions, wholePrices)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteNotAvailable)
	}
	fulfillment, err := s.loadImportFulfillment(ctx, snapshot.Target.VersionNo)
	if err != nil {
		return nil, err
	}
	result := &channelcommercecap.ImportSnapshot{
		ProtocolVersion: channelcommercecap.ImportProtocolVersion,
		ProductNo:       snapshot.Target.ProductNo,
		Product: channelcommercecap.CatalogProduct{PublicSlug: product.PublicSlug, NameZhCN: product.NameZhCN, NameEnUS: product.NameEnUS,
			SummaryZhCN: product.SummaryZhCN, SummaryEnUS: product.SummaryEnUS, Currencies: product.Currencies, Availability: product.Availability},
		ConfigurationRevision: snapshot.Revision, Schema: projection.schema, Mappings: projection.mappings,
		Prices: prices, PricingComplete: true, Fulfillment: *fulfillment,
		Currencies: make([]channelcommercecap.CatalogCurrency, 0, len(product.Currencies)),
	}
	for _, code := range product.Currencies {
		unit, parseErr := currency.ParseISO(code)
		if parseErr != nil || unit == currency.XXX {
			return nil, bizerr.NewCode(CodePublicCatalogUnavailable)
		}
		precision, _ := currency.Standard.Rounding(unit)
		result.Currencies = append(result.Currencies, channelcommercecap.CatalogCurrency{CurrencyCode: code, Precision: precision})
	}
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) > maximumPublicCatalogResponseBytes-publicCatalogEnvelopeReserve {
		return nil, bizerr.NewCode(CodePublicCatalogUnavailable)
	}
	if err = s.recheckStructuredConfiguration(ctx, snapshot); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *publicCatalogReaderImpl) loadImportFulfillment(ctx context.Context, versionNo string) (*channelcommercecap.ImportFulfillment, error) {
	var row struct {
		Granularity string `orm:"granularity_code"`
		Ongoing     bool   `orm:"ongoing_management"`
		Renewal     string `orm:"renewal_mode_code"`
	}
	if err := s.database.GetScan(ctx, &row, `SELECT granularity_code,ongoing_management,renewal_mode_code
FROM plugin_idc_core_catalog_product_version_fulfillment_policy WHERE tenant_id=0 AND version_no=?`, versionNo); err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	if row.Granularity != productfulfillmentcap.GranularityPerOrderItem && row.Granularity != productfulfillmentcap.GranularityPerQuantityUnit {
		return nil, bizerr.NewCode(CodePublicCatalogUnavailable)
	}
	switch row.Renewal {
	case productfulfillmentcap.RenewalModeUnavailable:
	case productfulfillmentcap.RenewalModeLocalEntitlement, productfulfillmentcap.RenewalModeProviderConfirmed:
		if !row.Ongoing {
			return nil, bizerr.NewCode(CodePublicCatalogUnavailable)
		}
	default:
		return nil, bizerr.NewCode(CodePublicCatalogUnavailable)
	}
	return &channelcommercecap.ImportFulfillment{Granularity: row.Granularity, OngoingService: row.Ongoing, RenewalMode: row.Renewal}, nil
}
