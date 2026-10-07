package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
)

func (c *ControllerV1) ElectronicContractSigningSubmissionCreate(
	ctx context.Context,
	req *v1.ElectronicContractSigningSubmissionCreateReq,
) (*v1.ElectronicContractSigningSubmissionCreateRes, error) {
	if c == nil || c.electronicContractPreparation == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	result, err := c.electronicContractPreparation.SubmitSigning(
		ctx, electroniccontractpreparation.SubmitSigningInput{
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
	return &v1.ElectronicContractSigningSubmissionCreateRes{
		Item: item,
	}, nil
}

func mapElectronicContractSubmission(
	result *electroniccontractpreparation.SubmittedSigning,
) *v1.ElectronicContractSigningSubmissionItem {
	if result == nil {
		return nil
	}
	return &v1.ElectronicContractSigningSubmissionItem{
		ContractNo: result.ContractNo, FlowNo: result.FlowNo,
		StatusCode: result.StatusCode, Version: result.Version, Reused: result.Reused,
	}
}
