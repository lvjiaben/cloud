// This file verifies compatible package resolution without changing saved facts
// and proves host calls receive the actual enabled release and method authority.
package providerexecution

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
)

type executionHost struct {
	pluginhost.BusinessExtensionRegistrar
	items      []*capmodel.BusinessExtensionProviderEvidence
	historical *capmodel.BusinessExtensionProviderEvidence
	reads      int
	admission  pluginhost.BusinessExtensionAdmissionInput
	invocation pluginhost.BusinessExtensionMethodInvokeInput
	outbound   pluginhost.ConfidentialOutboundContextInput
}

func (h *executionHost) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	h.reads++
	return &capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence]{Items: h.items}, nil
}
func (h *executionHost) GetProviderExact(context.Context, plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	if h.historical != nil {
		return h.historical, nil
	}
	return nil, errors.New("old package missing")
}
func (h *executionHost) GetProvidersExact(_ context.Context, inputs []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	if h.historical == nil {
		return nil, errors.New("old package missing")
	}
	result := make([]*capmodel.BusinessExtensionProviderEvidence, len(inputs))
	for index := range result {
		result[index] = h.historical
	}
	return result, nil
}
func (h *executionHost) WithNewBusinessAdmission(ctx context.Context, in pluginhost.BusinessExtensionAdmissionInput, fn capmodel.BusinessExtensionAdmissionAction) error {
	h.admission = in
	return fn(ctx, h.items[0])
}
func (h *executionHost) InvokeProviderMethod(_ context.Context, in pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	h.invocation = in
	return &capmodel.BusinessExtensionMethodResult{}, nil
}
func (h *executionHost) WithConfidentialOutboundContext(ctx context.Context, in pluginhost.ConfidentialOutboundContextInput, fn pluginhost.ConfidentialOutboundAction) error {
	h.outbound = in
	return fn(ctx, &capmodel.BusinessExtensionRuntimeContext{ContextID: "actual-current-context"})
}

