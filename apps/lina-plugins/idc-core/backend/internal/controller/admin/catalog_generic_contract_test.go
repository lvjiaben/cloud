package admin

import (
	"encoding/json"
	"strings"
	"testing"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	publicv1 "lina-plugin-idc-core/backend/api/public/v1"
)

var forbiddenGenericCatalogFields = []string{
	"productType", "configurationMode", "configurationGroups", "configurationOptions",
	"componentType", "normalizedValue", "resourcePools", "inventoryObservedAt",
}

func TestGenericCatalogCreateContractHasNoServerDiscriminator(t *testing.T) {
	for _, req := range []*v1.CatalogProductCreateReq{
		{CategoryNo: "ctg_ssl", PublicSlug: "managed-ssl", NameZhCn: "SSL 证书", NameEnUs: "SSL Certificate"},
		{CategoryNo: "ctg_phone", PublicSlug: "phone", NameZhCn: "手机", NameEnUs: "Phone"},
		{CategoryNo: "ctg_service", PublicSlug: "manual-service", NameZhCn: "人工服务", NameEnUs: "Manual Service"},
	} {
		encoded, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		payload := string(encoded)
		for _, forbidden := range forbiddenGenericCatalogFields {
			if strings.Contains(payload, forbidden) {
				t.Fatalf("generic request leaked %q: %s", forbidden, payload)
			}
		}
	}
}

func TestGenericCatalogResponsesHaveNoLegacyServerProjection(t *testing.T) {
	responses := []any{
		&v1.CatalogProductListRes{Products: []*v1.CatalogProductSummaryItem{{ProductNo: "prd_ssl"}}},
		&v1.CatalogProductDetailRes{
			Product:   &v1.CatalogProductDetailProduct{ProductNo: "prd_ssl"},
			Version:   &v1.CatalogProductDetailVersion{VersionNo: "ver_ssl"},
			Inventory: &v1.CatalogProductVersionInventory{TrackingEnabled: true, SellableQuantity: 5, Version: 1},
		},
		&publicv1.CatalogProductItem{PublicSlug: "managed-ssl", CategoryNo: "ctg_services"},
	}
	for _, response := range responses {
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		payload := string(encoded)
		for _, forbidden := range forbiddenGenericCatalogFields {
			if strings.Contains(payload, forbidden) {
				t.Fatalf("generic response leaked %q: %s", forbidden, payload)
			}
		}
	}
}

func TestGenericCategoryContractHasNoFixedType(t *testing.T) {
	encoded, err := json.Marshal(&v1.CatalogCategoryCreateReq{NameZhCn: "证书", NameEnUs: "Certificates"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "type") {
		t.Fatalf("category leaked fixed type: %s", encoded)
	}
}
