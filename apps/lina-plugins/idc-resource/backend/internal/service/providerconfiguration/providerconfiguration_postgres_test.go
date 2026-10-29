// This file verifies that supplier-channel creation and the generic product
// interface projection are persisted atomically in PostgreSQL.

package providerconfiguration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	providerspi "lina-plugin-idc-resource/backend/cap/resourcecap/spi"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
	"lina-plugin-idc-resource/backend/internal/service/interfaceprojection"
)

const providerConfigurationDSNEnv = "LINAPRO_IDC_RESOURCE_TEST_DSN"

type providerConfigurationAuthorizer struct{}

func (providerConfigurationAuthorizer) Authorize(
	context.Context,
	managementcap.AuthorizationRequest,
) (int, error) {
	return 101, nil
}

func TestProviderConfigurationPostgreSQLSeparatesResourceAndProductEvidence(t *testing.T) {
	ctx, database := newProviderConfigurationDatabase(t)
	channelNo := "CHN_evidence000000001"
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel
    (channel_no,provider_id,provider_plugin_id,provider_runtime_type,
     provider_contract_version,provider_release_version,provider_release_digest,
     provider_declaration_digest,display_name,endpoint,status_code)
VALUES (?, 'server-api','server-connector','dynamic','v1','v1.0.0',?,?,
        'Evidence channel','https://supplier.example','disabled')`, channelNo,
		strings.Repeat("a", 64), strings.Repeat("b", 64)); err != nil {
		t.Fatalf("seed evidence channel: %v", err)
	}
	resourceEvidence := &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "server-connector", ReleaseVersion: "v1.0.0",
		Point: resourcecap.DynamicProviderPoint, ContractVersion: "v1",
		AuthorizedMethods: []string{resourcecap.DynamicProviderMethodConfigurationCandidates, resourcecap.DynamicProviderMethodExecute},
		ReleaseDigest:     strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
	}
	projection := resourceinterfaceprojectioncap.Projection{
		InterfaceInstanceNo: "pfi_33333333333333333333333333333333",
		DisplayName:         "Evidence channel", ActorUserID: 101,
		OwnerID: resourceinterfaceprojectioncap.OwnerID, ResourceChannelRef: channelNo,
		PluginID: "server-connector", PluginReleaseVersion: "v1.0.0",
		ContractVersion: "v1", AuthorizedMethods: []string{"fulfillment.provision"},
		ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("c", 64),
		NewBusinessState:               resourceinterfaceprojectioncap.StateStopped,
		InterfaceConfigurationRevision: "rci_33333333333333333333333333333333",
	}
	if err := database.Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		return insertInterfaceChannelBinding(tx, projection.InterfaceInstanceNo, channelNo, projection.InterfaceConfigurationRevision, resourceEvidence, &projection)
	}); err != nil {
		t.Fatalf("insert separate resource/product evidence: %v", err)
	}
	var row struct {
		ResourceDigest  string   `orm:"resource_declaration_digest"`
		ProductDigest   string   `orm:"product_declaration_digest"`
		ResourceMethods []string `orm:"resource_authorized_methods"`
		ProductMethods  []string `orm:"product_authorized_methods"`
	}
	if err := database.GetScan(ctx, &row, `SELECT resource_declaration_digest,
       product_declaration_digest,resource_authorized_methods,product_authorized_methods
FROM plugin_idc_resource_interface_channel_binding WHERE channel_no=?`, channelNo); err != nil {
		t.Fatalf("read separate resource/product evidence: %v", err)
	}
	if row.ResourceDigest == row.ProductDigest || len(row.ResourceMethods) != 2 ||
		len(row.ProductMethods) != 1 || row.ProductMethods[0] != "fulfillment.provision" {
		t.Fatalf("resource/product evidence was mixed: %+v", row)
	}
}

type providerConfigurationAudit struct{}

func (providerConfigurationAudit) AppendBusinessTransition(
	context.Context,
	idccap.BusinessAuditTransition,
) error {
	return nil
}

func (providerConfigurationAudit) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func TestProviderConfigurationPostgreSQLPersistsCertificateTrust(t *testing.T) {
	ctx, database := newProviderConfigurationDatabase(t)
	service, err := New(database, managementcap.Runtime{
		Authorizer: providerConfigurationAuthorizer{}, Audit: providerConfigurationAudit{},
	}, []providerspi.Declaration{{Key: resourcecap.ProviderKey{PluginID: "server-connector", ProviderID: "server-api", ContractVersion: resourcecap.ContractVersion}, ProvisionParameters: testProviderConfigurationParameters}}, testUsageReader{}, testConfigurationProjection(t, database))
	if err != nil {
		t.Fatal(err)
	}
	pin := strings.Repeat("a", 64)
	created, err := service.CreateChannel(ctx, ChannelInput{ProviderPluginID: "server-connector", DisplayName: "Pinned", Endpoint: "https://supplier.example/v1", TLSCertificateSHA256: &pin})
	if err != nil {
		t.Fatal(err)
	}
	if created.TLSCertificateSHA256 != pin {
		t.Fatal("create lost certificate trust")
	}
	reader, err := channelconfig.NewReader(database)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := reader.ReadConfigured(ctx, created.ChannelNo)
	if err != nil || configuration.Config.TLSCertificateSHA256 != pin {
		t.Fatalf("current reader lost certificate trust: %v", err)
	}
	revision, err := database.GetValue(ctx, "SELECT current_configuration_revision FROM plugin_idc_resource_interface_channel_binding WHERE channel_no=?", created.ChannelNo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, "UPDATE plugin_idc_resource_channel SET status_code='enabled' WHERE channel_no=?", created.ChannelNo); err != nil {
		t.Fatal(err)
	}
	frozen, err := reader.ReadRevision(ctx, created.ChannelNo, revision.String())
	if err != nil || frozen.Config.TLSCertificateSHA256 != pin {
		t.Fatalf("revision reader lost channel transport trust: %v", err)
	}
	if _, err = database.Exec(ctx, "UPDATE plugin_idc_resource_channel SET status_code='disabled' WHERE channel_no=?", created.ChannelNo); err != nil {
		t.Fatal(err)
	}

	updated, err := service.UpdateChannel(ctx, ChannelInput{ChannelNo: created.ChannelNo, DisplayName: created.DisplayName, Endpoint: created.Endpoint, Status: "disabled", Version: created.Version})
	if err != nil || updated == nil || updated.TLSCertificateSHA256 != pin {
		t.Fatalf("omitted trust not preserved: %v", err)
	}
	invalid := "invalid"
	if _, err = service.UpdateChannel(ctx, ChannelInput{ChannelNo: updated.ChannelNo, DisplayName: updated.DisplayName, Endpoint: updated.Endpoint, Status: "disabled", Version: updated.Version, TLSCertificateSHA256: &invalid}); err == nil {
		t.Fatal("invalid trust accepted")
	}
	value, err := database.GetValue(ctx, "SELECT version FROM plugin_idc_resource_channel WHERE channel_no=?", updated.ChannelNo)
	if err != nil || value.Int64() != updated.Version {
		t.Fatal("invalid trust wrote configuration")
	}
	updated, err = service.UpdateChannel(ctx, ChannelInput{ChannelNo: updated.ChannelNo, DisplayName: updated.DisplayName, Endpoint: "https://other.example/v1", Status: "disabled", Version: updated.Version})
	if err != nil || updated == nil || updated.TLSCertificateSHA256 != "" {
		t.Fatalf("endpoint change retained stale trust: %v", err)
	}
}

func TestProviderConfigurationPostgreSQLCreatesStableInterfaceBindingAtomically(t *testing.T) {
	ctx, database := newProviderConfigurationDatabase(t)
	service, err := New(database, managementcap.Runtime{
		Authorizer: providerConfigurationAuthorizer{}, Audit: providerConfigurationAudit{},
	}, []providerspi.Declaration{{
		Key: resourcecap.ProviderKey{
			PluginID: "server-connector", ProviderID: "server-api",
			ContractVersion: resourcecap.ContractVersion,
		},
		ProvisionParameters: testProviderConfigurationParameters,
	}}, testUsageReader{}, testConfigurationProjection(t, database))
	if err != nil {
		t.Fatalf("construct Provider configuration service: %v", err)
	}
	created, err := service.CreateChannel(ctx, ChannelInput{
		ProviderPluginID: "server-connector", DisplayName: "Supplier A",
		Endpoint: "https://supplier.example/api",
	})
	if err != nil || created == nil || !strings.HasPrefix(created.ChannelNo, "CHN_") ||
		len(created.InterfaceInstanceNo) != 36 ||
		!strings.HasPrefix(created.InterfaceInstanceNo, "pfi_") {
		t.Fatalf("create Provider channel=%+v err=%v", created, err)
	}
	assertProviderInterfaceBinding(t, ctx, database, created.InterfaceInstanceNo, created.ChannelNo)
	var before struct {
		Revision string    `orm:"current_configuration_revision"`
		Updated  time.Time `orm:"updated_at"`
	}
	if err = database.GetScan(ctx, &before, `SELECT current_configuration_revision,updated_at
FROM plugin_idc_resource_interface_channel_binding WHERE channel_no=?`, created.ChannelNo); err != nil {
		t.Fatal(err)
	}
	_, _ = database.Exec(ctx, `SELECT pg_sleep(0.01)`)

	updated, err := service.UpdateChannel(ctx, ChannelInput{
		ChannelNo: created.ChannelNo, DisplayName: "Supplier A renamed",
		Endpoint: "https://supplier.example/v2", Status: "disabled", Version: created.Version,
	})
	if err != nil || updated == nil || updated.InterfaceInstanceNo != created.InterfaceInstanceNo {
		t.Fatalf("update changed interface binding: updated=%+v err=%v", updated, err)
	}
	var after struct {
		Revision string    `orm:"current_configuration_revision"`
		Updated  time.Time `orm:"updated_at"`
	}
	if err = database.GetScan(ctx, &after, `SELECT current_configuration_revision,updated_at
FROM plugin_idc_resource_interface_channel_binding WHERE channel_no=?`, created.ChannelNo); err != nil ||
		after.Revision == before.Revision || !after.Updated.After(before.Updated) {
		t.Fatalf("configuration revision did not advance: before=%+v after=%+v err=%v", before, after, err)
	}
	deleted, deleteErr := service.UpdateChannel(ctx, ChannelInput{
		ChannelNo: updated.ChannelNo, DisplayName: updated.DisplayName,
		Endpoint: updated.Endpoint, NonSecretSettings: updated.NonSecretSettings,
		Status: "deleted", Version: updated.Version,
	})
	if deleteErr != nil || deleted == nil || deleted.Status != "deleted" || deleted.Version != updated.Version+1 {
		t.Fatalf("delete must return committed state, not fail while rereading hidden configuration: %+v %v", deleted, deleteErr)
	}
	stored, storedErr := database.GetValue(ctx, "SELECT deleted_at IS NOT NULL FROM plugin_idc_resource_channel WHERE channel_no=?", created.ChannelNo)
	if storedErr != nil || !stored.Bool() {
		t.Fatalf("channel deletion was not persisted: %v %v", stored, storedErr)
	}
	reader, readerErr := channelconfig.NewReader(database)
	if readerErr != nil {
		t.Fatal(readerErr)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_channel SET status_code='enabled',deleted_at=NULL WHERE channel_no=?`, created.ChannelNo); err != nil {
		t.Fatal(err)
	}
	frozen, readerErr := reader.ReadRevision(ctx, created.ChannelNo, before.Revision)
	if readerErr != nil || frozen.Config.Endpoint != "https://supplier.example/api" {
		t.Fatalf("old immutable revision was lost: config=%+v err=%v", frozen, readerErr)
	}

	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel
    (channel_no,provider_id,provider_plugin_id,display_name,endpoint,status_code)
