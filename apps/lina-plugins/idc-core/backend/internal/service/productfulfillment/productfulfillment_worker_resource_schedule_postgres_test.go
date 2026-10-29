// This file checks persisted coordination delays, exact lease fences and
// unchanged attempt counters using transaction-local PostgreSQL fixtures.

package productfulfillment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

func TestLocalResourceTaskSchedulingPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN not configured")
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	err = database.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		if _, err := database.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_product_fulfillment_task(
tenant_id INT,task_no TEXT,status_code TEXT,phase_code TEXT,next_attempt_at TIMESTAMPTZ,
lease_token TEXT,lease_expires_at TIMESTAMPTZ,external_reference TEXT,last_result_code TEXT,
upstream_deliveries JSONB DEFAULT '[]',version BIGINT,updated_at TIMESTAMPTZ,
query_attempt_count INT DEFAULT 2,submit_attempt_count INT DEFAULT 1) ON COMMIT DROP`); err != nil {
			return err
		}
		worker := &fulfillmentWorker{database: database}
		for _, test := range []struct {
			name, owner string
			status      productfulfillmentcap.ResultStatus
			previous    string
			seconds     int
		}{
			{name: "local-running", owner: provisionresourcecap.OwnerID, status: productfulfillmentcap.StatusRunning, seconds: resourceOwnerProgressIntervalSeconds},
			{name: "local-still-running", owner: provisionresourcecap.OwnerID, previous: "running", status: productfulfillmentcap.StatusRunning, seconds: resourceOwnerProgressIntervalSeconds},
			{name: "local-absent", owner: provisionresourcecap.OwnerID, status: productfulfillmentcap.StatusNotFound, seconds: 0},
			{name: "remote-running", owner: "remote", status: productfulfillmentcap.StatusRunning, seconds: retryIntervalSeconds},
		} {
			if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_task(tenant_id,task_no,status_code,phase_code,lease_token,lease_expires_at,external_reference,version) VALUES(0,?,'querying','post_submit_query','owned-lease',CURRENT_TIMESTAMP+INTERVAL '2 minutes','resource-task',1)`, test.name); err != nil {
				return err
			}
			claim := &claimedFulfillment{TaskNo: test.name, LeaseToken: "owned-lease", EntitlementOwnerID: test.owner, Action: productfulfillmentcap.ActionQuery, PreviousStatus: test.previous}
			decision := workerDecision{TaskStatus: "running", PhaseCode: "post_submit_query", ScheduleNext: true}
			if err := worker.finalizeTaskState(ctx, claim, 1, decision, "resource-task", "provider_result", &productfulfillmentcap.Result{Status: test.status}); err != nil {
				return err
			}
			var row struct {
				Delay     int
				Unlocked  bool
				Version   int
				Queries   int
				Submits   int
				Reference string
			}
			if err := database.GetScan(ctx, &row, `SELECT EXTRACT(EPOCH FROM next_attempt_at-CURRENT_TIMESTAMP)::INT AS delay,(lease_token='' AND lease_expires_at IS NULL) AS unlocked,version,query_attempt_count AS queries,submit_attempt_count AS submits,external_reference AS reference FROM plugin_idc_core_product_fulfillment_task WHERE task_no=?`, test.name); err != nil {
				return err
			}
			if row.Delay != test.seconds || !row.Unlocked || row.Version != 2 || row.Queries != 2 || row.Submits != 1 || row.Reference != "resource-task" {
				t.Fatalf("local coordination changed its policy or execution facts: case=%s result=%+v", test.name, row)
			}
			if err := worker.finalizeTaskState(ctx, claim, 1, decision, "other-resource", "provider_result", &productfulfillmentcap.Result{Status: test.status}); err == nil {
				t.Fatal("stale lease/version rewrote a task schedule")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Numeric budget parameters must work through the real database driver. Local
// post-submit reads continue past ten; remote and preparation work stay bounded.
func TestResourceQueryBudgetClaimPredicatePostgreSQL(t *testing.T) {
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
	for _, tc := range []struct {
		owner, phase string
		count        int
		allowed      bool
	}{
		{provisionresourcecap.OwnerID, "post_submit_query", 20, true},
		{provisionresourcecap.OwnerID, "post_submit_query", resourceOwnerMaximumQueryAttempts, false},
		{provisionresourcecap.OwnerID, "pre_submit_query", maximumQueryAttempts, false},
		{"another-owner", "post_submit_query", maximumQueryAttempts, false},
	} {
		value, err := db.GetValue(t.Context(), `SELECT ? < `+fulfillmentQueryBudgetSQL+` FROM (SELECT CAST(? AS TEXT) AS entitlement_owner_id) intent CROSS JOIN (SELECT CAST(? AS TEXT) AS phase_code) task`, tc.count, provisionresourcecap.OwnerID, resourceOwnerMaximumQueryAttempts, maximumQueryAttempts, tc.owner, tc.phase)
		if err != nil {
			t.Fatal(err)
		}
		if value.Bool() != tc.allowed {
			t.Fatalf("claim owner=%s phase=%s count=%d allowed=%v", tc.owner, tc.phase, tc.count, value.Bool())
		}
	}
}

// Replaying the production migration must allow audited local reads beyond ten
// without weakening write attempts or counter consistency.
func TestLocalObservationMigrationPostgreSQL(t *testing.T) {
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
	sql, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "sql", "145-local-fulfillment-observation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	err = db.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		if _, err := db.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_product_fulfillment_task(maximum_attempts INT,retry_base_seconds INT,recovery_count INT,recovery_query_baseline INT,recovery_submit_baseline INT,query_attempt_count INT,submit_attempt_count INT,attempt_count INT) ON COMMIT DROP`); err != nil {
			return err
		}
		for i := 0; i < 2; i++ {
			if _, err := db.Exec(ctx, string(sql)); err != nil {
				return err
			}
		}
		if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_task VALUES(10,30,0,0,0,20,1,21)`); err != nil {
			return err
		}
		for _, values := range []string{"(10,30,0,0,0,20,11,31)", "(10,30,0,0,0,20,1,20)", "(10,30,0,21,0,20,1,21)"} {
			invalid := db.Transaction(ctx, func(inner context.Context, _ gdb.TX) error {
				_, err := db.Exec(inner, `INSERT INTO plugin_idc_core_product_fulfillment_task VALUES`+values)
				return err
			})
			if invalid == nil {
				t.Fatal("migration removed a write or audit invariant", values)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
