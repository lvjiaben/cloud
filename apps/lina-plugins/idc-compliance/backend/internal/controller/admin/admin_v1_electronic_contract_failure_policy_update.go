// This file passes one authorized policy update to the contract-failure
// management service and returns the created revision.

package admin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/contractfailure"
)

func (c *ControllerV1) ElectronicContractFailurePolicyUpdate(ctx context.Context, req *v1.ElectronicContractFailurePolicyUpdateReq) (res *v1.ElectronicContractFailurePolicyUpdateRes, err error) {
	if c == nil || c.contractFailures == nil || req == nil {
		return nil, gerror.New("contract failure policy is unavailable")
	}
	item, err := c.contractFailures.UpdatePolicy(ctx, contractfailure.PolicyUpdateInput{
		ExpectedVersion: req.ExpectedVersion, HandlingDays: req.HandlingDays,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractFailurePolicyUpdateRes{Item: mapFailurePolicy(item)}, nil
}
