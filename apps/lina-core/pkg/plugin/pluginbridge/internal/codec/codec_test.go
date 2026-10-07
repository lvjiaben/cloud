// This file tests bridge spec validation and request-response codec round trips.

package codec

import (
	"bytes"
	"net/http"
	"testing"
)

// TestValidateBridgeSpecRejectsTextCodec verifies executable bridge specs only
// accept protobuf request and response codecs.
func TestValidateBridgeSpecRejectsTextCodec(t *testing.T) {
	spec := &BridgeSpec{
		ABIVersion:     ABIVersionV1,
		RuntimeKind:    RuntimeKindWasm,
		RouteExecution: true,
		RequestCodec:   "json",
		ResponseCodec:  "protobuf",
	}
	if err := ValidateBridgeSpec(spec); err == nil {
		t.Fatal("expected text request codec to be rejected")
	}
}

// TestValidateRouteContractsRejectsInvalidPublicPermission verifies public
// routes cannot retain permission requirements.
func TestValidateRouteContractsRejectsInvalidPublicPermission(t *testing.T) {
	routes := []*RouteContract{
		{
			Path:        "/api/v1/review-summary",
			Method:      http.MethodGet,
			Access:      AccessPublic,
			Permission:  "demo-dynamic:review:view",
			RequestType: "ReviewSummaryReq",
		},
	}
	if err := ValidateRouteContracts("demo-dynamic", routes); err == nil {
		t.Fatal("expected public route with permission to be rejected")
	}
}

// TestValidateRouteContractsRejectsDuplicateMethodPath verifies route method
// and path remain the unique dynamic route identity inside one plugin.
func TestValidateRouteContractsRejectsDuplicateMethodPath(t *testing.T) {
	routes := []*RouteContract{
		{
			Path:        "/api/v1/review-summary",
			Method:      http.MethodGet,
			Access:      AccessLogin,
			RequestType: "ReviewSummaryReq",
		},
		{
			Path:        "/api/v1/review-summary",
			Method:      http.MethodGet,
			Access:      AccessLogin,
			RequestType: "ReviewSummaryReq",
		},
	}
	if err := ValidateRouteContracts("demo-dynamic", routes); err == nil {
		t.Fatal("expected duplicate method and path to be rejected")
	}
}

// TestValidateRouteContractsAllowsPluginOwnedPathShapes verifies dynamic route
// contracts do not force plugin-local paths to use a host API version prefix.
func TestValidateRouteContractsAllowsPluginOwnedPathShapes(t *testing.T) {
	routes := []*RouteContract{
		{
			Path:        "/api/v2/review-summary",
			Method:      http.MethodGet,
			Access:      AccessLogin,
			RequestType: "ReviewSummaryV2Req",
		},
		{
			Path:        "/interface/m1/review-summary",
			Method:      http.MethodPost,
			Access:      AccessLogin,
			RequestType: "InterfaceReviewSummaryReq",
		},
		{
			Path:        "/graphql",
			Method:      http.MethodPost,
			Access:      AccessPublic,
			RequestType: "GraphQLReq",
		},
	}
	if err := ValidateRouteContracts("demo-dynamic", routes); err != nil {
		t.Fatalf("expected plugin-owned route path shapes to be accepted, got error: %v", err)
	}
}

// TestEncodeDecodeRequestEnvelopeRoundTrip verifies the manual protobuf codec
// preserves nested route, request, and identity snapshots.
func TestEncodeDecodeRequestEnvelopeRoundTrip(t *testing.T) {
	input := &BridgeRequestEnvelopeV1{
		PluginID: "demo-dynamic",
		Route: &RouteMatchSnapshotV1{
			Method:       http.MethodGet,
			PublicPath:   "/x/demo-dynamic/api/v1/review-summary",
			InternalPath: "/api/v1/review-summary",
			RoutePath:    "/api/v1/review-summary",
			Access:       AccessLogin,
			Permission:   "demo-dynamic:review:view",
			RequestType:  "ReviewSummaryReq",
			PathParams: map[string]string{
				"id": "42",
			},
			QueryValues: map[string][]string{
				"q": {"hello"},
			},
		},
		Request: &HTTPRequestSnapshotV1{
			Method:       http.MethodGet,
			PublicPath:   "/x/demo-dynamic/api/v1/review-summary",
			InternalPath: "/api/v1/review-summary",
			RawPath:      "/x/demo-dynamic/api/v1/review-summary",
			RawQuery:     "q=hello",
			Host:         "localhost:9120",
			Scheme:       "http",
			ClientIP:     "127.0.0.1",
			Headers: map[string][]string{
				"Accept": {"application/json"},
			},
			Cookies: map[string]string{
				"lang": "zh-CN",
			},
			Body: []byte(`{"hello":"world"}`),
		},
		Identity: &IdentitySnapshotV1{
			TokenID:         "token-1",
			TenantId:        22,
			UserID:          1,
			Username:        "admin",
			Status:          1,
			ActingUserId:    7,
			ActingAsTenant:  true,
			IsImpersonation: true,
			Permissions:     []string{"demo-dynamic:review:view"},
			RoleNames:       []string{"超级管理员"},
			DataScope:       1,
			IsSuperAdmin:    true,
		},
		RequestID: "req-1",
	}

	content, err := EncodeRequestEnvelope(input)
	if err != nil {
		t.Fatalf("expected request encode to succeed, got error: %v", err)
	}
	output, err := DecodeRequestEnvelope(content)
	if err != nil {
		t.Fatalf("expected request decode to succeed, got error: %v", err)
	}
	if output.PluginID != input.PluginID || output.RequestID != input.RequestID {
		t.Fatalf("unexpected request identity fields: %#v", output)
	}
	if output.Route == nil || output.Route.Permission != input.Route.Permission {
		t.Fatalf("unexpected route snapshot: %#v", output.Route)
	}
	if output.Request == nil || output.Request.RawQuery != input.Request.RawQuery {
		t.Fatalf("unexpected request snapshot: %#v", output.Request)
	}
	if output.Identity == nil || !output.Identity.IsSuperAdmin {
		t.Fatalf("unexpected identity snapshot: %#v", output.Identity)
	}
	if output.Identity.DataScope != input.Identity.DataScope {
		t.Fatalf("unexpected identity data scope: %#v", output.Identity)
	}
	if output.Identity.TenantId != input.Identity.TenantId ||
		output.Identity.ActingUserId != input.Identity.ActingUserId ||
		output.Identity.ActingAsTenant != input.Identity.ActingAsTenant ||
		output.Identity.IsImpersonation != input.Identity.IsImpersonation {
		t.Fatalf("unexpected tenant impersonation snapshot: %#v", output.Identity)
	}
	if !bytes.Equal(output.Request.Body, input.Request.Body) {
		t.Fatalf("unexpected request body: %q", string(output.Request.Body))
	}
}
