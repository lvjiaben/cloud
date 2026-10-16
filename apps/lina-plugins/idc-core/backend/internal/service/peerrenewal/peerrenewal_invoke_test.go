// These rollback-only tests exercise exact entitlement reconstruction and prove
// that uncertain provider writes become unknown rather than eligible for replay.
package peerrenewal

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/renewalownercap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

type testExtensions struct {
	pluginhost.BusinessExtensionRegistrar
	fail        bool
	denyContext bool
	calls       int
	input       pluginhost.BusinessExtensionMethodInvokeInput
}

func (e *testExtensions) InvokeProviderMethod(_ context.Context, input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	e.calls++
	e.input = input
	if e.fail {
		return nil, errors.New("response lost")
	}
	var request renewalownercap.PeerRequest
	if err := json.Unmarshal(input.Payload, &request); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(renewalownercap.Task{TaskNo: request.Request.OperationNo, Status: renewalownercap.TaskStatusSucceeded, EvidenceDigest: strings.Repeat("a", 64), ObservedAt: time.Now()})
	return &capmodel.BusinessExtensionMethodResult{Version: capmodel.BusinessExtensionInvocationProtocolV1, Payload: raw}, err
}

func (e *testExtensions) WithConfidentialOutboundContext(ctx context.Context, input pluginhost.ConfidentialOutboundContextInput, action pluginhost.ConfidentialOutboundAction) error {
	if e.denyContext {
		return errors.New("exact context rejected")
	}
	if input.Handler == nil || input.ProviderMethod == "" || input.Purpose != capmodel.BusinessExtensionInvocationPurposeExistingBusiness || !slices.Equal(input.AllowedResultFields, []string{"StatusCode", "Data", "Authenticated"}) {
		return errors.New("missing governed outbound context")
	}
	return action(ctx, &capmodel.BusinessExtensionRuntimeContext{ContextID: "outbound-only"})
}

type testOutbound struct {
	fail  bool
	calls int
	input resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest
}

func (o *testOutbound) WithOutbound(ctx context.Context, input resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest, consume func(pluginhost.ConfidentialOutboundHandler) error) error {
	o.calls++
	o.input = input
	if o.fail {
		return errors.New("disabled resource")
	}
	return consume(func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
		return &pluginhost.ConfidentialOutboundResult{}, nil
	})
}

