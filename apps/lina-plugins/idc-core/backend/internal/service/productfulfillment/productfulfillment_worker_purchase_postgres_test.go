// This rollback-only integration test exercises the actual import lookup,
// immutable billing/configuration translation and confidential invocation fence.

package productfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

type peerPurchaseExtensions struct {
	pluginhost.BusinessExtensionRegistrar
	input       pluginhost.BusinessExtensionMethodInvokeInput
	calls       int
	fail        bool
	denyContext bool
}

func (e *peerPurchaseExtensions) WithConfidentialOutboundContext(ctx context.Context, input pluginhost.ConfidentialOutboundContextInput, action pluginhost.ConfidentialOutboundAction) error {
	if e.denyContext {
		return errors.New("exact context rejected")
	}
	if input.Handler == nil || input.Purpose != capmodel.BusinessExtensionInvocationPurposeExistingBusiness || !slices.Equal(input.AllowedResultFields, []string{"StatusCode", "Data", "Authenticated"}) {
		return errors.New("unscoped context")
	}
	return action(ctx, &capmodel.BusinessExtensionRuntimeContext{ContextID: "peer-confidential"})
}

func (e *peerPurchaseExtensions) InvokeProviderMethod(_ context.Context, input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	e.input = input
	e.calls++
	if e.fail {
		return nil, errors.New("response lost")
	}
	return &capmodel.BusinessExtensionMethodResult{Version: capmodel.BusinessExtensionInvocationProtocolV1, Payload: []byte(`{"version":"v1","status":"accepted","externalReference":"peer-order:19"}`)}, nil
}

type peerPurchaseOutbound struct {
	fail      bool
	input     resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest
	transfers int
}

func (o *peerPurchaseOutbound) TransferPurchaseFile(_ context.Context, input resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest, file resourceinterfaceprojectioncap.PurchaseFileTransfer) (string, error) {
	o.transfers++
	content, err := io.ReadAll(file.Body)
	if err != nil {
		return "", err
	}
	if input.PluginID != "idc-peer" || file.FieldToken != "upstream-field" || file.PublicSlug != "product" || int64(len(content)) != file.SizeBytes || sha256Hex(content) != file.SHA256Hex {
		return "", errors.New("invalid transferred evidence")
	}
	return "upstream-ready-token", nil
}

type peerConsumedFiles struct {
	purchaseconfigurationfile.ConsumedFileReader
	content []byte
}

func (f *peerConsumedFiles) VisitConsumedField(_ context.Context, scope purchaseconfigurationfile.ConsumedFileScope, field string, consume func(purchaseconfigurationfile.ConsumedFile, io.Reader) error) error {
	if scope.IntentNo != "intent-file" || field != "document" {
		return errors.New("wrong file scope")
	}
	return consume(purchaseconfigurationfile.ConsumedFile{Sequence: 1, FileName: "document.txt", MediaType: "text/plain", SizeBytes: int64(len(f.content)), SHA256Hex: sha256Hex(f.content)}, bytes.NewReader(f.content))
}

func (o *peerPurchaseOutbound) WithOutbound(ctx context.Context, input resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest, consume func(pluginhost.ConfidentialOutboundHandler) error) error {
	o.input = input
	if o.fail {
		return errors.New("resource disabled")
	}
	return consume(func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
		return &pluginhost.ConfidentialOutboundResult{}, nil
	})
}

func TestPeerPurchaseInvocationPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN not configured")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	rollback := errors.New("rollback")
	err = db.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		_, err := db.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_catalog_import_contract(tenant_id INT,version_no TEXT,interface_instance_no TEXT,snapshot JSONB,snapshot_digest TEXT,read_evidence JSONB);
