package assistanceadmin

import (
	"context"
	"net/url"

	"github.com/gogf/gf/v2/errors/gerror"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/runtimeconfig"
	"lina-plugin-idc-core/backend/internal/service/customerassistance"
)

type ControllerV1 struct{ service customerassistance.Service }

func NewV1(service customerassistance.Service) (*ControllerV1, error) {
	if service == nil {
		return nil, gerror.New("customer assistance admin controller requires service")
	}
	return &ControllerV1{service: service}, nil
}

func (c *ControllerV1) CustomerAssistanceSessionStart(ctx context.Context, req *v1.CustomerAssistanceSessionStartReq) (*v1.CustomerAssistanceSessionStartRes, error) {
	item, err := c.service.Start(ctx, customerassistance.StartInput{
		CustomerNo: req.CustomerNo, Reason: req.Reason, TicketNo: req.TicketNo, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	launchURL, err := runtimeconfig.CustomerPublicURL("/account/assistance?token=" + url.QueryEscape(item.AccessToken))
	if err != nil {
		return nil, err
	}
	return &v1.CustomerAssistanceSessionStartRes{
		SessionNo: item.SessionNo, CustomerNo: item.CustomerNo, Status: item.Status,
		LaunchURL: launchURL,
		StartedAt: item.StartedAt.UnixMilli(), ExpiresAt: item.ExpiresAt.UnixMilli(),
	}, nil
}

func (c *ControllerV1) CustomerAssistanceSessionEnd(ctx context.Context, req *v1.CustomerAssistanceSessionEndReq) (*v1.CustomerAssistanceSessionEndRes, error) {
	item, err := c.service.End(ctx, req.SessionNo)
	if err != nil {
		return nil, err
	}
	var endedAt int64
	if item.EndedAt != nil {
		endedAt = item.EndedAt.UnixMilli()
	}
	return &v1.CustomerAssistanceSessionEndRes{SessionNo: item.SessionNo, CustomerNo: item.CustomerNo, Status: item.Status, EndedAt: endedAt}, nil
}
