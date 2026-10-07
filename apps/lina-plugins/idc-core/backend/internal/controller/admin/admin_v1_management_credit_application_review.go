// This file executes an authorized independent credit application review.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

// ManagementCreditApplicationReview consumes a proof and appends an independent review.
func (c *ControllerV1) ManagementCreditApplicationReview(ctx context.Context, req *v1.ManagementCreditApplicationReviewReq) (res *v1.ManagementCreditApplicationReviewRes, err error) {
	intent := managementCreditReviewIntent(req.ApplicationNo, req.ManagementCreditApplicationReviewIntent)
	application, err := c.creditCommands.ReviewCreditApplication(ctx, intent, req.ProofToken)
	if err != nil {
		return nil, err
	}
	return &v1.ManagementCreditApplicationReviewRes{
		Application: toAPIManagementCreditApplication(application),
	}, nil
}
