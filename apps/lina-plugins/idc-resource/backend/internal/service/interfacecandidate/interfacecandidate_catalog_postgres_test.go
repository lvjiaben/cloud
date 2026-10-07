// This file proves catalog reads use one immutable endpoint/settings/secret
// snapshot and reject a revision change before returning owner-generated evidence.
package interfacecandidate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

type catalogReadExtensions struct {
	pluginhost.BusinessExtensionRegistrar
	evidence  *capmodel.BusinessExtensionProviderEvidence
	afterRead func() error
}

func (s *catalogReadExtensions) GetProviderExact(context.Context, plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	return s.evidence, nil
}
func (s *catalogReadExtensions) WithConfidentialOutboundContext(ctx context.Context, _ pluginhost.ConfidentialOutboundContextInput, action pluginhost.ConfidentialOutboundAction) error {
	return action(ctx, &capmodel.BusinessExtensionRuntimeContext{ContextID: "test"})
}
func (s *catalogReadExtensions) InvokeProviderMethod(context.Context, pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	if s.afterRead != nil {
		if err := s.afterRead(); err != nil {
			return nil, err
		}
	}
	return &capmodel.BusinessExtensionMethodResult{Payload: []byte(`{"product":{"reference":"101","name":"Snapshot product","description":""},"basePrices":[],"options":[],"priceScheduleComplete":false,"productConfiguration":[]}`)}, nil
}

func TestCatalogReadPinsOwnerConfigurationPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_RESOURCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_RESOURCE_TEST_DSN")
	}
	ctx := t.Context()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_catalog_read_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	if _, err = database.Exec(ctx, catalogReadFixtureSQL); err != nil {
		t.Fatal(err)
	}
	const interfaceNo = "pfi_11111111111111111111111111111111"
	const revision = "rci_11111111111111111111111111111111"
	const newerRevision = "rci_22222222222222222222222222222222"
	digestA, digestB := strings.Repeat("a", 64), strings.Repeat("b", 64)
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel VALUES(0,'CHN_catalog','server-api','server-connector','dynamic','v1','v1.0.0',?,?,'https://current.example','{}','enabled',NULL)`, digestA, digestB); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_channel_binding VALUES(0,?,'CHN_catalog','server-connector','v1.0.0','v1',ARRAY['catalog.products.detail'],?,?,'server-connector','v1.0.0','v1',ARRAY['provision','query'],?,?,'accepting',?)`, interfaceNo, digestA, digestB, digestA, digestB, revision); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_configuration_revision VALUES(0,?,?,'CHN_catalog','https://snapshot.example','{}','{"api_key":"snapshot-secret-ref"}')`, revision, interfaceNo); err != nil {
		t.Fatal(err)
	}
	extensions := &catalogReadExtensions{evidence: &capmodel.BusinessExtensionProviderEvidence{PluginID: "server-connector", Point: resourcecap.DynamicProviderPoint, ReleaseVersion: "v1.0.0", ContractVersion: "v1", ReleaseDigest: digestA, DeclarationDigest: digestB, AuthorizedMethods: []string{resourcecap.DynamicProviderMethodReadCatalogProduct}}}
	reader := NewLazy()
	var captured channelconfig.Configuration
	if err = reader.Bind(database, extensions, func(_ context.Context, configuration channelconfig.Configuration, _ string, _ []resourcecap.SecretBinding) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		captured = configuration
		return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
			return nil, gerror.New("unexpected direct outbound")
		}, func() {}, nil
	}); err != nil {
		t.Fatal(err)
	}
	source, err := NewCatalogSource(reader, &catalogAuthorizerStub{actor: 101})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := source.ReadCatalogProduct(ctx, interfaceNo, resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "101"})
	if err != nil {
		t.Fatal(err)
	}
	if captured.Config.Endpoint != "https://snapshot.example" || len(captured.Config.SecretBindings) != 1 || captured.Config.SecretBindings[0].Handle.OpaqueRef() != "snapshot-secret-ref" {
		t.Fatal("catalog mixed current channel with snapshot credentials")
	}
	if detail.ReadEvidence == nil || detail.ReadEvidence.InterfaceConfigurationRevision != revision || detail.ReadEvidence.InterfaceInstanceNo != interfaceNo || detail.ReadEvidence.Validate() != nil {
		t.Fatal("missing or mismatched resource-owner read evidence")
	}
	encoded, err := json.Marshal(detail)
	if err != nil || strings.Contains(string(encoded), revision) || strings.Contains(string(encoded), "ReadEvidence") {
		t.Fatal("internal read evidence reached browser JSON")
	}
	extensions.afterRead = func() error {
		_, writeErr := database.Exec(ctx, `UPDATE plugin_idc_resource_interface_channel_binding SET current_configuration_revision=?`, newerRevision)
		return writeErr
	}
	if _, err = source.ReadCatalogProduct(ctx, interfaceNo, resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "101"}); err == nil {
		t.Fatal("revision change during read was accepted")
	}
	if _, _, err = reader.invokeRead(ctx, resourceinterfaceprojectioncap.CandidateRequest{InterfaceInstanceNo: interfaceNo, ExpectedConfigurationRevision: revision, Method: resourcecap.DynamicProviderMethodReadCatalogProduct, Values: []byte(`{"reference":"101"}`)}, nil); err == nil {
		t.Fatal("pinned import read used another current revision")
	}
}

const catalogReadFixtureSQL = `
CREATE TABLE plugin_idc_resource_channel(tenant_id INT,channel_no TEXT,provider_id TEXT,provider_plugin_id TEXT,provider_runtime_type TEXT,provider_contract_version TEXT,provider_release_version TEXT,provider_release_digest TEXT,provider_declaration_digest TEXT,endpoint TEXT,non_secret_settings JSONB,status_code TEXT,deleted_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_resource_interface_channel_binding(tenant_id INT,interface_instance_no TEXT,channel_no TEXT,resource_plugin_id TEXT,resource_plugin_release_version TEXT,resource_contract_version TEXT,resource_authorized_methods TEXT[],resource_release_digest TEXT,resource_declaration_digest TEXT,product_plugin_id TEXT,product_plugin_release_version TEXT,product_contract_version TEXT,product_authorized_methods TEXT[],product_release_digest TEXT,product_declaration_digest TEXT,new_business_state TEXT,current_configuration_revision TEXT);
CREATE TABLE plugin_idc_resource_interface_configuration_revision(tenant_id INT,revision_no TEXT,interface_instance_no TEXT,channel_no TEXT,endpoint TEXT,non_secret_settings JSONB,secret_bindings JSONB);`
