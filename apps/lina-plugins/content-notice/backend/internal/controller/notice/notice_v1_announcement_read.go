// This file serves the signed-in announcementread endpoint.

package notice

import (
	"context"
	v1 "lina-plugin-content-notice/backend/api/notice/v1"
)

// AnnouncementRead handles the current user's published announcement request.
func (c *ControllerV1) AnnouncementRead(ctx context.Context, req *v1.AnnouncementReadReq) (*v1.AnnouncementReadRes, error) {
	if err := c.noticeSvc.MarkRead(ctx, req.Id); err != nil {
		return nil, err
	}
	return &v1.AnnouncementReadRes{}, nil
}
