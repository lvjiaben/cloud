package bridge

import (
	"context"
	"encoding/json"
	"testing"

	"lina-plugin-idc-core/backend/cap/idccap"
)

func TestClientUsesExactOwnerAwareEnvelope(t *testing.T) {
	t.Parallel()
	want := merchantFixture()
	client, err := NewClient(func(owner, service, version, method, resourceRef, table string, payload []byte) ([]byte, error) {
		if owner != idccap.OwnerPluginID || service != idccap.ServiceName ||
			version != string(idccap.ContractVersion) ||
			method != string(idccap.MethodMerchantContextCurrent) ||
			resourceRef != "" || table != "" || len(payload) != 0 {
			t.Fatalf("unexpected owner-aware envelope: %q %q %q %q %q %q payload=%d", owner, service, version, method, resourceRef, table, len(payload))
		}
		return json.Marshal(want)
	})
	if err != nil {
		t.Fatalf("create bridge client: %v", err)
	}
	got, err := client.CurrentMerchant(context.Background())
	if err != nil || got.MerchantNo != want.MerchantNo {
		t.Fatalf("read merchant context: got=%+v err=%v", got, err)
	}
}

func TestClientRejectsInvalidResponse(t *testing.T) {
	t.Parallel()
	client, err := NewClient(func(string, string, string, string, string, string, []byte) ([]byte, error) {
		return []byte(`{"tenantId":1}`), nil
	})
	if err != nil {
		t.Fatalf("create bridge client: %v", err)
	}
	if _, err = client.CurrentMerchant(context.Background()); err == nil {
		t.Fatal("invalid merchant projection was accepted")
	}
}

func merchantFixture() idccap.MerchantContext {
	return idccap.MerchantContext{
		TenantID: 0, MerchantNo: "merchant_local", DisplayName: "Local merchant",
		LegalName: "Local merchant Ltd.", CountryCode: "CN", DefaultLocale: "zh-CN",
		DefaultCurrency: "CNY", Timezone: "Asia/Shanghai", Enabled: true, Version: 1,
	}
}
