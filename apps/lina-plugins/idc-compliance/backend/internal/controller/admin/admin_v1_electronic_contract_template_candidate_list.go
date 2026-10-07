// This file adapts order-scoped template candidates to the management API.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
)

func (c *ControllerV1) ElectronicContractTemplateCandidateList(
	ctx context.Context,
	req *v1.ElectronicContractTemplateCandidateListReq,
) (*v1.ElectronicContractTemplateCandidateListRes, error) {
	if c == nil || c.electronicContractPreparation == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	options, err := c.electronicContractPreparation.GetManualOptionalOptions(ctx, req.OrderNo)
	if err != nil {
		return nil, err
	}
	items := make([]v1.ElectronicContractTemplateCandidateItem, 0, len(options.Templates))
	for _, template := range options.Templates {
		items = append(items, v1.ElectronicContractTemplateCandidateItem{
			TemplateNo: template.TemplateNo, TemplateVersion: template.VersionNo,
			TemplateName: template.Name, ScopeCode: template.ScopeCode,
			SigningOrderCode: template.SigningOrderCode,
			LanguageCode:     template.LanguageCode,
		})
	}
	return &v1.ElectronicContractTemplateCandidateListRes{
		OrderNo: options.OrderNo, CustomerNo: options.CustomerNo,
		CustomerType: string(options.CustomerType), Items: items,
	}, nil
}
