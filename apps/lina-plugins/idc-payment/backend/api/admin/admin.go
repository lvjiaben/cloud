// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package admin

import (
	"context"

	"lina-plugin-idc-payment/backend/api/admin/v1"
)

type IAdminV1 interface {
	PaymentChannelList(ctx context.Context, req *v1.PaymentChannelListReq) (res *v1.PaymentChannelListRes, err error)
	PaymentChannelCreate(ctx context.Context, req *v1.PaymentChannelCreateReq) (res *v1.PaymentChannelCreateRes, err error)
	PaymentChannelStatus(ctx context.Context, req *v1.PaymentChannelStatusReq) (res *v1.PaymentChannelStatusRes, err error)
	PaymentChannelConnectionVerification(ctx context.Context, req *v1.PaymentChannelConnectionVerificationReq) (res *v1.PaymentChannelConnectionVerificationRes, err error)
	PaymentChannelModesSave(ctx context.Context, req *v1.PaymentChannelModesSaveReq) (res *v1.PaymentChannelModesSaveRes, err error)
	ExceptionFundList(ctx context.Context, req *v1.ExceptionFundListReq) (res *v1.ExceptionFundListRes, err error)
	ExceptionFundAllocate(ctx context.Context, req *v1.ExceptionFundAllocateReq) (res *v1.ExceptionFundAllocateRes, err error)
	ExceptionFundRefund(ctx context.Context, req *v1.ExceptionFundRefundReq) (res *v1.ExceptionFundRefundRes, err error)
	ExceptionFundManualRefundConfirm(ctx context.Context, req *v1.ExceptionFundManualRefundConfirmReq) (res *v1.ExceptionFundManualRefundConfirmRes, err error)
	ExceptionFundProofUpload(ctx context.Context, req *v1.ExceptionFundProofUploadReq) (res *v1.ExceptionFundProofUploadRes, err error)
	PaymentPublicAddressGet(ctx context.Context, req *v1.PaymentPublicAddressGetReq) (res *v1.PaymentPublicAddressGetRes, err error)
	PaymentPublicAddressSave(ctx context.Context, req *v1.PaymentPublicAddressSaveReq) (res *v1.PaymentPublicAddressSaveRes, err error)
	ReconciliationHealth(ctx context.Context, req *v1.ReconciliationHealthReq) (res *v1.ReconciliationHealthRes, err error)
	ReconciliationBatchList(ctx context.Context, req *v1.ReconciliationBatchListReq) (res *v1.ReconciliationBatchListRes, err error)
	ReconciliationDifferenceList(ctx context.Context, req *v1.ReconciliationDifferenceListReq) (res *v1.ReconciliationDifferenceListRes, err error)
	ReconciliationSupplementSubmit(ctx context.Context, req *v1.ReconciliationSupplementSubmitReq) (res *v1.ReconciliationSupplementSubmitRes, err error)
	ReconciliationSupplementReview(ctx context.Context, req *v1.ReconciliationSupplementReviewReq) (res *v1.ReconciliationSupplementReviewRes, err error)
}
