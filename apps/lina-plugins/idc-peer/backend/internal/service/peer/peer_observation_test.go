// These tests ensure a sandbox without wall time consumes authenticated
// upstream observations instead of manufacturing a local timestamp.
package peer

import (
	"encoding/json"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestResourceObservationUsesAuthenticatedServerTime(t *testing.T) {
	for _, method := range []string{resourcecap.DynamicProviderMethodVerifyConnection, resourcecap.DynamicProviderMethodDiscoverCapabilities} {
		e := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"genericApi":true,"observedAt":1791676800123}}`}}}
		raw, err := ExecuteResource(e, "confidential-context", method, nil)
		if err != nil {
			t.Fatal(err)
		}
		var result resourcecap.ConnectionVerification
		if json.Unmarshal(raw, &result) != nil || result.ObservedAt.UnixMilli() != 1791676800123 || len(e.requests) != 1 || !e.requests[0].UseAuthentication {
			t.Fatalf("lost authenticated observation: %s", raw)
		}
	}
}

func TestResourceObservationRejectsMissingInvalidOrFailedClock(t *testing.T) {
	for _, response := range []string{`{"code":0,"data":{"genericApi":true}}`, `{"code":0,"data":{"genericApi":true,"observedAt":0}}`, `{"code":0,"data":{"genericApi":true,"observedAt":-1}}`, `{"code":0,"data":{"genericApi":true,"observedAt":253402300800000}}`, `{"code":1,"data":{"genericApi":true,"observedAt":1791676800123}}`} {
		e := &scriptedExecutor{steps: []step{{response: response}}}
		if raw, err := ExecuteResource(e, "confidential-context", resourcecap.DynamicProviderMethodVerifyConnection, nil); err == nil || raw != nil {
			t.Fatalf("invalid observation accepted: %s", response)
		}
	}
}
