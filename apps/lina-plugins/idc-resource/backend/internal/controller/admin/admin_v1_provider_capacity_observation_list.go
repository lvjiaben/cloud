// This file projects authorized local provider capacity collection records.

package admin

import (
	"context"

	"lina-plugin-idc-resource/backend/api/admin/v1"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/providercapacity"
)

// ProviderCapacityObservationList returns bounded observations and their exact filtered count.
func (c *ControllerV1) ProviderCapacityObservationList(
	ctx context.Context,
	req *v1.ProviderCapacityObservationListReq,
) (*v1.ProviderCapacityObservationListRes, error) {
	page, err := c.providerCapacity.ListOperations(ctx, providercapacity.OperationListInput{
		ChannelNo: req.ChannelNo, Kind: resourcecap.ProviderCapacityKind(req.Kind),
		Status: providercapacity.Status(req.Status), Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.ProviderCapacityObservationItem, len(page.Items))
	for index, operation := range page.Items {
		items[index] = &v1.ProviderCapacityObservationItem{
			ObservationNo: operation.ObservationNo, ChannelNo: operation.ChannelNo,
			ChannelName: operation.ChannelName, Kind: string(operation.Kind),
			Status: string(operation.Status), Trigger: operation.Trigger,
			ItemCount: operation.ItemCount, PageCount: operation.PageCount,
			FailureCode:    operation.FailureCode,
			RequestedAt:    operation.RequestedAt.UnixMilli(),
			LastObservedAt: operation.LastObservedAt.UnixMilli(),
			CompletedAt:    unixMilliPointer(operation.CompletedAt),
		}
	}
	channels := make([]*v1.ProviderCapacityChannelOption, len(page.Channels))
	for index, channel := range page.Channels {
		channels[index] = &v1.ProviderCapacityChannelOption{
			ChannelNo: channel.ChannelNo, DisplayName: channel.DisplayName,
		}
	}
	return &v1.ProviderCapacityObservationListRes{
		Items: items, NextCursor: page.NextCursor, Channels: channels, Total: page.Total,
	}, nil
}
