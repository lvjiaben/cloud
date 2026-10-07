package wasmbuilder

import (
	"encoding/json"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestBuildRuntimeArtifactPreservesBusinessExtensionsAndSeparateSurfaces(t *testing.T) {
	manifest := &pluginManifest{
		ID: "phone-supplier", Name: "Phone supplier", Version: "v0.1.0",
		Type: "dynamic", Distribution: "managed",
		BusinessExtensions: []*capmodel.BusinessExtensionDeclaration{{
			Point: capmodel.BusinessExtensionProductFulfillment, Version: "v1",
			Role: capmodel.BusinessExtensionRoleProvider, Owner: "commerce-core",
		}},
		AdminSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "phone-orders", Version: "v1", Title: "Phone orders", Slot: "commerce-services",
			Route: "phone-orders", Entry: "frontend/admin/index.html",
		}},
		CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "my-phone-orders", Version: "v1", Title: "My phone orders", Slot: "customer-services",
			Route: "phone-orders", Entry: "frontend/customer/index.html",
		}},
	}

	content, err := buildRuntimeArtifactContent(manifest, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "")
	if err != nil {
		t.Fatalf("build runtime artifact: %v", err)
	}
	sections, err := parseWasmCustomSections(content)
	if err != nil {
		t.Fatalf("parse runtime artifact sections: %v", err)
	}
	embedded := &dynamicArtifactManifest{}
	if err = json.Unmarshal(sections[pluginDynamicWasmSectionManifest], embedded); err != nil {
		t.Fatalf("decode embedded manifest: %v", err)
	}
	if embedded.Distribution != "managed" || len(embedded.BusinessExtensions) != 1 {
		t.Fatalf("runtime/distribution/business extension dimensions were not preserved: %#v", embedded)
	}
	if len(embedded.AdminSurfaces) != 1 || len(embedded.CustomerSurfaces) != 1 {
		t.Fatalf("admin/customer surfaces were not preserved separately: %#v", embedded)
	}
	if embedded.AdminSurfaces[0].Key == embedded.CustomerSurfaces[0].Key {
		t.Fatal("admin and customer surface declarations were collapsed")
	}
}
