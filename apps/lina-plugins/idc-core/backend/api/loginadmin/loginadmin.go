// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package loginadmin

import (
	"context"

	"lina-plugin-idc-core/backend/api/loginadmin/v1"
)

type ILoginadminV1 interface {
	ContactIndexRefresh(ctx context.Context, req *v1.ContactIndexRefreshReq) (res *v1.ContactIndexRefreshRes, err error)
}
