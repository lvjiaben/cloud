package plugin

import (
	"context"

	"lina-core/api/plugin/v1"
)

func (c *ControllerV1) StopNewBusiness(ctx context.Context, req *v1.StopNewBusinessReq) (res *v1.StopNewBusinessRes, err error) {
	if err = c.pluginSvc.StopNewBusiness(ctx, req.Id); err != nil {
		return nil, err
	}
	return &v1.StopNewBusinessRes{Id: req.Id, NewBusinessState: v1.NewBusinessStateStopped}, nil
}
