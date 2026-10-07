// This file issues an exact-preview proof for an independent credit application review.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

// ManagementCreditApplicationReviewProof issues an exact independent-review proof.
func (c *ControllerV1) ManagementCreditApplicationReviewProof(ctx context.Context, req *v1.ManagementCreditApplicationReviewProofReq) (res *v1.ManagementCreditApplicationReviewProofRes, err error) {
	intent := managementCreditReviewIntent(req.ApplicationNo, req.ManagementCreditApplicationReviewIntent)
	proof, err := c.creditCommands.IssueCreditApplicationReviewProof(ctx, intent)
	if err != nil {
		return nil, err
	}
	return &v1.ManagementCreditApplicationReviewProofRes{
		Proof: toAPIManagementActionProof(proof),
	}, nil
}
