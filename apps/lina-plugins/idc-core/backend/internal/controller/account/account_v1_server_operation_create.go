package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerOperationCreate(ctx context.Context, req *v1.ServerOperationCreateReq) (res *v1.ServerOperationCreateRes, err error) {
	password := []byte(req.Password)
	req.Password = ""
	defer clear(password)
	operation, err := c.serverOperations.CreateCurrentCustomer(ctx, serveroperation.AccountCommand{
		ServiceNo: req.ServiceNo, Action: serveroperationresourcecap.Action(req.Action),
		Parameters: req.Parameters, Password: password, ProofToken: req.ProofToken,
		IdempotencyKey: req.RequestNo,
	})
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	return &v1.ServerOperationCreateRes{Operation: &v1.ServerOperationSummary{
		OperationNo: operation.OperationNo, Action: string(operation.Action), Status: operation.Status,
		CreatedAt: operation.CreatedAt.UnixMilli(),
	}}, nil
}
