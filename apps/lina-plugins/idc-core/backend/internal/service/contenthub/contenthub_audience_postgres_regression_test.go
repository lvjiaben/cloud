// Search regressions verify PostgreSQL prefix matching without reviving retired audience kinds.

package contenthub

import (
	"fmt"
	"testing"
)

func TestCustomerGroupSearchEscapesLiteralPrefixesPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	for index, name := range []string{"100% group", "100X group", "中文_分组", "中文X分组", `path\group`} {
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_customer_group (tenant_id,group_no,name_zh_cn,name_en_us,status_code) VALUES (0,?,?,?,'active')`, fmt.Sprintf("GRP_LITERAL_%d", index), name, name); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct{ query, number string }{
		{"100%", "GRP_LITERAL_0"}, {"中文_", "GRP_LITERAL_2"}, {`path\`, "GRP_LITERAL_4"}, {"GRP_LITERAL_1", "GRP_LITERAL_1"},
	} {
		t.Run(test.query, func(t *testing.T) {
			page, err := service.SearchAudienceCustomerGroups(ctx, AudienceSearchInput{Keyword: test.query, Limit: 10})
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Items) != 1 || page.Items[0].Number != test.number {
				t.Fatalf("literal %q: %+v", test.query, page.Items)
			}
		})
	}
	page, err := service.SearchAudienceCustomerGroups(ctx, AudienceSearchInput{Keyword: "absent", Limit: 10})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("absent query: %+v %v", page, err)
	}
}

func TestContentSearchEscapesLiteralPrefixesPostgreSQL(t *testing.T) {
	ctx, database := newContentHubTestDatabase(t)
	service := newAuthorizedContentHubTestService(database)
	for index, title := range []string{"100% published", "100X published", "中文_内容", "中文X内容", `path\content`} {
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_content_item (tenant_id,content_no,content_type,visibility_code,title,body,public_path,status_code,created_by,updated_by) VALUES (0,?,'news','public',?,'Body',?,'published',7,7)`, fmt.Sprintf("CNT_LITERAL_%d", index), title, fmt.Sprintf("/news/literal-%d", index)); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct{ query, number string }{
		{"100%", "CNT_LITERAL_0"}, {"中文_", "CNT_LITERAL_2"}, {`path\`, "CNT_LITERAL_4"},
	} {
		t.Run(test.query, func(t *testing.T) {
			public, err := service.SearchPublic(ctx, SearchInput{Query: test.query, Limit: 10})
			if err != nil || len(public.Items) != 1 || public.Items[0].ContentNo != test.number {
				t.Fatalf("public literal %q: %+v %v", test.query, public, err)
			}
			admin, err := service.SearchAdmin(ctx, SearchInput{Query: test.query, Page: 1, Limit: 10})
			if err != nil || admin.Total != 1 || len(admin.Items) != 1 || admin.Items[0].ContentNo != test.number {
				t.Fatalf("admin literal %q: %+v %v", test.query, admin, err)
			}
		})
	}
}
