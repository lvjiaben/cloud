// Guest wall clocks can be deterministic epochs. Shape validation must not
// reject an otherwise valid read before the trusted owner stamps its receipt.
package backend

import (
	"encoding/json"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"testing"
	"time"
)

func TestResourceResultShapeDoesNotTrustGuestWallClock(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()
	evidence := []resourcecap.Evidence{{Kind: "upstream.read", Reference: "41", ObservedAt: epoch}}
	resource := resourcecap.ResourceSnapshot{ExternalResourceRef: "41", State: resourcecap.ResourceStateActive, Data: json.RawMessage(`{"addresses":["203.0.113.9"]}`), ObservedAt: epoch, Evidence: evidence}
	request := resourcecap.ProviderCapacityRequest{Kind: resourcecap.ProviderCapacityKindLine, Limit: 5, Filters: json.RawMessage(`{}`)}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, method string
		value        any
		request      json.RawMessage
	}{
		{"capacity", resourcecap.DynamicProviderMethodListProviderCapacity, resourcecap.ProviderCapacityPage{Items: []resourcecap.ProviderCapacityItem{{Kind: request.Kind, ExternalRef: "5", Data: json.RawMessage(`{"id":5,"name":"Node","area_id":2}`), ObservedAt: epoch}}, Complete: true, Evidence: evidence}, requestJSON},
		{"connection", resourcecap.DynamicProviderMethodVerifyConnection, resourcecap.ConnectionVerification{Verified: true, ObservedAt: epoch}, json.RawMessage(`{}`)},
		{"resource", resourcecap.DynamicProviderMethodQueryResource, resource, json.RawMessage(`{}`)},
		{"task", resourcecap.DynamicProviderMethodQueryTask, resourcecap.OperationResult{Status: resourcecap.TaskStatusSucceeded, ExternalTaskRef: "9", ExternalResourceRef: "41", Resource: &resource, Evidence: evidence}, json.RawMessage(`{}`)},
		{"definite rejection", resourcecap.DynamicProviderMethodExecute, resourcecap.OperationResult{Status: resourcecap.TaskStatusFailed, Failure: &resourcecap.ProviderFailure{Code: "upstream_request_rejected", Message: "Supplier rejected the request"}, Evidence: evidence}, json.RawMessage(`{}`)},
		{"lookup", resourcecap.DynamicProviderMethodLookupPrincipal, resourcecap.PrincipalLookupResult{Exists: true, ExternalPrincipalRef: "66", ObservedAt: epoch, Evidence: evidence}, json.RawMessage(`{}`)},
		{"principal", resourcecap.DynamicProviderMethodCreatePrincipal, resourcecap.PrincipalCreateResult{ExternalPrincipalRef: "66", ObservedAt: epoch, Evidence: evidence}, json.RawMessage(`{}`)},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload, marshalErr := json.Marshal(test.value)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			before := string(payload)
			if validateErr := validateResourceProviderResult(test.method, payload, test.request); validateErr != nil {
				t.Fatalf("valid shape rejected by untrusted guest clock: %v", validateErr)
			}
			if string(payload) != before {
				t.Fatal("shape validation rewrote the returned observation")
			}
		})
	}
}

func TestUnclockedGuestStillRejectsInvalidCapacityAndResourceFacts(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()
	request := resourcecap.ProviderCapacityRequest{Kind: resourcecap.ProviderCapacityKindLine, Limit: 1, Filters: json.RawMessage(`{}`)}
	rawRequest, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	invalid := resourcecap.ProviderCapacityPage{Items: []resourcecap.ProviderCapacityItem{{Kind: resourcecap.ProviderCapacityKindImage, ExternalRef: "5", Data: json.RawMessage(`{"id":5}`), ObservedAt: epoch}}, Complete: true}
	payload, err := json.Marshal(invalid)
	if err != nil {
		t.Fatal(err)
	}
	if validateResourceProviderResult(resourcecap.DynamicProviderMethodListProviderCapacity, payload, rawRequest) == nil {
		t.Fatal("capacity item changed its requested kind")
	}
	bad := resourcecap.ResourceSnapshot{ExternalResourceRef: "41", State: "invented_state", Data: json.RawMessage(`{}`), ObservedAt: epoch}
	payload, err = json.Marshal(bad)
	if err != nil {
		t.Fatal(err)
	}
	if validateResourceProviderResult(resourcecap.DynamicProviderMethodQueryResource, payload, nil) == nil {
		t.Fatal("resource shape validation accepted an invented state")
	}
}
