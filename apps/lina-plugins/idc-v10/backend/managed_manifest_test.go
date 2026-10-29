// This file validates the connector's actual declaration against both owners;
// a recognized module protocol cannot bypass the independent server contract.
package backend

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestManagedV10ManifestDeclaresNativeCommonProtocol(t *testing.T) {
	raw, err := os.ReadFile("../plugin.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		BusinessExtensions []*capmodel.BusinessExtensionDeclaration `yaml:"business_extensions"`
	}
	if yaml.Unmarshal(raw, &manifest) != nil {
		t.Fatal("invalid managed manifest")
	}
	var products, resources []*capmodel.BusinessExtensionDeclaration
	for _, declaration := range manifest.BusinessExtensions {
		switch declaration.Point {
		case capmodel.BusinessExtensionProductFulfillment:
			products = append(products, declaration)
		case resourcecap.DynamicProviderPoint:
			resources = append(resources, declaration)
		}
	}
	if len(products) != 1 || len(resources) != 1 {
		t.Fatal("missing owner declarations")
	}
	if err = productfulfillmentcap.ValidateProviderDeclarations(products); err != nil {
		t.Fatal(err)
	}
	if err = resourcecap.ValidateDynamicProviderDeclarations(resources); err != nil {
		t.Fatal(err)
	}
	evidence := &capmodel.BusinessExtensionProviderEvidence{Point: products[0].Point, ContractVersion: products[0].Version, ProductConfiguration: products[0].ProductConfiguration, CustomerConfiguration: products[0].CustomerConfiguration, AuthorizedMethods: products[0].Methods, ProviderMetadata: products[0].ProviderMetadata}
	if err = resourcecap.ValidateNativeProductEvidence(evidence); err != nil {
		t.Fatal(err)
	}
	if resourcecap.ValidateServerProductEvidence(evidence) == nil {
		t.Fatal("native product unexpectedly passed server hardware admission")
	}
	declaration, err := productfulfillmentcap.ParseProviderDeclaration(evidence.ProviderMetadata, evidence.AuthorizedMethods)
	if err != nil || declaration.SaleImportProtocol != productfulfillmentcap.SaleImportProtocolNativeV10CommonV1 {
		t.Fatal("V10 native protocol is not explicit")
	}
}
