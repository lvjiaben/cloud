// This file verifies product-version cost management against real PostgreSQL.
package catalog

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	productVersionCostTestVersionNo      = "ver_44444444444444444444444444444444"
	productVersionCostOtherTestVersionNo = "ver_55555555555555555555555555555555"
)

func TestProductVersionCostManagementPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}

	ctx := context.Background()
	database := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := database.Exec(ctx, `
CREATE TABLE plugin_idc_core_catalog_product_version (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0,
 version_no VARCHAR(40) NOT NULL UNIQUE
);
CREATE TABLE plugin_idc_core_catalog_product_version_cost (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0,
 cost_no VARCHAR(40) NOT NULL UNIQUE,
 version_no VARCHAR(40) NOT NULL,
 currency_code CHAR(3) NOT NULL,
 period_unit_code VARCHAR(16) NOT NULL,
 period_count INT NOT NULL,
 unit_cost_minor BIGINT NOT NULL,
 effective_from TIMESTAMPTZ NOT NULL,
 effective_until TIMESTAMPTZ,
 version BIGINT NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 CONSTRAINT ck_cost_test_tenant CHECK (tenant_id = 0),
 CONSTRAINT ck_cost_test_window CHECK (effective_until IS NULL OR effective_until > effective_from),
 CONSTRAINT ck_cost_test_version CHECK (version > 0),
 CONSTRAINT fk_cost_test_product_version FOREIGN KEY (version_no)
   REFERENCES plugin_idc_core_catalog_product_version(version_no) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX uk_cost_test_effective ON plugin_idc_core_catalog_product_version_cost
	(version_no,currency_code,period_unit_code,period_count,effective_from);`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no) VALUES
 (0, ?), (0, ?)`, productVersionCostTestVersionNo, productVersionCostOtherTestVersionNo); err != nil {
		t.Fatal(err)
	}

	authorizer := newProductVersionCostTestAuthorizer(t, bizctxcap.CurrentContext{
		UserID: 7, TenantID: 0, DataScope: 2,
	}, true)
	service, err := newManagementService(
		&managementTestAudit{}, authorizer, strings.NewReader(strings.Repeat("c", 16)),
	)
	if err != nil {
		t.Fatalf("create product cost management service: %v", err)
	}

	start := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 6, 0)
	created, err := service.CreateProductVersionCost(ctx, ProductVersionCostCreateInput{
		VersionNo: productVersionCostTestVersionNo, CurrencyCode: "CNY",
		PeriodUnit: PeriodUnitMonth, PeriodCount: 1, UnitCostMinor: 2300,
		EffectiveFrom: &start, EffectiveUntil: &end,
	})
	if err != nil {
		t.Fatalf("create product version cost: %v", err)
	}
	if created.CostNo != "cst_"+strings.Repeat("63", 16) || created.Version != 1 ||
		created.UnitCostMinor != 2300 || !created.EffectiveFrom.Equal(start) ||
		created.EffectiveUntil == nil || !created.EffectiveUntil.Equal(end) {
		t.Fatalf("created product version cost=%+v", created)
	}

	listed, err := service.ListProductVersionCosts(ctx, productVersionCostTestVersionNo)
	if err != nil || len(listed) != 1 || listed[0].CostNo != created.CostNo ||
		listed[0].VersionNo != productVersionCostTestVersionNo || listed[0].UnitCostMinor != 2300 {
		t.Fatalf("list product version costs=%+v err=%v", listed, err)
	}

	// Two staff requests use the same observed version. The row lock and
	// optimistic version predicate must allow exactly one committed update.
	concurrentService, err := newManagementService(
		&managementTestAudit{}, &managementTestAuthorizer{actor: 7}, strings.NewReader(strings.Repeat("d", 16)),
	)
	if err != nil {
		t.Fatalf("create concurrent product cost service: %v", err)
	}
	startGate := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, amount := range []int64{2400, 2500} {
		amount := amount
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-startGate
			_, updateErr := concurrentService.UpdateProductVersionCost(ctx, ProductVersionCostUpdateInput{
				ProductVersionCostCreateInput: ProductVersionCostCreateInput{
					VersionNo: productVersionCostTestVersionNo, CurrencyCode: "CNY",
					PeriodUnit: PeriodUnitMonth, PeriodCount: 1, UnitCostMinor: amount,
					EffectiveFrom: &start, EffectiveUntil: &end,
				},
				CostNo: created.CostNo, ExpectedVersion: 1,
			})
			results <- updateErr
		}()
	}
	close(startGate)
	wait.Wait()
	close(results)
	successes, conflicts := 0, 0
	for updateErr := range results {
		switch {
		case updateErr == nil:
			successes++
		case bizerr.Is(updateErr, CodeManagementConflict):
			conflicts++
		default:
			t.Fatalf("concurrent update returned %v", updateErr)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent product cost updates successes=%d conflicts=%d", successes, conflicts)
	}
	listed, err = service.ListProductVersionCosts(ctx, productVersionCostTestVersionNo)
	if err != nil || len(listed) != 1 || listed[0].Version != 2 ||
		(listed[0].UnitCostMinor != 2400 && listed[0].UnitCostMinor != 2500) {
		t.Fatalf("stored product version cost after concurrent update=%+v err=%v", listed, err)
	}

	deniedService, err := newManagementService(
		&managementTestAudit{},
		newProductVersionCostTestAuthorizer(t, bizctxcap.CurrentContext{UserID: 8, TenantID: 0, DataScope: 2}, false),
		strings.NewReader(strings.Repeat("e", 16)),
	)
	if err != nil {
		t.Fatalf("create denied product cost service: %v", err)
	}
	if _, err = deniedService.CreateProductVersionCost(ctx, ProductVersionCostCreateInput{
		VersionNo: productVersionCostOtherTestVersionNo, CurrencyCode: "CNY",
		PeriodUnit: PeriodUnitOnce, PeriodCount: 1, UnitCostMinor: 1,
	}); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("missing cost permission returned %v", err)
	}
	count, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_catalog_product_version_cost WHERE version_no=?`, productVersionCostOtherTestVersionNo)
	if err != nil || count.Int() != 0 {
		t.Fatalf("denied cost write reached persistence count=%d err=%v", count.Int(), err)
	}

	unsafeScopeService, err := newManagementService(
		&managementTestAudit{},
		newProductVersionCostTestAuthorizer(t, bizctxcap.CurrentContext{UserID: 9, TenantID: 0, DataScope: 3}, true),
		strings.NewReader(strings.Repeat("f", 16)),
	)
	if err != nil {
		t.Fatalf("create unsafe-scope product cost service: %v", err)
	}
	if _, err = unsafeScopeService.ListProductVersionCosts(ctx, productVersionCostTestVersionNo); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("unsupported data scope returned %v", err)
	}
	if _, err = service.ListProductVersionCosts(ctx, "ver_66666666666666666666666666666666"); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("missing product version returned %v", err)
	}
}

func newProductVersionCostTestAuthorizer(
	t *testing.T,
	current bizctxcap.CurrentContext,
	grant bool,
) ManagementAuthorizer {
	t.Helper()
	authorizer, err := NewManagementAuthorizer(
		managementTestBizCtx{current: current},
		&managementTestPrincipal{audience: &principal.Audience{
			UserID: current.UserID, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
		}},
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionCatalogCostWrite): grant,
		}},
	)
	if err != nil {
		t.Fatalf("create product version cost authorizer: %v", err)
	}
	return authorizer
}
