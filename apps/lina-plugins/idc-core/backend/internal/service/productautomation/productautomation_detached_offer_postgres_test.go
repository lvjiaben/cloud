// Real PostgreSQL checks exercise binding foreign keys, whole-transaction
// rollback and exact money preservation while a draft changes suppliers.
package productautomation

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

type detachedOfferTestAuthorizer struct {
	automationTestAuthorizer
	denyConfiguration bool
}

func (a *detachedOfferTestAuthorizer) AuthorizeVersionConfiguration(ctx context.Context, versionNo string) (Authorization, error) {
	if a.denyConfiguration {
		return Authorization{}, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return a.automationTestAuthorizer.AuthorizeVersionConfiguration(ctx, versionNo)
}

func TestDetachedOfferBindingRollbackMoneyAndExplicitMappingPostgreSQL(t *testing.T) {
	ctx := context.Background()
	db := portableBindingDatabase(t, ctx)
	if _, err := db.Exec(ctx, `CREATE TABLE plugin_idc_core_commerce_structured_configuration_snapshot(interface_instance_no VARCHAR(40) NOT NULL,interface_configuration_revision BIGINT NOT NULL);
CREATE TABLE plugin_idc_core_product_fulfillment_intent(strategy_code TEXT,provision_method TEXT,query_method TEXT,interface_instance_no TEXT,plugin_id TEXT,interface_configuration_revision BIGINT,configuration_snapshot_no TEXT,plugin_release_version TEXT,contract_version TEXT,release_digest TEXT,declaration_digest TEXT);
CREATE TABLE plugin_idc_core_catalog_configuration_price_dimension(tenant_id INT,version_no TEXT,dimension_no TEXT,field_key TEXT,candidate_value TEXT,pricing_mode_code TEXT,status_code TEXT);
CREATE TABLE detached_test_price(dimension_no TEXT,amount_minor TEXT);`); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/142-generic-offer-configuration.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	exact := automationEvidence([]string{"provision", "query.status"})
	custom := customerSelectionEvidence(t)
	exact.CustomerConfiguration, exact.ProviderMetadata = custom.CustomerConfiguration, custom.ProviderMetadata
	audit := &automationTestAudit{}
	authorizer := &detachedOfferTestAuthorizer{}
	service, err := newWithRandom(db, &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{exact}}, authorizer, audit, &automationTestCodec{evidence: exact}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	oldInterface, err := service.CreateInterface(ctx, CreateInterfaceInput{DisplayName: "Old certificate connector", SelectionToken: "opaque"})
	if err != nil {
		t.Fatal(err)
	}
	newInterface, err := service.CreateInterface(ctx, CreateInterfaceInput{DisplayName: "New certificate connector", SelectionToken: "opaque"})
	if err != nil {
		t.Fatal(err)
	}
	const version = "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no,status_code) VALUES(0,?,'draft')`, version); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Bind(ctx, BindInput{VersionNo: version, InterfaceInstanceNo: oldInterface.InterfaceInstanceNo}); err != nil {
		t.Fatal(err)
	}
	if err = service.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{VersionNo: version, CustomerFieldSelections: []CustomerFieldSelection{{FieldKey: "operating_system", InformationOnly: true}}}); !bizerr.Is(err, catalog.CodeManagementInvalid) {
		t.Fatal("a provider-owned field was falsely marked as information only", err)
	}
	input := SaveVersionConfigurationInput{VersionNo: version, CustomerFieldSelections: []CustomerFieldSelection{
		{FieldKey: "cpu", Minimum: "2", Maximum: "8", DisplayControl: configurationcontract.ControlRadio, NumericChoices: []string{"2", "4", "8"}},
		{FieldKey: "snapshot_count", Minimum: "0", Maximum: "5"},
		{FieldKey: "snapshot_unlimited", CandidateValues: []string{"unlimited"}},
	}}
	if err = service.SaveVersionConfiguration(ctx, input); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension VALUES(0,?,'exact-money','cpu','quantity','per_unit','active')`, version); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO detached_test_price VALUES('exact-money','1999999')`); err != nil {
		t.Fatal(err)
	}
	authorizer.denyConfiguration = true
	if err = service.Unbind(ctx, version); !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatal("binding permission alone changed a protected commercial definition", err)
	}
	authorizer.denyConfiguration = false
	audit.err = errors.New("isolated audit rejection")
	if err = service.Unbind(ctx, version); err == nil {
		t.Fatal("detachment committed without audit")
	}
	audit.err = nil
	if field, readErr := db.GetValue(ctx, `SELECT field_key FROM plugin_idc_core_catalog_configuration_price_dimension WHERE dimension_no='exact-money'`); readErr != nil || field.String() != "cpu" {
		t.Fatal("rollback changed price identity", readErr)
	}
	if binding, readErr := service.GetBinding(ctx, version); readErr != nil || binding == nil || binding.InterfaceInstanceNo != oldInterface.InterfaceInstanceNo {
		t.Fatal("rollback deleted binding", readErr)
	}
	if err = service.Unbind(ctx, version); err != nil {
		t.Fatal(err)
	}
	manual, err := service.GetVersionConfiguration(ctx, version)
	if err != nil || manual.Version != 2 || len(manual.CustomerDeclaration.Fields) != 4 || len(manual.CustomerDeclaration.Conditions) != 1 {
		t.Fatal("detached commercial definition lost fields or conditions", err)
	}
	for _, field := range manual.CustomerDeclaration.Fields {
		if !strings.HasPrefix(field.Key, localCustomerFieldPrefix) {
			t.Fatal("old supplier identity is still executable")
		}
	}
	if field, readErr := db.GetValue(ctx, `SELECT field_key FROM plugin_idc_core_catalog_configuration_price_dimension WHERE dimension_no='exact-money'`); readErr != nil || !strings.HasPrefix(field.String(), "local_retained_") {
		t.Fatal("price did not follow the retained field", readErr)
	}
	if amount, readErr := db.GetValue(ctx, `SELECT amount_minor FROM detached_test_price WHERE dimension_no='exact-money'`); readErr != nil || amount.String() != "1999999" {
		t.Fatal("conversion changed exact money", readErr)
	}
	if err = service.Unbind(ctx, version); err != nil {
		t.Fatal("idempotent detach failed", err)
	}
	input = SaveVersionConfigurationInput{VersionNo: version, ExpectedVersion: manual.Version, CustomerFieldSelections: manual.CustomerFieldSelections}
	if err = service.SaveVersionConfiguration(ctx, input); err != nil {
		t.Fatal("manual edit could not retain conditions", err)
	}
	manual, err = service.GetVersionConfiguration(ctx, version)
	if err != nil || len(manual.CustomerDeclaration.Conditions) != 1 {
		t.Fatal("ordinary manual save lost conditions", err)
	}
	if _, err = service.Bind(ctx, BindInput{VersionNo: version, InterfaceInstanceNo: newInterface.InterfaceInstanceNo}); err != nil {
		t.Fatal(err)
	}
	input.ExpectedVersion = manual.Version
	input.CustomerFieldSelections = manual.CustomerFieldSelections
	if err = service.SaveVersionConfiguration(ctx, input); !bizerr.Is(err, codeOfferMappingRequired) {
		t.Fatal("a blank old mapping was silently accepted", err)
	}
	for index := range input.CustomerFieldSelections {
		input.CustomerFieldSelections[index].InformationOnly = true
	}
	if err = service.SaveVersionConfiguration(ctx, input); err != nil {
		t.Fatal("explicit information-only choice was rejected", err)
	}
	updated, err := service.GetVersionConfiguration(ctx, version)
	if err != nil {
		t.Fatal(err)
	}
	for _, selection := range updated.CustomerFieldSelections {
		if selection.AutomationMappingPending {
			t.Fatal("explicit mapping resolution was not stored")
		}
	}
	// Direct replacement must release the configuration FK before deleting
	// the old binding, and its field aliases must stay inside this draft.
	const directVersion = "ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no,status_code) VALUES(0,?,'draft')`, directVersion); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Bind(ctx, BindInput{VersionNo: directVersion, InterfaceInstanceNo: oldInterface.InterfaceInstanceNo}); err != nil {
		t.Fatal(err)
	}
	if err = service.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{VersionNo: directVersion, CustomerFieldSelections: []CustomerFieldSelection{{FieldKey: "cpu", Minimum: "2", Maximum: "4"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Bind(ctx, BindInput{VersionNo: directVersion, InterfaceInstanceNo: newInterface.InterfaceInstanceNo}); err != nil {
		t.Fatal("direct binding replacement violated the configuration relationship", err)
	}
	replaced, err := service.GetVersionConfiguration(ctx, directVersion)
	if err != nil || replaced.Version != 2 {
		t.Fatal("replaced draft did not retain its commercial definition", err)
	}
	pending := 0
	for _, selection := range replaced.CustomerFieldSelections {
		if selection.AutomationMappingPending {
			pending++
		}
	}
	if pending != 2 {
		t.Fatal("direct replacement reused old supplier mappings")
	}
	if amount, readErr := db.GetValue(ctx, `SELECT amount_minor FROM detached_test_price WHERE dimension_no='exact-money'`); readErr != nil || amount.String() != "1999999" {
		t.Fatal("another draft's money changed", readErr)
	}
}
