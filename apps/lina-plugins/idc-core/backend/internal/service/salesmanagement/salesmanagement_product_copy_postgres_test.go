// This file tests current tier-price copying with real PostgreSQL, including
// history selection, customer exclusion, regenerated identity and rollback.

package salesmanagement

import (
	"context"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/util/guid"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
)

func TestProductPriceCopyPostgreSQL(t *testing.T) {
	managementPostgres(t, func(ctx context.Context, db gdb.DB, s *serviceImpl, audit *managementTestAudit) {
		suffix := guid.S()
		source, target, failed, category, customer, version, billing := "PCPS_"+suffix, "PCPT_"+suffix, "PCPF_"+suffix, "PCPC_"+suffix, "PCPU_"+suffix, "PCPV_"+suffix, "PCPB_"+suffix
		fixtures := []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO plugin_idc_core_customer_subject(tenant_id,customer_no,customer_type_code,status_code,display_name,country_code,preferred_locale,created_by,updated_by) VALUES(0,?,'individual','active','Copy Test','CN','zh-CN',81,81)`, []any{customer}},
			{`INSERT INTO plugin_idc_core_catalog_category(tenant_id,category_no,name_zh_cn,name_en_us,status_code,created_by,updated_by) VALUES(0,?,'Copy Test','Copy Test','active',81,81)`, []any{category}},
			{`INSERT INTO plugin_idc_core_catalog_product(tenant_id,product_no,category_no,status_code,public_slug,name_zh_cn,name_en_us,created_by,updated_by) VALUES(0,?,?,'active',?,'Source','Source',81,81),(0,?,?,'draft',?,'Copy','Copy',81,81),(0,?,?,'draft',?,'Failed','Failed',81,81)`, []any{source, category, "copy-source-" + suffix, target, category, "copy-target-" + suffix, failed, category, "copy-failed-" + suffix}},
			{`INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no,product_no,version_number,status_code,source_code,source_version,source_fingerprint,created_by) VALUES(0,?,?,1,'draft','manual','1',?,81)`, []any{version, source, strings.Repeat("a", 64)}},
			{`UPDATE plugin_idc_core_catalog_product SET current_version_no=? WHERE product_no=?`, []any{version, source}},
			{`INSERT INTO plugin_idc_core_catalog_billing_rule(tenant_id,billing_rule_no,version_no,rule_code,charge_type_code,period_unit_code,period_count,price_phase_code,quantity_mode_code,status_code) VALUES(0,?,?,'monthly','recurring','month',1,'new_purchase','fixed','active')`, []any{billing, version}},
			{`INSERT INTO plugin_idc_core_catalog_price(tenant_id,price_no,version_no,billing_rule_no,currency_code,amount_minor,status_code) VALUES(0,?,?,?,'CNY',10000,'active')`, []any{"PCPR_" + suffix, version, billing}},
		}
		for _, fixture := range fixtures {
			if _, err := db.Exec(ctx, fixture.sql, fixture.args...); err != nil {
				t.Fatal(err)
			}
		}
		level, err := s.SaveLevel(ctx, LevelInput{Name: "Copy Tier " + suffix, Status: statusActive})
		if err != nil {
			t.Fatal(err)
		}
		input := PriceInput{LevelNo: level.LevelNo, ProductNo: source, CurrencyCode: "CNY", ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1, AdjustmentType: adjustmentFixed, Status: statusActive, FixedPriceMinor: "8000"}
		price, err := s.SavePrice(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		input.PriceNo = price.PriceNo
		input.ExpectedVersion = price.Version
		input.FixedPriceMinor = "7000"
		if _, err = s.SavePrice(ctx, input); err != nil {
			t.Fatal(err)
		}
		customerInput := input
		customerInput.LevelNo = ""
		customerInput.CustomerNo = customer
		customerInput.PriceNo = ""
		customerInput.ExpectedVersion = 0
		customerInput.FixedPriceMinor = "5000"
		if _, err = s.SavePrice(ctx, customerInput); err != nil {
			t.Fatal(err)
		}
		if err = s.CopyLevelProductPrices(ctx, source, target); err != nil {
			t.Fatal(err)
		}
		copied, err := db.GetAll(ctx, `SELECT price_no,level_no,customer_no,fixed_price_minor,version,content_digest FROM plugin_idc_core_sales_purchase_price WHERE product_no=?`, target)
		if err != nil || len(copied) != 1 || copied[0]["price_no"].String() == price.PriceNo || copied[0]["level_no"].String() != level.LevelNo || !copied[0]["customer_no"].IsNil() || copied[0]["fixed_price_minor"].Int64() != 7000 || copied[0]["version"].Int64() != 1 || len(copied[0]["content_digest"].String()) != 64 {
			t.Fatalf("wrong copied latest price: %v %v", copied, err)
		}
		if err = s.CopyLevelProductPrices(ctx, source, target); !bizerr.Is(err, codeConflict) {
			t.Fatalf("duplicate copy accepted: %v", err)
		}
		duplicateCount, duplicateErr := db.GetCount(ctx, `SELECT 1 FROM plugin_idc_core_sales_purchase_price WHERE product_no=?`, target)
		if duplicateErr != nil || duplicateCount != 1 {
			t.Fatalf("duplicate copy changed target rules: %d %v", duplicateCount, duplicateErr)
		}
		audit.fail = true
		err = db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error { return s.CopyLevelProductPrices(txCtx, source, failed) })
		if err == nil {
			t.Fatal("failed audit accepted")
		}
		count, countErr := db.GetCount(ctx, `SELECT 1 FROM plugin_idc_core_sales_purchase_price WHERE product_no=?`, failed)
		if countErr != nil || count != 0 {
			t.Fatalf("copy survived rollback: %d %v", count, countErr)
		}
		audit.fail = false
		s.authorization = managementTestAuth{grants: map[authz.PermissionKey]bool{}}
		if err = s.CopyLevelProductPrices(ctx, source, failed); !bizerr.Is(err, codeDenied) {
			t.Fatalf("unauthorized price copy: %v", err)
		}
		count, countErr = db.GetCount(ctx, `SELECT 1 FROM plugin_idc_core_sales_purchase_price WHERE product_no=?`, source)
		if countErr != nil || count != 3 {
			t.Fatalf("source price history changed: %d %v", count, countErr)
		}
	})
}
