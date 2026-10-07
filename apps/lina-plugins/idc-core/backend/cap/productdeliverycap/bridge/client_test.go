package bridge

import (
	"context"
	"encoding/json"
	"testing"

	"lina-plugin-idc-core/backend/cap/productdeliverycap"
)

func TestGuestClientUsesExactOwnerServiceWithoutCustomerIdentity(t *testing.T) {
	client, err := NewClient(func(owner, service, version, method, resourceRef, table string, payload []byte) ([]byte, error) {
		if owner != productdeliverycap.OwnerPluginID || service != productdeliverycap.ServiceName ||
			version != productdeliverycap.ContractVersion || method != productdeliverycap.MethodCurrentCustomerDeliveriesList ||
			resourceRef != "" || table != "" {
			t.Fatalf("unexpected capability route: %s %s %s %s", owner, service, version, method)
		}
		var request map[string]any
		if json.Unmarshal(payload, &request) != nil || request["customerNo"] != nil || request["pluginId"] != nil {
			t.Fatalf("payload accepted caller-controlled identity: %s", payload)
		}
		return json.Marshal(&productdeliverycap.CurrentCustomerDeliveryPage{})
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.ListCurrentCustomerDeliveries(context.Background(), "ignored", productdeliverycap.CurrentCustomerDeliveryListRequest{Limit: 20})
	if err != nil || page == nil {
		t.Fatalf("list failed: page=%#v err=%v", page, err)
	}
}
