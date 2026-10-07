package productautomation

import (
	"context"
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
