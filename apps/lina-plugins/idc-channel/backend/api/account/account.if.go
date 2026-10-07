// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package account

import (
	"context"

	"lina-plugin-idc-channel/backend/api/account/v1"
)

type IAccountV1 interface {
	ListTokens(ctx context.Context, req *v1.ListTokensReq) (res *v1.ListTokensRes, err error)
	CreateToken(ctx context.Context, req *v1.CreateTokenReq) (res *v1.CreateTokenRes, err error)
	RevokeToken(ctx context.Context, req *v1.RevokeTokenReq) (res *v1.RevokeTokenRes, err error)
}
