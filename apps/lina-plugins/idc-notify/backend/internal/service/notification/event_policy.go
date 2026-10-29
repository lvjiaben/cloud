package notification

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
)

const eventPolicyTable = "plugin_idc_notify_event_policy"

type EventPolicy struct {
	EventCode     string
	Topic         string
	RecipientKind notificationcap.RecipientKind
	ChannelCode   string
	Enabled       bool
	Version       int64
	UpdatedAt     time.Time
}

type EventPolicyWriteInput struct {
	EventCode       string
	RecipientKind   notificationcap.RecipientKind
	ChannelCode     string
	Enabled         bool
	ExpectedVersion int64
	IdempotencyKey  string
}

type EventPolicyService interface {
	ListEventPolicies(context.Context, int) ([]EventPolicy, error)
	UpdateEventPolicy(context.Context, EventPolicyWriteInput) (*EventPolicy, error)
}

type eventPolicyRow struct {
	EventCode     string    `orm:"event_code"`
	TopicCode     string    `orm:"topic_code"`
	RecipientKind string    `orm:"recipient_kind"`
	ChannelCode   string    `orm:"channel_code"`
	Enabled       bool      `orm:"enabled"`
	Version       int64     `orm:"version"`
	UpdatedAt     time.Time `orm:"updated_at"`
}

func AsEventPolicyService(service Service) (EventPolicyService, error) {
	resolved, ok := service.(EventPolicyService)
	if !ok || resolved == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return resolved, nil
}

func (s *serviceImpl) ListEventPolicies(ctx context.Context, limit int) ([]EventPolicy, error) {
	if _, err := s.authorize(ctx, "notification-event-policies"); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = MaximumPageSize
	}
	if limit > MaximumPageSize {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var rows []eventPolicyRow
	err := s.database.Model(eventPolicyTable+" p").Ctx(ctx).
		InnerJoin("plugin_idc_notify_event_catalog e", "e.tenant_id=p.tenant_id AND e.event_code=p.event_code").
		Fields("p.event_code,e.topic_code,p.recipient_kind,p.channel_code,p.enabled,p.version,p.updated_at").
		Where("p.tenant_id", notificationTenantID).WhereIn("p.channel_code", supportedChannelCodes).
		OrderAsc("p.event_code").OrderAsc("p.recipient_kind").OrderAsc("p.channel_code").
		Limit(limit).Scan(&rows)
	if err != nil {
		return nil, publicError(err)
	}
	items := make([]EventPolicy, len(rows))
	for index := range rows {
		items[index] = projectEventPolicy(rows[index])
	}
	return items, nil
}

