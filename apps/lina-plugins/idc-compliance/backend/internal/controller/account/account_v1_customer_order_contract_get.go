// This file maps one customer-owned order contract to the account API.

package account

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/account/v1"
)

// CustomerOrderContractGet returns the authenticated customer's contract for one order.
func (c *ControllerV1) CustomerOrderContractGet(ctx context.Context, req *v1.CustomerOrderContractGetReq) (res *v1.CustomerOrderContractGetRes, err error) {
	item, err := c.contracts.CurrentOrderContract(ctx, req.OrderNo)
	if err != nil {
		return nil, err
	}
	return &v1.CustomerOrderContractGetRes{Item: *customerContractItem(item)}, nil
}
