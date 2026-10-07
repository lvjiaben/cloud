package plugin

import (
	"context"

	"lina-core/api/plugin/v1"
	pluginsvc "lina-core/internal/service/plugin"
)

func (c *ControllerV1) BusinessImpact(ctx context.Context, req *v1.BusinessImpactReq) (res *v1.BusinessImpactRes, err error) {
	report, err := c.pluginSvc.CheckBusinessImpact(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	items := make([]*v1.BusinessImpactItem, 0, len(report.Items))
	for _, item := range report.Items {
		items = append(items, businessImpactItemToAPI(item))
	}
	allowed := report.BlockingCount == 0
	return &v1.BusinessImpactRes{
		Id: req.Id, BlockingCount: report.BlockingCount,
		CanDisable: allowed, CanUninstall: allowed, Items: items,
	}, nil
}

func businessImpactItemToAPI(item pluginsvc.BusinessImpactItem) *v1.BusinessImpactItem {
	return &v1.BusinessImpactItem{
		OwnerPluginId: item.OwnerPluginID,
		Category:      v1.BusinessImpactCategory(item.Category),
		TotalCount:    item.TotalCount,
		BlockingCount: item.BlockingCount,
	}
}
