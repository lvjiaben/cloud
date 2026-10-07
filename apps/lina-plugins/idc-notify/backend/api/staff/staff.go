// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package staff

import (
	"context"

	"lina-plugin-idc-notify/backend/api/staff/v1"
)

type IStaffV1 interface {
	WeComBindingGet(ctx context.Context, req *v1.WeComBindingGetReq) (res *v1.WeComBindingGetRes, err error)
	WeComBindingRemovalCreate(ctx context.Context, req *v1.WeComBindingRemovalCreateReq) (res *v1.WeComBindingRemovalCreateRes, err error)
	WeComOAuthAuthorizationCreate(ctx context.Context, req *v1.WeComOAuthAuthorizationCreateReq) (res *v1.WeComOAuthAuthorizationCreateRes, err error)
	WeComOAuthCallbackGet(ctx context.Context, req *v1.WeComOAuthCallbackGetReq) (res *v1.WeComOAuthCallbackGetRes, err error)
}
