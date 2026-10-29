// This file verifies the custom provisioning and upgrade controls have localized API documentation.
package v1

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCustomParameterAndUpgradeControlsHaveLocalizedDocumentation(t *testing.T) {
	raw, err := os.ReadFile("../../../../manifest/i18n/zh-CN/apidoc/catalog-custom-parameters.json")
	if err != nil {
		t.Fatal(err)
	}
	var translations map[string]string
	if err = json.Unmarshal(raw, &translations); err != nil {
		t.Fatal(err)
	}
	typ := reflect.TypeOf(CatalogCustomerFieldSelection{})
	for _, name := range []string{"CustomerVisible", "DefaultValue", "ProductFieldKey", "LocalField", "ProductChoiceBindings", "UpgradeEnabled", "UpgradeMinimum", "UpgradeMaximum", "UpgradeCandidateValues"} {
		field, ok := typ.FieldByName(name)
		if !ok || translations[field.Tag.Get("dc")] == "" {
			t.Fatalf("missing API translation for %s", name)
		}
	}
}
