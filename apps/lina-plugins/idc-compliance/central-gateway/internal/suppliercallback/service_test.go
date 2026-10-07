// This file verifies callback input and verified-evidence byte boundaries
// without requiring a PostgreSQL integration environment.

package suppliercallback

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecideProviderStatusObservation(t *testing.T) {
	cases := []struct {
		current  string
		incoming string
		want     string
	}{
		{"", "INIT", "applied"},
		{"INIT", "PART", "applied"},
		{"PART", "WILLEXPIRE", "applied"},
		{"WILLEXPIRE", "PART", "stale"},
		{"ALL", "RELIEVED", "applied"},
		{"RELIEVED", "ALL", "stale"},
		{"REJECT", "DEADLINE", "conflict"},
	}
	for _, item := range cases {
		if got := decideProviderStatusObservation(item.current, item.incoming); got != item.want {
			t.Fatalf("provider status %q -> %q decision=%q want=%q", item.current, item.incoming, got, item.want)
		}
	}
}

func TestRawCallbackExactBoundsAndUnsafeHeaders(t *testing.T) {
	headers := map[string][]byte{
		"x1": bytes.Repeat([]byte{1}, 4094),
		"x2": bytes.Repeat([]byte{2}, 4094),
		"x3": bytes.Repeat([]byte{3}, 4094),
		"x4": bytes.Repeat([]byte{4}, 4094),
	}
	input := RawCallback{
		Provider:      ProviderTencentESSBasic,
		Body:          bytes.Repeat([]byte{5}, MaximumRawBodyBytes),
		Signature:     bytes.Repeat([]byte{6}, MaximumSignatureBytes),
		SignedHeaders: headers,
	}
	if !validRawCallback(input) {
		t.Fatal("exact callback byte bounds were rejected")
	}
	input.Body = append(input.Body, 7)
	if validRawCallback(input) {
		t.Fatal("callback body above 256 KiB was accepted")
	}
	input.Body = input.Body[:MaximumRawBodyBytes]
	input.SignedHeaders = map[string][]byte{"x-safe\r\nx-forged": []byte("value")}
	if validRawCallback(input) {
		t.Fatal("unsafe signed header name was accepted")
	}
	input.SignedHeaders = map[string][]byte{"x-safe": []byte("value\r\nforged")}
	if validRawCallback(input) {
		t.Fatal("unsafe signed header value was accepted")
	}
}

func TestVerifiedEvidenceExactBound(t *testing.T) {
	event := VerifiedEvent{
		RouteNo:  "lwr_" + strings.Repeat("1", 32),
		Provider: ProviderTencentESSBasic, OperationScope: "contract.sign",
		ExternalEventID: []byte("event"), Status: StatusSucceeded,
		EvidenceFrame: bytes.Repeat([]byte{1}, MaximumEvidenceFrameBytes),
	}
	if !validVerifiedEvent(event) {
		t.Fatal("exact verified evidence bound was rejected")
	}
	event.EvidenceFrame = append(event.EvidenceFrame, 2)
	if validVerifiedEvent(event) {
		t.Fatal("verified evidence above the fixed bound was accepted")
	}
}
