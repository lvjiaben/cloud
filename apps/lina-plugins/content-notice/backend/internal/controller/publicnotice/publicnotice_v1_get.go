// This file serves a public announcement detail without modifying read state.

package publicnotice

import (
	"context"
	v1 "lina-plugin-content-notice/backend/api/publicnotice/v1"
)

// Get returns one published announcement or the standard not-found error.
func (c *ControllerV1) Get(ctx context.Context, req *v1.GetReq) (*v1.GetRes, error) {
	item, err := c.service.PublishedByID(ctx, req.Id, false)
	if err != nil {
		return nil, err
	}
	return &v1.GetRes{Item: toItem(item)}, nil
}
