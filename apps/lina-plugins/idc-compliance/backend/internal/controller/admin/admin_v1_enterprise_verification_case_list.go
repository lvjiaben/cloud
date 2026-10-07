// This file maps the bounded enterprise-verification management directory.

package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/enterpriseverification"
)

// EnterpriseVerificationCaseList returns one bounded page with its authorized total.
func (c *ControllerV1) EnterpriseVerificationCaseList(
	ctx context.Context,
	req *v1.EnterpriseVerificationCaseListReq,
) (*v1.EnterpriseVerificationCaseListRes, error) {
	page, err := c.enterpriseVerification.ListCases(ctx, enterpriseverification.CaseListInput{
		StatusCode: req.StatusCode, CustomerNo: req.CustomerNo,
		Page: req.Page, PageSize: req.PageSize, Cursor: req.Cursor,
	})
	if err != nil {
		return nil, err
	}
	items := make([]v1.EnterpriseVerificationCaseItem, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, enterpriseCaseItem(item))
	}
	return &v1.EnterpriseVerificationCaseListRes{
		Items: items, Total: page.Total, NextCursor: page.NextCursor,
	}, nil
}
