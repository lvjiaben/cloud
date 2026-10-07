// This file handles the administrator command that freezes Tencent template mappings.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontracttemplate"
)

// ElectronicContractTemplateProviderMappingCreate validates and saves one immutable provider mapping set.
func (c *ControllerV1) ElectronicContractTemplateProviderMappingCreate(ctx context.Context, req *v1.ElectronicContractTemplateProviderMappingCreateReq) (res *v1.ElectronicContractTemplateProviderMappingCreateRes, err error) {
	if c == nil || c.electronicContractTemplates == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontracttemplate.CodeUnavailable)
	}
	fields := make([]electroniccontracttemplate.ProviderFieldMapping, 0, len(req.FieldMappings))
	for _, mapping := range req.FieldMappings {
		fields = append(fields, electroniccontracttemplate.ProviderFieldMapping{
			ComponentRef: mapping.ComponentRef, ValueCode: mapping.ValueCode,
		})
	}
	recipients := make([]electroniccontracttemplate.ProviderRecipientMapping, 0, len(req.RecipientMappings))
	for _, mapping := range req.RecipientMappings {
		recipients = append(recipients, electroniccontracttemplate.ProviderRecipientMapping{
			RecipientRef: mapping.RecipientRef, PartyCode: mapping.PartyCode,
		})
	}
	item, err := c.electronicContractTemplates.SaveProviderMapping(
		ctx, electroniccontracttemplate.SaveProviderMappingInput{
			TemplateNo: req.TemplateNo, VersionNo: req.VersionNo,
			ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
			FieldMappings: fields, RecipientMappings: recipients,
		},
	)
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractTemplateProviderMappingCreateRes{
		Item: mapElectronicContractTemplateEditor(item),
	}, nil
}
