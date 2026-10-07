package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerIPChangePolicyList(ctx context.Context,
	_ *v1.ServerIPChangePolicyListReq) (*v1.ServerIPChangePolicyListRes, error) {
	items, err := c.accessoryPricing.ListIPChangePolicies(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.ServerIPChangePolicy, len(items))
	for index := range items {
		result[index] = projectIPChangePolicy(items[index])
	}
	return &v1.ServerIPChangePolicyListRes{Items: result}, nil
}

func (c *ControllerV1) ServerIPChangePolicyPublish(ctx context.Context,
	req *v1.ServerIPChangePolicyPublishReq) (*v1.ServerIPChangePolicyPublishRes, error) {
	policy, err := c.accessoryPricing.PublishIPChangePolicy(ctx, serveroperation.IPChangePolicyInput{
		CurrencyCode: req.CurrencyCode, Enabled: req.Enabled,
		FreeChangesPerWindow: req.FreeChangesPerWindow, WindowDays: req.WindowDays,
		MaxChangesPerDay: req.MaxChangesPerDay, AmountMinor: req.AmountMinor,
		MaximumSnapshotAgeSeconds: req.MaximumSnapshotAgeSeconds,
		ExpectedVersion:           req.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ServerIPChangePolicyPublishRes{Policy: projectIPChangePolicy(*policy)}, nil
}

func projectIPChangePolicy(item serveroperation.IPChangePolicy) *v1.ServerIPChangePolicy {
	return &v1.ServerIPChangePolicy{PolicyNo: item.PolicyNo, CurrencyCode: item.CurrencyCode,
		Enabled: item.Enabled, FreeChangesPerWindow: item.FreeChangesPerWindow,
		WindowDays: item.WindowDays, MaxChangesPerDay: item.MaxChangesPerDay,
		AmountMinor: item.AmountMinor, MaximumSnapshotAgeSeconds: item.MaximumSnapshotAgeSeconds,
		Version: item.Version, PublishedAt: item.PublishedAt.UnixMilli()}
}
