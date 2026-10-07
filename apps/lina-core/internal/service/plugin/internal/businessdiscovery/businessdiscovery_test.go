package businessdiscovery

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"lina-core/internal/service/plugin/internal/plugintypes"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
)

func TestMain(m *testing.M) {
	cleanups := make([]func(), 0, 3)
	for _, owner := range []string{"idc-core", "other"} {
		declarations := pluginhost.NewDeclarations(owner)
		for _, version := range []string{"v1", "v2"} {
			if err := declarations.BusinessExtensions().RegisterProviderValidator(capmodel.BusinessExtensionProductFulfillment, version, func([]*capmodel.BusinessExtensionDeclaration) error { return nil }); err != nil {
				panic(err)
			}
		}
		cleanup, err := pluginhost.RegisterSourcePluginForTest(declarations)
		if err != nil {
			panic(err)
		}
		cleanups = append(cleanups, cleanup)
	}
	notificationOwner := pluginhost.NewDeclarations("notification-core")
	if err := notificationOwner.BusinessExtensions().RegisterProviderValidator(
		capmodel.BusinessExtensionSMSChannel,
		"v1",
		func([]*capmodel.BusinessExtensionDeclaration) error { return nil },
	); err != nil {
		panic(err)
	}
	notificationCleanup, err := pluginhost.RegisterSourcePluginForTest(notificationOwner)
	if err != nil {
		panic(err)
	}
	cleanups = append(cleanups, notificationCleanup)
	code := m.Run()
	for index := len(cleanups) - 1; index >= 0; index-- {
		cleanups[index]()
	}
	os.Exit(code)
}

func TestListNewBindingProvidersReadsCollectionOnceAndFailsClosedAboveBound(t *testing.T) {
	rows := make([]*activeReleaseRow, plugincap.MaxBusinessExtensionProviderScan+1)
	queryCount := 0
	page, err := listNewBindingProviders(context.Background(), "idc-core", plugincap.BusinessExtensionProviderListInput{
		Point: capmodel.BusinessExtensionProductFulfillment,
	}, func(_ context.Context, limit int) ([]*activeReleaseRow, error) {
		queryCount++
		if limit != plugincap.MaxBusinessExtensionProviderScan+1 {
			t.Fatalf("expected bounded probe limit %d, got %d", plugincap.MaxBusinessExtensionProviderScan+1, limit)
		}
		return rows, nil
	})
	if err == nil || page != nil {
		t.Fatalf("501 eligible active snapshots must fail closed: page=%#v err=%v", page, err)
	}
	if queryCount != 1 {
		t.Fatalf("candidate discovery must use exactly one collection read, got %d", queryCount)
	}
}

