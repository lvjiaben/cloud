package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
)

func (c *ControllerV1) ElectronicContractAutoSignAuthorizationList(ctx context.Context, req *v1.ElectronicContractAutoSignAuthorizationListReq) (res *v1.ElectronicContractAutoSignAuthorizationListRes, err error) {
	page, err := c.contractAutoSign.List(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	result := &v1.ElectronicContractAutoSignAuthorizationListRes{
		Items:      make([]v1.ElectronicContractAutoSignAuthorizationItem, 0, len(page.Items)),
		Candidates: make([]v1.ElectronicContractAutoSignTemplateRef, 0, len(page.Candidates)),
		Page:       page.Page, PageSize: page.PageSize, Total: page.Total,
	}
	for _, item := range page.Items {
		result.Items = append(result.Items, mapAutoSignAuthorization(item))
	}
	for _, ref := range page.Candidates {
		result.Candidates = append(result.Candidates, v1.ElectronicContractAutoSignTemplateRef{
			TemplateNo: ref.TemplateNo, VersionNo: ref.VersionNo, Name: ref.Name,
		})
	}
	return result, nil
}
