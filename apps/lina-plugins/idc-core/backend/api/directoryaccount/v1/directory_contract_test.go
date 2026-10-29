// This file guards customer directory API documentation against untranslated
// projection fields, without depending on a running server or real customer data.

package v1

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestServiceProjectionDocumentation(t *testing.T) {
	content, err := os.ReadFile("../../../../manifest/i18n/zh-CN/apidoc/product-directories.json")
	if err != nil {
		t.Fatal(err)
	}
	translations := make(map[string]string)
	if err = json.Unmarshal(content, &translations); err != nil {
		t.Fatal(err)
	}
	projection := reflect.TypeFor[Entitlement]()
	for i := range projection.NumField() {
		field := projection.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || field.Tag.Get("dc") == "" || field.Tag.Get("eg") == "" {
			t.Fatalf("undocumented service field %s", field.Name)
		}
		if translations["plugins.idc_core.api.directoryaccount.v1.Entitlement.fields."+name+".dc"] == "" {
			t.Fatalf("missing Chinese documentation for %s", name)
		}
	}
}
