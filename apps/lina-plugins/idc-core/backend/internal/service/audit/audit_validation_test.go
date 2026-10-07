// This file verifies actor, redaction, proxy, permission, and append-only contract invariants.

package audit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

var (
	testDigestKey    = []byte("0123456789abcdef0123456789abcdef")
	testIntegrityKey = []byte("abcdef0123456789abcdef0123456789")
	testLookupKey    = []byte("fedcba9876543210fedcba9876543210")
)

func testAuditConfig() Config {
	return Config{
		ReservationLookupKey: IntegrityKey{
			ID:     "test-reservation-lookup",
			Secret: testLookupKey,
		},
		ActiveReplayDigestKey: IntegrityKey{
			ID:     "test-replay-active",
			Secret: testDigestKey,
		},
		ActiveIntegrityKey: IntegrityKey{
			ID:     "test-active",
			Secret: testIntegrityKey,
		},
	}
}

type staticActorResolver struct {
	actor TrustedActor
	err   error
}

func (r staticActorResolver) ResolveTrustedActor(context.Context) (TrustedActor, error) {
	return r.actor, r.err
}

type staticBizCtxService struct {
	current bizctxcap.CurrentContext
}

func (s staticBizCtxService) Current(context.Context) bizctxcap.CurrentContext {
	return s.current
}

type staticPrincipalService struct {
	audience *principal.Audience
	err      error
}

type staticAuthorizationService struct {
	grants        map[authz.PermissionKey]bool
	platformAdmin bool
	err           error
}

func (s staticAuthorizationService) BatchGetPermissions(
	context.Context,
	[]authz.PermissionKey,
) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, s.err
}

func (s staticAuthorizationService) BatchHasPermissions(
	_ context.Context,
	keys []authz.PermissionKey,
) (map[authz.PermissionKey]bool, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := make(map[authz.PermissionKey]bool, len(keys))
	for _, key := range keys {
		result[key] = s.grants[key]
	}
	return result, nil
}

func (s staticAuthorizationService) HasPermission(
	_ context.Context,
	key authz.PermissionKey,
) (bool, error) {
	return s.grants[key], s.err
}

func (s staticAuthorizationService) IsPlatformAdmin(
	context.Context,
	authz.UserID,
) (bool, error) {
	return s.platformAdmin, s.err
}

func (s staticAuthorizationService) ReplaceRolePermissions(
	context.Context,
	authz.RoleID,
	[]authz.PermissionKey,
) error {
	return s.err
}

func (s staticPrincipalService) Current(context.Context) (*principal.Audience, error) {
	return s.audience, s.err
}

func (staticPrincipalService) EnsureCurrentStaff(context.Context) (*principal.Audience, error) {
	return nil, nil
}

func (staticPrincipalService) RedeemCustomerOnboarding(
	context.Context,
	int,
	string,
	string,
	int,
) (*principal.CustomerBinding, error) {
	return nil, nil
}

func (staticPrincipalService) OnboardingStatus(context.Context) (*principal.OnboardingState, error) {
	return &principal.OnboardingState{}, nil
}

// TestNewRejectsMissingTrustDependenciesAndWeakDigestKey verifies constructor fail-closed behavior.
func TestNewRejectsMissingTrustDependenciesAndWeakDigestKey(t *testing.T) {
	principalService := staticPrincipalService{}
	bizCtxService := staticBizCtxService{}
	authorization := staticAuthorizationService{}
	if _, err := New(nil, principalService, authorization, testAuditConfig()); err == nil {
		t.Fatal("expected missing bizctx service to fail")
	}
	if _, err := New(bizCtxService, nil, authorization, testAuditConfig()); err == nil {
		t.Fatal("expected missing principal service to fail")
	}
	if _, err := New(bizCtxService, principalService, nil, testAuditConfig()); err == nil {
		t.Fatal("expected missing authorization service to fail")
	}
	weakReplayConfig := testAuditConfig()
	weakReplayConfig.ActiveReplayDigestKey.Secret = []byte("short")
	if _, err := New(bizCtxService, principalService, authorization, weakReplayConfig); err == nil {
		t.Fatal("expected weak digest key to fail")
	}
	weakLookupConfig := testAuditConfig()
	weakLookupConfig.ReservationLookupKey.Secret = []byte("short")
	if _, err := New(bizCtxService, principalService, authorization, weakLookupConfig); err == nil {
		t.Fatal("expected weak reservation lookup key to fail")
	}
	weakIntegrityConfig := testAuditConfig()
	weakIntegrityConfig.ActiveIntegrityKey.Secret = []byte("short")
	if _, err := New(bizCtxService, principalService, authorization, weakIntegrityConfig); err == nil {
		t.Fatal("expected weak integrity key to fail")
	}
	sharedKeyConfig := testAuditConfig()
	sharedKeyConfig.ActiveIntegrityKey.Secret = testDigestKey
	if _, err := New(bizCtxService, principalService, authorization, sharedKeyConfig); err == nil {
		t.Fatal("expected shared replay and row-integrity key to fail")
	}
	sharedLookupKey := testAuditConfig()
	sharedLookupKey.ReservationLookupKey.Secret = testDigestKey
	if _, err := New(bizCtxService, principalService, authorization, sharedLookupKey); err == nil {
		t.Fatal("expected shared reservation-lookup and replay key to fail")
	}
	duplicateReplayID := testAuditConfig()
	duplicateReplayID.RetainedReplayDigestKeys = []IntegrityKey{{
		ID:     duplicateReplayID.ActiveReplayDigestKey.ID,
		Secret: []byte("retained-replay-key-0123456789abcdef"),
	}}
	if _, err := New(bizCtxService, principalService, authorization, duplicateReplayID); err == nil {
		t.Fatal("expected duplicate replay key ID to fail")
	}
	sharedRetainedKey := testAuditConfig()
	sharedRetainedKey.RetainedReplayDigestKeys = []IntegrityKey{{
		ID:     "test-replay-retained",
		Secret: testIntegrityKey,
	}}
	if _, err := New(bizCtxService, principalService, authorization, sharedRetainedKey); err == nil {
		t.Fatal("expected retained replay and row-integrity key reuse to fail")
	}
	overBoundReplayKeys := testAuditConfig()
	for index := 0; index <= maximumRetainedReplayKeys; index++ {
		overBoundReplayKeys.RetainedReplayDigestKeys = append(
			overBoundReplayKeys.RetainedReplayDigestKeys,
			IntegrityKey{
				ID:     "retained-" + strconv.Itoa(index),
				Secret: []byte(strings.Repeat(string(rune('A'+index)), minimumDigestKeyBytes)),
			},
		)
	}
	if _, err := New(bizCtxService, principalService, authorization, overBoundReplayKeys); err == nil {
		t.Fatal("expected replay keyring above its fixed bound to fail")
	}
	proxyConfig := testAuditConfig()
	proxyConfig.TrustedProxyCIDRs = []string{"not-a-prefix"}
	if _, err := New(
		bizCtxService,
		principalService,
		authorization,
		proxyConfig,
	); err == nil {
		t.Fatal("expected invalid trusted proxy CIDR to fail")
	}
}

