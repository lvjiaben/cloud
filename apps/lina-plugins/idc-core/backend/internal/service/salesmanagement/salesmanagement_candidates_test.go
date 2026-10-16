// These tests verify governed directory pagination, employee eligibility and
// bounded commission searches using rollback-only local PostgreSQL fixtures.
package salesmanagement

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-plugin-idc-core/backend/internal/dao"
)

type managementDirectoryUsers struct {
	usercap.Service
	result *capmodel.PageResult[*usercap.UserInfo]
	input  usercap.ListInput
	calls  int
}

func (s *managementDirectoryUsers) List(_ context.Context, input usercap.ListInput) (*capmodel.PageResult[*usercap.UserInfo], error) {
	s.calls++
	s.input = input
	return s.result, nil
}

func TestSalesManagementStaffCandidatesPostgreSQL(t *testing.T) {
	managementPostgres(t, func(ctx context.Context, db gdb.DB, s *serviceImpl, _ *managementTestAudit) {
		if _, err := db.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_principal_audience(tenant_id INT,sys_user_id INT,audience_code TEXT,status_code TEXT); INSERT INTO plugin_idc_core_principal_audience VALUES(0,901,'staff','active'),(0,902,'customer','active'),(0,903,'staff','revoked')`); err != nil {
			t.Fatal(err)
		}
		users := &managementDirectoryUsers{result: &capmodel.PageResult[*usercap.UserInfo]{Total: 101, Items: []*usercap.UserInfo{{ID: "901", Nickname: "Sales"}, {ID: "902", Username: "Customer"}, {ID: "903", Username: "Former staff"}}}}
		s.users = users
		page, err := s.StaffCandidates(ctx, PageInput{Page: 2, PageSize: 3, Search: "name"})
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 101 || len(page.Items) != 3 || !page.Items[0].Selectable || page.Items[1].Selectable || page.Items[2].Selectable {
			t.Fatalf("incorrect eligibility/page: %+v", page)
		}
		if users.calls != 1 || users.input.Page.PageNum != 2 || users.input.Page.PageSize != 3 || !users.input.EnabledOnly || users.input.Keyword != "name" {
			t.Fatalf("directory pagination changed: %+v", users)
		}
		s.authorization = managementTestAuth{allow: false}
		if _, err = s.StaffCandidates(ctx, PageInput{}); !bizerr.Is(err, codeDenied) || users.calls != 1 {
			t.Fatalf("permission did not precede directory: %v", err)
		}
	})
}

func TestSalesManagementCommissionSearchPostgreSQL(t *testing.T) {
	managementPostgres(t, func(ctx context.Context, db gdb.DB, s *serviceImpl, _ *managementTestAudit) {
		if _, err := db.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_sales_commission_account(id INT,tenant_id INT,account_no TEXT,beneficiary_type_code TEXT,beneficiary_reference TEXT,currency_code TEXT); CREATE TEMP TABLE plugin_idc_core_customer_subject(tenant_id INT,customer_no TEXT,display_name TEXT); INSERT INTO plugin_idc_core_sales_commission_account VALUES(1,0,'sales-match','sales','901','CNY'),(2,0,'collision','affiliate','901','CNY'); INSERT INTO plugin_idc_core_customer_subject VALUES(0,'901','different')`); err != nil {
			t.Fatal(err)
		}
		users := &managementDirectoryUsers{result: &capmodel.PageResult[*usercap.UserInfo]{Total: 1, Items: []*usercap.UserInfo{{ID: "901"}}}}
		s.users = users
		query, _, total, err := s.commissionPage(ctx, PageInput{Search: "employee"}, dao.SalesCommissionAccount.Table(), "beneficiary_reference")
		if err != nil {
			t.Fatal(err)
		}
		rows, err := query.Fields("account_no").All()
		if err != nil || total != 1 || len(rows) != 1 || rows[0]["account_no"].String() != "sales-match" {
			t.Fatalf("identity collision matched: %v %v %v", total, rows, err)
		}
		users.result.Total = 101
		query, _, total, err = s.commissionPage(ctx, PageInput{Search: "employee"}, dao.SalesCommissionAccount.Table(), "beneficiary_reference")
		if !bizerr.Is(err, codeStaffSearchBroad) || query != nil || total != 0 {
			t.Fatalf("broad search silently truncated: %v %v %v", query, total, err)
		}
	})
}