func TestEvidenceFromSnapshotFiltersOwnerPointVersionAndSortsMethods(t *testing.T) {
	snapshot, err := yaml.Marshal(manifestSnapshot{ID: "ssl-provider", Name: "SSL Provider", Dependencies: providerDependencies("idc-core", "other"), AuthorizedBusinessExtensions: []*capmodel.BusinessExtensionDeclaration{
		{Point: capmodel.BusinessExtensionProductFulfillment, Version: "v2", Role: capmodel.BusinessExtensionRoleProvider, Owner: "idc-core", Methods: []string{"fulfill", "query"}, ProviderMetadata: frozenProviderMetadata()},
		{Point: capmodel.BusinessExtensionProductFulfillment, Version: "v1", Role: capmodel.BusinessExtensionRoleProvider, Owner: "other", Methods: []string{"fulfill", "query"}, ProviderMetadata: frozenProviderMetadata()},
	}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := evidenceFromSnapshot("ssl-provider", "", "v3.2.1", "release-sha", string(snapshot), "idc-core", capmodel.BusinessExtensionProductFulfillment, "v2")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !reflect.DeepEqual(items[0].AuthorizedMethods, []string{"fulfill", "query"}) {
		t.Fatalf("unexpected evidence: %#v", items)
	}
	if items[0].DeclarationDigest == "" || items[0].ReleaseDigest != "release-sha" || items[0].DisplayName != "SSL Provider" {
		t.Fatalf("missing frozen digests: %#v", items[0])
	}
}

func TestEvidenceFromSnapshotFailsClosedOnCorruption(t *testing.T) {
	if _, err := evidenceFromSnapshot("broken-provider", "Broken", "v1.0.0", "digest", "businessExtensions: [", "idc-core", "product-fulfillment", ""); err == nil {
		t.Fatal("corrupt active snapshot must fail closed")
	}
	if _, err := evidenceFromSnapshot("missing-digest", "Missing", "v1.0.0", "", "{}", "idc-core", "product-fulfillment", ""); err == nil {
		t.Fatal("missing release digest must fail closed")
	}
}

func TestEvidenceFromSnapshotRejectsSemanticallyCorruptDeclarations(t *testing.T) {
	for _, methods := range [][]string{{"Fulfill"}, {"fulfill", "fulfill"}, {" fulfill"}} {
		snapshot, err := yaml.Marshal(manifestSnapshot{ID: "phone-provider", Name: "Phone Provider", Dependencies: providerDependencies("idc-core"), AuthorizedBusinessExtensions: []*capmodel.BusinessExtensionDeclaration{{
			Point: "product-fulfillment", Version: "v1", Role: "provider", Owner: "idc-core", Methods: methods,
		}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = evidenceFromSnapshot("phone-provider", "Phone Provider", "v1.0.0", "release", string(snapshot), "idc-core", "product-fulfillment", "v1"); err == nil {
			t.Fatalf("semantically corrupt methods must fail closed: %#v", methods)
		}
	}
}

func providerDependencies(owners ...string) *plugintypes.DependencySpec {
	dependencies := &plugintypes.DependencySpec{Plugins: make([]*plugintypes.PluginDependencySpec, 0, len(owners))}
	for _, owner := range owners {
		dependencies.Plugins = append(dependencies.Plugins, &plugintypes.PluginDependencySpec{ID: owner, Version: ">=v0.1.0 <v1.0.0"})
	}
	return dependencies
}

func TestDeclarationDigestIsOrderIndependentAfterNormalization(t *testing.T) {
	a := []string{"fulfill", "query"}
	b := []string{"query", "fulfill"}
	// Production parsing sorts methods before hashing.
	sort.Strings(b)
	left := &capmodel.BusinessExtensionDeclaration{Owner: "idc-core", Point: "product-fulfillment", Version: "v1", Role: capmodel.BusinessExtensionRoleProvider, Methods: a}
	right := &capmodel.BusinessExtensionDeclaration{Owner: "idc-core", Point: "product-fulfillment", Version: "v1", Role: capmodel.BusinessExtensionRoleProvider, Methods: b}
	left.ProviderMetadata = frozenProviderMetadata()
	right.ProviderMetadata = frozenProviderMetadata()
	if declarationDigest(left) != declarationDigest(right) {
		t.Fatal("normalized method order must produce stable evidence")
	}
}

func TestDeclarationDigestAndEvidenceFreezeConfiguration(t *testing.T) {
	declaration := &capmodel.BusinessExtensionDeclaration{
		Point: "product-fulfillment", Version: "v1", Role: "provider", Owner: "idc-core",
		Methods: []string{"candidates.list", "fulfill", "query"}, ProviderMetadata: frozenProviderMetadata(),
		InterfaceConfigurationCardinality: capmodel.InterfaceConfigurationCardinalityMultiple,
		InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
			Key: "endpoint", Type: capmodel.ConfigurationFieldSingleLine,
			Label: &capmodel.LocalizedText{EnUS: "Endpoint", ZhCN: "接口地址"},
		}}},
		ProductConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
			Key: "enabled", Type: capmodel.ConfigurationFieldBoolean,
			Label: &capmodel.LocalizedText{EnUS: "Enabled", ZhCN: "启用"},
		}}},
		CustomerConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
			Key: "color", Type: capmodel.ConfigurationFieldSingleSelect,
			Label:             &capmodel.LocalizedText{EnUS: "Color", ZhCN: "颜色"},
			DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidates.list"},
		}}},
	}
	snapshot, err := yaml.Marshal(manifestSnapshot{ID: "phone-provider", Name: "Phone Provider", Dependencies: providerDependencies("idc-core"), AuthorizedBusinessExtensions: []*capmodel.BusinessExtensionDeclaration{declaration}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := evidenceFromSnapshot("phone-provider", "Phone Provider", "v1.0.0", "release", string(snapshot), "idc-core", "product-fulfillment", "v1")
	if err != nil || len(items) != 1 || items[0].InterfaceConfiguration == nil ||
		items[0].InterfaceConfigurationCardinality != capmodel.InterfaceConfigurationCardinalityMultiple || items[0].ProviderMetadata == nil ||
		items[0].ProductConfiguration == nil || items[0].CustomerConfiguration == nil {
		t.Fatalf("configuration evidence: items=%#v err=%v", items, err)
	}
	before := items[0].DeclarationDigest
	declaration.ProviderMetadata.Values["granularity"] = "per_quantity_unit"
	if before == declarationDigest(declaration) {
		t.Fatal("provider metadata changes must change the declaration digest")
	}
}

func TestEvidenceEmptyMethodsStayNonNilAndEncodeAsJSONArray(t *testing.T) {
	snapshot, err := yaml.Marshal(manifestSnapshot{ID: "manual-provider", Name: "Manual Provider", Dependencies: providerDependencies("notification-core"), AuthorizedBusinessExtensions: []*capmodel.BusinessExtensionDeclaration{{
		Point: "sms-channel", Version: "v1", Role: "provider", Owner: "notification-core",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	items, err := evidenceFromSnapshot("manual-provider", "Manual Provider", "v1.0.0", "release", string(snapshot), "notification-core", "sms-channel", "v1")
	if err != nil || len(items) != 1 {
		t.Fatalf("empty method evidence: items=%#v err=%v", items, err)
	}
	if items[0].AuthorizedMethods == nil {
		t.Fatal("empty authorized methods must be a non-nil slice")
	}
	encoded, err := json.Marshal(items[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"authorizedMethods":[]`) {
		t.Fatalf("empty methods must encode as JSON array: %s", encoded)
	}
}

func TestGetExactProvidersReadsMultipleProviderReleasesOnce(t *testing.T) {
	inputs := make([]plugincap.BusinessExtensionProviderExactInput, 0, 2)
	rows := make([]*historicalReleaseRow, 0, 2)
	for _, pluginID := range []string{"phone-provider", "ssl-provider"} {
		snapshot, err := yaml.Marshal(manifestSnapshot{ID: pluginID, Name: pluginID, Dependencies: providerDependencies("idc-core"), AuthorizedBusinessExtensions: []*capmodel.BusinessExtensionDeclaration{{
			Point: capmodel.BusinessExtensionProductFulfillment, Version: "v1", Role: capmodel.BusinessExtensionRoleProvider, Owner: "idc-core", Methods: []string{"fulfill", "query"}, ProviderMetadata: frozenProviderMetadata(),
		}}})
		if err != nil {
			t.Fatal(err)
		}
		items, err := evidenceFromSnapshot(pluginID, "", "v1.0.0", pluginID+"-release", string(snapshot), "idc-core", capmodel.BusinessExtensionProductFulfillment, "v1")
		if err != nil || len(items) != 1 {
			t.Fatalf("prepare %s evidence: items=%#v err=%v", pluginID, items, err)
		}
		inputs = append(inputs, plugincap.BusinessExtensionProviderExactInput{PluginID: pluginID, ReleaseVersion: "v1.0.0", Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1", ReleaseDigest: pluginID + "-release", DeclarationDigest: items[0].DeclarationDigest})
		rows = append(rows, &historicalReleaseRow{PluginID: pluginID, ReleaseVersion: "v1.0.0", ReleaseDigest: pluginID + "-release", Snapshot: string(snapshot)})
	}
	queryCount := 0
	result, err := getExactProviders(context.Background(), "idc-core", inputs, func(_ context.Context, keys []historicalReleaseKey) ([]*historicalReleaseRow, error) {
		queryCount++
		if len(keys) != 2 {
			t.Fatalf("expected two exact release keys, got %#v", keys)
		}
		return rows, nil
	})
	if err != nil || len(result) != 2 || result[0] == nil || result[1] == nil || result[0].PluginID != "phone-provider" || result[1].PluginID != "ssl-provider" {
		t.Fatalf("batch exact evidence: result=%#v err=%v", result, err)
	}
	if queryCount != 1 {
		t.Fatalf("multiple providers must use one release collection query, got %d", queryCount)
	}
}

func TestHistoricalExactEvidenceKeepsOldProviderMetadataAcrossUpgrade(t *testing.T) {
	oldDeclaration := &capmodel.BusinessExtensionDeclaration{
		Point: capmodel.BusinessExtensionProductFulfillment, Version: "v1", Role: capmodel.BusinessExtensionRoleProvider,
		Owner: "idc-core", Methods: []string{"fulfill", "query"}, ProviderMetadata: frozenProviderMetadata(),
	}
	newDeclaration := catalogCloneForUpgrade(oldDeclaration)
	newDeclaration.ProviderMetadata.Values["granularity"] = "per_quantity_unit"
	oldSnapshot, err := yaml.Marshal(manifestSnapshot{ID: "phone-provider", Name: "Phone Provider", Dependencies: providerDependencies("idc-core"), AuthorizedBusinessExtensions: []*capmodel.BusinessExtensionDeclaration{oldDeclaration}})
	if err != nil {
		t.Fatal(err)
	}
	newSnapshot, err := yaml.Marshal(manifestSnapshot{ID: "phone-provider", Name: "Phone Provider", Dependencies: providerDependencies("idc-core"), AuthorizedBusinessExtensions: []*capmodel.BusinessExtensionDeclaration{newDeclaration}})
	if err != nil {
		t.Fatal(err)
	}
	oldEvidence, err := evidenceFromSnapshot("phone-provider", "", "v1.0.0", "old-release", string(oldSnapshot), "idc-core", capmodel.BusinessExtensionProductFulfillment, "v1")
	if err != nil || len(oldEvidence) != 1 {
		t.Fatalf("old evidence: items=%#v err=%v", oldEvidence, err)
	}
	newEvidence, err := evidenceFromSnapshot("phone-provider", "", "v2.0.0", "new-release", string(newSnapshot), "idc-core", capmodel.BusinessExtensionProductFulfillment, "v1")
	if err != nil || len(newEvidence) != 1 || oldEvidence[0].DeclarationDigest == newEvidence[0].DeclarationDigest {
		t.Fatalf("upgraded evidence did not change digest: old=%#v new=%#v err=%v", oldEvidence, newEvidence, err)
	}
	input := plugincap.BusinessExtensionProviderExactInput{PluginID: "phone-provider", ReleaseVersion: "v1.0.0", Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1", ReleaseDigest: "old-release", DeclarationDigest: oldEvidence[0].DeclarationDigest}
	result, err := getExactProviders(context.Background(), "idc-core", []plugincap.BusinessExtensionProviderExactInput{input}, func(context.Context, []historicalReleaseKey) ([]*historicalReleaseRow, error) {
		return []*historicalReleaseRow{{PluginID: "phone-provider", ReleaseVersion: "v1.0.0", ReleaseDigest: "old-release", Snapshot: string(oldSnapshot)}}, nil
	})
	if err != nil || len(result) != 1 || result[0] == nil || result[0].ProviderMetadata.Values["granularity"] != "per_order_item" {
		t.Fatalf("historical release contract drifted after upgrade: result=%#v err=%v", result, err)
	}
	input.DeclarationDigest = newEvidence[0].DeclarationDigest
	result, err = getExactProviders(context.Background(), "idc-core", []plugincap.BusinessExtensionProviderExactInput{input}, func(context.Context, []historicalReleaseKey) ([]*historicalReleaseRow, error) {
		return []*historicalReleaseRow{{PluginID: "phone-provider", ReleaseVersion: "v1.0.0", ReleaseDigest: "old-release", Snapshot: string(oldSnapshot)}}, nil
	})
	if err != nil || result[0] != nil {
		t.Fatalf("new declaration digest matched an old release: result=%#v err=%v", result, err)
	}
}

func catalogCloneForUpgrade(value *capmodel.BusinessExtensionDeclaration) *capmodel.BusinessExtensionDeclaration {
	clone := *value
	clone.Methods = append([]string(nil), value.Methods...)
	if value.ProviderMetadata != nil {
		metadata := *value.ProviderMetadata
		metadata.Values = make(map[string]any, len(value.ProviderMetadata.Values))
		for key, item := range value.ProviderMetadata.Values {
			metadata.Values[key] = item
		}
		clone.ProviderMetadata = &metadata
	}
	return &clone
}

func frozenProviderMetadata() *capmodel.BusinessExtensionProviderMetadata {
	return &capmodel.BusinessExtensionProviderMetadata{
		Type: "delivery-contract", Version: "v1", Values: map[string]any{
			"capability":  map[string]any{"enUS": "Automated fulfillment", "zhCN": "自动履约"},
			"granularity": "per_order_item", "methodRoles": map[string]any{"provision": "fulfill", "query": "query"},
		},
	}
}
