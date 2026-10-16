// This generated controller delegates staff product edits to the existing authorized owner workflow.
package admin

import (
	"context"
	account "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CustomerServerMetadataUpdate(ctx context.Context, req *v1.CustomerServerMetadataUpdateReq) (res *v1.CustomerServerMetadataUpdateRes, err error) {
	value, err := c.serverAccessoryManagement.UpdateMetadata(ctx, req.ServiceNo, req.Alias, req.Remark, req.ExpectedVersion)
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	return &v1.CustomerServerMetadataGetRes{Metadata: &account.ServerCustomerMetadata{Alias: value.Alias, Remark: value.Remark, Version: value.Version}}, nil
}
