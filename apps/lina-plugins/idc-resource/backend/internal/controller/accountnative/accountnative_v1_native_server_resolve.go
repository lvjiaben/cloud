// This file resolves an owned order entitlement to its public server page.
package accountnative

import (
	"context"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
)

func (c *ControllerV1) NativeServerResolve(ctx context.Context, req *v1.NativeServerResolveReq) (*v1.NativeServerResolveRes, error) {
	if pathErr := bindNativePath(ctx, "entitlementNo", &req.EntitlementNo); pathErr != nil {
		return nil, publicError(pathErr)
	}
	number, err := c.service.Resolve(ctx, req.EntitlementNo)
	if err != nil {
		return nil, publicError(err)
	}
	return &v1.NativeServerResolveRes{ServerNo: number}, nil
}
