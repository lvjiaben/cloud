// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package public

import (
	"context"

	"lina-plugin-idc-core/backend/api/public/v1"
)

type IPublicV1 interface {
	CatalogProductDetail(ctx context.Context, req *v1.CatalogProductDetailReq) (res *v1.CatalogProductDetailRes, err error)
	CatalogProductList(ctx context.Context, req *v1.CatalogProductListReq) (res *v1.CatalogProductListRes, err error)
	GuestCartOpen(ctx context.Context, req *v1.GuestCartOpenReq) (res *v1.GuestCartOpenRes, err error)
	GuestCartItemAdd(ctx context.Context, req *v1.GuestCartItemAddReq) (res *v1.GuestCartItemAddRes, err error)
	MarketingCampaignExposureCreate(ctx context.Context, req *v1.MarketingCampaignExposureCreateReq) (res *v1.MarketingCampaignExposureCreateRes, err error)
	MarketingCampaignList(ctx context.Context, req *v1.MarketingCampaignListReq) (res *v1.MarketingCampaignListRes, err error)
	PurchaseConfigurationEvaluate(ctx context.Context, req *v1.PurchaseConfigurationEvaluateReq) (res *v1.PurchaseConfigurationEvaluateRes, err error)
	PurchaseConfigurationGet(ctx context.Context, req *v1.PurchaseConfigurationGetReq) (res *v1.PurchaseConfigurationGetRes, err error)
	PurchaseConfigurationQuote(ctx context.Context, req *v1.PurchaseConfigurationQuoteReq) (res *v1.PurchaseConfigurationQuoteRes, err error)
}
