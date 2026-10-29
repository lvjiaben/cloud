// Tests price preparation with isolated core facts and bounded catalog/resource
// owners. Quotes never execute a provider write or create payment evidence.
package serveroperation

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"testing"
	"time"
)

type nativePriceCatalogFixture struct {
	catalog.CatalogReader
	catalog.RenewalQuoteAssembler
	quote        *catalog.PublicConfigurationQuote
	resolved     *catalog.ServerResizeConfiguration
	line         catalog.PurchaseQuoteLine
	renewalInput catalog.RenewalQuoteInput
}

func (f *nativePriceCatalogFixture) QuoteConfiguration(context.Context, catalog.PublicConfigurationQuoteInput) (*catalog.PublicConfigurationQuote, error) {
	return f.quote, nil
}
func (f *nativePriceCatalogFixture) ResolveServerResizeConfiguration(context.Context, catalog.ServerResizeConfigurationInput) (*catalog.ServerResizeConfiguration, error) {
	return f.resolved, nil
}
func (f *nativePriceCatalogFixture) LoadConfigurationPriceSelections(context.Context, []string) (map[string][]catalog.ConfigurationPriceSelection, error) {
	return map[string][]catalog.ConfigurationPriceSelection{"cfg": {{DimensionNo: "cpu", Quantity: 1}}}, nil
}
func (f *nativePriceCatalogFixture) AssembleRenewalQuote(_ context.Context, in catalog.RenewalQuoteInput) ([]catalog.PurchaseQuoteLine, error) {
	f.renewalInput = in
	return []catalog.PurchaseQuoteLine{f.line}, nil
}

type nativePriceSourceFixture struct {
	serveroperationresourcecap.Source
	projection  *serveroperationresourcecap.AccessoryProjection
	reservation *serveroperationresourcecap.IPReservation
	reserved    []serveroperationresourcecap.IPReservationRequest
	released    []serveroperationresourcecap.IPReservationReleaseRequest
	validated   int
}

