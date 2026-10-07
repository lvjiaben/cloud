// This file handles an administrator's explicit confirmation to stop one paid
// order after its required contract has terminally failed.

package admin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
)

func (c *ControllerV1) ElectronicContractOrderCancellationCreate(ctx context.Context, req *v1.ElectronicContractOrderCancellationCreateReq) (res *v1.ElectronicContractOrderCancellationCreateRes, err error) {
	if c == nil || c.contractFailures == nil || req == nil || !req.Confirmed {
		return nil, gerror.New("contract order cancellation is unavailable")
	}
	item, err := c.contractFailures.CancelNow(ctx, req.ContractNo)
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractOrderCancellationCreateRes{
		Item: &v1.ElectronicContractFailureCompensationItem{
			CompensationNo: item.CompensationNo, RefundRequestNo: item.RefundRequestNo,
			OrderNo: item.OrderNo, StatusCode: item.StatusCode,
			ReasonCode: item.ReasonCode, Reused: item.Reused,
		},
	}, nil
}
