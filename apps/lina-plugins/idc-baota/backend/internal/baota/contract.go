package baota

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
)

var errInvalidConfiguration = errors.New("BT Panel delivery configuration is invalid")
var domainPattern = regexp.MustCompile(`^(\*\.)?[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?(?::[1-9][0-9]{0,4})?$`)
var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,64}$`)

type ProductConfiguration struct {
	DeploymentType string `json:"deployment_type"`
	PHPVersion     string `json:"php_version,omitempty"`
}
type CustomerConfiguration struct {
	PrimaryDomain     string `json:"primary_domain"`
	AdditionalDomains string `json:"additional_domains,omitempty"`
}
type FrozenSitePlan struct {
	Version        string   `json:"version"`
	DeploymentType string   `json:"deploymentType"`
	Domains        []string `json:"domains"`
	Path           string   `json:"path"`
	PHPVersion     string   `json:"phpVersion"`
}

func BuildSitePlan(productJSON, customerJSON json.RawMessage) (json.RawMessage, error) {
	var product ProductConfiguration
	var customer CustomerConfiguration
	if strict(productJSON, &product) != nil || strict(customerJSON, &customer) != nil || (product.DeploymentType != "php_site" && product.DeploymentType != "static_site") {
		return nil, errInvalidConfiguration
	}
	if product.DeploymentType == "php_site" {
		if !identifierPattern.MatchString(product.PHPVersion) {
			return nil, errInvalidConfiguration
		}
	} else if product.PHPVersion != "" {
		return nil, errInvalidConfiguration
	}
	domains := []string{strings.TrimSpace(customer.PrimaryDomain)}
	if strings.HasPrefix(domains[0], "*.") {
		return nil, errInvalidConfiguration
	}
	for _, item := range strings.Fields(customer.AdditionalDomains) {
		domains = append(domains, item)
	}
	seen := map[string]struct{}{}
	for _, domain := range domains {
		if !domainPattern.MatchString(domain) {
			return nil, errInvalidConfiguration
		}
		key := strings.ToLower(domain)
		if _, ok := seen[key]; ok {
			return nil, errInvalidConfiguration
		}
		seen[key] = struct{}{}
	}
	primary := strings.TrimPrefix(strings.Split(domains[0], ":")[0], "*.")
	php := product.PHPVersion
	if product.DeploymentType == "static_site" {
		php = "00"
	}
	return json.Marshal(FrozenSitePlan{Version: "v1", DeploymentType: product.DeploymentType, Domains: domains, Path: "/www/wwwroot/" + primary, PHPVersion: php})
}

func strict(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errInvalidConfiguration
	}
	return nil
}
