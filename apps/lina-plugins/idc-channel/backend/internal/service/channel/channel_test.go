package channel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	idcchannel "lina-plugin-idc-channel"
)

func TestCreateTokenValidationFailsClosedBeforeDatabase(t *testing.T) {
	svc := &service{}
	cases := []struct {
		name string
		in   TokenInput
		want string
	}{
		{name: "missing scopes", in: TokenInput{ChannelID: "demo"}, want: "scope or source limit"},
		{name: "invalid source cidr", in: TokenInput{ChannelID: "demo", Scopes: []string{"catalog.read"}, SourceCIDRs: []string{"not-a-cidr"}}, want: "source restriction"},
		{name: "too many sources", in: TokenInput{ChannelID: "demo", Scopes: []string{"catalog.read"}, SourceCIDRs: make([]string, 33)}, want: "source limit"},
		{name: "too many scopes", in: TokenInput{ChannelID: "demo", Scopes: make([]string, 33)}, want: "scope or source limit"},
		{name: "expired token", in: TokenInput{ChannelID: "demo", Scopes: []string{"catalog.read"}, ExpiresAt: func() *time.Time { v := time.Now().Add(-time.Minute); return &v }()}, want: "expiry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateToken(context.Background(), tc.in)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("expected fail-closed validation containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestTokenScopeAndSourceAuthorization(t *testing.T) {
	if !tokenScopeAllows([]string{"catalog.read"}, "catalog.read") ||
		tokenScopeAllows([]string{"catalog.read"}, "order.read") ||
		!tokenScopeAllows([]string{TokenScopeAll}, "service.terminate") {
		t.Fatal("token scope matching did not remain exact")
	}
	if !tokenSourceAllows(nil, "not-an-ip") ||
		!tokenSourceAllows([]string{"192.0.2.0/24"}, "192.0.2.10") ||
		tokenSourceAllows([]string{"192.0.2.0/24"}, "198.51.100.10") ||
		tokenSourceAllows([]string{"not-a-cidr"}, "192.0.2.10") {
		t.Fatal("token source restrictions did not fail closed")
	}
}

func TestRequiredScopeForRequest(t *testing.T) {
	cases := map[string]string{
		"GET /x/idc-channel/api/v1/channel/catalog":                                              "catalog.read",
		"POST /x/idc-channel/api/v1/channel/quotes":                                              "catalog.read",
		"GET /x/idc-channel/api/v1/channel/orders/10":                                            "order.read",
		"POST /x/idc-channel/api/v1/channel/orders":                                              "order.create",
		"POST /x/idc-channel/api/v1/channel/orders/10/cancel":                                    "order.cancel",
		"POST /x/idc-channel/api/v1/channel/services/SVC_1/operations/reinstall":                 "service.reinstall",
		"POST /x/idc-channel/api/v1/channel/services/SVC_1/operations/COP_1/confidential-output": "service.output",
		"GET /x/idc-channel/api/v1/channel/webhooks/evt_01234567890123456789012345678901":        "webhook.read",
	}
	for request, want := range cases {
		parts := strings.SplitN(request, " ", 2)
		if got := requiredScopeForRequest(parts[0], parts[1]); got != want {
			t.Fatalf("%s: expected %q, got %q", request, want, got)
		}
	}
	if got := requiredScopeForRequest("POST", "/x/idc-channel/api/v1/channel/services/SVC_1/operations/start"); got != "" {
		t.Fatalf("unknown operation unexpectedly acquired scope %q", got)
	}
}

func TestRequestFingerprintBindsOrderPayload(t *testing.T) {
	base := OrderInput{ChannelID: "demo", RequestID: "req-001", ExternalCustomerID: "customer-1", OwnershipMode: "delegated_customer", ProductVersionID: 10, Currency: "CNY"}
	if got := requestFingerprint(base); got == "" {
		t.Fatal("fingerprint must not be empty")
	}
	variants := []OrderInput{
		{ChannelID: base.ChannelID, RequestID: base.RequestID, ExternalCustomerID: "customer-2", OwnershipMode: base.OwnershipMode, ProductVersionID: base.ProductVersionID, Currency: base.Currency},
		{ChannelID: base.ChannelID, RequestID: base.RequestID, ExternalCustomerID: base.ExternalCustomerID, OwnershipMode: "channel_owned", ProductVersionID: base.ProductVersionID, Currency: base.Currency},
		{ChannelID: base.ChannelID, RequestID: base.RequestID, ExternalCustomerID: base.ExternalCustomerID, OwnershipMode: base.OwnershipMode, ProductVersionID: 11, Currency: base.Currency},
		{ChannelID: base.ChannelID, RequestID: base.RequestID, ExternalCustomerID: base.ExternalCustomerID, OwnershipMode: base.OwnershipMode, ProductVersionID: base.ProductVersionID, Currency: "USD"},
	}
	for i, variant := range variants {
		if requestFingerprint(base) == requestFingerprint(variant) {
			t.Fatalf("variant %d changed payload but retained fingerprint", i)
		}
	}
	if requestFingerprint(base) != requestFingerprint(base) {
		t.Fatal("fingerprint must be deterministic")
	}
}

func TestCancelFingerprintBindsOrderID(t *testing.T) {
	a := CancelInput{ChannelID: "demo", RequestID: "request-001", OrderID: 10}
	if cancelFingerprint(a) == cancelFingerprint(CancelInput{ChannelID: "demo", RequestID: "request-001", OrderID: 11}) {
		t.Fatal("cancel fingerprint must bind order id")
	}
}

func TestGetOrderRequiresChannelScope(t *testing.T) {
	svc := &service{}
	for _, tc := range []struct {
		name      string
		orderID   int64
		channelID string
	}{
		{name: "missing order", orderID: 0, channelID: "demo"},
		{name: "missing channel", orderID: 10, channelID: ""},
		{name: "blank channel", orderID: 10, channelID: "  "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.GetOrder(context.Background(), tc.orderID, tc.channelID)
			if err == nil || !strings.Contains(err.Error(), "order not found") {
				t.Fatalf("expected uniform not-found error, got %v", err)
			}
		})
	}
}

func TestTokenSummaryNeverIncludesSecret(t *testing.T) {
	payload, err := json.Marshal(TokenSummary{ID: 42, ChannelID: "channel"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(payload)), "token") || strings.Contains(strings.ToLower(string(payload)), "hash") {
		t.Fatal("token summary must not contain a token or hash field")
	}
}

func TestCapabilitiesAdvertiseCoreGuardedLifecycleActions(t *testing.T) {
	capabilities := (&service{}).Capabilities(context.Background())
	if capabilities.Operations["quote"] != "enabled" || capabilities.Operations["create_order"] != "enabled" {
		t.Fatalf("core channel operations must remain enabled: %#v", capabilities.Operations)
	}
	for _, operation := range []string{"renew", "reconfigure", "reinstall", "console", "change_ip", "terminate"} {
		if capabilities.Operations[operation] != "enabled_core_guarded" {
			t.Fatalf("operation %q must use the guarded core runtime, got %q", operation, capabilities.Operations[operation])
		}
	}
}

func TestLifecycleOperationsFailClosedWithoutApprovedChannel(t *testing.T) {
	service := &service{}
	operations := []string{"renew", "reconfigure", "reinstall", "console", "change_ip", "terminate"}
	for _, operation := range operations {
		t.Run(operation, func(t *testing.T) {
			result, err := service.Lifecycle(context.Background(), LifecycleInput{
				ChannelID: "channel-demo", ServiceNo: "service-1", RequestID: "request-123",
				Operation: operation, Parameters: json.RawMessage(`{}`), Confirmed: true,
			})
			if err == nil || result != (LifecycleResult{}) {
				t.Fatalf("lifecycle operation %s unexpectedly executed: result=%+v err=%v", operation, result, err)
			}
			if !strings.Contains(err.Error(), "channel is not available") {
				t.Fatalf("lifecycle operation %s did not fail at the channel gate: %v", operation, err)
			}
		})
	}
}

func TestLifecycleRejectsUnknownAndMalformedRequestsBeforeExecution(t *testing.T) {
	service := &service{}
	if _, err := service.Lifecycle(context.Background(), LifecycleInput{
		ChannelID: "channel-demo", ServiceNo: "service-1", RequestID: "short", Operation: "renew",
	}); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("malformed lifecycle request was not rejected as invalid: %v", err)
	}
	if _, err := service.Lifecycle(context.Background(), LifecycleInput{
		ChannelID: "channel-demo", ServiceNo: "service-1", RequestID: "request-123", Operation: "start",
	}); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("unknown lifecycle operation was not rejected as invalid: %v", err)
	}
}