func (s *serviceImpl) UpdateEventPolicy(ctx context.Context, input EventPolicyWriteInput) (*EventPolicy, error) {
	actor, err := s.authorize(ctx, "notification-event-policy-"+input.EventCode)
	if err != nil {
		return nil, err
	}
	input.EventCode = strings.TrimSpace(input.EventCode)
	input.ChannelCode = strings.TrimSpace(input.ChannelCode)
	if !stableCodePattern.MatchString(input.EventCode) || !validEventPolicyRecipient(input.RecipientKind) ||
		!validEventPolicyChannel(input.RecipientKind, input.ChannelCode) || input.ExpectedVersion < 1 ||
		!validateIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var updated eventPolicyRow
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, lockErr := s.database.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))",
			"notification.event-policy:"+input.EventCode+":"+string(input.RecipientKind)); lockErr != nil {
			return lockErr
		}
		var current eventPolicyRow
		loadErr := s.database.Model(eventPolicyTable+" p").Ctx(ctx).
			InnerJoin("plugin_idc_notify_event_catalog e", "e.tenant_id=p.tenant_id AND e.event_code=p.event_code").
			Fields("p.event_code,e.topic_code,p.recipient_kind,p.channel_code,p.enabled,p.version,p.updated_at").
			Where("p.tenant_id", notificationTenantID).Where("p.event_code", input.EventCode).
			Where("p.recipient_kind", string(input.RecipientKind)).Where("p.channel_code", input.ChannelCode).
			LockUpdate().Scan(&current)
		if loadErr != nil || current.EventCode == "" {
			return errorOrCode(loadErr, CodeNotAccessible)
		}
		if current.Version != input.ExpectedVersion {
			if current.Enabled == input.Enabled {
				updated = current
				return nil
			}
			return bizerr.NewCode(CodeConflict)
		}
		if current.Enabled == input.Enabled {
			updated = current
			return nil
		}
		if !input.Enabled && current.TopicCode == string(notificationcap.TopicSecurity) &&
			input.RecipientKind == notificationcap.RecipientCustomerContact {
			remaining, countErr := s.database.Model(eventPolicyTable).Ctx(ctx).
				Where(tenantIDColumn, notificationTenantID).Where("event_code", input.EventCode).
				Where("recipient_kind", string(input.RecipientKind)).Where("enabled", true).
				WhereNot("channel_code", input.ChannelCode).
				WhereIn("channel_code", supportedChannelCodes).Count()
			if countErr != nil {
				return countErr
			}
			if remaining == 0 {
				return bizerr.NewCode(CodeConflict)
			}
		}
		updatedAt, timeErr := s.databaseTime(ctx)
		if timeErr != nil {
			return timeErr
		}
		result, updateErr := s.database.Model(eventPolicyTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("event_code", input.EventCode).
			Where("recipient_kind", string(input.RecipientKind)).Where("channel_code", input.ChannelCode).
			Where("version", input.ExpectedVersion).
			Data(modeldo.EventPolicy{Enabled: input.Enabled, UpdatedBy: actor,
				UpdatedAt: &updatedAt, Version: input.ExpectedVersion + 1}).Update()
		if updateErr != nil || requireOne(result) != nil {
			return errorOrCode(updateErr, CodeConflict)
		}
		if auditErr := s.management.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "notification.event-policy.update", IdempotencyKey: input.IdempotencyKey,
			OperationCode: "notification.event-policy.update", TargetType: "notification_event_policy",
			TargetNo:   input.EventCode + ":" + string(input.RecipientKind) + ":" + input.ChannelCode,
			ResultCode: "succeeded", ReasonCode: "merchant_event_policy_update", RiskCode: "normal",
			RequiredPermission: PermissionChannelManage,
			BeforeSnapshot:     map[string]any{"enabled": current.Enabled, "version": current.Version},
			AfterSnapshot:      map[string]any{"enabled": input.Enabled, "version": current.Version + 1},
		}); auditErr != nil {
			return auditErr
		}
		return s.database.Model(eventPolicyTable+" p").Ctx(ctx).
			InnerJoin("plugin_idc_notify_event_catalog e", "e.tenant_id=p.tenant_id AND e.event_code=p.event_code").
			Fields("p.event_code,e.topic_code,p.recipient_kind,p.channel_code,p.enabled,p.version,p.updated_at").
			Where("p.tenant_id", notificationTenantID).Where("p.event_code", input.EventCode).
			Where("p.recipient_kind", string(input.RecipientKind)).Where("p.channel_code", input.ChannelCode).
			Scan(&updated)
	})
	if err != nil {
		return nil, publicError(err)
	}
	item := projectEventPolicy(updated)
	return &item, nil
}

func projectEventPolicy(row eventPolicyRow) EventPolicy {
	return EventPolicy{EventCode: row.EventCode, Topic: row.TopicCode,
		RecipientKind: notificationcap.RecipientKind(row.RecipientKind), ChannelCode: row.ChannelCode,
		Enabled: row.Enabled, Version: row.Version, UpdatedAt: row.UpdatedAt.UTC()}
}

func validEventPolicyRecipient(value notificationcap.RecipientKind) bool {
	switch value {
	case notificationcap.RecipientCustomerContact, notificationcap.RecipientStaffUser, notificationcap.RecipientVerifiedDestination:
		return true
	default:
		return false
	}
}

func validEventPolicyChannel(_ notificationcap.RecipientKind, channel string) bool {
	return validChannelCode(channel)
}

var _ EventPolicyService = (*serviceImpl)(nil)
