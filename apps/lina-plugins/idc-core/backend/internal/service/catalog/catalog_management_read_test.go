package catalog

import (
	"bytes"
	"math"
	"testing"
)

// TestProductManagementNumberedPageBounds rejects unbounded or ambiguous input.
func TestProductManagementNumberedPageBounds(t *testing.T) {
	codec := testProductManagementCursorCodec(t)
	for _, input := range []ProductManagementSearchInput{
		{Page: -1, Limit: 10}, {Page: 1, Limit: 101}, {Page: 1, Cursor: "opaque", Limit: 10}, {Page: math.MaxInt, Limit: 100},
	} {
		if _, err := normalizeProductManagementSearch(input, codec); err == nil {
			t.Fatalf("accepted ambiguous or unbounded input: %+v", input)
		}
	}
	normalized, err := normalizeProductManagementSearch(ProductManagementSearchInput{Page: 7, Limit: 100}, codec)
	if err != nil || normalized.page != 7 || normalized.limit != 100 {
		t.Fatalf("valid numeric page: %+v, %v", normalized, err)
	}
}

func testProductManagementCursorCodec(t *testing.T) *productManagementCursorCodec {
	t.Helper()
	codec, err := newProductManagementCursorCodec(bytes.Repeat([]byte{0x5a}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return codec
}

func TestProductManagementSearchIsGenericAndCursorBound(t *testing.T) {
	codec := testProductManagementCursorCodec(t)
	normalized, err := normalizeProductManagementSearch(ProductManagementSearchInput{NamePrefix: "  Certificate_%  ", CategoryNo: "ctg_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Status: "active", Source: "manual", Limit: 25}, codec)
	if err != nil || normalized.namePrefix != "certificate_%" {
		t.Fatalf("normalized=%+v err=%v", normalized, err)
	}
	cursor, err := codec.encode(productManagementCursorBoundary{SortOrder: 10, ID: 9}, normalized)
	if err != nil {
		t.Fatal(err)
	}
	changed := normalized
	changed.status = "draft"
	if _, err = codec.decode(cursor, changed); err == nil {
		t.Fatal("cursor crossed filters")
	}
	changed = normalized
	changed.categoryNo = "ctg_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err = codec.decode(cursor, changed); err == nil {
		t.Fatal("cursor crossed category filters")
	}
	if _, err = normalizeProductManagementSearch(ProductManagementSearchInput{CategoryNo: "../unsafe", Limit: 25}, codec); err == nil {
		t.Fatal("unsafe category number was accepted")
	}
}

func TestProductManagementSearchSupportsDifferentBusinessKinds(t *testing.T) {
	codec := testProductManagementCursorCodec(t)
	for _, name := range []string{"SSL 证书", "手机", "人工服务"} {
		if _, err := normalizeProductManagementSearch(ProductManagementSearchInput{NamePrefix: name, Limit: 20}, codec); err != nil {
			t.Fatalf("%q rejected: %v", name, err)
		}
	}
}
