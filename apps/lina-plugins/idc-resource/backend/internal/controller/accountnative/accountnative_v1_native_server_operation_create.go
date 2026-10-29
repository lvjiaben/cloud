// This file accepts a confirmed owner-scoped management intent.
package accountnative

import (
	"context"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"
)

func (c *ControllerV1) NativeServerOperationCreate(ctx context.Context, req *v1.NativeServerOperationCreateReq) (*v1.NativeServerOperationCreateRes, error) {
	if pathErr := bindNativePath(ctx, "serverNo", &req.ServerNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	password := []byte(req.Password)
	req.Password = ""
	defer clear(password)
	operation, err := c.service.Create(ctx, customeroperations.Command{ServerNo: req.ServerNo, Action: req.Action, IdempotencyKey: req.IdempotencyKey, Confirmed: req.Confirmed, Parameters: req.Parameters, Password: password, ProofToken: req.ProofToken})
	if err != nil {
		return nil, publicError(err)
	}
	return &v1.NativeServerOperationCreateRes{Operation: projectOperation(*operation)}, nil
}
