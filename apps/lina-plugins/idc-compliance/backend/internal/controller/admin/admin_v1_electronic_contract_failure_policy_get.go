// This file maps the active contract-failure policy to its public API
// projection without exposing storage details.

package admin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/contractfailure"
)

func (c *ControllerV1) ElectronicContractFailurePolicyGet(ctx context.Context, req *v1.ElectronicContractFailurePolicyGetReq) (res *v1.ElectronicContractFailurePolicyGetRes, err error) {
	if c == nil || c.contractFailures == nil {
		return nil, gerror.New("contract failure policy is unavailable")
	}
	item, err := c.contractFailures.GetPolicy(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractFailurePolicyGetRes{Item: mapFailurePolicy(item)}, nil
}

func mapFailurePolicy(item *contractfailure.Policy) *v1.ElectronicContractFailurePolicyItem {
	if item == nil {
		return nil
	}
	return &v1.ElectronicContractFailurePolicyItem{
		PolicyNo: item.PolicyNo, PolicyVersion: item.PolicyVersion,
		HandlingDays: item.HandlingDays, CreatedAt: item.CreatedAt.UnixMilli(), Reused: item.Reused,
	}
}
