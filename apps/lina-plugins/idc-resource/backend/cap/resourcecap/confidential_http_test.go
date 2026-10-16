// This file verifies field-name values remain safe across governed JSON while
// preserving the exact deterministic upstream-to-slot mapping.

package resourcecap

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestConfidentialSecretFormBindingsEncodeSensitiveNamesAsValues(t *testing.T) {
	fields := map[string]string{"password": "password", "rootpass": "root_password"}
	bindings := NewConfidentialSecretFormBindings(fields)
	want := []ConfidentialSecretFormBinding{{Field: "password", Slot: "password"}, {Field: "rootpass", Slot: "root_password"}}
	if !reflect.DeepEqual(bindings, want) {
		t.Fatalf("unexpected bindings: %#v", bindings)
	}
	payload, err := json.Marshal(ConfidentialHTTPRequest{SecretFormBindings: bindings})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), `"password":`) || strings.Contains(string(payload), `secretFormFields`) || !strings.Contains(string(payload), `"field":"password","slot":"password"`) {
		t.Fatalf("unsafe or incomplete wire projection: %s", payload)
	}
	if NewConfidentialSecretFormBindings(nil) != nil {
		t.Fatal("nil fields must remain absent")
	}
}
