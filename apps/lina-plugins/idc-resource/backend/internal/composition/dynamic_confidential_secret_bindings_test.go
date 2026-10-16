// This file verifies governed slot normalization fails closed before HTTP or
// secret access, retaining the existing public-form and operation-slot guards.

package composition

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"testing"
)

func TestNormalizeConfidentialSecretFormBindings(t *testing.T) {
	for _, test := range []struct {
		name    string
		request resourcecap.ConfidentialHTTPRequest
		invalid bool
	}{
		{name: "none"},
		{name: "legacy owner map", request: resourcecap.ConfidentialHTTPRequest{SecretFormFields: map[string]string{"password": "password"}}},
		{name: "wire array", request: resourcecap.ConfidentialHTTPRequest{SecretFormBindings: []resourcecap.ConfidentialSecretFormBinding{{Field: "password", Slot: "password"}}}},
		{name: "mixed", request: resourcecap.ConfidentialHTTPRequest{SecretFormFields: map[string]string{"rootpass": "password"}, SecretFormBindings: []resourcecap.ConfidentialSecretFormBinding{{Field: "password", Slot: "password"}}}, invalid: true},
		{name: "duplicate", request: resourcecap.ConfidentialHTTPRequest{SecretFormBindings: []resourcecap.ConfidentialSecretFormBinding{{Field: "password", Slot: "password"}, {Field: "password", Slot: "other"}}}, invalid: true},
		{name: "empty field", request: resourcecap.ConfidentialHTTPRequest{SecretFormBindings: []resourcecap.ConfidentialSecretFormBinding{{Slot: "password"}}}, invalid: true},
		{name: "empty slot", request: resourcecap.ConfidentialHTTPRequest{SecretFormBindings: []resourcecap.ConfidentialSecretFormBinding{{Field: "password"}}}, invalid: true},
		{name: "whitespace", request: resourcecap.ConfidentialHTTPRequest{SecretFormBindings: []resourcecap.ConfidentialSecretFormBinding{{Field: " password", Slot: "password"}}}, invalid: true},
		{name: "over limit", request: resourcecap.ConfidentialHTTPRequest{SecretFormBindings: make([]resourcecap.ConfidentialSecretFormBinding, 9)}, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := test.request.SecretFormFields
			err := normalizeConfidentialSecretFormBindings(&test.request)
			if (err != nil) != test.invalid {
				t.Fatalf("invalid=%t err=%v", test.invalid, err)
			}
			if test.invalid && len(test.request.SecretFormFields) != len(before) {
				t.Fatal("invalid input partially mutated")
			}
			if test.name == "wire array" && (test.request.SecretFormFields["password"] != "password" || len(test.request.SecretFormBindings) != 0) {
				t.Fatal("wire binding did not normalize")
			}
		})
	}
}

func TestNormalizedSecretBindingsRetainHTTPGuards(t *testing.T) {
	for _, test := range []struct {
		name    string
		request resourcecap.ConfidentialHTTPRequest
	}{
		{name: "authentication", request: resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: "POST", Path: "/login"}},
		{name: "public form conflict", request: resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: "POST", Path: "/clouds", UseAuthentication: true, ResponseFields: []string{"data"}, Form: map[string]string{"password": "public"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.request.SecretFormBindings = []resourcecap.ConfidentialSecretFormBinding{{Field: "password", Slot: "password"}}
			if err := normalizeConfidentialSecretFormBindings(&test.request); err != nil {
				t.Fatal(err)
			}
			if err := validateConfidentialHTTPRequest(test.request); err == nil {
				t.Fatal("secret-binding instruction bypassed existing HTTP guard")
			}
		})
	}
}

func TestNormalizedSecretBindingsCannotResolveUnknownOperationSlot(t *testing.T) {
	request := resourcecap.ConfidentialHTTPRequest{SecretFormBindings: []resourcecap.ConfidentialSecretFormBinding{{Field: "password", Slot: "not-granted"}}}
	if err := normalizeConfidentialSecretFormBindings(&request); err != nil {
		t.Fatal(err)
	}
	handler := &confidentialHTTPHandler{}
	result, err := handler.performBusinessWithOperationSecrets(context.Background(), request, nil)
	if result != nil || !bizerr.Is(err, resourcecap.CodeProviderLocalAuthPolicy) {
		t.Fatalf("ungranted operation slot was not rejected: result=%#v err=%v", result, err)
	}
	// An invalid array also fails Execute before missing HTTP or secret services
	// could be consulted; no supplier request is possible.
	resultEnvelope, err := handler.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: []byte(`{"kind":"request","method":"PUT","path":"/clouds/41/password","secretFormBindings":[{"field":"password","slot":""}]}`)})
	if err == nil || resultEnvelope != nil {
		t.Fatal("invalid array reached confidential execution")
	}
}

func TestUnknownWireSlotRejectedBeforeAuthenticationOrSecretAccess(t *testing.T) {
	handler := &confidentialHTTPHandler{}
	result, err := handler.Execute(context.Background(), &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: []byte(`{"kind":"request","method":"PUT","path":"/clouds/41/password","useAuthentication":true,"responseFields":["data"],"secretFormBindings":[{"field":"password","slot":"not-granted"}]}`)})
	if result != nil || !bizerr.Is(err, resourcecap.CodeProviderLocalAuthPolicy) {
		t.Fatalf("unknown slot did not fail before missing auth services: result=%#v err=%v", result, err)
	}
}
