package backend

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

func TestCertificateConfigurationsCoverFourProductShapes(t *testing.T) {
	tests := []struct {
		name       string
		product    productConfiguration
		domains    []string
		validation string
	}{
		{name: "ordinary", product: productConfiguration{CertificateScope: "ordinary"}, domains: []string{"example.com"}, validation: "dns"},
		{name: "multi-domain", product: productConfiguration{CertificateScope: "multi_domain"}, domains: []string{"example.com", "www.example.com"}, validation: "file"},
		{name: "wildcard", product: productConfiguration{CertificateScope: "wildcard"}, domains: []string{"*.example.com"}, validation: "dns"},
		{name: "ip", product: productConfiguration{CertificateScope: "ip"}, domains: []string{"192.0.2.10"}, validation: "file"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := json.Unmarshal(testProductConfiguration(t, test.product.CertificateScope, "dv", true, true), &test.product); err != nil || hydrateProductConfiguration(&test.product) != nil {
				t.Fatal("hydrate test product")
			}
			var customer customerConfiguration
			if err := json.Unmarshal(certificateCustomerConfiguration(t, test.domains, test.validation, false), &customer); err != nil {
				t.Fatal(err)
			}
			if err := validateCertificateConfiguration(test.product, customer); err != nil {
				t.Fatalf("valid %s certificate rejected: %v", test.name, err)
			}
		})
	}
}

func TestOVCertificateRequiresCompleteOrganizationInformation(t *testing.T) {
	var product productConfiguration
	if err := json.Unmarshal(testProductConfiguration(t, "ordinary", "ov", true, false), &product); err != nil || hydrateProductConfiguration(&product) != nil {
		t.Fatal("hydrate OV product")
	}
	var customer customerConfiguration
	if err := json.Unmarshal(certificateCustomerConfiguration(t, []string{"example.com"}, "dns", false), &customer); err != nil {
		t.Fatal(err)
	}
	if err := validateCertificateConfiguration(product, customer); err == nil {
		t.Fatal("OV certificate without organization information was accepted")
	}
	if err := json.Unmarshal(certificateCustomerConfiguration(t, []string{"example.com"}, "dns", true), &customer); err != nil {
		t.Fatal(err)
	}
	if err := validateCertificateConfiguration(product, customer); err != nil {
		t.Fatalf("complete OV certificate was rejected: %v", err)
	}
}

func TestCSRIsRequiredAndMustExactlyMatchRequestedDomains(t *testing.T) {
	var product productConfiguration
	if err := json.Unmarshal(testProductConfiguration(t, "multi_domain", "dv", true, false), &product); err != nil || hydrateProductConfiguration(&product) != nil {
		t.Fatal("hydrate DV product")
	}
	var customer customerConfiguration
	if err := json.Unmarshal(certificateCustomerConfiguration(t, []string{"example.com", "api.example.com"}, "dns", false), &customer); err != nil {
		t.Fatal(err)
	}
	customer.CSR = ""
	if err := validateCertificateConfiguration(product, customer); err == nil {
		t.Fatal("missing CSR was accepted")
	}
	if err := json.Unmarshal(certificateCustomerConfiguration(t, []string{"example.com", "www.example.com"}, "dns", false), &customer); err != nil {
		t.Fatal(err)
	}
	customer.AdditionalDomains = "api.example.com"
	if err := validateCertificateConfiguration(product, customer); err == nil {
		t.Fatal("CSR for different domains was accepted")
	}
}

func TestCSRRespectsHostConfigurationFieldLimit(t *testing.T) {
	if err := validateCSR(strings.Repeat("x", maximumCSRBytes+1), []string{"example.com"}); err == nil {
		t.Fatal("CSR larger than the host configuration limit was accepted")
	}
}

func TestPureConfigurationValidationDoesNotCallSupplier(t *testing.T) {
	requestPayload := validRequest(t, "")
	var request productfulfillmentcap.Request
	if err := json.Unmarshal(requestPayload, &request); err != nil {
		t.Fatal(err)
	}
	result, err := executeConfigurationValidation(request)
	if err != nil || string(result) != `{"version":"v1","valid":true}` {
		t.Fatalf("unexpected validation result: %s err=%v", result, err)
	}
}

func TestGuestConfigurationValidationIsPureAndGoverned(t *testing.T) {
	interfaces := &supplierInterfaceStub{}
	response, err := NewGuestHandler(interfaces)(extensionEnvelope(t, validationMethod, capmodel.BusinessExtensionInvocationPurposeNewBusiness, validRequest(t, "")))
	if err != nil || response.StatusCode != 200 || len(interfaces.requests) != 0 || !strings.Contains(string(response.Body), `"valid":true`) {
		t.Fatalf("response=%#v calls=%d err=%v", response, len(interfaces.requests), err)
	}
}

func TestSupplierValidationCapabilityIsFrozenIntoCandidate(t *testing.T) {
	products, err := decodeProducts(json.RawMessage(`{"success":true,"data":[{"product_id":"ssl-file-only","name":"File SSL","brand":"Yunsu","type":"DV","file_validation_only":1,"dns_validation_only":0}]}`))
	if err != nil || len(products) != 1 {
		t.Fatalf("products=%#v err=%v", products, err)
	}
	product := productConfiguration{PlanID: products[0].Value, CertificateScope: "ordinary"}
	if hydrateProductConfiguration(&product) != nil || product.DNSValidationSupported || !product.FileValidationSupported {
		t.Fatalf("file-only capability was not frozen: %#v", product)
	}
	var customer customerConfiguration
	if err = json.Unmarshal(certificateCustomerConfiguration(t, []string{"example.com"}, "dns", false), &customer); err != nil {
		t.Fatal(err)
	}
	if err = validateCertificateConfiguration(product, customer); err == nil {
		t.Fatal("DNS validation was accepted for a file-only supplier product")
	}
}
