// This file verifies bounded configuration, redacted evidence, and deterministic retry validation.

package asynctask

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// TestNormalizeConfigDefaultsAndSafetyBounds verifies defaults and every hard ceiling.
func TestNormalizeConfigDefaultsAndSafetyBounds(t *testing.T) {
	defaults, err := normalizeConfig(Config{})
	if err != nil {
		t.Fatalf("normalize default configuration failed: %v", err)
	}
	if defaults.LeaseDuration != defaultLeaseDuration ||
		defaults.BaseBackoff != defaultBaseBackoff ||
		defaults.MaximumBackoff != defaultMaximumBackoff ||
		defaults.MaximumClaimBatch != defaultMaximumClaimBatch ||
		defaults.DefaultMaxAttempts != defaultMaxAttempts ||
		defaults.MaximumAttempts != defaultMaximumAttempts ||
		defaults.MaximumPayloadSize != defaultMaximumPayloadSize ||
		defaults.MaximumResultSize != defaultMaximumResultSize {
		t.Fatalf("unexpected default configuration: %+v", defaults)
	}

	ceiling := Config{
		LeaseDuration:  maximumLeaseDuration,
		BaseBackoff:    maximumBackoffDuration,
		MaximumBackoff: maximumBackoffDuration,
		MaximumClaimBatch: maximumClaimEvidenceBytes /
			(2*maximumPayloadSize + maximumResultSize),
		DefaultMaxAttempts: maximumAttempts,
		MaximumAttempts:    maximumAttempts,
		MaximumPayloadSize: maximumPayloadSize,
		MaximumResultSize:  maximumResultSize,
	}
	if _, err = normalizeConfig(ceiling); err != nil {
		t.Fatalf("exact configuration ceilings should be accepted: %v", err)
	}

	tests := []struct {
		name   string
		config Config
	}{
		{name: "negative lease", config: Config{LeaseDuration: -time.Nanosecond}},
		{name: "lease too long", config: Config{LeaseDuration: maximumLeaseDuration + time.Nanosecond}},
		{name: "base backoff too long", config: Config{BaseBackoff: maximumBackoffDuration + time.Nanosecond}},
		{name: "maximum backoff too long", config: Config{MaximumBackoff: maximumBackoffDuration + time.Nanosecond}},
		{name: "claim batch too large", config: Config{MaximumClaimBatch: maximumClaimBatch + 1}},
		{name: "attempts too large", config: Config{MaximumAttempts: maximumAttempts + 1}},
		{name: "payload too large", config: Config{MaximumPayloadSize: maximumPayloadSize + 1}},
		{name: "result too large", config: Config{MaximumResultSize: maximumResultSize + 1}},
		{name: "default attempts too large", config: Config{DefaultMaxAttempts: maximumAttempts + 1}},
		{name: "contradictory backoff", config: Config{BaseBackoff: time.Minute, MaximumBackoff: time.Second}},
		{
			name: "claim payload product too large",
			config: Config{
				MaximumClaimBatch:  maximumClaimBatch,
				MaximumPayloadSize: maximumPayloadSize,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := normalizeConfig(test.config); err == nil {
				t.Fatalf("unsafe configuration was accepted: %+v", test.config)
			}
		})
	}
}

// TestNormalizeJSONObjectRejectsSecretKeyVariants verifies recursive compact-key scanning.
func TestNormalizeJSONObjectRejectsSecretKeyVariants(t *testing.T) {
	keys := []string{
		"token", "accessToken", "id-token", "refresh token", "Password",
		"secret_id", "secretKey", "appKey", "app_secret", "API-Key",
		"apiSecret", "Authorization", "Cookie", "privateKey", "clientSecret",
		"access_key_id", "credentials", "proxyAuthorization", "set-cookie",
		"X-API-Key", "X-Auth-Token", "requestSignature", "sessionId",
	}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{
				"level": []any{map[string]any{key: "plaintext"}},
			})
			if err != nil {
				t.Fatalf("marshal test evidence failed: %v", err)
			}
			if _, err = normalizeJSONObject(raw, 4096); err == nil {
				t.Fatalf("secret-bearing key %q was accepted", key)
			}
		})
	}

	allowed := json.RawMessage(`{
		"accessTokenRef":"vault://token/1",
		"app_secret_reference":"vault://secret/2",
		"private-key-ref":"vault://key/3",
		"nested":[{"apiKeyReference":"vault://key/4"}],
		"tokenizer":"safe-domain-value"
	}`)
	canonical, err := normalizeJSONObject(allowed, 4096)
	if err != nil {
		t.Fatalf("reference-only evidence was rejected: %v", err)
	}
	if !json.Valid(canonical) || len(canonical) == 0 {
		t.Fatalf("reference-only evidence was not canonical JSON: %q", canonical)
	}
}

