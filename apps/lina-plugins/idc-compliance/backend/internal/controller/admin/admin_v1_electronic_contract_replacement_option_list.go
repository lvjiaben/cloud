package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
)

func (c *ControllerV1) ElectronicContractReplacementOptionList(
	ctx context.Context,
	req *v1.ElectronicContractReplacementOptionListReq,
) (*v1.ElectronicContractReplacementOptionListRes, error) {
	if c == nil || c.electronicContractPreparation == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	options, err := c.electronicContractPreparation.GetReplacementOptions(ctx, req.ContractNo)
	if err != nil {
		return nil, err
	}
	templates := make([]v1.ElectronicContractTemplateCandidateItem, 0, len(options.Templates))
	for _, template := range options.Templates {
		templates = append(templates, v1.ElectronicContractTemplateCandidateItem{
			TemplateNo: template.TemplateNo, TemplateVersion: template.VersionNo,
			TemplateName: template.Name, ScopeCode: template.ScopeCode,
			SigningOrderCode: template.SigningOrderCode,
			LanguageCode:     template.LanguageCode,
		})
	}
	signers := make([]v1.ElectronicContractReplacementSignerItem, 0, len(options.Signers))
	for _, signer := range options.Signers {
		signers = append(signers, v1.ElectronicContractReplacementSignerItem{
			SignerNo: signer.SignerNo, MaskedName: signer.MaskedName, MaskedPhone: signer.MaskedPhone,
		})
	}
	return &v1.ElectronicContractReplacementOptionListRes{
		ContractNo: options.ContractNo, OrderNo: options.OrderNo,
		CustomerNo: options.CustomerNo, CustomerType: string(options.CustomerType),
		Templates: templates, Signers: signers,
	}, nil
}
