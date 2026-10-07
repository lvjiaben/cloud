// This file owns the current employee's WeCom OAuth intent, encrypted identity
// binding, invalidation, unbinding, and crash recovery state machines.

package notification

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
)

const (
	wecomOAuthIntentTable  = "plugin_idc_notify_wecom_oauth_intent"
	wecomBindingTable      = "plugin_idc_notify_wecom_employee_binding"
	wecomBindingAuditTable = "plugin_idc_notify_wecom_binding_audit"

	wecomBindingConfiguring = "configuring"
	wecomBindingActive      = "active"
	wecomBindingRevoking    = "revoking"
	wecomBindingUnbound     = "unbound"
	wecomBindingInvalidated = "invalidated"

	wecomOAuthPending    = "pending"
	wecomOAuthExchanging = "exchanging"
	wecomOAuthCompleted  = "completed"
	wecomOAuthFailed     = "failed"

	wecomOAuthLifetime       = 5 * time.Minute
	wecomRecoveryLease       = 2 * time.Minute
	wecomRecoveryDelay       = 30 * time.Second
	maximumWeComCleanupBatch = 100

	wecomPublicOriginEnvironment = "LINAPRO_IDC_NOTIFY_PUBLIC_ORIGIN"
	wecomOAuthCallbackPath       = "/x/idc-notify/api/v1/staff/notifications/wechat-work/oauth-callback"
)

type wecomOAuthResolver interface {
	ResolveOAuthUser(context.Context, secretcap.Handle, map[string]string, string) ([]byte, error)
}

type wecomOAuthIntentRow struct {
	IntentNo          string     `orm:"intent_no"`
	StateDigest       string     `orm:"state_digest"`
	HostUserID        int        `orm:"host_user_id"`
	StaffVersion      int64      `orm:"staff_version"`
	SessionDigest     string     `orm:"session_digest"`
	ChannelGeneration int64      `orm:"channel_generation"`
	ChannelVersion    int64      `orm:"channel_version"`
	CorpID            string     `orm:"corp_id"`
	AgentID           int64      `orm:"agent_id"`
	StatusCode        string     `orm:"status_code"`
	FailureCode       string     `orm:"failure_code"`
	LeaseToken        *string    `orm:"lease_token"`
	LeaseExpiresAt    *time.Time `orm:"lease_expires_at"`
	ExpiresAt         time.Time  `orm:"expires_at"`
	CompletedAt       *time.Time `orm:"completed_at"`
	Version           int64      `orm:"version"`
}

type wecomBindingRow struct {
	BindingNo               string     `orm:"binding_no"`
	HostUserID              int        `orm:"host_user_id"`
	StaffVersion            int64      `orm:"staff_version"`
	CorpID                  string     `orm:"corp_id"`
	ChannelGeneration       int64      `orm:"channel_generation"`
	IdentityDigest          string     `orm:"identity_digest"`
	IdentitySecretRef       *string    `orm:"identity_secret_ref"`
	IdentitySecretUpdatedAt *time.Time `orm:"identity_secret_updated_at"`
	SecretOperationKey      string     `orm:"secret_operation_key"`
	StatusCode              string     `orm:"status_code"`
	TerminalStatusCode      string     `orm:"terminal_status_code"`
	RequestNo               string     `orm:"request_no"`
	AvailableAt             time.Time  `orm:"available_at"`
	LeaseToken              *string    `orm:"lease_token"`
	LeaseExpiresAt          *time.Time `orm:"lease_expires_at"`
	AttemptCount            int        `orm:"attempt_count"`
	LastErrorCode           string     `orm:"last_error_code"`
	Version                 int64      `orm:"version"`
	BoundAt                 *time.Time `orm:"bound_at"`
	TerminalAt              *time.Time `orm:"terminal_at"`
}

type WeComBindingAuthorization struct {
	AuthorizationURL string
	QRLoginURL       string
	ExpiresAt        time.Time
	Reused           bool
}

type WeComBindingAuthorizationInput struct {
	IdempotencyKey string
}

type WeComBindingCompletionInput struct {
	State string
	Code  string
}

type WeComBindingStatus struct {
	Bound   bool
	Status  string
	Version int64
	BoundAt *time.Time
	Reused  bool
}

type WeComBindingRemovalInput struct {
	ExpectedVersion int64
	IdempotencyKey  string
	Confirmed       bool
}

