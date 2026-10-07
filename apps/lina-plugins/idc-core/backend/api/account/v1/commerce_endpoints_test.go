package v1

import (
	"reflect"
	"testing"
)

func TestCommerceCartItemAddDoesNotExposeLegacyServerSelections(t *testing.T) {
	typeOf := reflect.TypeOf(CommerceCartItemAddReq{})
	for _, field := range []string{"Selections", "Revision", "TermNo"} {
		if _, ok := typeOf.FieldByName(field); ok {
			t.Fatalf("generic cart input must not expose legacy field %s", field)
		}
	}
	for name, jsonName := range map[string]string{
		"ConfigurationRevision": "configurationRevision",
		"BillingTermToken":      "billingTermToken",
	} {
		field, ok := typeOf.FieldByName(name)
		if !ok || field.Tag.Get("json") != jsonName {
			t.Fatalf("generic cart input field %s json tag=%q", name, field.Tag.Get("json"))
		}
	}
}