func TestWebhookSignatureBindsVersionAndEvent(t *testing.T) {
	secret := []byte("test-secret")
	payload := []byte(`{"status":"paid"}`)
	signature, err := SignWebhook(secret, "v1", "evt-001", payload)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyWebhook(secret, "v1", "evt-001", payload, signature) {
		t.Fatal("signature should verify for the original event")
	}
	if VerifyWebhook(secret, "v2", "evt-001", payload, signature) || VerifyWebhook(secret, "v1", "evt-002", payload, signature) || VerifyWebhook(secret, "v1", "evt-001", []byte(`{"status":"cancelled"}`), signature) {
		t.Fatal("signature must bind version, event id, and payload")
	}
	if VerifyWebhook([]byte("wrong-secret"), "v1", "evt-001", payload, signature) {
		t.Fatal("signature must reject a different secret")
	}
}

func TestWebhookSignatureRejectsMissingInputs(t *testing.T) {
	if _, err := SignWebhook(nil, "v1", "evt-001", nil); err == nil {
		t.Fatal("missing signing inputs must fail closed")
	}
}

func TestWebhookEventIDIsUniquePerChannelInSchema(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "manifest", "sql", "001-channel-core.sql")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	schema := string(payload)
	if !strings.Contains(schema, `UNIQUE ("channel_id", "event_id")`) {
		t.Fatal("webhook schema must reject duplicate event ids within one channel")
	}
}

