// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package account

import (
	"context"

	"lina-plugin-idc-notify/backend/api/account/v1"
)

type IAccountV1 interface {
	DeliveryList(ctx context.Context, req *v1.DeliveryListReq) (res *v1.DeliveryListRes, err error)
	PreferenceGet(ctx context.Context, req *v1.PreferenceGetReq) (res *v1.PreferenceGetRes, err error)
	PreferenceUpdate(ctx context.Context, req *v1.PreferenceUpdateReq) (res *v1.PreferenceUpdateRes, err error)
}
