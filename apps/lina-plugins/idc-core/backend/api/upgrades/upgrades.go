// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package upgrades

import (
	"context"

	"lina-plugin-idc-core/backend/api/upgrades/v1"
)

type IUpgradesV1 interface {
	List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error)
	Quote(ctx context.Context, req *v1.QuoteReq) (res *v1.QuoteRes, err error)
	Confirm(ctx context.Context, req *v1.ConfirmReq) (res *v1.ConfirmRes, err error)
	Pay(ctx context.Context, req *v1.PayReq) (res *v1.PayRes, err error)
}
