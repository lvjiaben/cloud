package admin

import (
	"context"

	"lina-plugin-idc-resource/backend/api/admin/v1"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func (c *ControllerV1) ProviderCapacityRefreshCreate(
	ctx context.Context,
	req *v1.ProviderCapacityRefreshCreateReq,
) (*v1.ProviderCapacityRefreshCreateRes, error) {
	observation, err := c.providerCapacity.RequestSynchronization(
		ctx, req.ChannelNo, resourcecap.ProviderCapacityKind(req.Kind),
	)
	if err != nil {
		return nil, err
	}
	return &v1.ProviderCapacityRefreshCreateRes{
		ObservationNo: observation.ObservationNo, Status: string(observation.Status),
	}, nil
}
