// This file creates customer-bound applications atomically. Only the secret hash
// is persisted; creating another credential rotates all old credentials.
package channel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
)

// authorizeStaff requires an installation-level staff data scope. Customer
// records are installation owned; narrower organization scopes are not widened.
func (s *service) authorizeStaff(ctx context.Context) error {
	if s.bizCtx == nil {
		return bizerr.NewCode(codeTokenNotAccepted)
	}
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != 0 || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return bizerr.NewCode(codeTokenNotAccepted)
	}
	return nil
}

func (s *service) createCustomerToken(ctx context.Context, input TokenInput) (TokenResult, error) {
	if err := s.authorizeStaff(ctx); err != nil {
		return TokenResult{}, err
	}
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.CustomerNo == "" || len(input.CustomerNo) > 40 || input.DisplayName == "" || len(input.DisplayName) > 160 {
		return TokenResult{}, bizerr.NewCode(codeLifecycleInvalid)
	}
	for _, scope := range input.Scopes {
		if scope == TokenScopeAll || scope == "settlement.repay" {
			return TokenResult{}, bizerr.NewCode(codeLifecycleInvalid)
		}
	}
	if _, err := s.core.ValidatePayer(ctx, input.CustomerNo); err != nil {
		return TokenResult{}, err
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return TokenResult{}, err
	}
	var result TokenResult
	err := s.db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		value, err := s.db.GetValue(ctx, `INSERT INTO plugin_idc_channel_account(channel_id,customer_no,display_name,status_code,new_sales_enabled,approved_by,approved_at,created_by,updated_by) VALUES(?,?,?,'approved',TRUE,?,CURRENT_TIMESTAMP,?,?) ON CONFLICT(customer_no) DO UPDATE SET display_name=EXCLUDED.display_name RETURNING channel_id`, "app_"+hex.EncodeToString(bytes), input.CustomerNo, input.DisplayName, s.actorID(ctx), s.actorID(ctx), s.actorID(ctx))
		if err != nil {
			return err
		}
		appID := value.String()
		input.ChannelID = appID
		input.CustomerNo = ""
		result, err = s.CreateToken(ctx, input)
		if err != nil {
			return err
		}
		_, err = s.db.Exec(ctx, `UPDATE plugin_idc_channel_token SET revoked_at=CURRENT_TIMESTAMP WHERE channel_id=? AND id<>? AND revoked_at IS NULL`, appID, result.ID)
		return err
	})
	return result, err
}
