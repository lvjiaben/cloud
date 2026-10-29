// These tests validate the independent algorithm migration on a disposable
// PostgreSQL temporary table, preserving old modes without changing data.
package catalog

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestGraduatedConfigurationMigrationPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL DSN not configured")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := db.Close(context.Background()); e != nil {
			t.Error(e)
		}
	})
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/144-graduated-configuration-pricing.sql"))
	if err != nil {
		t.Fatal(err)
	}
	err = db.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		if _, e := db.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_catalog_configuration_price_dimension(pricing_mode_code VARCHAR(16) NOT NULL) ON COMMIT DROP;
  INSERT INTO plugin_idc_core_catalog_configuration_price_dimension VALUES('fixed'),('per_unit'),('band_total');`); e != nil {
			return e
		}
		for n := 0; n < 2; n++ {
			if _, e := db.Exec(ctx, string(raw)); e != nil {
				return e
			}
		}
		if _, e := db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension VALUES('graduated')`); e != nil {
			return e
		}
		count, e := db.GetValue(ctx, `SELECT count(DISTINCT pricing_mode_code) FROM plugin_idc_core_catalog_configuration_price_dimension`)
		if e != nil {
			return e
		}
		if count.Int() != 4 {
			t.Fatal("migration rewrote existing algorithm data")
		}
		// The invalid value is rejected in a savepoint, keeping the outer fixture usable.
		if e = db.Transaction(ctx, func(inner context.Context, _ gdb.TX) error {
			_, insertErr := db.Exec(inner, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension VALUES('unknown')`)
			return insertErr
		}); e == nil {
			t.Fatal("unknown algorithm accepted")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