// TestNewRejectsCredentialBearingKeyIDs keeps every configured HMAC key ID
// from becoming a reversible plaintext channel in persisted integrity evidence.
func TestNewRejectsCredentialBearingKeyIDs(t *testing.T) {
	encodedCredential := "6170694b65793d736563726574"
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{
			name: "reservation lookup",
			mutate: func(config *Config) {
				config.ReservationLookupKey.ID = encodedCredential
			},
		},
		{
			name: "active replay",
			mutate: func(config *Config) {
				config.ActiveReplayDigestKey.ID = encodedCredential
			},
		},
		{
			name: "retained replay",
			mutate: func(config *Config) {
				config.RetainedReplayDigestKeys = []IntegrityKey{{
					ID:     encodedCredential,
					Secret: []byte("retained-replay-key-0123456789abcdef"),
				}}
			},
		},
		{
			name: "active integrity",
			mutate: func(config *Config) {
				config.ActiveIntegrityKey.ID = encodedCredential
			},
		},
		{
			name: "retained integrity",
			mutate: func(config *Config) {
				config.RetainedIntegrityKeys = []IntegrityKey{{
					ID:     encodedCredential,
					Secret: []byte("retained-integrity-key-0123456789abcdef"),
				}}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testAuditConfig()
			test.mutate(&config)
			if _, err := newService(
				staticActorResolver{actor: TrustedActor{Type: ActorSystem, Reference: "system.scheduler"}},
				staticAuthorizationService{},
				config,
				time.Now,
				strings.NewReader(strings.Repeat("a", 64)),
			); err == nil {
				t.Fatal("credential-bearing HMAC key ID was accepted")
			}
		})
	}

	if _, err := parseIntegritySnapshot(map[string]any{
		"version":         json.Number(strconv.Itoa(integritySchemaVersion)),
		"key_id":          encodedCredential,
		"replay_key_id":   "test-replay-active",
		"raw_replay_hmac": strings.Repeat("a", 64),
		"row_hmac":        strings.Repeat("b", 64),
	}); err == nil {
		t.Fatal("persisted credential-bearing integrity key ID was accepted")
	}
	if _, err := parseIntegritySnapshot(map[string]any{
		"version":         json.Number(strconv.Itoa(integritySchemaVersion)),
		"key_id":          "test-active",
		"replay_key_id":   encodedCredential,
		"raw_replay_hmac": strings.Repeat("a", 64),
		"row_hmac":        strings.Repeat("b", 64),
	}); err == nil {
		t.Fatal("persisted credential-bearing replay key ID was accepted")
	}
}

// TestNewRejectsTypedNilDependencies verifies interface-typed nil values never
// survive construction and panic later in a request path.
func TestNewRejectsTypedNilDependencies(t *testing.T) {
	var (
		nilBizCtx        *staticBizCtxService
		nilPrincipal     *staticPrincipalService
		nilAuthorization *staticAuthorizationService
		nilResolver      *staticActorResolver
		nilReader        *strings.Reader
	)
	principalService := staticPrincipalService{}
	bizCtxService := staticBizCtxService{}
	authorization := staticAuthorizationService{}
	if _, err := New(nilBizCtx, principalService, authorization, testAuditConfig()); err == nil {
		t.Fatal("expected typed-nil bizctx service to fail")
	}
	if _, err := New(bizCtxService, nilPrincipal, authorization, testAuditConfig()); err == nil {
		t.Fatal("expected typed-nil principal service to fail")
	}
	if _, err := New(bizCtxService, principalService, nilAuthorization, testAuditConfig()); err == nil {
		t.Fatal("expected typed-nil authorization service to fail")
	}
	if _, err := newService(nilResolver, authorization, testAuditConfig(), time.Now, io.Reader(nilReader)); err == nil {
		t.Fatal("expected typed-nil actor resolver to fail")
	}
	if _, err := newService(staticActorResolver{}, authorization, testAuditConfig(), time.Now, io.Reader(nilReader)); err == nil {
		t.Fatal("expected typed-nil random reader to fail")
	}
}

// TestUserActorResolverUsesBizctxAndPersistedAudience verifies default actor
// resolution cannot take identity fields from business input.
func TestUserActorResolverUsesBizctxAndPersistedAudience(t *testing.T) {
	resolver, err := newUserActorResolver(
		staticBizCtxService{current: bizctxcap.CurrentContext{
			UserID:   42,
			TenantID: 0,
		}},
		staticPrincipalService{audience: &principal.Audience{
			UserID:     42,
			Code:       principal.AudienceCustomer,
			Status:     principal.AudienceStatusActive,
			CustomerNo: "CUS_TEST",
		}},
	)
	if err != nil {
		t.Fatalf("create user actor resolver: %v", err)
	}
	actor, err := resolver.ResolveTrustedActor(context.Background())
	if err != nil {
		t.Fatalf("resolve user actor: %v", err)
	}
	if actor.Type != ActorUser || actor.UserID != 42 ||
		actor.AudienceCode != principal.AudienceCustomer || actor.CustomerNo != "CUS_TEST" {
		t.Fatalf("unexpected resolved actor: %+v", actor)
	}

	resolver.principalSvc = staticPrincipalService{audience: &principal.Audience{
		UserID: 43,
		Code:   principal.AudienceStaff,
		Status: principal.AudienceStatusActive,
	}}
	if _, err = resolver.ResolveTrustedActor(context.Background()); !bizerr.Is(err, CodeIdentityDenied) {
		t.Fatalf("inconsistent audience returned %v", err)
	}
}

