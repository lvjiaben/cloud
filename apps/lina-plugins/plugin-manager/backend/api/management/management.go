// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package management

import (
	"context"

	"lina-plugin-plugin-manager/backend/api/management/v1"
)

type IManagementV1 interface {
	Categories(ctx context.Context, req *v1.CategoriesReq) (res *v1.CategoriesRes, err error)
	List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error)
}
