package audit

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/idccap"
)

type capAdapterWriter struct {
	input      TransitionInput
	batchInput BatchTransitionInput
}

func (w *capAdapterWriter) AppendTransition(
	_ context.Context,
	input TransitionInput,
) (*Event, error) {
	w.input = input
	return &Event{}, nil
}

func (w *capAdapterWriter) AppendBatchTransition(
	_ context.Context,
	input BatchTransitionInput,
) (*AppendResult, error) {
	w.batchInput = input
	return &AppendResult{AuditNo: "AUD_11111111111111111111111111111111", Replayed: true}, nil
}

func TestBusinessWriterAdapterMapsBoundedBatchAndReplayResult(t *testing.T) {
	writer := &capAdapterWriter{}
	adapter, err := NewBusinessWriterAdapter(writer)
	if err != nil {
		t.Fatalf("create batch business audit adapter: %v", err)
	}
	input := idccap.BusinessAuditBatchTransition{
		IdempotencyScope: "customer.tag.batch", IdempotencyKey: "batch-one",
		OperationCode: "customer.tag.batch.assign", TargetType: "customer_tag_batch",
		TargetNo: "TAG_0123456789abcdef0123456789abcdef",
		AffectedCustomerNos: []string{
			"CUS_0123456789abcdef0123456789abcdef",
			"CUS_abcdef0123456789abcdef0123456789",
		},
		ResultCode: "succeeded", ReasonCode: "manual_segment_update", RiskCode: "medium",
		RequiredPermission: "idc:admin:customer:tag:batch", CorrelationNo: "BATCH_ONE",
		BeforeSnapshot: map[string]any{"state": "requested"},
		AfterSnapshot:  map[string]any{"state": "assigned"},
	}
	result, err := adapter.AppendBusinessBatchTransition(context.Background(), input)
	if err != nil || result.AuditNo == "" || !result.Replayed {
		t.Fatalf("append adapted batch result=%+v err=%v", result, err)
	}
	if writer.batchInput.RequiredPermission != input.RequiredPermission ||
		writer.batchInput.OperationCode != input.OperationCode ||
		len(writer.batchInput.AffectedCustomerNos) != 2 ||
		writer.batchInput.BeforeSnapshot == nil || writer.batchInput.AfterSnapshot == nil {
		t.Fatalf("unexpected adapted batch transition: %+v", writer.batchInput)
	}
}

func (*capAdapterWriter) RecordObservation(
	context.Context,
	ObservationInput,
) (*Event, error) {
	return &Event{}, nil
}

func TestBusinessWriterAdapterMapsCallerSemanticsWithoutActorFields(t *testing.T) {
	writer := &capAdapterWriter{}
	adapter, err := NewBusinessWriterAdapter(writer)
	if err != nil {
		t.Fatalf("create business audit adapter: %v", err)
	}
	input := idccap.BusinessAuditTransition{
		IdempotencyScope:   "principal.customer-onboarding.redeem",
		IdempotencyKey:     "INT_test",
		OperationCode:      "customer.onboarding.redeem",
		TargetType:         "customer",
		TargetNo:           "CUS_0123456789abcdef0123456789abcdef",
		CustomerNo:         "CUS_0123456789abcdef0123456789abcdef",
		ResultCode:         "succeeded",
		ReasonCode:         "onboarding",
		RiskCode:           "low",
		RequiredPermission: "idc:admin:principal:onboarding",
		CorrelationNo:      "INT_test",
		BeforeSnapshot:     map[string]any{},
		AfterSnapshot:      map[string]any{"status": "active"},
	}
	if err = adapter.AppendBusinessTransition(context.Background(), input); err != nil {
		t.Fatalf("append adapted audit transition: %v", err)
	}
	if writer.input.OperationCode != input.OperationCode ||
		writer.input.TargetNo != input.TargetNo || writer.input.CustomerNo != input.CustomerNo ||
		writer.input.RequiredPermission != input.RequiredPermission ||
		writer.input.Result != ResultSucceeded || writer.input.Risk != RiskLow ||
		writer.input.BeforeSnapshot == nil || writer.input.AfterSnapshot == nil {
		t.Fatalf("unexpected adapted transition: %+v", writer.input)
	}
}

func TestBusinessWriterAdapterRejectsNilWriter(t *testing.T) {
	var writer *capAdapterWriter
	if _, err := NewBusinessWriterAdapter(writer); err == nil {
		t.Fatal("typed nil audit writer was accepted")
	}
}

var _ Writer = (*capAdapterWriter)(nil)
