// Read-only directory and commercial rule metadata must remain localized and
// absent from write DTOs; writers obtain their own authorized observations.
package v1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCatalogRelationshipDocumentationAndReadOnlyContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "i18n", "zh-CN", "apidoc", "catalog-choice-relationships.json"))
	if err != nil {
		t.Fatal(err)
	}
	translations := map[string]string{}
	if err = json.Unmarshal(data, &translations); err != nil {
		t.Fatal(err)
	}
	var fields []reflect.StructField
	for _, typeOf := range []reflect.Type{reflect.TypeOf(CatalogDirectoryRelationship{}), reflect.TypeOf(CatalogDirectoryChoiceLink{})} {
		for index := 0; index < typeOf.NumField(); index++ {
			fields = append(fields, typeOf.Field(index))
		}
	}
	output := reflect.TypeOf(CatalogProductVersionConfiguration{})
	for _, name := range []string{"DirectoryRelationships", "CustomerChoiceRules", "PendingAutomationFields"} {
		field, found := output.FieldByName(name)
		if !found {
			t.Fatal("missing relationship read projection", name)
		}
		fields = append(fields, field)
		if _, accepted := reflect.TypeOf(CatalogProductVersionConfigurationUpdateReq{}).FieldByName(name); accepted {
			t.Fatal("a write DTO accepted read-only relationship evidence", name)
		}
	}
	information, found := reflect.TypeOf(CatalogCustomerFieldSelection{}).FieldByName("InformationOnly")
	if !found {
		t.Fatal("missing explicit information-only resolution")
	}
	fields = append(fields, information)
	for _, field := range fields {
		source := field.Tag.Get("dc")
		if source == "" || field.Tag.Get("eg") == "" || strings.TrimSpace(translations[source]) == "" {
			t.Fatal("relationship documentation or Chinese translation is missing", field.Name)
		}
		for _, char := range source {
			if char > 127 {
				t.Fatal("relationship documentation source must be English", field.Name)
			}
		}
	}
	for _, name := range []string{"ChoiceRules", "AutomationMappingPending", "BasePriceIncluded", "FixedPriceIncluded"} {
		if _, accepted := reflect.TypeOf(CatalogCustomerFieldSelection{}).FieldByName(name); accepted {
			t.Fatal("an ordinary field DTO accepted owner-derived rules or pricing policy", name)
		}
	}
}
