package plugin

import (
	"context"

	"lina-core/api/plugin/v1"
)

func (c *ControllerV1) ResumeNewBusiness(ctx context.Context, req *v1.ResumeNewBusinessReq) (res *v1.ResumeNewBusinessRes, err error) {
	if err = c.pluginSvc.ResumeNewBusiness(ctx, req.Id); err != nil {
		return nil, err
	}
	return &v1.ResumeNewBusinessRes{Id: req.Id, NewBusinessState: v1.NewBusinessStateAccepting}, nil
}
