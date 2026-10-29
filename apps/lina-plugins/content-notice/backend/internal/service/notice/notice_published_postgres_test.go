// This file verifies announcement migrations, publication filtering, sort persistence and isolated read state in PostgreSQL.

package notice

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
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/tenantcap"
	"lina-plugin-content-notice/backend/internal/dao"
)

// TestPublishedAnnouncementsPostgreSQL exercises the real owner data source and persistent read markers.
func TestPublishedAnnouncementsPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_CONTENT_NOTICE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_CONTENT_NOTICE_TEST_DSN for PostgreSQL integration")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "announcement_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	old, oldErr := gdb.GetConfigGroup("default")
	if oldErr != nil {
		old = nil
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	if err = gdb.SetConfigGroup("default", gdb.ConfigGroup{{Link: dsn + separator + "search_path=" + schema}}); err != nil {
		t.Fatal(err)
	}
	database := dao.Notice.DB()
	t.Cleanup(func() {
		if err := database.Close(ctx); err != nil {
			t.Error(err)
		}
		if err := gdb.SetConfigGroup("default", old); err != nil {
			t.Error(err)
		}
		if _, err := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Error(err)
		}
		if err := admin.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	if _, err = database.Exec(ctx, `CREATE TABLE sys_dict_type(name TEXT,type TEXT UNIQUE,status INT,is_builtin INT,remark TEXT,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ); CREATE TABLE sys_dict_data(dict_type TEXT,label TEXT,value TEXT,sort INT,tag_style TEXT,status INT,is_builtin INT,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ,UNIQUE(dict_type,value));`); err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		for _, file := range []string{"001-content-notice-schema.sql", "002-announcement-sort.sql", "003-announcement-cleanup.sql"} {
			content, readErr := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "sql", file))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if _, err = database.Exec(ctx, string(content)); err != nil {
				t.Fatalf("migration pass %d %s: %v", pass, file, err)
			}
			if pass == 0 && file == "001-content-notice-schema.sql" {
				// Simulate an existing installation with obsolete announcement columns and its old second source.
				if _, err = database.Exec(ctx, `ALTER TABLE plugin_content_notice ADD COLUMN type INT DEFAULT 2, ADD COLUMN file_ids TEXT DEFAULT '', ADD COLUMN remark TEXT DEFAULT ''; CREATE TABLE plugin_idc_core_content_customer_scope(content_no TEXT); CREATE TABLE plugin_idc_core_content_service_scope(content_no TEXT); CREATE TABLE plugin_idc_core_content_item(tenant_id INT,title TEXT,body TEXT,status_code TEXT,created_by BIGINT,updated_by BIGINT,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ,deleted_at TIMESTAMPTZ,content_type TEXT,content_no TEXT,visibility_code TEXT); INSERT INTO plugin_idc_core_content_item VALUES(0,'Migrated','<p>Old content</p>','published',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,NULL,'announcement','old_announcement','specific_customers');`); err != nil {
					t.Fatal(err)
				}
			}
			if pass == 0 && file == "003-announcement-cleanup.sql" {
				if _, err = database.Exec(ctx, `INSERT INTO plugin_content_notice_read(notice_id,user_id) SELECT id,999 FROM plugin_content_notice WHERE title='Migrated'`); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	migratedCount, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_content_notice WHERE title='Migrated'`)
	if err != nil || migratedCount.Int() != 1 {
		t.Fatalf("legacy migration duplicated/lost content: %v %v", migratedCount, err)
	}
	oldCount, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_content_item WHERE content_type='announcement'`)
	if err != nil || oldCount.Int() != 0 {
		t.Fatalf("second announcement source remained: %v %v", oldCount, err)
	}
	retainedReads, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_content_notice_read WHERE user_id=999`)
	if err != nil || retainedReads.Int() != 1 {
		t.Fatalf("migration changed read state: %v %v", retainedReads, err)
	}
	if _, err = database.Exec(ctx, `DELETE FROM plugin_content_notice WHERE title='Migrated'`); err != nil {
		t.Fatal(err)
	}
	count, err := database.GetValue(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=? AND table_name='plugin_content_notice' AND column_name IN ('type','file_ids','remark')`, schema)
	if err != nil || count.Int() != 0 {
		t.Fatalf("obsolete columns survived: %v %v", count, err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_content_notice(title,content,status,sort_order) VALUES ('Later','<p>Later</p>',1,9),('First','<p>First<img src="/photo.png" onerror="bad()"></p>',1,1),('Draft','private draft',0,0),('First','same title is allowed',1,20);`); err != nil {
		t.Fatal(err)
	}
	reader := func(userID int64) *serviceImpl {
		return &serviceImpl{bizCtxSvc: fakeNoticeBizCtxService{current: bizctxcap.CurrentContext{UserID: int(userID)}}, tenantSvc: fakeNoticeTenantService{filter: fakeNoticeTenantFilter{tenantCtx: tenantcap.TenantFilterContext{}}}}
	}
	userA, userB := reader(101), reader(202)
	page, err := userA.Published(ctx, 1, 2, true)
	if err != nil || page.Total != 3 || len(page.List) != 2 || page.List[0].Title != "First" || page.List[1].Title != "Later" {
		t.Fatalf("wrong published page: %+v %v", page, err)
	}
	firstID := page.List[0].Id
	if strings.Contains(page.List[0].Content, "onerror") || !strings.Contains(page.List[0].Content, "/photo.png") {
		t.Fatal("published HTML was not safely projected")
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err = userA.MarkRead(ctx, firstID); err != nil {
			t.Fatal(err)
		}
	}
	item, err := userA.PublishedByID(ctx, firstID, true)
	if err != nil || !item.Read {
		t.Fatalf("read state was not persisted: %+v %v", item, err)
	}
	item, err = userB.PublishedByID(ctx, firstID, true)
	if err != nil || item.Read {
		t.Fatalf("read state leaked between users: %+v %v", item, err)
	}
	public, err := userA.Published(ctx, 1, 10, false)
	if err != nil || public.Total != 3 || public.List[0].Read {
		t.Fatalf("public projection leaked read state: %+v %v", public, err)
	}
	readCount, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_content_notice_read WHERE user_id=101 AND notice_id=?`, firstID)
	if err != nil || readCount.Int() != 1 {
		t.Fatalf("repeat read created duplicate state: %v %v", readCount, err)
	}
	draftValue, err := database.GetValue(ctx, `SELECT id FROM plugin_content_notice WHERE title='Draft'`)
	if err != nil {
		t.Fatal(err)
	}
	draftID := draftValue.Int64()
	if _, err = userA.PublishedByID(ctx, draftID, true); !bizerr.Is(err, CodeNoticeNotFound) {
		t.Fatalf("draft content exposed: %v", err)
	}
	if err = userA.MarkRead(ctx, draftID); !bizerr.Is(err, CodeNoticeNotFound) {
		t.Fatalf("draft accepted as read: %v", err)
	}
	if _, err = reader(0).Published(ctx, 1, 10, true); !bizerr.Is(err, CodeLoginRequired) {
		t.Fatalf("anonymous reader accepted: %v", err)
	}
	sortOrder := 30
	if err = userA.Update(ctx, UpdateInput{Id: firstID, SortOrder: &sortOrder}); err != nil {
		t.Fatal(err)
	}
	reordered, err := userB.Published(ctx, 1, 10, true)
	if err != nil || reordered.List[len(reordered.List)-1].Id != firstID {
		t.Fatalf("saved sort did not reorder list: %+v %v", reordered, err)
	}
	if err = userA.Delete(ctx, []int64{firstID}); err != nil {
		t.Fatal(err)
	}
	if item, err = userA.PublishedByID(ctx, firstID, true); !bizerr.Is(err, CodeNoticeNotFound) {
		t.Fatalf("deleted announcement exposed: %v", err)
	}
	obsoleteScopes, err := database.GetValue(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=? AND table_name IN ('plugin_idc_core_content_customer_scope','plugin_idc_core_content_service_scope')`, schema)
	if err != nil || obsoleteScopes.Int() != 0 {
		t.Fatalf("obsolete audience tables survived: %v %v", obsoleteScopes, err)
	}
	if _, err = database.Exec(ctx, `CREATE TABLE sys_user(id BIGINT,username TEXT); INSERT INTO sys_user VALUES(1,'admin')`); err != nil {
		t.Fatal(err)
	}
	mock, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "manifest", "sql", "mock-data", "001-content-notice-mock-data.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		if _, err = database.Exec(ctx, string(mock)); err != nil {
			t.Fatal(err)
		}
	}
	mockCount, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_content_notice WHERE created_by=1`)
	if err != nil || mockCount.Int() != 4 {
		t.Fatalf("mock replay duplicated announcements: %v %v", mockCount, err)
	}

}
