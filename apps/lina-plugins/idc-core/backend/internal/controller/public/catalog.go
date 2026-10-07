// This file maps safe public catalog service projections to HTTP DTOs.

package public

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/public/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogProductList(
	ctx context.Context,
	req *v1.CatalogProductListReq,
) (*v1.CatalogProductListRes, error) {
	page, err := c.catalog.Search(ctx, catalog.PublicCatalogSearchInput{
		NamePrefix: req.NamePrefix,
		CategoryNo: req.CategoryNo,
		Currency:   req.Currency,
		Cursor:     req.Cursor,
		Limit:      req.Limit,
	})
	if err != nil {
		return nil, err
	}
	response := &v1.CatalogProductListRes{
		Products:            make([]*v1.CatalogProductItem, 0, len(page.Products)),
		Categories:          make([]*v1.CatalogCategoryItem, 0, len(page.Categories)),
		AvailableCurrencies: append([]string(nil), page.AvailableCurrencies...),
		NextCursor:          page.NextCursor,
	}
	for _, product := range page.Products {
		response.Products = append(response.Products, toPublicCatalogProduct(product))
	}
	for _, category := range page.Categories {
		response.Categories = append(response.Categories, &v1.CatalogCategoryItem{
			CategoryNo: category.CategoryNo,
			NameZhCN:   category.NameZhCN,
			NameEnUS:   category.NameEnUS,
		})
	}
	return response, nil
}

func (c *ControllerV1) CatalogProductDetail(
	ctx context.Context,
	req *v1.CatalogProductDetailReq,
) (*v1.CatalogProductDetailRes, error) {
	product, err := c.catalog.GetBySlug(ctx, req.PublicSlug)
	if err != nil {
		return nil, err
	}
	return &v1.CatalogProductDetailRes{Product: toPublicCatalogProduct(*product)}, nil
}

func toPublicCatalogProduct(product catalog.PublicCatalogProduct) *v1.CatalogProductItem {
	item := &v1.CatalogProductItem{
		PublicSlug:          product.PublicSlug,
		CategoryNo:          product.CategoryNo,
		CategoryNameZhCN:    product.CategoryNameZhCN,
		CategoryNameEnUS:    product.CategoryNameEnUS,
		NameZhCN:            product.NameZhCN,
		NameEnUS:            product.NameEnUS,
		SummaryZhCN:         product.SummaryZhCN,
		SummaryEnUS:         product.SummaryEnUS,
		Currencies:          append([]string(nil), product.Currencies...),
		Availability:        product.Availability,
		PricingPresentation: "base_price",
	}
	return item
}
