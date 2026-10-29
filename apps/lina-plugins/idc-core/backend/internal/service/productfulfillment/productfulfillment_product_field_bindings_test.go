// These tests prove immutable purchase bindings reach execution while ordinary
// information fields remain private to the order and legacy requests stay exact.
package productfulfillment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestFrozenProductChoiceBindingsUseSavedAliasesOnly(t *testing.T) {
	row := &claimedFulfillment{ProductDeclaration: `{"version":"v1","fields":[{"key":"storage","type":"single_select"}]}`, ProductConfiguration: json.RawMessage(`{"storage":"vendor-a"}`), CustomerDeclaration: `{"version":"v1","fields":[{"key":"local_storage","type":"single_select","required":true}]}`, CustomerPresentation: `{"local_storage":{"productFieldKey":"storage","productChoiceBindings":{"standard":"vendor-a","archive":"vendor-b"}}}`, CustomerConfiguration: json.RawMessage(`{"local_storage":"archive"}`)}
	result, digest, err := row.boundProductConfiguration()
	if err != nil || string(result) != `{"storage":"vendor-b"}` || digest == "" {
		t.Fatalf("frozen supplier value not applied: %s %v", result, err)
	}
	if string(row.CustomerConfiguration) != `{"local_storage":"archive"}` || string(row.ProductConfiguration) != `{"storage":"vendor-a"}` {
		t.Fatal("immutable commercial values or base changed")
	}
	newSale := *row
	newSale.CustomerPresentation = `{"local_storage":{"productFieldKey":"storage","productChoiceBindings":{"archive":"vendor-c"}}}`
	newResult, _, err := newSale.boundProductConfiguration()
	if err != nil || string(newResult) != `{"storage":"vendor-c"}` {
		t.Fatal("new sale did not use its own mapping")
	}
	oldResult, oldDigest, err := row.boundProductConfiguration()
	if err != nil || string(oldResult) != string(result) || oldDigest != digest {
		t.Fatal("a later mapping changed a paid snapshot")
	}
	for _, invalid := range []string{`{"local_storage":4}`, `{"local_storage":[]}`} {
		row.CustomerConfiguration = json.RawMessage(invalid)
		if _, _, err = row.boundProductConfiguration(); err == nil {
			t.Fatal("a non-choice input reached a choice mapping")
		}
	}
}

func TestFrozenProductBindingsUseCustomerChoiceAndKeepUnboundFieldsOut(t *testing.T) {
	row := &claimedFulfillment{ProductDeclaration: `{"version":"v1","fields":[{"key":"cpu","type":"integer"},{"key":"region","type":"single_select"}]}`, ProductConfiguration: json.RawMessage(`{"cpu":2,"region":"old"}`), ProductConfigurationDigest: "legacy-digest", CustomerDeclaration: `{"version":"v1","fields":[{"key":"local_cpu","type":"integer","required":true},{"key":"local_location","type":"single_select","required":true},{"key":"local_note","type":"single_line"}]}`, CustomerPresentation: `{"local_cpu":{"productFieldKey":"cpu"},"local_location":{"productFieldKey":"region"}}`, CustomerConfiguration: json.RawMessage(`{"local_cpu":8,"local_location":"new","local_note":"customer information"}`)}
	product, digest, err := row.boundProductConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	if string(product) != `{"cpu":8,"region":"new"}` {
		t.Fatalf("customer selection was not used: %s", product)
	}
	expected := sha256.Sum256(product)
	if digest != hex.EncodeToString(expected[:]) {
		t.Fatal("execution digest does not cover selected parameters")
	}
	customer, err := safeProviderCustomerConfiguration(row.CustomerDeclaration, row.CustomerConfiguration)
	if err != nil || string(customer) != "{}" {
		t.Fatalf("local information leaked: %s %v", customer, err)
	}
	if string(row.ProductConfiguration) != `{"cpu":2,"region":"old"}` {
		t.Fatal("frozen product base changed")
	}
	row.CustomerPresentation = ""
	legacy, legacyDigest, err := row.boundProductConfiguration()
	if err != nil || string(legacy) != string(row.ProductConfiguration) || legacyDigest != "legacy-digest" {
		t.Fatal("legacy paid order changed")
	}
}

