package callback

import (
	"context"

	"lina-plugin-idc-payment/backend/api/callback/v1"
)

type ICallbackV1 interface {
	PaymentReceive(context.Context, *v1.PaymentReceiveReq) (*v1.PaymentReceiveRes, error)
	RefundReceive(context.Context, *v1.RefundReceiveReq) (*v1.RefundReceiveRes, error)
}
