// This file verifies online presence expiry, de-duplication, visibility, and
// identifier safety against the same SQL projection used in production.

package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/internal/dao"
	"lina-core/internal/model/do"
	"lina-core/internal/service/datascope"
	"lina-core/pkg/plugin/capability/tenantcap/tenantspi"
	"lina-core/pkg/statusflag"
)

// TestBuildOnlineUserExistsRejectsUnsafeInputs prevents correlation expressions
// and absent governance from widening a plugin aggregate.
func TestBuildOnlineUserExistsRejectsUnsafeInputs(t *testing.T) {
	for _, column := range []string{"id", "a.id OR 1=1", "a.id; DROP TABLE x", "session_presence.user_id", "SESSION_PRESENCE.user_id", "sys_user.id", "SYS_USER.id", "a.\"id\""} {
		if _, _, err := BuildOnlineUserExists(context.Background(), column, time.Hour, presenceScope{}, presenceTenant{}); err == nil {
			t.Fatalf("accepted unsafe column %q", column)
		}
	}
	if _, _, err := BuildOnlineUserExists(context.Background(), "a.id", time.Hour, nil, presenceTenant{}); err == nil {
		t.Fatal("accepted missing role scope")
	}
	if _, _, err := BuildOnlineUserExists(context.Background(), "a.id", 0, presenceScope{}, presenceTenant{}); err == nil {
		t.Fatal("accepted invalid expiry policy")
	}
}

// TestBuildOnlineUserExistsFiltersBeforeCountAndPage proves two sessions count
// once, expired/foreign-tenant/disabled users do not count, and detail uses the
// same predicate as the aggregate.
func TestBuildOnlineUserExistsFiltersBeforeCountAndPage(t *testing.T) {
	var (
		ctx    = context.Background()
		prefix = fmt.Sprintf("presence-%d", time.Now().UnixNano())
		ids    = make([]int, 0, 4)
	)
	t.Cleanup(func() {
		if len(ids) == 0 {
			return
		}
		if _, err := dao.SysOnlineSession.Ctx(ctx).WhereIn(dao.SysOnlineSession.Columns().UserId, ids).Delete(); err != nil {
			t.Errorf("cleanup sessions: %v", err)
		}
		if _, err := dao.SysUser.Ctx(ctx).Unscoped().WhereIn(dao.SysUser.Columns().Id, ids).Delete(); err != nil {
			t.Errorf("cleanup users: %v", err)
		}
	})
	for i := range 4 {
		status := statusflag.EnabledValue
		if i == 3 {
			status = statusflag.Disabled
		}
		id, err := dao.SysUser.Ctx(ctx).Data(do.SysUser{Username: fmt.Sprintf("%s-%d", prefix, i), Password: "not-a-valid-password-hash", Status: status}).InsertAndGetId()
		if err != nil {
			t.Fatalf("create test user: %v", err)
		}
		ids = append(ids, int(id))
	}
	for i, userID := range []int{ids[0], ids[0], ids[1], ids[2], ids[3]} {
		activeAt := time.Now()
		tenantID := 0
		if i == 2 {
			activeAt = activeAt.Add(-2 * time.Hour)
		}
		if i == 3 {
			tenantID = 22
		}
		if _, err := dao.SysOnlineSession.Ctx(ctx).Data(do.SysOnlineSession{TokenId: fmt.Sprintf("%s-%d", prefix, i), UserId: userID, TenantId: tenantID, ClientType: "web", LastActiveTime: &activeAt}).Insert(); err != nil {
			t.Fatalf("create test session: %v", err)
		}
	}
	subquery, empty, err := BuildOnlineUserExists(ctx, "candidate.id", time.Hour, presenceScope{}, presenceTenant{})
	if err != nil || empty {
		t.Fatalf("build presence query: empty=%v err=%v", empty, err)
	}
	model := dao.SysUser.Ctx(ctx).As("candidate").WhereIn("candidate.id", ids).Where("EXISTS ?", subquery)
	count, err := model.Clone().Count()
	if err != nil || count != 1 {
		t.Fatalf("expected one distinct active user, count=%d err=%v", count, err)
	}
	rows, err := model.Clone().Fields("candidate.id").OrderAsc("candidate.id").Page(1, 1).All()
	if err != nil || len(rows) != 1 || rows[0]["id"].Int() != ids[0] {
		t.Fatalf("count and page disagree: rows=%v err=%v", rows, err)
	}
	subquery, empty, err = BuildOnlineUserExists(ctx, "candidate.id", time.Hour, presenceScope{userID: ids[1]}, presenceTenant{})
	if err != nil || empty {
		t.Fatalf("build self presence query: empty=%v err=%v", empty, err)
	}
	count, err = dao.SysUser.Ctx(ctx).As("candidate").WhereIn("candidate.id", ids).Where("EXISTS ?", subquery).Count()
	if err != nil || count != 0 {
		t.Fatalf("self scope leaked another user: count=%d err=%v", count, err)
	}
	if _, empty, err = BuildOnlineUserExists(ctx, "candidate.id", time.Hour, presenceScope{empty: true}, presenceTenant{}); err != nil || !empty {
		t.Fatalf("expected denied scope to be empty: empty=%v err=%v", empty, err)
	}
}

// presenceScope applies the test role visibility without replacing SQL reads.
type presenceScope struct {
	datascope.Service
	userID int
	empty  bool
}

// ApplyUserScope constrains the correlated session owner or denies all rows.
func (s presenceScope) ApplyUserScope(_ context.Context, model *gdb.Model, column string) (*gdb.Model, bool, error) {
	if s.userID > 0 {
		model = model.Where(column, s.userID)
	}
	return model, s.empty, nil
}

// presenceTenant provides a platform-only test tenant boundary.
type presenceTenant struct{ tenantspi.ScopeService }

// Apply injects the tenant constraint before count or pagination.
func (presenceTenant) Apply(_ context.Context, model *gdb.Model, column string) (*gdb.Model, error) {
	return model.Where(column, 0), nil
}

// ApplyUserTenantScope leaves platform test users visible after session scope.
func (presenceTenant) ApplyUserTenantScope(_ context.Context, model *gdb.Model, _ string) (*gdb.Model, bool, error) {
	return model, false, nil
}