// TestNormalizeJSONObjectCanonicalizesNestedObjects proves semantically equal
// nested evidence does not diverge solely because callers used a different key order.
func TestNormalizeJSONObjectCanonicalizesNestedObjects(t *testing.T) {
	left, err := normalizeJSONObject(
		json.RawMessage(`{"schema_version":1,"nested":{"alpha":1,"beta":2}}`),
		4096,
	)
	if err != nil {
		t.Fatalf("normalize left nested evidence failed: %v", err)
	}
	right, err := normalizeJSONObject(
		json.RawMessage(`{"nested":{"beta":2,"alpha":1},"schema_version":1}`),
		4096,
	)
	if err != nil {
		t.Fatalf("normalize right nested evidence failed: %v", err)
	}
	if !bytes.Equal(left, right) {
		t.Fatalf("nested object order changed canonical evidence: left=%s right=%s", left, right)
	}
}

// TestStringValidationRejectsControlCharacters verifies log-facing text cannot inject lines.
func TestStringValidationRejectsControlCharacters(t *testing.T) {
	if validRequiredString("worker\nforged", maximumWorkerIDLength) {
		t.Fatal("required string accepted a newline")
	}
	if validOptionalString("reason\tforged", maximumSummaryLength) {
		t.Fatal("optional string accepted a tab")
	}
	if !validRequiredString("正常-worker", maximumWorkerIDLength) {
		t.Fatal("valid UTF-8 worker identity was rejected")
	}

	service := newUnitService(t)
	if _, err := service.normalizeClaimInput(ClaimInput{WorkerID: "worker\rforged", Limit: 1}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("claim control character should return invalid, got %v", err)
	}
	lease := LeaseRef{
		TenantID:   idccap.DefaultTenantID,
		TaskNo:     "TSK_00000000000000000000000000000000",
		LeaseToken: "LSE_00000000000000000000000000000000",
		AttemptNo:  1,
	}
	if _, err := service.Fail(context.Background(), FailureInput{
		Lease:            lease,
		ErrorCode:        "provider_error",
		ErrorSummary:     "safe\nforged",
		ResponseSnapshot: json.RawMessage(`{}`),
	}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("failure summary control character should return invalid, got %v", err)
	}
	if _, err := service.Fail(context.Background(), FailureInput{
		Lease:            lease,
		ErrorCode:        "provider_error",
		ErrorSummary:     "Authorization: Bearer plaintext-token",
		ResponseSnapshot: json.RawMessage(`{}`),
	}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("credential-bearing failure summary should return invalid, got %v", err)
	}
	if _, err := service.TakeOver(context.Background(), TakeOverInput{
		TenantID:       idccap.DefaultTenantID,
		TaskNo:         lease.TaskNo,
		AssigneeUserID: 1,
		Reason:         "reason\nforged",
	}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("manual reason control character should return invalid, got %v", err)
	}
	if _, err := service.TakeOver(context.Background(), TakeOverInput{
		TenantID:       idccap.DefaultTenantID,
		TaskNo:         lease.TaskNo,
		AssigneeUserID: 1,
		Reason:         "credential token=plaintext-token",
	}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("credential-bearing takeover reason should return invalid, got %v", err)
	}
}

// TestNormalizeClaimInputTaskTypes verifies that a domain worker can only
// claim an explicit, bounded and unambiguous set of task families.
func TestNormalizeClaimInputTaskTypes(t *testing.T) {
	service := newUnitService(t)
	normalized, err := service.normalizeClaimInput(ClaimInput{
		WorkerID:  " customer-export ",
		Limit:     1,
		TaskTypes: []string{" customer.data_export ", "customer.archive"},
	})
	if err != nil {
		t.Fatalf("normalize filtered claim failed: %v", err)
	}
	if normalized.WorkerID != "customer-export" || len(normalized.TaskTypes) != 2 ||
		normalized.TaskTypes[0] != "customer.data_export" || normalized.TaskTypes[1] != "customer.archive" {
		t.Fatalf("filtered claim was not normalized: %+v", normalized)
	}

	invalid := []ClaimInput{
		{WorkerID: "worker", Limit: 1, TaskTypes: []string{"customer.data_export", " customer.data_export "}},
		{WorkerID: "worker", Limit: 1, TaskTypes: []string{""}},
		{WorkerID: "worker", Limit: 1, TaskTypes: []string{"customer.data_export\nforged"}},
	}
	tooMany := ClaimInput{WorkerID: "worker", Limit: 1}
	for index := 0; index <= maximumClaimTaskTypes; index++ {
		tooMany.TaskTypes = append(tooMany.TaskTypes, "task."+strings.Repeat("x", index+1))
	}
	invalid = append(invalid, tooMany)
	for index, input := range invalid {
		if _, err = service.normalizeClaimInput(input); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("invalid task type filter %d should be rejected, got %v", index, err)
		}
	}
}

