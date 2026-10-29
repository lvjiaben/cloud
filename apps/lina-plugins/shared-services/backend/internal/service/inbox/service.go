package inbox

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capregistry"
	"lina-core/pkg/plugin/capability/notifycap"
	"lina-plugin-shared-services/backend/cap/inboxcap"
	modeldo "lina-plugin-shared-services/backend/internal/model/do"
	"lina-plugin-shared-services/backend/internal/service/sharedcode"
	"lina-plugin-shared-services/backend/internal/service/sharedutil"
)

const (
	messageTable  = "plugin_shared_services_inbox_message"
	evidenceTable = "plugin_shared_services_inbox_delivery_evidence"
)

var callerPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,127}$`)

type messageRow struct {
	MessageNo     string    `orm:"message_no"`
	ContentDigest string    `orm:"content_digest"`
	CreatedAt     time.Time `orm:"created_at"`
}

type evidenceRow struct {
	EvidenceNo string `orm:"evidence_no"`
}

type serviceImpl struct {
	database      gdb.DB
	notifications notifycap.Service
	bizctx        bizctxcap.Service
}

// ServiceRuntime is the owner-composed runtime used only by plugin startup.
type ServiceRuntime = serviceImpl

type boundClient struct {
	service *serviceImpl
	caller  string
}

var _ inboxcap.Service = (*boundClient)(nil)
var _ capregistry.Invoker = (*Invoker)(nil)

func New(database gdb.DB) (*serviceImpl, error) {
	if database == nil {
		return nil, gerror.New("shared inbox database is required")
	}
	return &serviceImpl{database: database}, nil
}

func Bind(service *serviceImpl, caller string) (inboxcap.Service, error) {
	if service == nil || !callerPattern.MatchString(caller) || caller == inboxcap.OwnerPluginID {
		return nil, gerror.New("shared inbox caller binding is invalid")
	}
	return &boundClient{service: service, caller: caller}, nil
}

func (c *boundClient) Publish(ctx context.Context, input inboxcap.PublishRequest) (*inboxcap.PublishResult, error) {
	return c.service.publish(ctx, c.caller, input)
}

func (s *serviceImpl) publish(ctx context.Context, caller string, input inboxcap.PublishRequest) (*inboxcap.PublishResult, error) {
	if err := inboxcap.ValidatePublishRequest(input); err != nil {
		return nil, bizerr.WrapCode(err, sharedcode.CodeInboxInvalid)
	}
	encoded, err := json.Marshal(input)
	if err != nil || len(encoded) > inboxcap.MaximumPayloadBytes {
		return nil, bizerr.NewCode(sharedcode.CodeInboxInvalid)
	}
	digest := sharedutil.Digest(string(encoded))
	messageNo, err := sharedutil.RandomToken(24)
	if err != nil {
		return nil, inboxUnavailable(err)
	}
	evidenceNo, err := sharedutil.RandomToken(24)
	if err != nil {
		return nil, inboxUnavailable(err)
	}
	result := &inboxcap.PublishResult{}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		insert, insertErr := s.database.Model(messageTable).Ctx(txCtx).Data(modeldo.InboxMessage{
			TenantId: 0, MessageNo: messageNo, RecipientUserId: input.RecipientUserID,
			CallerPluginId: caller, SourceType: input.SourceType, SourceNo: input.SourceNo,
			IdempotencyKey: input.IdempotencyKey, ContentDigest: digest, Title: input.Title,
			Body: input.Body, Category: input.Category, ActionUrl: nullableString(input.ActionURL), Status: "published",
		}).InsertIgnore()
		if insertErr != nil {
			return insertErr
		}
		affected, affectedErr := insert.RowsAffected()
		if affectedErr != nil {
			return affectedErr
		}
		if affected == 1 {
			if _, insertErr = s.database.Model(evidenceTable).Ctx(txCtx).Data(modeldo.InboxDeliveryEvidence{
				TenantId: 0, EvidenceNo: evidenceNo, MessageNo: messageNo, CallerPluginId: caller,
				EvidenceType: "published", ContentDigest: digest,
			}).Insert(); insertErr != nil {
				return insertErr
			}
			var created messageRow
			if loadErr := s.database.Model(messageTable).Ctx(txCtx).Where("tenant_id", 0).
				Where("message_no", messageNo).Scan(&created); loadErr != nil {
				return loadErr
			}
			result = &inboxcap.PublishResult{MessageNo: messageNo, EvidenceNo: evidenceNo, PublishedAt: created.CreatedAt}
			return nil
		}
		var existing messageRow
		if loadErr := s.database.Model(messageTable).Ctx(txCtx).Where("tenant_id", 0).
			Where("caller_plugin_id", caller).Where("recipient_user_id", input.RecipientUserID).
			Where("source_type", input.SourceType).Where("source_no", input.SourceNo).
			Where("idempotency_key", input.IdempotencyKey).LockUpdate().Scan(&existing); loadErr != nil {
			return loadErr
		}
		if existing.MessageNo == "" || existing.ContentDigest != digest {
			return bizerr.NewCode(sharedcode.CodeInboxConflict)
		}
		var evidence evidenceRow
		if loadErr := s.database.Model(evidenceTable).Ctx(txCtx).Where("tenant_id", 0).
			Where("message_no", existing.MessageNo).Where("evidence_type", "published").
			OrderAsc("id").Limit(1).Scan(&evidence); loadErr != nil {
			return loadErr
		}
		if evidence.EvidenceNo == "" {
			return gerror.New("shared inbox evidence is missing")
		}
		result = &inboxcap.PublishResult{MessageNo: existing.MessageNo, EvidenceNo: evidence.EvidenceNo,
			PublishedAt: existing.CreatedAt, Reused: true}
		return nil
	})
	if err != nil {
		return nil, inboxPublicError(err)
	}
	return result, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func inboxPublicError(err error) error {
	for _, code := range []*bizerr.Code{sharedcode.CodeInboxInvalid, sharedcode.CodeInboxConflict, sharedcode.CodeSourceNotAuthorized} {
		if bizerr.Is(err, code) {
			return err
		}
	}
	return inboxUnavailable(err)
}

func inboxUnavailable(err error) error {
	return bizerr.WrapCode(gerror.Wrap(err, "shared inbox operation failed"), sharedcode.CodeInboxUnavailable)
}
