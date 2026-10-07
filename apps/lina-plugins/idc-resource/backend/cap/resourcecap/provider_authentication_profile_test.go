package resourcecap

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestAuthenticationProfileCompilesWithoutCrossingPublicOrGuestBoundary(t *testing.T) {
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "protocol", Type: capmodel.ConfigurationFieldSingleSelect, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "legacy"}, {Value: "bms"}}},
		{Key: "username", Type: capmodel.ConfigurationFieldSecret}, {Key: "password", Type: capmodel.ConfigurationFieldSecret},
		{Key: "access_user", Type: capmodel.ConfigurationFieldSecret}, {Key: "access_token", Type: capmodel.ConfigurationFieldSecret},
	}}
	metadata := &capmodel.BusinessExtensionProviderMetadata{Type: DynamicProviderMetadataType, Version: "v1", Values: map[string]any{
		"providerId": "dcim", "interfaceSettingsProjection": []any{"protocol"},
		"authenticationProfiles": map[string]any{"selectorField": "protocol", "profiles": map[string]any{
			"legacy": map[string]any{"strategy": "per_request_form", "fields": map[string]any{"username": "username", "password": "password"}},
			"bms":    map[string]any{"strategy": "per_request_headers", "fields": map[string]any{"access-user": "access_user", "access-token": "access_token"}},
		}},
	}}
	if err := ValidateProviderMetadata(metadata, schema); err != nil {
		t.Fatalf("validate profiles: %v", err)
	}
	compiled, err := CompileInterfaceSettings(json.RawMessage(`{"protocol":"bms"}`), metadata)
	if err != nil {
		t.Fatalf("compile profile: %v", err)
	}
	if err = (ProviderConfig{ChannelRef: "channel-test", Endpoint: "https://provider.example.test", Settings: compiled}).Validate(); err != nil {
		t.Fatalf("compiled host policy did not remain secret-free: %v", err)
	}
	if !strings.Contains(string(compiled), "per_request_headers") || !strings.Contains(string(compiled), "access-token") {
		t.Fatalf("host profile was not compiled: %s", compiled)
	}
	for _, guest := range []bool{false, true} {
		projected, projectErr := ProjectPublicInterfaceSettings(compiled, guest)
		if projectErr != nil || string(projected) != `{"protocol":"bms"}` || strings.Contains(string(projected), "access-token") {
			t.Fatalf("unsafe settings projection guest=%v payload=%s err=%v", guest, projected, projectErr)
		}
	}
	if _, err = CompileInterfaceSettings(json.RawMessage(`{"protocol":"bms","authStrategy":"per_request_headers"}`), metadata); err == nil {
		t.Fatal("administrator-supplied authentication policy was accepted")
	}
}

func TestAuthenticationProfilesRejectUndeclaredOrUnsafeMappings(t *testing.T) {
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "username", Type: capmodel.ConfigurationFieldSecret}}}
	for _, profile := range []map[string]any{
		{"strategy": "per_request_headers", "fields": map[string]any{"Authorization": "username"}},
		{"strategy": "login_session", "fields": map[string]any{"username": "username"}, "loginPath": "https://evil.invalid/login", "tokenJSONPath": []any{"jwt"}, "tokenHeader": "Authorization", "sessionTTLSeconds": 3600},
	} {
		metadata := &capmodel.BusinessExtensionProviderMetadata{Type: DynamicProviderMetadataType, Version: "v1", Values: map[string]any{
			"providerId": "unsafe", "authenticationProfiles": map[string]any{"profiles": map[string]any{"default": profile}},
		}}
		if err := ValidateProviderMetadata(metadata, schema); err == nil {
			t.Fatalf("unsafe authentication profile was accepted: %#v", profile)
		}
	}
}

func TestBTPanelAuthenticationProfileIsHostOnlyAndExact(t *testing.T) {
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "endpoint", Type: capmodel.ConfigurationFieldSingleLine},
		{Key: "api_key", Type: capmodel.ConfigurationFieldSecret},
	}}
	metadata := &capmodel.BusinessExtensionProviderMetadata{Type: DynamicProviderMetadataType, Version: "v1", Values: map[string]any{
		"providerId": "bt-panel", "authenticationProfiles": map[string]any{"profiles": map[string]any{"default": map[string]any{
			"strategy": "bt_panel_double_md5", "fields": map[string]any{"material": "api_key"},
		}}},
	}}
	if err := ValidateProviderMetadata(metadata, schema); err != nil {
		t.Fatalf("validate BT Panel profile: %v", err)
	}
	compiled, err := CompileInterfaceSettings(json.RawMessage(`{"endpoint":"https://panel.example.test"}`), metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err = (ProviderConfig{ChannelRef: "channel-test", Endpoint: "https://panel.example.test", Settings: compiled}).Validate(); err != nil {
		t.Fatalf("compiled BT Panel host policy did not remain secret-free: %v", err)
	}
	guest, err := ProjectPublicInterfaceSettings(compiled, true)
	if err != nil || string(guest) != `{}` || strings.Contains(string(guest), "api_key") || strings.Contains(string(guest), "bt_panel") {
		t.Fatalf("BT Panel host policy crossed guest boundary: %s err=%v", guest, err)
	}
	for _, fields := range []map[string]any{
		{"apiKey": "api_key"},
		{"material": "endpoint"},
		{"material": "api_key", "password": "api_key"},
	} {
		metadata.Values["authenticationProfiles"] = map[string]any{"profiles": map[string]any{"default": map[string]any{
			"strategy": "bt_panel_double_md5", "fields": fields,
		}}}
		if err := ValidateProviderMetadata(metadata, schema); err == nil {
			t.Fatalf("unsafe BT Panel mapping was accepted: %#v", fields)
		}
	}
}

func TestProviderCorrelationAcceptsGovernedHTTPSCallback(t *testing.T) {
	value := &ProviderCorrelation{DownstreamID: 1, DownstreamURL: "https://platform.example.test/api/plugins/idc-resource/provider-callback"}
	if err := ValidateProviderCorrelation(value); err != nil {
		t.Fatalf("governed callback rejected: %v", err)
	}
}
