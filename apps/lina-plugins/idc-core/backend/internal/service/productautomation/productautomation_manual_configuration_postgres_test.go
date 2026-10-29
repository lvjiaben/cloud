// These database cases exercise manual persistence, null relationships,
// concurrency, rollback and copying without supplier-management permission.
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
)

type manualOfferTestAuthorizer struct{ automationTestAuthorizer }

func (a *manualOfferTestAuthorizer) AuthorizeBindingWrite(context.Context, string) (Authorization, error) {
	a.calls = append(a.calls, "unexpected-binding-write")
	return Authorization{}, bizerr.NewCode(catalog.CodeManagementNotAccessible)
}

func TestManualOfferSaveCopyRollbackAndNoSupplierPostgreSQL(t *testing.T) {
	ctx := context.Background()
	database := portableBindingDatabase(t, ctx)
	// This focused fixture does not load the unrelated commerce tables. The
	// full-schema migration and checkout are separately exercised by TC072.
	if _, err := database.Exec(ctx, `CREATE TABLE plugin_idc_core_commerce_structured_configuration_snapshot(interface_instance_no VARCHAR(40) NOT NULL,interface_configuration_revision BIGINT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `CREATE TABLE plugin_idc_core_product_fulfillment_intent(
strategy_code TEXT,provision_method TEXT,query_method TEXT,interface_instance_no TEXT,
plugin_id TEXT,interface_configuration_revision BIGINT,configuration_snapshot_no TEXT,
plugin_release_version TEXT,contract_version TEXT,release_digest TEXT,declaration_digest TEXT)`); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/142-generic-offer-configuration.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 2; round++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	const manualIntent = `INSERT INTO plugin_idc_core_product_fulfillment_intent VALUES('manual','','','','',0,?,'','','','')`
	if _, err = database.Exec(ctx, manualIntent, "SCF_"+strings.Repeat("a", 32)); err != nil {
		t.Fatal("manual delivery rejected its commercial selection snapshot", err)
	}
	if _, err = database.Exec(ctx, manualIntent, "unverified_snapshot"); err == nil {
		t.Fatal("manual delivery accepted a malformed commercial snapshot identity")
	}
	const source = "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const target = "ver_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no,status_code) VALUES(0,?,'draft')`, source); err != nil {
		t.Fatal(err)
	}
	authorizer := &manualOfferTestAuthorizer{}
	extensions := &automationTestExtensions{}
	audit := &automationTestAudit{}
	codec := &automationTestCodec{}
	service, err := newWithRandom(database, extensions, authorizer, audit, codec, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	selection := CustomerFieldSelection{FieldKey: "local_note", LocalField: &capmodel.ConfigurationFieldDeclaration{Key: "local_note", Type: capmodel.ConfigurationFieldSingleLine, Label: &capmodel.LocalizedText{ZhCN: "服务用途", EnUS: "Service purpose"}, MaximumLength: 256}}
	input := SaveVersionConfigurationInput{VersionNo: source, ExpectedVersion: 0, CustomerFieldSelections: []CustomerFieldSelection{selection}}
	if err = service.SaveVersionConfiguration(ctx, input); err != nil {
		t.Fatal(err)
	}
	configuration, err := service.GetVersionConfiguration(ctx, source)
	if err != nil || configuration.Version != 1 || len(configuration.CustomerDeclaration.Fields) != 1 {
		t.Fatalf("manual definition not restored: configuration=%+v err=%v", configuration, err)
	}
	nullRelationship, err := database.GetValue(ctx, `SELECT interface_instance_no IS NULL AND interface_configuration_revision IS NULL FROM plugin_idc_core_catalog_product_version_configuration WHERE version_no=?`, source)
	if err != nil || !nullRelationship.Bool() {
		t.Fatal("manual offer fabricated an interface relationship")
	}
	if err = service.SaveVersionConfiguration(ctx, input); !bizerr.Is(err, catalog.CodeManagementConflict) {
		t.Fatalf("stale manual revision accepted: %v", err)
	}
	input.ExpectedVersion = 1
	audit.err = errors.New("isolated audit rejection")
	if err = service.SaveVersionConfiguration(ctx, input); err == nil {
		t.Fatal("manual configuration committed without audit")
	}
	audit.err = nil
	configuration, err = service.GetVersionConfiguration(ctx, source)
	if err != nil || configuration.Version != 1 {
		t.Fatal("audit failure leaked a revision change")
	}
	copyService, err := ProductCopyServiceFromService(service)
	if err != nil {
		t.Fatal(err)
	}
	if err = copyService.WithProductCopy(ctx, source, true, func(copyCtx context.Context) (string, error) {
		_, insertErr := database.Exec(copyCtx, `INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no,status_code) VALUES(0,?,'draft')`, target)
		return target, insertErr
	}); err != nil {
		t.Fatal(err)
	}
	copied, err := service.GetVersionConfiguration(ctx, target)
	if err != nil || copied.Version != 1 || len(copied.CustomerFieldSelections) != 1 || copied.CustomerFieldSelections[0].LocalField.Label.ZhCN != "服务用途" {
		t.Fatalf("manual copy lost its commercial fields: %+v %v", copied, err)
	}
	if strings.Contains(strings.Join(authorizer.calls, ","), "unexpected-binding-write") || extensions.exactCalls != 0 || extensions.listCalls != 0 || codec.encodes != 0 || codec.decodes != 0 {
		t.Fatal("manual definition or copying required supplier authority or discovery")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version SET status_code='published' WHERE version_no=?`, source); err != nil {
		t.Fatal(err)
	}
	if err = service.SaveVersionConfiguration(ctx, input); err == nil {
		t.Fatal("manual path overwrote an immutable published definition")
	}
}
