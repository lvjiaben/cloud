package catalogdraft

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func TestCatalogAutomationPublicationPostgreSQLIntegration(t *testing.T) {
	ctx := context.Background()
	configureCatalogDraftIntegrationDB(t, ctx)
	installAutomationPublicationMigration(t, ctx)
	insertAutomationPublicationCatalog(t, ctx)

	evidence := automationPublicationEvidence()
	extensions := &automationPublicationExtensions{evidence: evidence}
	automationAuthorizer := automationPublicationAuthorizer{}
	audit := &catalogDraftTestAuditWriter{}
	automation, err := productautomation.New(
		g.DB(), extensions, automationAuthorizer, audit,
		automationPublicationCodec{evidence: evidence},
	)
	if err != nil {
		t.Fatalf("construct automation admission: %v", err)
	}
	publicationAuthorizer := &catalogProductPublicationTestAuthorizer{actorUserID: 17}
	publication, err := catalog.NewPublication(audit, publicationAuthorizer, automation)
	if err != nil {
		t.Fatalf("construct catalog publication: %v", err)
	}
	publicationPreview, err := catalog.NewPublicationPreview(
		publicationAuthorizer, automation,
	)
	if err != nil {
		t.Fatalf("construct catalog publication preview: %v", err)
	}

	assertAutomationPreviewCheck(t, ctx, publicationPreview, extensions, automationVersionNo(1), true, 0)
	if _, err = publication.PublishVersion(ctx, automationVersionNo(1)); err != nil {
		t.Fatalf("publish unbound product: %v", err)
	}
	interfaceRow, err := automation.CreateInterface(ctx, productautomation.CreateInterfaceInput{
		DisplayName: "SSL 供应商 A", SelectionToken: "opaque-provider-selection",
	})
	if err != nil {
		t.Fatalf("create automation interface: %+v", err)
	}
	if _, err = automation.Bind(ctx, productautomation.BindInput{
		VersionNo: automationVersionNo(2), InterfaceInstanceNo: interfaceRow.InterfaceInstanceNo,
	}); err != nil {
		t.Fatalf("bind accepting interface: %v", err)
	}
	saveAutomationPublicationConfiguration(t, ctx, automation, automationVersionNo(2))
	saveAutomationPublicationPolicy(t, ctx, automationVersionNo(2), evidence)
	assertAutomationPreviewCheck(t, ctx, publicationPreview, extensions, automationVersionNo(2), true, 1)
	if _, err = publication.PublishVersion(ctx, automationVersionNo(2)); err != nil {
		t.Fatalf("publish accepting bound product: %v", err)
	}

	// Sale-state changes preserve the exact published pointer and reject stale commands.
	product, loadErr := g.DB().GetOne(ctx, `SELECT product_no,version FROM plugin_idc_core_catalog_product WHERE current_version_no=?`, automationVersionNo(2))
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	sale := catalog.ProductSaleStateInput{ProductNo: product["product_no"].String(), VersionNo: automationVersionNo(2), ExpectedVersion: product["version"].Int64(), Active: false}
	next, saleErr := publication.SetSaleState(ctx, sale)
	if saleErr != nil {
		t.Fatalf("stop sale: %v", saleErr)
	}
	if _, saleErr = publication.SetSaleState(ctx, sale); !bizerr.Is(saleErr, catalog.CodePublicationState) {
		t.Fatalf("stale sale state: %v", saleErr)
	}
	// Stopping remains available with a stopped interface; resuming requires its admission.
	if saleErr = automation.StopNewBusiness(ctx, interfaceRow.InterfaceInstanceNo); saleErr != nil {
		t.Fatal(saleErr)
	}
	sale.ExpectedVersion = next
	sale.Active = true
	if _, saleErr = publication.SetSaleState(ctx, sale); saleErr == nil {
		t.Fatal("resumed stopped provider")
	}
	if saleErr = automation.ResumeNewBusiness(ctx, interfaceRow.InterfaceInstanceNo); saleErr != nil {
		t.Fatal(saleErr)
	}
	if _, saleErr = publication.SetSaleState(ctx, sale); saleErr != nil {
		t.Fatalf("resume sale: %v", saleErr)
	}
	assertAutomationPublicationVersionStatus(t, ctx, automationVersionNo(2), "published")

	if _, err = automation.Bind(ctx, productautomation.BindInput{
		VersionNo: automationVersionNo(3), InterfaceInstanceNo: interfaceRow.InterfaceInstanceNo,
	}); err != nil {
		t.Fatalf("bind stopped test version: %v", err)
	}
	saveAutomationPublicationConfiguration(t, ctx, automation, automationVersionNo(3))
	saveAutomationPublicationPolicy(t, ctx, automationVersionNo(3), evidence)
	if err = automation.StopNewBusiness(ctx, interfaceRow.InterfaceInstanceNo); err != nil {
		t.Fatalf("stop new business: %v", err)
	}
	assertAutomationPreviewCheck(t, ctx, publicationPreview, extensions, automationVersionNo(3), false, 1)
	if _, err = publication.PublishVersion(ctx, automationVersionNo(3)); !bizerr.Is(err, catalog.CodePublicationIncomplete) {
		t.Fatalf("stopped interface publication error=%v", err)
	}
	assertAutomationPublicationVersionStatus(t, ctx, automationVersionNo(3), "draft")
	if err = automation.ResumeNewBusiness(ctx, interfaceRow.InterfaceInstanceNo); err != nil {
		t.Fatalf("resume new business: %v", err)
	}

	if _, err = automation.Bind(ctx, productautomation.BindInput{
		VersionNo: automationVersionNo(4), InterfaceInstanceNo: interfaceRow.InterfaceInstanceNo,
	}); err != nil {
		t.Fatalf("bind host rejection version: %v", err)
	}
	saveAutomationPublicationConfiguration(t, ctx, automation, automationVersionNo(4))
	saveAutomationPublicationPolicy(t, ctx, automationVersionNo(4), evidence)
	extensions.setAdmissionError(errors.New("host release admission unavailable"))
	previewAdmissionCalls := extensions.admissionCount()
	if _, err = publicationPreview.PreviewVersion(ctx, automationVersionNo(4)); !bizerr.Is(err, catalog.CodePublicationUnavailable) ||
		bizerr.Is(err, catalog.CodeManagementUnavailable) {
		t.Fatalf("host admission failure leaked through publication preview: %v", err)
	}
	if delta := extensions.admissionCount() - previewAdmissionCalls; delta != 1 {
		t.Fatalf("host-unavailable preview admission calls=%d want=1", delta)
	}
	if _, err = publication.PublishVersion(ctx, automationVersionNo(4)); !bizerr.Is(err, catalog.CodePublicationUnavailable) ||
		bizerr.Is(err, catalog.CodeManagementUnavailable) {
		t.Fatalf("host admission failure leaked through publication: %v", err)
	}
	assertAutomationPublicationVersionStatus(t, ctx, automationVersionNo(4), "draft")
	extensions.setAdmissionError(nil)

	if _, err = automation.Bind(ctx, productautomation.BindInput{
		VersionNo: automationVersionNo(6), InterfaceInstanceNo: interfaceRow.InterfaceInstanceNo,
	}); err != nil {
		t.Fatalf("bind evidence conflict version: %v", err)
	}
	saveAutomationPublicationConfiguration(t, ctx, automation, automationVersionNo(6))
	saveAutomationPublicationPolicy(t, ctx, automationVersionNo(6), evidence)
	mismatched := *evidence
	mismatched.DeclarationDigest = strings.Repeat("c", 64)
	extensions.setEvidence(&mismatched)
	assertAutomationPreviewCheck(t, ctx, publicationPreview, extensions, automationVersionNo(6), true, 1)
	extensions.setEvidence(evidence)

	if _, err = automation.Bind(ctx, productautomation.BindInput{
		VersionNo: automationVersionNo(5), InterfaceInstanceNo: interfaceRow.InterfaceInstanceNo,
	}); err != nil {
		t.Fatalf("bind lock test version: %v", err)
	}
	saveAutomationPublicationConfiguration(t, ctx, automation, automationVersionNo(5))
	saveAutomationPublicationPolicy(t, ctx, automationVersionNo(5), evidence)
	blockingAudit := newAutomationPublicationBlockingAudit(automationVersionNo(5))
	lockingPublication, err := catalog.NewPublication(blockingAudit, publicationAuthorizer, automation)
	if err != nil {
		t.Fatalf("construct locking publication: %v", err)
	}
	publishResult := make(chan error, 1)
	go func() {
		_, publishErr := lockingPublication.PublishVersion(ctx, automationVersionNo(5))
		publishResult <- publishErr
	}()
	select {
	case <-blockingAudit.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("publication did not reach in-transaction audit")
	}
	stopResult := make(chan error, 1)
	go func() { stopResult <- automation.StopNewBusiness(ctx, interfaceRow.InterfaceInstanceNo) }()
	select {
	case stopErr := <-stopResult:
		t.Fatalf("stop returned before publication released interface lock: %v", stopErr)
	case <-time.After(200 * time.Millisecond):
	}
	close(blockingAudit.release)
	if err = <-publishResult; err != nil {
		t.Fatalf("locking publication failed: %v", err)
	}
	select {
	case err = <-stopResult:
		if err != nil {
			t.Fatalf("stop after publication: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stop remained blocked after publication commit")
	}
	assertAutomationPublicationVersionStatus(t, ctx, automationVersionNo(5), "published")
}

func saveAutomationPublicationConfiguration(
	t *testing.T,
	ctx context.Context,
	automation productautomation.Service,
	versionNo string,
) {
	t.Helper()
	if err := automation.SaveVersionConfiguration(ctx, productautomation.SaveVersionConfigurationInput{
		VersionNo: versionNo, ExpectedVersion: 0, MerchantValues: []productautomation.ConfigurationValue{},
	}); err != nil {
		t.Fatalf("save automation publication configuration: %v", err)
	}
}

func saveAutomationPublicationPolicy(
	t *testing.T,
	ctx context.Context,
	versionNo string,
	evidence *capmodel.BusinessExtensionProviderEvidence,
) {
	t.Helper()
	declaration, err := productfulfillmentcap.ParseProviderDeclaration(evidence.ProviderMetadata, evidence.AuthorizedMethods)
	if err != nil {
		t.Fatalf("parse automation fulfillment declaration: %v", err)
	}
	_, err = catalog.SaveFulfillmentPolicy(ctx, catalog.FulfillmentPolicyUpdateInput{
		VersionNo: versionNo, StrategyCode: catalog.FulfillmentStrategyPaymentConfirmedPlugin,
	}, catalog.FulfillmentPolicyWriteAuthorization{
		ActorUserID: 17,
		Contract: &catalog.FrozenFulfillmentContract{
			Capability: &capmodel.LocalizedText{
				ZhCN: declaration.Capability.ZhCN, EnUS: declaration.Capability.EnUS,
			},
			Granularity: declaration.Granularity, OngoingManagement: declaration.OngoingManagement,
			RenewalMode:     productfulfillmentcap.RenewalModeUnavailable,
			ProvisionMethod: declaration.MethodRoles.Provision, QueryMethod: declaration.MethodRoles.Query,
			DeclarationDigest: evidence.DeclarationDigest,
		},
	})
	if err != nil {
		t.Fatalf("save automation fulfillment policy: %v", err)
	}
}

func installAutomationPublicationMigration(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := g.DB().Exec(ctx, automationPublicationDictionaryFixtureSQL); err != nil {
		t.Fatalf("install automation dictionary fixture: %v", err)
	}
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql")
	for _, name := range []string{
		"010-catalog-foundation.sql",
		"013-catalog-billing-policy.sql",
		"019-catalog-tax-policy.sql",
		"073-general-product-catalog.sql",
		"074-product-automation-binding.sql",
		"132-local-product-revision-binding.sql",
	} {
		migration, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read automation migration %s: %v", name, err)
		}
		if _, err = g.DB().Exec(ctx, string(migration)); err != nil {
			t.Fatalf("install automation migration %s: %v", name, err)
		}
	}
	// Publication reads catalog price dimensions even when this fixture has no
	// configured add-ons. Reuse the authoritative catalog portions of migration
	// 094 without installing its unrelated commerce tables and cart updates.
	pricingMigration, err := os.ReadFile(filepath.Join(root, "094-generic-configuration-pricing.sql"))
	if err != nil {
		t.Fatalf("read automation pricing migration: %v", err)
	}
	pricingSQL := string(pricingMigration)
	commerceStart := strings.Index(pricingSQL, "ALTER TABLE plugin_idc_core_commerce_configuration_snapshot")
	indexStart := strings.Index(pricingSQL, "DROP INDEX IF EXISTS uk_idc_catalog_price_base_tier;")
	selectionStart := strings.Index(pricingSQL, "-- Purpose: Stores Commerce Configuration Price Selection records.")
	if commerceStart < 0 || indexStart <= commerceStart || selectionStart <= indexStart {
		t.Fatal("automation pricing migration catalog boundaries changed")
	}
	if _, err = g.DB().Exec(ctx, pricingSQL[:commerceStart]+pricingSQL[indexStart:selectionStart]); err != nil {
		t.Fatalf("install automation catalog pricing migration: %v", err)
	}
	quantityMigration, err := os.ReadFile(filepath.Join(root, "127-product-commerce-workflow.sql"))
	if err != nil {
		t.Fatal(err)
	}
	quantitySQL := string(quantityMigration)
	quantityEnd := strings.Index(quantitySQL, "ALTER TABLE plugin_idc_core_commerce_configuration_price_selection")
	if quantityEnd < 0 {
		t.Fatal("quantity migration catalog boundary changed")
	}
	if _, err = g.DB().Exec(ctx, quantitySQL[:quantityEnd]); err != nil {
		t.Fatal(err)
	}
	// Managed interface reads also use the origin columns and identity guard
	// from migration 096; its fulfillment outbox is outside this test's scope.
	resourceMigration, err := os.ReadFile(filepath.Join(root, "096-resource-owned-product-fulfillment.sql"))
	if err != nil {
		t.Fatalf("read automation resource-origin migration: %v", err)
	}
	resourceSQL := string(resourceMigration)
	outboxStart := strings.Index(resourceSQL, "-- Purpose: Store durable resource-owner outcome reports without supplier secrets.")
	if outboxStart < 0 {
		t.Fatal("automation resource-origin migration boundary changed")
	}
	if _, err = g.DB().Exec(ctx, resourceSQL[:outboxStart]); err != nil {
		t.Fatalf("install automation resource-origin migration: %v", err)
	}
	if _, err := g.DB().Exec(ctx, automationPublicationFulfillmentPolicyFixtureSQL); err != nil {
		t.Fatalf("install automation fulfillment policy fixture: %v", err)
	}
}