// TestNormalizeEntryRecursivelyRedactsButFingerprintsRawSecretChanges proves
// persisted snapshots are safe while idempotent semantics still detect changed secrets.
func TestNormalizeEntryRecursivelyRedactsButFingerprintsRawSecretChanges(t *testing.T) {
	service := newUnitService(t, TrustedActor{
		Type:         ActorUser,
		UserID:       7,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_ONE",
	})
	input := unitTransitionInput()
	input.BeforeSnapshot = map[string]any{
		"state": "old",
		"note":  "Authorization: Bearer plaintext-header-token",
		"nested": []any{map[string]any{
			"api-key":      "plaintext-one",
			"password_ref": "vault://password/one",
		}, "token=plaintext-array-token"},
	}
	input.AfterSnapshot = map[string]any{
		"state": "running",
		"note":  "-----BEGIN OPENSSH PRIVATE KEY----- plaintext-key",
	}
	input.ContextSnapshot = map[string]any{
		"note":        "Cookie: session=plaintext-cookie",
		"spaced_note": "API key = plaintext-spaced-key",
	}
	first, err := service.normalizeEntry(
		context.Background(),
		entryKindTransition,
		input.Fields,
		input.BeforeSnapshot,
		input.AfterSnapshot,
	)
	if err != nil {
		t.Fatalf("normalize first entry: %v", err)
	}
	firstRow := buildUnitPersistedAuditRow(t, service, first)
	joined := string(first.beforeSnapshot) + string(first.afterSnapshot) + firstRow.ContextSnapshot
	if strings.Contains(joined, "plaintext-one") ||
		strings.Contains(joined, "plaintext-header-token") ||
		strings.Contains(joined, "plaintext-array-token") ||
		strings.Contains(joined, "plaintext-key") ||
		strings.Contains(joined, "plaintext-cookie") ||
		strings.Contains(joined, "plaintext-spaced-key") ||
		!strings.Contains(string(first.beforeSnapshot), redactedValue) ||
		strings.Contains(string(first.beforeSnapshot), "vault://password/one") {
		t.Fatalf("unexpected redacted snapshots: %s", joined)
	}
	if !strings.Contains(string(first.beforeSnapshot), `"schema_version":1`) {
		t.Fatalf("snapshot is not versioned: %s", first.beforeSnapshot)
	}

	changed := input.BeforeSnapshot.(map[string]any)
	changed["nested"].([]any)[0].(map[string]any)["api-key"] = "plaintext-two"
	second, err := service.normalizeEntry(
		context.Background(),
		entryKindTransition,
		input.Fields,
		changed,
		input.AfterSnapshot,
	)
	if err != nil {
		t.Fatalf("normalize changed entry: %v", err)
	}
	if string(first.beforeSnapshot) != string(second.beforeSnapshot) {
		t.Fatalf("redacted snapshots should converge: first=%s second=%s", first.beforeSnapshot, second.beforeSnapshot)
	}
	secondRow := buildUnitPersistedAuditRow(t, service, second)
	if firstRow.ContextSnapshot == secondRow.ContextSnapshot {
		t.Fatal("keyed raw-evidence fingerprint did not change")
	}
	if strings.Contains(firstRow.ContextSnapshot, "plaintext-one") ||
		strings.Contains(secondRow.ContextSnapshot, "plaintext-two") {
		t.Fatal("context snapshot leaked raw secret material")
	}
	stored, err := service.parseStoredStateEnvelope(`{"schema_version":1,"data":{"note":"Bearer persisted-token"}}`)
	if err != nil || strings.Contains(string(stored.redacted), "persisted-token") ||
		!strings.Contains(string(stored.redacted), redactedValue) {
		t.Fatalf("defensive read redaction failed: stored=%s err=%v", stored.redacted, err)
	}
}

// TestNormalizeObjectRejectsDuplicateKeysRecursively verifies every streamed
// object rejects duplicate names, including subtrees that are later redacted.
func TestNormalizeObjectRejectsDuplicateKeysRecursively(t *testing.T) {
	service := newUnitService(t, TrustedActor{Type: ActorSystem, Reference: "system.scheduler"})
	tests := []string{
		`{"state":"old","state":"new"}`,
		`{"nested":{"state":"old","state":"new"}}`,
		`{"items":[{"state":"old","state":"new"}]}`,
		`{"password":{"left":1,"left":2}}`,
	}
	for _, value := range tests {
		if _, err := service.normalizeObject(json.RawMessage(value)); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("duplicate-key document %s returned %v", value, err)
		}
	}
	input := unitTransitionInput()
	for _, location := range []string{"before", "after", "context"} {
		candidate := input
		duplicate := json.RawMessage(`{"nested":{"value":1,"value":2}}`)
		switch location {
		case "before":
			candidate.BeforeSnapshot = duplicate
		case "after":
			candidate.AfterSnapshot = duplicate
		case "context":
			candidate.ContextSnapshot = duplicate
		}
		if _, err := service.normalizeEntry(
			context.Background(),
			entryKindTransition,
			candidate.Fields,
			candidate.BeforeSnapshot,
			candidate.AfterSnapshot,
		); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("duplicate key in %s evidence returned %v", location, err)
		}
	}
}

// TestCatalogCategoryAuditPayload remains compatible with the sealed audit contract.
func TestCatalogCategoryAuditPayload(t *testing.T) {
	fields := Fields{
		IdempotencyScope: "catalog.category.create", IdempotencyKey: "ctg_0123456789abcdef0123456789abcdef",
		OperationCode: "catalog.category.create", TargetType: "catalog_category",
		TargetNo: "ctg_0123456789abcdef0123456789abcdef", Result: ResultSucceeded,
		ReasonCode: "manual_create", Risk: RiskMedium,
		RequiredPermission: "idc:admin:catalog:category:create",
	}
	if _, err := normalizeFields(fields); err != nil {
		t.Fatalf("catalog category audit fields: %v", err)
	}
	service := newUnitService(t, TrustedActor{Type: ActorSystem, Reference: "system.scheduler"})
	if _, err := service.normalizeObject(map[string]any{
		"state": "active", "category_type": "cloud_server", "parent_category_no": "",
	}); err != nil {
		t.Fatalf("catalog category audit snapshot: %v", err)
	}
}

// TestNormalizeEntryRejectsReservedBatchCustomerEvidence prevents ordinary
// audit callers from forging the sealed customer-link count or digest.
func TestNormalizeEntryRejectsReservedBatchCustomerEvidence(t *testing.T) {
	service := newUnitService(t, TrustedActor{Type: ActorSystem, Reference: "system.scheduler"})
	for _, key := range []string{
		batchCustomerCountContextKey,
		batchCustomerDigestContextKey,
	} {
		input := unitTransitionInput()
		input.Fields.ContextSnapshot = map[string]any{key: "forged"}
		if _, err := service.normalizeEntry(
			context.Background(),
			entryKindTransition,
			input.Fields,
			input.BeforeSnapshot,
			input.AfterSnapshot,
		); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("reserved context key %q returned %v", key, err)
		}
	}
}

// TestNormalizeObjectRejectsCredentialBearingKeys prevents a JSON property
// name from preserving plaintext that value-only redaction cannot remove.
func TestNormalizeObjectRejectsCredentialBearingKeys(t *testing.T) {
	service := newUnitService(t, TrustedActor{Type: ActorSystem, Reference: "system.scheduler"})
	for _, key := range []string{
		"apiKey=must-not-pass",
		base64.StdEncoding.EncodeToString([]byte("apiKey=must-not-pass")),
		"6170694b65793d6d7573742d6e6f742d70617373",
	} {
		if _, err := service.normalizeObject(map[string]any{key: "ordinary"}); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("credential-bearing JSON key %q returned %v", key, err)
		}
	}
}

