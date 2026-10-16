// Package financeops owns staff-only financial convenience operations that
// compose billing, notification, audit, and private-storage owners.
package financeops

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

var (
	CodeInvalid       = bizerr.MustDefine("IDC_FINANCIAL_OPERATION_INVALID", "The financial operation is invalid", gcode.CodeInvalidParameter)
	CodeNotAccessible = bizerr.MustDefine("IDC_FINANCIAL_OPERATION_NOT_ACCESSIBLE", "The financial operation is not accessible", gcode.CodeNotAuthorized)
	CodeConflict      = bizerr.MustDefine("IDC_FINANCIAL_OPERATION_CONFLICT", "The financial operation conflicts with current facts", gcode.CodeInvalidOperation)
	CodeUnavailable   = bizerr.MustDefine("IDC_FINANCIAL_OPERATION_UNAVAILABLE", "The financial operation is temporarily unavailable", gcode.CodeInternalError)
)

type InvoiceDeliveryPreview struct {
	InvoiceNo, CustomerNo, ContactNo, MaskedEmail string
	InvoiceVersion                                int64
}

type InvoiceDeliveryResult struct {
	IntentNo, InvoiceNo, NotificationEventNo string
	CreatedAt                                time.Time
	Reused                                   bool
}

type InternalNote struct {
	NoteNo, TargetType, TargetNo, Content, CorrectsNoteNo string
	CreatedBy                                             int
	CreatedAt                                             time.Time
}

type AddNoteInput struct {
	TargetType, TargetNo, Content, CorrectsNoteNo, IdempotencyKey string
}

type SavedFilter struct {
	FilterNo, ViewCode, DisplayName string
	Filter                          json.RawMessage
	Version                         int64
	CreatedAt, UpdatedAt            time.Time
}

type SaveFilterInput struct {
	ViewCode, DisplayName string
	Filter                json.RawMessage
}

type Service interface {
	PreviewInvoiceDelivery(context.Context, string) (*InvoiceDeliveryPreview, error)
	ResendInvoice(context.Context, string, string) (*InvoiceDeliveryResult, error)
	ListInternalNotes(context.Context, string, string, int) ([]InternalNote, error)
	AddInternalNote(context.Context, AddNoteInput) (*InternalNote, error)
	ListSavedFilters(context.Context, string) ([]SavedFilter, error)
	SavePersonalFilter(context.Context, SaveFilterInput) (*SavedFilter, error)
	CreateExport(context.Context, ExportRequestInput) (*ExportStatus, error)
	GenerateExport(context.Context, string) (*ExportStatus, error)
	GetExportStatus(context.Context, string) (*ExportStatus, error)
	IssueExportTicket(context.Context, string) (*ExportTicket, error)
	OpenExport(context.Context, string) (*ExportDownload, error)
}

// Authorizer combines the ordinary financial-list checks with the staff-only
// operation checks. Requiring the complete boundary at construction time keeps
// exports fail-closed instead of discovering a missing permission adapter only
// after a request has been created.
type Authorizer interface {
	billing.ManagementFinanceAuthorizer
	billing.ManagementFinanceOperationsAuthorizer
}

type serviceImpl struct {
	database      gdb.DB
	authorizer    Authorizer
	audit         idccap.BusinessAuditWriter
	notifications notificationcap.EventSink
	storage       storagecap.Service
	random        io.Reader
}

func New(database gdb.DB, authorizer Authorizer, audit idccap.BusinessAuditWriter, notifications notificationcap.EventSink, storage storagecap.Service) (Service, error) {
	if database == nil || authorizer == nil || audit == nil || notifications == nil || storage == nil {
		return nil, gerror.New("financial operations require database, authorization, audit, notification, and storage services")
	}
	return &serviceImpl{database: database, authorizer: authorizer, audit: audit, notifications: notifications, storage: storage, random: rand.Reader}, nil
}

