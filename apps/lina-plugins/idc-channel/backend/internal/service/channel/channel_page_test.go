// This file verifies numeric pagination and channel isolation against PostgreSQL.
package channel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

func TestChannelPageValidation(t *testing.T) {
	page, err := normalizePage(PageQuery{})
	if err != nil || page.Page != 1 || page.PageSize != 10 {
		t.Fatalf("default page: %+v %v", page, err)
	}
	for _, input := range []PageQuery{{Page: -1}, {Page: 1000001}, {PageSize: -1}, {PageSize: 101}, {Keyword: strings.Repeat("x", 161)}} {
		if _, err := normalizePage(input); err == nil {
			t.Fatalf("invalid query accepted: %+v", input)
		}
	}
}

func TestChannelPagesPostgreSQL(t *testing.T) {
	database := channelPageDatabase(t)
	ctx := context.Background()
	for _, statement := range []string{
		`CREATE TABLE plugin_idc_channel_account(channel_id TEXT PRIMARY KEY,customer_no TEXT,display_name TEXT,status_code TEXT,new_sales_enabled BOOLEAN,created_at TIMESTAMPTZ DEFAULT now(),updated_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE plugin_idc_channel_token(id BIGINT PRIMARY KEY,channel_id TEXT,scopes JSONB DEFAULT '[]',source_cidrs JSONB DEFAULT '[]',expires_at TIMESTAMPTZ,revoked_at TIMESTAMPTZ,created_at TIMESTAMPTZ DEFAULT now())`,
		`CREATE TABLE plugin_idc_channel_publication(id BIGINT PRIMARY KEY,channel_id TEXT,product_version_id BIGINT,currency TEXT,price NUMERIC,price_minor BIGINT,stock INT,status TEXT,public_slug TEXT,configuration_revision TEXT,billing_term_token TEXT,purchase_cycles INT,quantity INT)`,
		`CREATE TABLE plugin_idc_channel_request(id BIGINT PRIMARY KEY,channel_id TEXT,request_id TEXT,operation TEXT,status TEXT,result_code TEXT,created_at TIMESTAMPTZ DEFAULT now(),completed_at TIMESTAMPTZ)`,
		`INSERT INTO plugin_idc_channel_account(channel_id,customer_no,display_name,status_code,new_sales_enabled) SELECT 'channel_'||lpad(i::text,3,'0'),'customer_'||i,'Partner '||i,'approved',true FROM generate_series(1,225) i`,
		`INSERT INTO plugin_idc_channel_token(id,channel_id,scopes) SELECT i,CASE WHEN i<=225 THEN 'channel_001' ELSE 'channel_002' END,'["catalog.read"]' FROM generate_series(1,250) i`,
		`INSERT INTO plugin_idc_channel_publication SELECT i,CASE WHEN i<=225 THEN 'channel_001' ELSE 'channel_002' END,i,'CNY',88,8800,10,'published','product_'||i,'r1','month',1,1 FROM generate_series(1,250) i`,
		`INSERT INTO plugin_idc_channel_request(id,channel_id,request_id,operation,status,result_code) SELECT i,CASE WHEN i<=225 THEN 'channel_001' ELSE 'channel_002' END,'req_'||i,'quote','completed','ok' FROM generate_series(1,250) i`,
	} {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	svc := &service{db: database, bizCtx: applicationContext{scope: 1}}
	indexSQL, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "sql", "005-channel-list-pagination.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = database.Exec(ctx, string(indexSQL)); err != nil {
			t.Fatal(err)
		}
	}
	query := PageQuery{Page: 23, PageSize: 10}
	accounts, err := svc.ListChannels(ctx, query)
	if err != nil || accounts.Total != 225 || len(accounts.List) != 5 || accounts.List[0].ChannelID != "channel_221" {
		t.Fatalf("account last page: %+v %v", accounts, err)
	}
	filtered, err := svc.ListChannels(ctx, PageQuery{Keyword: "Partner 225"})
	if err != nil || filtered.Total != 1 || filtered.List[0].ChannelID != "channel_225" {
		t.Fatalf("candidate scope: %+v %v", filtered, err)
	}
	literal, err := svc.ListChannels(ctx, PageQuery{Keyword: "%"})
	if err != nil || literal.Total != 0 {
		t.Fatalf("literal search: %+v %v", literal, err)
	}
	tokens, err := svc.ListTokenPage(ctx, "channel_001", query)
	if err != nil || tokens.Total != 225 || len(tokens.List) != 5 || tokens.List[0].ID != 5 {
		t.Fatalf("token last page: %+v %v", tokens, err)
	}
	publications, err := svc.ListPublications(ctx, "channel_001", query)
	if err != nil || publications.Total != 225 || len(publications.List) != 5 || publications.List[0].ProductVersionID != 5 {
		t.Fatalf("publication last page: %+v %v", publications, err)
	}
	catalog, err := svc.CatalogPage(ctx, "channel_001", query)
	if err != nil || catalog.Total != 225 || len(catalog.List) != 5 || catalog.List[0].ProductVersionID != 221 {
		t.Fatalf("catalog last page: %+v %v", catalog, err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_channel_publication SET status='draft' WHERE id=225`); err != nil {
		t.Fatal(err)
	}
	published, err := svc.CatalogPage(ctx, "channel_001", query)
	if err != nil || published.Total != 224 || len(published.List) != 4 {
		t.Fatalf("catalog publication scope: %+v %v", published, err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_channel_account SET status_code='paused',new_sales_enabled=false WHERE channel_id='channel_002'`); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.CatalogPage(ctx, "channel_002", PageQuery{}); err == nil {
		t.Fatal("paused channel catalog was exposed")
	}
	audit, err := svc.ListRequestAudit(ctx, "channel_001", query)
	if err != nil || audit.Total != 225 || len(audit.List) != 5 || audit.List[0].ID != 5 {
		t.Fatalf("audit last page: %+v %v", audit, err)
	}
	empty, err := svc.ListTokenPage(ctx, "channel_001", PageQuery{Page: 24, PageSize: 10})
	if err != nil || empty.Total != 225 || len(empty.List) != 0 {
		t.Fatalf("past last page: %+v %v", empty, err)
	}
	other, err := svc.ListTokenPage(ctx, "channel_002", PageQuery{PageSize: 100})
	if err != nil || other.Total != 25 || len(other.List) != 25 {
		t.Fatalf("other channel scope: %+v %v", other, err)
	}
}

func channelPageDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("LINAPRO_IDC_CORE_TEST_DSN is required for PostgreSQL pagination")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "channel_page_test_" + hex.EncodeToString(random[:])
	ctx := context.Background()
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); cleanupErr != nil {
			t.Error(cleanupErr)
		}
		_ = admin.Close(ctx)
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	return database
}
