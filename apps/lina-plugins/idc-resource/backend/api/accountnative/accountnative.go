// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package accountnative

import (
	"context"

	"lina-plugin-idc-resource/backend/api/accountnative/v1"
)

type IAccountnativeV1 interface {
	NativeServerDetail(ctx context.Context, req *v1.NativeServerDetailReq) (res *v1.NativeServerDetailRes, err error)
	NativeServerResolve(ctx context.Context, req *v1.NativeServerResolveReq) (res *v1.NativeServerResolveRes, err error)
	NativeServerOperationCreate(ctx context.Context, req *v1.NativeServerOperationCreateReq) (res *v1.NativeServerOperationCreateRes, err error)
	NativeServerCredentialReveal(ctx context.Context, req *v1.NativeServerCredentialRevealReq) (res *v1.NativeServerCredentialRevealRes, err error)
	NativeServerList(ctx context.Context, req *v1.NativeServerListReq) (res *v1.NativeServerListRes, err error)
	NativeServerCommercialQuote(ctx context.Context, req *v1.NativeServerCommercialQuoteReq) (res *v1.NativeServerCommercialQuoteRes, err error)
	NativeServerCommercialConfirmation(ctx context.Context, req *v1.NativeServerCommercialConfirmationReq) (res *v1.NativeServerCommercialConfirmationRes, err error)
	NativeServerCommercialCheckout(ctx context.Context, req *v1.NativeServerCommercialCheckoutReq) (res *v1.NativeServerCommercialCheckoutRes, err error)
	NativeServerTrafficPackages(ctx context.Context, req *v1.NativeServerTrafficPackagesReq) (res *v1.NativeServerTrafficPackagesRes, err error)
	NativeServerCommercialOperations(ctx context.Context, req *v1.NativeServerCommercialOperationsReq) (res *v1.NativeServerCommercialOperationsRes, err error)
	NativeServerAccessoryBilling(ctx context.Context, req *v1.NativeServerAccessoryBillingReq) (res *v1.NativeServerAccessoryBillingRes, err error)
	NativeServerConfigurationContext(ctx context.Context, req *v1.NativeServerConfigurationContextReq) (res *v1.NativeServerConfigurationContextRes, err error)
	NativeServerTargetQuote(ctx context.Context, req *v1.NativeServerTargetQuoteReq) (res *v1.NativeServerTargetQuoteRes, err error)
	NativeServerConfirmation(ctx context.Context, req *v1.NativeServerConfirmationReq) (res *v1.NativeServerConfirmationRes, err error)
	NativeServerFacts(ctx context.Context, req *v1.NativeServerFactsReq) (res *v1.NativeServerFactsRes, err error)
	NativeServerOperationDetail(ctx context.Context, req *v1.NativeServerOperationDetailReq) (res *v1.NativeServerOperationDetailRes, err error)
	NativeServerOutputReveal(ctx context.Context, req *v1.NativeServerOutputRevealReq) (res *v1.NativeServerOutputRevealRes, err error)
	NativeServerOperations(ctx context.Context, req *v1.NativeServerOperationsReq) (res *v1.NativeServerOperationsRes, err error)
	NativeServerOptions(ctx context.Context, req *v1.NativeServerOptionsReq) (res *v1.NativeServerOptionsRes, err error)
}
