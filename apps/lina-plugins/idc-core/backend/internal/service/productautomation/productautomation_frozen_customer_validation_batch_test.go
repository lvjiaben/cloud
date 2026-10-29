package productautomation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestFrozenProviderEvidenceBatchUsesOneRegistryCollectionCall(t *testing.T) {
	providers := []*capmodel.BusinessExtensionProviderEvidence{
		{PluginID: "phone-provider", DisplayName: "Phone Provider", ReleaseVersion: "v1.0.0", Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1", AuthorizedMethods: []string{"fulfill"}, ReleaseDigest: "phone-release", DeclarationDigest: "phone-declaration"},
		{PluginID: "ssl-provider", DisplayName: "SSL Provider", ReleaseVersion: "v2.0.0", Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1", AuthorizedMethods: []string{"fulfill"}, ReleaseDigest: "ssl-release", DeclarationDigest: "ssl-declaration"},
	}
	exts := &automationTestExtensions{evidence: providers}
	validator := &frozenCustomerConfigurationValidator{extensions: exts}
	rows := make([]frozenValidationBatchRow, 0, len(providers))
	for index, provider := range providers {
		rows = append(rows, frozenValidationBatchRow{
			VersionNo: "ver_" + provider.PluginID, BindingInterfaceInstanceNo: "pfi_test", InterfaceDisplayNameSnapshot: provider.DisplayName,
			PluginID: provider.PluginID, ExtensionPointCode: provider.Point, PluginReleaseVersion: provider.ReleaseVersion,
			ContractVersion: provider.ContractVersion, AuthorizedMethods: append([]string(nil), provider.AuthorizedMethods...), ReleaseDigest: provider.ReleaseDigest,
			BindingDeclarationDigest: provider.DeclarationDigest, InterfaceConfigurationRevision: int64(index + 1),
		})
	}
	result, err := validator.loadFrozenProviderEvidenceBatch(context.Background(), rows)
	if err != nil || len(result) != 2 {
		t.Fatalf("load frozen provider batch: result=%#v err=%v", result, err)
	}
	if exts.exactBatchCalls != 1 || exts.exactCalls != 0 {
		t.Fatalf("multiple exact providers must use one batch call: batch=%d single=%d", exts.exactBatchCalls, exts.exactCalls)
	}
}

func TestNativeResourceOwnedFrozenInterfaceUsesOwnerProjection(t *testing.T) {
	exact := automationEvidence(nil)
	exact.InterfaceConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "connector-v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "endpoint", Type: capmodel.ConfigurationFieldSingleLine, Label: &capmodel.LocalizedText{ZhCN: "连接地址", EnUS: "Endpoint"}}}}
	owner := &capmodel.ConfigurationSchemaDeclaration{Version: "resource-owner-v1"}
	ownerJSON, _ := json.Marshal(owner)
	row := frozenValidationBatchRow{InterfaceOrigin: "resource", ResourceOwnerID: "idc-resource", ResourceOwnerRevision: "rci_" + strings.Repeat("1", 32), InterfaceSchemaVersion: owner.Version, InterfaceDeclarationSnapshot: string(ownerJSON), InterfacePublicValueSnapshot: "{}", InterfaceDeclarationDigest: configurationDigest(ownerJSON), InterfaceValueDigest: configurationDigest([]byte("{}")), ProductDeclarationSnapshot: "{}", CustomerDeclarationSnapshot: "{}", ProductSchemaVersion: configurationSchemaVersion(nil), CustomerSchemaVersion: configurationSchemaVersion(nil), VersionDeclarationDigest: configurationDigest([]byte("{}{}"))}
	input := FrozenCustomerConfigurationInput{CustomerDeclarationDigest: configurationDigest([]byte("{}"))}
	if _, err := validateFrozenBatchDeclarations(row, input, exact); err != nil {
		t.Fatal("native resource projection rejected", err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*frozenValidationBatchRow)
	}{
		{"missing owner revision", func(r *frozenValidationBatchRow) { r.ResourceOwnerRevision = "" }},
		{"wrong owner", func(r *frozenValidationBatchRow) { r.ResourceOwnerID = "other" }},
		{"managed interface with owner snapshot", func(r *frozenValidationBatchRow) { r.InterfaceOrigin = "managed" }},
		{"tampered interface digest", func(r *frozenValidationBatchRow) { r.InterfaceDeclarationDigest = strings.Repeat("f", 64) }},
		{"tampered public values", func(r *frozenValidationBatchRow) { r.InterfacePublicValueSnapshot = `{"unexpected":"value"}` }},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := row
			test.mutate(&changed)
			if _, err := validateFrozenBatchDeclarations(changed, input, exact); err == nil {
				t.Fatal("invalid resource evidence accepted")
			}
		})
	}
}