VALUES ('CHN_conflict000000000001','server-api','server-connector',
        'Conflict','https://supplier.example','disabled')`); err != nil {
		t.Fatalf("seed conflict channel: %v", err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_resource_interface_channel_binding
    (interface_instance_no,channel_no,binding_version)
VALUES (?, 'CHN_conflict000000000001', 1)`, created.InterfaceInstanceNo); err == nil {
		t.Fatal("same interface number was rebound to another channel")
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_resource_interface_channel_binding
    (interface_instance_no,channel_no,binding_version)
VALUES ('pfi_22222222222222222222222222222222', ?, 1)`, created.ChannelNo); err == nil {
		t.Fatal("same channel accepted a second product interface number")
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_resource_interface_channel_binding
    (interface_instance_no,channel_no,binding_version)
VALUES (?, ?, 1)`, created.InterfaceInstanceNo, created.ChannelNo); err == nil {
		t.Fatal("exact interface binding replay bypassed the conflict fence")
	}

	if _, err = database.Exec(ctx, `
CREATE FUNCTION reject_interface_binding() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'injected binding failure'; END; $$;
CREATE TRIGGER reject_interface_binding
BEFORE INSERT ON plugin_idc_resource_interface_channel_binding
FOR EACH ROW EXECUTE FUNCTION reject_interface_binding()`); err != nil {
		t.Fatalf("install binding rollback fixture: %v", err)
	}
	if _, err = service.CreateChannel(ctx, ChannelInput{
		ProviderPluginID: "server-connector", DisplayName: "Must Roll Back",
		Endpoint: "https://rollback.example/api",
	}); err == nil {
		t.Fatal("channel creation unexpectedly survived binding failure")
	}
	count, countErr := database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_resource_channel
