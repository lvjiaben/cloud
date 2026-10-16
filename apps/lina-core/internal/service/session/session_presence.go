// This file composes session presence as a governed correlated query. It uses
// the shared SQL projection, excludes expired sessions before aggregation, and
// never materializes an unbounded set of user or token identifiers.

package session

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/internal/dao"
	"lina-core/internal/model/do"
	"lina-core/internal/service/datascope"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/tenantcap/tenantspi"
	"lina-core/pkg/statusflag"
)

// presenceColumn accepts a single alias-qualified column, never expressions.
var presenceColumn = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\.[A-Za-z_][A-Za-z0-9_]*$`)

// BuildOnlineUserExists composes host-owned online presence with the current
// tenant and role scope. The timeout must come from the shared auth owner;
// callers may only embed the returned model as an EXISTS predicate.
func BuildOnlineUserExists(
	ctx context.Context,
	userIDColumn string,
	timeout time.Duration,
	scopeSvc datascope.Service,
	tenantSvc tenantspi.ScopeService,
) (*gdb.Model, bool, error) {
	qualifier, _, _ := strings.Cut(userIDColumn, ".")
	if !presenceColumn.MatchString(userIDColumn) || strings.EqualFold(qualifier, "session_presence") || strings.EqualFold(qualifier, "sys_user") {
		return nil, false, bizerr.NewCode(capmodel.CodeCapabilityDenied)
	}
	if scopeSvc == nil || tenantSvc == nil || timeout <= 0 {
		return nil, false, bizerr.NewCode(capmodel.CodeCapabilityUnavailable, bizerr.P("capability", "session.scope"))
	}
	var (
		cols       = dao.SysOnlineSession.Columns()
		userColumn = "session_presence." + cols.UserId
		model      = dao.SysOnlineSession.Ctx(ctx).As("session_presence").Fields(gdb.Raw("1")).
				Where(userColumn, gdb.Raw(userIDColumn)).
				WhereGT("session_presence."+cols.LastActiveTime, time.Now().Add(-timeout))
	)
	model, err := tenantSvc.Apply(ctx, model, "session_presence."+cols.TenantId)
	if err != nil {
		return nil, false, err
	}
	model, empty, err := scopeSvc.ApplyUserScope(ctx, model, userColumn)
	if err != nil || empty {
		return model, empty, err
	}
	// User lookup stays host-owned and inherits GoFrame's soft-delete filter.
	users := dao.SysUser.Ctx(ctx).Fields(gdb.Raw("1")).
		Where(do.SysUser{Status: statusflag.EnabledValue}).
		Where(dao.SysUser.Columns().Id, gdb.Raw(userColumn))
	users, empty, err = tenantSvc.ApplyUserTenantScope(ctx, users, dao.SysUser.Table()+"."+dao.SysUser.Columns().Id)
	if err != nil || empty {
		return nil, empty, err
	}
	return model.Where("EXISTS ?", users), false, nil
}
