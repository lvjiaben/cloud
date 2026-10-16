// These tests prove malformed or oversized GET selections fail before service
// calls and public delivery times remain JSON numbers.
package channel

import (
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"strings"
	"testing"
	"time"
)

func TestConfigurationQueryRejectsMalformedAndOversizedValues(t *testing.T) {
	for _, raw := range []string{"null", "{}", "[", strings.Repeat(" ", 32769), "[" + strings.Repeat("{},", 128) + "{}]"} {
		if _, err := configurationQueryValues(raw); err == nil {
			t.Fatalf("accepted malformed query: %d bytes", len(raw))
		}
	}
	for _, raw := range []string{"", "[]", `[{"fieldToken":"field","textValue":"hello & 中文"}]`} {
		if _, err := configurationQueryValues(raw); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDeliveryProjectionUsesUnixMilliseconds(t *testing.T) {
	when := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	result := projectChannelDeliveries(&channelcommercecap.OrderDeliveries{OrderNo: "order", Items: []channelcommercecap.OrderDelivery{{StartedAt: &when, Services: []channelcommercecap.DeliveryService{{StartedAt: &when, ExpiresAt: &when}}}}})
	raw, err := json.Marshal(result)
	if err != nil || strings.Contains(string(raw), "2026-") || !strings.Contains(string(raw), `"startedAt":1791676800000`) || !strings.Contains(string(raw), `"completedAt":null`) {
		t.Fatalf("time contract: %s %v", raw, err)
	}
}
