// This file proves that imports retain only typed connector facts and tolerate
// missing fields without weakening the ordinary publication compiler.

package productautomation

import (
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"testing"
)

func TestImportedPartialConfigurationDoesNotBypassStrictCompiler(t *testing.T) {
	t.Parallel()
	label := &capmodel.LocalizedText{EnUS: "CPU", ZhCN: "处理器"}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "cpu", Type: capmodel.ConfigurationFieldInteger, Label: label, Required: true, Minimum: "1"},
		{Key: "upstream_product_id", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, Required: true, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "101", Label: label}}},
	}}
	value := "101"
	values, err := importConfigurationValues([]resourceinterfaceprojectioncap.CatalogConfigurationValue{{FieldKey: "upstream_product_id", OptionValue: &value}})
	if err != nil {
		t.Fatal(err)
	}
	partial, err := configurationcontract.CompileImportedDraftResolved(schema, values, nil, configurationcontract.ResolvedCandidates{})
	if err != nil {
		t.Fatal(err)
	}
	if string(partial.ValuesJSON) != `{"upstream_product_id":"101"}` {
		t.Fatalf("guessed missing fields: %s", partial.ValuesJSON)
	}
	if _, err = configurationcontract.CompileResolved(schema, values, configurationcontract.SurfaceProduct, nil, configurationcontract.ResolvedCandidates{}); err == nil {
		t.Fatal("publication accepted missing CPU")
	}
	zero := int64(0)
	values = append(values, ConfigurationValue{FieldKey: "cpu", IntegerValue: &zero})
	if _, err = configurationcontract.CompileImportedDraftResolved(schema, values, nil, configurationcontract.ResolvedCandidates{}); err == nil {
		t.Fatal("import accepted invalid supplied CPU")
	}
}

func TestImportReadEvidenceRejectsSourceAndProjectionChanges(t *testing.T) {
	t.Parallel()
	exact := automationEvidence(nil)
	read := resourceinterfaceprojectioncap.CatalogReadEvidence{InterfaceInstanceNo: "pfi_11111111111111111111111111111111",
		OwnerID: resourceinterfaceprojectioncap.OwnerID, ResourceChannelRef: "CHN_import", InterfaceConfigurationRevision: "rci_11111111111111111111111111111111",
		PluginID: exact.PluginID, PluginReleaseVersion: exact.ReleaseVersion, ContractVersion: exact.ContractVersion,
		AuthorizedMethods: exact.AuthorizedMethods, ReleaseDigest: exact.ReleaseDigest, DeclarationDigest: exact.DeclarationDigest}
	instance := &entity.ProductAutomationInterfaceInstance{InterfaceInstanceNo: read.InterfaceInstanceNo, InterfaceOriginCode: "resource", ResourceOwnerId: read.OwnerID, ResourceChannelRef: read.ResourceChannelRef}
	revision := &entity.ProductAutomationInterfaceConfigurationRevision{InterfaceInstanceNo: read.InterfaceInstanceNo, ResourceOwnerConfigurationRevision: read.InterfaceConfigurationRevision}
	if !matchesImportReadEvidence(&read, instance, revision, exact) {
		t.Fatal("exact owner read was rejected")
	}
	for name, mutate := range map[string]func(*resourceinterfaceprojectioncap.CatalogReadEvidence){
		"different interface": func(e *resourceinterfaceprojectioncap.CatalogReadEvidence) {
			e.InterfaceInstanceNo = "pfi_22222222222222222222222222222222"
		},
		"different owner":   func(e *resourceinterfaceprojectioncap.CatalogReadEvidence) { e.OwnerID = "other-owner" },
		"different channel": func(e *resourceinterfaceprojectioncap.CatalogReadEvidence) { e.ResourceChannelRef = "CHN_other" },
		"old or unprojected revision": func(e *resourceinterfaceprojectioncap.CatalogReadEvidence) {
			e.InterfaceConfigurationRevision = "rci_22222222222222222222222222222222"
		},
		"different connector": func(e *resourceinterfaceprojectioncap.CatalogReadEvidence) { e.PluginID = "other-connector" },
		"different release":   func(e *resourceinterfaceprojectioncap.CatalogReadEvidence) { e.PluginReleaseVersion = "v2.0.0" },
		"different method set": func(e *resourceinterfaceprojectioncap.CatalogReadEvidence) {
			e.AuthorizedMethods = []string{"provision"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := read
			mutate(&changed)
			if matchesImportReadEvidence(&changed, instance, revision, exact) {
				t.Fatal("mismatched catalog source accepted")
			}
		})
	}
	if matchesImportReadEvidence(nil, instance, revision, exact) {
		t.Fatal("missing owner evidence accepted")
	}
	first := dynamicCandidateScope{interfaceInstanceNo: read.InterfaceInstanceNo, configurationRevision: 1, resourceConfigurationRevision: read.InterfaceConfigurationRevision}
	second := first
	second.resourceConfigurationRevision = "rci_22222222222222222222222222222222"
	if dynamicCandidateCacheKey(exact, first, "candidate.product", []byte(`{}`)) == dynamicCandidateCacheKey(exact, second, "candidate.product", []byte(`{}`)) {
		t.Fatal("resource snapshots shared an import candidate cache entry")
	}
}

func TestImportConfigurationRejectsMultipleArmsAndUnknownFields(t *testing.T) {
	t.Parallel()
	text := "101"
	number := int64(2)
	if _, err := importConfigurationValues([]resourceinterfaceprojectioncap.CatalogConfigurationValue{{FieldKey: "cpu", TextValue: &text, IntegerValue: &number}}); err == nil {
		t.Fatal("multiple scalar arms accepted")
	}
	values, err := importConfigurationValues([]resourceinterfaceprojectioncap.CatalogConfigurationValue{{FieldKey: "not_declared", TextValue: &text}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = configurationcontract.CompileImportedDraftResolved(nil, values, nil, nil); err == nil {
		t.Fatal("unknown schema field accepted")
	}
}

func TestImportedDraftKeepsUnfilledDynamicFieldsWithoutRemoteLookups(t *testing.T) {
	t.Parallel()
	label := &capmodel.LocalizedText{EnUS: "Region", ZhCN: "地区"}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "region", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, Required: true, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "configuration.candidates"}}}}
	resolution, empty, err := importResolutionSchema(schema, nil, []string{"configuration.candidates"})
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Fields[0].DynamicCandidates != nil || schema.Fields[0].DynamicCandidates == nil {
		t.Fatal("mutated original schema or retained missing candidate lookup")
	}
	if _, err = configurationcontract.CompileImportedDraftResolved(schema, nil, []string{"configuration.candidates"}, empty); err != nil {
		t.Fatalf("missing required dynamic field blocked draft: %v", err)
	}
	if _, err = configurationcontract.CompileResolved(schema, nil, configurationcontract.SurfaceProduct, []string{"configuration.candidates"}, empty); err == nil {
		t.Fatal("missing dynamic field passed publication")
	}
}
