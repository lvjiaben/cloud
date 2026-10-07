package admin

import (
	"context"
	"reflect"
	"testing"
	"time"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

type productFulfillmentManagementStub struct {
	lastMethod string
	listInput  productfulfillment.ListInput
	action     productfulfillment.ActionInput
	outcome    string
	detail     *productfulfillment.IntentDetail
}

func (s *productFulfillmentManagementStub) List(_ context.Context, input productfulfillment.ListInput) (*productfulfillment.IntentPage, error) {
	s.lastMethod, s.listInput = "list", input
	return &productfulfillment.IntentPage{Items: []productfulfillment.IntentDetail{*s.detail}, NextCursor: "next"}, nil
}

func (s *productFulfillmentManagementStub) Detail(_ context.Context, intentNo string) (*productfulfillment.IntentDetail, error) {
	s.lastMethod = "detail"
	s.action.IntentNo = intentNo
	return s.detail, nil
}

func (s *productFulfillmentManagementStub) ApproveAndStart(_ context.Context, input productfulfillment.ActionInput) (*productfulfillment.IntentDetail, error) {
	s.lastMethod, s.action = "approve", input
	return s.detail, nil
}

func (s *productFulfillmentManagementStub) RejectToManual(_ context.Context, input productfulfillment.ActionInput) (*productfulfillment.IntentDetail, error) {
	s.lastMethod, s.action = "reject", input
	return s.detail, nil
}

func (s *productFulfillmentManagementStub) Takeover(_ context.Context, input productfulfillment.ActionInput) (*productfulfillment.IntentDetail, error) {
	s.lastMethod, s.action = "takeover", input
	return s.detail, nil
}

func (s *productFulfillmentManagementStub) ConcludeManual(_ context.Context, input productfulfillment.ManualConclusionInput) (*productfulfillment.IntentDetail, error) {
	s.lastMethod, s.action, s.outcome = "conclude", input.ActionInput, input.Outcome
	return s.detail, nil
}

func TestProductFulfillmentControllersUseOnlyManagementService(t *testing.T) {
	now := time.UnixMilli(1_788_000_000_123)
	stub := &productFulfillmentManagementStub{detail: &productfulfillment.IntentDetail{
		IntentNo: "PFI_1", OrderNo: "ORD_1", OrderItemNo: "ORI_1", CustomerNo: "10001",
		CustomerCode: "10001", CustomerDisplayName: "张三", CustomerPrimaryPhone: "+8613800138000",
		ProductNameZhCN: "证书", ProductNameEnUS: "Certificate",
		StrategyCode: "approval_then_plugin", GranularityCode: "per_quantity_unit", StatusCode: "awaiting_review",
		Quantity: 3, UnitSequence: 2, Version: 3, FinanciallySettledAt: now, UpdatedAt: now,
		ReasonOptions: map[string][]productfulfillment.ReasonOption{
			fulfillmentReasonReviewApprove: {{Code: "verified", LabelZhCN: "已核对", LabelEnUS: "Verified"}},
		},
		Events: []productfulfillment.IntentEvent{{
			TypeCode: "review_approve", ReasonCode: "verified", OperatorDisplayName: "运营甲",
			ReasonLabelZhCN: "当时已核对", ReasonLabelEnUS: "Verified then",
			SafeNote: "已核对付款凭据", OccurredAt: now,
		}},
	}}
	controller := &ControllerV1{productFulfillment: stub}

	page, err := controller.ProductFulfillmentTaskList(context.Background(), &v1.ProductFulfillmentTaskListReq{
		OrderNo: "ORD_1", CustomerNo: "10001", FulfillmentModeCode: "approval_then_plugin",
		StatusCode: "awaiting_review", Cursor: "cursor", Limit: 25,
	})
	if err != nil || page.NextCursor != "next" || len(page.Tasks) != 1 {
		t.Fatalf("list mapping failed: page=%+v err=%v", page, err)
	}
	if page.Tasks[0].OrderItemNo != "ORI_1" || page.Tasks[0].Quantity != 3 ||
		page.Tasks[0].UnitSequence != 2 || page.Tasks[0].GranularityCode != "per_quantity_unit" ||
		page.Tasks[0].CustomerCode != "10001" || page.Tasks[0].CustomerDisplayName != "张三" ||
		page.Tasks[0].CustomerPrimaryPhone != "+8613800138000" {
		t.Fatalf("generic quantity evidence was not preserved: %+v", page.Tasks[0])
	}
	wantList := productfulfillment.ListInput{OrderNo: "ORD_1", CustomerNo: "10001", Mode: "approval_then_plugin", SafeStatus: "awaiting_review", Cursor: "cursor", Limit: 25}
	if !reflect.DeepEqual(stub.listInput, wantList) {
		t.Fatalf("list input mismatch: got=%+v want=%+v", stub.listInput, wantList)
	}

	detail, err := controller.ProductFulfillmentTaskDetail(context.Background(), &v1.ProductFulfillmentTaskDetailReq{IntentNo: "PFI_1"})
	if err != nil || detail.Task == nil || len(detail.Task.Events) != 1 {
		t.Fatalf("detail mapping failed: detail=%+v err=%v", detail, err)
	}
	event := detail.Task.Events[0]
	if event.OperatorDisplayName != "运营甲" || event.SafeNote != "已核对付款凭据" ||
		event.ReasonLabelZhCN != "当时已核对" || event.ReasonLabelEnUS != "Verified then" ||
		len(detail.Task.ReviewApprovalReasons) != 1 || len(detail.Task.ReviewRejectionReasons) != 0 {
		t.Fatalf("safe detail fields were not preserved: %+v", detail.Task)
	}

	action := v1.ProductFulfillmentActionReq{IntentNo: "PFI_1", ExpectedVersion: 3, IdempotencyKey: "attempt-fixed", ReasonCode: "verified", Note: "已核对付款凭据"}
	wantAction := productfulfillment.ActionInput{IntentNo: "PFI_1", ExpectedVersion: 3, IdempotencyKey: "attempt-fixed", ReasonCode: "verified", Note: "已核对付款凭据"}
	assertAction := func(method string, err error) {
		t.Helper()
		if err != nil || stub.lastMethod != method || !reflect.DeepEqual(stub.action, wantAction) {
			t.Fatalf("%s forwarding failed: method=%s input=%+v err=%v", method, stub.lastMethod, stub.action, err)
		}
	}
	_, err = controller.ProductFulfillmentTaskReviewApproval(context.Background(), &v1.ProductFulfillmentTaskReviewApprovalReq{ProductFulfillmentActionReq: action})
	assertAction("approve", err)
	_, err = controller.ProductFulfillmentTaskReviewRejection(context.Background(), &v1.ProductFulfillmentTaskReviewRejectionReq{ProductFulfillmentActionReq: action})
	assertAction("reject", err)
	_, err = controller.ProductFulfillmentTaskTakeover(context.Background(), &v1.ProductFulfillmentTaskTakeoverReq{ProductFulfillmentActionReq: action})
	assertAction("takeover", err)
	_, err = controller.ProductFulfillmentTaskManualConclusion(context.Background(), &v1.ProductFulfillmentTaskManualConclusionReq{
		ProductFulfillmentManualActionReq: v1.ProductFulfillmentManualActionReq{
			IntentNo: action.IntentNo, ExpectedVersion: action.ExpectedVersion, IdempotencyKey: action.IdempotencyKey,
			ReasonCode: action.ReasonCode, Note: action.Note,
		},
		OutcomeCode: "unable_to_complete",
	})
	assertAction("conclude", err)
	if stub.outcome != "unable_to_complete" {
		t.Fatalf("manual outcome changed: %q", stub.outcome)
	}
}
