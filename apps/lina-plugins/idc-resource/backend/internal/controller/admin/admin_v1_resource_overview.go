// This file projects the authorized resource overview and both exact table totals.

package admin

import (
	"context"

	"lina-plugin-idc-resource/backend/api/admin/v1"
	"lina-plugin-idc-resource/backend/internal/service/resourceoverview"
)

// ResourceOverview delegates both table pages to their local data owner.
func (c *ControllerV1) ResourceOverview(ctx context.Context, req *v1.ResourceOverviewReq) (*v1.ResourceOverviewRes, error) {
	summary, err := c.overview.Get(ctx, resourceoverview.ListInput{PoolPage: req.PoolPage,
		PoolPageSize: req.PoolPageSize, FinancePage: req.FinancePage, FinancePageSize: req.FinancePageSize})
	if err != nil {
		return nil, err
	}
	result := &v1.ResourceOverviewSummary{
		PoolTotal: summary.PoolTotal, FinanceTotal: summary.FinanceTotal,
		EnabledChannels: summary.EnabledChannels, EnabledPools: summary.EnabledPools,
		ActiveMappings: summary.ActiveMappings, PendingTasks: summary.PendingTasks,
		UnknownTasks: summary.UnknownTasks, LatestProviderCapacityAt: unixMilliPointer(summary.LatestProviderCapacityAt),
		Pools:   make([]*v1.ResourceOverviewPool, len(summary.Pools)),
		Finance: make([]*v1.ResourceFinanceFact, len(summary.Finance)),
	}
	for index, item := range summary.Pools {
		result.Pools[index] = &v1.ResourceOverviewPool{PoolNo: item.PoolNo, ChannelNo: item.ChannelNo,
			ChannelName: item.ChannelName, DisplayName: item.DisplayName, ResourceKind: item.ResourceKind,
			Status: item.Status, RoutingPriority: item.RoutingPriority, RoutingWeight: item.RoutingWeight,
			UpdatedAt: item.UpdatedAt.UnixMilli()}
	}
	for index, item := range summary.Finance {
		result.Finance[index] = &v1.ResourceFinanceFact{SnapshotNo: item.SnapshotNo, ChannelNo: item.ChannelNo,
			ChannelName: item.ChannelName, Kind: item.Kind, ExternalResourceRef: item.ExternalResourceRef,
			CurrencyCode: item.CurrencyCode, Amount: item.Amount, ExpiresAt: unixMilliPointer(item.ExpiresAt),
			ObservedAt: item.ObservedAt.UnixMilli()}
	}
	return &v1.ResourceOverviewRes{Summary: result}, nil
}
