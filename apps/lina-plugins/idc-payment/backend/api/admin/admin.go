package admin

import (
	"context"

	"lina-plugin-idc-payment/backend/api/admin/v1"
)

type IAdminV1 interface {
	PaymentChannelList(context.Context, *v1.PaymentChannelListReq) (*v1.PaymentChannelListRes, error)
	PaymentChannelCreate(context.Context, *v1.PaymentChannelCreateReq) (*v1.PaymentChannelCreateRes, error)
	PaymentChannelStatus(context.Context, *v1.PaymentChannelStatusReq) (*v1.PaymentChannelStatusRes, error)
	PaymentChannelConnectionVerification(context.Context, *v1.PaymentChannelConnectionVerificationReq) (*v1.PaymentChannelConnectionVerificationRes, error)
	ReconciliationHealth(context.Context, *v1.ReconciliationHealthReq) (*v1.ReconciliationHealthRes, error)
	ReconciliationBatchList(context.Context, *v1.ReconciliationBatchListReq) (*v1.ReconciliationBatchListRes, error)
	ReconciliationDifferenceList(context.Context, *v1.ReconciliationDifferenceListReq) (*v1.ReconciliationDifferenceListRes, error)
	ReconciliationSupplementSubmit(context.Context, *v1.ReconciliationSupplementSubmitReq) (*v1.ReconciliationSupplementSubmitRes, error)
	ReconciliationSupplementReview(context.Context, *v1.ReconciliationSupplementReviewReq) (*v1.ReconciliationSupplementReviewRes, error)
	ExceptionFundList(context.Context, *v1.ExceptionFundListReq) (*v1.ExceptionFundListRes, error)
	ExceptionFundAllocate(context.Context, *v1.ExceptionFundAllocateReq) (*v1.ExceptionFundAllocateRes, error)
	ExceptionFundRefund(context.Context, *v1.ExceptionFundRefundReq) (*v1.ExceptionFundRefundRes, error)
	ExceptionFundManualRefundConfirm(context.Context, *v1.ExceptionFundManualRefundConfirmReq) (*v1.ExceptionFundManualRefundConfirmRes, error)
	ExceptionFundProofUpload(context.Context, *v1.ExceptionFundProofUploadReq) (*v1.ExceptionFundProofUploadRes, error)
}
