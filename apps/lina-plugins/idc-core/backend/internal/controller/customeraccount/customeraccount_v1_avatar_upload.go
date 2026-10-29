// This file implements the generated avatar endpoint using the explicitly injected customer service.

package customeraccount

import (
	"context"
	"lina-core/pkg/bizerr"
	accountapi "lina-plugin-idc-core/backend/api/customeraccount"
	"lina-plugin-idc-core/backend/api/customeraccount/v1"
	"lina-plugin-idc-core/backend/internal/service/customeravatar"
)

var _ accountapi.ICustomeraccountV1 = (*ControllerV1)(nil)

// AvatarUpload stores one validated image for the authenticated customer; request identity and scene cannot be overridden.
func (c *ControllerV1) AvatarUpload(ctx context.Context, req *v1.AvatarUploadReq) (res *v1.AvatarUploadRes, err error) {
	if req.File == nil || req.File.Size > 5*1024*1024 {
		return nil, bizerr.NewCode(customeravatar.CodeInvalid)
	}
	content, err := req.File.Open()
	if err != nil {
		return nil, bizerr.WrapCode(err, customeravatar.CodeUnavailable)
	}
	defer func() {
		if closeErr := content.Close(); closeErr != nil && err == nil {
			res = nil
			err = bizerr.WrapCode(closeErr, customeravatar.CodeUnavailable)
		}
	}()
	url, err := c.avatars.Upload(ctx, content)
	if err != nil {
		return nil, err
	}
	return &v1.AvatarUploadRes{URL: url}, nil
}
