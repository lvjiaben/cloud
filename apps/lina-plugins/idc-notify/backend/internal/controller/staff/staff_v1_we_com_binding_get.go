// This file implements the current staff member's WeCom binding query endpoint.

package staff

import (
	"context"

	"lina-plugin-idc-notify/backend/api/staff/v1"
)

// WeComBindingGet returns only the current staff member's safe binding state.
func (c *ControllerV1) WeComBindingGet(ctx context.Context, req *v1.WeComBindingGetReq) (res *v1.WeComBindingGetRes, err error) {
	item, err := c.notifications.GetOwnWeComBinding(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.WeComBindingGetRes{Item: weComBindingItem(item)}, nil
}
