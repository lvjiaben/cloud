// These rollback-only PostgreSQL tests prove independent monthly performance,
// cumulative refunds, distinct counts, currency isolation and scope rejection.
package salesmanagement

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/usercap"
)

func TestSalesManagementPerformancePostgreSQL(t *testing.T) {
	managementPostgres(t, func(ctx context.Context, db gdb.DB, s *serviceImpl, _ *managementTestAudit) {
		exec := func(query string) {
			t.Helper()
			if _, err := db.Exec(ctx, query); err != nil {
				t.Fatal(err)
			}
		}
		// Temporary projections hide real tables only on this transaction connection;
		// no host or existing plugin record is modified and no commission rule exists.
		exec(`CREATE TEMP TABLE plugin_idc_core_merchant_profile(tenant_id INT,enabled BOOLEAN,timezone TEXT);
 INSERT INTO plugin_idc_core_merchant_profile VALUES(0,true,'Asia/Shanghai');
 CREATE TEMP TABLE plugin_idc_core_customer_subject(tenant_id INT,customer_no TEXT,display_name TEXT);
 INSERT INTO plugin_idc_core_customer_subject VALUES(0,'referrer','Referral test');
 CREATE TEMP TABLE plugin_idc_core_sales_payment_fact(tenant_id INT,invoice_no TEXT,order_no TEXT,currency_code TEXT,operation_type_code TEXT,received_amount_minor BIGINT,received_at TIMESTAMPTZ);
 CREATE TEMP TABLE plugin_idc_core_sales_payment_owner(tenant_id INT,invoice_no TEXT,beneficiary_type_code TEXT,beneficiary_reference TEXT);
 CREATE TEMP TABLE plugin_idc_core_billing_refund_allocation(tenant_id INT,invoice_no TEXT,currency_code TEXT,direction_code TEXT,amount_minor BIGINT,created_at TIMESTAMPTZ);
 INSERT INTO plugin_idc_core_sales_payment_fact VALUES
 (0,'first','order1','CNY','new_purchase',10000,'2026-08-31 16:00:00Z'),
 (0,'extra','order1','CNY','new_purchase',1000,'2026-09-03 00:00:00Z'),
 (0,'free','order2','CNY','new_purchase',0,'2026-09-04 00:00:00Z'),
 (0,'renew',NULL,'CNY','renewal',2000,'2026-09-05 00:00:00Z'),
 (0,'free-renew',NULL,'CNY','renewal',0,'2026-09-06 00:00:00Z'),
 (0,'dollar','order3','USD','new_purchase',700,'2026-09-06 00:00:00Z'),
 (0,'outside','order4','CNY','new_purchase',999999,'2026-08-31 15:59:59Z'),
 (0,'next','order5','CNY','new_purchase',888888,'2026-09-30 16:00:00Z'),
 (1,'foreign','order6','CNY','new_purchase',777777,'2026-09-06 00:00:00Z');
 INSERT INTO plugin_idc_core_sales_payment_owner SELECT tenant_id,invoice_no,'affiliate','referrer' FROM plugin_idc_core_sales_payment_fact;
 INSERT INTO plugin_idc_core_sales_payment_owner VALUES(0,'first','sales','81');
 INSERT INTO plugin_idc_core_billing_refund_allocation VALUES
 (0,'first','CNY','apply',3000,'2026-10-01 00:00:00Z'),
 (0,'first','CNY','reverse',500,'2026-10-02 00:00:00Z'),
 (0,'dollar','USD','apply',100,'2026-10-02 00:00:00Z'),
 (1,'first','CNY','apply',111111,'2026-10-02 00:00:00Z');`)
		input := PerformanceInput{PageInput: PageInput{Page: 1, PageSize: 1, Search: "Referral"}, Month: "2026-09", BeneficiaryType: beneficiaryAffiliate}
		page, err := s.ListPerformance(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 2 || len(page.Items) != 1 || page.Timezone != "Asia/Shanghai" || page.AsOf == 0 {
			t.Fatalf("page/currency totals: %+v", page)
		}
		cny := page.Items[0]
		if cny.CurrencyCode != "CNY" || cny.BeneficiaryName != "Referral test" || cny.PaidOrderCount != 2 || cny.PaidRenewalCount != 2 || cny.ReceivedMinor != "13000" || cny.RefundedMinor != "2500" || cny.NetReceivedMinor != "10500" {
			t.Fatalf("incorrect monthly immutable totals: %+v", cny)
		}
		input.Page = 2
		page, err = s.ListPerformance(ctx, input)
		if err != nil || len(page.Items) != 1 || page.Items[0].CurrencyCode != "USD" || page.Items[0].NetReceivedMinor != "600" {
			t.Fatalf("currency page: %+v %v", page, err)
		}
		input.Page = 1
		input.CurrencyCode = "USD"
		page, err = s.ListPerformance(ctx, input)
		if err != nil || page.Total != 1 || page.Items[0].ReceivedMinor != "700" {
			t.Fatalf("currency filter: %+v %v", page, err)
		}
		input.Month = "2026-13"
		if _, err = s.ListPerformance(ctx, input); !bizerr.Is(err, codeInvalid) {
			t.Fatalf("invalid month: %v", err)
		}
		s.authorization = managementTestAuth{allow: false}
		if _, err = s.ListPerformance(ctx, input); !bizerr.Is(err, codeDenied) {
			t.Fatalf("permission before data: %v", err)
		}
		s.authorization = managementTestAuth{allow: true}
		s.bizCtx = managementTestContext{current: bizctxcap.CurrentContext{UserID: 81, DataScope: 3}}
		if _, err = s.ListPerformance(ctx, input); !bizerr.Is(err, codeDenied) {
			t.Fatalf("unsupported scope: %v", err)
		}
	})
}

func TestSalesManagementPerformanceSearchBoundsPostgreSQL(t *testing.T) {
	managementPostgres(t, func(ctx context.Context, _ gdb.DB, s *serviceImpl, _ *managementTestAudit) {
		users := &managementDirectoryUsers{result: &capmodel.PageResult[*usercap.UserInfo]{Total: 101}}
		s.users = users
		if _, _, err := s.performanceOwnerFilter(ctx, PerformanceInput{PageInput: PageInput{Search: "employee"}}); !bizerr.Is(err, codeStaffSearchBroad) {
			t.Fatalf("broad search must fail: %v", err)
		}
		users.result = &capmodel.PageResult[*usercap.UserInfo]{Total: 1, Items: []*usercap.UserInfo{{ID: "81"}}}
		where, args, err := s.performanceOwnerFilter(ctx, PerformanceInput{PageInput: PageInput{Search: "employee"}})
		if err != nil || len(args) != 4 || where == "" || users.calls != 2 {
			t.Fatalf("bounded directory filter: %s %+v %v", where, args, err)
		}
	})
}