func (s *serviceImpl) PreviewInvoiceDelivery(ctx context.Context, invoiceNo string) (*InvoiceDeliveryPreview, error) {
	if _, err := s.authorizer.AuthorizeInvoiceResend(ctx, invoiceNo); err != nil {
		return nil, err
	}
	row, err := s.loadInvoiceContact(ctx, invoiceNo)
	if err != nil || row == nil {
		return nil, inaccessible(err)
	}
	masked := maskEmail(row.Email)
	clearBytes := []byte(row.Email)
	clear(clearBytes)
	return &InvoiceDeliveryPreview{InvoiceNo: row.InvoiceNo, CustomerNo: row.CustomerNo, ContactNo: row.ContactNo, MaskedEmail: masked, InvoiceVersion: row.InvoiceVersion}, nil
}

type invoiceContactRow struct {
	InvoiceNo, CustomerNo, ContactNo, Email string
	InvoiceVersion                          int64
}

func (s *serviceImpl) loadInvoiceContact(ctx context.Context, invoiceNo string) (*invoiceContactRow, error) {
	if !validReference(invoiceNo, 40) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var row *invoiceContactRow
	err := s.database.GetScan(ctx, &row, `
SELECT invoice.invoice_no,invoice.customer_no,invoice.version AS invoice_version,
       contact.contact_no,contact.email
FROM plugin_idc_core_billing_invoice invoice
JOIN plugin_idc_core_customer_contact_role role
  ON role.tenant_id=invoice.tenant_id AND role.customer_no=invoice.customer_no
 AND role.role_code='billing' AND role.status_code='active'
JOIN plugin_idc_core_customer_contact contact
  ON contact.tenant_id=role.tenant_id AND contact.customer_no=role.customer_no
 AND contact.contact_no=role.contact_no AND contact.status_code='active'
 AND contact.email<>'' AND contact.email_verified_at IS NOT NULL
WHERE invoice.tenant_id=0 AND invoice.invoice_no=?`, invoiceNo)
	return row, err
}

