// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

type IChannelV1 interface {
	Catalog(ctx context.Context, req *v1.CatalogReq) (res *v1.CatalogRes, err error)
	Quote(ctx context.Context, req *v1.QuoteReq) (res *v1.QuoteRes, err error)
	CreateOrder(ctx context.Context, req *v1.CreateOrderReq) (res *v1.CreateOrderRes, err error)
	GetOrder(ctx context.Context, req *v1.GetOrderReq) (res *v1.GetOrderRes, err error)
	Lifecycle(ctx context.Context, req *v1.LifecycleReq) (res *v1.LifecycleRes, err error)
	LifecycleOutput(ctx context.Context, req *v1.LifecycleOutputReq) (res *v1.LifecycleOutputRes, err error)
	CancelOrder(ctx context.Context, req *v1.CancelOrderReq) (res *v1.CancelOrderRes, err error)
	SettleOrder(ctx context.Context, req *v1.SettleOrderReq) (res *v1.SettleOrderRes, err error)
	Finance(ctx context.Context, req *v1.FinanceReq) (res *v1.FinanceRes, err error)
	RepayCredit(ctx context.Context, req *v1.RepayCreditReq) (res *v1.RepayCreditRes, err error)
	ListServices(ctx context.Context, req *v1.ListServicesReq) (res *v1.ListServicesRes, err error)
	GetService(ctx context.Context, req *v1.GetServiceReq) (res *v1.GetServiceRes, err error)
	Capabilities(ctx context.Context, req *v1.CapabilitiesReq) (res *v1.CapabilitiesRes, err error)
	GetWebhook(ctx context.Context, req *v1.GetWebhookReq) (res *v1.GetWebhookRes, err error)
}