func (f *nativePriceSourceFixture) GetAccessories(_ context.Context, route serveroperationresourcecap.CapabilityRoute) (*serveroperationresourcecap.AccessoryProjection, error) {
	if f.projection == nil || route.ServiceNo != f.projection.ServiceNo {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	return f.projection, nil
}
func (f *nativePriceSourceFixture) ValidateParameters(_ context.Context, route serveroperationresourcecap.CapabilityRoute, _ serveroperationresourcecap.Action, _ json.RawMessage) error {
	if f.projection != nil && route.ServiceNo != f.projection.ServiceNo {
		return serveroperationresourcecap.ErrCapabilityUnavailable
	}
	f.validated++
	return nil
}
func (f *nativePriceSourceFixture) ReserveIP(_ context.Context, in serveroperationresourcecap.IPReservationRequest) (*serveroperationresourcecap.IPReservation, error) {
	f.reserved = append(f.reserved, in)
	if f.reservation != nil {
		return f.reservation, nil
	}
	return &serveroperationresourcecap.IPReservation{ReservationNo: "reservation", CapacityObservationNo: "capacity", Quantity: in.Quantity, Status: serveroperationresourcecap.IPReservationReserved, ExpiresAt: in.ExpiresAt}, nil
}
func (f *nativePriceSourceFixture) ReleaseIP(_ context.Context, in serveroperationresourcecap.IPReservationReleaseRequest) (*serveroperationresourcecap.IPReservation, error) {
	f.released = append(f.released, in)
	return nil, nil
}
func (f *nativePriceSourceFixture) ConsumeIP(context.Context, serveroperationresourcecap.IPReservationConsumeRequest) (*serveroperationresourcecap.IPReservation, error) {
	panic("pricing must never consume capacity")
}

func TestNativePriceBuilderRequiresExplicitDependencies(t *testing.T) {
	var reader *nativePriceCatalogFixture
	if _, err := NewNativePriceBuilder(nil, reader, &nativePriceSourceFixture{}); err == nil {
		t.Fatal("accepted missing startup graph")
	}
	source := &nativePriceSourceFixture{}
	builder := &NativePriceBuilder{source: source}
	if _, err := builder.PrepareIPChange(context.Background(), NativePriceContext{}, "native-key", time.Now()); err == nil || len(source.reserved) != 0 {
		t.Fatal("unbound entitlement reached reservation")
	}
}

func nativePriceFixture(t *testing.T) (context.Context, gdb.DB, *NativePriceBuilder, *nativePriceCatalogFixture, *nativePriceSourceFixture, NativePriceContext, time.Time) {
	t.Helper()
	ctx, db := newBatchDirectoryDatabase(t)
	const fixture = `CREATE TABLE plugin_idc_core_catalog_tax_policy(id BIGSERIAL,tenant_id BIGINT DEFAULT 0,tax_policy_no TEXT,version_no TEXT,currency_code TEXT,tax_treatment_code TEXT,tax_rate_basis_points INT,tax_category_code TEXT,rounding_scope_code TEXT,rounding_mode_code TEXT,rounding_quantum_minor BIGINT,price_includes_tax BOOLEAN);
 INSERT INTO plugin_idc_core_catalog_tax_policy(tax_policy_no,version_no,currency_code,tax_treatment_code,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor,price_includes_tax) VALUES('tax','version','CNY','exempt','cloud','line','half_up',1,TRUE);
 CREATE TABLE plugin_idc_core_commerce_configuration_snapshot_item(id BIGSERIAL,tenant_id BIGINT DEFAULT 0,configuration_snapshot_no TEXT,group_no TEXT,option_no TEXT,group_code TEXT,option_code TEXT,component_type_code TEXT,group_name_zh_cn TEXT,group_name_en_us TEXT,option_name_zh_cn TEXT,option_name_en_us TEXT,normalized_value JSONB,quantity INT,sort_order INT);
 INSERT INTO plugin_idc_core_commerce_configuration_snapshot_item(configuration_snapshot_no,group_no,option_no,group_code,option_code,component_type_code,normalized_value,quantity,sort_order) VALUES('cfg','cpu','cpu-1','cpu','one','cpu','{"cores":1}',1,1),('cfg','disk','disk-80','disk','80','system_disk','{"gigabytes":80}',1,2);
 CREATE TABLE plugin_idc_core_server_ip_change_policy(tenant_id BIGINT DEFAULT 0,policy_no TEXT,currency_code TEXT,current_version BIGINT);
 CREATE TABLE plugin_idc_core_server_ip_change_policy_version(tenant_id BIGINT DEFAULT 0,policy_no TEXT,policy_version BIGINT,enabled BOOLEAN,free_changes_per_window INT,window_days INT,max_changes_per_day INT,amount_minor BIGINT,maximum_snapshot_age_seconds INT);
 INSERT INTO plugin_idc_core_server_ip_change_policy(policy_no,currency_code,current_version) VALUES('ip-policy','CNY',1);
 INSERT INTO plugin_idc_core_server_ip_change_policy_version(policy_no,policy_version,enabled,free_changes_per_window,window_days,max_changes_per_day,amount_minor,maximum_snapshot_age_seconds) VALUES('ip-policy',1,TRUE,1,30,3,100,300);
 CREATE TABLE plugin_idc_core_business_operation(tenant_id BIGINT DEFAULT 0,subject_type_code TEXT,subject_no TEXT,operation_type_code TEXT,created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP);`
	if _, err := db.Exec(ctx, fixture); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	start := now.Add(-20 * 24 * time.Hour)
	end := now.Add(10 * 24 * time.Hour)
	const paidFixture = `CREATE TABLE plugin_idc_core_service_entitlement(tenant_id INT DEFAULT 0,entitlement_no TEXT,customer_no TEXT,currency_code TEXT,version BIGINT,order_no TEXT,order_item_no TEXT,current_price_snapshot_no TEXT,started_at TIMESTAMPTZ);
 CREATE TABLE plugin_idc_core_commerce_order_item(tenant_id INT DEFAULT 0,order_no TEXT,order_item_no TEXT,quote_item_no TEXT);
 CREATE TABLE plugin_idc_core_service_price_snapshot(tenant_id INT DEFAULT 0,price_snapshot_no TEXT,subject_type_code TEXT,subject_no TEXT,customer_no TEXT,source_quote_no TEXT,source_quote_item_no TEXT,phase_code TEXT,net_amount_minor BIGINT,total_amount_minor BIGINT,original_expires_at TIMESTAMPTZ);
 CREATE TABLE plugin_idc_core_channel_service_quote(tenant_id INT DEFAULT 0,quote_no TEXT,prepared_price JSONB,source_code TEXT,action_code TEXT);
 CREATE TABLE plugin_idc_core_commerce_quote_item_snapshot(tenant_id INT DEFAULT 0,quote_item_no TEXT,product_no TEXT,version_no TEXT,billing_rule_no TEXT,purchase_cycles INT,period_unit_code TEXT,period_count INT,tax_policy_no TEXT,price_includes_tax BOOLEAN,tax_treatment_code TEXT,tax_rate_basis_points INT,tax_category_code TEXT,rounding_scope_code TEXT,rounding_mode_code TEXT,rounding_quantum_minor BIGINT);
 CREATE TABLE plugin_idc_core_commerce_quote_item_quantity_snapshot(tenant_id INT DEFAULT 0,quote_item_no TEXT,quantity INT);
 CREATE TABLE plugin_idc_core_commerce_quote_charge_snapshot(tenant_id INT DEFAULT 0,quote_item_no TEXT,charge_kind_code TEXT,option_no TEXT,configuration_price_dimension_no TEXT,quantity INT,charge_amount_minor BIGINT,setup_fee_minor BIGINT,priced_amount_minor BIGINT,price_source_digest TEXT,line_number INT);
 CREATE TABLE plugin_idc_core_commerce_quote_adjustment_snapshot(tenant_id INT DEFAULT 0,quote_item_no TEXT,adjustment_type_code TEXT,source_reference TEXT,source_version_digest TEXT,amount_minor BIGINT,original_period_amount_minor BIGINT,target_period_amount_minor BIGINT,application_order INT);
 INSERT INTO plugin_idc_core_commerce_order_item(order_no,order_item_no,quote_item_no) VALUES('paid-order','paid-item','paid-quote-item');
 INSERT INTO plugin_idc_core_commerce_quote_item_snapshot(quote_item_no,product_no,version_no,billing_rule_no,purchase_cycles,period_unit_code,period_count,tax_policy_no,price_includes_tax,tax_treatment_code,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor) VALUES('paid-quote-item','product','version','monthly',1,'month',1,'tax',TRUE,'exempt','cloud','line','half_up',1);
 INSERT INTO plugin_idc_core_commerce_quote_item_quantity_snapshot(quote_item_no,quantity) VALUES('paid-quote-item',1);
 INSERT INTO plugin_idc_core_commerce_quote_charge_snapshot(quote_item_no,charge_kind_code,quantity,charge_amount_minor,setup_fee_minor,priced_amount_minor,line_number) VALUES('paid-quote-item','base',1,1000,700,1700,1);`
	if _, err := db.Exec(ctx, paidFixture); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,customer_no,currency_code,version,order_no,order_item_no,started_at) VALUES('ENT-native','customer','CNY',2,'paid-order','paid-item',?)`, start); err != nil {
		t.Fatal(err)
	}
	value := NativePriceContext{EntitlementNo: "ENT-native", CustomerNo: "customer", CurrencyCode: "CNY", ProductNo: "product", VersionNo: "version", BillingRuleNo: "monthly", PriceConfigurationNo: "cfg", PublicSlug: "server", PeriodUnitCode: "month", PeriodCount: 1, LocalResourceNo: "resource-local", ChannelNo: "supplier-channel", PoolNo: "pool", EntitlementVersion: 2, CustomerVersion: 3, StartedAt: start, ExpiresAt: &end}
	total, net, tax := int64(2001), int64(2001), int64(0)
	taxSnapshot := catalog.PublicQuoteTaxSnapshot{PriceIncludesTax: true, TaxTreatment: catalog.TaxTreatmentExempt, TaxCategoryCode: "cloud", RoundingScope: catalog.TaxRoundingScopeLine, RoundingMode: catalog.TaxRoundingHalfUp, RoundingQuantumMinor: 1}
	reader := &nativePriceCatalogFixture{quote: &catalog.PublicConfigurationQuote{Tax: taxSnapshot, Terms: []catalog.PublicQuoteTerm{{TermNo: "monthly", Status: catalog.PublicQuoteFinal, ChargeType: catalog.ChargeTypeRecurring, PeriodUnit: catalog.PeriodUnitMonth, PeriodCount: 1, TotalAmountMinor: &total, NetAmountMinor: &net, TaxAmountMinor: &tax}}}, resolved: &catalog.ServerResizeConfiguration{ProductNo: "product", VersionNo: "version", BillingRuleNo: "monthly", CurrencyCode: "CNY", Items: []catalog.PurchaseConfigurationItem{{GroupCode: "cpu", OptionNo: "cpu-2", ComponentTypeCode: "cpu", NormalizedValue: `{"cores":2}`, Quantity: 1}, {GroupCode: "disk", OptionNo: "disk-80", ComponentTypeCode: "system_disk", NormalizedValue: `{"gigabytes":80}`, Quantity: 1}}}, line: catalog.PurchaseQuoteLine{PeriodUnitCode: "month", PriceIncludesTax: true, TaxTreatmentCode: "exempt", TaxCategoryCode: "cloud", RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1, Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 1, ChargeAmountMinor: 1000, PricedAmountMinor: 1000}}}}
	reader.quote.Terms[0].Lines = []catalog.PublicQuoteLine{{Kind: "base", Quantity: 1, ChargeAmountMinor: 2001, SetupFeeMinor: 800, PricedAmountMinor: 2801}}
	source := &nativePriceSourceFixture{projection: &serveroperationresourcecap.AccessoryProjection{ServiceNo: value.LocalResourceNo, SnapshotNo: "accessory-snapshot", ObservedAt: now, ExpiresAt: now.Add(time.Hour), Actions: []serveroperationresourcecap.Action{serveroperationresourcecap.ActionChangeIP}, Items: []serveroperationresourcecap.AccessoryItem{{ObjectNo: "ip1", Kind: serveroperationresourcecap.AccessoryKindIP, State: "active", Projection: json.RawMessage(`{"address":"192.0.2.1"}`)}}}}
	builder, err := NewNativePriceBuilder(db, reader, source)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, db, builder, reader, source, value, now
}

func TestNativePriceResizePreservesEntitlementAndProrationPostgreSQL(t *testing.T) {
	ctx, _, builder, reader, source, value, now := nativePriceFixture(t)
	input := NativePriceResizeInput{CurrencyCode: "CNY", TermNo: "monthly", Selections: []catalog.PublicConfigurationSelection{{OptionNo: "cpu-2", Quantity: 1}}}
	quote, err := builder.PrepareResize(ctx, value, input, now)
	if err != nil {
		t.Fatal(err)
	}
	if quote.Direction != "upgrade" || quote.TotalAmountMinor != 334 || quote.CatalogChargeMinor != 334 || quote.CurrentRecurringMinor != 1000 || quote.TargetRecurringMinor != 2001 || string(quote.Parameters) != `{"cpu":2}` || !quote.EffectiveAt.Equal(now) || quote.Context.EntitlementNo != "ENT-native" || source.validated != 1 {
		t.Fatalf("wrong native upgrade %+v", quote)
	}
	if len(reader.renewalInput.Lines) != 0 {
		t.Fatal("current paid price was replaced by today's renewal quote")
	}
	lower := int64(500)
	reader.quote.Terms[0].TotalAmountMinor = &lower
	reader.quote.Terms[0].NetAmountMinor = &lower
	reader.quote.Terms[0].Lines[0].ChargeAmountMinor = lower
	reader.quote.Terms[0].Lines[0].PricedAmountMinor = lower + reader.quote.Terms[0].Lines[0].SetupFeeMinor
	quote, err = builder.PrepareResize(ctx, value, input, now)
	if err != nil || quote.Direction != "downgrade" || quote.TotalAmountMinor != 0 || !quote.EffectiveAt.Equal(*value.ExpiresAt) {
		t.Fatalf("unsafe downgrade %+v %v", quote, err)
	}
}

func TestNativePriceResizeRejectsCrossPeriodAndDiskShrinkPostgreSQL(t *testing.T) {
	ctx, _, builder, reader, source, value, now := nativePriceFixture(t)
	input := NativePriceResizeInput{CurrencyCode: "CNY", TermNo: "monthly", Selections: []catalog.PublicConfigurationSelection{{OptionNo: "cpu-2", Quantity: 1}}}
	reader.quote.Terms[0].PeriodCount = 12
	if _, err := builder.PrepareResize(ctx, value, input, now); err == nil || source.validated != 0 {
		t.Fatal("explicit term bypassed existing cycle")
	}
	reader.quote.Terms[0].PeriodCount = 1
	reader.resolved.Items[1].NormalizedValue = `{"gigabytes":79}`
	reader.resolved.Items[1].OptionNo = "disk-79"
	if _, err := builder.PrepareResize(ctx, value, input, now); err == nil || source.validated != 0 {
		t.Fatal("disk shrink reached provider validation")
	}
}

func TestNativePriceResizeUsesPaidNetAndTaxesOnlyDifferencePostgreSQL(t *testing.T) {
	ctx, db, builder, reader, _, value, now := nativePriceFixture(t)
	for _, statement := range []string{
		`UPDATE plugin_idc_core_commerce_quote_item_snapshot SET tax_treatment_code='taxable',tax_rate_basis_points=600`,
		`UPDATE plugin_idc_core_commerce_quote_charge_snapshot SET charge_amount_minor=1060,setup_fee_minor=212,priced_amount_minor=1272`,
		`UPDATE plugin_idc_core_catalog_tax_policy SET tax_treatment_code='taxable',tax_rate_basis_points=600`,
	} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	rate := 600
	reader.quote.Tax.TaxTreatment = catalog.TaxTreatmentTaxable
	reader.quote.Tax.TaxRateBasisPoints = &rate
	reader.quote.Terms[0].Lines = []catalog.PublicQuoteLine{{Kind: "base", Quantity: 1, ChargeAmountMinor: 2120, SetupFeeMinor: 530, PricedAmountMinor: 2650}}
	quote, err := builder.PrepareResize(ctx, value, NativePriceResizeInput{CurrencyCode: "CNY", TermNo: "monthly", Selections: []catalog.PublicConfigurationSelection{{OptionNo: "cpu-2", Quantity: 1}}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if quote.CurrentRecurringMinor != 1060 || quote.CurrentPeriodicNetMinor != 1000 || quote.TargetNetMinor != 2000 || quote.NetAmountMinor != 334 || quote.TaxAmountMinor != 20 || quote.TotalAmountMinor != 354 || quote.TargetRecurringMinor != 2120 || quote.Tax.PriceIncludesTax {
		t.Fatalf("tax/setup applied twice %+v", quote)
	}
}

func TestNativePriceIPUsesPublishedFrequencyAndExactReservationPostgreSQL(t *testing.T) {
	ctx, db, builder, _, source, value, now := nativePriceFixture(t)
	quote, err := builder.PrepareIPChange(ctx, value, "native-ip-first", now)
	if err != nil {
		t.Fatal(err)
	}
	if quote.TotalAmountMinor != 0 || quote.PolicyNo != "ip-policy" || quote.IPQuantity != 1 || len(source.reserved) != 1 || source.reserved[0].Route.ServiceNo != value.LocalResourceNo || quote.ReservationRequest.Route.PoolNo != "pool" {
		t.Fatalf("wrong frozen entitlement IP quote %+v", quote)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation(subject_type_code,subject_no,operation_type_code) VALUES('entitlement','other','ip_change'),('entitlement',?,'ip_change')`, value.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	quote, err = builder.PrepareIPChange(ctx, value, "native-ip-paid", now)
	if err != nil || quote.TotalAmountMinor != 100 || quote.NetAmountMinor != 100 || quote.TaxAmountMinor != 0 {
		t.Fatalf("wrong paid IP quote %+v %v", quote, err)
	}
	if err = builder.RevalidateIP(ctx, quote); err != nil {
		t.Fatal(err)
	}
	source.reservation = &serveroperationresourcecap.IPReservation{ReservationNo: quote.Reservation.ReservationNo, CapacityObservationNo: "different-capacity", Quantity: 1, Status: serveroperationresourcecap.IPReservationReserved, ExpiresAt: quote.ExpiresAt}
	if err = builder.RevalidateIP(ctx, quote); err == nil {
		t.Fatal("different capacity evidence accepted")
	}
	if err = builder.ReleaseIP(ctx, quote, "core_quote_failed"); err != nil || len(source.released) != 1 || source.released[0].ReservationNo != quote.Reservation.ReservationNo {
		t.Fatal("failed quote compensation lost reservation")
	}
}

