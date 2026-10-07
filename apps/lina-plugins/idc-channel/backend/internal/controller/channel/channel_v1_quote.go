package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

func (c *ControllerV1) Quote(ctx context.Context, req *v1.QuoteReq) (res *v1.QuoteRes, err error) {
	return c.delegate.Quote(ctx, req)
}
