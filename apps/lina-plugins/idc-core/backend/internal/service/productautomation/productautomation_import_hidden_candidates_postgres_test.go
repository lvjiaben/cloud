// This test resolves an uncached hidden directory default without inventing a
// public token. The source request omits it and the returned exact directory is
// still persisted and validated locally.
package productautomation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func TestImportedHiddenUncachedDirectoryDefaultPostgreSQL(t *testing.T) {
	ctx := context.Background()
	db := portableBindingDatabase(t, ctx)
	if _, err := db.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_import_candidate(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,version_no TEXT NOT NULL,field_key TEXT NOT NULL,candidate_value TEXT NOT NULL,option_token TEXT NOT NULL,created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,UNIQUE(version_no,field_key,candidate_value))`); err != nil {
		t.Fatal(err)
	}
	snapshot := portableAutomationSnapshot()
	hidden := false
	snapshot.ProtocolVersion = channelcommercecap.ImportPurchaseCycleLimitProtocolVersion
	snapshot.Schema.Fields[0].Type = capmodel.ConfigurationFieldSingleSelect
	snapshot.Schema.Fields[0].DynamicCandidates = &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: importedCandidateMethod}
	snapshot.Mappings[0].CustomerVisible = &hidden
	snapshot.Mappings[0].DefaultValue = json.RawMessage(`"opaque-default"`)
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	exact := automationEvidence([]string{importedCandidateMethod})
	exact.PluginID = portableImportProviderID
	interfaceNo := "pfi_" + strings.Repeat("a", 32)
	version := "ver_" + strings.Repeat("b", 32)
	read := resourceinterfaceprojectioncap.CatalogReadEvidence{InterfaceInstanceNo: interfaceNo, OwnerID: "idc-resource", ResourceChannelRef: "CHN_hidden", InterfaceConfigurationRevision: "rci_" + strings.Repeat("c", 32), PluginID: exact.PluginID, PluginReleaseVersion: exact.ReleaseVersion, ContractVersion: exact.ContractVersion, AuthorizedMethods: exact.AuthorizedMethods, ReleaseDigest: exact.ReleaseDigest, DeclarationDigest: exact.DeclarationDigest}
	readRaw, err := json.Marshal(read)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_import_contract(tenant_id,version_no,interface_instance_no,snapshot,snapshot_digest,read_evidence) VALUES(0,?,?,CAST(? AS JSONB),?,CAST(? AS JSONB))`, version, interfaceNo, string(raw), configurationDigest(raw), string(readRaw)); err != nil {
		t.Fatal(err)
	}
	source := &resourceCandidateSourceStub{result: &resourceinterfaceprojectioncap.CandidateResult{Fields: []resourceinterfaceprojectioncap.CandidateField{{FieldKey: "region", Items: []resourceinterfaceprojectioncap.Candidate{{Value: "opaque-default", OptionToken: "upstream-default-token", LabelZhCN: "默认区域", LabelEnUS: "Default region"}}}}}}
	svc := &serviceImpl{database: db, resourceCandidates: source}
	resolved, err := svc.resolveImportedCandidates(ctx, exact, snapshot.Schema, []configurationcontract.ConfigurationValue{{FieldKey: "region", OptionTokens: []string{"opaque-default"}}}, dynamicCandidateScope{versionNo: version, interfaceInstanceNo: interfaceNo})
	if err != nil || len(resolved["region"]) != 1 {
		t.Fatalf("uncached default failed: %+v %v", resolved, err)
	}
	var sent channelcommercecap.ImportCandidatesInput
	if err = json.Unmarshal(source.request.Values, &sent); err != nil {
		t.Fatal(err)
	}
	if len(sent.Values) != 0 {
		t.Fatalf("hidden default was forwarded as editable input: %+v", sent.Values)
	}
}
