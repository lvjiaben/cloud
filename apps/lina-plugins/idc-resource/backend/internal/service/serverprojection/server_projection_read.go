// This file implements customer-owned list and detail projection reads.

package serverprojection

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/serverdirectory"
)

// ListCurrentCustomerServers resolves customer identity from trusted context
// and performs one bounded, set-based resource-owner query.
func (service *Service) ListCurrentCustomerServers(ctx context.Context, request resourcecap.CurrentCustomerServerListRequest) (*resourcecap.CurrentCustomerServerPage, error) {
	if service == nil || service.customers == nil || service.directory == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("resource current customer server projection is unavailable")
	}
	if err := resourcecap.ValidateCurrentCustomerServerListRequest(request); err != nil {
		return nil, err
	}
	subject, err := service.customers.CurrentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	if subject == nil {
		return nil, gerror.New("resource current customer server projection is unavailable")
	}
	page, err := service.directory.List(ctx, serverdirectory.ListRequest{
		Scope:  serverdirectory.TrustedCustomerScope{CustomerNo: subject.CustomerNo},
		Cursor: request.Cursor,
		Limit:  request.Limit,
	})
	if err != nil {
		return nil, err
	}
	result := &resourcecap.CurrentCustomerServerPage{
		Items:      make([]resourcecap.CurrentCustomerServerItem, len(page.Items)),
		NextCursor: page.NextCursor,
	}
	for index := range page.Items {
		result.Items[index] = projectItem(page.Items[index])
	}
	if err = resourcecap.ValidateCurrentCustomerServerPage(result, request); err != nil {
		return nil, err
	}
	return result, nil
}

// GetCurrentCustomerServer resolves customer identity from trusted context and
// returns the same unavailable result for foreign and missing public numbers.
func (service *Service) GetCurrentCustomerServer(ctx context.Context, request resourcecap.CurrentCustomerServerGetRequest) (*resourcecap.CurrentCustomerServerResult, error) {
	if service == nil || service.customers == nil || service.directory == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("resource current customer server projection is unavailable")
	}
	if err := resourcecap.ValidateCurrentCustomerServerGetRequest(request); err != nil {
		return nil, err
	}
	subject, err := service.customers.CurrentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	if subject == nil {
		return nil, gerror.New("resource current customer server projection is unavailable")
	}
	item, err := service.directory.Detail(ctx, serverdirectory.DetailRequest{
		Scope:    serverdirectory.TrustedCustomerScope{CustomerNo: subject.CustomerNo},
		ServerNo: request.ServerNo,
	})
	if err != nil {
		return nil, err
	}
	result := &resourcecap.CurrentCustomerServerResult{Item: projectItem(*item)}
	if err = resourcecap.ValidateCurrentCustomerServerResult(result, request); err != nil {
		return nil, err
	}
	return result, nil
}

func projectItem(item serverdirectory.Item) resourcecap.CurrentCustomerServerItem {
	actions := make([]resourcecap.ServerActionCode, len(item.AvailableActions))
	for index := range item.AvailableActions {
		actions[index] = resourcecap.ServerActionCode(item.AvailableActions[index])
	}
	return resourcecap.CurrentCustomerServerItem{
		ServerNo: item.ServerNo, ProductNameZhCN: item.ProductNameZhCN,
		ProductNameEnUS: item.ProductNameEnUS, ChannelName: item.ChannelName,
		PoolName: item.PoolName, Kind: string(item.Kind), State: string(item.State),
		LatestOperation: string(item.LatestOperation), AvailableActions: actions,
		BoundAt: item.BoundAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(),
	}
}
