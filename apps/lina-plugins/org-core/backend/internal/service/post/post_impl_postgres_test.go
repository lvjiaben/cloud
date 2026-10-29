// This file verifies tenant-scoped bulk deletion and all-or-nothing conflict handling.
package post

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	_ "lina-core/pkg/dbdriver"
	"lina-core/pkg/plugin/capability/tenantcap"
	"lina-plugin-org-core/backend/internal/dao"
)

type postBatchTenant struct {
	tenantcap.Service
	calls int
}

type postBatchFilter struct{ owner *postBatchTenant }

func (s *postBatchTenant) Filter() tenantcap.FilterService { return postBatchFilter{owner: s} }
func (s postBatchFilter) Context(context.Context) tenantcap.TenantFilterContext {
	s.owner.calls++
	return tenantcap.TenantFilterContext{TenantID: 7}
}

func TestDeletePostsUsesOneAssignmentReadPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_ORG_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_ORG_CORE_TEST_DSN")
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
	schema := "org_post_batch_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	old, configErr := gdb.GetConfigGroup("default")
	if configErr != nil {
		old = nil
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	if err = gdb.SetConfigGroup("default", gdb.ConfigGroup{{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)}}); err != nil {
		t.Fatal(err)
	}
	db := dao.Post.DB()
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
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
	if _, err = db.Exec(ctx, `CREATE TABLE plugin_org_core_post(id INT PRIMARY KEY,tenant_id INT,deleted_at TIMESTAMPTZ); CREATE TABLE plugin_org_core_user_post(tenant_id INT,user_id INT,post_id INT); INSERT INTO plugin_org_core_post SELECT n,7,NULL FROM generate_series(1,500) n; INSERT INTO plugin_org_core_post VALUES(501,8,NULL); INSERT INTO plugin_org_core_user_post VALUES(7,1,10),(7,2,20),(8,3,30);`); err != nil {
		t.Fatal(err)
	}
	tenant := &postBatchTenant{}
	svc := &serviceImpl{tenantSvc: tenant}
	if err = svc.Delete(ctx, []int{20, 10, 1}); !bizerr.Is(err, CodePostAssignedDeleteDenied) {
		t.Fatalf("assigned batch accepted: %v", err)
	}
	count, err := db.GetValue(ctx, `SELECT COUNT(*) FROM plugin_org_core_post WHERE deleted_at IS NULL`)
	if err != nil || count.Int() != 501 {
		t.Fatal("denied batch deleted records", err)
	}
	if _, err = db.Exec(ctx, `DELETE FROM plugin_org_core_user_post WHERE tenant_id=7`); err != nil {
		t.Fatal(err)
	}
	ids := make([]int, 500)
	for i := range ids {
		ids[i] = i + 1
	}
	tenant.calls = 0
	if err = svc.Delete(ctx, ids); err != nil {
		t.Fatal(err)
	}
	if tenant.calls > 2 {
		t.Fatalf("batch repeated tenant-scoped queries %d times", tenant.calls)
	}
	count, err = db.GetValue(ctx, `SELECT COUNT(*) FROM plugin_org_core_post WHERE deleted_at IS NOT NULL AND tenant_id=7`)
	if err != nil || count.Int() != 500 {
		t.Fatal("batch incomplete", err)
	}
	count, err = db.GetValue(ctx, `SELECT COUNT(*) FROM plugin_org_core_post WHERE tenant_id=8 AND deleted_at IS NULL`)
	if err != nil || count.Int() != 1 {
		t.Fatal("another tenant changed", err)
	}
	if err = svc.Delete(ctx, []int{0, -1}); !bizerr.Is(err, CodePostValidIDRequired) {
		t.Fatal("invalid batch accepted", err)
	}
}
