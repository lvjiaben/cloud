// This integration boundary starts from real paid commerce/entitlement facts.
// Typed compiler documents are fixture-owned; the compiler is a strict source
// substitute. Public target compilation has separate commerce-owner coverage.
package serveroperation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type nativeStructuredSourceFixture struct {
	*nativePricedResourceFixture
	target       servercommercialcap.TargetSnapshot
	compileCalls int
	reject       bool
}

func (f *nativeStructuredSourceFixture) CompileUpgrade(_ context.Context, in serveroperationresourcecap.UpgradeConfigurationRequest) (*serveroperationresourcecap.UpgradeConfiguration, error) {
	f.compileCalls++
	var current, target map[string]int
	if f.reject || in.ServiceNo != f.target.EntitlementNo || in.CustomerNo != f.target.CustomerNo || json.Unmarshal(in.CurrentConfiguration, &current) != nil || json.Unmarshal(in.TargetConfiguration, &target) != nil || current["cpu"] != 1 || target["cpu"] != 2 {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return &serveroperationresourcecap.UpgradeConfiguration{Route: serveroperationresourcecap.EntitlementRoute{ServiceNo: f.target.LocalResourceNo, ChannelNo: f.target.ChannelNo, PoolNo: f.target.PoolNo}, Parameters: json.RawMessage(`{"cpu":2}`)}, nil
}

func newNativeStructuredFixture(t *testing.T) (nativeCommercialFixture, *nativeStructuredSourceFixture) {
	t.Helper()
	f, base := newNativePricedFixture(t, 0)
	for _, migration := range []string{"072-sales-channel-foundation.sql", "094-generic-configuration-pricing.sql", "116-sales-management-pricing.sql", "127-product-commerce-workflow.sql"} {
		nativeLifecycleMigration(t, f.database, migration)
	}
	for _, statement := range []string{
		`INSERT INTO plugin_idc_core_product_automation_interface_instance(interface_instance_no,display_name,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,created_by,changed_by) VALUES('pfi_00000000000000000000000000000001','Native test','idc-resource','product-fulfillment','1','1',ARRAY['server.provision','server.query'],repeat('a',64),repeat('a',64),101,101)`,
		`INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding(version_no,interface_instance_no,interface_display_name_snapshot,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,bound_by) VALUES('ver_checkout_test','pfi_00000000000000000000000000000001','Native test','idc-resource','product-fulfillment','1','1',ARRAY['server.provision','server.query'],repeat('a',64),repeat('a',64),101)`,
		`INSERT INTO plugin_idc_core_product_automation_interface_configuration_revision(interface_instance_no,revision,schema_version,declaration_snapshot,public_value_snapshot,declaration_digest,value_digest,created_by) VALUES('pfi_00000000000000000000000000000001',1,'1','{}','{}',repeat('a',64),repeat('a',64),101)`,
		`INSERT INTO plugin_idc_core_catalog_product_version_configuration(version_no,interface_instance_no,interface_configuration_revision,product_schema_version,product_declaration_snapshot,merchant_value_snapshot,customer_schema_version,customer_declaration_snapshot,declaration_digest,merchant_value_digest,changed_by) VALUES('ver_checkout_test','pfi_00000000000000000000000000000001',1,'1','{}','{}','1','{}',repeat('a',64),repeat('a',64),101)`,
		`INSERT INTO plugin_idc_core_commerce_quote_charge_snapshot(quote_charge_no,quote_item_no,currency_code,line_number,charge_kind_code,charge_phase_code,quantity,charge_amount_minor,setup_fee_minor,priced_amount_minor,price_source_digest) VALUES('native-parent-charge','aqi_checkout_test','CNY',1,'base','periodic',1,800,200,1000,repeat('a',64))`,
		`INSERT INTO plugin_idc_core_commerce_structured_configuration_snapshot(snapshot_no,customer_no,uploader_user_id,quote_no,quote_item_no,product_no,version_no,configuration_revision,interface_instance_no,interface_configuration_revision,customer_declaration_digest,candidate_snapshot_digest,customer_value_digest,candidate_snapshot,typed_value_snapshot,content_digest,order_no,order_item_no,bound_at) SELECT 'SCF_native_parent',i.customer_no,101,'AQU_checkout_test','aqi_checkout_test',i.product_no,i.version_no,'revision','pfi_00000000000000000000000000000001',1,repeat('a',64),repeat('a',64),repeat('a',64),'{}','{"cpu":1}',repeat('a',64),i.order_no,i.order_item_no,CURRENT_TIMESTAMP FROM plugin_idc_core_service_entitlement i`,
		`UPDATE plugin_idc_core_service_entitlement SET configuration_snapshot_no='SCF_native_parent'`,
	} {
		if _, err := f.database.Exec(f.ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	source := &nativeStructuredSourceFixture{nativePricedResourceFixture: base, target: f.target}
	f.native.prices.source = source
	f.native.pricing.parameters = source
	return f, source
}

// Clone only immutable source document fields; each target has independent
// quote/item/SCF identities and its own exact entitlement binding.
func nativeStructuredTargetFixture(t *testing.T, f nativeCommercialFixture, quoteNo string, periodNet int64) string {
	t.Helper()
	quote, err := f.database.GetOne(f.ctx, `SELECT * FROM plugin_idc_core_commerce_quote_snapshot WHERE quote_no='AQU_checkout_test'`)
	if err != nil {
		t.Fatal(err)
	}
	data := make(map[string]any, len(quote))
	for key, value := range quote {
		if key != "id" {
			data[key] = value.Interface()
		}
	}
	data["quote_no"], data["purpose_code"], data["created_by"] = quoteNo, "change", 101
	if quoteNo == "target_reject" {
		data["source_code"] = "staff"
	}
	data["subtotal_minor"], data["net_amount_minor"], data["total_amount_minor"] = periodNet+500, periodNet+500, periodNet+500
	if _, err = f.database.Model("plugin_idc_core_commerce_quote_snapshot").Ctx(f.ctx).Data(data).Insert(); err != nil {
		t.Fatal(err)
	}
	item, err := f.database.GetOne(f.ctx, `SELECT * FROM plugin_idc_core_commerce_quote_item_snapshot WHERE quote_item_no='aqi_checkout_test'`)
	if err != nil {
		t.Fatal(err)
	}
	data = make(map[string]any, len(item))
	for key, value := range item {
		if key != "id" {
			data[key] = value.Interface()
		}
	}
	itemNo := "item_" + quoteNo
	data["quote_item_no"], data["quote_no"] = itemNo, quoteNo
	data["setup_subtotal_minor"], data["period_subtotal_minor"], data["subtotal_minor"], data["net_amount_minor"], data["total_amount_minor"] = int64(500), periodNet, periodNet+500, periodNet+500, periodNet+500
	if _, err = f.database.Model("plugin_idc_core_commerce_quote_item_snapshot").Ctx(f.ctx).Data(data).Insert(); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO plugin_idc_core_commerce_quote_item_quantity_snapshot(quote_item_no,quantity,content_digest) VALUES(?,1,repeat('a',64))`, []any{itemNo}},
		{`INSERT INTO plugin_idc_core_commerce_quote_charge_snapshot(quote_charge_no,quote_item_no,currency_code,line_number,charge_kind_code,charge_phase_code,quantity,charge_amount_minor,setup_fee_minor,priced_amount_minor,price_source_digest) VALUES(?,?,'CNY',1,'base','periodic',1,?,500,?,repeat('a',64))`, []any{"charge_" + quoteNo, itemNo, periodNet, periodNet + 500}},
		{`INSERT INTO plugin_idc_core_commerce_structured_configuration_snapshot(snapshot_no,customer_no,uploader_user_id,quote_no,quote_item_no,product_no,version_no,configuration_revision,interface_instance_no,interface_configuration_revision,customer_declaration_digest,candidate_snapshot_digest,customer_value_digest,candidate_snapshot,typed_value_snapshot,content_digest) VALUES(?,'CUS_checkout_test',101,?,?,'prd_checkout_test','ver_checkout_test','revision','pfi_00000000000000000000000000000001',1,repeat('a',64),repeat('a',64),repeat('a',64),'{}','{"cpu":2}',repeat('a',64))`, []any{"scf_" + quoteNo, quoteNo, itemNo}},
	} {
		if _, err = f.database.Exec(f.ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, _ := json.Marshal(f.target)
	if _, err = f.database.Exec(f.ctx, `INSERT INTO plugin_idc_core_native_server_target_quote(target_quote_no,quote_item_no,structured_configuration_no,entitlement_no,customer_no,expected_entitlement_version,customer_subject_version,currency_code,native_target_snapshot) VALUES(?,?,?,?,?,?,1,'CNY',?::JSONB)`, quoteNo, itemNo, "scf_"+quoteNo, f.target.EntitlementNo, f.target.CustomerNo, f.target.EntitlementVersion, string(snapshot)); err != nil {
		t.Fatal(err)
	}
	return quoteNo
}

func TestNativeStructuredResizePaidFactsAndProjectionPostgreSQL(t *testing.T) {
	f, source := newNativeStructuredFixture(t)
	targetQuote := nativeStructuredTargetFixture(t, f, "target_native", 2000)
	quote, err := f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionResize, Resize: &servercommercialcap.ResizeInput{TargetQuoteNo: targetQuote}, IdempotencyKey: "native-structured-resize"})
	if err != nil {
		t.Fatalf("structured quote: %+v", err)
	}
	q, err := f.native.loadNativePricedQuote(f.ctx, quote.QuoteNo, f.target.CustomerNo, "")
	if err != nil {
		t.Fatal(err)
	}
	if q.Price.CurrentRecurringMinor != 800 || q.Price.TargetNetMinor != 2000 || q.Price.TargetRecurringMinor != 2000 || q.Price.TotalAmountMinor > 1200 || q.Price.TotalAmountMinor < 1199 || source.compileCalls != 1 || q.TargetConfigurationNo != "CFG_checkout_test" || q.Price.TargetStructuredConfigurationNo != "scf_target_native" {
		t.Fatalf("setup/catalog leaked into difference: %+v", q.Price)
	}
	assertNativeResizeCyclePersistence(t, f, q)
	if _, err = f.native.Quote(f.ctx, servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionResize, Resize: &servercommercialcap.ResizeInput{TargetQuoteNo: targetQuote}, IdempotencyKey: "duplicate-target-new-key"}); err == nil {
		t.Fatal("one target quote admitted a second commercial quote")
	}
	p, err := f.native.Checkout(f.ctx, servercommercialcap.CheckoutRequest{Target: f.target, QuoteNo: quote.QuoteNo, IdempotencyKey: "native-structured-checkout"})
	if err != nil {
		t.Fatalf("structured checkout: %+v", err)
	}
	nativeRecoverySettle(t, f, p, false)
	if err = f.native.WithReadyOperation(f.ctx, p.OperationNo, func(context.Context, servercommercialcap.Operation) (string, error) { return "native-resize-task", nil }); err != nil {
		t.Fatal(err)
	}
	completion := servercommercialcap.Completion{OperationNo: p.OperationNo, EntitlementNo: f.target.EntitlementNo, TaskNo: "native-resize-task", Status: servercommercialcap.CompletionSucceeded, EvidenceDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ObservedAt: time.Now().UTC()}
	if err = f.native.Complete(f.ctx, completion); err != nil {
		t.Fatalf("resize complete: %+v", err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_service_entitlement e JOIN plugin_idc_core_service_price_snapshot price ON price.price_snapshot_no=e.current_price_snapshot_no WHERE e.configuration_snapshot_no='scf_target_native' AND price.structured_configuration_snapshot_no=e.configuration_snapshot_no AND price.phase_code='resize_target' AND price.net_amount_minor=2000`, 1)
	if err = f.native.Complete(f.ctx, completion); err != nil {
		t.Fatal(err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_service_price_snapshot WHERE phase_code='resize_target'`, 1)
	currentContext := q.Price.Context
	currentContext.EntitlementVersion++
	basis, err := f.native.prices.paidPeriod(f.ctx, currentContext)
	if err != nil || basis.NetAmountMinor != 2000 || !basis.StartedAt.Equal(q.Price.Context.StartedAt) {
		t.Fatalf("next upgrade lost current-cycle paid basis: %+v %v", basis, err)
	}
}

// A writer-level reverse check keeps the frozen paid-cycle count in both
// ordinary quote evidence and the difference price snapshot. It is rolled back.
func assertNativeResizeCyclePersistence(t *testing.T, f nativeCommercialFixture, original *nativePricedQuote) {
	t.Helper()
	checked := false
	err := f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
		q := *original
		p := *original.Price
		q.Price = &p
		p.PurchaseCycles = 3
		q.QuoteNo = "native_three_cycle_writer"
		var err error
		q.ContentDigest, err = nativePricedContent(&q)
		if err != nil {
			return err
		}
		target, err := f.native.readNativeCommercialTarget(ctx, f.target, false)
		if err != nil {
			return err
		}
		if err = f.native.persistNativePricedCommerce(ctx, &q, target); err != nil {
			return err
		}
		matches, err := f.database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_commerce_quote_item_snapshot item JOIN plugin_idc_core_service_price_snapshot price ON price.source_quote_no=item.quote_no AND price.source_quote_item_no=item.quote_item_no WHERE item.quote_no=? AND item.purchase_cycles=3 AND price.purchase_cycles=3 AND price.phase_code='resize_charge'`, q.QuoteNo)
		if err != nil {
			return err
		}
		if matches.Int() != 1 {
			return ErrOperationConflict
		}
		checked = true
		return ErrInvalidInput
	})
	if !checked || err != ErrInvalidInput {
		t.Fatalf("paid-cycle writer lost original count: checked=%v error=%+v", checked, err)
	}
}

func TestNativeStructuredResizeRejectsUnboundAndCompilerFailurePostgreSQL(t *testing.T) {
	f, source := newNativeStructuredFixture(t)
	targetQuote := nativeStructuredTargetFixture(t, f, "target_reject", 2000)
	input := servercommercialcap.QuoteRequest{Target: f.target, Action: serveroperationresourcecap.ActionResize, Resize: &servercommercialcap.ResizeInput{TargetQuoteNo: targetQuote}, IdempotencyKey: "native-structured-reject"}
	source.reject = true
	if _, err := f.native.Quote(f.ctx, input); err == nil {
		t.Fatal("unsupported compiler result admitted")
	}
	if source.compileCalls != 1 {
		t.Fatal("staff target never reached actual compiler boundary")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_channel_service_quote WHERE source_code='native'`, 0)
	source.reject = false
	negative := nativeStructuredTargetFixture(t, f, "target_negative", 500)
	input.Resize = &servercommercialcap.ResizeInput{TargetQuoteNo: negative}
	input.IdempotencyKey = "negative-paid-difference"
	if _, err := f.native.Quote(f.ctx, input); err == nil {
		t.Fatal("negative paid-period difference admitted")
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_channel_service_quote WHERE source_code='native'`, 0)
	input.Resize = &servercommercialcap.ResizeInput{Selections: []servercommercialcap.ResizeSelection{{OptionNo: "cpu-2", Quantity: 1}}}
	if _, err := f.native.Quote(f.ctx, input); err == nil {
		t.Fatal("legacy selections became generic target")
	}
	if err := f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
		_, err := f.database.Exec(ctx, `UPDATE plugin_idc_core_native_server_target_quote SET entitlement_no='foreign' WHERE target_quote_no=?`, targetQuote)
		return err
	}); err == nil {
		t.Fatal("target entitlement binding was mutable")
	}
}
