package v1

import (
	"reflect"
	"testing"
)

func TestInvoiceProfileProjectionExcludesBankSecretReference(t *testing.T) {
	typeValue := reflect.TypeOf(InvoiceProfile{})
	if _, exists := typeValue.FieldByName("BankAccountSecretRef"); exists {
		t.Fatal("ordinary customer invoice profile API exposes the bank secret reference")
	}
	field, exists := typeValue.FieldByName("BankAccountMasked")
	if !exists || field.Tag.Get("json") != "bankAccountMasked,omitempty" {
		t.Fatal("customer invoice profile API is missing the display-only bank mask")
	}
}
