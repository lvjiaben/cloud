// This file verifies outbox bounds, canonical evidence, secret rejection, and deterministic retry timing.

package outbox

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
)

// TestNormalizeConfigAppliesFiniteDefaultsAndBounds verifies safe construction policy.
func TestNormalizeConfigAppliesFiniteDefaultsAndBounds(t *testing.T) {
	config, err := normalizeConfig(Config{})
	if err != nil {
		t.Fatalf("normalize default configuration failed: %v", err)
	}
	if config.LeaseDuration != defaultLeaseDuration ||
		config.BaseBackoff != defaultBaseBackoff ||
		config.MaximumBackoff != defaultMaximumBackoff ||
		config.MaximumClaimBatch != defaultMaximumClaimBatch ||
		config.DefaultMaxAttempts != defaultMaxAttempts ||
		config.MaximumAttempts != defaultMaximumAttempts ||
		config.MaximumPayloadSize != defaultMaximumPayloadSize ||
		config.MaximumMetadataSize != defaultMaximumMetadataSize {
		t.Fatalf("unexpected normalized defaults: %+v", config)
	}
	invalid := []Config{
		{LeaseDuration: -time.Second},
		{MaximumClaimBatch: maximumClaimBatch + 1},
		{MaximumAttempts: maximumAttempts + 1},
		{BaseBackoff: time.Minute, MaximumBackoff: time.Second},
		{DefaultMaxAttempts: 3, MaximumAttempts: 2},
		{MaximumPayloadSize: maximumPayloadSize + 1},
		{MaximumMetadataSize: maximumMetadataSize + 1},
		{
			MaximumClaimBatch:   maximumClaimBatch,
			MaximumPayloadSize:  maximumPayloadSize,
			MaximumMetadataSize: maximumMetadataSize,
		},
	}
	for index, candidate := range invalid {
		if _, err = normalizeConfig(candidate); err == nil {
			t.Fatalf("invalid configuration %d was accepted: %+v", index, candidate)
		}
	}
}

// TestNormalizeClaimInputCanonicalizesEventTypes verifies a dedicated worker
// receives a stable, bounded exact-type allowlist without changing generic claims.
func TestNormalizeClaimInputCanonicalizesEventTypes(t *testing.T) {
	service, err := newService(Config{MaximumClaimBatch: 8}, time.Now, bytes.NewReader(make([]byte, 64)))
	if err != nil {
		t.Fatalf("create claim validation service failed: %v", err)
	}
	normalized, err := service.normalizeClaimInput(ClaimInput{
		WorkerID:   " worker ",
		EventTypes: []string{"event.z", " event.a ", "event.z"},
	})
	if err != nil {
		t.Fatalf("normalize event-type claim failed: %v", err)
	}
	if normalized.WorkerID != "worker" || normalized.Limit != 8 ||
		len(normalized.EventTypes) != 2 || normalized.EventTypes[0] != "event.a" ||
		normalized.EventTypes[1] != "event.z" {
		t.Fatalf("unexpected normalized event-type claim: %+v", normalized)
	}
	generic, err := service.normalizeClaimInput(ClaimInput{WorkerID: "generic", Limit: 1})
	if err != nil || len(generic.EventTypes) != 0 {
		t.Fatalf("generic claim changed: value=%+v err=%v", generic, err)
	}
	invalid := []ClaimInput{
		{WorkerID: "worker", EventTypes: []string{""}},
		{WorkerID: "worker", EventTypes: []string{"event\ninvalid"}},
		{WorkerID: "worker", EventTypes: []string{strings.Repeat("x", maximumEventTypeLength+1)}},
		{WorkerID: "worker", EventTypes: make([]string, maximumClaimEventTypes+1)},
	}
	for index, input := range invalid {
		if _, normalizeErr := service.normalizeClaimInput(input); !bizerr.Is(normalizeErr, CodeInvalid) {
			t.Fatalf("invalid event-type claim %d returned %v", index, normalizeErr)
		}
	}
}

// TestNormalizeEvidenceCanonicalizesAndRejectsSecrets verifies recursive evidence governance.
func TestNormalizeEvidenceCanonicalizesAndRejectsSecrets(t *testing.T) {
	canonical, err := normalizeEvidence(json.RawMessage(` { "z": 1.00, "a": {"ok": true} } `), 1024)
	if err != nil {
		t.Fatalf("normalize valid evidence failed: %v", err)
	}
	if string(canonical) != `{"a":{"ok":true},"z":1.00}` {
		t.Fatalf("unexpected canonical JSON: %s", canonical)
	}
	empty, err := normalizeEvidence(nil, 2)
	if err != nil || string(empty) != `{}` {
		t.Fatalf("empty evidence was not normalized to object: value=%s err=%v", empty, err)
	}
	invalid := []json.RawMessage{
		json.RawMessage(`[]`),
		json.RawMessage(`null`),
		json.RawMessage(`{} {}`),
		json.RawMessage(`{"nested":{"token":"redact-me"}}`),
		json.RawMessage(`{"items":[{"appKey":"redact-me"}]}`),
		json.RawMessage(`{"app_secret":"redact-me"}`),
		json.RawMessage(`{"secretId":"redact-me"}`),
		json.RawMessage(`{"access-token":"redact-me"}`),
		json.RawMessage(`{"X-API-Key":"redact-me"}`),
		json.RawMessage(`{"X-Auth-Token":"redact-me"}`),
	}
	for index, raw := range invalid {
		if _, err = normalizeEvidence(raw, 1024); !bizerr.Is(err, CodeEvidenceInvalid) {
			t.Fatalf("invalid evidence %d returned %v", index, err)
		}
	}
	if _, err = normalizeEvidence(json.RawMessage(`{"large":"value"}`), 4); !bizerr.Is(err, CodeEvidenceTooLarge) {
		t.Fatalf("oversized evidence returned %v", err)
	}
	allowedReference, err := normalizeEvidence(
		json.RawMessage(`{"xApiKeyRef":"secret://provider/1","tokenizer":"safe"}`),
		1024,
	)
	if err != nil || !json.Valid(allowedReference) {
		t.Fatalf("secret reference evidence was rejected: value=%s err=%v", allowedReference, err)
	}
	_, err = normalizeFailureInput(FailureInput{
		Lease: LeaseRef{
			EventNo:    "EVT_00000000000000000000000000000000",
			LeaseToken: "worker:LSE_00000000000000000000000000000000",
			AttemptNo:  1,
		},
		ErrorCode:    "remote_rejected",
		ErrorSummary: "Authorization: Bearer plaintext-token",
	})
	if !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("credential-bearing error summary returned %v", err)
	}
}

