// This file tests owner-proven catalog visibility without interpreting the
// unrelated product-fulfillment method set or connector display names.
package productautomation

import (
	"lina-plugin-idc-core/backend/internal/model/entity"
	"reflect"
	"testing"
)

func TestInterfaceCatalogAvailabilityUsesResourceOwnerBatch(t *testing.T) {
	t.Parallel()
	source := &resourceCandidateSourceStub{catalog: map[string]bool{"pfi_finance": true, "pfi_cloud": false}}
	service := &serviceImpl{resourceCandidates: source}
	rows := []*entity.ProductAutomationInterfaceInstance{
		{InterfaceInstanceNo: "pfi_finance", InterfaceOriginCode: "resource", AuthorizedMethods: []string{"provision", "query"}},
		{InterfaceInstanceNo: "pfi_cloud", InterfaceOriginCode: "resource", AuthorizedMethods: []string{"provision", "query"}},
		{InterfaceInstanceNo: "pfi_managed", InterfaceOriginCode: "managed", AuthorizedMethods: []string{"catalog.products.list", "catalog.products.detail"}},
	}
	result, err := service.interfaceCatalogAvailability(t.Context(), rows)
	if err != nil || !result["pfi_finance"] || result["pfi_cloud"] || result["pfi_managed"] || !reflect.DeepEqual(source.catalogIDs, []string{"pfi_finance", "pfi_cloud"}) {
		t.Fatalf("owner availability=%v ids=%v err=%v", result, source.catalogIDs, err)
	}
}
