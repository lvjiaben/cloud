// This file verifies bounded customer ticket and category keyword searches against PostgreSQL.
package supportdesk

import (
	"fmt"
	"math"
	"testing"
)

// TestTicketNumberedPaginationPostgreSQL verifies that total and rows use the
// same customer/staff scope, including empty pages and bounded invalid inputs.
func TestTicketNumberedPaginationPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	customer := newCustomerSupportDeskTestService(database, "CUS_owner")
	staff := newAuthorizedSupportDeskTestService(database)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_department(tenant_id,department_no,name_zh_cn,name_en_us,assignment_mode,status_code,created_by,updated_by,org_dept_id) VALUES(0,'DEP_page','Page','Page','manual_claim','active',7,7,11)`); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 25; index++ {
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket
			(tenant_id,ticket_no,ticket_type,visibility_code,customer_no,subject,base_stage,priority_code,current_round,assignee_user_id)
			VALUES (0,?,'customer','customer','CUS_owner','Pagination target','new','normal',1,?)`, fmt.Sprintf("TKT_PAGE_%02d", index), 7+index%2); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket
		(tenant_id,ticket_no,ticket_type,visibility_code,customer_no,subject,base_stage,priority_code,current_round)
		VALUES (0,'TKT_OTHER','customer','customer','CUS_other','Pagination target','new','normal',1),
		(0,'TKT_INTERNAL','internal','internal',NULL,'Pagination target','new','normal',1)`); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		page, size, rows int
		first            string
	}{
		{1, 10, 10, "TKT_PAGE_25"}, {2, 10, 10, "TKT_PAGE_15"},
		{3, 10, 5, "TKT_PAGE_05"}, {4, 10, 0, ""},
		{2, 20, 5, "TKT_PAGE_05"}, {1, 100, 25, "TKT_PAGE_25"},
	} {
		page, err := customer.SearchAccount(ctx, SearchInput{Page: test.page, Limit: test.size, Query: "Pagination"})
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 25 || len(page.Tickets) != test.rows || (test.rows > 0 && page.Tickets[0].TicketNo != test.first) {
			t.Fatalf("page %d size %d: got %+v", test.page, test.size, page)
		}
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET department_no='DEP_page' WHERE customer_no='CUS_owner'`); err != nil {
		t.Fatal(err)
	}
	page, err := staff.search(ctx, SearchInput{Page: 1, Limit: 10, Query: "Pagination"}, &actor{UserID: 7, DataScope: 4, Departments: []string{"DEP_page"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 12 || len(page.Tickets) != 10 {
		t.Fatalf("staff count leaked outside assignment: %+v", page)
	}
	for _, row := range page.Tickets {
		if row.AssigneeUserID != 7 {
			t.Fatalf("staff row leaked: %+v", row)
		}
	}
	first, err := customer.SearchAccount(ctx, SearchInput{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	next, err := customer.SearchAccount(ctx, SearchInput{Limit: 10, Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if next.Total != 25 || len(next.Tickets) != 10 || next.Tickets[0].TicketNo != "TKT_PAGE_15" {
		t.Fatalf("cursor count changed: %+v", next)
	}
	for _, input := range []SearchInput{{Page: -1}, {Page: 1, Cursor: first.NextCursor}, {Page: math.MaxInt, Limit: 100}, {Page: 1, Limit: 101}} {
		if _, err = customer.SearchAccount(ctx, input); err == nil {
			t.Fatalf("invalid pagination accepted: %+v", input)
		}
	}
	empty, err := customer.SearchAccount(ctx, SearchInput{Page: 1, Limit: 10, Query: "absent"})
	if err != nil || empty.Total != 0 || len(empty.Tickets) != 0 {
		t.Fatalf("empty page = %+v, %v", empty, err)
	}
}

func TestAccountTicketKeywordSearchEscapesLiteralsPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	customer := newCustomerSupportDeskTestService(database, "CUS_owner")
	for _, fixture := range []struct {
		ticketNo   string
		subject    string
		customerNo string
		ticketType string
		visibility string
	}{
		{"TKT_network", "网络连接异常", "CUS_owner", "customer", "customer"},
		{"TKT_percent", "100% packet loss", "CUS_owner", "customer", "customer"},
		{"TKT_underscore", "disk_read failure", "CUS_owner", "customer", "customer"},
		{"TKT_backslash", `C:\data unavailable`, "CUS_owner", "customer", "customer"},
		{"TKT_decoy", "100X diskXread C:data", "CUS_owner", "customer", "customer"},
		{"TKT_other", "网络连接异常", "CUS_other", "customer", "customer"},
		{"TKT_internal", "网络连接异常", "", "internal", "internal"},
	} {
		_, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,customer_no,subject,base_stage,priority_code,current_round,created_by,updated_by) VALUES (0,?,?,?,NULLIF(?,''),?,'new','normal',1,8,8)`, fixture.ticketNo, fixture.ticketType, fixture.visibility, fixture.customerNo, fixture.subject)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct{ query, ticketNo string }{
		{" 网络 ", "TKT_network"},
		{"100%", "TKT_percent"},
		{"DISK_", "TKT_underscore"},
		{`c:\`, "TKT_backslash"},
		{"TKT_network", "TKT_network"},
		{"连接", ""}, // Existing search matches a title prefix, not an arbitrary substring.
	} {
		t.Run(test.query, func(t *testing.T) {
			page, err := customer.SearchAccount(ctx, SearchInput{Query: test.query, Limit: 2})
			if err != nil {
				t.Fatal(err)
			}
			if test.ticketNo == "" {
				if len(page.Tickets) != 0 {
					t.Fatalf("unexpected non-prefix match: %+v", page.Tickets)
				}
				return
			}
			if len(page.Tickets) != 1 || page.Tickets[0].TicketNo != test.ticketNo || page.NextCursor != "" {
				t.Fatalf("query %q returned %+v, want only %s", test.query, page, test.ticketNo)
			}
		})
	}
}

func TestPublicCategoryKeywordSearchEscapesLiteralsPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	service := &serviceImpl{database: database}
	for _, fixture := range []struct{ categoryNo, nameZhCN, nameEnUS, status string }{
		{"CAT_network", "网络故障", "Network issue", "active"},
		{"CAT_percent", "100%丢包", "100% packet loss", "active"},
		{"CAT_underscore", "磁盘_读取", "disk_read", "active"},
		{"CAT_backslash", `C:\目录`, `C:\directory`, "active"},
		{"CAT_decoy", "100X丢包", "diskXread", "active"},
		{"CAT_disabled", "网络停用", "Network disabled", "disabled"},
	} {
		_, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_issue_category (tenant_id,category_no,name_zh_cn,name_en_us,priority_code,status_code) VALUES (0,?,?,?,'normal',?)`, fixture.categoryNo, fixture.nameZhCN, fixture.nameEnUS, fixture.status)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct{ query, categoryNo string }{
		{"网络", "CAT_network"},
		{"network", "CAT_network"},
		{"100%", "CAT_percent"},
		{"DISK_", "CAT_underscore"},
		{`c:\`, "CAT_backslash"},
		{"CAT_network", "CAT_network"},
	} {
		t.Run(test.query, func(t *testing.T) {
			page, err := service.SearchPublicCategories(ctx, CategorySearchInput{Keyword: test.query, Limit: 2})
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Items) != 1 || page.Items[0].CategoryNo != test.categoryNo || page.NextCursor != "" {
				t.Fatalf("query %q returned %+v, want only %s", test.query, page, test.categoryNo)
			}
		})
	}
}
