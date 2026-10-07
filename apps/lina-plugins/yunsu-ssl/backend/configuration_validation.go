package backend

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net"
	"net/mail"
	"strings"

	"golang.org/x/net/idna"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

const (
	maximumCertificateDomains = 100
	maximumCSRBytes           = 8192
)

type configurationValidationResult struct {
	Version string `json:"version"`
	Valid   bool   `json:"valid"`
}

func executeConfigurationValidation(request productfulfillmentcap.Request) (json.RawMessage, error) {
	product, customer, err := decodeConfigurations(request)
	if err != nil {
		return json.Marshal(configurationValidationResult{Version: "v1", Valid: false})
	}
	if _, err = normalizedDomainList(product, customer); err != nil {
		return json.Marshal(configurationValidationResult{Version: "v1", Valid: false})
	}
	return json.Marshal(configurationValidationResult{Version: "v1", Valid: true})
}

func validateCertificateConfiguration(product productConfiguration, customer customerConfiguration) error {
	if !validProductConfiguration(product) ||
		(customer.ValidationMethod != "dns" && customer.ValidationMethod != "file") ||
		(customer.ValidationMethod == "dns" && !product.DNSValidationSupported) ||
		(customer.ValidationMethod == "file" && !product.FileValidationSupported) ||
		strings.TrimSpace(customer.PrimaryDomain) == "" || strings.TrimSpace(customer.Phone) == "" {
		return io.ErrUnexpectedEOF
	}
	address, err := mail.ParseAddress(strings.TrimSpace(customer.Email))
	if err != nil || address.Address != strings.TrimSpace(customer.Email) {
		return io.ErrUnexpectedEOF
	}
	if product.CertificateValidationLevel != "dv" && !completeOrganizationConfiguration(customer) {
		return io.ErrUnexpectedEOF
	}
	domains, err := normalizedDomainList(product, customer)
	if err != nil {
		return err
	}
	return validateCSR(customer.CSR, domains)
}

func validProductConfiguration(product productConfiguration) bool {
	return strings.HasPrefix(product.PlanID, "ysp1.") && len(product.PlanID) <= 2048 && safeReference.MatchString(product.SupplierPlanID) &&
		(product.CertificateScope == "ordinary" || product.CertificateScope == "multi_domain" || product.CertificateScope == "wildcard" || product.CertificateScope == "ip") &&
		(product.CertificateValidationLevel == "dv" || product.CertificateValidationLevel == "ov" || product.CertificateValidationLevel == "ev") &&
		(product.DNSValidationSupported || product.FileValidationSupported)
}

func completeOrganizationConfiguration(customer customerConfiguration) bool {
	values := []string{
		customer.ContactFirstName, customer.ContactLastName, customer.ContactTitle,
		customer.OrganizationName, customer.OrganizationPhone, customer.OrganizationAddress,
		customer.OrganizationCity, customer.OrganizationState, customer.OrganizationCountry,
		customer.OrganizationPostcode, customer.OrganizationRegistrationNumber,
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	country := strings.TrimSpace(customer.OrganizationCountry)
	return len(country) == 2 && country == strings.ToUpper(country)
}

func normalizedDomainList(product productConfiguration, customer customerConfiguration) ([]string, error) {
	values := []string{strings.TrimSpace(customer.PrimaryDomain)}
	for _, value := range strings.FieldsFunc(customer.AdditionalDomains, func(character rune) bool {
		return character == ',' || character == '\n' || character == '\r'
	}) {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 || len(values) > maximumCertificateDomains {
		return nil, io.ErrUnexpectedEOF
	}
	if product.CertificateScope == "multi_domain" {
		if len(values) < 2 {
			return nil, io.ErrUnexpectedEOF
		}
	} else if len(values) != 1 {
		return nil, io.ErrUnexpectedEOF
	}
	if product.CertificateScope == "ip" && customer.ValidationMethod != "file" || product.CertificateScope == "wildcard" && customer.ValidationMethod != "dns" {
		return nil, io.ErrUnexpectedEOF
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		normalized, kind, err := normalizeCertificateIdentifier(value)
		if err != nil || index > 0 && kind != "domain" {
			return nil, io.ErrUnexpectedEOF
		}
		expectedKind := "domain"
		if product.CertificateScope == "ip" {
			expectedKind = "ip"
		} else if product.CertificateScope == "wildcard" {
			expectedKind = "wildcard"
		}
		if index == 0 && kind != expectedKind || index > 0 && strings.HasPrefix(normalized, "*.") {
			return nil, io.ErrUnexpectedEOF
		}
		if _, duplicated := seen[normalized]; duplicated {
			return nil, io.ErrUnexpectedEOF
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result, nil
}

func normalizeCertificateIdentifier(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if parsed := net.ParseIP(value); parsed != nil {
		return parsed.String(), "ip", nil
	}
	kind := "domain"
	if strings.HasPrefix(value, "*.") {
		kind = "wildcard"
		value = strings.TrimPrefix(value, "*.")
	}
	ascii, err := idna.Lookup.ToASCII(strings.TrimSuffix(value, "."))
	if err != nil || !validASCIIDomain(ascii) {
		return "", "", io.ErrUnexpectedEOF
	}
	if kind == "wildcard" {
		ascii = "*." + ascii
	}
	return strings.ToLower(ascii), kind, nil
}

func validASCIIDomain(value string) bool {
	if len(value) < 3 || len(value) > 253 || !strings.Contains(value, ".") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if character != '-' && (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
				return false
			}
		}
	}
	return true
}

func validateCSR(content string, domains []string) error {
	if len(content) > maximumCSRBytes {
		return io.ErrUnexpectedEOF
	}
	block, trailing := pem.Decode([]byte(strings.TrimSpace(content)))
	if block == nil || len(bytes.TrimSpace(trailing)) != 0 || (block.Type != "CERTIFICATE REQUEST" && block.Type != "NEW CERTIFICATE REQUEST") {
		return io.ErrUnexpectedEOF
	}
	request, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil || request.CheckSignature() != nil {
		return io.ErrUnexpectedEOF
	}
	requested := make(map[string]struct{}, len(request.DNSNames)+len(request.IPAddresses)+1)
	for _, value := range request.DNSNames {
		normalized, _, err := normalizeCertificateIdentifier(value)
		if err != nil {
			return io.ErrUnexpectedEOF
		}
		requested[normalized] = struct{}{}
	}
	for _, value := range request.IPAddresses {
		requested[value.String()] = struct{}{}
	}
	if len(requested) == 0 && request.Subject.CommonName != "" {
		normalized, _, err := normalizeCertificateIdentifier(request.Subject.CommonName)
		if err != nil {
			return io.ErrUnexpectedEOF
		}
		requested[normalized] = struct{}{}
	}
	if len(requested) != len(domains) {
		return io.ErrUnexpectedEOF
	}
	for _, domain := range domains {
		if _, exists := requested[domain]; !exists {
			return io.ErrUnexpectedEOF
		}
	}
	return nil
}
