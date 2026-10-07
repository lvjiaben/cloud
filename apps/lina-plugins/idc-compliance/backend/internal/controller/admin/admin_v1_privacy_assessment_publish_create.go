// Privacy assessment publication controller forwards explicit confirmation and concurrency data. It does not publish implicitly or suppress service errors.

package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
)

func (c *ControllerV1) PrivacyAssessmentPublishCreate(ctx context.Context, req *v1.PrivacyAssessmentPublishCreateReq) (res *v1.PrivacyAssessmentPublishCreateRes, err error) {
	item, err := c.privacyGovernance.PublishAssessment(ctx, privacygovernance.PublishAssessmentInput{
		AssessmentNo: req.AssessmentNo, ExpectedRowVersion: req.ExpectedRowVersion,
		Confirmed: req.Confirmed, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.PrivacyAssessmentPublishCreateRes{Item: privacyAssessmentItem(item)}, nil
}
