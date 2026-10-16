// This file handles owned trial conversion pay using the startup renewal service.

package account

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/renewal"
)

func (c *ControllerV1) TrialConversionPay(ctx context.Context, req *v1.TrialConversionPayReq) (res *v1.TrialConversionPayRes, err error) {
	settlement, err := c.renewals.PayCurrentCustomer(ctx, renewal.PayInput{OperationNo: req.OperationNo})
	if err != nil {
		return nil, err
	}
	return &v1.TrialConversionPayRes{Operation: toAPITrialConversionOperation(settlement.Operation)}, nil
}
