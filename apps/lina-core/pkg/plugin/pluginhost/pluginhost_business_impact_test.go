package pluginhost

import (
	"context"
	"testing"
)

func TestBusinessImpactHandlerRegistrationAndDiscovery(t *testing.T) {
	plugin := NewDeclarations("business-impact-owner-test")
	handler := func(
		_ context.Context,
		input PluginBusinessImpactInput,
	) ([]PluginBusinessImpactItem, error) {
		if input.TargetPluginID != "target-provider" {
			t.Fatalf("unexpected target plugin id: %s", input.TargetPluginID)
		}
		return []PluginBusinessImpactItem{{
			Category: BusinessImpactActiveServices, TotalCount: 2, BlockingCount: 1,
		}}, nil
	}
	if err := plugin.Lifecycle().RegisterBusinessImpactHandler(handler); err != nil {
		t.Fatalf("expected business impact registration to succeed: %v", err)
	}
	if err := plugin.Lifecycle().RegisterBusinessImpactHandler(handler); err == nil {
		t.Fatal("expected duplicate business impact registration to fail")
	}

	cleanup, err := RegisterSourcePluginForTest(plugin)
	if err != nil {
		t.Fatalf("expected test source plugin registration to succeed: %v", err)
	}
	t.Cleanup(cleanup)

	var found SourcePluginDefinition
	for _, candidate := range ListSourcePluginBusinessImpactProviders() {
		if candidate.ID() == plugin.ID() {
			found = candidate
			break
		}
	}
	if found == nil {
		t.Fatal("expected registered business owner in provider discovery")
	}
	items, err := found.GetBusinessImpactHandler()(
		context.Background(),
		PluginBusinessImpactInput{TargetPluginID: "target-provider"},
	)
	if err != nil || len(items) != 1 || items[0].BlockingCount != 1 {
		t.Fatalf("unexpected business impact result: items=%#v err=%v", items, err)
	}
}

func TestBusinessImpactCategoriesAreBounded(t *testing.T) {
	supported := []BusinessImpactCategory{
		BusinessImpactSellingProductVersions,
		BusinessImpactInFlightTasks,
		BusinessImpactActiveServices,
		BusinessImpactPendingHistoricalActions,
	}
	for _, category := range supported {
		if !IsSupportedBusinessImpactCategory(category) {
			t.Fatalf("expected category %q to be supported", category)
		}
	}
	if IsSupportedBusinessImpactCategory("arbitrary_private_table") {
		t.Fatal("expected arbitrary impact category to be rejected")
	}
}
