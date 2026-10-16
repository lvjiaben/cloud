// This file exposes a bounded numbered catalog page for upstream import pickers.
// Counting and paging share the public catalog filter; no client-side scanning
// or per-product detail calls are used to invent a total.

package catalog

import (
	"context"
	"encoding/json"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// PublicCatalogNumberedInput selects a numeric page of publicly saleable products.
type PublicCatalogNumberedInput struct {
	NamePrefix string
	CategoryNo string
	Currency   string
	Page       int
	PageSize   int
}

// PublicCatalogNumberedPage contains the exact visible total, including empty pages.
type PublicCatalogNumberedPage struct {
	Products []PublicCatalogProduct
	Total    int
}

func (s *publicCatalogReaderImpl) SearchNumbered(ctx context.Context, input PublicCatalogNumberedInput) (*PublicCatalogNumberedPage, error) {
	if input.Page < 1 || input.Page > 1_000_000 || input.PageSize < 1 || input.PageSize > 100 {
		return nil, bizerr.NewCode(CodePublicCatalogInvalid)
	}
	normalized, err := normalizePublicCatalogSearch(PublicCatalogSearchInput{NamePrefix: input.NamePrefix, CategoryNo: input.CategoryNo, Currency: input.Currency, Limit: 1}, s.cursor)
	if err != nil {
		return nil, err
	}
	var (
		args = []any{idccap.DefaultTenantID, "", "", normalized.namePrefix, normalized.likePrefix, normalized.likePrefix, normalized.categoryNo, normalized.categoryNo, normalized.currency, normalized.currency}
		page = &PublicCatalogNumberedPage{Products: make([]PublicCatalogProduct, 0, input.PageSize)}
		rows = make([]publicCatalogProductRow, 0, input.PageSize)
	)
	count, err := s.database.GetValue(ctx, `SELECT COUNT(*) FROM (`+publicCatalogProductsFilteredSQL+`) visible_products`, args...)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	page.Total = count.Int()
	queryArgs := append(append([]any{}, args...), 0, int64(0), int64(0), int64(0), input.PageSize, (input.Page-1)*input.PageSize)
	if err = s.database.GetScan(ctx, &rows, publicCatalogProductsSQL, queryArgs...); err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	for index := range rows {
		if err = decodePublicCatalogCurrencies(&rows[index]); err != nil {
			return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
		}
		if err = validatePublicCatalogProduct(rows[index].PublicCatalogProduct); err != nil {
			return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
		}
		page.Products = append(page.Products, rows[index].PublicCatalogProduct)
	}
	encoded, err := json.Marshal(page)
	if err != nil || len(encoded) > maximumPublicCatalogResponseBytes-publicCatalogEnvelopeReserve {
		return nil, bizerr.NewCode(CodePublicCatalogUnavailable)
	}
	return page, nil
}
