// This file handles idempotent enterprise-verification review-start commands.

package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
)

// EnterpriseVerificationReviewStartCreate starts or restarts manual review.
func (c *ControllerV1) EnterpriseVerificationReviewStartCreate(
	ctx context.Context,
	req *v1.EnterpriseVerificationReviewStartCreateReq,
) (*v1.EnterpriseVerificationReviewStartCreateRes, error) {
	result, err := c.enterpriseVerification.StartReview(ctx, enterpriseverification.ReviewInput{
		CaseNo: req.CaseNo, ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.EnterpriseVerificationReviewStartCreateRes{
		Item: enterpriseReviewResult(result),
	}, nil
}
