package wasmbuilder

import (
	"lina-core/pkg/plugin/capability/capmodel"
	"strings"
	"testing"
)

func TestValidatePackagedFrontendAssets(t *testing.T) {
	for _, test := range []struct {
		name   string
		assets []*frontendAsset
		entry  string
		mount  string
		want   string
	}{
		{"empty declared source", nil, "admin/index.html", "/", "no packaged assets"},
		{"missing admin entry", []*frontendAsset{{Path: "frontend/dist/customer/index.html"}}, "admin/index.html", "/", "missing packaged entry"},
		{"built entry", []*frontendAsset{{Path: "frontend/dist/admin/index.html"}}, "admin/index.html", "/", ""},
		{"mounted entry", []*frontendAsset{{Path: "frontend/dist/admin/index.html"}}, "certificates/admin/index.html", "/certificates", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest := &pluginManifest{PublicAssets: []*publicAssetSpec{{Source: "frontend/dist", Mount: test.mount}}, AdminSurfaces: []*capmodel.FrontendSurfaceDeclaration{{Key: "certificates", Entry: test.entry}}}
			err := validatePackagedFrontendAssets(manifest, test.assets)
			if test.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
			}
		})
	}
}
