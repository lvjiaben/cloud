// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package renewalstaff

import (
	"context"

	"lina-plugin-idc-core/backend/api/renewalstaff/v1"
)

type IRenewalstaffV1 interface {
	NotificationSyncCreate(ctx context.Context, req *v1.NotificationSyncCreateReq) (res *v1.NotificationSyncCreateRes, err error)
	WorkItemList(ctx context.Context, req *v1.WorkItemListReq) (res *v1.WorkItemListRes, err error)
}
