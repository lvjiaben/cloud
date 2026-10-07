// This file delegates the path-bound detail to the account controller, which
// proves current customer ownership again even when a ticket was issued earlier.

package serverpage

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	accountv1 "lina-plugin-idc-resource/backend/api/account/v1"
	"lina-plugin-idc-resource/backend/api/serverpage/v1"
)

// ServerDetail preserves the owner's indistinguishable foreign/missing result.
func (c *ControllerV1) ServerDetail(ctx context.Context, _ *v1.ServerDetailReq) (*v1.ServerDetailRes, error) {
	// Query values can override a bound DTO field in GoFrame. Only the matched
	// router value is covered by the ticket; missing HTTP context stays invalid.
	serverNo := ""
	if request := g.RequestFromCtx(ctx); request != nil {
		serverNo = request.GetRouter("serverNo").String()
	}
	detail, err := c.account.ServerDetail(ctx, &accountv1.ServerDetailReq{ServerNo: serverNo})
	if err != nil {
		return nil, err
	}
	return &v1.ServerDetailRes{Item: detail.Item}, nil
}
