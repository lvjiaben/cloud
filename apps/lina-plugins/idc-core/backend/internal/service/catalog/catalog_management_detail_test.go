package catalog

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

func TestProductManagementDetailAuthorizesBeforePersistence(t *testing.T) {
	authorizationError := errors.New("permission dependency unavailable")
	authorizer := &managementTestAuthorizer{actor: 7, err: authorizationError}
	reader, err := NewProductManagementReader(authorizer, bytes.Repeat([]byte{0x33}, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reader.GetProductDetail(context.Background(), "product-route"); !errors.Is(err, authorizationError) {
		t.Fatalf("authorization error was not fail-closed: %v", err)
	}
}

func TestProductManagementDetailAcceptsGenericCommercialProjection(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	detail := &ProductManagementDetail{
		Product:      ProductManagementDetailProduct{ProductNo: "product-phone", CreatedAt: now, UpdatedAt: now},
		Version:      ProductManagementDetailVersion{VersionNo: "version-phone-1", VersionNumber: 1, CreatedAt: now},
		Inventory:    ProductVersionInventory{Version: 1},
		BillingRules: []ProductManagementDetailBillingRule{{BillingRuleNo: "billing-phone"}},
		Prices:       []ProductManagementDetailPrice{{PriceNo: "price-phone", BillingRuleNo: "billing-phone", CurrencyCode: "CNY"}},
	}
	if err := validateProductManagementDetail(detail); err != nil {
		t.Fatalf("generic projection rejected: %v", err)
	}
}
