// This file freezes the ordinary supplier API against subject-plaintext input or output.

package api

import (
	"reflect"
	"testing"

	adminv1 "lina-plugin-idc-compliance/backend/api/admin/v1"
)

func TestOrdinarySupplierAPIExcludesProviderSubjectPlaintext(t *testing.T) {
	assertSupplierJSONFields(t, adminv1.SupplierCredentialUpdateReq{}, []string{
		"providerCode", "credentialEnvelope", "expectedVersion", "idempotencyKey",
	})
	assertSupplierJSONFields(t, adminv1.SupplierSubjectBindingCreateReq{}, []string{
		"providerCode",
	})
	assertSupplierJSONFields(t, adminv1.SupplierSubjectRevocationCreateReq{}, []string{
		"providerCode", "bindingNo", "reasonCode", "expectedVersion", "idempotencyKey",
	})
	assertSupplierJSONFields(t, adminv1.SupplierSubjectReopeningCreateReq{}, []string{
		"providerCode",
	})
	assertSupplierJSONFields(t, adminv1.SupplierConfigurationItem{}, []string{
		"providerCode", "credentialStatus", "credentialConfigured", "credentialMaskedTail",
		"credentialGeneration", "subjectStatus", "subjectBindingNo", "lastOperationKind",
		"lastOperationResult", "lastOperationNo", "lastOperationAt",
		"capabilityEvidenceCode", "adapterEnabled", "version", "updatedAt",
	})
}

func assertSupplierJSONFields(t *testing.T, value any, expected []string) {
	t.Helper()
	typeOf := reflect.TypeOf(value)
	actual := make([]string, 0, typeOf.NumField())
	for index := 0; index < typeOf.NumField(); index++ {
		field := typeOf.Field(index)
		name := field.Tag.Get("json")
		if name == "" || name == "-" {
			continue
		}
		actual = append(actual, name)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("%s JSON fields=%v want=%v", typeOf.Name(), actual, expected)
	}
}
