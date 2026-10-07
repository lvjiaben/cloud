// This file adapts reviewed contract reissue submission to the management API.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
)

func (c *ControllerV1) ElectronicContractReissueCreate(
	ctx context.Context,
	req *v1.ElectronicContractReissueCreateReq,
) (*v1.ElectronicContractReissueCreateRes, error) {
	if c == nil || c.electronicContractPreparation == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	result, err := c.electronicContractPreparation.SubmitReissue(
		ctx, electroniccontractpreparation.ReissueSigningInput{
			ContractNo: req.ContractNo, ExpectedVersion: req.ExpectedVersion,
			IdempotencyKey: req.IdempotencyKey,
		},
	)
	if err != nil {
		return nil, err
	}
	item := mapElectronicContractSubmission(result)
	if item == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	return &v1.ElectronicContractReissueCreateRes{Item: item}, nil
}
