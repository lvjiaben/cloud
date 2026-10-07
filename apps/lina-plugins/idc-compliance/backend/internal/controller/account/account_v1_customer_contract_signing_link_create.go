// This file returns one non-persisted Tencent signing link with no-store response headers.

package account

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-compliance/backend/api/account/v1"
)

// CustomerContractSigningLinkCreate creates one fresh link for the existing signable flow.
func (c *ControllerV1) CustomerContractSigningLinkCreate(ctx context.Context, req *v1.CustomerContractSigningLinkCreateReq) (res *v1.CustomerContractSigningLinkCreateRes, err error) {
	item, err := c.contracts.CreateCurrentSigningLink(ctx, req.ContractNo)
	if err != nil {
		return nil, err
	}
	request := g.RequestFromCtx(ctx)
	if request != nil && request.Response != nil {
		request.Response.Header().Set("Cache-Control", "no-store, max-age=0")
		request.Response.Header().Set("Pragma", "no-cache")
		request.Response.Header().Set("Referrer-Policy", "no-referrer")
	}
	return &v1.CustomerContractSigningLinkCreateRes{Item: v1.CustomerSigningLinkItem{
		Contract: *customerContractItem(&item.Contract), URL: item.URL,
		ExpiresAt: item.ExpiresAt.UTC().UnixMilli(),
	}}, nil
}
