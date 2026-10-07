// This file serves the audited Provider health policy update API.

package admin

import (
	"context"

	"lina-plugin-idc-resource/backend/api/admin/v1"
	"lina-plugin-idc-resource/backend/internal/service/providerhealth"
)

// ProviderHealthPolicyUpdate applies one exact optimistic policy update.
func (c *ControllerV1) ProviderHealthPolicyUpdate(ctx context.Context, req *v1.ProviderHealthPolicyUpdateReq) (res *v1.ProviderHealthPolicyUpdateRes, err error) {
	item, err := c.health.UpdatePolicy(ctx, providerhealth.HealthPolicyUpdateInput{
		ChannelNo: req.ChannelNo, WindowSampleLimit: req.WindowSampleLimit,
		MinimumSamplesToProtect: req.MinimumSamplesToProtect,
		FailureLimit:            req.FailureLimit, UnknownResultLimit: req.UnknownResultLimit,
		ConsecutiveFailureLimit:   req.ConsecutiveFailureLimit,
		ProtectionDurationSeconds: req.ProtectionDurationSeconds,
		ExpectedVersion:           req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ProviderHealthPolicyUpdateRes{Item: providerHealthItem(*item)}, nil
}
