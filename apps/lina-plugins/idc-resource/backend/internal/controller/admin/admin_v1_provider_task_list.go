// This file projects the authorized local provider task directory.

package admin

import (
	"context"

	"lina-plugin-idc-resource/backend/api/admin/v1"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

// ProviderTaskList returns one locally projected keyset page.
func (c *ControllerV1) ProviderTaskList(ctx context.Context, req *v1.ProviderTaskListReq) (res *v1.ProviderTaskListRes, err error) {
	page, err := c.tasks.List(ctx, resourcetask.TaskListInput{
		ChannelNo: req.ChannelNo, Status: resourcetask.Status(req.Status),
		Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.ProviderTaskItem, len(page.Items))
	for index := range page.Items {
		items[index] = providerTaskItem(page.Items[index])
	}
	return &v1.ProviderTaskListRes{Items: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}