WHERE display_name='Must Roll Back'`)
	if countErr != nil || count.Int() != 0 {
		t.Fatalf("channel escaped failed interface-binding transaction: count=%d err=%v", count.Int(), countErr)
	}
}

func TestProviderConfigurationPostgreSQLCreatesPoolForOwnedChannel(t *testing.T) {
	ctx, database := newProviderConfigurationDatabase(t)
	service, err := New(database, managementcap.Runtime{
		Authorizer: providerConfigurationAuthorizer{}, Audit: providerConfigurationAudit{},
	}, []providerspi.Declaration{{
		Key: resourcecap.ProviderKey{
			PluginID: "server-connector", ProviderID: "server-api",
			ContractVersion: resourcecap.ContractVersion,
		},
		ProvisionParameters: testProviderConfigurationParameters,
	}}, testUsageReader{}, testConfigurationProjection(t, database))
	if err != nil {
		t.Fatalf("construct Provider configuration service: %v", err)
	}
	const channelNo = "CHN_poolcreate000000001"
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel
    (channel_no,provider_id,provider_plugin_id,display_name,endpoint,status_code)
VALUES (?, 'server-api','server-connector','Pool channel',
        'https://supplier.example','disabled')`, channelNo); err != nil {
		t.Fatalf("seed pool channel: %v", err)
	}
	created, err := service.CreatePool(ctx, PoolInput{
		ChannelNo: channelNo, DisplayName: "Primary pool", ResourceKind: "virtual_server",
		RoutingPriority: 10, RoutingWeight: 100,
	})
	if err != nil || created == nil || !strings.HasPrefix(created.PoolNo, "POL_") ||
		created.ChannelNo != channelNo || created.Status != "disabled" {
		t.Fatalf("create resource pool=%+v err=%v", created, err)
	}
	count, countErr := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_resource_pool
