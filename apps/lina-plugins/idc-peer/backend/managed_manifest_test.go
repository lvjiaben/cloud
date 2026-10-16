// These tests keep the uninstalled connector declaration aligned with both
// domain-owner contracts before a managed artifact can be packaged.
package backend

import (
	"gopkg.in/yaml.v3"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"os"
	"slices"
	"testing"
)

func TestManagedManifestOwnerContracts(t *testing.T) {
	raw, err := os.ReadFile("../plugin.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		ID           string                                   `yaml:"id"`
		Type         string                                   `yaml:"type"`
		Distribution string                                   `yaml:"distribution"`
		Declarations []*capmodel.BusinessExtensionDeclaration `yaml:"business_extensions"`
	}
	if yaml.Unmarshal(raw, &manifest) != nil || manifest.ID != pluginID || manifest.Type != "dynamic" || manifest.Distribution != "managed" {
		t.Fatal("managed identity is invalid")
	}
	var fulfillment, resource []*capmodel.BusinessExtensionDeclaration
	for _, declaration := range manifest.Declarations {
		switch declaration.Point {
		case capmodel.BusinessExtensionProductFulfillment:
			fulfillment = append(fulfillment, declaration)
		case resourcecap.DynamicProviderPoint:
			resource = append(resource, declaration)
		}
	}
	if err = productfulfillmentcap.ValidateProviderDeclarations(fulfillment); err != nil {
		t.Fatal(err)
	}
	ownerRaw, err := os.ReadFile("../../idc-core/plugin.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var owner struct {
		Declarations []*capmodel.BusinessExtensionDeclaration `yaml:"business_extensions"`
	}
	if err = yaml.Unmarshal(ownerRaw, &owner); err != nil {
		t.Fatal(err)
	}
	var methods []string
	for _, declaration := range owner.Declarations {
		if declaration.Point == capmodel.BusinessExtensionProductFulfillment && declaration.Role == capmodel.BusinessExtensionRoleOwner {
			methods = declaration.Methods
		}
	}
	for _, method := range fulfillment[0].Methods {
		if !slices.Contains(methods, method) {
			t.Fatalf("owner does not publish connector method %s", method)
		}
	}
	if err = resourcecap.ValidateDynamicProviderDeclarations(resource); err != nil {
		t.Fatal(err)
	}
	resourceOwnerRaw, err := os.ReadFile("../../idc-resource/plugin.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err = yaml.Unmarshal(resourceOwnerRaw, &owner); err != nil {
		t.Fatal(err)
	}
	methods = nil
	for _, declaration := range owner.Declarations {
		if declaration.Point == resourcecap.DynamicProviderPoint && declaration.Role == capmodel.BusinessExtensionRoleOwner {
			methods = declaration.Methods
		}
	}
	for _, method := range resource[0].Methods {
		if !slices.Contains(methods, method) {
			t.Fatalf("resource owner does not publish connector method %s", method)
		}
	}
	declaration, err := productfulfillmentcap.ParseProviderDeclaration(fulfillment[0].ProviderMetadata, fulfillment[0].Methods)
	if err != nil || declaration.EntitlementOwnerID != "idc-peer" {
		t.Fatal("same-system products must not create upstream terminal customers or fake server ownership")
	}
}
