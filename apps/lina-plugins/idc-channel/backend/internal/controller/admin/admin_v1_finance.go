package admin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-channel/backend/api/admin/v1"
)

func (c *ControllerV1) Finance(ctx context.Context, req *v1.FinanceReq) (res *v1.FinanceRes, err error) {
	return nil, gerror.NewCode(gcode.CodeNotImplemented)
}