const automationPublicationDictionaryFixtureSQL = `
CREATE TABLE IF NOT EXISTS sys_dict_type (
 id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
 name VARCHAR(128) NOT NULL DEFAULT '', type VARCHAR(128) NOT NULL DEFAULT '',
 status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
 allow_tenant_override BOOL NOT NULL DEFAULT FALSE, remark VARCHAR(512) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
 UNIQUE (tenant_id,type)
);
CREATE TABLE IF NOT EXISTS sys_dict_data (
 id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
 dict_type VARCHAR(128) NOT NULL DEFAULT '', label VARCHAR(128) NOT NULL DEFAULT '',
 value VARCHAR(128) NOT NULL DEFAULT '', sort INT NOT NULL DEFAULT 0,
 tag_style VARCHAR(128) NOT NULL DEFAULT '', status SMALLINT NOT NULL DEFAULT 1,
 is_builtin SMALLINT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
 UNIQUE (tenant_id,dict_type,value)
);`

const automationPublicationFulfillmentPolicyFixtureSQL = `
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product_import(
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT,product_no TEXT,version_no TEXT,interface_instance_no TEXT,upstream_reference TEXT,source_snapshot JSONB);

CREATE TABLE IF NOT EXISTS plugin_idc_core_product_automation_interface_configuration_revision (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0,
 interface_instance_no VARCHAR(40) NOT NULL,
 revision BIGINT NOT NULL,
 schema_version VARCHAR(32) NOT NULL,
 declaration_snapshot JSONB NOT NULL,
 public_value_snapshot JSONB NOT NULL,
 declaration_digest CHAR(64) NOT NULL,
 value_digest CHAR(64) NOT NULL,
 secret_field_count INT NOT NULL DEFAULT 0,
 created_by INT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE (tenant_id,interface_instance_no,revision),
 FOREIGN KEY (interface_instance_no) REFERENCES plugin_idc_core_product_automation_interface_instance(interface_instance_no) ON DELETE RESTRICT
);
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_automation_interface_secret_reference (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0,
 configuration_revision_id BIGINT NOT NULL,
 field_key VARCHAR(64) NOT NULL,
 secret_ref VARCHAR(512) NOT NULL,
 configured BOOLEAN NOT NULL DEFAULT TRUE,
 masked_tail VARCHAR(32) NOT NULL DEFAULT '',
 secret_updated_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE (configuration_revision_id,field_key),
 FOREIGN KEY (configuration_revision_id) REFERENCES plugin_idc_core_product_automation_interface_configuration_revision(id) ON DELETE RESTRICT
);
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product_version_configuration (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0,
 version_no VARCHAR(40) NOT NULL,
 interface_instance_no VARCHAR(40) NOT NULL,
 interface_configuration_revision BIGINT NOT NULL,
 product_schema_version VARCHAR(32) NOT NULL,
 product_declaration_snapshot JSONB NOT NULL,
 merchant_value_snapshot JSONB NOT NULL,
 customer_schema_version VARCHAR(32) NOT NULL,
 customer_declaration_snapshot JSONB NOT NULL,
 declaration_digest CHAR(64) NOT NULL,
 merchant_value_digest CHAR(64) NOT NULL,
 version BIGINT NOT NULL DEFAULT 1,
 changed_by INT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 changed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE (tenant_id,version_no),
 FOREIGN KEY (version_no) REFERENCES plugin_idc_core_catalog_product_version(version_no) ON DELETE RESTRICT,
 FOREIGN KEY (version_no) REFERENCES plugin_idc_core_catalog_product_version_fulfillment_binding(version_no) ON DELETE RESTRICT,
 FOREIGN KEY (tenant_id,interface_instance_no,interface_configuration_revision)
  REFERENCES plugin_idc_core_product_automation_interface_configuration_revision(tenant_id,interface_instance_no,revision) ON DELETE RESTRICT
);
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product_version_fulfillment_policy (
 tenant_id INT NOT NULL DEFAULT 0,
 version_no VARCHAR(40) PRIMARY KEY,
 strategy_code VARCHAR(32) NOT NULL,
 granularity_code VARCHAR(32) NOT NULL,
 ongoing_management BOOLEAN NOT NULL DEFAULT FALSE,
 renewal_mode_code VARCHAR(32) NOT NULL DEFAULT 'unavailable',
 renewal_method VARCHAR(128) NOT NULL DEFAULT '',
 entitlement_owner_id VARCHAR(64) NOT NULL DEFAULT '',
 provision_method VARCHAR(128) NOT NULL DEFAULT '',
 query_method VARCHAR(128) NOT NULL DEFAULT '',
 maximum_attempts INT NOT NULL,
 retry_base_seconds INT NOT NULL,
 policy_digest CHAR(64) NOT NULL,
 created_by BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

func insertAutomationPublicationCatalog(t *testing.T, ctx context.Context) {
	t.Helper()
	manualPolicy, err := catalog.BuildFulfillmentPolicyEvidence(catalog.FulfillmentStrategyManual, nil)
	if err != nil {
		t.Fatalf("build manual fulfillment policy: %v", err)
	}
	var sql strings.Builder
	sql.WriteString(`INSERT INTO plugin_idc_core_catalog_category (category_no,name_zh_cn,name_en_us) VALUES ('category-automation','通用商品','Generic products');`)
	for index := 1; index <= 6; index++ {
		productNo := fmt.Sprintf("product-automation-%d", index)
		versionNo := automationVersionNo(index)
		fmt.Fprintf(&sql, `
