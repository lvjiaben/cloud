package supportadmin

import (
	"testing"

	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

func TestStatusTransitionProjectionUsesServerAuthorizedOptions(t *testing.T) {
	result := ToDetail(&supportdesk.Detail{
		Ticket:           &supportdesk.Ticket{TicketNo: "TKT_status", BaseStage: supportdesk.StageInProgress},
		AvailableActions: []string{supportdesk.ActionStatusTransition},
		AvailableStatusTransitions: []supportdesk.StatusTransitionOption{
			{StatusNo: "STS_wait", NameZhCN: "等客户", NameEnUS: "Waiting for customer", BaseStage: supportdesk.StageWaitingCustomer, Version: 3, RequiresReason: false},
			{StatusNo: "STS_close", NameZhCN: "已关闭", NameEnUS: "Closed", BaseStage: supportdesk.StageClosed, Version: 2, RequiresReason: true},
		},
	})
	if result == nil || len(result.AvailableStatusTransitions) != 2 ||
		result.AvailableStatusTransitions[1].StatusNo != "STS_close" ||
		!result.AvailableStatusTransitions[1].RequiresReason {
		t.Fatalf("unexpected status transition projection: %+v", result)
	}
}
