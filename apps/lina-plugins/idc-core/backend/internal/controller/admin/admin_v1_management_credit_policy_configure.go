// This file appends an authorized immutable credit policy revision.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

// ManagementCreditPolicyConfigure consumes a proof and appends one immutable policy revision.
func (c *ControllerV1) ManagementCreditPolicyConfigure(ctx context.Context, req *v1.ManagementCreditPolicyConfigureReq) (res *v1.ManagementCreditPolicyConfigureRes, err error) {
	intent, err := managementCreditPolicyIntent(req.ManagementCreditPolicyIntent)
	if err != nil {
		return nil, err
	}
	policy, err := c.creditCommands.ConfigureCreditPolicy(ctx, intent, req.ProofToken)
	if err != nil {
		return nil, err
	}
	return &v1.ManagementCreditPolicyConfigureRes{Policy: toAPIManagementCreditPolicy(policy)}, nil
}
