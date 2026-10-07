package catalog

import (
	"context"
	"reflect"
	"testing"

	"lina-core/pkg/bizerr"
)

func TestPurchaseRevalidationRejectsUnboundedOrDuplicateInput(t *testing.T) {
	service := &publicCatalogReaderImpl{}
	tooManyReferences := make([]PurchaseConfigurationReference, maximumPurchaseRevalidationItems+1)
	if _, err := service.RevalidatePurchaseConfigurations(context.Background(), tooManyReferences); !bizerr.Is(err, CodePublicQuoteInvalid) {
		t.Fatalf("unbounded purchase references error=%v", err)
	}
	duplicate := []PurchaseConfigurationReference{{
		ReferenceNo: "cart-item-1", ProductNo: "product-1", VersionNo: "version-1",
		BillingRuleNo: "billing-1",
	}, {
		ReferenceNo: "cart-item-1", ProductNo: "product-2", VersionNo: "version-2",
		BillingRuleNo: "billing-2",
	}}
	if _, err := service.RevalidatePurchaseConfigurations(context.Background(), duplicate); !bizerr.Is(err, CodePublicQuoteInvalid) {
		t.Fatalf("duplicate purchase reference error=%v", err)
	}
}

func TestGenericPurchaseContractsExcludeLegacyServerConfiguration(t *testing.T) {
	for _, contract := range []struct {
		name   string
		value  any
		fields []string
	}{
		{name: "input", value: PurchaseConfigurationInput{}, fields: []string{"Selections", "Revision", "TermNo"}},
		{name: "result", value: PurchaseConfiguration{}, fields: []string{"Items"}},
		{name: "reference", value: PurchaseConfigurationReference{}, fields: []string{"OptionNos"}},
		{name: "quote line", value: PurchaseQuoteLineInput{}, fields: []string{"Options"}},
	} {
		typeOf := reflect.TypeOf(contract.value)
		for _, field := range contract.fields {
			if _, exists := typeOf.FieldByName(field); exists {
				t.Fatalf("generic purchase %s still exposes %s", contract.name, field)
			}
		}
	}
	inputType := reflect.TypeOf(PurchaseConfigurationInput{})
	for _, field := range []string{"ConfigurationRevision", "BillingTermToken"} {
		if _, exists := inputType.FieldByName(field); !exists {
			t.Fatalf("generic purchase input is missing %s", field)
		}
	}
	if _, exists := reflect.TypeOf(ServerResizeConfigurationInput{}).FieldByName("Selections"); !exists {
		t.Fatal("server resize input lost its isolated server selections")
	}
	if _, exists := reflect.TypeOf(ServerResizeConfiguration{}).FieldByName("Items"); !exists {
		t.Fatal("server resize result lost its isolated server configuration")
	}
}

func TestUniqueStringsSortsAndDeduplicates(t *testing.T) {
	values := uniqueStrings([]string{"b", "a", "b", "c", "a"})
	want := []string{"a", "b", "c"}
	if len(values) != len(want) {
		t.Fatalf("unique values=%v", values)
	}
	for index := range want {
		if values[index] != want[index] {
			t.Fatalf("unique values=%v want=%v", values, want)
		}
	}
}