// TestCanonicalJSONNumberMatchesJSONB verifies exponent and signed-zero forms
// are normalized before their bytes enter persisted-row integrity material.
func TestCanonicalJSONNumberMatchesJSONB(t *testing.T) {
	tests := map[string]string{
		"1e2":     "100",
		"1E+2":    "100",
		"1.20e2":  "120",
		"1.20e1":  "12.0",
		"1e-2":    "0.01",
		"0.00120": "0.00120",
		"-0e-2":   "0.00",
	}
	for input, expected := range tests {
		actual, err := canonicalJSONNumber(json.Number(input), defaultMaximumSnapshotBytes)
		if err != nil || actual.String() != expected {
			t.Fatalf("canonical number %q = %q, err=%v, want %q", input, actual, err, expected)
		}
	}
	if _, err := canonicalJSONNumber(json.Number("1e999999"), defaultMaximumSnapshotBytes); !bizerr.Is(err, CodeEvidenceTooLarge) {
		t.Fatalf("oversized exponent returned %v", err)
	}
}

// TestSerializedPageSizeMatchesJSONEncoding locks the incremental response
// budget calculation to the actual Page JSON representation.
func TestSerializedPageSizeMatchesJSONEncoding(t *testing.T) {
	events := []Event{
		{AuditNo: "AUD_00000000000000000000000000000001", BeforeSnapshot: json.RawMessage(`{"schema_version":1,"data":{}}`)},
		{AuditNo: "AUD_00000000000000000000000000000002", AfterSnapshot: json.RawMessage(`{"schema_version":1,"data":{"state":"running"}}`)},
	}
	eventBytes := 0
	for index := range events {
		encoded, err := json.Marshal(events[index])
		if err != nil {
			t.Fatalf("encode page event %d: %v", index, err)
		}
		eventBytes += len(encoded)
	}
	next := &Cursor{OccurredAt: time.Date(2026, 8, 7, 1, 2, 3, 456000, time.UTC), InternalID: 42}
	actual, err := serializedPageSize(eventBytes, len(events), next)
	if err != nil {
		t.Fatalf("measure page response: %v", err)
	}
	encoded, err := json.Marshal(&Page{Events: events, Next: next})
	if err != nil {
		t.Fatalf("encode page response: %v", err)
	}
	if actual != len(encoded) {
		t.Fatalf("incremental page size=%d, actual=%d, payload=%s", actual, len(encoded), encoded)
	}
}

// TestSnapshotRedactionRejectsCredentialBypasses covers credential-bearing key
// suffixes and encoded header, assignment, path, and array value forms.
func TestSnapshotRedactionRejectsCredentialBypasses(t *testing.T) {
	service := newUnitService(t, TrustedActor{Type: ActorSystem, Reference: "system.scheduler"})
	doubleEncoded := url.QueryEscape(url.QueryEscape("/token/double-secret"))
	encodedAssignment := base64.StdEncoding.EncodeToString([]byte("apiKey=base64-secret"))
	wrappedAssignment := "payload=" + base64.StdEncoding.EncodeToString([]byte("apiKey=wrapped-secret"))
	for range 6 {
		wrappedAssignment = url.QueryEscape(wrappedAssignment)
	}
	object, err := service.normalizeObject(map[string]any{
		"passwordRef":                "vault-reference-secret",
		"apiKeyValue":                "api-value-secret",
		"providerApiKeyValue":        "provider-api-value-secret",
		"providerApiKeyValueArchive": "decorated-api-value-secret",
		"tokenDigest":                "token-digest-secret",
		"secretFingerprint":          "fingerprint-secret",
		"headers": []any{
			"Proxy-Authorization: Basic proxy-secret",
			"Cookie=session-cookie-secret",
		},
		"path":          "https://example.invalid/v1/token/path-secret",
		"encoded_path":  doubleEncoded,
		"encoded_value": encodedAssignment,
		"wrapped_value": wrappedAssignment,
		"ordinary_note": "basic service plan",
	})
	if err != nil {
		t.Fatalf("normalize credential bypass evidence: %v", err)
	}
	encoded, err := json.Marshal(object.redacted)
	if err != nil {
		t.Fatalf("encode credential bypass evidence: %v", err)
	}
	for _, secret := range []string{
		"vault-reference-secret",
		"api-value-secret",
		"provider-api-value-secret",
		"decorated-api-value-secret",
		"token-digest-secret",
		"fingerprint-secret",
		"proxy-secret",
		"session-cookie-secret",
		"path-secret",
		"double-secret",
		"base64-secret",
		"wrapped-secret",
	} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("credential bypass %q survived redaction: %s", secret, encoded)
		}
	}
	if strings.Count(string(encoded), redactedValue) < 7 {
		t.Fatalf("credential bypass evidence was not comprehensively redacted: %s", encoded)
	}
	if !strings.Contains(string(encoded), "basic service plan") {
		t.Fatalf("ordinary free text was over-redacted: %s", encoded)
	}
}

// TestCredentialScanBudgetExhaustionFailsClosed proves total-work exhaustion
// cannot turn an adversarially encoded value into accepted plaintext.
func TestCredentialScanBudgetExhaustionFailsClosed(t *testing.T) {
	if !containsCredentialBearingText("payload=YXBpS2V5PXNlY3JldA==") {
		t.Fatal("credential scanner missed a wrapped base64 apiKey assignment")
	}
	budget := credentialScanBudget{remainingBytes: 1, remainingCandidates: 1}
	if !containsCredentialBearingTextWithBudget("ordinary text", &budget) {
		t.Fatal("credential scanner accepted text after exhausting its total work budget")
	}
}

// TestCredentialScannerStructuralBoundsFailClosed covers the exact depth and
// fragment caps so reaching either bound cannot silently skip another layer.
func TestCredentialScannerStructuralBoundsFailClosed(t *testing.T) {
	deeplyEncoded := "apiKey=must-not-pass"
	for range maximumCredentialScanDepth + 1 {
		deeplyEncoded = url.QueryEscape(deeplyEncoded)
	}
	if !containsCredentialBearingText(deeplyEncoded) {
		t.Fatal("credential scanner accepted a credential beyond its decode-depth bound")
	}

	var fragmented strings.Builder
	for index := range maximumCredentialFragments {
		fragmented.WriteString("part")
		fragmented.WriteString(strconv.Itoa(index))
		fragmented.WriteString("=ordinary-value;")
	}
	fragmented.WriteString("payload=")
	fragmented.WriteString(base64.StdEncoding.EncodeToString([]byte("apiKey=must-not-pass")))
	if !containsCredentialBearingText(fragmented.String()) {
		t.Fatal("credential scanner accepted a credential beyond its fragment bound")
	}
}

// TestCredentialScannerRejectsURIUnicodeHTMLAndEncodedMatrix is a bounded
// regression matrix for scalar columns and nested evidence redaction.
func TestCredentialScannerRejectsURIUnicodeHTMLAndEncodedMatrix(t *testing.T) {
	encodedAssignment := base64.StdEncoding.EncodeToString([]byte("apiKey=must-not-pass"))
	hexAssignment := "6170694b65793d6d7573742d6e6f742d70617373"
	for _, value := range []string{
		"https://user:secret@host.invalid/path",
		"postgres://user:secret@host.invalid/database",
		"redis://:secret@host.invalid/0",
		"Bearer\u00a0secret",
		"Bearer\u200bsecret",
		"api&nbsp;key=secret",
		"private%2Bkey=secret",
		encodedAssignment,
		hexAssignment,
		"dG9rZW49eA",
		"746f6b656e3d78",
		"payload=" + encodedAssignment,
	} {
		if !containsCredentialBearingText(value) {
			t.Fatalf("credential scanner accepted %q", value)
		}
	}
	for _, value := range []string{
		"https://host.invalid/public/status",
		"ordinary customer note",
	} {
		if containsCredentialBearingText(value) {
			t.Fatalf("credential scanner over-redacted %q", value)
		}
	}
}

