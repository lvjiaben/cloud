// This file verifies every diagnostic DTO field has an English source and a
// corresponding Chinese API-document translation owned by the core plugin.
package v1

import (
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExecutionDiagnosticDocumentationTranslations(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "manifest", "i18n", "zh-CN", "apidoc", "admin-product-fulfillment.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var translations map[string]string
	if err = json.Unmarshal(data, &translations); err != nil {
		t.Fatal(err)
	}
	for _, typ := range []reflect.Type{reflect.TypeOf(ProductFulfillmentTaskDetail{}), reflect.TypeOf(ProductFulfillmentExecutionDiagnostic{}), reflect.TypeOf(ProductFulfillmentNonAcceptanceVerificationReq{}), reflect.TypeOf(ProductFulfillmentNonAcceptanceVerificationRes{}), reflect.TypeOf(ProductFulfillmentNonAcceptanceRecord{})} {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if typ.Name() == "ProductFulfillmentTaskDetail" && field.Name != "Execution" && field.Name != "CanVerifyNonAcceptance" && field.Name != "NonAcceptanceVerifications" {
				continue
			}
			if field.Name == "Meta" || field.Anonymous {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			key := "plugins.idc_core.api.admin.v1." + typ.Name() + ".fields." + name + ".dc"
			if field.Tag.Get("dc") == "" || field.Tag.Get("eg") == "" || strings.TrimSpace(translations[key]) == "" {
				t.Errorf("missing source, example, or Chinese translation for %s", key)
			}
		}
	}
}

// Recovery fields retain plugin-owned documentation even when a nested schema
// is declared by the public capability contract rather than this API package.
func TestRecoveryConfigurationDocumentationTranslations(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "i18n", "zh-CN", "apidoc", "admin-product-fulfillment.json"))
	if err != nil {
		t.Fatal(err)
	}
	translations := map[string]string{}
	if err = json.Unmarshal(data, &translations); err != nil {
		t.Fatal(err)
	}
	types := []reflect.Type{reflect.TypeOf(provisionresourcecap.RecoveryConfiguration{}), reflect.TypeOf(provisionresourcecap.RecoveryOption{}), reflect.TypeOf(provisionresourcecap.RecoverySpecification{}), reflect.TypeOf(provisionresourcecap.RecoveryEditor{}), reflect.TypeOf(ProductFulfillmentRecoveryConfigurationRecord{})}
	for _, typ := range types {
		prefix := "plugins.idc_core.backend.cap.provisionresourcecap."
		if typ.PkgPath() == reflect.TypeOf(ProductFulfillmentRecoveryConfigurationRecord{}).PkgPath() {
			prefix = "plugins.idc_core.api.admin.v1."
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			key := prefix + typ.Name() + ".fields." + name + ".dc"
			if field.Tag.Get("dc") == "" || field.Tag.Get("eg") == "" || translations[key] == "" {
				t.Errorf("missing recovery API source/example/translation: %s", key)
			}
		}
	}
}
