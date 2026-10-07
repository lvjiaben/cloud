// This file issues an exact-preview proof for an immutable credit policy revision.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

// ManagementCreditPolicyProof issues an exact immutable-policy proof.
func (c *ControllerV1) ManagementCreditPolicyProof(ctx context.Context, req *v1.ManagementCreditPolicyProofReq) (res *v1.ManagementCreditPolicyProofRes, err error) {
	intent, err := managementCreditPolicyIntent(req.ManagementCreditPolicyIntent)
	if err != nil {
		return nil, err
	}
	proof, err := c.creditCommands.IssueCreditPolicyProof(ctx, intent)
	if err != nil {
		return nil, err
	}
	return &v1.ManagementCreditPolicyProofRes{Proof: toAPIManagementActionProof(proof)}, nil
}