// TestReservationLookupIsStableAcrossReplayRotation proves lookup identity is
// not derived from a replay key that operators are allowed to retire.
func TestReservationLookupIsStableAcrossReplayRotation(t *testing.T) {
	actor := TrustedActor{Type: ActorSystem, Reference: "system.scheduler"}
	oldService := newUnitService(t, actor)
	rotatedConfig := testAuditConfig()
	rotatedConfig.ActiveReplayDigestKey = IntegrityKey{
		ID:     "test-replay-rotated",
		Secret: []byte("rotated-replay-key-0123456789abcdef"),
	}
	rotatedService, err := newService(
		staticActorResolver{actor: actor},
		staticAuthorizationService{},
		rotatedConfig,
		time.Now,
		strings.NewReader(strings.Repeat("r", 64)),
	)
	if err != nil {
		t.Fatalf("create replay-rotated service: %v", err)
	}
	fields := unitTransitionInput().Fields
	oldCandidates, err := oldService.auditReservationCandidates(fields, actor)
	if err != nil {
		t.Fatalf("derive old reservation lookup: %v", err)
	}
	rotatedCandidates, err := rotatedService.auditReservationCandidates(fields, actor)
	if err != nil {
		t.Fatalf("derive rotated reservation lookup: %v", err)
	}
	if !reflect.DeepEqual(oldCandidates, rotatedCandidates) {
		t.Fatalf("replay rotation changed stable locator: old=%+v rotated=%+v", oldCandidates, rotatedCandidates)
	}
}

// TestStoredEnvelopeRejectsSchemaDrift verifies persisted evidence accepts only
// the exact versioned state/context schema with strongly typed members.
func TestStoredEnvelopeRejectsSchemaDrift(t *testing.T) {
	service := newUnitService(t, TrustedActor{Type: ActorSystem, Reference: "system.scheduler"})
	for _, value := range []string{
		`{"data":{}}`,
		`{"schema_version":1,"data":{},"extra":true}`,
		`{"schema_version":1.0,"data":{}}`,
		`{"schema_version":1e0,"data":{}}`,
		`{"schema_version":2,"data":{}}`,
		`{"schema_version":1,"data":[]}`,
	} {
		if _, err := service.parseStoredStateEnvelope(value); err == nil {
			t.Fatalf("state schema drift was accepted: %s", value)
		}
	}
	normalized, err := service.normalizeEntry(
		context.Background(),
		entryKindTransition,
		unitTransitionInput().Fields,
		unitTransitionInput().BeforeSnapshot,
		unitTransitionInput().AfterSnapshot,
	)
	if err != nil {
		t.Fatalf("normalize strict-envelope fixture: %v", err)
	}
	row, reservation := buildUnitPersistedAuditRecord(t, service, normalized)
	if _, err = service.verifyPersistedEvent(row, reservation); err != nil {
		t.Fatalf("verify valid strict envelope: %v", err)
	}
	var contextValue map[string]any
	if err = json.Unmarshal([]byte(row.ContextSnapshot), &contextValue); err != nil {
		t.Fatalf("decode strict context fixture: %v", err)
	}
	contextValue["extra"] = true
	mutated, err := json.Marshal(contextValue)
	if err != nil {
		t.Fatalf("encode strict context mutation: %v", err)
	}
	if _, err = service.parseStoredContextEnvelope(string(mutated)); err == nil {
		t.Fatal("context envelope accepted an unknown field")
	}
	exponentVersion := strings.Replace(
		row.ContextSnapshot,
		`"integrity":{"version":1`,
		`"integrity":{"version":1e0`,
		1,
	)
	if exponentVersion == row.ContextSnapshot {
		t.Fatal("strict context fixture did not contain integrity version")
	}
	if _, err = service.parseStoredContextEnvelope(exponentVersion); err == nil {
		t.Fatal("context envelope accepted an exponent-form integrity version")
	}
	contextMutations := []func(map[string]any){
		func(value map[string]any) {
			delete(value["security"].(map[string]any), "actor_audience_code")
		},
		func(value map[string]any) {
			value["security"].(map[string]any)["unknown"] = true
		},
		func(value map[string]any) {
			value["integrity"].(map[string]any)["version"] = "1"
		},
		func(value map[string]any) {
			delete(value["integrity"].(map[string]any), "row_hmac")
		},
		func(value map[string]any) {
			delete(value["integrity"].(map[string]any), "replay_key_id")
		},
	}
	for index, mutate := range contextMutations {
		var candidate map[string]any
		if err = json.Unmarshal([]byte(row.ContextSnapshot), &candidate); err != nil {
			t.Fatalf("decode context mutation %d: %v", index, err)
		}
		mutate(candidate)
		encoded, encodeErr := json.Marshal(candidate)
		if encodeErr != nil {
			t.Fatalf("encode context mutation %d: %v", index, encodeErr)
		}
		if _, parseErr := service.parseStoredContextEnvelope(string(encoded)); parseErr == nil {
			t.Fatalf("context schema mutation %d was accepted: %s", index, encoded)
		}
	}
}

// TestNormalizeRequestEvidenceUsesTrustedProxyChainAndHMAC verifies untrusted
// forwarded headers cannot select the stored source digest.
func TestNormalizeRequestEvidenceUsesTrustedProxyChainAndHMAC(t *testing.T) {
	config := testAuditConfig()
	config.TrustedProxyCIDRs = []string{"10.0.0.0/8", "192.0.2.0/24"}
	service, err := newService(
		staticActorResolver{actor: TrustedActor{Type: ActorSystem}},
		staticAuthorizationService{},
		config,
		time.Now,
		strings.NewReader(strings.Repeat("x", 64)),
	)
	if err != nil {
		t.Fatalf("create request test service: %v", err)
	}
	trustedRaw, err := service.normalizeRequestEvidence(RequestEvidence{
		RemoteAddress: "10.0.0.2:443",
		ForwardedFor:  "203.0.113.8, 192.0.2.9",
		UserAgent:     " Example   Agent/1.0 ",
	})
	if err != nil {
		t.Fatalf("normalize trusted request: %v", err)
	}
	trusted, err := service.requestEvidenceForKey(
		trustedRaw,
		service.config.ActiveReplayDigestKey.Secret,
	)
	if err != nil {
		t.Fatalf("digest trusted request: %v", err)
	}
	if trusted.ipDigest != mustKeyedDigest(t, service, "source-ip", "203.0.113.8") ||
		trusted.userAgentDigest != mustKeyedDigest(t, service, "user-agent", "Example Agent/1.0") {
		t.Fatalf("unexpected trusted request digests: %+v", trusted)
	}
	if strings.Contains(trusted.ipDigest, "203.0.113.8") ||
		strings.Contains(trusted.userAgentDigest, "Example") {
		t.Fatal("raw request metadata leaked through digest")
	}

	untrustedRaw, err := service.normalizeRequestEvidence(RequestEvidence{
		RemoteAddress: "198.51.100.10:443",
		ForwardedFor:  "203.0.113.9",
	})
	if err != nil {
		t.Fatalf("normalize untrusted request: %v", err)
	}
	untrusted, err := service.requestEvidenceForKey(
		untrustedRaw,
		service.config.ActiveReplayDigestKey.Secret,
	)
	if err != nil {
		t.Fatalf("digest untrusted request: %v", err)
	}
	if untrusted.ipDigest != mustKeyedDigest(t, service, "source-ip", "198.51.100.10") {
		t.Fatalf("untrusted forwarded header selected source: %+v", untrusted)
	}
}