// StartOwnWeComBinding creates a session-bound, five-minute official OAuth
// state. Replays return the same URL without storing the raw state in SQL.
func (s *serviceImpl) StartOwnWeComBinding(
	ctx context.Context,
	input WeComBindingAuthorizationInput,
) (*WeComBindingAuthorization, error) {
	if !validateIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	staff, session, sessionDigest, err := s.currentWeComStaffSession(ctx, true)
	if err != nil {
		return nil, err
	}
	channel, configuration, err := s.loadWeComChannel(ctx)
	if err != nil {
		return nil, err
	}
	callbackURL, err := wecomOAuthCallbackURL()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeChannelUnavailable)
	}
	if existing, loadErr := s.loadActiveWeComBinding(ctx, staff.HostUserID); loadErr != nil {
		return nil, publicError(loadErr)
	} else if existing != nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	intentNo := stableNumber("nwcoauth_", strconv.Itoa(staff.HostUserID), sessionDigest, input.IdempotencyKey)
	var existing wecomOAuthIntentRow
	if err = s.database.Model(wecomOAuthIntentTable).Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).Where("intent_no", intentNo).Scan(&existing); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, publicError(err)
	}
	reused := existing.IntentNo != ""
	expiresAt := s.now().UTC().Add(wecomOAuthLifetime)
	if reused {
		if existing.HostUserID != staff.HostUserID || existing.StaffVersion != staff.StaffVersion ||
			existing.SessionDigest != sessionDigest || existing.ChannelGeneration != channel.ConfigurationGeneration ||
			existing.ChannelVersion != channel.ConfigurationVersion || existing.CorpID != configuration["corp_id"] ||
			existing.AgentID != parsePositiveInt64(configuration["agent_id"]) || existing.StatusCode != wecomOAuthPending ||
			!existing.ExpiresAt.After(s.now().UTC()) {
			return nil, bizerr.NewCode(CodeConflict)
		}
		expiresAt = existing.ExpiresAt.UTC()
	}
	state := deterministicWeComState(string(session.ID), intentNo, expiresAt)
	stateDigest := digestText("wecom_oauth_state_v1", state)
	if !reused {
		result, insertErr := s.database.Model(wecomOAuthIntentTable).Ctx(ctx).Data(modeldo.WecomOauthIntent{
			TenantId: notificationTenantID, IntentNo: intentNo, StateDigest: stateDigest,
			HostUserId: staff.HostUserID, StaffVersion: staff.StaffVersion, SessionDigest: sessionDigest,
			ChannelGeneration: channel.ConfigurationGeneration, ChannelVersion: channel.ConfigurationVersion,
			CorpId: configuration["corp_id"], AgentId: parsePositiveInt64(configuration["agent_id"]),
			StatusCode: wecomOAuthPending, ExpiresAt: expiresAt,
		}).Insert()
		if insertErr != nil || requireOne(result) != nil {
			return nil, publicError(firstChannelIntentError(insertErr, CodeConflict))
		}
	}
	return &WeComBindingAuthorization{
		AuthorizationURL: wecomAuthorizationURL(configuration["corp_id"], callbackURL, state),
		QRLoginURL:       wecomQRLoginURL(configuration["corp_id"], configuration["agent_id"], callbackURL, state),
		ExpiresAt:        expiresAt, Reused: reused,
	}, nil
}

