package account

import (
	"context"

	"lina-plugin-idc-payment/backend/api/account/v1"
)

type IAccountV1 interface {
	PaymentCreate(context.Context, *v1.PaymentCreateReq) (*v1.PaymentCreateRes, error)
	PaymentStatus(context.Context, *v1.PaymentStatusReq) (*v1.PaymentStatusRes, error)
}
