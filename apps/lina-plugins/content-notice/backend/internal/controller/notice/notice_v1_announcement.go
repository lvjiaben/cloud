// This file serves the signed-in announcement endpoint.

package notice

import (
	"context"
	v1 "lina-plugin-content-notice/backend/api/notice/v1"
)

// Announcement handles the current user's published announcement request.
func (c *ControllerV1) Announcement(ctx context.Context, req *v1.AnnouncementReq) (*v1.AnnouncementRes, error) {
	item, err := c.noticeSvc.PublishedByID(ctx, req.Id, true)
	if err != nil {
		return nil, err
	}
	return &v1.AnnouncementRes{AnnouncementItem: toAnnouncementItem(item)}, nil
}
