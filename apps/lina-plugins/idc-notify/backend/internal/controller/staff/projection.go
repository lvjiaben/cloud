// This file maps WeCom binding service results to safe staff API projections.

package staff

import (
	"lina-plugin-idc-notify/backend/api/staff/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func weComBindingItem(item *notification.WeComBindingStatus) *v1.WeComBindingItem {
	if item == nil {
		return nil
	}
	projected := &v1.WeComBindingItem{Bound: item.Bound, Status: item.Status, Version: item.Version, Reused: item.Reused}
	if item.BoundAt != nil {
		projected.BoundAt = item.BoundAt.UTC().UnixMilli()
	}
	return projected
}
