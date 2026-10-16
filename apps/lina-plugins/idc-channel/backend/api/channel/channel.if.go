// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

type IChannelV1 interface {
	CatalogConfiguration(ctx context.Context, req *v1.CatalogConfigurationReq) (res *v1.CatalogConfigurationRes, err error)
	CatalogConfigurationEvaluate(ctx context.Context, req *v1.CatalogConfigurationEvaluateReq) (res *v1.CatalogConfigurationEvaluateRes, err error)
	CatalogBillingTerms(ctx context.Context, req *v1.CatalogBillingTermsReq) (res *v1.CatalogBillingTermsRes, err error)
	Catalog(ctx context.Context, req *v1.CatalogReq) (res *v1.CatalogRes, err error)
	Quote(ctx context.Context, req *v1.QuoteReq) (res *v1.QuoteRes, err error)
	CreateOrder(ctx context.Context, req *v1.CreateOrderReq) (res *v1.CreateOrderRes, err error)
	GetOrder(ctx context.Context, req *v1.GetOrderReq) (res *v1.GetOrderRes, err error)
	Lifecycle(ctx context.Context, req *v1.LifecycleReq) (res *v1.LifecycleRes, err error)
	LifecycleOutput(ctx context.Context, req *v1.LifecycleOutputReq) (res *v1.LifecycleOutputRes, err error)
	CancelOrder(ctx context.Context, req *v1.CancelOrderReq) (res *v1.CancelOrderRes, err error)
	Finance(ctx context.Context, req *v1.FinanceReq) (res *v1.FinanceRes, err error)
	ListServices(ctx context.Context, req *v1.ListServicesReq) (res *v1.ListServicesRes, err error)
	GetService(ctx context.Context, req *v1.GetServiceReq) (res *v1.GetServiceRes, err error)
	Capabilities(ctx context.Context, req *v1.CapabilitiesReq) (res *v1.CapabilitiesRes, err error)
	GetWebhook(ctx context.Context, req *v1.GetWebhookReq) (res *v1.GetWebhookRes, err error)
	RenewalByRequest(ctx context.Context, req *v1.RenewalByRequestReq) (res *v1.RenewalByRequestRes, err error)
	GetLifecycle(ctx context.Context, req *v1.GetLifecycleReq) (res *v1.GetLifecycleRes, err error)
	CatalogNumbered(ctx context.Context, req *v1.CatalogNumberedReq) (res *v1.CatalogNumberedRes, err error)
	ProductSnapshot(ctx context.Context, req *v1.ProductSnapshotReq) (res *v1.ProductSnapshotRes, err error)
	GetOrderByRequest(ctx context.Context, req *v1.GetOrderByRequestReq) (res *v1.GetOrderByRequestRes, err error)
	ImportSnapshot(ctx context.Context, req *v1.ImportSnapshotReq) (res *v1.ImportSnapshotRes, err error)
	ImportCandidates(ctx context.Context, req *v1.ImportCandidatesReq) (res *v1.ImportCandidatesRes, err error)
	OrderDeliveries(ctx context.Context, req *v1.OrderDeliveriesReq) (res *v1.OrderDeliveriesRes, err error)
	PurchaseFileReserve(ctx context.Context, req *v1.PurchaseFileReserveReq) (res *v1.PurchaseFileReserveRes, err error)
	PurchaseFileUpload(ctx context.Context, req *v1.PurchaseFileUploadReq) (res *v1.PurchaseFileUploadRes, err error)
}
