// These tests keep public template reads independent of administrative candidate
// projection while preserving current-provider failure and enablement semantics.
package siteconfiguration

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-portal/backend/cap/websitecap"
)

type publicReadRegistry struct {
	registryFixture
	failure error
	pages   [][]*capmodel.BusinessExtensionProviderEvidence
}

func (r *publicReadRegistry) ListProviders(_ context.Context, input plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	r.lists++
	if r.failure != nil {
		return nil, r.failure
	}
	if r.pages != nil {
		return &capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence]{Items: r.pages[input.Page.PageNum-1], Total: 500}, nil
	}
	return &capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence]{Items: r.items, Total: len(r.items)}, nil
}
func TestPublicReadUsesSelectedTemplateOnly(t *testing.T) {
	ctx := context.Background()
	r := &publicReadRegistry{failure: errors.New("discovery unavailable")}
	service, err := New(r, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, active, readErr := service.Public(ctx); readErr != nil || active || r.lists != 0 {
		t.Fatalf("unselected read queried discovery: active=%v err=%v calls=%d", active, readErr, r.lists)
	}
	selected := &capmodel.BusinessExtensionProviderEvidence{PluginID: "selected", InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{}, ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Values: map[string]any{"renderer": websitecap.Renderer}}}
	r.failure = nil
	r.items = []*capmodel.BusinessExtensionProviderEvidence{selected}
	if err = service.Save(ctx, Settings{SelectedPlugin: "selected", Common: Common{SiteName: "Public shop"}, Config: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	// An unrelated candidate's malformed presentation must not be constructed.
	unrelated := &capmodel.BusinessExtensionProviderEvidence{PluginID: "unused", ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Values: map[string]any{"entry": "../unsafe.html", "pages": true}}}
	r.items = []*capmodel.BusinessExtensionProviderEvidence{unrelated, selected}
	before := r.lists
	result, active, err := service.Public(ctx)
	if err != nil || !active || result.SelectedPlugin != "selected" || result.Renderer != websitecap.Renderer || r.lists-before != 1 {
		t.Fatalf("public=%+v active=%v err=%v calls=%d", result, active, err, r.lists-before)
	}
	// A later page can contain the selected provider; stop immediately there.
	r.pages = [][]*capmodel.BusinessExtensionProviderEvidence{make([]*capmodel.BusinessExtensionProviderEvidence, 100), {selected}}
	before = r.lists
	if _, active, err = service.Public(ctx); err != nil || !active || r.lists-before != 2 {
		t.Fatalf("selected later page: active=%v err=%v calls=%d", active, err, r.lists-before)
	}
	r.pages = nil
	r.items = nil
	if _, active, err = service.Public(ctx); err != nil || active {
		t.Fatalf("disabled provider remained active: %v %v", active, err)
	}
	r.failure = errors.New("discovery unavailable")
	if _, active, err = service.Public(ctx); err == nil || active {
		t.Fatal("registry failure was hidden")
	}
}