func TestFrozenProductBindingsRejectMissingOrDuplicateTargets(t *testing.T) {
	for _, presentation := range []string{`{"local_x":{"productFieldKey":"invalid name"}}`, `{"local_x":{"productFieldKey":"cpu"},"local_y":{"productFieldKey":"cpu"}}`, `{"private":{"productFieldKey":"cpu"}}`} {
		row := &claimedFulfillment{ProductDeclaration: `{"version":"v1","fields":[{"key":"cpu","type":"integer"}]}`, ProductConfiguration: json.RawMessage(`{"cpu":2}`), CustomerDeclaration: `{"version":"v1","fields":[{"key":"local_x","type":"integer","required":true},{"key":"local_y","type":"integer","required":true},{"key":"private","type":"integer","required":true}]}`, CustomerConfiguration: json.RawMessage(`{"local_x":4,"local_y":8,"private":16}`), CustomerPresentation: presentation}
		if _, _, err := row.boundProductConfiguration(); err == nil {
			t.Fatal("invalid frozen mapping accepted")
		}
	}
}

func TestFrozenBindingCanSupplyOptionalProductText(t *testing.T) {
	row := &claimedFulfillment{ProductDeclaration: `{"version":"v1","fields":[{"key":"label","type":"single_line"}]}`, ProductConfiguration: json.RawMessage(`{}`), CustomerDeclaration: `{"version":"v1","fields":[{"key":"local_label","type":"single_line","required":true}]}`, CustomerPresentation: `{"local_label":{"productFieldKey":"label"}}`, CustomerConfiguration: json.RawMessage(`{"local_label":"customer choice"}`)}
	result, _, err := row.boundProductConfiguration()
	if err != nil || string(result) != `{"label":"customer choice"}` {
		t.Fatalf("optional bound parameter lost: %s %v", result, err)
	}
	row.ProductDeclaration = `{"version":"v1","fields":[{"key":"label","type":"secret"}]}`
	if _, _, err = row.boundProductConfiguration(); err == nil {
		t.Fatal("secret product parameter binding accepted")
	}
}

func TestFrozenStringProfileChoiceReachesDeclaredTextParameter(t *testing.T) {
	row := &claimedFulfillment{ProductDeclaration: `{"version":"v1","fields":[{"key":"system_disk_io_limit","type":"single_line"}]}`, ProductConfiguration: json.RawMessage(`{"system_disk_io_limit":"0,0,0,0"}`), CustomerDeclaration: `{"version":"v1","fields":[{"key":"local_disk","type":"single_select","required":true}]}`, CustomerPresentation: `{"local_disk":{"productFieldKey":"system_disk_io_limit"}}`, CustomerConfiguration: json.RawMessage(`{"local_disk":"500,500,2000,2000"}`)}
	result, _, err := row.boundProductConfiguration()
	if err != nil || string(result) != `{"system_disk_io_limit":"500,500,2000,2000"}` {
		t.Fatalf("selected performance profile lost: %s %v", result, err)
	}
}

func TestCustomNativeParametersReachFrozenExecutionWithExactContent(t *testing.T) {
	row := &claimedFulfillment{ProductDeclaration: `{"version":"v1","fields":[]}`, ProductConfiguration: json.RawMessage(`{}`),
		CustomerDeclaration:   `{"version":"v1","fields":[{"key":"local_shape","type":"single_select","required":true},{"key":"local_note","type":"single_line","required":true},{"key":"local_private","type":"single_line"}]}`,
		CustomerPresentation:  `{"local_shape":{"productFieldKey":"vendor_profile","productChoiceBindings":{"premium":"500,500,2000,2000"}},"local_note":{"productFieldKey":"扩展备注"}}`,
		CustomerConfiguration: json.RawMessage(`{"local_shape":"premium","local_note":"客户 & + = 备注","local_private":"do not send"}`)}
	raw, digest, err := row.boundProductConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]map[string]string
	if json.Unmarshal(raw, &result) != nil || result["custom_parameters"]["vendor_profile"] != "500,500,2000,2000" || result["custom_parameters"]["扩展备注"] != "客户 & + = 备注" {
		t.Fatalf("custom native content changed: %s", raw)
	}
	sum := sha256.Sum256(raw)
	if digest != hex.EncodeToString(sum[:]) {
		t.Fatal("custom parameters missing from execution digest")
	}
	safe, err := safeProviderCustomerConfiguration(row.CustomerDeclaration, row.CustomerConfiguration)
	if err != nil || string(safe) != "{}" {
		t.Fatalf("local field IDs leaked: %s %v", safe, err)
	}
	if string(row.ProductConfiguration) != "{}" {
		t.Fatal("historical source was mutated")
	}
}
