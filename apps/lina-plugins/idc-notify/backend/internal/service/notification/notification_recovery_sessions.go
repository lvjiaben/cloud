// This file captures exact pre-reset sessions, denies them with one indexed
// plugin lookup, and retries host revocation without touching newer sessions.
package notification

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-notify/backend/internal/model/do"
	"lina-plugin-idc-notify/backend/internal/model/entity"
	"strconv"
)

const revokedSessionTable = "plugin_idc_notify_revoked_login_session"

func sessionDigest(tokenID string) string { return shaText("revoked-login-session-v1:" + tokenID) }

// CheckRecoverySession denies captured old sessions immediately, including while
// host revocation is unavailable. It trusts only the real authenticated context.
// There is one indexed plugin query, no host lookup, cache or per-row work.
func (s *serviceImpl) CheckRecoverySession(ctx context.Context) error {
	current := s.bizctx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != notificationTenantID || current.TokenID == "" {
		return nil
	}
	if token, ok := ctx.Value(recoveryWriteKey{}).(string); ok && token == current.TokenID {
		return nil
	}
	count, err := s.database.Model(revokedSessionTable).Ctx(ctx).Where(do.RevokedLoginSession{TenantId: notificationTenantID, UserId: current.UserID, SessionDigest: sessionDigest(current.TokenID)}).Count()
	if err != nil {
		return publicError(err)
	}
	if count > 0 {
		return bizerr.NewCode(CodePasswordChanged)
	}
	return nil
}

// captureRecoverySessions uses bounded public pages with the current customer's
// actual visibility. Exact IDs resolve timestamp ties without blocking new logins.
func (s *serviceImpl) captureRecoverySessions(ctx context.Context, recoveryNo string) error {
	current := s.bizctx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != notificationTenantID || current.TokenID == "" {
		return bizerr.NewCode(CodeNotAccessible)
	}
	rows := make([]do.RevokedLoginSession, 0, 8)
	for pageNum := 1; pageNum <= 10; pageNum++ {
		page, err := s.sessions.List(ctx, sessioncap.ListInput{Page: capmodel.PageRequest{PageNum: pageNum, PageSize: 100}})
		if err != nil {
			logger.Warningf(ctx, "read own recovery session page failed: %v", err)
			return publicError(err)
		}
		if page == nil || page.Total > 1000 {
			return bizerr.NewCode(CodeUnavailable)
		}
		for _, item := range page.Items {
			if item != nil && item.UserID == strconv.Itoa(current.UserID) && string(item.TenantID) == "0" {
				rows = append(rows, do.RevokedLoginSession{TenantId: notificationTenantID, UserId: current.UserID, SessionDigest: sessionDigest(string(item.ID)), SessionId: string(item.ID), RecoveryNo: recoveryNo})
			}
		}
		if pageNum*100 >= page.Total {
			break
		}
	}
	if len(rows) == 0 {
		logger.Warning(ctx, "recovery session projection did not include the authenticated customer")
		return bizerr.NewCode(CodeUnavailable)
	}
	_, err := s.database.Model(revokedSessionTable).Ctx(ctx).Data(rows).InsertIgnore()
	return publicError(err)
}

// CleanupRecoverySessions revokes only the captured sessions of this completed
// reset. Missing host sessions are already invalid and can be marked complete.
func (s *serviceImpl) cleanupRecoverySessions(ctx context.Context, recoveryNo string) (bool, error) {
	current := s.bizctx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != notificationTenantID || current.IsImpersonation || current.ActingUserID != 0 {
		return true, bizerr.NewCode(CodeNotAccessible)
	}
	var guard entity.LoginContactGuard
	if err := s.database.Model(contactGuardTable).Ctx(ctx).Where(do.LoginContactGuard{TenantId: notificationTenantID, UserId: current.UserID}).Scan(&guard); err != nil {
		return true, publicError(err)
	}
	if recoveryNo != "" && guard.RecoveryNo != recoveryNo {
		return true, bizerr.NewCode(CodeVerificationInvalid)
	}
	if !guard.CleanupPending {
		return false, nil
	}
	for range 10 {
		var rows []entity.RevokedLoginSession
		if err := s.database.Model(revokedSessionTable).Ctx(ctx).Where(do.RevokedLoginSession{TenantId: notificationTenantID, UserId: current.UserID, RecoveryNo: guard.RecoveryNo, CleanupDone: false}).OrderAsc("session_digest").Limit(100).Scan(&rows); err != nil {
			return true, publicError(err)
		}
		if len(rows) == 0 {
			result, err := s.database.Model(contactGuardTable).Ctx(ctx).Where(do.LoginContactGuard{TenantId: notificationTenantID, UserId: current.UserID, RecoveryNo: guard.RecoveryNo}).Data(do.LoginContactGuard{CleanupPending: false}).Update()
			if err != nil {
				return true, publicError(err)
			}
			if err = requireOne(result); err != nil {
				return true, err
			}
			return false, nil
		}
		ids := make([]sessioncap.SessionID, 0, len(rows))
		digests := make([]string, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, sessioncap.SessionID(row.SessionId))
			digests = append(digests, row.SessionDigest)
		}
		visible, err := s.sessions.BatchGet(ctx, ids)
		if err != nil {
			return true, publicError(err)
		}
		live := make([]sessioncap.SessionID, 0, len(visible.Items))
		for id, item := range visible.Items {
			if item == nil || item.UserID != strconv.Itoa(current.UserID) || string(item.TenantID) != "0" {
				return true, bizerr.NewCode(CodeNotAccessible)
			}
			live = append(live, id)
		}
		if err = s.sessions.RevokeMany(ctx, live); err != nil {
			return true, publicError(err)
		}
		if _, err = s.database.Model(revokedSessionTable).Ctx(ctx).Where(do.RevokedLoginSession{TenantId: notificationTenantID, UserId: current.UserID, RecoveryNo: guard.RecoveryNo}).WhereIn("session_digest", digests).Data(do.RevokedLoginSession{CleanupDone: true}).Update(); err != nil {
			return true, publicError(err)
		}
	}
	return true, nil
}

// CleanupRecoverySessions serializes retries with each other and with newer
// resets. An older cleanup grant cannot consume a newer reset's pending work.
func (s *serviceImpl) CleanupRecoverySessions(ctx context.Context, recoveryNo string) (bool, error) {
	pending := true
	current := s.bizctx.Current(ctx)
	err := s.lockContact(ctx, current.UserID, func(tx context.Context, _ int64) error {
		var err error
		pending, err = s.cleanupRecoverySessions(tx, recoveryNo)
		return err
	})
	return pending, err
}
