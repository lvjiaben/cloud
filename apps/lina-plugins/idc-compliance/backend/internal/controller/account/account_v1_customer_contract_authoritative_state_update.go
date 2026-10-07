// This file maps an explicit customer contract recheck to the authoritative service path.

package account

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/account/v1"
)

// CustomerContractAuthoritativeStateUpdate performs one supplier-authoritative read.
func (c *ControllerV1) CustomerContractAuthoritativeStateUpdate(ctx context.Context, req *v1.CustomerContractAuthoritativeStateUpdateReq) (res *v1.CustomerContractAuthoritativeStateUpdateRes, err error) {
	item, err := c.contracts.RecheckCurrentContract(ctx, req.ContractNo)
	if err != nil {
		return nil, err
	}
	return &v1.CustomerContractAuthoritativeStateUpdateRes{Item: *customerContractItem(item)}, nil
}
