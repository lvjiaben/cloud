// This file serves public website announcement pages from the owner source.

package publicnotice

import (
	"context"
	v1 "lina-plugin-content-notice/backend/api/publicnotice/v1"
)

// List returns published announcements in saved order.
func (c *ControllerV1) List(ctx context.Context, req *v1.ListReq) (*v1.ListRes, error) {
	out, err := c.service.Published(ctx, req.PageNum, req.PageSize, false)
	if err != nil {
		return nil, err
	}
	items := make([]*v1.Item, 0, len(out.List))
	for _, item := range out.List {
		view := toItem(item)
		items = append(items, &view)
	}
	return &v1.ListRes{List: items, Total: out.Total}, nil
}
