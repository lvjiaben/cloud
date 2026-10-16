package admin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-channel/backend/api/admin/v1"
)

func (c *ControllerV1) SaveChannel(ctx context.Context, req *v1.SaveChannelReq) (res *v1.SaveChannelRes, err error) {
	return nil, gerror.NewCode(gcode.CodeNotImplemented)
}
