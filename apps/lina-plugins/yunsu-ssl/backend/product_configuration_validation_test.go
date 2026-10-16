// This file verifies unsupported product publication is rejected without any
// supplier request, including forged capability claims and malformed envelopes.

package backend

import (
	"encoding/json"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

func TestProductPublicationRequiresDocumentedSupplierCapabilities(t *testing.T) {
	for _, configuration := range []string{`{}`, `{"plan_id":"ssl-dv-1","certificate_scope":"wildcard","maximum_domains":100}`, `{"plan_id":"ysp1.fake","certificate_scope":"ip"}`} {
		t.Run(configuration, func(t *testing.T) {
			interfaces := &supplierInterfaceStub{}
			payload, err := json.Marshal(productfulfillmentcap.ConfigurationValidationRequest{
				Version: productfulfillmentcap.ContractVersion, ProductConfiguration: json.RawMessage(configuration),
			})
			if err != nil {
				t.Fatal(err)
			}
			response, err := NewGuestHandler(interfaces)(extensionEnvelope(t, productfulfillmentcap.MethodValidateProductConfiguration,
				capmodel.BusinessExtensionInvocationPurposeNewBusiness, payload))
			if err != nil || response.StatusCode != 200 || len(interfaces.requests) != 0 {
				t.Fatalf("publication validation made a supplier call or failed: %#v %v", response, err)
			}
			var bridge capmodel.BusinessExtensionBridgeResponse
			var result productfulfillmentcap.ConfigurationValidationResult
			if json.Unmarshal(response.Body, &bridge) != nil || json.Unmarshal(bridge.Payload, &result) != nil || result.Version != productfulfillmentcap.ContractVersion || result.Valid || result.Reason != productfulfillmentcap.ConfigurationValidationSupplierCapabilitiesUnconfirmed {
				t.Fatalf("undocumented plan accepted: %s", response.Body)
			}
		})
	}
}

func TestProductPublicationRejectsMalformedValidationRequests(t *testing.T) {
	for _, payload := range []string{`{}`, `{"version":"v2","productConfiguration":{}}`, `{"version":"v1"}`, `{"version":"v1","productConfiguration":{},"capabilitiesConfirmed":true}`, `{"version":"v1","productConfiguration":{}} {}`} {
		if _, err := executeProductConfigurationValidation(json.RawMessage(payload)); err == nil {
			t.Fatalf("malformed request accepted: %s", payload)
		}
	}
}
