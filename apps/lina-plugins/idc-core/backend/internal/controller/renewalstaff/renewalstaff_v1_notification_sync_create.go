// This file creates current-staff notification intents without accepting recipients.

package renewalstaff

import (
	"context"
	"lina-plugin-idc-core/backend/api/renewalstaff/v1"
)

func (c *ControllerV1) NotificationSyncCreate(ctx context.Context, req *v1.NotificationSyncCreateReq) (*v1.NotificationSyncCreateRes, error) {
	result, err := c.service.Sync(ctx, req.Locale)
	if err != nil {
		return nil, err
	}
	return &v1.NotificationSyncCreateRes{Total: result.Total, EventNo: result.EventNo}, nil
}
