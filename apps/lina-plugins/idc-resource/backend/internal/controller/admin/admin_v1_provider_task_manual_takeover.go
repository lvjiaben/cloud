package admin

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-resource/backend/api/admin/v1"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

// ProviderTaskManualTakeover closes automation without deciding the Provider result.
func (c *ControllerV1) ProviderTaskManualTakeover(ctx context.Context, req *v1.ProviderTaskManualTakeoverReq) (res *v1.ProviderTaskManualTakeoverRes, err error) {
	request := g.RequestFromCtx(ctx)
	transportViolation := request == nil || providerTaskTakeoverBodyViolation(request.GetBody(), req)
	item, err := c.tasks.TakeOver(ctx, resourcetask.ManualTakeoverInput{
		Scope: resourcetask.TaskScope{ServiceNo: req.ServiceNo, ChannelNo: req.ChannelNo}, TaskNo: req.TaskNo, ExpectedVersion: req.ExpectedVersion,
		ReasonCode: req.ReasonCode, EvidenceNos: req.EvidenceNos,
		TransportViolation: transportViolation,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ProviderTaskManualTakeoverRes{Item: providerTaskDetail(item)}, nil
}
