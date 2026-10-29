// Current-user synchronization projects durable plugin intents into the host inbox.
package inbox

import (
	"context"
	"html"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	usermsgv1 "lina-core/api/usermsg/v1"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/notifycap"
	"lina-plugin-shared-services/backend/cap/inboxcap"
	"lina-plugin-shared-services/backend/internal/dao"
	modeldo "lina-plugin-shared-services/backend/internal/model/do"
	"lina-plugin-shared-services/backend/internal/service/sharedcode"
	"lina-plugin-shared-services/backend/internal/service/sharedutil"
)

const hostPublishedEvidence = "host_published"

// NewCurrentSync explicitly receives the host capabilities required for delivery.
// The database must be the host's default group so Send joins the same transaction.
func NewCurrentSync(database gdb.DB, notifications notifycap.Service, bizctx bizctxcap.Service) (*ServiceRuntime, error) {
	if database == nil || notifications == nil || bizctx == nil {
		return nil, gerror.New("shared inbox synchronization dependencies are required")
	}
	return &serviceImpl{database: database, notifications: notifications, bizctx: bizctx}, nil
}

type pendingMessage struct {
	MessageNo      string
	CallerPluginId string
	SourceType     string
	SourceNo       string
	ContentDigest  string
	Title          string
	Body           string
	Category       string
	ActionUrl      string
}

var _ inboxcap.CurrentSyncService = (*ServiceRuntime)(nil)

func (s *serviceImpl) currentRecipient(ctx context.Context) (int, error) {
	if s == nil || s.bizctx == nil || s.notifications == nil || ctx == nil || ctx.Err() != nil {
		return 0, bizerr.NewCode(sharedcode.CodeSourceNotAuthorized)
	}
	// Inbox ownership is strictly self-only; employee organization/data scopes
	// do not grant broader access and are not required on customer requests.
	current := s.bizctx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != 0 || current.ActingUserID != 0 || current.ActingAsTenant || current.IsImpersonation {
		return 0, bizerr.NewCode(sharedcode.CodeSourceNotAuthorized)
	}
	return current.UserID, nil
}

func (s *serviceImpl) pending(ctx context.Context, userID int) *gdb.Model {
	return s.database.Model(dao.InboxMessage.Table()).Ctx(ctx).Where(modeldo.InboxMessage{TenantId: 0, RecipientUserId: userID}).
		Where("NOT EXISTS (SELECT 1 FROM "+dao.InboxDeliveryEvidence.Table()+" evidence WHERE evidence.tenant_id="+dao.InboxMessage.Table()+".tenant_id AND evidence.message_no="+dao.InboxMessage.Table()+".message_no AND evidence.evidence_type=?)", hostPublishedEvidence)
}

// CurrentSync delivers only the real current user's intents; neither callers nor
// stored recipients can replace that identity. Host Send and evidence share a TX.
func (s *serviceImpl) CurrentSync(ctx context.Context, limit int) (*inboxcap.CurrentSyncResult, error) {
	userID, err := s.currentRecipient(ctx)
	if err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = inboxcap.MaximumCurrentSync
	}
	if limit < 1 || limit > inboxcap.MaximumCurrentSync {
		return nil, bizerr.NewCode(sharedcode.CodeInboxInvalid)
	}
	result := &inboxcap.CurrentSyncResult{}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var rows []pendingMessage
		cols := dao.InboxMessage.Columns()
		if scanErr := s.pending(txCtx, userID).Fields(cols.MessageNo, cols.CallerPluginId, cols.SourceType, cols.SourceNo, cols.ContentDigest, cols.Title, cols.Body, cols.Category, cols.ActionUrl).
			OrderAsc(cols.CreatedAt).OrderAsc(cols.Id).Limit(limit).LockUpdateSkipLocked().Scan(&rows); scanErr != nil {
			return scanErr
		}
		currentID, identityErr := s.currentRecipient(txCtx)
		if identityErr != nil {
			return identityErr
		}
		if currentID != userID {
			return bizerr.NewCode(sharedcode.CodeSourceNotAuthorized)
		}
		// A row lock alone does not refresh an earlier NOT EXISTS snapshot. Recheck
		// evidence after acquiring locks so a just-committed sender cannot duplicate.
		delivered := make(map[string]bool, len(rows))
		if len(rows) > 0 {
			messageNos := make([]string, 0, len(rows))
			for _, row := range rows {
				messageNos = append(messageNos, row.MessageNo)
			}
			var existing []struct{ MessageNo string }
			if readErr := s.database.Model(dao.InboxDeliveryEvidence.Table()).Ctx(txCtx).
				Where(modeldo.InboxDeliveryEvidence{TenantId: 0, EvidenceType: hostPublishedEvidence}).
				WhereIn(dao.InboxDeliveryEvidence.Columns().MessageNo, messageNos).Fields(dao.InboxDeliveryEvidence.Columns().MessageNo).Scan(&existing); readErr != nil {
				return readErr
			}
			for _, row := range existing {
				delivered[row.MessageNo] = true
			}
		}
		for _, row := range rows {
			if delivered[row.MessageNo] {
				continue
			}
			delivery, sendErr := s.notifications.Send(txCtx, notifycap.SendInput{
				Recipients: []string{strconv.Itoa(userID)}, SourceType: usermsgv1.SourceTypePlugin, SourceID: row.MessageNo,
				// The host preview renders HTML; stored intents and template variables are plain text.
				Title: row.Title, Content: strings.ReplaceAll(html.EscapeString(row.Body), "\n", "<br>"), Category: notifycap.CategoryCode(row.Category),
				Payload: map[string]any{"callerPluginId": row.CallerPluginId, "sourceType": row.SourceType, "sourceNo": row.SourceNo, "actionUrl": row.ActionUrl},
			})
			if sendErr != nil {
				return sendErr
			}
			if delivery == nil || delivery.DeliveryCount != 1 {
				return gerror.New("host inbox delivery acknowledgment is invalid")
			}
			hostID, parseErr := strconv.ParseInt(string(delivery.MessageID), 10, 64)
			if parseErr != nil || hostID <= 0 {
				return gerror.New("host inbox message identifier is invalid")
			}
			evidenceNo, tokenErr := sharedutil.RandomToken(24)
			if tokenErr != nil {
				return tokenErr
			}
			if _, insertErr := s.database.Model(dao.InboxDeliveryEvidence.Table()).Ctx(txCtx).Data(modeldo.InboxDeliveryEvidence{
				TenantId: 0, EvidenceNo: evidenceNo, MessageNo: row.MessageNo, CallerPluginId: row.CallerPluginId,
				EvidenceType: hostPublishedEvidence, ContentDigest: row.ContentDigest, HostMessageId: string(delivery.MessageID),
			}).Insert(); insertErr != nil {
				return insertErr
			}
			result.SyncedCount++
		}
		result.PendingCount, err = s.pending(txCtx, userID).Count()
		return err
	})
	if err != nil {
		return nil, inboxPublicError(err)
	}
	return result, nil
}
