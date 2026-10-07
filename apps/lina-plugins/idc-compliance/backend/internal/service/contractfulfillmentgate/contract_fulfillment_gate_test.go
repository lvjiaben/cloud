package contractfulfillmentgate

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/contractfulfillmentcap"
)

func TestBatchEvaluateUsesOneOwnerReadAndFailsClosed(t *testing.T) {
	calls := 0
	service := &source{loadRows: func(_ context.Context, orderNos []string) ([]contractRow, error) {
		calls++
		if len(orderNos) != 4 {
			t.Fatalf("required order count=%d", len(orderNos))
		}
		return []contractRow{
			{OrderNo: "ORDER_COMPLETED", RequiredForFulfillment: true, StatusCode: completedStatus},
			{OrderNo: "ORDER_PENDING", RequiredForFulfillment: true, StatusCode: "signing"},
			{OrderNo: "ORDER_MISMATCH", RequiredForFulfillment: false, StatusCode: completedStatus},
		}, nil
	}}
	requirements := []contractfulfillmentcap.Requirement{
		{OrderNo: "ORDER_OPTIONAL", RequiresContract: false},
		{OrderNo: "ORDER_COMPLETED", RequiresContract: true},
		{OrderNo: "ORDER_PENDING", RequiresContract: true},
		{OrderNo: "ORDER_MISSING", RequiresContract: true},
		{OrderNo: "ORDER_MISMATCH", RequiresContract: true},
	}
	decisions, err := service.BatchEvaluate(context.Background(), requirements)
	if err != nil {
		t.Fatalf("evaluate fulfillment gate: %v", err)
	}
	if calls != 1 {
		t.Fatalf("owner reads=%d, want one set query", calls)
	}
	wantReady := []bool{true, true, false, false, false}
	for index, decision := range decisions {
		if decision.OrderNo != requirements[index].OrderNo || decision.Ready != wantReady[index] {
			t.Fatalf("decision[%d]=%+v", index, decision)
		}
	}
}

func TestBatchEvaluateSkipsOwnerReadWhenNoContractIsRequired(t *testing.T) {
	calls := 0
	service := &source{loadRows: func(context.Context, []string) ([]contractRow, error) {
		calls++
		return nil, nil
	}}
	decisions, err := service.BatchEvaluate(context.Background(), []contractfulfillmentcap.Requirement{
		{OrderNo: "ORDER_OPTIONAL", RequiresContract: false},
	})
	if err != nil || len(decisions) != 1 || !decisions[0].Ready ||
		decisions[0].ReasonCode != contractfulfillmentcap.ReasonNotRequired {
		t.Fatalf("optional contract decisions=%+v err=%v", decisions, err)
	}
	if calls != 0 {
		t.Fatalf("optional contracts triggered owner reads=%d", calls)
	}
}

func TestBatchEvaluateRejectsAmbiguousOwnerRows(t *testing.T) {
	service := &source{loadRows: func(context.Context, []string) ([]contractRow, error) {
		return []contractRow{{OrderNo: "ORDER_1"}, {OrderNo: "ORDER_1"}}, nil
	}}
	if _, err := service.BatchEvaluate(context.Background(), []contractfulfillmentcap.Requirement{
		{OrderNo: "ORDER_1", RequiresContract: true},
	}); err == nil {
		t.Fatal("ambiguous owner evidence was accepted")
	}
}