WHERE channel_no=? AND display_name='Primary pool'`, channelNo)
	if countErr != nil || count.Int() != 1 {
		t.Fatalf("resource pool was not persisted once: count=%d err=%v", count.Int(), countErr)
	}

	if _, err = service.CreatePool(ctx, PoolInput{
		ChannelNo: "CHN_missingpoolchannel", DisplayName: "Must not exist", ResourceKind: "virtual_server",
		RoutingPriority: 10, RoutingWeight: 100,
	}); err == nil {
		t.Fatal("missing channel unexpectedly accepted a resource pool")
	}
	count, countErr = database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_resource_pool
WHERE display_name='Must not exist'`)
	if countErr != nil || count.Int() != 0 {
		t.Fatalf("failed resource pool escaped transaction: count=%d err=%v", count.Int(), countErr)
	}
}

func TestBTPanelAuthenticationProfileAndSecretHandleFreezeInPostgreSQLRevision(t *testing.T) {
	ctx, database := newProviderConfigurationDatabase(t)
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "endpoint", Type: capmodel.ConfigurationFieldSingleLine},
		{Key: "api_key", Type: capmodel.ConfigurationFieldSecret},
	}}
	metadata := &capmodel.BusinessExtensionProviderMetadata{Type: resourcecap.DynamicProviderMetadataType, Version: "v1", Values: map[string]any{
		"providerId": "bt-panel", "authenticationProfiles": map[string]any{"profiles": map[string]any{"default": map[string]any{
			"strategy": "bt_panel_double_md5", "fields": map[string]any{"material": "api_key"},
		}}},
	}}
	compiled, err := resourcecap.CompileInterfaceSettings(json.RawMessage(`{"endpoint":"https://panel-a.example.test"}`), metadata)
	if err != nil {
		t.Fatal(err)
	}
	const (
		channelNo   = "CHN_btpanel0000000001"
		interfaceNo = "pfi_44444444444444444444444444444444"
		revisionNo  = "rci_44444444444444444444444444444444"
	)
	frozenHandle, handleErr := secretcap.NewHandle("secret_bt_panel_a")
	if handleErr != nil {
		t.Fatal(handleErr)
	}
	if validateErr := (resourcecap.ProviderConfig{ChannelRef: channelNo, Endpoint: "https://panel-a.example.test", Settings: compiled,
		SecretBindings: []resourcecap.SecretBinding{{Name: "api_key", Handle: frozenHandle}}}).Validate(); validateErr != nil {
		t.Fatalf("validate BT Panel frozen configuration fixture: %v", validateErr)
	}
	resourceEvidence := &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "idc-baota", ReleaseVersion: "v0.1.1", Point: resourcecap.DynamicProviderPoint,
		ContractVersion: resourcecap.DynamicProviderContractVersion, AuthorizedMethods: resourcecap.DynamicProviderMethods(),
		ReleaseDigest: strings.Repeat("d", 64), DeclarationDigest: strings.Repeat("e", 64),
		InterfaceConfiguration: schema, ProviderMetadata: metadata,
	}
	productEvidence := validProductEvidence(t, resourceEvidence)
	projection := resourceinterfaceprojectioncap.Projection{
		InterfaceInstanceNo: interfaceNo, DisplayName: "BT Panel A", ActorUserID: 101,
		OwnerID: resourceinterfaceprojectioncap.OwnerID, ResourceChannelRef: channelNo,
		PluginID: productEvidence.PluginID, PluginReleaseVersion: productEvidence.ReleaseVersion,
		ContractVersion: productEvidence.ContractVersion, AuthorizedMethods: append([]string(nil), productEvidence.AuthorizedMethods...),
		ReleaseDigest: productEvidence.ReleaseDigest, DeclarationDigest: productEvidence.DeclarationDigest,
		NewBusinessState: resourceinterfaceprojectioncap.StateStopped, InterfaceConfigurationRevision: revisionNo,
	}
	err = database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		if _, insertErr := tx.Exec(`INSERT INTO plugin_idc_resource_channel
 (channel_no,provider_id,provider_plugin_id,provider_runtime_type,provider_contract_version,
  provider_release_version,provider_release_digest,provider_declaration_digest,
  display_name,endpoint,non_secret_settings,status_code)
VALUES (?, 'bt-panel','idc-baota','dynamic','v1','v0.1.1',?,?,
        'BT Panel A','https://panel-a.example.test',CAST(? AS jsonb),'enabled')`, channelNo,
			strings.Repeat("d", 64), strings.Repeat("e", 64), string(compiled)); insertErr != nil {
			return insertErr
		}
		if _, insertErr := tx.Exec(`INSERT INTO plugin_idc_resource_channel_secret_binding
 (binding_no,channel_no,binding_name,secret_ref,status_code)
VALUES ('BND_btpanel000000001',?,'api_key','secret_bt_panel_a','active')`, channelNo); insertErr != nil {
			return insertErr
		}
		if insertErr := insertInterfaceChannelBinding(tx, interfaceNo, channelNo, revisionNo, resourceEvidence, productEvidenceProjection(productEvidence, projection)); insertErr != nil {
			return insertErr
		}
		return insertConfigurationRevision(txCtx, tx, revisionNo, interfaceNo, channelNo, "https://panel-a.example.test", compiled, 101)
	})
	if err != nil {
		t.Fatalf("freeze BT Panel revision: %v", err)
	}
	// Credential revisions can advance the outbox beyond the channel row version.
	// A subsequent settings/name edit must append after them, never reuse version*2.
	if err = database.Transaction(ctx, func(txCtx context.Context, tx gdb.TX) error {
		if err := interfaceprojection.Enqueue(txCtx, tx, projection, 6); err != nil {
			return err
		}
		return enqueueUpdatedChannelProjection(txCtx, tx, ChannelInput{
			ChannelNo: channelNo, DisplayName: "BT Panel renamed", Status: "disabled", Version: 2,
		}, 101, true, revisionNo)
	}); err != nil {
		t.Fatalf("editing after credential revisions reused a sequence: %v", err)
	}
	sequence, err := database.GetValue(ctx, `SELECT MAX(projection_sequence) FROM plugin_idc_resource_interface_projection_outbox WHERE interface_instance_no=?`, interfaceNo)
	if err != nil || sequence.Int64() != 8 {
		t.Fatalf("expected ordered configuration/name events 7 and 8 after secret event 6: sequence=%v err=%v", sequence, err)
	}

	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_channel_secret_binding
