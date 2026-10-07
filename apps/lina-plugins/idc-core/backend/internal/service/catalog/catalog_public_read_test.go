package catalog

import (
	"bytes"
	"strings"
	"testing"
)

func testPublicCatalogCursorCodec(t *testing.T) *publicCatalogCursorCodec {
	t.Helper()
	codec, err := newPublicCatalogCursorCodec(bytes.Repeat([]byte{0x39}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return codec
}

func TestPublicCatalogAvailabilityUsesSimpleInventory(t *testing.T) {
	if !strings.Contains(publicCatalogProductsSQL, "plugin_idc_core_catalog_product_version_inventory") ||
		!strings.Contains(publicCatalogProductsSQL, "THEN 'available'") ||
		!strings.Contains(publicCatalogProductsSQL, "ELSE 'unavailable'") {
		t.Fatal("public catalog does not project authoritative simple inventory")
	}
	if strings.Contains(publicCatalogProductsSQL, "manual_confirmation") {
		t.Fatal("public catalog still exposes manual inventory confirmation")
	}
}

func TestPublicCatalogSearchAcceptsGenericProducts(t *testing.T) {
	codec := testPublicCatalogCursorCodec(t)
	for _, name := range []string{"SSL 证书", "手机", "人工服务"} {
		normalized, err := normalizePublicCatalogSearch(PublicCatalogSearchInput{NamePrefix: name, Currency: "CNY", Limit: 12}, codec)
		if err != nil || normalized.namePrefix == "" {
			t.Fatalf("%q normalized=%+v err=%v", name, normalized, err)
		}
	}
}

func TestPublicCatalogProjectionContainsOnlyGenericAvailability(t *testing.T) {
	for _, availability := range []string{"available", "unavailable"} {
		valid := PublicCatalogProduct{PublicSlug: "managed-ssl", CategoryNo: "ctg_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Currencies: []string{"CNY", "USD"}, Availability: availability}
		if err := validatePublicCatalogProduct(valid); err != nil {
			t.Fatalf("availability %q rejected: %v", availability, err)
		}
	}
	for _, availability := range []string{"manual_confirmation", "provider_pending"} {
		invalid := PublicCatalogProduct{PublicSlug: "managed-ssl", CategoryNo: "ctg_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Currencies: []string{"CNY"}, Availability: availability}
		if err := validatePublicCatalogProduct(invalid); err == nil {
			t.Fatalf("unsafe availability %q accepted", availability)
		}
	}
}
