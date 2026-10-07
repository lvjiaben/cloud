package v1

import (
	"reflect"
	"testing"
)

func TestCustomerTaxInvoiceProjectionExcludesInternalAndSupplierFields(t *testing.T) {
	typeValue := reflect.TypeOf(CustomerTaxInvoiceItem{})
	for _, forbidden := range []string{
		"CustomerNo", "DecisionInternalNote", "ExternalInvoiceRef",
		"ProviderCode", "SellerSnapshotRef", "BuyerSnapshotRef",
	} {
		if _, exists := typeValue.FieldByName(forbidden); exists {
			t.Fatalf("customer projection exposes forbidden field %s", forbidden)
		}
	}
}

func TestCustomerTaxInvoiceCreateAcceptsNoTaxOrInvoiceTypeFields(t *testing.T) {
	typeValue := reflect.TypeOf(CustomerTaxInvoiceCreateReq{})
	for _, forbidden := range []string{
		"CustomerNo", "InvoiceTypeCode", "TaxRatePPM", "TaxAmountMinor",
		"SellerNo", "ProviderCode",
	} {
		if _, exists := typeValue.FieldByName(forbidden); exists {
			t.Fatalf("customer create request exposes forbidden field %s", forbidden)
		}
	}
}
