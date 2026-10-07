package v1

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCommerceQuoteCreateDoesNotAcceptClientConfigurationState(t *testing.T) {
	var request CommerceQuoteCreateReq
	if err := json.Unmarshal([]byte(`{
  "expectedCartVersion": 7,
  "configurationFrozen": true,
  "configurationRevision": "client-forged"
}`), &request); err != nil {
		t.Fatalf("decode quote request: %v", err)
	}
	if request.ExpectedCartVersion != 7 {
		t.Fatalf("expected cart version=%d", request.ExpectedCartVersion)
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("encode quote request: %v", err)
	}
	if strings.Contains(string(encoded), "configurationFrozen") ||
		strings.Contains(string(encoded), "configurationRevision") {
		t.Fatalf("client configuration authority leaked into request DTO: %s", encoded)
	}
}