func TestPeerRenewalFrozenInvocationPostgreSQL(t *testing.T) {
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
	err = db.Transaction(context.Background(), func(ctx context.Context, _ gdb.TX) error {
		_, err := db.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_business_operation(tenant_id INT,operation_no TEXT,price_snapshot_no TEXT,resource_owner_code TEXT,subject_no TEXT,customer_no TEXT,expected_subject_version BIGINT,operation_type_code TEXT,execution_mode_code TEXT,status_code TEXT);
 CREATE TEMP TABLE plugin_idc_core_service_entitlement(tenant_id INT,entitlement_no TEXT,customer_no TEXT,plugin_id TEXT,plugin_release_version TEXT,contract_version TEXT,declaration_digest TEXT,interface_instance_no TEXT,source_intent_no TEXT,order_no TEXT,order_item_no TEXT,configuration_snapshot_no TEXT,unit_sequence INT,status_code TEXT,renewal_mode_code TEXT,version BIGINT,version_no TEXT,upstream_order_reference TEXT,upstream_service_no TEXT);
 CREATE TEMP TABLE plugin_idc_core_service_price_snapshot(tenant_id INT,price_snapshot_no TEXT,subject_no TEXT,customer_no TEXT,original_expires_at TIMESTAMPTZ,projected_expires_at TIMESTAMPTZ,purchase_cycles INT,period_count INT,period_unit_code TEXT,currency_code TEXT);
 CREATE TEMP TABLE plugin_idc_core_product_fulfillment_intent(tenant_id INT,intent_no TEXT,order_no TEXT,order_item_no TEXT,unit_sequence INT,release_digest TEXT,renewal_method TEXT,interface_configuration_revision BIGINT);
 CREATE TEMP TABLE plugin_idc_core_product_fulfillment_task(tenant_id INT,intent_no TEXT,status_code TEXT,external_reference TEXT);
 CREATE TEMP TABLE plugin_idc_core_catalog_import_contract(tenant_id INT,version_no TEXT,interface_instance_no TEXT,read_evidence JSONB);
 CREATE TEMP TABLE plugin_idc_core_product_automation_interface_configuration_revision(tenant_id INT,interface_instance_no TEXT,revision BIGINT,resource_owner_configuration_revision TEXT);
 INSERT INTO plugin_idc_core_product_automation_interface_configuration_revision VALUES(0,'pfi_11111111111111111111111111111111',7,'rci_22222222222222222222222222222222');
 INSERT INTO plugin_idc_core_business_operation VALUES(0,'op','price','idc-peer','ent','customer',2,'renewal','provider_confirmed','ready');
 INSERT INTO plugin_idc_core_service_entitlement VALUES(0,'ent','customer','idc-peer','v0.1.0','v1',repeat('a',64),'pfi_11111111111111111111111111111111','intent','order','item','config',2,'active','provider_confirmed',2,'product-version','peer-order:42','exact_second');
 INSERT INTO plugin_idc_core_service_price_snapshot VALUES(0,'price','ent','customer','2026-10-01T00:00:00Z','2026-11-01T00:00:00Z',1,1,'month','CNY');
 INSERT INTO plugin_idc_core_product_fulfillment_intent VALUES(0,'intent','order','item',2,repeat('b',64),'renew',7);
 INSERT INTO plugin_idc_core_product_fulfillment_task VALUES(0,'intent','succeeded','peer-order:42');`)
		if err != nil {
			t.Fatal(err)
		}
		read := resourceinterfaceprojectioncap.CatalogReadEvidence{InterfaceInstanceNo: "pfi_11111111111111111111111111111111", OwnerID: resourceinterfaceprojectioncap.OwnerID, ResourceChannelRef: "CHN_peer", InterfaceConfigurationRevision: "rci_22222222222222222222222222222222", PluginID: "idc-peer", PluginReleaseVersion: "v0.1.0", ContractVersion: "v1", ReleaseDigest: strings.Repeat("b", 64), DeclarationDigest: strings.Repeat("a", 64), AuthorizedMethods: []string{"renew", "renew.query"}}
		readRaw, marshalErr := json.Marshal(read)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_import_contract VALUES(0,'product-version','pfi_11111111111111111111111111111111',?::jsonb)`, readRaw); err != nil {
			t.Fatal(err)
		}
		outbound := &testOutbound{}
		extensions := &testExtensions{}
		source := &source{database: db, extensions: extensions, outbound: outbound}
		request := renewalownercap.TaskRequest{Version: "v1", OwnerID: "idc-peer", OperationNo: "op", EntitlementNo: "ent", PriceSnapshotNo: "price", PluginID: "idc-peer", PluginReleaseVersion: "v0.1.0", ContractVersion: "v1", DeclarationDigest: strings.Repeat("a", 64), RenewalMethod: "renew", ExpectedEntitlementVersion: 2, OriginalExpiresAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), ProjectedExpiresAt: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
		task, err := source.CreateTask(ctx, request)
		if err != nil || task.Status != renewalownercap.TaskStatusSucceeded {
			t.Fatalf("submit: %+v %v", task, err)
		}
		var payload renewalownercap.PeerRequest
		if err = json.Unmarshal(extensions.input.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if extensions.input.RuntimeContext.ContextID != "outbound-only" || outbound.input.ResourceChannelRef != "CHN_peer" || payload.ExternalReference != "peer-order:42" || payload.UpstreamServiceNo != "exact_second" || payload.PurchaseCycles != 1 {
			t.Fatalf("frozen routing: %+v", payload)
		}
		if _, err = source.ReadTask(ctx, renewalownercap.TaskReadRequest{TaskNo: "op", Request: request}); err != nil || extensions.input.Method != renewalownercap.PeerRenewQueryMethod {
			t.Fatalf("query: %v", err)
		}
		extensions.fail = true
		task, err = source.CreateTask(ctx, request)
		if err != nil || task.Status != renewalownercap.TaskStatusResultUnknown || task.FailureCode == "" {
			t.Fatalf("unknown replay protection: %+v %v", task, err)
		}
		calls := extensions.calls
		extensions.denyContext = true
		if _, err = source.CreateTask(ctx, request); !errors.Is(err, renewalownercap.ErrOwnerUnavailable) || extensions.calls != calls {
			t.Fatalf("disabled context crossed boundary: %v", err)
		}
		extensions.denyContext = false
		outbound.fail = true
		if _, err = source.CreateTask(ctx, request); !errors.Is(err, renewalownercap.ErrOwnerUnavailable) || extensions.calls != calls {
			t.Fatalf("unavailable outbound crossed boundary: %v", err)
		}
		outbound.fail = false
		request.ExpectedEntitlementVersion = 3
		if _, err = source.CreateTask(ctx, request); !errors.Is(err, renewalownercap.ErrTaskConflict) || extensions.calls != calls {
			t.Fatalf("stale facts crossed boundary: %v", err)
		}
		request.ExpectedEntitlementVersion = 2
		if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_task VALUES(0,'intent','succeeded','peer-service:ambiguous')`); err != nil {
			t.Fatal(err)
		}
		if _, err = source.CreateTask(ctx, request); !errors.Is(err, renewalownercap.ErrTaskConflict) || extensions.calls != calls {
			t.Fatalf("ambiguous task accepted: %v", err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}
