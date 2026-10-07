// This file adapts reviewed contract termination submission to the management API.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
)

func (c *ControllerV1) ElectronicContractTerminationCreate(
	ctx context.Context,
	req *v1.ElectronicContractTerminationCreateReq,
) (*v1.ElectronicContractTerminationCreateRes, error) {
	if c == nil || c.electronicContractPreparation == nil || req == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	result, err := c.electronicContractPreparation.TerminateContract(
		ctx, electroniccontractpreparation.TerminateContractInput{
			ContractNo: req.ContractNo, ExpectedVersion: req.ExpectedVersion,
			Reason: req.Reason, RemainInForceItem: req.RemainInForceItem,
			OriginalExpenseSettlement: req.OriginalExpenseSettlement,
			OriginalOtherSettlement:   req.OriginalOtherSettlement, OtherDeals: req.OtherDeals,
			Confirmed: req.Confirmed, IdempotencyKey: req.IdempotencyKey,
		},
	)
	if err != nil {
		return nil, err
	}
	item := mapElectronicContractSubmission(result)
	if item == nil {
		return nil, bizerr.NewCode(electroniccontractpreparation.CodeUnavailable)
	}
	return &v1.ElectronicContractTerminationCreateRes{Item: item}, nil
}