// TestReservationScopeBindsTrustedActor verifies idempotency cannot collide
// across user, acting-user, audience, customer, or trusted reference boundaries.
func TestReservationScopeBindsTrustedActor(t *testing.T) {
	service := newUnitService(t, TrustedActor{Type: ActorSystem, Reference: "system.scheduler"})
	fields := unitTransitionInput().Fields
	fields.CustomerNo = "CUS_ONE"
	actors := []TrustedActor{
		{Type: ActorUser, UserID: 10, AudienceCode: principal.AudienceCustomer, CustomerNo: "CUS_ONE"},
		{Type: ActorUser, UserID: 11, AudienceCode: principal.AudienceCustomer, CustomerNo: "CUS_ONE"},
		{Type: ActorUser, UserID: 10, ActingUserID: 12, AudienceCode: principal.AudienceCustomer, CustomerNo: "CUS_ONE", Delegated: true},
		{Type: ActorUser, UserID: 10, AudienceCode: principal.AudienceStaff},
		{Type: ActorSystem, Reference: "system.scheduler"},
		{Type: ActorSystem, Reference: "system.reconciler"},
		{Type: ActorExternal, Reference: "provider.callback"},
	}
	seen := make(map[string]struct{}, len(actors)+1)
	for _, actor := range actors {
		candidates, err := service.auditReservationCandidates(fields, actor)
		if err != nil || len(candidates) != 1 ||
			!isCanonicalDigest(candidates[0].reservationScopeDigest) {
			t.Fatalf(
				"derive actor-bound reservation scope for %+v: candidates=%v err=%v",
				actor,
				candidates,
				err,
			)
		}
		scope := candidates[0].reservationScopeDigest
		if _, duplicate := seen[scope]; duplicate {
			t.Fatalf("trusted actor boundary produced a duplicate scope: %+v", actor)
		}
		seen[scope] = struct{}{}
		if actor.Reference != "" && strings.Contains(scope, actor.Reference) {
			t.Fatalf("trusted reference leaked into reservation scope: %s", scope)
		}
	}
	otherCustomer := fields
	otherCustomer.CustomerNo = "CUS_TWO"
	candidates, err := service.auditReservationCandidates(otherCustomer, actors[0])
	if err != nil || len(candidates) != 1 {
		t.Fatalf(
			"derive affected-customer reservation scope: candidates=%v err=%v",
			candidates,
			err,
		)
	}
	if _, duplicate := seen[candidates[0].reservationScopeDigest]; duplicate {
		t.Fatal("affected customer was not bound into the reservation scope")
	}
}

// TestReplayAndRowMACAreSeparated verifies raw request replay and persisted-row
// integrity use independent keys, domains, and constant-time comparisons.
func TestReplayAndRowMACAreSeparated(t *testing.T) {
	service := newUnitService(t, TrustedActor{
		Type:         ActorUser,
		UserID:       7,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_ONE",
	})
	input := unitTransitionInput()
	normalized, err := service.normalizeEntry(
		context.Background(),
		entryKindTransition,
		input.Fields,
		input.BeforeSnapshot,
		input.AfterSnapshot,
	)
	if err != nil {
		t.Fatalf("normalize separated-MAC entry: %v", err)
	}
	row, reservation := buildUnitPersistedAuditRecord(t, service, normalized)
	verified, err := service.verifyPersistedEvent(row, reservation)
	if err != nil {
		t.Fatalf("verify separated-MAC fixture: %v", err)
	}
	if equalHMAC(verified.parsed.integrity.RawReplayHMAC, verified.parsed.integrity.RowHMAC) {
		t.Fatal("raw replay and persisted-row MAC unexpectedly match")
	}
	tamperedIdentityRow := *row
	tamperedIdentityReservation := *reservation
	tamperedIdentityRow.Id++
	tamperedIdentityReservation.OwnerInternalId = tamperedIdentityRow.Id
	if _, err = service.verifyPersistedEvent(
		&tamperedIdentityRow,
		&tamperedIdentityReservation,
	); err == nil {
		t.Fatal("simultaneous row and owner internal ID tampering passed verification")
	}
	row.TargetNo = "SRV_TAMPERED"
	if _, err = service.verifyPersistedEvent(row, reservation); err == nil {
		t.Fatal("business scalar tampering passed persisted-row verification")
	}
}

// TestWriteAndReadAuthorityFailClosed verifies customer ownership, concrete
// staff permission evidence, and non-representable read scopes are rejected.
func TestWriteAndReadAuthorityFailClosed(t *testing.T) {
	customer := TrustedActor{
		Type:         ActorUser,
		UserID:       1,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_ONE",
	}
	service := newUnitServiceWithAuthorization(t, customer, staticAuthorizationService{})
	fields := Fields{CustomerNo: "CUS_TWO", Result: ResultSucceeded}
	if _, err := service.validateWriteAuthority(context.Background(), customer, &fields); !bizerr.Is(err, CodePermissionDenied) {
		t.Fatalf("cross-customer write returned %v", err)
	}
	fields = Fields{Result: ResultSucceeded}
	if evidence, err := service.validateWriteAuthority(context.Background(), customer, &fields); err != nil || evidence.granted() || fields.CustomerNo != "CUS_ONE" {
		t.Fatalf("own customer write failed: fields=%+v evidence=%+v err=%v", fields, evidence, err)
	}

	staff := TrustedActor{
		Type:         ActorUser,
		UserID:       2,
		AudienceCode: principal.AudienceStaff,
		DataScope:    hostDataScopeDepartment,
	}
	authorizedService := newUnitServiceWithAuthorization(t, staff, staticAuthorizationService{
		grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionAuditRead): true},
	})
	if err := authorizedService.requireReadAuthority(context.Background(), staff); !bizerr.Is(err, CodePermissionDenied) {
		t.Fatalf("department-scoped reader returned %v", err)
	}
	staff.DataScope = hostDataScopeTenant
	if err := authorizedService.requireReadAuthority(context.Background(), staff); err != nil {
		t.Fatalf("tenant-scoped authorized reader failed: %v", err)
	}
	permissionlessService := newUnitServiceWithAuthorization(t, staff, staticAuthorizationService{})
	if err := permissionlessService.requireReadAuthority(context.Background(), staff); !bizerr.Is(err, CodePermissionDenied) {
		t.Fatalf("permissionless reader returned %v", err)
	}
	failingService := newUnitServiceWithAuthorization(t, staff, staticAuthorizationService{err: errors.New("authz unavailable")})
	if err := failingService.requireReadAuthority(context.Background(), staff); !bizerr.Is(err, CodePermissionDenied) {
		t.Fatalf("authorization failure did not fail closed: %v", err)
	}
}