func (s *serviceImpl) ResendInvoice(ctx context.Context, invoiceNo, idempotencyKey string) (*InvoiceDeliveryResult, error) {
	actor, err := s.authorizer.AuthorizeInvoiceResend(ctx, invoiceNo)
	if err != nil || !validIdempotency(idempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	idempotencyDigest := digestString(idempotencyKey)
	var result *InvoiceDeliveryResult
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		contact, loadErr := s.loadInvoiceContact(ctx, invoiceNo)
		if loadErr != nil || contact == nil {
			return inaccessible(loadErr)
		}
		requestDigest, digestErr := digestJSON(struct {
			InvoiceNo, CustomerNo, ContactNo string
			Version                          int64
		}{contact.InvoiceNo, contact.CustomerNo, contact.ContactNo, contact.InvoiceVersion})
		if digestErr != nil {
			return digestErr
		}
		var replay *struct {
			IntentNo, InvoiceNo, EventNo, RequestDigest string
			CreatedAt                                   time.Time
		}
		if loadErr = s.database.GetScan(ctx, &replay, `SELECT intent_no,invoice_no,notification_event_no AS event_no,request_digest,created_at FROM plugin_idc_core_financial_delivery_intent WHERE tenant_id=0 AND invoice_no=? AND idempotency_digest=?`, invoiceNo, idempotencyDigest); loadErr != nil {
			return loadErr
		}
		if replay != nil {
			if replay.RequestDigest != requestDigest {
				return bizerr.NewCode(CodeConflict)
			}
			result = &InvoiceDeliveryResult{IntentNo: replay.IntentNo, InvoiceNo: replay.InvoiceNo, NotificationEventNo: replay.EventNo, CreatedAt: replay.CreatedAt.UTC(), Reused: true}
			return nil
		}
		intentNo, numberErr := newNumber(s.random, "FDI_")
		if numberErr != nil {
			return numberErr
		}
		receipt, enqueueErr := s.notifications.Enqueue(ctx, notificationcap.EventRequest{
			EventCode:          "billing.invoice.resend",
			Recipient:          notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact, CustomerNo: contact.CustomerNo, ContactNo: contact.ContactNo},
			BusinessObjectType: notificationcap.BusinessObjectBilling, BusinessObjectNo: contact.InvoiceNo, BusinessObjectVersion: contact.InvoiceVersion,
			Variables: map[string]string{"invoice_no": contact.InvoiceNo}, CorrelationNo: intentNo, IdempotencyKey: intentNo,
		})
		if enqueueErr != nil {
			return enqueueErr
		}
		createdAt, timeErr := databaseNow(ctx, s.database)
		if timeErr != nil {
			return timeErr
		}
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_financial_delivery_intent (tenant_id,intent_no,invoice_no,customer_no,contact_no,invoice_version,notification_event_no,requested_by,idempotency_digest,request_digest,created_at) VALUES (0,?,?,?,?,?,?,?,?,?,?)`, intentNo, contact.InvoiceNo, contact.CustomerNo, contact.ContactNo, contact.InvoiceVersion, receipt.EventNo, actor, idempotencyDigest, requestDigest, createdAt); insertErr != nil {
			return insertErr
		}
		if auditErr := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"intent_no": ""}, IdempotencyScope: "billing.invoice.resend", IdempotencyKey: idempotencyKey, OperationCode: "billing.invoice.resend", TargetType: "billing_invoice", TargetNo: contact.InvoiceNo, CustomerNo: contact.CustomerNo, ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: "staff_resend", RiskCode: "medium", RequiredPermission: "idc:admin:billing:invoice:resend", CorrelationNo: intentNo, AfterSnapshot: map[string]any{"state": "notification_requested", "invoice_version": contact.InvoiceVersion, "contact_no": contact.ContactNo}}); auditErr != nil {
			return auditErr
		}
		result = &InvoiceDeliveryResult{IntentNo: intentNo, InvoiceNo: contact.InvoiceNo, NotificationEventNo: receipt.EventNo, CreatedAt: createdAt.UTC()}
		return nil
	})
	return result, normalize(err)
}

func (s *serviceImpl) ListInternalNotes(ctx context.Context, targetType, targetNo string, limit int) ([]InternalNote, error) {
	if _, err := s.authorizer.AuthorizeInternalNoteList(ctx, targetNo); err != nil {
		return nil, err
	}
	if !validTarget(targetType, targetNo) || limit < 1 || limit > 100 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var rows []InternalNote
	if err := s.database.GetScan(ctx, &rows, `SELECT note_no,target_type_code AS target_type,target_no,content,COALESCE(corrects_note_no,'') AS corrects_note_no,created_by,created_at FROM plugin_idc_core_financial_internal_note WHERE tenant_id=0 AND target_type_code=? AND target_no=? ORDER BY created_at DESC,id DESC LIMIT ?`, targetType, targetNo, limit); err != nil {
		return nil, normalize(err)
	}
	return rows, nil
}

func (s *serviceImpl) AddInternalNote(ctx context.Context, input AddNoteInput) (*InternalNote, error) {
	actor, err := s.authorizer.AuthorizeInternalNoteCreate(ctx, input.TargetNo)
	input.Content = strings.TrimSpace(input.Content)
	if err != nil {
		return nil, err
	}
	if !validTarget(input.TargetType, input.TargetNo) || len(input.Content) < 1 || len(input.Content) > 2000 || !validIdempotency(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	idempotencyDigest := digestString(input.IdempotencyKey)
	requestDigest, err := digestJSON(input)
	if err != nil {
		return nil, normalize(err)
	}
	var result *InternalNote
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		customerNo, targetErr := s.resolveTargetCustomer(ctx, input.TargetType, input.TargetNo)
		if targetErr != nil || customerNo == "" {
			return inaccessible(targetErr)
		}
		var replay *struct {
			InternalNote
			RequestDigest string
		}
		if loadErr := s.database.GetScan(ctx, &replay, `SELECT note_no,target_type_code AS target_type,target_no,content,COALESCE(corrects_note_no,'') AS corrects_note_no,created_by,created_at,request_digest FROM plugin_idc_core_financial_internal_note WHERE tenant_id=0 AND created_by=? AND target_type_code=? AND target_no=? AND idempotency_digest=?`, actor, input.TargetType, input.TargetNo, idempotencyDigest); loadErr != nil {
			return loadErr
		}
		if replay != nil {
			if replay.RequestDigest != requestDigest {
				return bizerr.NewCode(CodeConflict)
			}
			value := replay.InternalNote
			result = &value
			return nil
		}
		if input.CorrectsNoteNo != "" {
			value, loadErr := s.database.GetValue(ctx, `SELECT COUNT(1) FROM plugin_idc_core_financial_internal_note WHERE tenant_id=0 AND note_no=? AND target_type_code=? AND target_no=?`, input.CorrectsNoteNo, input.TargetType, input.TargetNo)
			if loadErr != nil || value.Int() != 1 {
				return bizerr.NewCode(CodeInvalid)
			}
		}
		noteNo, numberErr := newNumber(s.random, "FNT_")
		if numberErr != nil {
			return numberErr
		}
		createdAt, timeErr := databaseNow(ctx, s.database)
		if timeErr != nil {
			return timeErr
		}
		var correction any
		if input.CorrectsNoteNo != "" {
			correction = input.CorrectsNoteNo
		}
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_financial_internal_note (tenant_id,note_no,target_type_code,target_no,customer_no,content,corrects_note_no,created_by,idempotency_digest,request_digest,created_at) VALUES (0,?,?,?,?,?,?,?,?,?,?)`, noteNo, input.TargetType, input.TargetNo, customerNo, input.Content, correction, actor, idempotencyDigest, requestDigest, createdAt); insertErr != nil {
			return insertErr
		}
		if auditErr := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"note_no": ""}, IdempotencyScope: "billing.internal_note.append", IdempotencyKey: input.IdempotencyKey, OperationCode: "billing.internal_note.append", TargetType: input.TargetType, TargetNo: input.TargetNo, CustomerNo: customerNo, ResultCode: "succeeded", ReasonCode: "staff_internal_note", RiskCode: "low", RequiredPermission: "idc:admin:billing:note:create", CorrelationNo: noteNo, AfterSnapshot: map[string]any{"state": "appended", "corrects_note_no": input.CorrectsNoteNo}}); auditErr != nil {
			return auditErr
		}
		result = &InternalNote{NoteNo: noteNo, TargetType: input.TargetType, TargetNo: input.TargetNo, Content: input.Content, CorrectsNoteNo: input.CorrectsNoteNo, CreatedBy: actor, CreatedAt: createdAt.UTC()}
		return nil
	})
	return result, normalize(err)
}

