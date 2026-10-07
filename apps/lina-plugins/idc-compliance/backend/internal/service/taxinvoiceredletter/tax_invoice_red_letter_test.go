// This file verifies red-letter input validation and closed supplier behavior.

package taxinvoiceredletter

import (
	"context"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
)

func TestClosedGatewayRejectsSupplierActions(t *testing.T) {
	gateway := ClosedGateway{}
	if actionURL, err := gateway.CustomerAction(
		context.Background(), "nuonuo", "external-task-1",
	); err == nil || actionURL != "" {
		t.Fatalf("closed customer action url=%q err=%v", actionURL, err)
	}
	if result, err := gateway.Recheck(
		context.Background(), "nuonuo", "external-task-1",
	); err == nil || result != (AuthoritativeResult{}) {
		t.Fatalf("closed authoritative result=%+v err=%v", result, err)
	}
}

func TestNormalizeManualCompletionEvidence(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	previousClock := timeNowUTC
	timeNowUTC = func() time.Time { return now }
	t.Cleanup(func() { timeNowUTC = previousClock })
	base := ManualCompletionInput{
		TaskNo: "red-letter-task-1", ExpectedVersion: 1,
		IdempotencyKey:    "manual-completion-key-0001",
		EvidenceReference: "red-letter-proof-1",
		EvidenceSHA256:    strings.Repeat("a", 64), OccurredAt: now,
	}
	if _, _, err := normalizeCompletion(base); err != nil {
		t.Fatalf("reference-only evidence rejected: %v", err)
	}
	withObject := base
	withObject.UploadedObjectRef = "private://red-letter/2026/proof-1"
	withObject.MediaType = "application/pdf"
	withObject.ByteSize = 1024
	if _, _, err := normalizeCompletion(withObject); err != nil {
		t.Fatalf("complete private object metadata rejected: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*ManualCompletionInput)
	}{
		{name: "object without media type", mutate: func(input *ManualCompletionInput) {
			input.UploadedObjectRef, input.ByteSize = "private://proof", 1
		}},
		{name: "metadata without object", mutate: func(input *ManualCompletionInput) {
			input.MediaType, input.ByteSize = "application/pdf", 1
		}},
		{name: "control character in object", mutate: func(input *ManualCompletionInput) {
			input.UploadedObjectRef, input.MediaType, input.ByteSize = "private://proof\nother", "application/pdf", 1
		}},
		{name: "oversized object", mutate: func(input *ManualCompletionInput) {
			input.UploadedObjectRef, input.MediaType, input.ByteSize = "private://proof", "application/pdf", 20*1024*1024+1
		}},
		{name: "future evidence", mutate: func(input *ManualCompletionInput) {
			input.OccurredAt = now.Add(6 * time.Minute)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := base
			test.mutate(&input)
			if _, _, err := normalizeCompletion(input); !bizerr.Is(err, CodeInvalid) {
				t.Fatalf("error=%v want invalid", err)
			}
		})
	}
}

func TestTaskIdentityIgnoresAllocationOrder(t *testing.T) {
	leftNo, leftDigest := taskIdentity("refund-operation-1", []string{"b", "a"})
	rightNo, rightDigest := taskIdentity("refund-operation-1", []string{"a", "b"})
	if leftNo != rightNo || leftDigest != rightDigest {
		t.Fatalf("allocation order changed task identity: %s/%s versus %s/%s",
			leftNo, leftDigest, rightNo, rightDigest)
	}
}