SET secret_ref='secret_bt_panel_b' WHERE channel_no=? AND binding_name='api_key'`, channelNo); err != nil {
		t.Fatal(err)
	}
	reader, err := channelconfig.NewReader(database)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := reader.ReadRevision(ctx, channelNo, revisionNo)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := resourcecap.DecodeHostAuthenticationSettings(frozen.Config.Settings)
	if err != nil || profile.Strategy != "bt_panel_double_md5" || profile.Fields["material"] != "api_key" ||
		len(frozen.Config.SecretBindings) != 1 || frozen.Config.SecretBindings[0].Name != "api_key" || frozen.Config.SecretBindings[0].Handle.OpaqueRef() != "secret_bt_panel_a" {
		t.Fatalf("BT Panel revision crossed channel instances or lost its secret handle: profile=%+v config=%+v err=%v", profile, frozen.Config, err)
	}
}

func productEvidenceProjection(evidence *capmodel.BusinessExtensionProviderEvidence, projection resourceinterfaceprojectioncap.Projection) *resourceinterfaceprojectioncap.Projection {
	projection.PluginID = evidence.PluginID
	projection.PluginReleaseVersion = evidence.ReleaseVersion
	projection.ContractVersion = evidence.ContractVersion
	projection.AuthorizedMethods = append([]string(nil), evidence.AuthorizedMethods...)
	projection.ReleaseDigest = evidence.ReleaseDigest
	projection.DeclarationDigest = evidence.DeclarationDigest
	return &projection
}

func testProviderConfigurationParameters(
	context.Context,
	resourcecap.ServerProvisionPlan,
) (json.RawMessage, error) {
	return json.RawMessage(`{"accepted":true}`), nil
}

func assertProviderInterfaceBinding(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	interfaceInstanceNo string,
	channelNo string,
) {
	t.Helper()
	var row struct {
		InterfaceInstanceNo string `orm:"interface_instance_no"`
		ChannelNo           string `orm:"channel_no"`
		BindingVersion      int64  `orm:"binding_version"`
	}
	if err := database.GetScan(ctx, &row, `
