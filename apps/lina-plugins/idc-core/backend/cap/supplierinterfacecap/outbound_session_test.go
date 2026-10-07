// This file verifies the bounded, secret-free supplier session contract.

package supplierinterfacecap

import (
	"net/http"
	"testing"
)

func TestOutboundSessionContractAcceptsBoundedFormTokenPlan(t *testing.T) {
	t.Parallel()
	request := OutboundSessionRequest{
		EndpointField: "endpoint",
		Authentication: OutboundSessionAuthentication{
			Method: http.MethodPost, Path: "/login",
			SecretFormFields: map[string]string{"username": "username", "password": "password"},
			TokenSelector:    OutboundTokenRootString, TokenHeader: "Access-Token",
		},
		Steps: []OutboundSessionStep{{
			Method: http.MethodGet, Path: "/clouds/create_page",
			ResponseFields: []string{"data", "status"},
		}},
	}
	if err := ValidateOutboundSessionRequest(request); err != nil {
		t.Fatalf("valid session rejected: %v", err)
	}
	request.Steps = append(request.Steps, make([]OutboundSessionStep, MaximumOutboundSessionSteps)...)
	if err := ValidateOutboundSessionRequest(request); err == nil {
		t.Fatal("session with more than eight steps was accepted")
	}
}

func TestOutboundSessionContractRejectsSecretResponseAndAmbiguousForms(t *testing.T) {
	t.Parallel()
	base := OutboundSessionRequest{
		EndpointField: "endpoint",
		Authentication: OutboundSessionAuthentication{
			Method: http.MethodPost, Path: "/login", SecretFormFields: map[string]string{"password": "password"},
			TokenSelector: OutboundTokenField, TokenField: "access_token", TokenHeader: "Authorization",
		},
		Steps: []OutboundSessionStep{{Method: http.MethodPost, Path: "/orders", Form: map[string]string{"plan": "1"}, ResponseFields: []string{"id"}}},
	}
	if err := ValidateOutboundSessionRequest(base); err != nil {
		t.Fatalf("valid field-token plan rejected: %v", err)
	}
	base.Steps[0].ResponseFields = []string{"token"}
	if err := ValidateOutboundSessionRequest(base); err == nil {
		t.Fatal("token response field was accepted")
	}
	base.Steps[0].ResponseFields = []string{"id"}
	base.Steps[0].SecretFormFields = map[string]string{"plan": "password"}
	if err := ValidateOutboundSessionRequest(base); err == nil {
		t.Fatal("public and secret form collision was accepted")
	}
}
