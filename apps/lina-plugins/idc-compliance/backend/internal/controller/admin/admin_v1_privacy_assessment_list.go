// Privacy assessment list controller maps a bounded administrator query to safe API items. It preserves service permission and isolation decisions.

package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
)

func (c *ControllerV1) PrivacyAssessmentList(ctx context.Context, req *v1.PrivacyAssessmentListReq) (res *v1.PrivacyAssessmentListRes, err error) {
	page, err := c.privacyGovernance.ListAssessments(
		ctx, privacygovernance.AssessmentType(req.AssessmentTypeCode), req.PageSize,
	)
	if err != nil {
		return nil, err
	}
	res = &v1.PrivacyAssessmentListRes{Items: make([]v1.PrivacyAssessmentItem, 0, len(page.Items))}
	for index := range page.Items {
		res.Items = append(res.Items, privacyAssessmentItem(&page.Items[index]))
	}
	return res, nil
}