// TestProductionLikeBizctxUsesAuthzEvidence proves empty production bizctx
// permission fields do not reject a host-authorized staff operation or grant
// access merely because tenant zero reports PlatformBypass.
func TestProductionLikeBizctxUsesAuthzEvidence(t *testing.T) {
	bizCtx := staticBizCtxService{current: bizctxcap.CurrentContext{
		UserID:         88,
		TenantID:       0,
		DataScope:      hostDataScopeTenant,
		PlatformBypass: true,
		Permissions:    nil,
		IsSuperAdmin:   false,
	}}
	principalService := staticPrincipalService{audience: &principal.Audience{
		UserID: 88,
		Code:   principal.AudienceStaff,
		Status: principal.AudienceStatusActive,
	}}
	authorized, err := New(
		bizCtx,
		principalService,
		staticAuthorizationService{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionAuditRead): true,
		}},
		testAuditConfig(),
	)
	if err != nil {
		t.Fatalf("create production-like authorized service: %v", err)
	}
	actor, err := authorized.(*serviceImpl).resolveActor(context.Background())
	if err != nil {
		t.Fatalf("resolve production-like staff actor: %v", err)
	}
	if err = authorized.(*serviceImpl).requireReadAuthority(context.Background(), actor); err != nil {
		t.Fatalf("host authz grant was not honored: %v", err)
	}

	unauthorized, err := New(
		bizCtx,
		principalService,
		staticAuthorizationService{},
		testAuditConfig(),
	)
	if err != nil {
		t.Fatalf("create production-like unauthorized service: %v", err)
	}
	if err = unauthorized.(*serviceImpl).requireReadAuthority(context.Background(), actor); !bizerr.Is(err, CodePermissionDenied) {
		t.Fatalf("PlatformBypass incorrectly granted audit permission: %v", err)
	}
}

// TestServiceInterfaceIsStrictlyAppendOnly prevents mutable audit operations
// from being added to the service contract without an explicit governance review.
func TestServiceInterfaceIsStrictlyAppendOnly(t *testing.T) {
	typeOfService := reflect.TypeOf((*Service)(nil)).Elem()
	allowed := map[string]struct{}{
		"AppendBatchTransition": {},
		"AppendTransition":      {},
		"RecordObservation":     {},
		"Search":                {},
	}
	if typeOfService.NumMethod() != len(allowed) {
		t.Fatalf("audit service exposes %d methods, want %d", typeOfService.NumMethod(), len(allowed))
	}
	for index := range typeOfService.NumMethod() {
		method := typeOfService.Method(index)
		if _, ok := allowed[method.Name]; !ok {
			t.Fatalf("audit service exposes mutable or unknown method %q", method.Name)
		}
	}
	persistedType := reflect.TypeOf(entity.BusinessAuditEvent{})
	for _, forbidden := range []string{"UpdatedAt", "DeletedAt", "Version"} {
		if _, exists := persistedType.FieldByName(forbidden); exists {
			t.Fatalf("append-only audit entity unexpectedly exposes mutable field %q", forbidden)
		}
	}
}

func TestBatchAuditCustomerLinksRequireCanonicalBoundedUniqueNumbers(t *testing.T) {
	valid := []string{
		"CUS_22222222222222222222222222222222",
		"CUS_11111111111111111111111111111111",
	}
	normalized, err := normalizeAffectedCustomerNos(valid)
	if err != nil || normalized[0] != "CUS_11111111111111111111111111111111" {
		t.Fatalf("normalize affected customers=%v err=%v", normalized, err)
	}
	for _, values := range [][]string{
		nil,
		{valid[0], valid[0]},
		{"CUS_invalid"},
		append(make([]string, maximumAffectedCustomers), valid[0]),
	} {
		if _, err = normalizeAffectedCustomerNos(values); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("invalid affected customers %v returned %v", values, err)
		}
	}
}

// TestSnapshotBoundsRejectDeepOrOversizedEvidence verifies finite recursive limits.
func TestSnapshotBoundsRejectDeepOrOversizedEvidence(t *testing.T) {
	config := testAuditConfig()
	config.MaximumSnapshotBytes = 256
	config.MaximumSnapshotDepth = 2
	config.MaximumSnapshotNodes = 8
	config.MaximumResponseBytes = minimumEventScalarAllowance + 3*config.MaximumSnapshotBytes
	service, err := newService(
		staticActorResolver{actor: TrustedActor{Type: ActorSystem}},
		staticAuthorizationService{},
		config,
		time.Now,
		strings.NewReader(strings.Repeat("x", 64)),
	)
	if err != nil {
		t.Fatalf("create bounded service: %v", err)
	}
	if _, err = service.normalizeObject(map[string]any{"a": map[string]any{"b": map[string]any{"c": 1}}}); !bizerr.Is(err, CodeEvidenceTooLarge) {
		t.Fatalf("deep evidence returned %v", err)
	}
	if _, err = service.normalizeObject(map[string]any{"value": strings.Repeat("x", 300)}); !bizerr.Is(err, CodeEvidenceTooLarge) {
		t.Fatalf("oversized evidence returned %v", err)
	}
}

// TestSearchTargetFilterRequiresPair keeps the composite target index usable
// and prevents ambiguous type-only or number-only audit searches.
func TestSearchTargetFilterRequiresPair(t *testing.T) {
	service := newUnitService(t, TrustedActor{Type: ActorSystem, Reference: "system.scheduler"})
	for _, input := range []SearchInput{
		{TargetType: "server"},
		{TargetNo: "SRV_ONE"},
	} {
		if _, err := service.normalizeSearchInput(input); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("unpaired target filter %+v returned %v", input, err)
		}
	}
	if normalized, err := service.normalizeSearchInput(SearchInput{
		TargetType: "server",
		TargetNo:   "SRV_ONE",
	}); err != nil || normalized.TargetNo != "SRV_ONE" {
		t.Fatalf("paired target filter was rejected: normalized=%+v err=%v", normalized, err)
	}
}