func TestCapabilitiesFailClosedForUnverifiedAdapters(t *testing.T) {
	caps := (&service{}).Capabilities(context.Background())
	if !caps.WebhookDelivery {
		t.Fatal("versioned webhook delivery must be advertised")
	}
	for _, adapter := range []string{"zjmf-finance", "v10", "whmcs"} {
		if got := caps.Adapters[adapter]; got != "disabled_unverified" {
			t.Fatalf("adapter %q must remain disabled without verified evidence, got %q", adapter, got)
		}
	}
	for _, operation := range []string{"renew", "reconfigure", "reinstall", "console", "change_ip", "terminate"} {
		if got := caps.Operations[operation]; got != "enabled_core_guarded" {
			t.Fatalf("operation %q must remain core guarded, got %q", operation, got)
		}
	}
}

func TestStableWebhookEventIDIsDeterministicAndBoundToIdentity(t *testing.T) {
	base := WebhookEventInput{ChannelID: "demo", Version: "v1", EventType: "order.updated", SubjectRef: "order-100", Sequence: 2, Payload: []byte(`{"status":"paid"}`), Secret: []byte("secret")}
	a, err := StableWebhookEventID(base)
	if err != nil || !strings.HasPrefix(a, "evt_") {
		t.Fatalf("stable event id failed: %q %v", a, err)
	}
	b, err := StableWebhookEventID(base)
	if err != nil || a != b {
		t.Fatalf("same event identity must produce the same id: %q != %q", a, b)
	}
	for i, variant := range []WebhookEventInput{
		{ChannelID: "other", Version: base.Version, EventType: base.EventType, SubjectRef: base.SubjectRef, Sequence: base.Sequence},
		{ChannelID: base.ChannelID, Version: "v2", EventType: base.EventType, SubjectRef: base.SubjectRef, Sequence: base.Sequence},
		{ChannelID: base.ChannelID, Version: base.Version, EventType: "invoice.paid", SubjectRef: base.SubjectRef, Sequence: base.Sequence},
		{ChannelID: base.ChannelID, Version: base.Version, EventType: base.EventType, SubjectRef: base.SubjectRef, Sequence: 3},
	} {
		c, variantErr := StableWebhookEventID(variant)
		if variantErr != nil || c == a {
			t.Fatalf("variant %d must not reuse event id: %q %v", i, c, variantErr)
		}
	}
}

func TestWebhookRetryBackoffIsBoundedAndStops(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	want := []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute, 30 * time.Minute}
	for attempt, delay := range want {
		got, ok := WebhookRetryAt(now, attempt+1)
		if !ok || !got.Equal(now.Add(delay)) {
			t.Fatalf("attempt %d: expected %s, got %s (ok=%v)", attempt+1, now.Add(delay), got, ok)
		}
	}
	if _, ok := WebhookRetryAt(now, WebhookMaxAttempts); ok {
		t.Fatal("retry budget must stop after the sixth retry window")
	}
}

func TestWebhookFailureIsStoredAsClosedCode(t *testing.T) {
	if got := normalizeWebhookFailure(" timeout "); got != "timeout" {
		t.Fatalf("known retry failure should be preserved as a code, got %q", got)
	}
	if got := normalizeWebhookFailure("provider said secret=abc"); got != "delivery_failed" {
		t.Fatalf("unknown failure must not persist provider text, got %q", got)
	}
}

func TestWebhookDeliverySchemaIncludesLeaseAndReplayFields(t *testing.T) {
	base := filepath.Join("..", "..", "..", "..", "manifest", "sql")
	first, err := os.ReadFile(filepath.Join(base, "001-channel-core.sql"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(base, "002-channel-webhook-delivery.sql"))
	if err != nil {
		t.Fatal(err)
	}
	schema := string(first) + "\n" + string(second)
	for _, field := range []string{"event_type", "subject_ref", "sequence_no", "payload_text", "payload_hash", "lease_token", "lease_expires_at", "delivered_at", "last_error", "idx_idc_channel_webhook_due"} {
		if !strings.Contains(schema, field) {
			t.Fatalf("webhook schema is missing %s", field)
		}
	}
	embedded, err := idcchannel.EmbeddedFiles.ReadFile("manifest/sql/002-channel-webhook-delivery.sql")
	if err != nil {
		t.Fatalf("webhook delivery migration must be embedded for installation: %v", err)
	}
	if string(embedded) != string(second) {
		t.Fatal("embedded webhook delivery migration differs from its SQL source")
	}
}
