package supplierinterfacecap

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestOutboundRequestAcceptsSecretFieldBindingsOnly(t *testing.T) {
	request := OutboundHTTPRequest{
		Method: http.MethodPost, Path: "/payment/balance", EndpointField: "endpoint",
		Body:           json.RawMessage(`{"product_id":"ssl-dv-1"}`),
		SecretHeaders:  map[string]string{"X-API-Key": "api_key", "X-API-Secret": "api_secret"},
		ResponseFields: []string{"success", "order_id"},
	}
	if err := ValidateOutboundHTTPRequest(request); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(request)
	for _, forbidden := range []string{"plain-key", "plain-secret", "https://supplier.example"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("outbound request exposed %q", forbidden)
		}
	}
}

func TestOutboundRequestRejectsUnsafePathsAndResponseFields(t *testing.T) {
	base := OutboundHTTPRequest{Method: http.MethodPost, Path: "/order/sync", EndpointField: "endpoint", Body: json.RawMessage(`{}`), ResponseFields: []string{"success"}}
	for name, mutate := range map[string]func(*OutboundHTTPRequest){
		"absolute path":  func(request *OutboundHTTPRequest) { request.Path = "https://evil.invalid/order" },
		"traversal":      func(request *OutboundHTTPRequest) { request.Path = "/order/../secret" },
		"token response": func(request *OutboundHTTPRequest) { request.ResponseFields = []string{"access_token"} },
		"header injection": func(request *OutboundHTTPRequest) {
			request.SecretHeaders = map[string]string{"X-Key\r\nInjected": "api_key"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := base
			mutate(&request)
			if ValidateOutboundHTTPRequest(request) == nil {
				t.Fatal("unsafe outbound request was accepted")
			}
		})
	}
}
