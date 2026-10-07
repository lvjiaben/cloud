package v1

import (
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestProductFulfillmentActionNoteValidationMatchesEachAction(t *testing.T) {
	optional := &ProductFulfillmentTaskReviewApprovalReq{ProductFulfillmentActionReq: ProductFulfillmentActionReq{
		IntentNo: "PFI_1", ExpectedVersion: 1, IdempotencyKey: "attempt-fixed-001", ReasonCode: "verified.step-1:ok",
	}}
	if err := g.Validator().Data(optional).Run(t.Context()); err != nil {
		t.Fatalf("review note must be optional and governed reason syntax accepted: %v", err)
	}
	optional.IdempotencyKey = "too-short"
	if err := g.Validator().Data(optional).Run(t.Context()); err == nil {
		t.Fatal("idempotency key shorter than the service minimum must be rejected")
	}
	optional.IdempotencyKey = "attempt-fixed-001"
	optional.Note = string(make([]byte, 513))
	if err := g.Validator().Data(optional).Run(t.Context()); err == nil {
		t.Fatal("review note longer than 512 bytes must be rejected")
	}

	manual := &ProductFulfillmentTaskManualConclusionReq{
		OutcomeCode: "completed",
		ProductFulfillmentManualActionReq: ProductFulfillmentManualActionReq{
			IntentNo: "PFI_1", ExpectedVersion: 1, IdempotencyKey: "attempt-fixed-001", ReasonCode: "completed",
		},
	}
	if err := g.Validator().Data(manual).Run(t.Context()); err == nil {
		t.Fatal("manual conclusion note shorter than 10 characters must be rejected")
	}
	manual.Note = "completed safely"
	if err := g.Validator().Data(manual).Run(t.Context()); err != nil {
		t.Fatalf("valid manual conclusion note was rejected: %v", err)
	}
}

func TestProductFulfillmentTaskListRejectsPreSettlementFilter(t *testing.T) {
	request := &ProductFulfillmentTaskListReq{StatusCode: "awaiting_payment", Limit: 25}
	if err := g.Validator().Data(request).Run(t.Context()); err == nil {
		t.Fatal("pre-settlement status cannot be offered by the fulfillment intent directory")
	}
	request.StatusCode = "awaiting_review"
	if err := g.Validator().Data(request).Run(t.Context()); err != nil {
		t.Fatalf("post-settlement task status was rejected: %v", err)
	}
}
