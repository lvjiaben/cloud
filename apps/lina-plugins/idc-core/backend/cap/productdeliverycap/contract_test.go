package productdeliverycap

import "testing"

func TestCustomerDeliveryProjectionRejectsPrivateOrUnsafeValues(t *testing.T) {
	request := CurrentCustomerDeliveryListRequest{Limit: 20}
	page := &CurrentCustomerDeliveryPage{Items: []CurrentCustomerDeliveryItem{{
		DeliveryNo: "DLV-0123456789ABCDEF0123456789ABCDEF", ProductNameZhCN: "通配符证书",
		ProductNameEnUS: "Wildcard certificate", Status: "processing", Quantity: 1,
		ProviderReference: "CERT-0123456789ABCDEF", UpdatedAt: 1,
	}}}
	if err := ValidatePage(page, request); err != nil {
		t.Fatalf("valid safe page rejected: %v", err)
	}
	page.Items[0].ProviderReference = "secret value with spaces"
	if err := ValidatePage(page, request); err == nil {
		t.Fatal("unsafe provider reference was accepted")
	}
}

func TestCustomerDeliveryRequestDoesNotAcceptCustomerOrPluginIdentity(t *testing.T) {
	request := CurrentCustomerDeliveryListRequest{Limit: 50, Cursor: "next_1"}
	if err := ValidateListRequest(request); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	if err := ValidateListRequest(CurrentCustomerDeliveryListRequest{Limit: 51}); err == nil {
		t.Fatal("oversized request was accepted")
	}
}
