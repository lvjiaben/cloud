// These tests prove currency configuration, default protection and audit rollback on real PostgreSQL.
package currency

import (
	"context"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"os"
	"testing"
)

type testContext struct {
	bizctxcap.Service
	current bizctxcap.CurrentContext
}

func (s testContext) Current(context.Context) bizctxcap.CurrentContext { return s.current }

type testIdentity struct{ customer bool }

func (s testIdentity) Current(context.Context) (*principal.Audience, error) {
	if s.customer {
		return &principal.Audience{UserID: 81, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "customer"}, nil
	}
	return &principal.Audience{UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, nil
}

type testAuth struct {
	authz.Service
	allow bool
}

func (s testAuth) BatchHasPermissions(_ context.Context, keys []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	r := map[authz.PermissionKey]bool{}
	for _, k := range keys {
		r[k] = s.allow
	}
	return r, nil
}

type testAudit struct {
	idccap.BusinessAuditWriter
	fail  bool
	count int
}

func (s *testAudit) AppendBusinessTransition(_ context.Context, input idccap.BusinessAuditTransition) error {
	if input.BeforeSnapshot == nil {
		return gerror.New("audit requires object before snapshot")
	}
	if s.fail {
		return gerror.New("test audit failure")
	}
	s.count++
	return nil
}
func TestCurrencyAuthorization(t *testing.T) {
	s := &serviceImpl{bizCtx: testContext{current: bizctxcap.CurrentContext{UserID: 81, DataScope: 1}}, principal: testIdentity{}, authorization: testAuth{allow: false}}
	if _, err := s.List(context.Background(), 1, 20); !bizerr.Is(err, codeDenied) {
		t.Fatalf("permission denial: %v", err)
	}
	s.authorization = testAuth{allow: true}
	s.bizCtx = testContext{current: bizctxcap.CurrentContext{UserID: 81, DataScope: 3}}
	if _, err := s.Create(context.Background(), "USD"); !bizerr.Is(err, codeDenied) {
		t.Fatalf("scope denial: %v", err)
	}
	s.bizCtx = testContext{current: bizctxcap.CurrentContext{UserID: 81, DataScope: 1}}
	s.principal = testIdentity{customer: true}
	if _, err := s.Update(context.Background(), "CNY", 1, true, true); !bizerr.Is(err, codeDenied) {
		t.Fatalf("customer denial: %v", err)
	}
}
func TestCurrencyConfigurationPostgreSQL(t *testing.T) {
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
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			t.Error(err)
		}
	}()
	ctx := gdb.WithTX(tx.GetCtx(), tx)
	// Lock the same merchant key before fixture deletion; everything rolls back.
	if _, err = db.Exec(ctx, `SELECT tenant_id FROM plugin_idc_core_merchant_profile WHERE tenant_id=0 FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_merchant_profile SET default_currency='CNY' WHERE tenant_id=0; DELETE FROM plugin_idc_core_merchant_currency WHERE tenant_id=0 AND currency_code IN ('USD','EUR'); UPDATE plugin_idc_core_merchant_currency SET enabled=TRUE WHERE tenant_id=0 AND currency_code='CNY'`); err != nil {
		t.Fatal(err)
	}
	audit := &testAudit{}
	owner, err := New(db, testContext{current: bizctxcap.CurrentContext{UserID: 81, DataScope: 1}}, testIdentity{}, testAuth{allow: true}, audit)
	if err != nil {
		t.Fatal(err)
	}
	item, err := owner.Create(ctx, "USD")
	if err != nil || !item.Enabled {
		t.Fatalf("create: %+v %v", item, err)
	}
	if _, err = owner.Create(ctx, "USD"); !bizerr.Is(err, codeConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err = owner.Create(ctx, "ZZZ"); !bizerr.Is(err, codeInvalid) {
		t.Fatalf("invalid: %v", err)
	}
	item, err = owner.Update(ctx, "USD", item.Version, false, false)
	if err != nil {
		t.Fatal(err)
	}
	allowed, err := owner.IsEnabled(ctx, "USD")
	if err != nil || allowed {
		t.Fatalf("disabled admission: %v %v", allowed, err)
	}
	if _, err = owner.Update(ctx, "USD", 1, true, false); !bizerr.Is(err, codeConflict) {
		t.Fatalf("stale: %v", err)
	}
	item, err = owner.Update(ctx, "USD", item.Version, true, true)
	if err != nil || !item.IsDefault {
		t.Fatalf("default: %+v %v", item, err)
	}
	if _, err = owner.Update(ctx, "USD", item.Version, false, false); !bizerr.Is(err, codeDefault) {
		t.Fatalf("default disable: %v", err)
	}
	codes, err := owner.Enabled(ctx)
	if err != nil || len(codes) < 2 || codes[0] != "USD" {
		t.Fatalf("options: %v %v", codes, err)
	}
	page, err := owner.List(ctx, 1, 1)
	if err != nil || page.Total < 2 || len(page.Items) != 1 || page.Items[0].CurrencyCode != "USD" {
		t.Fatalf("pagination: %+v %v", page, err)
	}
	if err = owner.Delete(ctx, "USD", item.Version); !bizerr.Is(err, codeDefault) {
		t.Fatalf("default deletion: %v", err)
	}
	cnyVersion, err := db.GetValue(ctx, `SELECT version FROM plugin_idc_core_merchant_currency WHERE currency_code='CNY'`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Update(ctx, "CNY", cnyVersion.Int64(), true, true); err != nil {
		t.Fatal(err)
	}
	audit.fail = true
	if err = owner.Delete(ctx, "USD", item.Version); err == nil {
		t.Fatal("audit failure accepted deletion")
	}
	allowed, err = owner.IsEnabled(ctx, "USD")
	if err != nil || !allowed {
		t.Fatalf("delete rollback: %v %v", allowed, err)
	}
	audit.fail = false
	if err = owner.Delete(ctx, "USD", item.Version-1); !bizerr.Is(err, codeConflict) {
		t.Fatalf("stale deletion: %v", err)
	}
	historyBefore, err := db.GetValue(ctx, `SELECT json_build_array((SELECT COUNT(*) FROM plugin_idc_core_catalog_price),(SELECT COUNT(*) FROM plugin_idc_core_billing_invoice),(SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_account))::text`)
	if err != nil {
		t.Fatal(err)
	}
	if err = owner.Delete(ctx, "USD", item.Version); err != nil {
		t.Fatal(err)
	}
	allowed, err = owner.IsEnabled(ctx, "USD")
	if err != nil || allowed {
		t.Fatalf("deleted admission: %v %v", allowed, err)
	}
	historyAfter, err := db.GetValue(ctx, `SELECT json_build_array((SELECT COUNT(*) FROM plugin_idc_core_catalog_price),(SELECT COUNT(*) FROM plugin_idc_core_billing_invoice),(SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_account))::text`)
	if err != nil || historyBefore.String() != historyAfter.String() {
		t.Fatalf("historical records changed: %v", err)
	}
	removed, err := owner.IsRemoved(ctx, "USD")
	if err != nil || !removed {
		t.Fatalf("removed marker: %v %v", removed, err)
	}
	page, err = owner.List(ctx, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range page.Items {
		if row.CurrencyCode == "USD" {
			t.Fatal("deleted currency listed")
		}
	}
	restored, err := owner.Create(ctx, "USD")
	if err != nil || restored.Version != item.Version+2 {
		t.Fatalf("restore continuity: %+v %v", restored, err)
	}
	item, err = owner.Update(ctx, "USD", restored.Version, true, true)
	if err != nil {
		t.Fatal(err)
	}
	audit.fail = true
	if _, err = owner.Create(ctx, "EUR"); err == nil {
		t.Fatal("audit failure accepted")
	}
	exists, err := db.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_merchant_currency WHERE tenant_id=0 AND currency_code='EUR'`)
	if err != nil {
		t.Fatal(err)
	}
	if exists.Int() != 0 {
		t.Fatal("audit failure created currency")
	}
	if _, err = owner.Update(ctx, "USD", item.Version, false, false); !bizerr.Is(err, codeDefault) {
		t.Fatalf("default policy lost: %v", err)
	}
	audit.fail = true
	if _, err = owner.Update(ctx, "USD", item.Version, true, false); err == nil {
		t.Fatal("audit update accepted")
	}
	page, err = owner.List(ctx, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range page.Items {
		if row.CurrencyCode == "USD" && row.Version != item.Version {
			t.Fatal("audit failure changed currency version")
		}
	}
}
