package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerDetail(ctx context.Context, req *v1.ServerDetailReq) (res *v1.ServerDetailRes, err error) {
	detail, err := c.servers.GetCurrentCustomer(ctx, req.ServiceNo)
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	return &v1.ServerDetailRes{Detail: toAPICustomerServerDetail(detail)}, nil
}
