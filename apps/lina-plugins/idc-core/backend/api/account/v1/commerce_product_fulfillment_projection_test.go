package v1

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCommerceDeliveryContainsOnlyCustomerSafeFields(t *testing.T) {
	raw, err := json.Marshal(&CommerceDelivery{
		Quantity: 2, OngoingService: true,
		ProductNameZhCN: "证书", ProductNameEnUS: "Certificate",
		StatusCode: "processing", StartedAt: 1_787_999_000_000, UpdatedAt: 1_788_000_000_000,
	})
	if err != nil {
		t.Fatalf("marshal customer fulfillment projection: %v", err)
	}
	encoded := string(raw)
	for _, forbidden := range []string{
		"taskNo", "intentNo", "plugin", "provider", "supplier", "external",
		"evidence", "configuration", "secret", "error", "reasonCode",
		"unitSequence", "granularityCode", "orderItemNo", "deliveryNo",
	} {
		if strings.Contains(strings.ToLower(encoded), strings.ToLower(forbidden)) {
			t.Fatalf("customer fulfillment projection exposed %q: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(encoded, `"quantity":2`) || !strings.Contains(encoded, `"ongoingService":true`) {
		t.Fatalf("customer delivery projection lost safe product facts: %s", encoded)
	}
}
