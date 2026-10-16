// This file verifies trial publication admission against active billing rules
// and exact-release lifecycle evidence, including manual and imported drafts.

package productautomation

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func TestTrialPublicationRequiresPauseResumePostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newAutomationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteAutomationIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, `DROP SCHEMA `+quoteAutomationIdentifier(schema)+` CASCADE`)
		_ = admin.Close(ctx)
	})
	database := openAutomationDatabase(t, ctx, dsn, schema)
	_, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_billing_rule (
tenant_id INT NOT NULL,version_no VARCHAR(40) NOT NULL,
charge_type_code VARCHAR(16) NOT NULL,status_code VARCHAR(16) NOT NULL);
INSERT INTO plugin_idc_core_catalog_billing_rule VALUES
(0,'trial-version','trial','active'),(0,'ordinary-version','recurring','active'),
(0,'inactive-version','trial','inactive');`)
	if err != nil {
		t.Fatal(err)
	}
	service := &serviceImpl{database: database}
	for _, scenario := range []struct {
		name, version, strategy string
		contract                *catalog.FrozenFulfillmentContract
		denied                  bool
	}{
		{"manual trial", "trial-version", catalog.FulfillmentStrategyManual, nil, true},
		{"automatic without lifecycle", "trial-version", catalog.FulfillmentStrategyPaymentConfirmedPlugin, &catalog.FrozenFulfillmentContract{}, true},
		{"manual cannot borrow capability", "trial-version", catalog.FulfillmentStrategyManual, &catalog.FrozenFulfillmentContract{TrialPauseResume: true}, true},
		{"supported automatic trial", "trial-version", catalog.FulfillmentStrategyPaymentConfirmedPlugin, &catalog.FrozenFulfillmentContract{TrialPauseResume: true}, false},
		{"ordinary manual product", "ordinary-version", catalog.FulfillmentStrategyManual, nil, false},
		{"inactive historical trial", "inactive-version", catalog.FulfillmentStrategyManual, nil, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			err := service.validateTrialFulfillment(ctx, scenario.version, scenario.strategy, scenario.contract)
			if scenario.denied {
				if !bizerr.Is(err, catalog.CodeTrialFulfillmentUnavailable) {
					t.Fatalf("unsafe trial publication error=%v", err)
				}
			} else if err != nil {
				t.Fatalf("unrelated or supported product rejected: %v", err)
			}
		})
	}
}
