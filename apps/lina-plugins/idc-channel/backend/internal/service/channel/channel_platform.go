// This file isolates customer platform reads and atomic policy-preserving rotations.
// Secrets are hashed, and both staff creation and customer rotation lock the account.
package channel

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"time"
)

func (s *service) ownAccount(ctx context.Context) (*channelAccountRow, error) {
	if s.bizCtx == nil {
		return nil, bizerr.NewCode(codeTokenNotAccepted)
	}
	actor := s.bizCtx.Current(ctx)
	if actor.UserID <= 0 || actor.TenantID != 0 || actor.IsImpersonation || actor.ActingUserID != 0 || actor.ActingAsTenant {
		return nil, bizerr.NewCode(codeTokenNotAccepted)
	}
	current, err := s.core.CurrentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	if current == nil || current.Status != "active" || current.ActorUserID != actor.UserID {
		return nil, bizerr.NewCode(codeTokenNotAccepted)
	}
	var row *channelAccountRow
	err = s.db.Model("plugin_idc_channel_account").Ctx(ctx).Fields("channel_id,customer_no,display_name,status_code,new_sales_enabled").Where("customer_no=?", current.CustomerNo).Scan(&row)
	return row, err
}

// OwnPlatform returns an empty authorization to unbound customers and retains disabled history.
func (s *service) OwnPlatform(ctx context.Context) (PlatformOverview, error) {
	account, err := s.ownAccount(ctx)
	if err != nil {
		return PlatformOverview{}, err
	}
	if account == nil {
		return PlatformOverview{Tokens: []TokenSummary{}}, nil
	}
	tokens, err := scanTokenSummaries(s.db.Model("plugin_idc_channel_token").Ctx(ctx).Where("channel_id=?", account.ChannelID).OrderDesc("id").Limit(1))
	return PlatformOverview{ResetEnabled: account.NewSalesEnable, Authorized: true, AppID: account.ChannelID, DisplayName: account.DisplayName, Status: account.StatusCode, Tokens: tokens}, err
}

// RotateOwnSecret uses the displayed token version to make concurrent resets fail closed.
func (s *service) RotateOwnSecret(ctx context.Context, expectedID int64) (TokenResult, error) {
	account, err := s.ownAccount(ctx)
	if err != nil {
		return TokenResult{}, err
	}
	if account == nil || expectedID <= 0 {
		return TokenResult{}, bizerr.NewCode(codeTokenNotAccepted)
	}
	return s.rotateAccountSecret(ctx, account, expectedID)
}

// rotateAccountSecret shares policy-preserving rotation between staff and customer entry points.
func (s *service) rotateAccountSecret(ctx context.Context, account *channelAccountRow, expectedID int64) (TokenResult, error) {
	var result TokenResult
	err := s.db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var locked *channelAccountRow
		if err := s.db.GetScan(ctx, &locked, `SELECT channel_id,status_code,new_sales_enabled FROM plugin_idc_channel_account WHERE channel_id=? FOR UPDATE`, account.ChannelID); err != nil {
			return err
		}
		if locked == nil || locked.StatusCode != "approved" || !locked.NewSalesEnable {
			return bizerr.NewCode(codeTokenNotAccepted)
		}
		var policy struct {
			ID        int64
			ExpiresAt *time.Time
		}
		if err := s.db.GetScan(ctx, &policy, `SELECT id,expires_at FROM plugin_idc_channel_token WHERE channel_id=? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>CURRENT_TIMESTAMP) ORDER BY id DESC LIMIT 1 FOR UPDATE`, account.ChannelID); err != nil {
			return err
		}
		if policy.ID != expectedID {
			return bizerr.NewCode(codeTokenNotAccepted)
		}
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return err
		}
		plain := hex.EncodeToString(bytes)
		hash := sha256.Sum256([]byte(plain))
		id, err := s.db.GetValue(ctx, `INSERT INTO plugin_idc_channel_token(channel_id,token_hash,scopes,source_cidrs,rate_limit,expires_at,created_by) SELECT channel_id,?,scopes,source_cidrs,rate_limit,expires_at,? FROM plugin_idc_channel_token WHERE id=? AND channel_id=? RETURNING id`, hex.EncodeToString(hash[:]), s.actorID(ctx), policy.ID, account.ChannelID)
		if err != nil {
			return err
		}
		// Carry the current window forward so resetting cannot replenish this minute's quota.
		if _, err := s.db.Exec(ctx, `INSERT INTO plugin_idc_channel_token_rate_window(token_id,window_started_at,request_count,updated_at) SELECT ?,window_started_at,request_count,CURRENT_TIMESTAMP FROM plugin_idc_channel_token_rate_window WHERE token_id=?`, id.Int64(), policy.ID); err != nil {
			return err
		}
		if _, err := s.db.Exec(ctx, `UPDATE plugin_idc_channel_token SET revoked_at=CURRENT_TIMESTAMP WHERE channel_id=? AND id<>? AND revoked_at IS NULL`, account.ChannelID, id.Int64()); err != nil {
			return err
		}
		result = TokenResult{AppID: account.ChannelID, ID: id.Int64(), Token: plain, ExpiresAt: policy.ExpiresAt}
		return nil
	})
	return result, err
}

// OwnCalls enforces ownership before applying database pagination and matching counts.
func (s *service) OwnCalls(ctx context.Context, query PageQuery) (PageResult[APICall], error) {
	account, err := s.ownAccount(ctx)
	if err != nil {
		return PageResult[APICall]{}, err
	}
	if account == nil {
		return PageResult[APICall]{}, bizerr.NewCode(codeTokenNotAccepted)
	}
	model := s.db.Model("plugin_idc_channel_api_call").Ctx(ctx).Where("channel_id=?", account.ChannelID).OrderDesc("id")
	return queryPage(model, query, func(page *gdb.Model) ([]APICall, error) {
		rows := []APICall{}
		err := page.Fields("id,method,path,succeeded,duration_millis,created_at").Scan(&rows)
		return rows, err
	})
}

// RecordCall appends metadata only, never affecting an already committed transaction.
func (s *service) RecordCall(ctx context.Context, call APICall) error {
	_, err := s.db.Exec(ctx, `INSERT INTO plugin_idc_channel_api_call(channel_id,method,path,succeeded,duration_millis) VALUES(?,?,?,?,?)`, call.ChannelID, call.Method, call.Path, call.Succeeded, call.DurationMillis)
	return err
}
