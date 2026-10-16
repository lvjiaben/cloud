// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package platform

import (
	"context"

	"lina-plugin-idc-channel/backend/api/platform/v1"
)

type IPlatformV1 interface {
	Overview(ctx context.Context, req *v1.OverviewReq) (res *v1.OverviewRes, err error)
	Rotate(ctx context.Context, req *v1.RotateReq) (res *v1.RotateRes, err error)
	Calls(ctx context.Context, req *v1.CallsReq) (res *v1.CallsRes, err error)
}
