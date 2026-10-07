package v1

import (
	"reflect"
	"testing"
)

func TestCommerceAssistedQuoteItemDoesNotExposeLegacyServerSelections(t *testing.T) {
	typeOf := reflect.TypeOf(CommerceAssistedQuoteItemInput{})
	for _, field := range []string{"Selections", "Revision", "TermNo"} {
		if _, ok := typeOf.FieldByName(field); ok {
			t.Fatalf("generic assisted quote input must not expose legacy field %s", field)
		}
	}
	for name, jsonName := range map[string]string{
		"ConfigurationRevision": "configurationRevision",
		"BillingTermToken":      "billingTermToken",
	} {
		field, ok := typeOf.FieldByName(name)
		if !ok || field.Tag.Get("json") != jsonName {
			t.Fatalf("generic assisted quote input field %s json tag=%q", name, field.Tag.Get("json"))
		}
	}
}
