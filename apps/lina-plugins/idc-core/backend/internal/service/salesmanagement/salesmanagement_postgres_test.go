// These tests use rollback-only transactions against the explicitly configured
// local PostgreSQL database. They do not mutate host tables or existing rows.
package salesmanagement

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/util/guid"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
)

type managementTestContext struct {
	bizctxcap.Service
	current bizctxcap.CurrentContext
}

func (s managementTestContext) Current(context.Context) bizctxcap.CurrentContext { return s.current }

type managementTestPrincipal struct{}

func (managementTestPrincipal) Current(context.Context) (*principal.Audience, error) {
	return &principal.Audience{UserID: 81, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, nil
}

type managementTestAuth struct {
	authz.Service
	allow  bool
	grants map[authz.PermissionKey]bool
}

func (s managementTestAuth) BatchHasPermissions(_ context.Context, keys []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	result := map[authz.PermissionKey]bool{}
	for _, key := range keys {
		result[key] = s.allow || s.grants[key]
	}
	return result, nil
}

type managementTestAudit struct {
	fail  bool
	count int
}

func (s *managementTestAudit) AppendBusinessTransition(context.Context, idccap.BusinessAuditTransition) error {
	if s.fail {
		return errors.New("audit unavailable")
	}
	s.count++
	return nil
}
func (s *managementTestAudit) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type managementTestUsers struct{ usercap.Service }

// managementPostgres rolls back fixture and service writes even on assertion failure.
func managementPostgres(t *testing.T, run func(context.Context, gdb.DB, *serviceImpl, *managementTestAudit)) {
	t.Helper()
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
	tx, err := database.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			t.Error(err)
		}
	}()
	ctx := gdb.WithTX(tx.GetCtx(), tx)
	audit := &managementTestAudit{}
	owner, err := New(database, managementTestContext{current: bizctxcap.CurrentContext{UserID: 81, DataScope: 1}}, managementTestPrincipal{}, managementTestAuth{allow: true}, managementTestUsers{}, saleschannel.New(), audit)
	if err != nil {
		t.Fatal(err)
	}
	run(ctx, database, owner.(*serviceImpl), audit)
}
func TestSalesManagementLevelsPostgreSQL(t *testing.T) {
	managementPostgres(t, func(ctx context.Context, db gdb.DB, s *serviceImpl, audit *managementTestAudit) {
		suffix := guid.S()
		level, err := s.SaveLevel(ctx, LevelInput{Name: "test-" + suffix, Status: statusActive})
		if err != nil {
			t.Fatal(err)
		}
		page, err := s.ListLevels(ctx, PageInput{Search: suffix})
		if err != nil || page.Total != 1 || len(page.Items) != 1 {
			t.Fatalf("persisted page: %+v %v", page, err)
		}
		if _, err = s.SaveLevel(ctx, LevelInput{LevelNo: level.LevelNo, Name: "stale", Status: statusActive, ExpectedVersion: 0}); !bizerr.Is(err, codeConflict) {
			t.Fatalf("stale update: %v", err)
		}
		audit.fail = true
		if _, err = s.SaveLevel(ctx, LevelInput{LevelNo: level.LevelNo, Name: "should rollback", Status: statusActive, ExpectedVersion: level.Version}); err == nil {
			t.Fatal("audit failure was ignored")
		}
		audit.fail = false
		page, err = s.ListLevels(ctx, PageInput{Search: suffix})
		if err != nil || page.Total != 1 {
			t.Fatalf("audit failure wrote level: %+v %v", page, err)
		}
	})
}
func TestSalesManagementPricingPostgreSQL(t *testing.T) {
	managementPostgres(t, func(ctx context.Context, db gdb.DB, s *serviceImpl, audit *managementTestAudit) {
		suffix := guid.S()
		customer := "CUS_" + suffix
		product := "PRD_" + suffix
		category := "CAT_" + suffix
		version := "VER_" + suffix
		billing := "BIL_" + suffix
		fixture := []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO plugin_idc_core_customer_subject(tenant_id,customer_no,customer_type_code,status_code,display_name,country_code,preferred_locale,created_by,updated_by) VALUES(0,?,'individual','active','Sales Test','CN','zh-CN',81,81)`, []any{customer}},
			{`INSERT INTO plugin_idc_core_catalog_category(tenant_id,category_no,name_zh_cn,name_en_us,status_code,created_by,updated_by) VALUES(0,?,'Sales Test','Sales Test','active',81,81)`, []any{category}},
			{`INSERT INTO plugin_idc_core_catalog_product(tenant_id,product_no,category_no,status_code,public_slug,name_zh_cn,name_en_us,created_by,updated_by) VALUES(0,?,?,'active',?,'Sales Test','Sales Test',81,81)`, []any{product, category, "sales-" + suffix}},
			{`INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no,product_no,version_number,status_code,source_code,source_version,source_fingerprint,created_by) VALUES(0,?,?,1,'draft','manual','1',? ,81)`, []any{version, product, strings.Repeat("a", 64)}},
			{`UPDATE plugin_idc_core_catalog_product SET current_version_no=? WHERE product_no=?`, []any{version, product}},
			{`INSERT INTO plugin_idc_core_catalog_billing_rule(tenant_id,billing_rule_no,version_no,rule_code,charge_type_code,period_unit_code,period_count,price_phase_code,quantity_mode_code,status_code) VALUES(0,?,?,'monthly','recurring','month',1,'new_purchase','fixed','active')`, []any{billing, version}},
			{`INSERT INTO plugin_idc_core_catalog_price(tenant_id,price_no,version_no,billing_rule_no,currency_code,amount_minor,status_code) VALUES(0,?,?,?,'CNY',10000,'active')`, []any{"CPR_" + suffix, version, billing}},
		}
		for _, item := range fixture {
			if _, err := db.Exec(ctx, item.sql, item.args...); err != nil {
				t.Fatal(err)
			}
		}
		level, err := s.SaveLevel(ctx, LevelInput{Name: "Agent test " + suffix, Status: statusActive})
		if err != nil {
			t.Fatal(err)
		}
		agent, err := s.SaveAgent(ctx, AgentInput{CustomerNo: customer, LevelNo: level.LevelNo, Status: statusActive})
		if err != nil {
			t.Fatal(err)
		}
		unset, err := s.GetLevelPricing(ctx, level.LevelNo)
		if err != nil || unset.Version != 0 || unset.PercentageBasisPoints != nil {
			t.Fatalf("unset default: %+v %v", unset, err)
		}
		charged := 8000
		global, err := s.SaveLevelPricing(ctx, level.LevelNo, &charged, 0)
		if err != nil || global.Version != 1 {
			t.Fatalf("save default: %+v %v", global, err)
		}
		audit.fail = true
		if _, err = s.SaveLevelPricing(ctx, level.LevelNo, &charged, global.Version); err == nil {
			t.Fatal("price audit failure ignored")
		}
		audit.fail = false
		retained, err := s.GetLevelPricing(ctx, level.LevelNo)
		if err != nil || retained.Version != global.Version {
			t.Fatalf("failed audit wrote default: %+v %v", retained, err)
		}
		s.authorization = managementTestAuth{grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionOverview): true}}
		overviewOnly, err := s.ListLevels(ctx, PageInput{Search: suffix})
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range overviewOnly.Items {
			if item.PercentageBasisPoints != nil || item.PricingVersion != 0 {
				t.Fatalf("overview leaked tier pricing: %+v", item)
			}
		}
		s.authorization = managementTestAuth{grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionPricing): true}}
		pricingOnly, err := s.PriceLevelCandidates(ctx, PageInput{Search: suffix})
		if err != nil {
			t.Fatal(err)
		}
		foundPrice := false
		for _, item := range pricingOnly.Items {
			if item.LevelNo == level.LevelNo {
				foundPrice = item.PercentageBasisPoints != nil && *item.PercentageBasisPoints == 8000 && item.PricingVersion == global.Version
			}
		}
		if !foundPrice {
			t.Fatal("pricing candidate lost authorized percentage")
		}
		s.authorization = managementTestAuth{allow: true}
		defaultInput := PriceEvaluationInput{CustomerNo: customer, CurrencyCode: "CNY", Now: time.Now().UTC(), Lines: []PriceLine{{ReferenceNo: "global", ProductNo: product, ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1, Quantity: 1, PurchaseCycles: 1, PeriodSubtotalMinor: 10000}}}
		defaults, err := s.EvaluatePrices(ctx, defaultInput)
		if err != nil || len(defaults) != 1 || defaults[0].TargetAmountMinor != 8000 || !strings.HasPrefix(defaults[0].PriceNo, "AGP_") {
			t.Fatalf("global 80%%: %+v %v", defaults, err)
		}
		historicalInput := defaultInput
		historicalInput.Lines = append([]PriceLine(nil), defaultInput.Lines...)
		historicalInput.Lines[0].PeriodUnitCode = "year"
		historical, err := s.EvaluatePrices(ctx, historicalInput)
		if err != nil || len(historical) != 1 || historical[0].TargetAmountMinor != 8000 {
			t.Fatalf("historical authoritative term lost default: %+v %v", historical, err)
		}
		resolved, err := s.ResolvePriceRules(ctx, customer, product, defaultInput.Now)
		if err != nil || len(resolved) != 1 || resolved[0].DiscountBasisPoints == nil || *resolved[0].DiscountBasisPoints != 2000 {
			t.Fatalf("API import default: %+v %v", resolved, err)
		}
		if _, err = s.SaveLevelPricing(ctx, level.LevelNo, &charged, 0); !bizerr.Is(err, codeConflict) {
			t.Fatalf("stale default: %v", err)
		}
		if _, err = s.SaveLevel(ctx, LevelInput{LevelNo: level.LevelNo, Name: "Renamed tier", Status: statusActive, ExpectedVersion: level.Version}); err != nil {
			t.Fatal(err)
		}
		afterRename, err := s.GetLevelPricing(ctx, level.LevelNo)
		if err != nil || afterRename.Version != 1 || *afterRename.PercentageBasisPoints != 8000 {
			t.Fatalf("ordinary CRUD changed pricing: %+v %v", afterRename, err)
		}
		other, err := s.SaveLevel(ctx, LevelInput{Name: "Other tier", Status: statusActive})
		if err != nil {
			t.Fatal(err)
		}
		otherPrice := 7500
		if _, err = s.SaveLevelPricing(ctx, other.LevelNo, &otherPrice, 0); err != nil {
			t.Fatal(err)
		}
		defaults, err = s.EvaluatePrices(ctx, defaultInput)
		if err != nil || len(defaults) != 1 || defaults[0].TargetAmountMinor != 8000 {
			t.Fatalf("other tier leaked: %+v %v", defaults, err)
		}
		rate := 3000
		rule, err := s.SavePrice(ctx, PriceInput{LevelNo: level.LevelNo, ProductNo: product, CurrencyCode: "CNY", ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1, AdjustmentType: adjustmentDiscount, DiscountBasisPoints: &rate, Status: statusActive})
		if err != nil {
			t.Fatal(err)
		}
		input := PriceEvaluationInput{CustomerNo: customer, CurrencyCode: "CNY", Now: time.Now().UTC(), Lines: []PriceLine{{ReferenceNo: "line", ProductNo: product, ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1, Quantity: 2, PurchaseCycles: 3, PeriodSubtotalMinor: 60000}}}
		prices, err := s.EvaluatePrices(ctx, input)
		if err != nil || len(prices) != 1 || prices[0].TargetAmountMinor != 42000 {
			t.Fatalf("tier discount: %+v %v", prices, err)
		}
		specific, err := s.SavePrice(ctx, PriceInput{CustomerNo: customer, ProductNo: product, CurrencyCode: "CNY", ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1, AdjustmentType: adjustmentFixed, FixedPriceMinor: "7000", Status: statusActive})
		if err != nil {
			t.Fatal(err)
		}
		prices, err = s.EvaluatePrices(ctx, input)
		if err != nil || len(prices) != 1 || prices[0].TargetAmountMinor != 42000 || prices[0].PriceNo != specific.PriceNo {
			t.Fatalf("customer overrides tier: %+v %v", prices, err)
		}
		customerPage, err := s.ListPrices(ctx, PageInput{ProductNo: product, Target: priceTargetCustomer})
		if err != nil || customerPage.Total != 1 || len(customerPage.Items) != 1 {
			t.Fatalf("customer filtered page: %+v %v", customerPage, err)
		}
		levelPage, err := s.ListPrices(ctx, PageInput{ProductNo: product, Target: priceTargetLevel})
		if err != nil || levelPage.Total != 1 || levelPage.Items[0].PriceNo != rule.PriceNo {
			t.Fatalf("level filtered page: %+v %v", levelPage, err)
		}
		s.authorization = managementTestAuth{grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionPricing): true}}
		if _, err := s.ListPrices(ctx, PageInput{ProductNo: product, Target: priceTargetCustomer}); !bizerr.Is(err, codeDenied) {
			t.Fatalf("customer list must require customer permission: %v", err)
		}
		if _, err := s.ListPrices(ctx, PageInput{ProductNo: product, Target: priceTargetLevel}); err != nil {
			t.Fatalf("pricing-only tier list denied: %v", err)
		}
		if _, err := s.PriceLevelCandidates(ctx, PageInput{}); err != nil {
			t.Fatal(err)
		}
		candidate, err := s.PriceProductCandidates(ctx, PageInput{ProductNo: product})
		if err != nil || candidate.Total != 1 || candidate.Items[0].ProductNo != product {
			t.Fatalf("exact product candidate: %+v %v", candidate, err)
		}
		if _, err := s.DeletePrice(ctx, specific.PriceNo, specific.Version); !bizerr.Is(err, codeDenied) {
			t.Fatalf("customer deletion must require customer permission: %v", err)
		}
		s.authorization = managementTestAuth{allow: true}
		deleted, err := s.DeletePrice(ctx, specific.PriceNo, specific.Version)
		if err != nil || deleted.Status != statusDeleted || deleted.Version != 2 {
			t.Fatalf("delete: %+v %v", deleted, err)
		}
		if _, err = s.SavePrice(ctx, PriceInput{PriceNo: deleted.PriceNo, ExpectedVersion: deleted.Version, CustomerNo: customer, ProductNo: product, CurrencyCode: "CNY", ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1, AdjustmentType: adjustmentFixed, FixedPriceMinor: "5000", Status: statusActive}); !bizerr.Is(err, codeConflict) {
			t.Fatalf("PUT must not restore deleted price: %v", err)
		}
		if _, err = s.DeletePrice(ctx, specific.PriceNo, specific.Version); !bizerr.Is(err, codeConflict) {
			t.Fatalf("stale delete: %v", err)
		}
		customerPage, err = s.ListPrices(ctx, PageInput{ProductNo: product, Target: priceTargetCustomer})
		if err != nil || customerPage.Total != 0 {
			t.Fatalf("deleted price still listed: %+v %v", customerPage, err)
		}
		prices, err = s.EvaluatePrices(ctx, input)
		if err != nil || len(prices) != 1 || prices[0].PriceNo != rule.PriceNo {
			t.Fatalf("deleted customer fallback: %+v %v", prices, err)
		}
		original, err := db.Model("plugin_idc_core_sales_purchase_price").Ctx(ctx).Where("price_no", specific.PriceNo).Where("version", 1).One()
		if err != nil || original["status_code"].String() != statusActive || original["fixed_price_minor"].Int64() != 7000 {
			t.Fatalf("history changed: %v", err)
		}
		recreated, err := s.SavePrice(ctx, PriceInput{CustomerNo: customer, ProductNo: product, CurrencyCode: "CNY", ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1, AdjustmentType: adjustmentFixed, FixedPriceMinor: "5000", Status: statusActive})
		if err != nil || recreated.Version != 3 || recreated.PriceNo != specific.PriceNo {
			t.Fatalf("recreate: %+v %v", recreated, err)
		}
		prices, err = s.EvaluatePrices(ctx, input)
		if err != nil || len(prices) != 1 || prices[0].TargetAmountMinor != 30000 {
			t.Fatalf("custom 50 overrides agent 80: %+v %v", prices, err)
		}
		if _, err = s.SavePrice(ctx, PriceInput{CustomerNo: customer, ProductNo: product, CurrencyCode: "CNY", ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1, AdjustmentType: adjustmentFixed, FixedPriceMinor: "5000", Status: statusActive}); !bizerr.Is(err, codeConflict) {
			t.Fatalf("duplicate recreate: %v", err)
		}
		if _, err = s.DeletePrice(ctx, recreated.PriceNo, recreated.Version); err != nil {
			t.Fatal(err)
		}

		if _, err = s.DeletePrice(ctx, rule.PriceNo, rule.Version); err != nil {
			t.Fatal(err)
		}
		defaults, err = s.EvaluatePrices(ctx, defaultInput)
		if err != nil || len(defaults) != 1 || defaults[0].TargetAmountMinor != 8000 {
			t.Fatalf("delete product restores global: %+v %v", defaults, err)
		}
		for _, value := range []int{-1, 10001} {
			if _, err = s.SaveLevelPricing(ctx, level.LevelNo, &value, global.Version); !bizerr.Is(err, codeInvalid) {
				t.Fatalf("invalid percentage accepted: %v", err)
			}
		}
		for _, value := range []int{0, 10000} {
			global, err = s.SaveLevelPricing(ctx, level.LevelNo, &value, global.Version)
			if err != nil {
				t.Fatal(err)
			}
			defaults, err = s.EvaluatePrices(ctx, defaultInput)
			if err != nil || len(defaults) != 1 || defaults[0].TargetAmountMinor != int64(value) {
				t.Fatalf("percentage boundary: %+v %v", defaults, err)
			}
		}
		global, err = s.SaveLevelPricing(ctx, level.LevelNo, nil, global.Version)
		if err != nil {
			t.Fatal(err)
		}
		defaults, err = s.EvaluatePrices(ctx, defaultInput)
		if err != nil || len(defaults) != 0 {
			t.Fatalf("clear must restore public price: %+v %v", defaults, err)
		}
		resolved, err = s.ResolvePriceRules(ctx, customer, product, defaultInput.Now)
		if err != nil || len(resolved) != 0 {
			t.Fatalf("API retained cleared global: %+v %v", resolved, err)
		}
		global, err = s.SaveLevelPricing(ctx, level.LevelNo, &charged, global.Version)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.SaveAgent(ctx, AgentInput{CustomerNo: customer, LevelNo: level.LevelNo, Status: statusSuspended, ExpectedVersion: agent.Version})
		if err != nil {
			t.Fatal(err)
		}
		prices, err = s.EvaluatePrices(ctx, input)
		if err != nil || len(prices) != 0 {
			t.Fatalf("suspended tier still applied: %+v %v", prices, err)
		}
	})
}
func TestSalesManagementScopeAndPermissions(t *testing.T) {
	s := &serviceImpl{bizCtx: managementTestContext{current: bizctxcap.CurrentContext{UserID: 81, DataScope: 1}}, principal: managementTestPrincipal{}, authorization: managementTestAuth{allow: false}}
	if _, err := s.ListLevels(context.Background(), PageInput{}); !bizerr.Is(err, codeDenied) {
		t.Fatalf("all-data scope granted permission: %v", err)
	}
	s.authorization = managementTestAuth{allow: true}
	s.bizCtx = managementTestContext{current: bizctxcap.CurrentContext{UserID: 81, DataScope: 3}}
	if _, err := s.ListLevels(context.Background(), PageInput{}); !bizerr.Is(err, codeDenied) {
		t.Fatalf("unsupported scope accepted: %v", err)
	}
}

// TestSalesManagementLevelPricingPermissions ensures qualification administration cannot change prices.
func TestSalesManagementLevelPricingPermissions(t *testing.T) {
	s := &serviceImpl{bizCtx: managementTestContext{current: bizctxcap.CurrentContext{UserID: 81, DataScope: 1}}, principal: managementTestPrincipal{}, authorization: managementTestAuth{grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionQualification): true}}}
	if _, err := s.GetLevelPricing(context.Background(), "tier"); !bizerr.Is(err, codeDenied) {
		t.Fatalf("qualification-only pricing read: %v", err)
	}
	value := 8000
	if _, err := s.SaveLevelPricing(context.Background(), "tier", &value, 0); !bizerr.Is(err, codeDenied) {
		t.Fatalf("qualification-only pricing write: %v", err)
	}
}
