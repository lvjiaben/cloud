// This test keeps the newly exposed assisted configuration contract localized.
package api

import (
	"reflect"
	"strings"
	"testing"

	adminv1 "lina-plugin-idc-core/backend/api/admin/v1"
)

func TestAssistedConfigurationAPIDocChineseTranslation(t *testing.T) {
	bundle := readFinancialAPIDocBundle(t, "manifest/i18n/zh-CN/apidoc")
	for _, key := range []string{
		"CommerceAssistedQuoteItemInput.fields.values.dc", "CommerceAssistedQuoteCreateReq.fields.paymentDueAt.dc",
		"CommerceAssistedQuoteCreateReq.meta.dc", "CommerceAssistedOrderOptionsReq.meta.dc", "CommerceAssistedOrderOptionsRes.fields.currencies.dc",
		"CommerceAssistedQuote.fields.statusCode.dc", "CommerceAssistedQuote.fields.canApprove.dc",
		"CommerceAssistedQuote.fields.canReject.dc", "CommerceAssistedQuote.fields.canCheckout.dc",
		"CommerceAssistedQuote.fields.decisionAt.dc", "CommerceAssistedQuote.fields.createdAt.dc",
		"CommerceAssistedQuote.fields.expiresAt.dc", "CommerceAssistedQuote.fields.quoteNo.dc",
		"CommerceAssistedQuoteListReq.meta.dc", "CommerceAssistedQuoteDetailReq.meta.dc",
	} {
		if bundle["plugins.idc_core.api.admin.v1."+key] == "" {
			t.Fatalf("missing assisted API Chinese translation: %s", key)
		}
	}
}

func TestAssistedOrderAPIDocDescribesDirectCheckout(t *testing.T) {
	response := reflect.TypeOf(adminv1.CommerceAssistedQuote{})
	for _, name := range []string{"CanApprove", "CanReject", "CanCheckout"} {
		field, ok := response.FieldByName(name)
		if !ok || !strings.Contains(field.Tag.Get("dc"), "Always false") || field.Tag.Get("eg") != "false" {
			t.Fatalf("%s must document that legacy workflow actions are disabled", name)
		}
	}
	for _, name := range []string{"DecisionAt", "ExpiresAt", "CreatedAt"} {
		field, ok := response.FieldByName(name)
		if !ok || !strings.Contains(field.Tag.Get("dc"), "Unix timestamp in milliseconds") {
			t.Fatalf("%s must document millisecond timestamps", name)
		}
	}
	field, _ := response.FieldByName("StatusCode")
	if field.Tag.Get("eg") != "ordered" || !strings.Contains(field.Tag.Get("dc"), "read-only historical") {
		t.Fatal("status must distinguish direct orders from read-only approval history")
	}
}
