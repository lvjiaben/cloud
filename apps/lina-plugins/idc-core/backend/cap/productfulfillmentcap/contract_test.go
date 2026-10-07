package productfulfillmentcap

import (
	"encoding/json"
	"testing"
)

func TestValidateRequestAndResult(t *testing.T) {
	request := &Request{Version: "v1", IntentNo: "pfi_1", IdempotencyKey: "fulfillment:pfi_1",
		OrderNo: "ord_1", OrderItemNo: "item_1", UnitSequence: 3, Quantity: 3,
		ProductNo: "product_1", ProductVersionNo: "version_1", ConfigurationRevision: "revision_1",
		ProductConfiguration: json.RawMessage(`{"sku":"phone-pro"}`), ProductConfigurationDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CustomerConfigurationID: "snapshot_1", CustomerConfiguration: json.RawMessage(`{"model":"phone"}`)}
	if err := ValidateRequest(request); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	request.UnitSequence = 4
	if err := ValidateRequest(request); err == nil {
		t.Fatal("unit outside the frozen quantity was accepted")
	}
	request.UnitSequence = 3
	request.PreviousExternalReference = "supplier/reference"
	if err := ValidateRequest(request); err == nil {
		t.Fatal("unsafe previous external reference was accepted")
	}
	request.PreviousExternalReference = "supplier-order:1001"
	request.ProductConfiguration = json.RawMessage(`[]`)
	if err := ValidateRequest(request); err == nil {
		t.Fatal("non-object merchant product configuration was accepted")
	}
	if err := ValidateResult(ActionSubmit, &Result{Version: "v1", Status: StatusAccepted}); err == nil {
		t.Fatal("accepted result without external reference was accepted")
	}
	if err := ValidateResult(ActionSubmit, &Result{Version: "v1", Status: StatusNotAccepted}); err != nil {
		t.Fatalf("explicit non-acceptance was rejected: %v", err)
	}
	if err := ValidateResult(ActionQuery, &Result{Version: "v1", Status: StatusNotAccepted}); err == nil {
		t.Fatal("submit-only non-acceptance was accepted from a query")
	}
}
