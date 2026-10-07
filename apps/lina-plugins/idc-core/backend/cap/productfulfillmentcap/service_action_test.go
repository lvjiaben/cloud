// This file verifies the uniform long-lived service action JSON envelope.

package productfulfillmentcap

import (
	"encoding/json"
	"testing"
)

func TestManagedServiceActionEnvelopeRejectsCustomerIdentityAndMissingIdempotency(t *testing.T) {
	request := &ManagedServiceActionRequest{
		Version: ManagedServiceActionVersion, ActionKey: "cancel", EntitlementNo: "ent_10001",
		IdempotencyKey: "act_10001", ExternalReference: "YS-10001",
		ProductConfiguration: json.RawMessage(`{"plan":"ssl-dv"}`), Input: json.RawMessage(`{}`),
	}
	if err := ValidateManagedServiceActionRequest(request, "cancel", true); err != nil {
		t.Fatalf("valid mutation envelope was rejected: %v", err)
	}
	request.IdempotencyKey = ""
	if err := ValidateManagedServiceActionRequest(request, "cancel", true); err == nil {
		t.Fatal("mutation without idempotency evidence was accepted")
	}
	var decoded map[string]json.RawMessage
	content, _ := json.Marshal(request)
	if json.Unmarshal(content, &decoded) != nil || decoded["customerNo"] != nil {
		t.Fatalf("customer identity leaked into provider envelope: %s", content)
	}
}

func TestManagedServiceActionResultAllowsJSONProjectionOnly(t *testing.T) {
	result := &ManagedServiceActionResult{
		Version: ManagedServiceActionVersion, ActionKey: "detail", Status: ManagedServiceActionStatusProjected,
		ExternalReference: "YS-10001", Projection: json.RawMessage(`{"status":"pending"}`),
	}
	if err := ValidateManagedServiceActionResult(result, "detail", true); err != nil {
		t.Fatalf("valid projection was rejected: %v", err)
	}
	result.Projection = json.RawMessage(`"binary-certificate"`)
	if err := ValidateManagedServiceActionResult(result, "detail", true); err == nil {
		t.Fatal("non-object projection was accepted")
	}
}
