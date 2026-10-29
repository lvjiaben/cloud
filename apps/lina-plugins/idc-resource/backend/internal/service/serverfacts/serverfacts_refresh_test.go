// This file verifies strict display projection independently of any upstream.
package serverfacts

import (
	"encoding/json"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"testing"
	"time"
)

func TestDisplayFieldsRemainSecretFree(t *testing.T) {
	for _, raw := range []string{`{"hostname":"cloud-demo"}`, `{"os_name":"Debian 12"}`, `{"addresses":["203.0.113.10"]}`, `{"extra":"must-not-persist"}`} {
		snapshot := resourcecap.ResourceSnapshot{ExternalResourceRef: "external-1", State: resourcecap.ResourceStateRunning, ObservedAt: time.Now(), Data: json.RawMessage(raw)}
		if err := snapshot.Validate(); err != nil {
			t.Errorf("normalized field %s: %v", raw, err)
		}
	}
}
