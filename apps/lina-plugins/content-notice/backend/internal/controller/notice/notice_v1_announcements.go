// This file serves the signed-in announcements endpoint.

package notice

import (
	"context"
	v1 "lina-plugin-content-notice/backend/api/notice/v1"
)

// Announcements handles the current user's published announcement request.
func (c *ControllerV1) Announcements(ctx context.Context, req *v1.AnnouncementsReq) (*v1.AnnouncementsRes, error) {
	out, err := c.noticeSvc.Published(ctx, req.PageNum, req.PageSize, true)
	if err != nil {
		return nil, err
	}
	items := make([]*v1.AnnouncementItem, 0, len(out.List))
	for _, item := range out.List {
		view := toAnnouncementItem(item)
		items = append(items, &view)
	}
	return &v1.AnnouncementsRes{List: items, Total: out.Total}, nil
}
