// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package callback

import (
	"context"

	"lina-plugin-idc-notify/backend/api/callback/v1"
)

type ICallbackV1 interface {
	WeChatOfficialVerify(ctx context.Context, req *v1.WeChatOfficialVerifyReq) (res *v1.WeChatOfficialVerifyRes, err error)
	WeChatOfficialReceive(ctx context.Context, req *v1.WeChatOfficialReceiveReq) (res *v1.WeChatOfficialReceiveRes, err error)
}