func TestCompatiblePackageExecutionUsesActualHostEvidence(t *testing.T) {
	ctx := context.Background()
	old := plugincap.BusinessExtensionProviderExactInput{PluginID: "connector", Point: "product-fulfillment", ContractVersion: "v1", ReleaseVersion: "v1.0.0", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("d", 64)}
	current := &capmodel.BusinessExtensionProviderEvidence{PluginID: old.PluginID, Point: old.Point, ContractVersion: old.ContractVersion, ReleaseVersion: "v1.1.0", ReleaseDigest: strings.Repeat("b", 64), DeclarationDigest: old.DeclarationDigest, AuthorizedMethods: []string{"provision"}}
	host := &executionHost{items: []*capmodel.BusinessExtensionProviderEvidence{current}}
	runtime, err := New(host)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := runtime.GetProviderExact(ctx, old)
	if err != nil || actual != current {
		t.Fatalf("resolve actual package: %#v %v", actual, err)
	}
	err = runtime.WithNewBusinessAdmission(ctx, pluginhost.BusinessExtensionAdmissionInput{PluginID: old.PluginID, Point: old.Point, ContractVersion: old.ContractVersion, ReleaseVersion: old.ReleaseVersion, ReleaseDigest: old.ReleaseDigest, DeclarationDigest: old.DeclarationDigest, Method: "provision"}, func(_ context.Context, e *capmodel.BusinessExtensionProviderEvidence) error {
		if e != current {
			t.Fatal("admission fabricated evidence")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtime.InvokeProviderMethod(ctx, pluginhost.BusinessExtensionMethodInvokeInput{PluginID: old.PluginID, Point: old.Point, ContractVersion: old.ContractVersion, ReleaseVersion: old.ReleaseVersion, ReleaseDigest: old.ReleaseDigest, DeclarationDigest: old.DeclarationDigest, Method: "provision"})
	if err != nil {
		t.Fatal(err)
	}
	err = runtime.WithConfidentialOutboundContext(ctx, pluginhost.ConfidentialOutboundContextInput{ProviderPluginID: old.PluginID, Point: old.Point, ContractVersion: old.ContractVersion, ReleaseVersion: old.ReleaseVersion, ReleaseDigest: old.ReleaseDigest, DeclarationDigest: old.DeclarationDigest, ProviderMethod: "provision"}, func(context.Context, *capmodel.BusinessExtensionRuntimeContext) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if host.admission.ReleaseVersion != "" || host.admission.ReleaseDigest != "" || host.admission.DeclarationDigest != "" || host.invocation.ReleaseDigest != current.ReleaseDigest || host.outbound.ReleaseDigest != current.ReleaseDigest || host.invocation.ReleaseVersion != current.ReleaseVersion {
		t.Fatal("host received obsolete package identity")
	}
	if old.ReleaseDigest != strings.Repeat("a", 64) {
		t.Fatal("saved facts changed")
	}
	// A package update remains compatible even if the old package still exists.
	host.historical = &capmodel.BusinessExtensionProviderEvidence{ReleaseVersion: old.ReleaseVersion, ReleaseDigest: old.ReleaseDigest}
	actual, err = runtime.GetProviderExact(ctx, old)
	if err != nil || actual != current {
		t.Fatal("old package availability pinned new business")
	}
	// Recovery for a full page requires one discovery call, not one per interface.
	before := host.reads
	batch, err := runtime.GetProvidersExact(ctx, []plugincap.BusinessExtensionProviderExactInput{old, old, old})
	if err != nil || len(batch) != 3 || host.reads-before != 1 {
		t.Fatalf("unbounded or failed batch recovery: %v reads=%d", err, host.reads-before)
	}
	_, err = runtime.InvokeProviderMethod(ctx, pluginhost.BusinessExtensionMethodInvokeInput{PluginID: old.PluginID, Point: old.Point, ContractVersion: old.ContractVersion, ReleaseVersion: old.ReleaseVersion, ReleaseDigest: old.ReleaseDigest, DeclarationDigest: old.DeclarationDigest, Method: "undeclared"})
	if err == nil {
		t.Fatal("undeclared method accepted")
	}
}

func TestCurrentProviderRejectsDisabledWrongIdentityAndAmbiguity(t *testing.T) {
	for _, scenario := range []string{"disabled", "changed-contract", "changed-plugin", "ambiguous"} {
		t.Run(scenario, func(t *testing.T) {
			old := plugincap.BusinessExtensionProviderExactInput{PluginID: "connector", Point: "product-fulfillment", ContractVersion: "v1", ReleaseVersion: "v1.0.0", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("d", 64)}
			item := &capmodel.BusinessExtensionProviderEvidence{PluginID: old.PluginID, Point: old.Point, ContractVersion: old.ContractVersion, ReleaseVersion: "v1.1.0", ReleaseDigest: strings.Repeat("b", 64), DeclarationDigest: old.DeclarationDigest, AuthorizedMethods: []string{"provision"}}
			host := &executionHost{items: []*capmodel.BusinessExtensionProviderEvidence{item}}
			switch scenario {
			case "disabled":
				host.items = nil
			case "changed-contract":
				item.ContractVersion = "v2"
			case "changed-plugin":
				item.PluginID = "other"
			case "ambiguous":
				host.items = append(host.items, item)
			}
			runtime, err := New(host)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = runtime.GetProviderExact(context.Background(), old); err == nil {
				t.Fatal("incompatible package admitted")
			}
		})
	}
}

// TestAdditiveUpgradeUsesActiveEvidenceInSingleAndBatchPaths catches stale
// package pinning even when the historical package remains fully discoverable.
func TestAdditiveUpgradeUsesActiveEvidenceInSingleAndBatchPaths(t *testing.T) {
	old := plugincap.BusinessExtensionProviderExactInput{PluginID: "connector", Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1", ReleaseVersion: "v1.0.0", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64)}
	previous := &capmodel.BusinessExtensionProviderEvidence{PluginID: old.PluginID, Point: old.Point, ContractVersion: old.ContractVersion, ReleaseVersion: old.ReleaseVersion, ReleaseDigest: old.ReleaseDigest, DeclarationDigest: old.DeclarationDigest, AuthorizedMethods: []string{"provision"}, ProductConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1"}, CustomerConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1"}, ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Type: "product-fulfillment-contract", Version: "v1", Values: map[string]any{"merchantCustomerFields": []string{}}}}
	current := *previous
	current.ReleaseVersion, current.ReleaseDigest, current.DeclarationDigest = "v1.1.0", strings.Repeat("c", 64), strings.Repeat("d", 64)
	current.CustomerConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "cpu", Type: capmodel.ConfigurationFieldInteger, Minimum: "1"}}}
	host := &executionHost{items: []*capmodel.BusinessExtensionProviderEvidence{&current}, historical: previous}
	runtime, err := New(host)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := runtime.GetProviderExact(context.Background(), old)
	if err != nil || actual != &current {
		t.Fatalf("optional field replacement: %p %v", actual, err)
	}
	inputs := make([]plugincap.BusinessExtensionProviderExactInput, 50)
	for index := range inputs {
		inputs[index] = old
	}
	before := host.reads
	batch, err := runtime.GetProvidersExact(context.Background(), inputs)
	if err != nil || len(batch) != 50 || host.reads != before+1 {
		t.Fatalf("batch replacement: count=%d reads=%d %v", len(batch), host.reads-before, err)
	}
	for _, item := range batch {
		if item != &current {
			t.Fatal("historical result pinned obsolete package")
		}
	}
	current.CustomerConfiguration.Fields[0].Required = true
	if _, err = runtime.GetProvidersExact(context.Background(), inputs); err != nil {
		t.Fatalf("current declaration discovery must not require historical compatibility: %v", err)
	}
}
