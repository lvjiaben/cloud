// This file verifies retained add-on snapshots, optimistic updates, tenant
// isolation and full rollback when the required audit cannot be appended.

package catalog

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"reflect"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type addonAudit struct {
	idccap.BusinessAuditWriter
	fail   bool
	events []idccap.BusinessAuditTransition
}

func (a *addonAudit) AppendBusinessTransition(_ context.Context, event idccap.BusinessAuditTransition) error {
	if event.BeforeSnapshot == nil || event.AfterSnapshot == nil {
		return errors.New("missing transition snapshots")
	}
	a.events = append(a.events, event)
	if a.fail {
		return errors.New("audit unavailable")
	}
	return nil
}

func TestProductAddonsAuditPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	const product = "prd_11111111111111111111111111111111"
	const old = "prd_22222222222222222222222222222222"
	const next = "prd_33333333333333333333333333333333"
	const foreign = "prd_44444444444444444444444444444444"

	for _, statement := range []string{
		"CREATE TABLE plugin_idc_core_catalog_product(id BIGSERIAL,tenant_id INT,product_no TEXT,version BIGINT,updated_by INT,updated_at TIMESTAMPTZ,deleted_at TIMESTAMPTZ)",
		"CREATE TABLE plugin_idc_core_catalog_product_addon(tenant_id INT,product_no TEXT,addon_product_no TEXT)",
	} {
		if _, err := db.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(ctx, "INSERT INTO plugin_idc_core_catalog_product(tenant_id,product_no,version) VALUES(0,?,1),(0,?,1),(0,?,1),(9,?,1)", product, old, next, foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO plugin_idc_core_catalog_product_addon VALUES(0,?,?)", product, old); err != nil {
		t.Fatal(err)
	}
	audit := &addonAudit{fail: true}
	svc, err := newManagementService(audit, &managementTestAuthorizer{actor: 7}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	input := ProductAddonsInput{ProductNo: product, ProductNos: []string{next}, Version: 1}
	if _, err = svc.SetProductAddons(ctx, input); err == nil {
		t.Fatal("audit failure committed")
	}
	retained, err := db.GetValue(ctx, "SELECT addon_product_no FROM plugin_idc_core_catalog_product_addon")
	if err != nil || retained.String() != old {
		t.Fatalf("rollback lost: %v %v", retained, err)
	}
	audit.fail = false
	if version, err := svc.SetProductAddons(ctx, input); err != nil || version != 2 {
		t.Fatalf("update: %d %v", version, err)
	}
	event := audit.events[len(audit.events)-1]
	before := event.BeforeSnapshot.(map[string]any)
	if !reflect.DeepEqual(before["product_nos"], []string{old}) || before["version"] != int64(1) {
		t.Fatalf("wrong before: %#v", before)
	}
	if _, err = svc.SetProductAddons(ctx, input); !bizerr.Is(err, CodeManagementConflict) {
		t.Fatalf("stale revision: %v", err)
	}
	input.Version = 2
	input.ProductNos = []string{foreign}
	if _, err = svc.SetProductAddons(ctx, input); !bizerr.Is(err, CodeManagementInvalid) {
		t.Fatalf("foreign offer: %v", err)
	}
	input.ProductNos = []string{}
	if _, err = svc.SetProductAddons(ctx, input); err != nil {
		t.Fatal(err)
	}
	count, err := db.GetValue(ctx, "SELECT COUNT(1) FROM plugin_idc_core_catalog_product_addon")
	if err != nil || count.Int() != 0 {
		t.Fatal("clear failed")
	}
}