INSERT INTO plugin_idc_core_catalog_product
 (product_no,category_no,status_code,public_slug,name_zh_cn,name_en_us,summary_zh_cn,summary_en_us)
 VALUES ('%s','category-automation','draft','automation-%d','通用商品%d','Generic product %d','通用商品摘要','Generic product summary');
INSERT INTO plugin_idc_core_catalog_product_version
 (version_no,product_no,version_number,status_code,source_code,source_version)
 VALUES ('%s','%s',1,'draft','manual','');
INSERT INTO plugin_idc_core_catalog_billing_rule
 (billing_rule_no,version_no,rule_code,charge_type_code,period_unit_code,period_count,price_phase_code)
 VALUES ('billing-automation-%d','%s','monthly','recurring','month',1,'both');
INSERT INTO plugin_idc_core_catalog_price
 (price_no,version_no,billing_rule_no,currency_code,amount_minor,renewal_amount_minor)
 VALUES ('price-automation-%d','%s','billing-automation-%d','CNY',10000,10000);
INSERT INTO plugin_idc_core_catalog_tax_policy
 (tax_policy_no,version_no,currency_code,price_includes_tax,tax_treatment_code,tax_rate_basis_points,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor)
 VALUES ('tax-automation-%d','%s','CNY',TRUE,'taxable',600,'general_service','line','half_up',1);
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy
 (version_no,strategy_code,granularity_code,ongoing_management,renewal_mode_code,maximum_attempts,retry_base_seconds,policy_digest,created_by)
 VALUES ('%s','manual','per_order_item',FALSE,'unavailable',10,30,'%s',17);`,
			productNo, index, index, index, versionNo, productNo,
			index, versionNo, index, versionNo, index, index, versionNo,
			versionNo, manualPolicy.PolicyDigest)
	}
	if _, err := g.DB().Exec(ctx, sql.String()); err != nil {
		t.Fatalf("insert automation publication catalog: %v", err)
	}
}

func automationVersionNo(index int) string {
	return fmt.Sprintf("ver_%032x", index)
}

func assertAutomationPublicationVersionStatus(t *testing.T, ctx context.Context, versionNo, want string) {
	t.Helper()
	value, err := g.DB().GetValue(ctx, `SELECT status_code FROM plugin_idc_core_catalog_product_version WHERE version_no=?`, versionNo)
	if err != nil || value.String() != want {
		t.Fatalf("version %s status=%q want=%q err=%v", versionNo, value.String(), want, err)
	}
}

func automationPublicationEvidence() *capmodel.BusinessExtensionProviderEvidence {
	metadata, err := productfulfillmentcap.NewProviderMetadata(&productfulfillmentcap.ProviderDeclaration{
		Capability:         productfulfillmentcap.ProviderCapability{ZhCN: "证书签发", EnUS: "Certificate issuance"},
		Granularity:        productfulfillmentcap.GranularityPerOrderItem,
		OngoingManagement:  false,
		RenewalSupport:     productfulfillmentcap.RenewalSupportUnsupported,
		ExpiryManagement:   productfulfillmentcap.ExpiryManagementUnsupported,
		EntitlementOwnerID: "",
		MethodRoles: productfulfillmentcap.ProviderMethodRoles{
			Provision: "provision", Query: "query.status",
		},
	})
	if err != nil {
		panic(err)
	}
	return &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "ssl-provider", DisplayName: "SSL Provider", ReleaseVersion: "1.0.0",
		Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1",
		AuthorizedMethods: []string{"provision", "query.status"}, ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
		ProviderMetadata: metadata,
	}
}

type automationPublicationExtensions struct {
	mu             sync.RWMutex
	evidence       *capmodel.BusinessExtensionProviderEvidence
	admissionErr   error
	admissionCalls int
}

func (e *automationPublicationExtensions) setAdmissionError(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.admissionErr = err
}

func (e *automationPublicationExtensions) setEvidence(evidence *capmodel.BusinessExtensionProviderEvidence) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.evidence = evidence
}

func (e *automationPublicationExtensions) admissionCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.admissionCalls
}

func (e *automationPublicationExtensions) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	return &capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence]{Items: []*capmodel.BusinessExtensionProviderEvidence{e.evidence}, Total: 1}, nil
}

func (e *automationPublicationExtensions) GetProviderExact(context.Context, plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	return e.evidence, nil
}

func (e *automationPublicationExtensions) GetProvidersExact(_ context.Context, inputs []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	items := make([]*capmodel.BusinessExtensionProviderEvidence, len(inputs))
	for i := range inputs {
		items[i] = e.evidence
	}
	return items, nil
}

func (e *automationPublicationExtensions) WithNewBusinessAdmission(ctx context.Context, _ pluginhost.BusinessExtensionAdmissionInput, action capmodel.BusinessExtensionAdmissionAction) error {
	e.mu.Lock()
	e.admissionCalls++
	err := e.admissionErr
	evidence := e.evidence
	e.mu.Unlock()
	if err != nil {
		return err
	}
	return action(ctx, evidence)
}

func (*automationPublicationExtensions) WithConfidentialOutboundContext(
	context.Context,
	pluginhost.ConfidentialOutboundContextInput,
	pluginhost.ConfidentialOutboundAction,
) error {
	return errors.New("confidential outbound context is outside publication admission tests")
}

func (*automationPublicationExtensions) InvokeProviderMethod(
	context.Context,
	pluginhost.BusinessExtensionMethodInvokeInput,
) (*capmodel.BusinessExtensionMethodResult, error) {
	return nil, errors.New("provider method invocation is outside publication admission tests")
}

func assertAutomationPreviewCheck(
	t *testing.T,
	ctx context.Context,
	preview catalog.PublicationPreviewService,
	extensions *automationPublicationExtensions,
	versionNo string,
	want bool,
	wantAdmissionCalls int,
) {
	t.Helper()
	beforeAdmissionCalls := extensions.admissionCount()
	result, err := preview.PreviewVersion(ctx, versionNo)
	if err != nil {
		t.Fatalf("preview %s: %v", versionNo, err)
	}
	found := false
	for _, check := range result.Checks {
		if check.Code == catalog.PublicationCheckAutomation {
			found = true
			if check.Passed != want {
				t.Fatalf("preview %s automation=%v want=%v checks=%+v", versionNo, check.Passed, want, result.Checks)
			}
		}
	}
	if !found {
		t.Fatalf("preview %s omitted automation check: %+v", versionNo, result.Checks)
	}
	if result.Ready != want {
		t.Fatalf("preview %s ready=%v want=%v", versionNo, result.Ready, want)
	}
	if delta := extensions.admissionCount() - beforeAdmissionCalls; delta != wantAdmissionCalls {
		t.Fatalf("preview %s host admission calls=%d want=%d", versionNo, delta, wantAdmissionCalls)
	}
}

type automationPublicationCodec struct {
	evidence *capmodel.BusinessExtensionProviderEvidence
}

func (c automationPublicationCodec) EncodeProviderSelection(context.Context, *capmodel.BusinessExtensionProviderEvidence) (string, error) {
	return "opaque-provider-selection", nil
}

func (c automationPublicationCodec) DecodeProviderSelection(context.Context, string) (*capmodel.BusinessExtensionProviderEvidence, error) {
	return c.evidence, nil
}

type automationPublicationAuthorizer struct{}

func (automationPublicationAuthorizer) authorization() (productautomation.Authorization, error) {
	return productautomation.Authorization{ActorUserID: 17, RequiredPermission: catalog.PermissionCatalogProductPublish}, nil
}
func (a automationPublicationAuthorizer) AuthorizeProviderCandidates(context.Context) (productautomation.Authorization, error) {
	return a.authorization()
}
func (a automationPublicationAuthorizer) AuthorizeInterfaceList(context.Context) (productautomation.Authorization, error) {
	return a.authorization()
}
func (a automationPublicationAuthorizer) AuthorizeInterfaceCreate(context.Context) (productautomation.Authorization, error) {
	return a.authorization()
}
func (a automationPublicationAuthorizer) AuthorizeInterfaceRename(context.Context, string) (productautomation.Authorization, error) {
	return a.authorization()
}
func (a automationPublicationAuthorizer) AuthorizeInterfaceDelete(context.Context, string) (productautomation.Authorization, error) {
	return a.authorization()
}
func (a automationPublicationAuthorizer) AuthorizeInterfaceStop(context.Context, string) (productautomation.Authorization, error) {
	return a.authorization()
}
func (a automationPublicationAuthorizer) AuthorizeInterfaceResume(context.Context, string) (productautomation.Authorization, error) {
	return a.authorization()
}
func (a automationPublicationAuthorizer) AuthorizeInterfaceConfiguration(context.Context, string) (productautomation.Authorization, error) {
	return a.authorization()
}
func (a automationPublicationAuthorizer) AuthorizeBindingRead(context.Context, string) (productautomation.Authorization, error) {
	return a.authorization()
}
func (a automationPublicationAuthorizer) AuthorizeBindingWrite(context.Context, string) (productautomation.Authorization, error) {
	return a.authorization()
}
func (a automationPublicationAuthorizer) AuthorizeVersionConfiguration(context.Context, string) (productautomation.Authorization, error) {
	return a.authorization()
}
func (a automationPublicationAuthorizer) AuthorizePublicationAdmission(context.Context, string) (productautomation.Authorization, error) {
	return a.authorization()
}

type automationPublicationBlockingAudit struct {
	target  string
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newAutomationPublicationBlockingAudit(target string) *automationPublicationBlockingAudit {
	return &automationPublicationBlockingAudit{target: target, entered: make(chan struct{}), release: make(chan struct{})}
}

func (a *automationPublicationBlockingAudit) AppendBusinessTransition(_ context.Context, input idccap.BusinessAuditTransition) error {
	if input.OperationCode == "catalog.product-version.publish" && input.TargetNo == a.target {
		a.once.Do(func() { close(a.entered) })
		<-a.release
	}
	return nil
}

func (*automationPublicationBlockingAudit) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}
