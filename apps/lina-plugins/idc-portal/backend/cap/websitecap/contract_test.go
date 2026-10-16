// This file checks renderer and provider-only website contract boundaries.
package websitecap

import (
	"lina-core/pkg/plugin/capability/capmodel"
	"testing"
)

// TestWebsiteContract rejects executable methods and unknown renderers.
func TestWebsiteContract(t *testing.T) {
	d := &capmodel.BusinessExtensionDeclaration{Point: Point, Version: Version, Owner: Owner, Role: capmodel.BusinessExtensionRoleProvider, Methods: []string{MethodDescribe}, InterfaceConfigurationCardinality: capmodel.InterfaceConfigurationCardinalitySingleton, InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{}, ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Type: MetadataType, Version: Version, Values: map[string]any{"renderer": Renderer}}}
	if err := ValidateDeclarations([]*capmodel.BusinessExtensionDeclaration{d}); err != nil {
		t.Fatal(err)
	}
	d.ProviderMetadata.Values["renderer"] = MinimalRenderer
	if err := ValidateDeclarations([]*capmodel.BusinessExtensionDeclaration{d}); err != nil {
		t.Fatal(err)
	}
	d.Methods = []string{"run"}
	if err := ValidateDeclarations([]*capmodel.BusinessExtensionDeclaration{d}); err == nil {
		t.Fatal("executable methods accepted")
	}
	d.Methods = []string{MethodDescribe}
	d.ProviderMetadata.Values["renderer"] = []string{"evil"}
	if err := ValidateDeclarations([]*capmodel.BusinessExtensionDeclaration{d}); err == nil {
		t.Fatal("unknown renderer accepted")
	}
}
