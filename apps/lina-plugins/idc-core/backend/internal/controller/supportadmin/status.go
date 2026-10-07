package supportadmin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

func (c *ControllerV1) TicketStatusTransition(ctx context.Context, req *v1.TicketStatusTransitionReq) (*v1.TicketStatusTransitionRes, error) {
	item, err := c.service.TransitionStatus(ctx, supportdesk.StatusTransitionInput{
		TicketNo: req.TicketNo, CustomStatusNo: req.CustomStatusNo,
		Reason: req.Reason, TicketVersion: req.TicketVersion,
	})
	if err != nil {
		return nil, err
	}
	return &v1.TicketStatusTransitionRes{Item: ToDetail(item)}, nil
}
