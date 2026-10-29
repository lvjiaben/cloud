// This file queries a bounded merchant work queue and publishes one immutable
// summary intent per observed work set, without enumerating or impersonating staff.

package renewalnotice

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	workStatusOpen          = "open"
	merchantAllDataScope    = 1
	merchantTenantDataScope = 2
	maximumPageSize         = 50
	attentionEvent          = "billing.accessory_renewal_attention"
)

// currentStaff rejects delegation and unsupported merchant scopes before any queue read.
func (s *serviceImpl) currentStaff(ctx context.Context) (int, error) {
	current := s.bizCtx.Current(ctx)
	// The domain queue is merchant-wide. Department/self scopes have no defined
	// customer ownership mapping and must not silently become merchant-wide.
	if current.UserID <= 0 || current.TenantID != 0 || current.ActingUserID != 0 ||
		current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported ||
		(current.DataScope != merchantAllDataScope && current.DataScope != merchantTenantDataScope) {
		return 0, bizerr.NewCode(CodeDenied)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive || audience.CustomerNo != "" {
		return 0, bizerr.NewCode(CodeDenied)
	}
	grants, err := s.authorization.BatchHasPermissions(ctx, []authz.PermissionKey{authz.PermissionKey(Permission), authz.PermissionKey(idccap.HostPermissionWildcard)})
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeDenied)
	}
	admin, err := s.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeDenied)
	}
	if !admin && !grants[authz.PermissionKey(Permission)] && !grants[authz.PermissionKey(idccap.HostPermissionWildcard)] {
		return 0, bizerr.NewCode(CodeDenied)
	}
	return current.UserID, nil
}

func (s *serviceImpl) List(ctx context.Context, page, pageSize int) (*Page, error) {
	if page < 1 || page > 1000000 || pageSize < 1 || pageSize > maximumPageSize {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.currentStaff(ctx); err != nil {
		return nil, err
	}
	result, err := s.readPage(ctx, page, pageSize)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}

// readPage reads count and items from one independent read-only snapshot.
// Its fixed two queries share the queue index and never look up related rows.
func (s *serviceImpl) readPage(ctx context.Context, page, pageSize int) (*Page, error) {
	result := &Page{Items: make([]Item, 0)}
	err := s.database.TransactionWithOptions(ctx, gdb.TxOptions{
		Isolation: sql.LevelRepeatableRead, ReadOnly: true, Propagation: gdb.PropagationRequiresNew,
	}, func(ctx context.Context, _ gdb.TX) error {
		columns := dao.ServerAccessoryRenewalWorkItem.Columns()
		model := s.database.Model(dao.ServerAccessoryRenewalWorkItem.Table()).Ctx(ctx).Where(do.ServerAccessoryRenewalWorkItem{
			TenantId: idccap.DefaultTenantID, RequiredPermission: Permission, StatusCode: workStatusOpen,
		})
		count, err := model.Clone().Count()
		if err != nil {
			return err
		}
		result.Total = count
		return model.Fields(columns.WorkItemNo, columns.InvoiceNo, columns.CustomerNo, columns.SubjectNo, columns.ReasonCode, columns.CreatedAt).
			OrderDesc(columns.CreatedAt).OrderDesc(columns.Id).Page(page, pageSize).Scan(&result.Items)
	})
	return result, err
}

func (s *serviceImpl) Sync(ctx context.Context, locale string) (*Synced, error) {
	if locale != "zh-CN" && locale != "en-US" {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	userID, err := s.currentStaff(ctx)
	if err != nil {
		return nil, err
	}
	page, err := s.readPage(ctx, 1, maximumPageSize)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	// Recheck current authority after the snapshot read. This does not claim to
	// lock host permissions; it rejects revoked or changed identities before enqueue.
	currentUserID, err := s.currentStaff(ctx)
	if err != nil {
		return nil, err
	}
	if currentUserID != userID {
		return nil, bizerr.NewCode(CodeDenied)
	}
	result := &Synced{Total: page.Total}
	if page.Total == 0 || len(page.Items) == 0 {
		return s.syncCurrentInbox(ctx, result)
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		// Persist the complete rendered payload's digest in the deduplication key.
		// Tabs and instances observing the same set cannot duplicate its event;
		// changes to count, invoice, locale or this newest bounded page create a new
		// immutable event. Created-at/id ordering puts newly queued work first.
		payload, encodeErr := json.Marshal(struct {
			UserID int
			Locale string
			Page   *Page
		}{userID, locale, page})
		if encodeErr != nil {
			return encodeErr
		}
		digest := sha256.Sum256(payload)
		receipt, enqueueErr := s.events.Enqueue(ctx, notificationcap.EventRequest{
			EventCode:          attentionEvent,
			Recipient:          notificationcap.Recipient{Kind: notificationcap.RecipientStaffUser, StaffUserID: userID},
			BusinessObjectType: notificationcap.BusinessObjectBilling, BusinessObjectNo: page.Items[0].InvoiceNo,
			Variables:     map[string]string{"pending_count": strconv.Itoa(page.Total), "invoice_no": page.Items[0].InvoiceNo, "locale_code": locale},
			CorrelationNo: page.Items[0].WorkItemNo, IdempotencyKey: "accessory-attention:" + hex.EncodeToString(digest[:]),
		})
		if enqueueErr != nil {
			return enqueueErr
		}
		result.EventNo = receipt.EventNo
		return nil
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.syncCurrentInbox(ctx, result)
}

// syncCurrentInbox runs after the outbox transaction, using the actual staff
// context. It also drains already-materialized messages when no work remains.
// A new event may materialize later; the next online/manual sync then delivers it.
func (s *serviceImpl) syncCurrentInbox(ctx context.Context, result *Synced) (*Synced, error) {
	if _, err := s.inbox.CurrentSync(ctx, maximumPageSize); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}
