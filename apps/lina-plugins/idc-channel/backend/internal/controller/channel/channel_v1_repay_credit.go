package channel

import (
	"context"

	"lina-plugin-idc-channel/backend/api/channel/v1"
)

func (c *ControllerV1) RepayCredit(ctx context.Context, req *v1.RepayCreditReq) (*v1.RepayCreditRes, error) {
	return c.delegate.RepayCredit(ctx, req)
}
