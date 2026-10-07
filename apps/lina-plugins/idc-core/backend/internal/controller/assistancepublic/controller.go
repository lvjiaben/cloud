package assistancepublic

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	v1 "lina-plugin-idc-core/backend/api/assistancepublic/v1"
	"lina-plugin-idc-core/backend/internal/service/customerassistance"
)

type ControllerV1 struct{ service customerassistance.Service }

func NewV1(service customerassistance.Service) (*ControllerV1, error) {
	if service == nil {
		return nil, gerror.New("customer assistance public controller requires service")
	}
	return &ControllerV1{service: service}, nil
}

func (c *ControllerV1) CustomerAssistanceSnapshot(ctx context.Context, req *v1.CustomerAssistanceSnapshotReq) (*v1.CustomerAssistanceSnapshotRes, error) {
	item, err := c.service.Snapshot(ctx, req.AccessToken)
	if err != nil {
		return nil, err
	}
	return &v1.CustomerAssistanceSnapshotRes{
		SessionNo: item.SessionNo, AccessToken: item.AccessToken,
		HomePath: "/account", ExpiresAt: item.ExpiresAt.UnixMilli(),
	}, nil
}