CREATE TEMP TABLE plugin_idc_core_catalog_import_term(tenant_id INT,version_no TEXT,billing_rule_no TEXT,currency_code TEXT,upstream_term_reference TEXT);
CREATE TEMP TABLE plugin_idc_core_catalog_import_candidate(tenant_id INT,version_no TEXT,field_key TEXT,candidate_value TEXT,option_token TEXT);
CREATE TEMP TABLE plugin_idc_core_peer_purchase_file(tenant_id INT,intent_no TEXT,field_key TEXT,file_sequence INT,sha256_hex TEXT,upstream_file_token TEXT, UNIQUE(intent_no,field_key,file_sequence));
CREATE TEMP TABLE plugin_idc_core_product_fulfillment_task(tenant_id INT,task_no TEXT,lease_token TEXT,status_code TEXT,lease_expires_at TIMESTAMPTZ);
INSERT INTO plugin_idc_core_product_fulfillment_task VALUES(0,'task','lease','submitting',CURRENT_TIMESTAMP+INTERVAL '1 second');
INSERT INTO plugin_idc_core_catalog_import_term VALUES(0,'version','billing','CNY','old-term');`)
		if err != nil {
			return err
		}
		snapshot := channelcommercecap.ImportSnapshot{ProtocolVersion: "1", Product: channelcommercecap.CatalogProduct{PublicSlug: "product"}, ConfigurationRevision: "revision", PricingComplete: true, Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: "CNY", Precision: 2}}, Prices: []channelcommercecap.ImportPriceSchedule{{CurrencyCode: "CNY", TermReference: "old-term", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1, Base: []channelcommercecap.ImportPriceBand{{AmountMinor: 1000}}}}, Fulfillment: channelcommercecap.ImportFulfillment{Granularity: productfulfillmentcap.GranularityPerOrderItem, RenewalMode: productfulfillmentcap.RenewalModeUnavailable}}
		if err := snapshot.Validate(); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		claim := &claimedFulfillment{VersionNo: "version", BillingRuleNo: "billing", CurrencyCode: "CNY", PurchaseCycles: 3, Quantity: 2, CustomerConfiguration: json.RawMessage(`{}`), PluginID: "idc-peer", ReleaseVersion: "v0.1.0", ContractVersion: "v1", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), InterfaceInstanceNo: "pfi_11111111111111111111111111111111", ResourceInterfaceConfigurationRevision: "rci_22222222222222222222222222222222", Action: productfulfillmentcap.ActionSubmit}
		read := resourceinterfaceprojectioncap.CatalogReadEvidence{InterfaceInstanceNo: claim.InterfaceInstanceNo, OwnerID: resourceinterfaceprojectioncap.OwnerID, ResourceChannelRef: "CHN_peer", InterfaceConfigurationRevision: claim.ResourceInterfaceConfigurationRevision, PluginID: claim.PluginID, PluginReleaseVersion: claim.ReleaseVersion, ContractVersion: claim.ContractVersion, ReleaseDigest: claim.ReleaseDigest, DeclarationDigest: claim.DeclarationDigest, AuthorizedMethods: []string{"provision", "query"}}
		readRaw, err := json.Marshal(read)
		if err != nil {
			return err
		}
		if _, err = db.Model(do.CatalogImportContract{}).Ctx(ctx).Data(do.CatalogImportContract{TenantId: 0, VersionNo: "version", InterfaceInstanceNo: claim.InterfaceInstanceNo, Snapshot: string(raw), SnapshotDigest: sha256Hex(raw), ReadEvidence: string(readRaw)}).Insert(); err != nil {
			return err
		}
		extensions := &peerPurchaseExtensions{}
		outbound := &peerPurchaseOutbound{}
		worker := &fulfillmentWorker{database: db, extensions: extensions, peerOutbound: outbound}
		leaseClaim := &claimedFulfillment{TaskNo: "task", LeaseToken: "lease"}
		if !worker.refreshPeerLease(ctx, leaseClaim) {
			t.Fatal("active lease not renewed")
		}
		leaseClaim.LeaseToken = "other"
		if worker.refreshPeerLease(ctx, leaseClaim) {
			t.Fatal("foreign lease renewed")
		}
		claim.IntentNo = "intent-file"
		files := &peerConsumedFiles{content: bytes.Repeat([]byte("data"), 350000)}
		worker.consumedFiles = files
		fileValues := []configurationcontract.ConfigurationValue{{FieldKey: "document", FileTokens: []string{"local-token"}}}
		for range 2 {
			mapped := []channelcommercecap.PurchaseValue{{FieldToken: "upstream-field", Type: "file"}}
			if err = worker.transferPeerFiles(ctx, claim, &snapshot, &read, "provision", fileValues, mapped); err != nil {
				t.Fatal(err)
			}
			if len(mapped[0].FileTokens) != 1 || mapped[0].FileTokens[0] != "upstream-ready-token" {
				t.Fatal("wrong upstream file token")
			}
		}
		if outbound.transfers != 1 {
			t.Fatal("retry uploaded file twice")
		}
		files.content = []byte("tampered")
		if err = worker.transferPeerFiles(ctx, claim, &snapshot, &read, "provision", fileValues, []channelcommercecap.PurchaseValue{{FieldToken: "upstream-field"}}); err == nil {
			t.Fatal("changed file evidence reused token")
		}
		request := &productfulfillmentcap.Request{IdempotencyKey: "intent", Billing: &productfulfillmentcap.BillingSchedule{CurrencyCode: "CNY", ChargeType: productfulfillmentcap.BillingChargeRecurring, PeriodUnit: productfulfillmentcap.BillingPeriodMonth, PeriodCount: 1, PurchaseCycles: 3}}
		claim.ProvisionMethod = "provision"
		result, _, crossed, err := worker.invokeDynamicProvider(ctx, claim, request)
		if err != nil || !crossed || result.ExternalReference != "peer-order:19" {
			t.Fatalf("invoke failed: %#v %v %v", result, crossed, err)
		}
		var payload struct {
			PeerPurchase channelcommercecap.Purchase `json:"peerPurchase"`
		}
		if err = json.Unmarshal(extensions.input.Payload, &payload); err != nil {
			return err
		}
		if payload.PeerPurchase.Quantity != 2 || payload.PeerPurchase.PurchaseCycles != 3 || payload.PeerPurchase.BillingTermToken != "old-term" || extensions.input.RuntimeContext.ContextID != "peer-confidential" || outbound.input.ResourceChannelRef != "CHN_peer" {
			t.Fatalf("frozen wire mismatch: %+v %+v", payload, extensions.input)
		}
		outbound.fail = true
		if _, _, crossed, err = worker.invokePeerProvider(ctx, claim, request, "provision"); err == nil || crossed || extensions.calls != 1 {
			t.Fatal("disabled resource crossed provider boundary")
		}
		outbound.fail = false
		extensions.denyContext = true
		if _, _, crossed, err = worker.invokeDynamicProvider(ctx, claim, request); err == nil || crossed || extensions.calls != 1 {
			t.Fatal("rejected confidential identity crossed provider boundary")
		}
		extensions.denyContext = false
		extensions.fail = true
		if _, _, crossed, err = worker.invokePeerProvider(ctx, claim, request, "provision"); err == nil || !crossed {
			t.Fatal("uncertain write lost boundary marker")
		}
		claim.ResourceInterfaceConfigurationRevision = "rci_33333333333333333333333333333333"
		if _, _, crossed, err = worker.invokePeerProvider(ctx, claim, request, "provision"); err == nil || crossed || extensions.calls != 2 {
			t.Fatal("changed interface revision accepted")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}