// TestIndependentAuditColumnsRejectCredentialBearingText prevents non-JSON
// columns from becoming alternate plaintext credential stores.
func TestIndependentAuditColumnsRejectCredentialBearingText(t *testing.T) {
	base := unitTransitionInput().Fields
	tests := []struct {
		name   string
		mutate func(*Fields)
	}{
		{name: "operation code", mutate: func(value *Fields) { value.OperationCode = "Authorization: Bearer secret" }},
		{name: "hex operation code", mutate: func(value *Fields) { value.OperationCode = "6170694b65793d736563726574" }},
		{name: "short hex operation code", mutate: func(value *Fields) { value.OperationCode = "746f6b656e3d78" }},
		{name: "target type", mutate: func(value *Fields) { value.TargetType = "token=secret" }},
		{name: "hex target type", mutate: func(value *Fields) { value.TargetType = "6170694b65793d736563726574" }},
		{name: "target number", mutate: func(value *Fields) { value.TargetNo = "SRV_token=secret" }},
		{name: "short raw-base64 target number", mutate: func(value *Fields) { value.TargetNo = "dG9rZW49eA" }},
		{name: "target URI userinfo", mutate: func(value *Fields) { value.TargetNo = "https://user:secret@host.invalid" }},
		{name: "customer number", mutate: func(value *Fields) { value.CustomerNo = "CUS_Bearer secret" }},
		{name: "reason code", mutate: func(value *Fields) { value.ReasonCode = "password=secret" }},
		{name: "hex reason code", mutate: func(value *Fields) { value.ReasonCode = "6170694b65793d736563726574" }},
		{name: "hex required permission", mutate: func(value *Fields) { value.RequiredPermission = "6170694b65793d736563726574" }},
		{name: "correlation number", mutate: func(value *Fields) { value.CorrelationNo = "COR_token=secret" }},
		{name: "correlation HTML entity", mutate: func(value *Fields) { value.CorrelationNo = "api&nbsp;key=secret" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := base
			test.mutate(&candidate)
			if _, err := normalizeFields(candidate); !bizerr.Is(err, CodeInvalid) {
				t.Fatalf("credential-bearing %s returned %v", test.name, err)
			}
		})
	}
	service := newUnitService(t, TrustedActor{
		Type:         ActorUser,
		UserID:       9,
		AudienceCode: principal.AudienceCustomer,
		CustomerNo:   "CUS_ONE",
	})
	for _, request := range []RequestEvidence{
		{RequestID: "REQ_token=secret"},
		{TraceID: "TRACE_Authorization: Bearer secret"},
		{RequestID: "Bearer\u00a0secret"},
		{TraceID: base64.StdEncoding.EncodeToString([]byte("apiKey=secret"))},
	} {
		if _, err := service.normalizeRequestEvidence(request); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("credential-bearing request identifier returned %v", err)
		}
	}
}

// TestResolveActorRejectsInconsistentTrustedEvidence verifies that even the
// internal resolver seam cannot persist contradictory actor attribution.
func TestResolveActorRejectsInconsistentTrustedEvidence(t *testing.T) {
	tests := []TrustedActor{
		{
			Type:         ActorUser,
			UserID:       10,
			AudienceCode: principal.AudienceCustomer,
			CustomerNo:   "6170694b65793d736563726574",
		},
		{
			Type:         ActorUser,
			UserID:       10,
			AudienceCode: principal.AudienceStaff,
			Delegated:    true,
		},
		{
			Type:           ActorExternal,
			Reference:      "provider.callback",
			PlatformBypass: true,
		},
	}
	for _, actor := range tests {
		service := newUnitService(t, actor)
		if _, err := service.resolveActor(context.Background()); !bizerr.Is(err, CodeIdentityDenied) {
			t.Fatalf("inconsistent actor %+v returned %v", actor, err)
		}
	}
}

func newUnitService(t *testing.T, actor TrustedActor) *serviceImpl {
	return newUnitServiceWithAuthorization(t, actor, staticAuthorizationService{})
}

func newUnitServiceWithAuthorization(
	t *testing.T,
	actor TrustedActor,
	authorization authz.Service,
) *serviceImpl {
	t.Helper()
	service, err := newService(
		staticActorResolver{actor: actor},
		authorization,
		testAuditConfig(),
		time.Now,
		strings.NewReader(strings.Repeat("x", 256)),
	)
	if err != nil {
		t.Fatalf("create unit audit service: %v", err)
	}
	return service
}

// mustKeyedDigest returns a keyed digest or fails the current test.
func mustKeyedDigest(t *testing.T, service *serviceImpl, domain string, value string) string {
	t.Helper()
	digest, err := keyedDigestWithKey(
		service.config.ActiveReplayDigestKey.Secret,
		domain,
		value,
	)
	if err != nil {
		t.Fatalf("compute keyed digest: %v", err)
	}
	return digest
}

func buildUnitPersistedAuditRow(
	t *testing.T,
	service *serviceImpl,
	input normalizedEntry,
) *entity.BusinessAuditEvent {
	t.Helper()
	row, _ := buildUnitPersistedAuditRecord(t, service, input)
	return row
}

func buildUnitPersistedAuditRecord(
	t *testing.T,
	service *serviceImpl,
	input normalizedEntry,
) (*entity.BusinessAuditEvent, *entity.BusinessNumber) {
	t.Helper()
	reservation := &entity.BusinessNumber{
		TenantId:               0,
		NumberType:             businessNumberTypeAudit,
		BusinessNo:             "AUD_0123456789abcdef0123456789abcdef",
		ReservationScopeDigest: input.reservationCandidates[0].reservationScopeDigest,
		IdempotencyKeyDigest:   input.reservationCandidates[0].idempotencyKeyDigest,
	}
	row, err := service.buildUnsealedAuditRow(
		input,
		reservation,
		databaseTime(time.Date(2026, 8, 7, 1, 2, 3, 0, time.UTC)),
	)
	if err != nil {
		t.Fatalf("build unit persisted audit row: %v", err)
	}
	row.Id = 101
	reservation.OwnerType = businessNumberOwnerAudit
	reservation.OwnerInternalId = row.Id
	reservation.AssignedAt = row.OccurredAt
	contextSnapshot, err := service.buildSealedContextSnapshot(input, row, reservation)
	if err != nil {
		t.Fatalf("seal unit persisted audit row: %v", err)
	}
	row.ContextSnapshot = string(contextSnapshot)
	return row, reservation
}

func unitTransitionInput() TransitionInput {
	return TransitionInput{
		Fields: Fields{
			IdempotencyScope: "server.lifecycle",
			IdempotencyKey:   "request-one",
			OperationCode:    "server.start",
			TargetType:       "server",
			TargetNo:         "SRV_ONE",
			Result:           ResultSucceeded,
			Risk:             RiskMedium,
			ContextSnapshot:  json.RawMessage(`{"source":"unit"}`),
		},
		BeforeSnapshot: map[string]any{"state": "stopped"},
		AfterSnapshot:  map[string]any{"state": "running"},
	}
}