func (s *serviceImpl) resolveTargetCustomer(ctx context.Context, targetType, targetNo string) (string, error) {
	query := map[string]string{
		"order":   `SELECT customer_no FROM plugin_idc_core_commerce_order WHERE tenant_id=0 AND order_no=?`,
		"invoice": `SELECT customer_no FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND invoice_no=?`,
		"service": `SELECT customer_no FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND entitlement_no=?`,
	}[targetType]
	if query == "" {
		return "", bizerr.NewCode(CodeInvalid)
	}
	value, err := s.database.GetValue(ctx, query, targetNo)
	return value.String(), err
}

func (s *serviceImpl) ListSavedFilters(ctx context.Context, viewCode string) ([]SavedFilter, error) {
	actor, err := s.authorizer.AuthorizePersonalFilter(ctx)
	if err != nil {
		return nil, err
	}
	if !validView(viewCode) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var rows []SavedFilter
	if err = s.database.GetScan(ctx, &rows, `SELECT filter_no,view_code,display_name,filter_json AS filter,version,created_at,updated_at FROM plugin_idc_core_financial_saved_filter WHERE tenant_id=0 AND owner_user_id=? AND view_code=? ORDER BY updated_at DESC,id DESC LIMIT 100`, actor, viewCode); err != nil {
		return nil, normalize(err)
	}
	return rows, nil
}

