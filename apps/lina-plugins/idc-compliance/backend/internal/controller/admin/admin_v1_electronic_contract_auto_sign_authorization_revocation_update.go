package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/contractautosign"
)

func (c *ControllerV1) ElectronicContractAutoSignAuthorizationRevocationUpdate(ctx context.Context, req *v1.ElectronicContractAutoSignAuthorizationRevocationUpdateReq) (res *v1.ElectronicContractAutoSignAuthorizationRevocationUpdateRes, err error) {
	item, err := c.contractAutoSign.Revoke(ctx, contractautosign.RevokeInput{
		AuthorizationNo: req.AuthorizationNo, ExpectedVersion: req.ExpectedVersion,
		Confirmed: req.Confirmed, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ElectronicContractAutoSignAuthorizationRevocationUpdateRes{Item: mapAutoSignAuthorization(*item)}, nil
}
