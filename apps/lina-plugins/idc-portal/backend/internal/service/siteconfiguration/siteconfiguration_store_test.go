// This file verifies installed-provider gating, independent settings and persistence.
package siteconfiguration

import (
	"context"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-portal/backend/cap/websitecap"
	"testing"
)

// registryFixture implements only the discovery and admission paths under test.
type registryFixture struct {
	pluginhost.BusinessExtensionRegistrar
	items []*capmodel.BusinessExtensionProviderEvidence
	lists int
}

func (r *registryFixture) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	r.lists++
	return &capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence]{Items: r.items, Total: len(r.items)}, nil
}
func (r *registryFixture) WithNewBusinessAdmission(ctx context.Context, input pluginhost.BusinessExtensionAdmissionInput, action capmodel.BusinessExtensionAdmissionAction) error {
	for _, item := range r.items {
		if item.PluginID == input.PluginID {
			return action(ctx, item)
		}
	}
	return nil
}

// TestTemplateSettingsPersistence verifies changing templates does not overwrite independent fields.
func TestTemplateSettingsPersistence(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	schema := &capmodel.ConfigurationSchemaDeclaration{Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "herotitle", Type: capmodel.ConfigurationFieldSingleLine}}}
	r := &registryFixture{items: []*capmodel.BusinessExtensionProviderEvidence{{PluginID: "a", InterfaceConfiguration: schema, ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Values: map[string]any{"renderer": websitecap.Renderer}}}, {PluginID: "b", InterfaceConfiguration: schema, ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Values: map[string]any{"renderer": websitecap.MinimalRenderer}}}}}
	service, err := New(r, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		if err = service.Save(ctx, Settings{SelectedPlugin: id, Common: Common{SiteName: "Shop", CompanyName: "Example Ltd", ICPNumber: "ICP 123", ICPURL: "https://records.example/icp", PoliceNumber: "Police 456", PoliceURL: "https://records.example/police"}, Config: map[string]any{"herotitle": id}}); err != nil {
			t.Fatal(err)
		}
	}
	restarted, err := New(r, root)
	if err != nil {
		t.Fatal(err)
	}
	view, err := restarted.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.Configurations["a"]["herotitle"] != "a" || view.Configurations["b"]["herotitle"] != "b" {
		t.Fatalf("settings mixed: %#v", view)
	}
	if view.Common.ICPNumber != "ICP 123" || view.Common.PoliceNumber != "Police 456" || view.Common.CompanyName != "Example Ltd" {
		t.Fatal("shared settings did not survive restart and template switch")
	}
	before := r.lists
	public, active, err := restarted.Public(ctx)
	if err != nil || !active || public.Renderer != websitecap.MinimalRenderer || r.lists-before != 1 {
		t.Fatalf("public: %#v %v %v", public, active, err)
	}
	r.items = nil
	if _, active, err = restarted.Public(ctx); err != nil || active {
		t.Fatal("uninstalled template still public")
	}
	if err = restarted.Save(ctx, Settings{SelectedPlugin: "b", Common: Common{SiteName: "Shop"}}); err == nil {
		t.Fatal("uninstalled template accepted")
	}
}

// TestInvalidPublicContent verifies nested payloads and unsafe image URLs are rejected without panic.
func TestInvalidPublicContent(t *testing.T) {
	schema := &capmodel.ConfigurationSchemaDeclaration{Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "heroimage", Type: capmodel.ConfigurationFieldSingleLine}}}
	for _, value := range []any{[]any{"x"}, map[string]any{"x": "y"}, "javascript:alert(1)", "//evil.example/a"} {
		if err := validate(Settings{Common: Common{SiteName: "Shop"}, Config: map[string]any{"heroimage": value}}, schema); err == nil {
			t.Fatalf("accepted %#v", value)
		}
	}
}

// TestNormalizedImageAndColor validates normalized provider fields before public projection.
func TestNormalizedImageAndColor(t *testing.T) {
	schema := &capmodel.ConfigurationSchemaDeclaration{Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "heroimage", Type: capmodel.ConfigurationFieldSingleLine},
		{Key: "primarycolor", Type: capmodel.ConfigurationFieldSingleLine},
	}}
	for _, image := range []string{"/banner.png", "https://example.test/banner.png"} {
		if err := validate(Settings{Common: Common{SiteName: "Shop"}, Config: map[string]any{"heroimage": image, "primarycolor": "#123aBC"}}, schema); err != nil {
			t.Fatalf("valid normalized fields rejected: %v", err)
		}
	}
	for _, color := range []string{"red", "#123", "url(javascript:alert(1))"} {
		if err := validate(Settings{Common: Common{SiteName: "Shop"}, Config: map[string]any{"primarycolor": color}}, schema); err == nil {
			t.Fatalf("unsafe color accepted: %q", color)
		}
	}
}

// TestRegistrationLinks rejects executable or authority-confusing destinations.
func TestRegistrationLinks(t *testing.T) {
	for _, value := range []string{"javascript:alert(1)", "//evil.example", "https://user:password@example.com", "https://example.com\\evil", "https://example.com\r\n"} {
		if safeRegistrationLink(value) {
			t.Fatalf("accepted unsafe link %q", value)
		}
	}
	for _, value := range []string{"", "https://records.example/query?id=123"} {
		if !safeRegistrationLink(value) {
			t.Fatalf("rejected safe link %q", value)
		}
	}
}