func (s *serviceImpl) SavePersonalFilter(ctx context.Context, input SaveFilterInput) (*SavedFilter, error) {
	actor, err := s.authorizer.AuthorizePersonalFilter(ctx)
	if err != nil {
		return nil, err
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	filter, err := normalizeJSONObject(input.Filter)
	if err != nil || !validView(input.ViewCode) || len(input.DisplayName) < 1 || len(input.DisplayName) > 80 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	filterNo, err := newNumber(s.random, "FSF_")
	if err != nil {
		return nil, normalize(err)
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, normalize(err)
	}
	var result *SavedFilter
	err = s.database.GetScan(ctx, &result, `INSERT INTO plugin_idc_core_financial_saved_filter (tenant_id,filter_no,owner_user_id,view_code,display_name,filter_json,filter_digest,version,created_at,updated_at) VALUES (0,?,?,?,?,?,?,1,?,?) ON CONFLICT (owner_user_id,view_code,display_name) DO UPDATE SET filter_json=EXCLUDED.filter_json,filter_digest=EXCLUDED.filter_digest,version=plugin_idc_core_financial_saved_filter.version+1,updated_at=EXCLUDED.updated_at RETURNING filter_no,view_code,display_name,filter_json AS filter,version,created_at,updated_at`, filterNo, actor, input.ViewCode, input.DisplayName, string(filter), digestString(string(filter)), now, now)
	return result, normalize(err)
}

func validReference(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	if len(value) < 1 || len(value) > maximum {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && !strings.ContainsRune("_.:-", r) {
			return false
		}
	}
	return true
}
func validIdempotency(value string) bool { return validReference(value, 128) && len(value) >= 16 }
func validTarget(kind, value string) bool {
	return (kind == "order" || kind == "invoice" || kind == "service") && validReference(value, 40)
}
func validView(value string) bool {
	return value == "invoice" || value == "transaction" || value == "adjustment" || value == "receivable"
}
func digestString(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
func digestJSON(value any) (string, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return digestString(string(content)), nil
}
func normalizeJSONObject(value json.RawMessage) (json.RawMessage, error) {
	if len(value) < 2 || len(value) > 8192 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var object map[string]any
	if err := json.Unmarshal(value, &object); err != nil || object == nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	normalized, err := json.Marshal(object)
	return normalized, err
}
func newNumber(reader io.Reader, prefix string) (string, error) {
	raw := make([]byte, 16)
	if _, err := io.ReadFull(reader, raw); err != nil {
		return "", err
	}
	value := prefix + hex.EncodeToString(raw)
	clear(raw)
	return value, nil
}
func databaseNow(ctx context.Context, database gdb.DB) (time.Time, error) {
	value, err := database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return time.Time{}, err
	}
	now := value.Time()
	if now.IsZero() {
		return time.Time{}, gerror.New("database time is unavailable")
	}
	return now, nil
}
func maskEmail(value string) string {
	parts := strings.Split(value, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "***"
	}
	local := []rune(parts[0])
	visible := string(local[0])
	if len(local) > 1 {
		visible += "***"
	}
	domain := parts[1]
	dot := strings.Index(domain, ".")
	if dot <= 0 {
		return visible + "@***"
	}
	return visible + "@" + string([]rune(domain)[0]) + "***" + domain[dot:]
}
func inaccessible(err error) error {
	if err == nil {
		return bizerr.NewCode(CodeNotAccessible)
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeNotAccessible)
}
func normalize(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeUnavailable)
}
