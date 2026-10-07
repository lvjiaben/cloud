package serveroperation

import (
	"encoding/json"
	"testing"
)

func TestNormalizeBatchPreviewRejectsDuplicatesAndUnsafeParameters(t *testing.T) {
	valid := BatchPreviewInput{Action: BatchActionTagAdd, IdempotencyKey: "batch-idempotency-0001",
		Items: []BatchPreviewItemInput{{ServiceNo: "service-1", Parameters: json.RawMessage(`{"tag":"priority"}`)}}}
	items, digest, err := normalizeBatchPreview(valid)
	if err != nil || len(items) != 1 || len(digest) != 64 {
		t.Fatalf("valid safe batch rejected: items=%v digest=%q err=%v", items, digest, err)
	}
	duplicated := valid
	duplicated.Items = append(duplicated.Items, valid.Items[0])
	if _, _, err = normalizeBatchPreview(duplicated); err == nil {
		t.Fatal("duplicate service was accepted")
	}
	invalidTag := valid
	invalidTag.Items = []BatchPreviewItemInput{{ServiceNo: "service-1", Parameters: json.RawMessage(`{"tag":"Admin Label"}`)}}
	if _, _, err = normalizeBatchPreview(invalidTag); err == nil {
		t.Fatal("unsafe free-form tag was accepted")
	}
	secretNote := valid
	secretNote.Action = BatchActionNoteAppend
	secretNote.Items = []BatchPreviewItemInput{{ServiceNo: "service-1", Parameters: json.RawMessage(`{"note":"  hidden whitespace  "}`)}}
	if _, _, err = normalizeBatchPreview(secretNote); err == nil {
		t.Fatal("non-normalized note was accepted")
	}
}

func TestDangerousBatchBindingFreezesPreview(t *testing.T) {
	first := &ServiceBatch{BatchNo: "batch-1", PreviewDigest: string(make([]byte, 64)),
		Action: BatchActionTerminate, Dangerous: true}
	second := *first
	second.PreviewDigest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	firstBinding := batchReauthBinding(first)
	secondBinding := batchReauthBinding(&second)
	if firstBinding.Purpose != "resource.bulk" || firstBinding.TargetDigest == "" {
		t.Fatalf("unexpected dangerous binding: %+v", firstBinding)
	}
	if firstBinding.TargetDigest == secondBinding.TargetDigest {
		t.Fatal("changed preview reused the same recent-login binding")
	}
}

func TestBatchImpactMarksUnsafeAndIrreversibleServices(t *testing.T) {
	impact := projectBatchImpact(1, BatchActionTerminate, json.RawMessage(`{}`), batchServiceRow{
		ServiceNo: "service-1", CustomerNo: "customer-1", CurrencyCode: "CNY",
		EntitlementStatus: "active", RuntimeStatus: "unknown", ProviderStatus: "none",
		ProductType: "cloud_server", PoolNo: "pool-1", ChannelNo: "channel-1", Version: 2,
	})
	if impact.PreviewStatus != "ineligible" || impact.FailureCode != "service_state_not_safe" || !impact.Irreversible {
		t.Fatalf("unsafe terminate impact was not frozen correctly: %+v", impact)
	}
}

func TestDangerousBatchFailureEntersManualReview(t *testing.T) {
	if got := batchFailureStatus(true); got != "manual_review" {
		t.Fatalf("dangerous item failure status = %q", got)
	}
	if got := batchFailureStatus(false); got != "failed" {
		t.Fatalf("safe item failure status = %q", got)
	}
}

func TestBatchReinstallRequiresOnePositiveImageReference(t *testing.T) {
	if reference, ok := batchReinstallReference(json.RawMessage(`{"os":101}`)); !ok || reference != "101" {
		t.Fatalf("valid reinstall image rejected: reference=%q ok=%v", reference, ok)
	}
	for _, raw := range []string{`{}`, `{"os":0}`, `{"os":"101"}`, `{"os":101,"port":22}`} {
		if _, ok := batchReinstallReference(json.RawMessage(raw)); ok {
			t.Fatalf("unsafe reinstall parameters accepted: %s", raw)
		}
	}
}

func TestBatchClassificationRequiresOneFixedReference(t *testing.T) {
	if reference, ok := batchClassificationReference(json.RawMessage(`{"classificationNo":"scl_core"}`)); !ok || reference != "scl_core" {
		t.Fatalf("valid classification rejected: reference=%q ok=%v", reference, ok)
	}
	for _, raw := range []string{`{}`, `{"classificationNo":""}`, `{"classificationNo":"scl_core","name":"guess"}`} {
		if _, ok := batchClassificationReference(json.RawMessage(raw)); ok {
			t.Fatalf("invalid classification accepted: %s", raw)
		}
	}
}