func TestNativePriceIPRejectsForeignExpiredAndDailyLimitBeforeReservePostgreSQL(t *testing.T) {
	ctx, db, builder, _, source, value, now := nativePriceFixture(t)
	source.projection.ServiceNo = "ENT-other"
	if _, err := builder.PrepareIPChange(ctx, value, "native-ip-foreign", now); err == nil || len(source.reserved) != 0 {
		t.Fatal("foreign accessory projection reserved capacity")
	}
	source.projection.ServiceNo = value.LocalResourceNo
	source.projection.ExpiresAt = now
	if _, err := builder.PrepareIPChange(ctx, value, "native-ip-expired", now); err == nil || len(source.reserved) != 0 {
		t.Fatal("expired projection reserved capacity")
	}
	source.projection.ExpiresAt = now.Add(time.Hour)
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation(subject_type_code,subject_no,operation_type_code) SELECT 'entitlement',?,'ip_change' FROM generate_series(1,3)`, value.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	if _, err := builder.PrepareIPChange(ctx, value, "native-ip-maxday", now); err == nil || len(source.reserved) != 0 {
		t.Fatal("daily limit reserved capacity")
	}
}

// JSONB changes raw object key order. Semantic fingerprints retain exact large
// integer tokens and array order rather than treating those changes as tampering.
func TestNativePricedContentSurvivesJSONBObjectNormalization(t *testing.T) {
	q := &nativePricedQuote{QuoteNo: "quote", TargetConfigurationNo: "cfg", RequestDigest: "request", Target: servercommercialcap.TargetSnapshot{EntitlementNo: "ENT", Items: []serveroperationresourcecap.AccessoryItem{{Projection: json.RawMessage(`{"z":1,"aa":2}`)}}}, Price: &PreparedNativePrice{Context: NativePriceContext{EntitlementVersion: 9007199254740993}, Parameters: json.RawMessage(`{"memory":4096,"cpu":2}`)}}
	first, err := nativePricedContent(q)
	if err != nil {
		t.Fatal(err)
	}
	q.Target.Items[0].Projection = json.RawMessage(`{"aa": 2, "z": 1}`)
	q.Price.Parameters = json.RawMessage(`{"cpu": 2, "memory": 4096}`)
	same, err := nativePricedContent(q)
	if err != nil || same != first {
		t.Fatalf("JSONB normalization changed quote hash: %v", err)
	}
	q.Price.Context.EntitlementVersion--
	different, err := nativePricedContent(q)
	if err != nil || different == first {
		t.Fatal("large integer revision lost precision")
	}
}
