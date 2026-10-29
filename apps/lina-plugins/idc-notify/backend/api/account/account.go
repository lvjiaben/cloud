// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package account

import (
	"context"

	"lina-plugin-idc-notify/backend/api/account/v1"
)

type IAccountV1 interface {
	LoginContactCodeCreate(ctx context.Context, req *v1.LoginContactCodeCreateReq) (res *v1.LoginContactCodeCreateRes, err error)
	LoginContactCodeVerify(ctx context.Context, req *v1.LoginContactCodeVerifyReq) (res *v1.LoginContactCodeVerifyRes, err error)
	RecoverySessionCleanup(ctx context.Context, req *v1.RecoverySessionCleanupReq) (res *v1.RecoverySessionCleanupRes, err error)
}
