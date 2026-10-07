// This file atomically consumes and directly delivers customer-authorized
// confidential server-operation output without retaining plaintext.

package serveroperationsource

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/internal/composition"
)

var _ serveroperationresourcecap.ConfidentialOutputRedeemer = (*Service)(nil)

type confidentialOutputRow struct {
	StatusCode   string     `orm:"status_code"`
	SecretRef    string     `orm:"secret_ref"`
	OutputPolicy string     `orm:"output_policy"`
	ExpiresAt    *time.Time `orm:"expires_at"`
}

func (s *Service) RedeemOutput(ctx context.Context,
	request serveroperationresourcecap.ConfidentialOutputRequest,
	consume serveroperationresourcecap.ConfidentialOutputConsumer) error {
	if s == nil || s.database == nil || s.secrets == nil || ctx == nil || ctx.Err() != nil ||
		!validDirectoryReference(request.ProviderTaskNo, 64) || !validDirectoryReference(request.ChannelNo, 64) ||
		request.Handle.IsZero() || (request.Name != "generatedCredential" && request.Name != "consoleTicket") ||
		(request.Policy != serveroperationresourcecap.SecretOutputPolicyOneTimeCredential &&
			request.Policy != serveroperationresourcecap.SecretOutputPolicyShortLivedTicket) || consume == nil {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	accessor, err := composition.BindProviderSecretAccessor(s.secrets, request.ChannelNo)
	if err != nil {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	purpose, err := secretcap.NewPurpose("resource.customer-output")
	if err != nil {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	accessRequest, err := secretcap.NewAccessRequest(request.Handle, purpose)
	if err != nil {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	var deliveryErr error
	err = accessor.Access(ctx, accessRequest, func(value []byte) error {
		if len(value) == 0 || len(value) > 1<<20 || !json.Valid(value) || value[0] != '{' {
			return gerror.New("invalid confidential output")
		}
		if consumeErr := s.consumeOutputBinding(ctx, request); consumeErr != nil {
			deliveryErr = consumeErr
			return consumeErr
		}
		deliveryErr = consume(value)
		return deliveryErr
	})
	if deliveryErr != nil {
		if errors.Is(deliveryErr, serveroperationresourcecap.ErrOutputConsumed) ||
			errors.Is(deliveryErr, serveroperationresourcecap.ErrOutputUnavailable) {
			return deliveryErr
		}
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	if err != nil {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	return nil
}

// consumeOutputBinding atomically changes the exact active output binding to
// revealed before its plaintext is delivered to the HTTP response.
func (s *Service) consumeOutputBinding(ctx context.Context,
	request serveroperationresourcecap.ConfidentialOutputRequest) error {
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var row *confidentialOutputRow
		if queryErr := tx.GetScan(&row, `SELECT binding.status_code,binding.secret_ref,
binding.output_policy,binding.expires_at
FROM plugin_idc_resource_task_secret_binding binding
JOIN plugin_idc_resource_task task ON task.tenant_id=binding.tenant_id AND task.task_no=binding.task_no
WHERE binding.tenant_id=? AND binding.task_no=? AND task.channel_no=?
 AND binding.binding_direction='output' AND binding.binding_name=?
FOR UPDATE`, idccap.DefaultTenantID, request.ProviderTaskNo, request.ChannelNo, request.Name); queryErr != nil {
			return gerror.Wrap(queryErr, "lock confidential server output")
		}
		if row == nil || row.SecretRef != request.Handle.OpaqueRef() || row.OutputPolicy != string(request.Policy) ||
			!sameOutputExpiry(row.ExpiresAt, request.ExpiresAt) {
			return serveroperationresourcecap.ErrOutputUnavailable
		}
		if row.StatusCode != "active" {
			return serveroperationresourcecap.ErrOutputConsumed
		}
		nowValue, queryErr := tx.GetValue("SELECT CURRENT_TIMESTAMP")
		if queryErr != nil {
			return gerror.Wrap(queryErr, "read confidential output database time")
		}
		now := nowValue.Time().UTC()
		if row.ExpiresAt != nil && !row.ExpiresAt.After(now) {
			return serveroperationresourcecap.ErrOutputUnavailable
		}
		result, updateErr := tx.Exec(`UPDATE plugin_idc_resource_task_secret_binding
SET status_code='revealed',revealed_at=?,version=version+1,updated_at=?
WHERE tenant_id=? AND task_no=? AND binding_direction='output' AND binding_name=? AND status_code='active'`,
			now, now, idccap.DefaultTenantID, request.ProviderTaskNo, request.Name)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "consume confidential server output")
		}
		affected, updateErr := result.RowsAffected()
		if updateErr != nil || affected != 1 {
			return serveroperationresourcecap.ErrOutputConsumed
		}
		return nil
	})
}

func sameOutputExpiry(stored *time.Time, expected time.Time) bool {
	if stored == nil {
		return expected.IsZero()
	}
	return !expected.IsZero() && stored.UTC().Truncate(time.Microsecond).Equal(expected.UTC().Truncate(time.Microsecond))
}