// TestRetryBackoffIsDeterministicAndBounded verifies exponential jitter and cap behavior.
func TestRetryBackoffIsDeterministicAndBounded(t *testing.T) {
	service, err := newService(Config{
		BaseBackoff:    time.Second,
		MaximumBackoff: 10 * time.Second,
	}, time.Now, bytes.NewReader(make([]byte, 64)))
	if err != nil {
		t.Fatalf("create retry test service failed: %v", err)
	}
	for _, test := range []struct {
		attempt int
		floor   time.Duration
		ceiling time.Duration
	}{
		{attempt: 1, floor: time.Second, ceiling: 1250 * time.Millisecond},
		{attempt: 2, floor: 2 * time.Second, ceiling: 2500 * time.Millisecond},
		{attempt: 3, floor: 4 * time.Second, ceiling: 5 * time.Second},
		{attempt: 20, floor: 10 * time.Second, ceiling: 10 * time.Second},
	} {
		first := service.retryBackoff("TSK_deterministic", test.attempt)
		second := service.retryBackoff("TSK_deterministic", test.attempt)
		if first != second || first < test.floor || first > test.ceiling {
			t.Fatalf("attempt %d backoff=%s/%s, expected deterministic range [%s,%s]",
				test.attempt, first, second, test.floor, test.ceiling)
		}
	}
}

// TestPublicValidationRunsBeforePersistence verifies evidence and transaction gates.
func TestPublicValidationRunsBeforePersistence(t *testing.T) {
	service := newUnitService(t)
	input := unitCreateInput()
	if _, err := service.Create(context.Background(), input); !bizerr.Is(err, CodeTransactionRequired) {
		t.Fatalf("create without outer transaction should fail explicitly, got %v", err)
	}
	input.TaskType = "provision\nforged"
	if _, err := service.Create(context.Background(), input); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("invalid create input should fail before transaction lookup, got %v", err)
	}
	lease := LeaseRef{
		TenantID:   idccap.DefaultTenantID,
		TaskNo:     "TSK_00000000000000000000000000000000",
		LeaseToken: "LSE_00000000000000000000000000000000",
		AttemptNo:  1,
	}
	if _, err := service.Complete(context.Background(), CompleteInput{
		Lease:          lease,
		ResultSnapshot: json.RawMessage(`{}`),
	}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("empty success evidence should return invalid, got %v", err)
	}
	if _, err := service.Fail(context.Background(), FailureInput{
		Lease:            lease,
		ResponseSnapshot: json.RawMessage(`{}`),
		ErrorCode:        "provider_error",
	}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("failure without summary should return invalid, got %v", err)
	}
	if _, err := service.MarkResultUnknown(context.Background(), UnknownInput{
		Lease:            lease,
		ResponseSnapshot: json.RawMessage(`{}`),
		ErrorSummary:     "provider response missing",
	}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("unknown result without code should return invalid, got %v", err)
	}
}

// TestNewOpaqueTokenUsesFullRandomInput verifies fixed-width unpredictable token encoding.
func TestNewOpaqueTokenUsesFullRandomInput(t *testing.T) {
	random := bytes.NewReader([]byte(strings.Repeat("a", 16) + strings.Repeat("b", 16)))
	first, err := newOpaqueToken(random, leaseTokenPrefix)
	if err != nil {
		t.Fatalf("generate first token failed: %v", err)
	}
	second, err := newOpaqueToken(random, leaseTokenPrefix)
	if err != nil {
		t.Fatalf("generate second token failed: %v", err)
	}
	if first == second || !strings.HasPrefix(first, leaseTokenPrefix) || len(first) != len(leaseTokenPrefix)+32 {
		t.Fatalf("unexpected opaque tokens: %q %q", first, second)
	}
}

// newUnitService creates a deterministic validation-only service.
func newUnitService(t *testing.T) *serviceImpl {
	t.Helper()
	service, err := newService(
		Config{},
		func() time.Time { return time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC) },
		bytes.NewReader(make([]byte, 256)),
	)
	if err != nil {
		t.Fatalf("create unit service failed: %v", err)
	}
	return service
}

// unitCreateInput returns one minimal valid task intent.
func unitCreateInput() CreateInput {
	return CreateInput{
		TaskType:       "server.provision",
		SubjectType:    "order",
		SubjectNo:      "ORD_unit",
		CustomerNo:     "CUS_unit",
		IdempotencyKey: "unit-create",
		Payload:        json.RawMessage(`{"schema_version":1}`),
	}
}