// TestNormalizeEnqueueInputUsesCreationTimeForImplicitAvailability verifies
// normal events retain an operational timestamp while replay can distinguish explicit scheduling.
func TestNormalizeEnqueueInputUsesCreationTimeForImplicitAvailability(t *testing.T) {
	now := time.Date(2026, 8, 6, 10, 11, 12, 123456789, time.FixedZone("test", 8*60*60))
	service, err := newService(Config{}, func() time.Time { return now }, bytes.NewReader(make([]byte, 64)))
	if err != nil {
		t.Fatalf("create deterministic service failed: %v", err)
	}
	input := validUnitEnqueueInput()
	normalized, err := service.normalizeEnqueueInput(input)
	if err != nil {
		t.Fatalf("normalize implicit schedule failed: %v", err)
	}
	if normalized.availableAtExplicit || !normalized.AvailableAt.Equal(databaseTime(now)) {
		t.Fatalf("implicit schedule mismatch: %+v", normalized)
	}
	explicit := now.Add(time.Hour + 321*time.Nanosecond)
	input.AvailableAt = explicit
	normalized, err = service.normalizeEnqueueInput(input)
	if err != nil {
		t.Fatalf("normalize explicit schedule failed: %v", err)
	}
	if !normalized.availableAtExplicit || !normalized.AvailableAt.Equal(databaseTime(explicit)) {
		t.Fatalf("explicit schedule mismatch: %+v", normalized)
	}
}

// TestRetryBackoffIsDeterministicJitteredAndCapped freezes equal-jitter boundaries.
func TestRetryBackoffIsDeterministicJitteredAndCapped(t *testing.T) {
	service, err := newService(
		Config{BaseBackoff: 8 * time.Second, MaximumBackoff: 20 * time.Second},
		time.Now,
		bytes.NewReader(make([]byte, 64)),
	)
	if err != nil {
		t.Fatalf("create retry service failed: %v", err)
	}
	cases := []struct {
		attempt int
		minimum time.Duration
		maximum time.Duration
		want    time.Duration
	}{
		{attempt: 1, minimum: 4 * time.Second, maximum: 8 * time.Second, want: 7_233_086_162 * time.Nanosecond},
		{attempt: 2, minimum: 8 * time.Second, maximum: 16 * time.Second, want: 9_474_733_826 * time.Nanosecond},
		{attempt: 3, minimum: 10 * time.Second, maximum: 20 * time.Second, want: 11_070_786_456 * time.Nanosecond},
		{attempt: 20, minimum: 10 * time.Second, maximum: 20 * time.Second, want: 17_586_962_653 * time.Nanosecond},
	}
	for _, item := range cases {
		got := service.retryBackoff("EVT_FIXED", item.attempt)
		if got < item.minimum || got > item.maximum || got != item.want {
			t.Fatalf("attempt %d delay=%s, bounds=[%s,%s], want=%s", item.attempt, got, item.minimum, item.maximum, item.want)
		}
		if repeated := service.retryBackoff("EVT_FIXED", item.attempt); repeated != got {
			t.Fatalf("attempt %d jitter changed across replay: %s then %s", item.attempt, got, repeated)
		}
	}
	if service.retryBackoff("EVT_OTHER", 3) == service.retryBackoff("EVT_FIXED", 3) {
		t.Fatal("different event numbers unexpectedly shared deterministic jitter")
	}
}

// TestNewOpaqueTokenProducesBoundedOpaqueValues verifies identifier shape and entropy failures.
func TestNewOpaqueTokenProducesBoundedOpaqueValues(t *testing.T) {
	token, err := newOpaqueToken(bytes.NewReader(bytes.Repeat([]byte{0xab}, 16)), eventNumberPrefix)
	if err != nil {
		t.Fatalf("generate deterministic event number failed: %v", err)
	}
	if token != "EVT_abababababababababababababababab" || len(token) > maximumEventNoLength {
		t.Fatalf("unexpected event token %q", token)
	}
	if _, err = newOpaqueToken(bytes.NewReader(nil), eventNumberPrefix); err == nil {
		t.Fatal("short random source did not fail closed")
	}
}

// validUnitEnqueueInput returns one redacted versioned event envelope.
func validUnitEnqueueInput() EnqueueInput {
	return EnqueueInput{
		EventType:        "server.order.created",
		EventVersion:     1,
		Topic:            "server.lifecycle",
		AggregateType:    "server_order",
		AggregateNo:      "ORD_TEST",
		CorrelationNo:    "COR_TEST",
		DeduplicationKey: "dedupe-test",
		Payload:          json.RawMessage(`{"orderNo":"ORD_TEST"}`),
		Metadata:         json.RawMessage(`{"source":"unit"}`),
	}
}
