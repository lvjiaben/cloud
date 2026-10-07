package finance

import (
	"bytes"
	"encoding/json"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestClearCartKeepsQueryFirstTokenOutsideGuestMemory(t *testing.T) {
	intent := validProvisionIntent()
	first, secrets, err := BuildClearCartCall(intent)
	if err != nil {
		t.Fatal(err)
	}
	if first.Path != "/cart/clear" || first.Form.Get("downstream_token") != "" || secrets["downstream_token"] != "downstream_token" {
		t.Fatalf("query-first callback secret crossed the guest boundary: %#v secrets=%#v", first, secrets)
	}
	encoded, err := json.Marshal(first)
	if err != nil || bytes.Contains(encoded, []byte("callback-secret-value")) || bytes.Contains(encoded, []byte("downstream_token")) {
		t.Fatalf("guest-visible request leaked callback authentication material: %s err=%v", encoded, err)
	}
}

func TestClearCartReconcilesExistingHostOrInvoice(t *testing.T) {
	existing, err := ClassifyClearOutcome(json.RawMessage(`{"status":400,"hostid":91,"domainstatus":"Active"}`))
	if err != nil || existing.ExistingResourceRef != "91" || existing.ContinuePurchase {
		t.Fatalf("existing host was not reconciled: %#v err=%v", existing, err)
	}
	invoice, err := ClassifyClearOutcome(json.RawMessage(`{"status":200,"invoiceid":71,"hostid":91}`))
	if err != nil || invoice.ExistingInvoiceRef != "71" || invoice.ExistingResourceRef != "91" {
		t.Fatalf("existing invoice was not reconciled: %#v err=%v", invoice, err)
	}
	clean, err := ClassifyClearOutcome(json.RawMessage(`{"status":200}`))
	if err != nil || !clean.ContinuePurchase {
		t.Fatalf("empty cart did not continue safely: %#v err=%v", clean, err)
	}
}

func TestPurchaseCallsUseFrozenProductAndSecretSlotOnly(t *testing.T) {
	intent := validProvisionIntent()
	add, secrets, err := BuildAddToCartCall(intent, "1")
	if err != nil {
		t.Fatal(err)
	}
	if add.Path != "/cart/add_to_shop" || add.Form.Get("pid") != "42" || add.Form.Get("configoption[11]") != "102" || len(secrets) != 0 || add.Form.Get("password") != "" {
		t.Fatalf("unsafe add-to-cart request: %#v secrets=%#v", add, secrets)
	}
	settle, settlementSecrets, err := BuildSettleCall(intent)
	if err != nil || settle.Path != "/cart/settle" || settle.Form.Get("cart_data[configoptions][11]") != "102" || settlementSecrets["downstream_token"] != "downstream_token" {
		t.Fatalf("unexpected settlement request: %#v err=%v", settle, err)
	}
}

func TestSettlementWithoutProvenHostFailsClosed(t *testing.T) {
	result := SettlementResult("request-1", json.RawMessage(`{"status":200,"data":{"invoiceid":71}}`))
	if result.Status != resourcecap.TaskStatusFailed || result.ExternalResourceRef != "" {
		t.Fatalf("invoice-only response became provisioned: %#v", result)
	}
	unknown := unknownResult("request-1", "")
	if unknown.Status != resourcecap.TaskStatusUnknown || unknown.Failure == nil || !unknown.Failure.ResultUnknown || unknown.Failure.Retryable {
		t.Fatalf("post-settlement transport ambiguity is unsafe: %#v", unknown)
	}
}

func validProvisionIntent() ProvisionIntent {
	return ProvisionIntent{
		DownstreamURL: "https://platform.example.test/api/provider-callback",
		DownstreamID:  "1001", DownstreamTokenBinding: "downstream_token",
		Parameters: json.RawMessage(`{"pid":"42","billingcycle":"monthly","host":"server-1","qty":1,"configoption":{"11":"102"}}`),
	}
}
