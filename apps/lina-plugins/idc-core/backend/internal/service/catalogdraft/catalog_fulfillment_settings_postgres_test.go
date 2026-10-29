// This file verifies the unified provisioning form's transaction against real
// PostgreSQL. Interface admission is isolated; saved policy and publication are real.
package catalogdraft

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func TestFulfillmentSettingsAtomicPostgreSQL(t *testing.T) {
	ctx := context.Background()
	configureCatalogDraftIntegrationDB(t, ctx)
	installAutomationPublicationMigration(t, ctx)
	insertUnifiedPublicationCatalog(t, ctx)
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/081-simple-product-version-inventory.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.DB().Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	const versionNo = "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err = g.DB().Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,version_number,status_code,source_code) VALUES (?,'product-unified',3,'draft','manual')`, versionNo); err != nil {
		t.Fatal(err)
	}
	if _, err = g.DB().Exec(ctx, `CREATE TABLE fulfillment_settings_test_binding(version_no TEXT PRIMARY KEY,interface_no TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	authority := &catalogProductPublicationTestAuthorizer{actorUserID: 17}
	publication, err := catalog.NewPublication(&catalogDraftTestAuditWriter{}, authority, catalogDraftTestAutomationAdmission{})
	if err != nil {
		t.Fatal(err)
	}
	failingPublication, err := catalog.NewPublication(&catalogDraftTestAuditWriter{err: errors.New("injected final audit failure")}, authority, catalogDraftTestAutomationAdmission{})
	if err != nil {
		t.Fatal(err)
	}
	automation := &fulfillmentSettingsAutomation{}
	selected := "pfi_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	input := productautomation.FulfillmentSettingsInput{VersionNo: versionNo, StrategyCode: catalog.FulfillmentStrategyManual, InterfaceInstanceNo: &selected}
	assertNoPartial := func() {
		t.Helper()
		for _, table := range []string{"fulfillment_settings_test_binding", "plugin_idc_core_catalog_product_version_fulfillment_policy"} {
			count, e := g.DB().GetValue(ctx, "SELECT COUNT(*) FROM "+table+" WHERE version_no=?", versionNo)
			if e != nil || count.Int() != 0 {
				t.Fatalf("partial edit survived in %s: %v %v", table, count, e)
			}
		}
		status, e := g.DB().GetValue(ctx, `SELECT status_code FROM plugin_idc_core_catalog_product_version WHERE version_no=?`, versionNo)
		if e != nil || status.String() != "draft" {
			t.Fatalf("failed edit applied: %v %v", status, e)
		}
	}
	// A failed policy validation must undo the interface selection.
	input.StrategyCode = catalog.FulfillmentStrategyPaymentConfirmedPlugin
	if _, err = productautomation.SaveFulfillmentSettings(ctx, automation, publication, input); !bizerr.Is(err, catalog.CodeFulfillmentPolicyBindingRequired) {
		t.Fatalf("automatic policy without a valid contract: %v", err)
	}
	assertNoPartial()
	// Final application/audit failure also undoes both policy and binding.
	input.StrategyCode = catalog.FulfillmentStrategyManual
	if _, err = productautomation.SaveFulfillmentSettings(ctx, automation, failingPublication, input); err == nil {
		t.Fatal("audit failure reported success")
	}
	assertNoPartial()
	automation.reject = true
	if _, err = productautomation.SaveFulfillmentSettings(ctx, automation, publication, input); !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatalf("interface admission failure: %v", err)
	}
	assertNoPartial()
	automation.reject = false
	result, err := productautomation.SaveFulfillmentSettings(ctx, automation, publication, input)
	if err != nil || result.Policy.StrategyCode != catalog.FulfillmentStrategyManual {
		t.Fatalf("atomic save: %v %+v", err, result)
	}
	row, err := g.DB().GetOne(ctx, `SELECT v.status_code,p.current_version_no,b.interface_no,f.strategy_code FROM plugin_idc_core_catalog_product_version v JOIN plugin_idc_core_catalog_product p USING(product_no) JOIN fulfillment_settings_test_binding b USING(version_no) JOIN plugin_idc_core_catalog_product_version_fulfillment_policy f USING(version_no) WHERE v.version_no=?`, versionNo)
	if err != nil || row["status_code"].String() != "published" || row["current_version_no"].String() != versionNo || row["interface_no"].String() != selected || row["strategy_code"].String() != catalog.FulfillmentStrategyManual {
		t.Fatalf("binding and mode did not apply together: %v %v", row, err)
	}
}

type fulfillmentSettingsAutomation struct {
	productautomation.Service
	reject bool
}

func (s *fulfillmentSettingsAutomation) Bind(ctx context.Context, input productautomation.BindInput) (*productautomation.Binding, error) {
	if s.reject {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	_, err := g.DB().Exec(ctx, `INSERT INTO fulfillment_settings_test_binding(version_no,interface_no) VALUES(?,?)`, input.VersionNo, input.InterfaceInstanceNo)
	return &productautomation.Binding{InterfaceInstanceNo: input.InterfaceInstanceNo}, err
}
func (*fulfillmentSettingsAutomation) AuthorizeFulfillmentPolicyWrite(context.Context, string) (catalog.FulfillmentPolicyWriteAuthorization, error) {
	return catalog.FulfillmentPolicyWriteAuthorization{ActorUserID: 17}, nil
}

// The real automation service must share publication's transaction, including
// its binding audit, irreversible-reference flag and frozen policy validation.
func TestFulfillmentSettingsRealBindingPostgreSQL(t *testing.T) {
	ctx := context.Background()
	configureCatalogDraftIntegrationDB(t, ctx)
	installAutomationPublicationMigration(t, ctx)
	insertAutomationPublicationCatalog(t, ctx)
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/081-simple-product-version-inventory.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.DB().Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	evidence := automationPublicationEvidence()
	extensions := &automationPublicationExtensions{evidence: evidence}
	audit := &catalogDraftTestAuditWriter{}
	automation, err := productautomation.New(g.DB(), extensions, automationPublicationAuthorizer{}, audit, automationPublicationCodec{evidence: evidence})
	if err != nil {
		t.Fatal(err)
	}
	authority := &catalogProductPublicationTestAuthorizer{actorUserID: 17}
	publication, err := catalog.NewPublication(audit, authority, automation)
	if err != nil {
		t.Fatal(err)
	}
	failing, err := catalog.NewPublication(&catalogDraftTestAuditWriter{err: errors.New("injected final audit failure")}, authority, automation)
	if err != nil {
		t.Fatal(err)
	}
	target, err := automation.CreateInterface(ctx, productautomation.CreateInterfaceInput{DisplayName: "Provisioning supplier", SelectionToken: "opaque-provider-selection"})
	if err != nil {
		t.Fatal(err)
	}
	input := productautomation.FulfillmentSettingsInput{VersionNo: automationVersionNo(2), StrategyCode: catalog.FulfillmentStrategyPaymentConfirmedPlugin, InterfaceInstanceNo: &target.InterfaceInstanceNo}
	assertRolledBack := func() {
		t.Helper()
		count, e := g.DB().GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_catalog_product_version_fulfillment_binding WHERE version_no=?`, input.VersionNo)
		if e != nil || count.Int() != 0 {
			t.Fatalf("partial real binding: %v %v", count, e)
		}
		policy, e := g.DB().GetValue(ctx, `SELECT strategy_code FROM plugin_idc_core_catalog_product_version_fulfillment_policy WHERE version_no=?`, input.VersionNo)
		if e != nil || policy.String() != catalog.FulfillmentStrategyManual {
			t.Fatalf("previous manual policy did not survive rollback: %v %v", policy, e)
		}
		referenced, e := g.DB().GetValue(ctx, `SELECT ever_referenced FROM plugin_idc_core_product_automation_interface_instance WHERE interface_instance_no=?`, target.InterfaceInstanceNo)
		if e != nil || referenced.Bool() {
			t.Fatalf("failed transaction permanently referenced the interface: %v %v", referenced, e)
		}
		assertAutomationPublicationVersionStatus(t, ctx, input.VersionNo, "draft")
	}

	if _, err = productautomation.SaveFulfillmentSettings(ctx, automation, failing, input); err == nil {
		t.Fatal("final audit failure accepted")
	}
	assertRolledBack()
	if err = automation.StopNewBusiness(ctx, target.InterfaceInstanceNo); err != nil {
		t.Fatal(err)
	}
	if _, err = productautomation.SaveFulfillmentSettings(ctx, automation, publication, input); err == nil {
		t.Fatal("stopped interface accepted for a new binding")
	}
	assertRolledBack()
	if err = automation.ResumeNewBusiness(ctx, target.InterfaceInstanceNo); err != nil {
		t.Fatal(err)
	}
	result, err := productautomation.SaveFulfillmentSettings(ctx, automation, publication, input)
	if err != nil || result.Policy.StrategyCode != catalog.FulfillmentStrategyPaymentConfirmedPlugin || result.Contract == nil {
		t.Fatalf("real binding and automatic policy did not save together: %+v %v", result, err)
	}
	binding, err := automation.GetBinding(ctx, input.VersionNo)
	if err != nil || binding == nil || binding.InterfaceInstanceNo != target.InterfaceInstanceNo {
		t.Fatalf("saved binding: %+v %v", binding, err)
	}
	// Explicitly clearing a different editable target does not change the
	// already published historical binding.
	next := automationVersionNo(3)
	if _, err = automation.Bind(ctx, productautomation.BindInput{VersionNo: next, InterfaceInstanceNo: target.InterfaceInstanceNo}); err != nil {
		t.Fatal(err)
	}
	empty := ""
	if _, err = productautomation.SaveFulfillmentSettings(ctx, automation, publication, productautomation.FulfillmentSettingsInput{VersionNo: next, StrategyCode: catalog.FulfillmentStrategyManual, InterfaceInstanceNo: &empty}); err != nil {
		t.Fatal(err)
	}
	if cleared, e := automation.GetBinding(ctx, next); e != nil || cleared != nil {
		t.Fatalf("manual form did not remove selected binding: %+v %v", cleared, e)
	}
	if historical, e := automation.GetBinding(ctx, input.VersionNo); e != nil || historical == nil || historical.InterfaceInstanceNo != target.InterfaceInstanceNo {
		t.Fatalf("historical binding changed: %+v %v", historical, e)
	}
}
