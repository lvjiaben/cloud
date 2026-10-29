// This file verifies read-only product details remain available when a historical
// automation release cannot be verified; execution gates remain in the owner.
package admin

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

type unverifiedBindingReader struct{ productautomation.Service }

func (*unverifiedBindingReader) GetBinding(context.Context, string) (*productautomation.Binding, error) {
	return &productautomation.Binding{InterfaceDisplayName: "Frozen interface", EvidenceVerified: false}, nil
}
func (*unverifiedBindingReader) GetFrozenFulfillmentContract(context.Context, string) (*catalog.FrozenFulfillmentContract, error) {
	panic("unverified display evidence must not be treated as an executable contract")
}
func TestCatalogDetailRetainsPricesWithUnverifiedBinding(t *testing.T) {
	reader := &unverifiedProductReader{detail: &catalog.ProductManagementDetail{
		Product:           catalog.ProductManagementDetailProduct{ProductNo: "product-read"},
		Version:           catalog.ProductManagementDetailVersion{VersionNo: "version-read", Status: "published"},
		FulfillmentPolicy: &catalog.FulfillmentPolicyEvidence{StrategyCode: catalog.FulfillmentStrategyPaymentConfirmedPlugin},
		Prices:            []catalog.ProductManagementDetailPrice{{PriceNo: "price-read", CurrencyCode: "CNY", AmountMinor: 500, ConfigurationPricingMode: catalog.ConfigurationPricePerUnit}},
	}}
	controller := &ControllerV1{catalogManagement: reader}
	controller.productAutomation = &unverifiedBindingReader{}
	result, err := controller.CatalogProductDetail(context.Background(), &v1.CatalogProductDetailReq{ProductNo: "product-read"})
	if err != nil || result == nil || len(result.Prices) != 1 || result.Prices[0].ConfigurationPricingMode != "per_unit" || result.FulfillmentBinding == nil || result.FulfillmentBinding.EvidenceVerified || result.FulfillmentPolicy == nil || result.FulfillmentPolicy.TrialSupported {
		t.Fatalf("unverified read-only detail=%+v err=%v", result, err)
	}
}

type unverifiedProductReader struct {
	catalog.ProductManagementReader
	detail *catalog.ProductManagementDetail
}

func (r *unverifiedProductReader) GetProductDetail(context.Context, string) (*catalog.ProductManagementDetail, error) {
	return r.detail, nil
}
