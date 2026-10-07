// This file adapts immutable replacement-preview preparation to the management API.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
)

func (c *ControllerV1) ElectronicContractReplacementPreviewCreate(
	ctx context.Context,
	req *v1.ElectronicContractReplacementPreviewCreateReq,
) (*v1.ElectronicContractReplacementPreviewCreateRes, error) {
	if c == nil || c.electronicContractPreparation == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	prepared, err := c.electronicContractPreparation.PrepareReplacement(
		ctx, electroniccontractpreparation.PrepareReplacementInput{
			ContractNo: req.ContractNo, TemplateNo: req.TemplateNo,
			SignerNo: req.SignerNo, IdempotencyKey: req.IdempotencyKey,
		},
	)
	if err != nil {
		return nil, err
	}
	defer prepared.Clear()
	item := mapElectronicContractPreparation(prepared)
	if item == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	return &v1.ElectronicContractReplacementPreviewCreateRes{Item: item}, nil
}
