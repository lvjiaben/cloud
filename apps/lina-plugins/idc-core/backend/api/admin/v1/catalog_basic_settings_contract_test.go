// This file checks basic-form API documentation stays English and translated.
package v1

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestBasicSettingsDocumentation(t *testing.T) {
	raw, err := os.ReadFile("../../../../manifest/i18n/zh-CN/apidoc/catalog-description.json")
	if err != nil {
		t.Fatal(err)
	}
	messages := map[string]string{}
	if err = json.Unmarshal(raw, &messages); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []reflect.Type{reflect.TypeFor[CatalogProductDescriptionUpdateReq](), reflect.TypeFor[CatalogBasicInventoryInput](), reflect.TypeFor[CatalogBasicLoginRulesInput]()} {
		for _, field := range reflect.VisibleFields(kind) {
			if field.Name == "Meta" {
				for _, tag := range []string{"summary", "dc"} {
					if messages["plugins.idc_core.api.admin.v1."+kind.Name()+".meta."+tag] == "" {
						t.Fatalf("missing translated metadata %s", tag)
					}
				}
				continue
			}
			if field.Anonymous {
				continue
			}
			name := field.Tag.Get("json")
			for i, c := range name {
				if c == ',' {
					name = name[:i]
					break
				}
			}
			if field.Tag.Get("dc") == "" || field.Tag.Get("eg") == "" || messages["plugins.idc_core.api.admin.v1."+kind.Name()+".fields."+name+".dc"] == "" {
				t.Fatalf("incomplete translated field %s.%s", kind.Name(), field.Name)
			}
			for _, r := range field.Tag.Get("dc") {
				if r > 127 {
					t.Fatal("source document must be English")
				}
			}
		}
	}
}
