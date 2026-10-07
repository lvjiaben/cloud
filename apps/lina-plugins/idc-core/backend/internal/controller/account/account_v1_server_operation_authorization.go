package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerOperationAuthorization(ctx context.Context, req *v1.ServerOperationAuthorizationReq) (res *v1.ServerOperationAuthorizationRes, err error) {
	authorization, err := c.serverOperations.PrepareCurrentCustomer(ctx, serveroperation.AccountCommand{
		ServiceNo: req.ServiceNo, Action: serveroperationresourcecap.Action(req.Action),
		Parameters: req.Parameters, IdempotencyKey: req.RequestNo,
	})
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	result := &v1.ServerOperationAuthorizationRes{Required: authorization.Required,
		ProofToken: authorization.Token}
	if authorization.ExpiresAt > 0 {
		result.ExpiresAt = &authorization.ExpiresAt
	}
	return result, nil
}