SELECT interface_instance_no,channel_no,binding_version
FROM plugin_idc_resource_interface_channel_binding
WHERE interface_instance_no=?`, interfaceInstanceNo); err != nil ||
		row.InterfaceInstanceNo != interfaceInstanceNo || row.ChannelNo != channelNo ||
		row.BindingVersion != 1 {
		t.Fatalf("interface binding=%+v err=%v", row, err)
	}
}

func newProviderConfigurationDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(providerConfigurationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run Provider configuration PostgreSQL tests", providerConfigurationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create Provider configuration admin database: %v", err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate Provider configuration schema: %v", err)
	}
	schema := "linapro_provider_configuration_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create Provider configuration schema: %v", err)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: appendProviderConfigurationDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("create scoped Provider configuration database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped Provider configuration database: %v", closeErr)
		}
		if !strings.HasPrefix(schema, "linapro_provider_configuration_test_") {
			t.Errorf("refusing to drop unexpected Provider configuration schema %q", schema)
		} else if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Errorf("drop Provider configuration schema: %v", dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close Provider configuration admin database: %v", closeErr)
		}
	})
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve Provider configuration migration path")
	}
	paths, err := filepath.Glob(filepath.Join(
		filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "[0-9][0-9][0-9]-*.sql",
	))
	if err != nil || len(paths) == 0 {
		t.Fatalf("resolve Provider configuration migrations: %v", err)
	}
	for _, path := range paths {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read Provider configuration migration %s: %v", filepath.Base(path), readErr)
		}
		if _, executeErr := database.Exec(ctx, string(migration)); executeErr != nil {
			t.Fatalf("execute Provider configuration migration %s: %v", filepath.Base(path), executeErr)
		}
	}
	return ctx, database
}

func appendProviderConfigurationDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}
