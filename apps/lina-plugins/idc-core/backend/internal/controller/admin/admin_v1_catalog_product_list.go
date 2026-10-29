// This file maps catalog product workbench summaries to the admin API contract.

package admin

import (
	"context"
	"time"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (c *ControllerV1) CatalogProductList(
	ctx context.Context,
	req *v1.CatalogProductListReq,
) (res *v1.CatalogProductListRes, err error) {
	page, err := c.catalogManagement.SearchProducts(ctx, catalog.ProductManagementSearchInput{
		NamePrefix: req.NamePrefix, CategoryNo: req.CategoryNo, Status: req.Status,
		Source: req.Source, Cursor: req.Cursor, Page: req.Page, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.CatalogProductSummaryItem, 0, len(page.Products))
	for _, product := range page.Products {
		var basePrice *v1.CatalogProductBasePricePreview
		if product.BasePrice != nil {
			price := product.BasePrice
			basePrice = &v1.CatalogProductBasePricePreview{CurrencyCode: price.CurrencyCode, AmountMinor: price.AmountMinor, SetupFeeMinor: price.SetupFeeMinor, PeriodUnit: price.PeriodUnit, PeriodCount: price.PeriodCount, ChargeType: price.ChargeType}
		}
		items = append(items, &v1.CatalogProductSummaryItem{
			AvailableQuantity: product.AvailableQuantity, ProvisionedQuantity: product.ProvisionedQuantity, TotalQuantity: product.TotalQuantity, SortOrder: product.SortOrder, ProductNo: product.ProductNo, CategoryNo: product.CategoryNo, BasePrice: basePrice,
			CategoryNameZhCn: product.CategoryNameZhCN, CategoryNameEnUs: product.CategoryNameEnUS,
			VersionNo: product.VersionNo, VersionNumber: product.VersionNumber,
			Status: product.Status, VersionStatus: product.VersionStatus, Source: product.Source,
			PublicSlug: product.PublicSlug, NameZhCn: product.NameZhCN, NameEnUs: product.NameEnUS,
			SummaryZhCn: product.SummaryZhCN, SummaryEnUs: product.SummaryEnUS,
			BillingRuleCount: product.BillingRuleCount, CurrencyCount: product.CurrencyCount,
			PublishedAt: toUnixMilliseconds(product.PublishedAt),
			CreatedAt:   product.CreatedAt.UnixMilli(), UpdatedAt: product.UpdatedAt.UnixMilli(),
		})
	}
	return &v1.CatalogProductListRes{Products: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}

func toUnixMilliseconds(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	milliseconds := value.UnixMilli()
	return &milliseconds
}
