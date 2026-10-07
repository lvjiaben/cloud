package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerCustomerMetadataUpdate(ctx context.Context,
	req *v1.ServerCustomerMetadataUpdateReq) (*v1.ServerCustomerMetadataUpdateRes, error) {
	metadata, err := c.serverOperations.UpdateCurrentCustomerMetadata(ctx, req.ServiceNo,
		req.Alias, req.Remark, req.ExpectedVersion)
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	return &v1.ServerCustomerMetadataUpdateRes{Metadata: &v1.ServerCustomerMetadata{
		Alias: metadata.Alias, Remark: metadata.Remark, Version: metadata.Version,
	}}, nil
}
