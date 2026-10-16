package admin

import (
	"context"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-channel/backend/api/admin/v1"
)

func (c *ControllerV1) SavePublication(ctx context.Context, req *v1.SavePublicationReq) (res *v1.SavePublicationRes, err error) {
	return nil, gerror.NewCode(gcode.CodeNotImplemented)
}