// CompleteOwnWeComBinding consumes the state before the provider call and
// never retries an ambiguous code exchange. Secret persistence is recoverable
// through ProcessWeComBindings without retaining the provider UserID in SQL.
func (s *serviceImpl) CompleteOwnWeComBinding(
	ctx context.Context,
	input WeComBindingCompletionInput,
) (*WeComBindingStatus, error) {
	if input.State == "" || input.State != strings.TrimSpace(input.State) || len(input.State) > 128 ||
		input.Code == "" || input.Code != strings.TrimSpace(input.Code) || len(input.Code) > 512 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	staff, _, sessionDigest, err := s.currentWeComStaffSession(ctx, true)
	if err != nil {
		return nil, err
	}
	intent, reused, err := s.claimWeComOAuthIntent(ctx, input.State, staff, sessionDigest)
	if err != nil {
		return nil, err
	}
	if reused {
		status, statusErr := s.ownWeComBindingStatus(ctx, staff)
		if statusErr == nil {
			status.Reused = true
		}
		return status, statusErr
	}
	channel, configuration, err := s.loadWeComChannel(ctx)
	if err != nil || channel.ConfigurationGeneration != intent.ChannelGeneration ||
		channel.ConfigurationVersion != intent.ChannelVersion || configuration["corp_id"] != intent.CorpID ||
		parsePositiveInt64(configuration["agent_id"]) != intent.AgentID {
		s.failWeComOAuthIntent(ctx, intent, "channel_changed")
		return nil, errorOrCode(err, CodeConflict)
	}
	provider, exists := s.adapters[providerWeCom]
	resolver, resolverOK := provider.adapter.(wecomOAuthResolver)
	if !exists || !resolverOK || channel.CredentialSecretRef == nil {
		s.failWeComOAuthIntent(ctx, intent, "resolver_unavailable")
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	handle, err := secretcap.NewHandle(*channel.CredentialSecretRef)
	if err != nil {
		s.failWeComOAuthIntent(ctx, intent, "credential_unavailable")
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	userID, err := resolver.ResolveOAuthUser(ctx, handle, configuration, input.Code)
	if err != nil {
		s.failWeComOAuthIntent(ctx, intent, "identity_exchange_failed")
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	defer clear(userID)
	current, _, currentSessionDigest, err := s.currentWeComStaffSession(ctx, true)
	if err != nil || current.HostUserID != staff.HostUserID || current.StaffVersion != staff.StaffVersion ||
		currentSessionDigest != sessionDigest {
		s.failWeComOAuthIntent(ctx, intent, "staff_changed")
		return nil, errorOrCode(err, CodeNotAccessible)
	}
	binding, err := s.prepareWeComBinding(ctx, intent, userID)
	if err != nil {
		s.failWeComOAuthIntent(ctx, intent, "binding_conflict")
		return nil, publicError(err)
	}
	secretBinding, err := s.secretBinding("wechat_work")
	if err != nil {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	created, err := secretBinding.Manager.Create(ctx, binding.SecretOperationKey, func(consume secretcap.Consumer) error {
		return consume(userID)
	})
	if err != nil || created.Handle.IsZero() || !validConfiguredSecretMetadata(created.Metadata) {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	active, err := s.activateWeComBinding(ctx, binding, created.Handle, created.Metadata)
	if err != nil {
		return nil, publicError(err)
	}
	if !active {
		return nil, bizerr.NewCode(CodeConflict)
	}
	status, err := s.ownWeComBindingStatus(ctx, staff)
	if err == nil {
		status.Reused = created.Reused
	}
	return status, err
}

func (s *serviceImpl) GetOwnWeComBinding(ctx context.Context) (*WeComBindingStatus, error) {
	staff, _, _, err := s.currentWeComStaffSession(ctx, false)
	if err != nil {
		return nil, err
	}
	return s.ownWeComBindingStatus(ctx, staff)
}

func (s *serviceImpl) RemoveOwnWeComBinding(
	ctx context.Context,
	input WeComBindingRemovalInput,
) (*WeComBindingStatus, error) {
	if !input.Confirmed || input.ExpectedVersion <= 0 || !validateIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	staff, _, _, err := s.currentWeComStaffSession(ctx, true)
	if err != nil {
		return nil, err
	}
	requestNo := stableNumber("nwcunbind_", strconv.Itoa(staff.HostUserID), input.IdempotencyKey)
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		binding, loadErr := s.loadCurrentWeComBindingForUpdate(ctx, staff.HostUserID)
		if loadErr != nil {
			return loadErr
		}
		if binding == nil || binding.StaffVersion != staff.StaffVersion {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if binding.StatusCode == wecomBindingRevoking && binding.RequestNo == requestNo && binding.TerminalStatusCode == wecomBindingUnbound {
			return nil
		}
		if binding.StatusCode != wecomBindingActive || binding.Version != input.ExpectedVersion || binding.IdentitySecretRef == nil || binding.IdentitySecretUpdatedAt == nil {
			return bizerr.NewCode(CodeConflict)
		}
		updated, updateErr := s.database.Model(wecomBindingTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("binding_no", binding.BindingNo).
			Where("status_code", wecomBindingActive).Where("version", input.ExpectedVersion).
			Data(modeldo.WecomEmployeeBinding{
				StatusCode: wecomBindingRevoking, TerminalStatusCode: wecomBindingUnbound,
				RequestNo: requestNo, SecretOperationKey: stableNumber("nwcsecop_", binding.BindingNo, requestNo),
				AvailableAt: gdb.Raw("CURRENT_TIMESTAMP"), Version: gdb.Raw("version + 1"),
			}).Update()
		if updateErr != nil || requireOne(updated) != nil {
			return firstChannelIntentError(updateErr, CodeConflict)
		}
		return s.appendWeComBindingAudit(ctx, binding.BindingNo, staff.HostUserID, "unbind", "employee_self_service",
			wecomBindingActive, wecomBindingRevoking, input.ExpectedVersion+1)
	})
	if err != nil {
		return nil, publicError(err)
	}
	_ = s.ProcessWeComBindings(ctx, 1)
	return s.ownWeComBindingStatus(ctx, staff)
}

// ProcessWeComBindings recovers secret creation/revocation and incrementally
// cancels queued deliveries. It performs no provider message send.
func (s *serviceImpl) ProcessWeComBindings(ctx context.Context, limit int) error {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > maximumWorkerBatch {
		return bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.database.Model(wecomOAuthIntentTable).Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).
		Where("status_code IN ('pending','exchanging')").
		WhereLTE("expires_at", s.now().UTC()).
		Data(modeldo.WecomOauthIntent{StatusCode: wecomOAuthFailed, FailureCode: "authorization_expired",
			LeaseToken: gdb.Raw("NULL::varchar"), LeaseExpiresAt: gdb.Raw("NULL::timestamptz"), Version: gdb.Raw("version + 1")}).Update(); err != nil {
		return publicError(err)
	}
	rows, err := s.claimWeComBindings(ctx, limit)
	if err != nil {
		return publicError(err)
	}
	for index := range rows {
		row := &rows[index]
		if recoverErr := s.recoverWeComBinding(ctx, row); recoverErr != nil {
			if releaseErr := s.releaseWeComBinding(ctx, row, "recovery_waiting"); releaseErr != nil {
				return publicError(releaseErr)
			}
		}
	}
	return nil
}

// InvalidateWeComBindingsForCorpChange makes every old-corporation mapping
// immediately non-active inside the caller's channel-configuration transaction.
func (s *serviceImpl) InvalidateWeComBindingsForCorpChange(ctx context.Context, previousCorpID, currentCorpID string, actor int) error {
	if previousCorpID == "" || previousCorpID == currentCorpID {
		return nil
	}
	if actor <= 0 {
		return bizerr.NewCode(CodeNotAccessible)
	}
	_, err := s.database.GetValue(ctx, `
WITH updated AS (
    UPDATE plugin_idc_notify_wecom_employee_binding
    SET status_code='revoking',terminal_status_code='invalidated',
        request_no='nwcinvalid_' || md5(binding_no || ':' || ?),
        secret_operation_key='nwcsecop_' || md5(binding_no || ':invalidate:' || ?),
        available_at=CURRENT_TIMESTAMP,lease_token=NULL,lease_expires_at=NULL,
        version=version+1,updated_at=CURRENT_TIMESTAMP
    WHERE tenant_id=? AND corp_id=? AND status_code='active'
    RETURNING tenant_id,binding_no,host_user_id,version
), audited AS (
    INSERT INTO plugin_idc_notify_wecom_binding_audit
        (tenant_id,audit_no,binding_no,actor_user_id,action_code,reason_code,
         previous_status,current_status,binding_version)
    SELECT tenant_id,'nwca_' || md5(binding_no || ':invalidate:' || version::text),binding_no,?,
           'invalidate','corp_id_changed','active','revoking',version
    FROM updated
    ON CONFLICT (tenant_id,audit_no) DO NOTHING
    RETURNING 1
)
SELECT count(*) FROM updated`, currentCorpID, currentCorpID, notificationTenantID, previousCorpID, actor)
	return err
}

// WithWeComDestination releases one active binding only inside the callback.
// Raw UserIDs never leave this method, enter SQL, logs, audit, or API DTOs.
func (s *serviceImpl) WithWeComDestination(ctx context.Context, hostUserID int, consume secretcap.Consumer) error {
	if hostUserID <= 0 || consume == nil {
		return bizerr.NewCode(CodeInvalid)
	}
	channel, configuration, err := s.loadWeComChannel(ctx)
	if err != nil {
		return err
	}
	binding, err := s.loadActiveWeComBinding(ctx, hostUserID)
	if err != nil || binding == nil || binding.CorpID != configuration["corp_id"] || binding.IdentitySecretRef == nil ||
		channel.ProviderCode == nil || *channel.ProviderCode != providerWeCom {
		return errorOrCode(err, CodeChannelUnavailable)
	}
	handle, err := secretcap.NewHandle(*binding.IdentitySecretRef)
	if err != nil {
		return bizerr.NewCode(CodeChannelUnavailable)
	}
	secretBinding, err := s.secretBinding("wechat_work")
	if err != nil {
		return bizerr.NewCode(CodeChannelUnavailable)
	}
	purpose, err := secretcap.NewPurpose("notify.send")
	if err != nil {
		return bizerr.NewCode(CodeChannelUnavailable)
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return bizerr.NewCode(CodeChannelUnavailable)
	}
	return secretBinding.Accessor.Access(ctx, request, func(value []byte) error {
		if len(value) < 1 || len(value) > 64 {
			return bizerr.NewCode(CodeChannelUnavailable)
		}
		return consume(value)
	})
}

func (s *serviceImpl) currentWeComStaffSession(
	ctx context.Context,
	recent bool,
) (notificationcap.CurrentStaffProjection, *sessioncap.SessionInfo, string, error) {
	staff, err := s.sources.Staff.Current(ctx)
	if err != nil || staff.HostUserID <= 0 || staff.StaffVersion <= 0 {
		return notificationcap.CurrentStaffProjection{}, nil, "", errorOrCode(err, CodeNotAccessible)
	}
	current, err := s.sessions.Current(ctx)
	if err != nil || current == nil || current.ID == "" || current.UserID != strconv.Itoa(staff.HostUserID) || current.TenantID != "0" {
		return notificationcap.CurrentStaffProjection{}, nil, "", errorOrCode(err, CodeNotAccessible)
	}
	if recent {
		if current.LoginAt == nil {
			return notificationcap.CurrentStaffProjection{}, nil, "", bizerr.NewCode(CodeRecentAuthenticationRequired)
		}
		now := s.now().UTC()
		loginAt := current.LoginAt.UTC()
		if loginAt.After(now.Add(15*time.Second)) || now.Sub(loginAt) > 5*time.Minute {
			return notificationcap.CurrentStaffProjection{}, nil, "", bizerr.NewCode(CodeRecentAuthenticationRequired)
		}
	}
	sessionDigest := digestText("wecom_staff_session_v1", string(current.ID), strconv.Itoa(staff.HostUserID))
	return staff, current, sessionDigest, nil
}

func (s *serviceImpl) loadWeComChannel(ctx context.Context) (*channelRow, map[string]string, error) {
	var row channelRow
	err := s.database.Model(channelTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("channel_code", "wechat_work").Scan(&row)
	if err != nil || row.ChannelCode == "" || row.ProviderCode == nil || *row.ProviderCode != providerWeCom ||
		row.CredentialSecretRef == nil || row.ConfigurationGeneration <= 0 || row.ConfigurationVersion <= 0 ||
		(ChannelStatus(row.StatusCode) != ChannelEnabled && ChannelStatus(row.StatusCode) != ChannelDisabled) {
		return nil, nil, errorOrCode(err, CodeChannelUnavailable)
	}
	configuration := make(map[string]string)
	if json.Unmarshal([]byte(row.NonSecretConfiguration), &configuration) != nil || configuration["corp_id"] == "" ||
		parsePositiveInt64(configuration["agent_id"]) <= 0 {
		return nil, nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	return &row, configuration, nil
}

func (s *serviceImpl) claimWeComOAuthIntent(
	ctx context.Context,
	state string,
	staff notificationcap.CurrentStaffProjection,
	sessionDigest string,
) (*wecomOAuthIntentRow, bool, error) {
	stateDigest := digestText("wecom_oauth_state_v1", state)
	var claimed wecomOAuthIntentRow
	reused := false
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var row wecomOAuthIntentRow
		if err := s.database.Model(wecomOAuthIntentTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("state_digest", stateDigest).LockUpdate().Scan(&row); err != nil {
			return err
		}
		if row.IntentNo == "" || row.HostUserID != staff.HostUserID || row.StaffVersion != staff.StaffVersion ||
			row.SessionDigest != sessionDigest {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if row.StatusCode == wecomOAuthCompleted {
			claimed, reused = row, true
			return nil
		}
		if row.StatusCode != wecomOAuthPending || !row.ExpiresAt.After(s.now().UTC()) {
			return bizerr.NewCode(CodeConflict)
		}
		lease, err := newWeComLease("nwcoauthlease_")
		if err != nil {
			return err
		}
		result, err := s.database.Model(wecomOAuthIntentTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("intent_no", row.IntentNo).
			Where("status_code", wecomOAuthPending).Where("version", row.Version).
			Data(modeldo.WecomOauthIntent{StatusCode: wecomOAuthExchanging, LeaseToken: lease,
				LeaseExpiresAt: gdb.Raw("CURRENT_TIMESTAMP + INTERVAL '2 minutes'"), Version: gdb.Raw("version + 1")}).Update()
		if err != nil || requireOne(result) != nil {
			return firstChannelIntentError(err, CodeConflict)
		}
		row.StatusCode, row.LeaseToken, row.Version = wecomOAuthExchanging, &lease, row.Version+1
		claimed = row
		return nil
	})
	if err != nil {
		return nil, false, publicError(err)
	}
	return &claimed, reused, nil
}

func (s *serviceImpl) failWeComOAuthIntent(ctx context.Context, intent *wecomOAuthIntentRow, code string) {
	if intent == nil || intent.LeaseToken == nil {
		return
	}
	_, _ = s.database.Model(wecomOAuthIntentTable).Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).Where("intent_no", intent.IntentNo).
		Where("status_code", wecomOAuthExchanging).Where("lease_token", *intent.LeaseToken).
		Data(modeldo.WecomOauthIntent{StatusCode: wecomOAuthFailed, FailureCode: code,
			LeaseToken: gdb.Raw("NULL::varchar"), LeaseExpiresAt: gdb.Raw("NULL::timestamptz"), Version: gdb.Raw("version + 1")}).Update()
}

func (s *serviceImpl) prepareWeComBinding(ctx context.Context, intent *wecomOAuthIntentRow, userID []byte) (*wecomBindingRow, error) {
	if intent == nil || intent.LeaseToken == nil || len(userID) < 1 || len(userID) > 64 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	identityHash := sha256.Sum256([]byte(strings.ToLower(string(userID))))
	identityDigest := digestText("wecom_identity_v1", intent.CorpID, hex.EncodeToString(identityHash[:]))
	bindingNo := stableNumber("nwcb_", intent.IntentNo)
	secretOperationKey := stableNumber("nwcsecop_", intent.IntentNo, "bind")
	row := &wecomBindingRow{BindingNo: bindingNo, HostUserID: intent.HostUserID, StaffVersion: intent.StaffVersion,
		CorpID: intent.CorpID, ChannelGeneration: intent.ChannelGeneration, IdentityDigest: identityDigest,
		SecretOperationKey: secretOperationKey, StatusCode: wecomBindingConfiguring, RequestNo: intent.IntentNo, Version: 1}
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var locked wecomOAuthIntentRow
		if err := s.database.Model(wecomOAuthIntentTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("intent_no", intent.IntentNo).LockUpdate().Scan(&locked); err != nil {
			return err
		}
		if locked.StatusCode != wecomOAuthExchanging || locked.LeaseToken == nil || *locked.LeaseToken != *intent.LeaseToken {
			return bizerr.NewCode(CodeConflict)
		}
		result, err := s.database.Model(wecomBindingTable).Ctx(ctx).Data(modeldo.WecomEmployeeBinding{
			TenantId: notificationTenantID, BindingNo: bindingNo, HostUserId: intent.HostUserID,
			StaffVersion: intent.StaffVersion, CorpId: intent.CorpID, ChannelGeneration: intent.ChannelGeneration,
			IdentityDigest: identityDigest, SecretOperationKey: secretOperationKey,
			StatusCode: wecomBindingConfiguring, RequestNo: intent.IntentNo,
			AvailableAt: gdb.Raw("CURRENT_TIMESTAMP + INTERVAL '30 seconds'"),
		}).Insert()
		if err != nil || requireOne(result) != nil {
			return firstChannelIntentError(err, CodeConflict)
		}
		return nil
	})
	return row, err
}

func (s *serviceImpl) activateWeComBinding(
	ctx context.Context,
	binding *wecomBindingRow,
	handle secretcap.Handle,
	metadata secretcap.Metadata,
) (bool, error) {
	if binding == nil || handle.IsZero() || !validConfiguredSecretMetadata(metadata) {
		return false, bizerr.NewCode(CodeChannelUnavailable)
	}
	active := false
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var channel struct {
			ProviderCode           *string `orm:"provider_code"`
			StatusCode             string  `orm:"status_code"`
			NonSecretConfiguration string  `orm:"non_secret_configuration"`
		}
		if err := s.database.Model(channelTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("channel_code", "wechat_work").LockUpdate().Scan(&channel); err != nil {
			return err
		}
		configuration := make(map[string]string)
		channelMatches := channel.ProviderCode != nil && *channel.ProviderCode == providerWeCom &&
			(ChannelStatus(channel.StatusCode) == ChannelEnabled || ChannelStatus(channel.StatusCode) == ChannelDisabled) &&
			json.Unmarshal([]byte(channel.NonSecretConfiguration), &configuration) == nil &&
			configuration["corp_id"] == binding.CorpID
		if !channelMatches {
			result, err := s.database.Model(wecomBindingTable).Ctx(ctx).
				Where(tenantIDColumn, notificationTenantID).Where("binding_no", binding.BindingNo).
				Where("status_code", wecomBindingConfiguring).
				Data(modeldo.WecomEmployeeBinding{
					IdentitySecretRef: handle.OpaqueRef(), IdentitySecretUpdatedAt: metadata.UpdatedAt.UTC(),
					StatusCode: wecomBindingRevoking, TerminalStatusCode: wecomBindingInvalidated,
					AvailableAt: gdb.Raw("CURRENT_TIMESTAMP"), LeaseToken: gdb.Raw("NULL::varchar"), LeaseExpiresAt: gdb.Raw("NULL::timestamptz"),
					LastErrorCode: "corp_id_changed", Version: gdb.Raw("version + 1"),
				}).Update()
			if err != nil || requireOne(result) != nil {
				return firstChannelIntentError(err, CodeConflict)
			}
			if err = s.appendWeComBindingAudit(ctx, binding.BindingNo, binding.HostUserID, "invalidate", "corp_id_changed",
				wecomBindingConfiguring, wecomBindingRevoking, binding.Version+1); err != nil {
				return err
			}
			_, err = s.database.Model(wecomOAuthIntentTable).Ctx(ctx).
				Where(tenantIDColumn, notificationTenantID).Where("intent_no", binding.RequestNo).
				Where("status_code", wecomOAuthExchanging).
				Data(modeldo.WecomOauthIntent{StatusCode: wecomOAuthFailed, FailureCode: "corp_id_changed",
					LeaseToken: gdb.Raw("NULL::varchar"), LeaseExpiresAt: gdb.Raw("NULL::timestamptz"), Version: gdb.Raw("version + 1")}).Update()
			return err
		}
		result, err := s.database.Model(wecomBindingTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("binding_no", binding.BindingNo).
			Where("status_code", wecomBindingConfiguring).
			Data(modeldo.WecomEmployeeBinding{
				IdentitySecretRef: handle.OpaqueRef(), IdentitySecretUpdatedAt: metadata.UpdatedAt.UTC(),
				StatusCode: wecomBindingActive, BoundAt: gdb.Raw("COALESCE(bound_at,CURRENT_TIMESTAMP)"),
				AvailableAt: gdb.Raw("CURRENT_TIMESTAMP"), LeaseToken: gdb.Raw("NULL::varchar"), LeaseExpiresAt: gdb.Raw("NULL::timestamptz"),
				LastErrorCode: "", Version: gdb.Raw("version + 1"),
			}).Update()
		if err != nil || requireOne(result) != nil {
			return firstChannelIntentError(err, CodeConflict)
		}
		if err = s.appendWeComBindingAudit(ctx, binding.BindingNo, binding.HostUserID, "bind", "employee_oauth",
			wecomBindingConfiguring, wecomBindingActive, binding.Version+1); err != nil {
			return err
		}
		completed, err := s.database.Model(wecomOAuthIntentTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("intent_no", binding.RequestNo).
			Where("status_code", wecomOAuthExchanging).
			Data(modeldo.WecomOauthIntent{StatusCode: wecomOAuthCompleted,
				CompletedAt: gdb.Raw("CURRENT_TIMESTAMP"), LeaseToken: gdb.Raw("NULL::varchar"), LeaseExpiresAt: gdb.Raw("NULL::timestamptz"),
				FailureCode: "", Version: gdb.Raw("version + 1")}).Update()
		if err != nil || requireOne(completed) != nil {
			return firstChannelIntentError(err, CodeConflict)
		}
		active = true
		return nil
	})
	return active, err
}

func (s *serviceImpl) ownWeComBindingStatus(ctx context.Context, staff notificationcap.CurrentStaffProjection) (*WeComBindingStatus, error) {
	binding, err := s.loadCurrentWeComBinding(ctx, staff.HostUserID)
	if err != nil {
		return nil, publicError(err)
	}
	if binding == nil {
		return &WeComBindingStatus{Status: "unbound"}, nil
	}
	status := &WeComBindingStatus{Bound: binding.StatusCode == wecomBindingActive && binding.StaffVersion == staff.StaffVersion,
		Status: binding.StatusCode, Version: binding.Version, BoundAt: utcTimePointer(binding.BoundAt)}
	if binding.StaffVersion != staff.StaffVersion && binding.StatusCode == wecomBindingActive {
		status.Bound, status.Status = false, wecomBindingInvalidated
	}
	return status, nil
}

func (s *serviceImpl) loadActiveWeComBinding(ctx context.Context, hostUserID int) (*wecomBindingRow, error) {
	var row wecomBindingRow
	err := s.database.Model(wecomBindingTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("host_user_id", hostUserID).Where("status_code", wecomBindingActive).Scan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil || row.BindingNo == "" {
		return nil, err
	}
	return &row, nil
}

func (s *serviceImpl) loadCurrentWeComBinding(ctx context.Context, hostUserID int) (*wecomBindingRow, error) {
	var row wecomBindingRow
	err := s.database.Model(wecomBindingTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("host_user_id", hostUserID).OrderDesc("created_at").OrderDesc("binding_no").Limit(1).Scan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil || row.BindingNo == "" {
		return nil, err
	}
	return &row, nil
}

func (s *serviceImpl) loadCurrentWeComBindingForUpdate(ctx context.Context, hostUserID int) (*wecomBindingRow, error) {
	var row wecomBindingRow
	err := s.database.Model(wecomBindingTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("host_user_id", hostUserID).OrderDesc("created_at").OrderDesc("binding_no").Limit(1).LockUpdate().Scan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil || row.BindingNo == "" {
		return nil, err
	}
	return &row, nil
}

func (s *serviceImpl) claimWeComBindings(ctx context.Context, limit int) ([]wecomBindingRow, error) {
	lease, err := newWeComLease("nwclease_")
	if err != nil {
		return nil, err
	}
	var rows []wecomBindingRow
	err = s.database.GetScan(ctx, &rows, `
WITH due AS (
    SELECT tenant_id,binding_no
    FROM plugin_idc_notify_wecom_employee_binding
    WHERE tenant_id=? AND status_code IN ('configuring','revoking')
      AND available_at <= CURRENT_TIMESTAMP
      AND (lease_token IS NULL OR lease_expires_at < CURRENT_TIMESTAMP)
    ORDER BY available_at,binding_no
    FOR UPDATE SKIP LOCKED
    LIMIT ?
)
UPDATE plugin_idc_notify_wecom_employee_binding b
SET lease_token=?,lease_expires_at=CURRENT_TIMESTAMP + INTERVAL '2 minutes',
    attempt_count=attempt_count+1,updated_at=CURRENT_TIMESTAMP
FROM due
WHERE b.tenant_id=due.tenant_id AND b.binding_no=due.binding_no
RETURNING b.*`, notificationTenantID, limit, lease)
	return rows, err
}

func (s *serviceImpl) recoverWeComBinding(ctx context.Context, row *wecomBindingRow) error {
	if row == nil || row.LeaseToken == nil {
		return gerror.New("WeCom binding recovery lease is unavailable")
	}
	secretBinding, err := s.secretBinding("wechat_work")
	if err != nil {
		return err
	}
	switch row.StatusCode {
	case wecomBindingConfiguring:
		created, recoverErr := secretBinding.Manager.RecoverCreate(ctx, row.SecretOperationKey)
		if recoverErr != nil || created.Handle.IsZero() || !validConfiguredSecretMetadata(created.Metadata) {
			return gerror.New("WeCom binding secret create is not recoverable yet")
		}
		_, activateErr := s.activateWeComBinding(ctx, row, created.Handle, created.Metadata)
		return activateErr
	case wecomBindingRevoking:
		cancelled, cancelErr := s.cancelQueuedWeComTasks(ctx, row.HostUserID, maximumWeComCleanupBatch)
		if cancelErr != nil {
			return cancelErr
		}
		if cancelled == maximumWeComCleanupBatch {
			return gerror.New("WeCom queued delivery cleanup continues")
		}
		if row.IdentitySecretRef == nil || row.IdentitySecretUpdatedAt == nil {
			return gerror.New("WeCom binding secret state is invalid")
		}
		handle, handleErr := secretcap.NewHandle(*row.IdentitySecretRef)
		if handleErr != nil {
			return handleErr
		}
		metadata, revokeErr := secretBinding.Manager.Revoke(ctx, handle, row.IdentitySecretUpdatedAt.UTC())
		if revokeErr != nil || metadata.Configured || !metadata.UpdatedAt.UTC().After(row.IdentitySecretUpdatedAt.UTC()) {
			return gerror.New("WeCom binding secret revoke is not recoverable yet")
		}
		return s.finalizeWeComRevocation(ctx, row)
	default:
		return gerror.New("WeCom binding recovery status is invalid")
	}
}

func (s *serviceImpl) finalizeWeComRevocation(ctx context.Context, row *wecomBindingRow) error {
	if row == nil || row.LeaseToken == nil || (row.TerminalStatusCode != wecomBindingUnbound && row.TerminalStatusCode != wecomBindingInvalidated) {
		return bizerr.NewCode(CodeConflict)
	}
	result, err := s.database.Model(wecomBindingTable).Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).Where("binding_no", row.BindingNo).
		Where("status_code", wecomBindingRevoking).Where("lease_token", *row.LeaseToken).
		Data(modeldo.WecomEmployeeBinding{StatusCode: row.TerminalStatusCode,
			IdentitySecretRef: gdb.Raw("NULL::varchar"), IdentitySecretUpdatedAt: gdb.Raw("NULL::timestamptz"),
			LeaseToken: gdb.Raw("NULL::varchar"), LeaseExpiresAt: gdb.Raw("NULL::timestamptz"), LastErrorCode: "",
			TerminalAt: gdb.Raw("CURRENT_TIMESTAMP"), Version: gdb.Raw("version + 1")}).Update()
	if err != nil || requireOne(result) != nil {
		return firstChannelIntentError(err, CodeConflict)
	}
	return nil
}

func (s *serviceImpl) releaseWeComBinding(ctx context.Context, row *wecomBindingRow, code string) error {
	if row == nil || row.LeaseToken == nil {
		return gerror.New("WeCom binding recovery lease is unavailable")
	}
	result, err := s.database.Model(wecomBindingTable).Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).Where("binding_no", row.BindingNo).
		Where("lease_token", *row.LeaseToken).Where("status_code IN ('configuring','revoking')").
		Data(modeldo.WecomEmployeeBinding{LeaseToken: gdb.Raw("NULL::varchar"), LeaseExpiresAt: gdb.Raw("NULL::timestamptz"),
			AvailableAt: s.now().UTC().Add(wecomRecoveryDelay), LastErrorCode: code}).Update()
	if err != nil {
		return err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return rowsErr
	} else if affected != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func (s *serviceImpl) cancelQueuedWeComTasks(ctx context.Context, hostUserID, limit int) (int, error) {
	if hostUserID <= 0 || limit < 1 || limit > maximumWeComCleanupBatch {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	value, err := s.database.GetValue(ctx, `
WITH candidates AS (
    SELECT tenant_id,task_no,status_code,version
    FROM plugin_idc_notify_delivery_task
    WHERE tenant_id=? AND channel_code='wechat_work' AND owner_user_id=?
      AND external_boundary_crossed=FALSE AND status_code IN ('queued','retry_wait')
    ORDER BY queued_at,task_no
    FOR UPDATE SKIP LOCKED
    LIMIT ?
), updated AS (
    UPDATE plugin_idc_notify_delivery_task t
    SET status_code='cancelled',result_class='invalid_recipient',completed_at=CURRENT_TIMESTAMP,
        next_attempt_at=NULL,attempt_lease=NULL,attempt_lease_until=NULL,
        version=t.version+1,updated_at=CURRENT_TIMESTAMP
    FROM candidates c
    WHERE t.tenant_id=c.tenant_id AND t.task_no=c.task_no AND t.version=c.version
    RETURNING t.tenant_id,t.task_no,c.status_code AS previous_status,t.status_code,t.version
), audited AS (
    INSERT INTO plugin_idc_notify_delivery_audit
        (tenant_id,audit_no,task_no,action_code,actor_type,actor_user_id,reason_code,
         previous_status,current_status,task_version)
    SELECT tenant_id,'nda_ub_' || right(task_no,64),task_no,'cancel','staff',?,
           'employee_unbound',previous_status,status_code,version
    FROM updated
    ON CONFLICT (tenant_id,audit_no) DO NOTHING
    RETURNING 1
)
SELECT count(*) FROM updated`, notificationTenantID, hostUserID, limit, hostUserID)
	if err != nil {
		return 0, err
	}
	return value.Int(), nil
}

func (s *serviceImpl) appendWeComBindingAudit(
	ctx context.Context,
	bindingNo string,
	actor int,
	action, reason, previous, current string,
	version int64,
) error {
	if actor <= 0 {
		return bizerr.NewCode(CodeNotAccessible)
	}
	_, err := s.database.Model(wecomBindingAuditTable).Ctx(ctx).Data(modeldo.WecomBindingAudit{
		TenantId: notificationTenantID, AuditNo: stableNumber("nwca_", bindingNo, action, strconv.FormatInt(version, 10)),
		BindingNo: bindingNo, ActorUserId: actor, ActionCode: action, ReasonCode: reason,
		PreviousStatus: previous, CurrentStatus: current, BindingVersion: version,
	}).InsertIgnore()
	return err
}

func deterministicWeComState(sessionID, intentNo string, expiresAt time.Time) string {
	digest := sha256.Sum256([]byte("wecom-oauth-state-v1\x00" + sessionID + "\x00" + intentNo + "\x00" + strconv.FormatInt(expiresAt.UTC().UnixMilli(), 10)))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func wecomOAuthCallbackURL() (string, error) {
	origin := strings.TrimSpace(os.Getenv(wecomPublicOriginEnvironment))
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.Path != "" && parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", gerror.New("WeCom public origin is invalid")
	}
	return strings.TrimRight(origin, "/") + wecomOAuthCallbackPath, nil
}

func wecomAuthorizationURL(corpID, callbackURL, state string) string {
	values := url.Values{"appid": {corpID}, "redirect_uri": {callbackURL}, "response_type": {"code"},
		"scope": {"snsapi_base"}, "state": {state}}
	return "https://open.weixin.qq.com/connect/oauth2/authorize?" + values.Encode() + "#wechat_redirect"
}

func wecomQRLoginURL(corpID, agentID, callbackURL, state string) string {
	values := url.Values{"login_type": {"CorpApp"}, "appid": {corpID}, "agentid": {agentID},
		"redirect_uri": {callbackURL}, "state": {state}}
	return "https://login.work.weixin.qq.com/wwlogin/sso/login?" + values.Encode()
}

func newWeComLease(prefix string) (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", gerror.Wrap(err, "generate WeCom binding lease")
	}
	return prefix + hex.EncodeToString(value[:]), nil
}

func parsePositiveInt64(value string) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}
