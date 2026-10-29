// This file checks standalone public bridge compatibility and fail-closed parsing.
package descriptor

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestHandle checks valid descriptor dispatch plus forged and malformed calls.
func TestHandle(t *testing.T) {
	in := invocation{Version: "v1", Owner: "idc-portal", Provider: "test-template", Point: "website-template", Contract: "v1", Method: "capabilities.describe", Release: "v1.0.0", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), Purpose: "new-business", Runtime: &struct {
		ContextID string `json:"contextId"`
	}{"opaque"}, Payload: json.RawMessage(`{}`)}
	encode := func(value invocation) []byte {
		t.Helper()
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return appendBytes(appendBytes(appendBytes(nil, 1, []byte("test-template")), 2, appendBytes(nil, 7, []byte("BusinessExtensionBridgeRequest"))), 3, appendBytes(nil, 13, body))
	}
	result := Handle(encode(in), "test-template", "idc-portal", "website-template")
	if !bytes.Contains(result, []byte(`"pluginId":"test-template"`)) {
		t.Fatalf("missing descriptor: %s", result)
	}
	for _, change := range []func(*invocation){func(v *invocation) { v.Owner = "other" }, func(v *invocation) { v.Runtime = nil }, func(v *invocation) { v.Method = "delete" }, func(v *invocation) { v.Purpose = "unknown" }, func(v *invocation) { v.Provider = "forged" }, func(v *invocation) { v.ReleaseDigest = "" }} {
		bad := in
		change(&bad)
		if bytes.Contains(Handle(encode(bad), "test-template", "idc-portal", "website-template"), []byte(`"pluginId"`)) {
			t.Fatal("invalid invocation accepted")
		}
	}
	for _, wire := range [][]byte{{0}, {0x1a, 0xff}, {0x0b}, append(encode(in), appendBytes(nil, 1, []byte("test-template"))...)} {
		if bytes.Contains(Handle(wire, "test-template", "idc-portal", "website-template"), []byte(`"pluginId"`)) {
			t.Fatal("invalid wire accepted")
		}
	}
}
