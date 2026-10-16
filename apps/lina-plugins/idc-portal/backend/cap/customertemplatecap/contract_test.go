package customertemplatecap

import (
	"lina-core/pkg/plugin/capability/capmodel"
	"testing"
)

// TestCustomerStaticTemplateDeclaration covers customer routes and framework-neutral entries.
func TestCustomerStaticTemplateDeclaration(t *testing.T) {
	metadata := &capmodel.BusinessExtensionProviderMetadata{Type: MetadataType, Version: Version, Values: map[string]any{"entry": "index.html", "spa": true, "pages": []Page{{ID: "login", NameZh: "Login", NameEn: "Login", Path: "/auth/login"}, {ID: "orders", NameZh: "Orders", NameEn: "Orders", Path: "/account/orders"}}}}
	declaration := &capmodel.BusinessExtensionDeclaration{Point: Point, Version: Version, Owner: Owner, Role: capmodel.BusinessExtensionRoleProvider, Methods: []string{MethodDescribe}, InterfaceConfigurationCardinality: capmodel.InterfaceConfigurationCardinalitySingleton, InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: Version}, ProviderMetadata: metadata}
	if err := ValidateDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/admin/users", "/api/v1/orders", "/x/idc-core/api/v1/account/orders", "//outside.test", "/account/../admin"} {
		metadata.Values["pages"] = []Page{{ID: "invalid", NameZh: "Invalid", NameEn: "Invalid", Path: path}}
		if err := ValidateDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration}); err == nil {
			t.Fatalf("accepted unsafe customer route: %s", path)
		}
	}
}
