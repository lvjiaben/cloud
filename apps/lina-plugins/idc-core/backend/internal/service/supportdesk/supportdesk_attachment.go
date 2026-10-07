package supportdesk

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
)

func (s *serviceImpl) ReserveStaffAttachment(ctx context.Context, input AttachmentUploadInput) (*AttachmentReservation, error) {
	if input.Context != "public_reply" && input.Context != "internal_note" {
		return nil, bizInvalid()
	}
	permission := PermissionReply
	if input.Context == "internal_note" {
		permission = PermissionNote
	}
	a, err := s.staffActor(ctx, permission)
	if err != nil {
		return nil, err
	}
	return s.reserveAttachment(ctx, input, a, "", "staff", "")
}
func (s *serviceImpl) ReserveStaffInitialAttachment(ctx context.Context, input AttachmentUploadInput) (*AttachmentReservation, error) {
	a, err := s.staffActor(ctx, PermissionCreate)
	if err != nil {
		return nil, err
	}
	input.TicketNo = ""
	input.Context = "internal_initial"
	return s.reserveAttachment(ctx, input, a, "", "staff", "")
}
func (s *serviceImpl) ReserveAccountAttachment(ctx context.Context, input AttachmentUploadInput) (*AttachmentReservation, error) {
	a, err := s.customerActor(ctx)
	if err != nil {
		return nil, err
	}
	input.Context = "customer_reply"
	return s.reserveAttachment(ctx, input, nil, a.CustomerNo, "customer", "")
}
func (s *serviceImpl) ReserveAccountInitialAttachment(ctx context.Context, input AttachmentUploadInput) (*AttachmentReservation, error) {
	a, err := s.customerActor(ctx)
	if err != nil {
		return nil, err
	}
	input.TicketNo = ""
	input.Context = "customer_initial"
	return s.reserveAttachment(ctx, input, nil, a.CustomerNo, "customer", "")
}

func (s *serviceImpl) ReserveAnonymousAttachment(ctx context.Context, accessToken string, input AttachmentUploadInput) (*AttachmentReservation, error) {
	ticket, err := s.anonymousTicket(ctx, accessToken, false)
	if err != nil {
		return nil, err
	}
	input.TicketNo = ticket.TicketNo
	input.Context = "anonymous_reply"
	return s.reserveAttachment(ctx, input, nil, "", "anonymous", accessToken)
}

func (s *serviceImpl) reserveAttachment(ctx context.Context, input AttachmentUploadInput, staff *actor, customerNo, uploaderType, anonymousToken string) (*AttachmentReservation, error) {
	fileName, ok := normalizeText(filepath.Base(input.FileName), 255)
	initial := input.Context == "internal_initial" || input.Context == "customer_initial"
	if !ok || input.SizeBytes < 1 || input.SizeBytes > maxAttachmentBytes || (initial && input.TicketNo != "") || (!initial && !businessNoPattern.MatchString(input.TicketNo)) {
		return nil, bizInvalid()
	}
	uploadNo, err := s.newNo("TUP_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate attachment upload")
	}
	actorUserID, err := s.attachmentActorUserID(ctx, staff, uploaderType)
	if err != nil {
		return nil, err
	}
	var result *AttachmentReservation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if !initial {
			ticket, loadErr := s.authorizeAttachmentTicket(ctx, input.TicketNo, staff, customerNo, uploaderType, anonymousToken, true)
			if loadErr != nil {
				return loadErr
			}
			if ticket.BaseStage == "closed" {
				return bizerr.NewCode(CodeConflict)
			}
			used, sumErr := s.database.GetValue(ctx, `SELECT COALESCE((SELECT SUM(size_bytes) FROM plugin_idc_core_support_ticket_attachment WHERE tenant_id=0 AND ticket_no=?),0)+COALESCE((SELECT SUM(declared_size_bytes) FROM plugin_idc_core_support_ticket_attachment_upload WHERE tenant_id=0 AND ticket_no=? AND status_code IN ('reserved','ready')),0)`, input.TicketNo, input.TicketNo)
			if sumErr != nil {
				return sumErr
			}
			if used.Int64()+input.SizeBytes > maxTicketUploadBytes {
				return bizerr.NewCode(CodeAttachmentRejected)
			}
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		expires := now.Add(time.Hour)
		objectPath := "support/attachment-uploads/" + uploadNo
		_, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_attachment_upload (tenant_id,upload_no,ticket_no,uploader_user_id,uploader_type,context_code,file_name,declared_size_bytes,temporary_object_ref,status_code,expires_at,created_at,updated_at) VALUES (0,?,?,?,?,?,?,?,?,'reserved',?,?,?)`, uploadNo, nullable(input.TicketNo), actorUserID, uploaderType, input.Context, fileName, input.SizeBytes, objectPath, expires, now, now)
		if insertErr != nil {
			return insertErr
		}
		result = &AttachmentReservation{UploadNo: uploadNo, TicketNo: input.TicketNo, FileName: fileName, Context: input.Context, SizeBytes: input.SizeBytes, NextOffset: 0, ExpiresAt: expires, Version: 1}
		return nil
	})
	if err != nil {
		return nil, wrapUnavailable(err, "reserve ticket attachment")
	}
	return result, nil
}

func (s *serviceImpl) UploadStaffAttachment(ctx context.Context, uploadNo, contentType string, body io.Reader) (*AttachmentUploadResult, error) {
	contextCode, err := s.staffUploadContext(ctx, uploadNo)
	if err != nil {
		return nil, err
	}
	permission := PermissionReply
	if contextCode == "internal_note" {
		permission = PermissionNote
	} else if contextCode == "internal_initial" {
		permission = PermissionCreate
	} else if contextCode != "public_reply" {
		return nil, bizNotAccessible()
	}
	a, err := s.staffActor(ctx, permission)
	if err != nil {
		return nil, err
	}
	return s.uploadAttachment(ctx, uploadNo, contentType, body, a, "", "staff", "")
}

func (s *serviceImpl) staffUploadContext(ctx context.Context, uploadNo string) (string, error) {
	if !businessNoPattern.MatchString(uploadNo) {
		return "", bizNotAccessible()
	}
	var rows []struct {
		Context string `orm:"context_code"`
	}
	if err := s.database.GetScan(ctx, &rows, `SELECT context_code FROM plugin_idc_core_support_ticket_attachment_upload WHERE tenant_id=0 AND upload_no=? AND uploader_type='staff' AND status_code='reserved' LIMIT 1`, uploadNo); err != nil {
		return "", wrapUnavailable(err, "load attachment upload context")
	}
	if len(rows) != 1 {
		return "", bizNotAccessible()
	}
	return rows[0].Context, nil
}
func (s *serviceImpl) UploadAccountAttachment(ctx context.Context, uploadNo, contentType string, body io.Reader) (*AttachmentUploadResult, error) {
	a, err := s.customerActor(ctx)
	if err != nil {
		return nil, err
	}
	return s.uploadAttachment(ctx, uploadNo, contentType, body, nil, a.CustomerNo, "customer", "")
}

func (s *serviceImpl) UploadAnonymousAttachment(ctx context.Context, accessToken, uploadNo, contentType string, body io.Reader) (*AttachmentUploadResult, error) {
	return s.uploadAttachment(ctx, uploadNo, contentType, body, nil, "", "anonymous", accessToken)
}

func (s *serviceImpl) OpenStaffAttachment(ctx context.Context, attachmentNo string) (*AttachmentOpen, error) {
	a, err := s.staffActor(ctx, PermissionQuery)
	if err != nil {
		return nil, err
	}
	var where strings.Builder
	where.WriteString(` WHERE a.tenant_id=0 AND a.attachment_no=? AND a.hidden_at IS NULL`)
	args := []any{attachmentNo}
	appendStaffAccess(&where, &args, a, "t")
	return s.openAttachment(ctx, where.String(), args...)
}

func (s *serviceImpl) OpenAccountAttachment(ctx context.Context, attachmentNo string) (*AttachmentOpen, error) {
	a, err := s.customerActor(ctx)
	if err != nil {
		return nil, err
	}
	return s.openAttachment(ctx, ` WHERE a.tenant_id=0 AND a.attachment_no=? AND a.visibility_code='customer_visible' AND a.hidden_at IS NULL AND t.ticket_type='customer' AND t.visibility_code='customer' AND t.customer_no=?`, attachmentNo, a.CustomerNo)
}

func (s *serviceImpl) OpenAnonymousAttachment(ctx context.Context, accessToken, attachmentNo string) (*AttachmentOpen, error) {
	accessToken = strings.TrimSpace(accessToken)
	if len(accessToken) < 40 || len(accessToken) > 128 {
		return nil, bizNotAccessible()
	}
	return s.openAttachment(ctx, ` JOIN plugin_idc_core_support_ticket_access_link l ON l.tenant_id=0 AND l.ticket_no=t.ticket_no WHERE a.tenant_id=0 AND a.attachment_no=? AND a.visibility_code='customer_visible' AND a.hidden_at IS NULL AND t.ticket_type='presales' AND t.visibility_code='anonymous' AND l.token_digest=? AND l.revoked_at IS NULL AND l.expires_at>CURRENT_TIMESTAMP`, attachmentNo, anonymousTokenDigest(accessToken))
}

func (s *serviceImpl) openAttachment(ctx context.Context, suffix string, args ...any) (*AttachmentOpen, error) {
	if len(args) == 0 {
		return nil, bizNotAccessible()
	}
	attachmentNo, ok := args[0].(string)
	if !ok || !businessNoPattern.MatchString(attachmentNo) {
		return nil, bizNotAccessible()
	}
	var rows []struct {
		FileName, MediaType, ObjectRef string
		SizeBytes                      int64
	}
	query := `SELECT a.file_name,a.media_type,a.size_bytes,a.object_ref FROM plugin_idc_core_support_ticket_attachment a JOIN plugin_idc_core_support_ticket t ON t.tenant_id=0 AND t.ticket_no=a.ticket_no` + suffix + ` LIMIT 1`
	if err := s.database.GetScan(ctx, &rows, query, args...); err != nil {
		return nil, wrapUnavailable(err, "authorize ticket attachment download")
	}
	if len(rows) != 1 || rows[0].ObjectRef == "" || rows[0].SizeBytes < 1 {
		return nil, bizNotAccessible()
	}
	object, err := s.storage.Get(ctx, storagecap.GetInput{Path: rows[0].ObjectRef})
	if err != nil {
		return nil, wrapUnavailable(err, "open ticket attachment")
	}
	if object == nil || !object.Found || object.Body == nil {
		if object != nil && object.Body != nil {
			return nil, errors.Join(bizNotAccessible(), object.Body.Close())
		}
		return nil, bizNotAccessible()
	}
	return &AttachmentOpen{FileName: rows[0].FileName, MediaType: rows[0].MediaType, SizeBytes: rows[0].SizeBytes, Body: object.Body}, nil
}

type attachmentUploadRow struct {
	UploadNo, TicketNo, UploaderType, Context, FileName, ObjectRef, Status string
	UploaderUserID, DeclaredSize, Version                                  int64
	ExpiresAt                                                              time.Time
}

func (s *serviceImpl) uploadAttachment(ctx context.Context, uploadNo, declaredType string, body io.Reader, staff *actor, customerNo, uploaderType, anonymousToken string) (*AttachmentUploadResult, error) {
	if !businessNoPattern.MatchString(uploadNo) || body == nil {
		return nil, bizInvalid()
	}
	actorUserID, err := s.attachmentActorUserID(ctx, staff, uploaderType)
	if err != nil {
		return nil, err
	}
	var rows []attachmentUploadRow
	if err := s.database.GetScan(ctx, &rows, `SELECT upload_no,ticket_no,uploader_user_id,uploader_type,context_code AS context,file_name,declared_size_bytes AS declared_size,temporary_object_ref AS object_ref,status_code AS status,expires_at,version FROM plugin_idc_core_support_ticket_attachment_upload WHERE tenant_id=0 AND upload_no=? LIMIT 1`, uploadNo); err != nil {
		return nil, wrapUnavailable(err, "load attachment upload")
	}
	if len(rows) != 1 || rows[0].UploaderType != uploaderType || rows[0].UploaderUserID != int64(actorUserID) || rows[0].Status != "reserved" {
		return nil, bizNotAccessible()
	}
	row := rows[0]
	initial := row.Context == "internal_initial" || row.Context == "customer_initial"
	if !initial {
		if _, err := s.authorizeAttachmentTicket(ctx, row.TicketNo, staff, customerNo, uploaderType, anonymousToken, false); err != nil {
			return nil, err
		}
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, wrapUnavailable(err, "read attachment upload time")
	}
	if !row.ExpiresAt.After(now) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	content, err := io.ReadAll(io.LimitReader(body, maxAttachmentBytes+1))
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAttachmentRejected)
	}
	if int64(len(content)) != row.DeclaredSize || int64(len(content)) > maxAttachmentBytes {
		return nil, bizerr.NewCode(CodeAttachmentRejected)
	}
	mediaType, err := inspectTicketAttachment(row.FileName, content)
	if err != nil {
		return nil, err
	}
	if declared := strings.TrimSpace(strings.Split(declaredType, ";")[0]); declared != "" && declared != "application/octet-stream" && declared != mediaType {
		return nil, bizerr.NewCode(CodeAttachmentRejected)
	}
	digest := sha256.Sum256(content)
	hash := hex.EncodeToString(digest[:])
	if _, err = s.storage.Put(ctx, storagecap.PutInput{Path: row.ObjectRef, Body: bytes.NewReader(content), Size: int64(len(content)), ContentType: mediaType, Overwrite: false}); err != nil {
		return nil, wrapUnavailable(err, "store ticket attachment")
	}
	if uploaderType == "anonymous" {
		if _, authErr := s.authorizeAttachmentTicket(ctx, row.TicketNo, nil, "", uploaderType, anonymousToken, false); authErr != nil {
			return nil, s.removeStoredAttachment(ctx, row.ObjectRef, authErr)
		}
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_attachment_upload SET media_type=?,sha256_hex=?,status_code='ready',completed_at=?,version=version+1,updated_at=? WHERE tenant_id=0 AND upload_no=? AND status_code='reserved' AND version=?`, mediaType, hash, now, now, uploadNo, row.Version)
	if err != nil {
		return nil, wrapUnavailable(s.removeStoredAttachment(ctx, row.ObjectRef, err), "confirm ticket attachment")
	}
	changed, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return nil, wrapUnavailable(s.removeStoredAttachment(ctx, row.ObjectRef, rowsErr), "confirm ticket attachment")
	}
	if changed != 1 {
		return nil, s.removeStoredAttachment(ctx, row.ObjectRef, bizerr.NewCode(CodeConflict))
	}
	return &AttachmentUploadResult{UploadNo: uploadNo, FileName: row.FileName, MediaType: mediaType, SizeBytes: int64(len(content)), Version: row.Version + 1, Ready: true}, nil
}

func (s *serviceImpl) attachmentActorUserID(ctx context.Context, staff *actor, uploaderType string) (int, error) {
	if staff != nil {
		return staff.UserID, nil
	}
	if uploaderType == "anonymous" {
		return 0, nil
	}
	if uploaderType != "customer" {
		return 0, bizNotAccessible()
	}
	a, err := s.customerActor(ctx)
	if err != nil {
		return 0, err
	}
	return a.UserID, nil
}

func (s *serviceImpl) authorizeAttachmentTicket(ctx context.Context, ticketNo string, staff *actor, customerNo, uploaderType, anonymousToken string, lock bool) (*Ticket, error) {
	if uploaderType != "anonymous" {
		return s.lockAccessibleTicket(ctx, ticketNo, staff, customerNo)
	}
	ticket, err := s.anonymousTicket(ctx, anonymousToken, lock)
	if err != nil {
		return nil, err
	}
	if ticket.TicketNo != ticketNo || ticket.TicketType != "presales" || ticket.Visibility != "anonymous" {
		return nil, bizNotAccessible()
	}
	return ticket, nil
}

func (s *serviceImpl) removeStoredAttachment(ctx context.Context, objectPath string, cause error) error {
	return errors.Join(cause, s.storage.Delete(ctx, storagecap.DeleteInput{Path: objectPath}))
}

func inspectTicketAttachment(fileName string, content []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	disallowed := map[string]bool{".exe": true, ".msi": true, ".com": true, ".bat": true, ".cmd": true, ".ps1": true, ".sh": true, ".js": true, ".mjs": true, ".py": true, ".php": true, ".pl": true, ".rb": true, ".jar": true, ".scr": true, ".dll": true}
	if disallowed[ext] {
		return "", bizerr.NewCode(CodeAttachmentRejected)
	}
	media := strings.Split(http.DetectContentType(content), ";")[0]
	if strings.HasPrefix(media, "image/") && media != "image/svg+xml" {
		return media, nil
	}
	if media == "application/pdf" && bytes.HasPrefix(content, []byte("%PDF-")) {
		return media, nil
	}
	if bytes.HasPrefix(content, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) && map[string]bool{".doc": true, ".xls": true, ".ppt": true}[ext] {
		return "application/x-ole-storage", nil
	}
	if bytes.HasPrefix(content, []byte("PK\x03\x04")) {
		kind, err := inspectZip(fileName, content)
		if err != nil {
			return "", err
		}
		if kind == "office" {
			return "application/vnd.openxmlformats-officedocument", nil
		}
		return "application/zip", nil
	}
	if media == "text/plain" && (ext == ".txt" || ext == ".log" || ext == ".csv" || ext == "") {
		if bytes.IndexByte(content, 0) >= 0 {
			return "", bizerr.NewCode(CodeAttachmentRejected)
		}
		return "text/plain", nil
	}
	return "", bizerr.NewCode(CodeAttachmentRejected)
}

func inspectZip(fileName string, content []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", bizerr.NewCode(CodeAttachmentRejected)
	}
	if len(reader.File) > 2000 {
		return "", bizerr.NewCode(CodeAttachmentRejected)
	}
	office := false
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == ".docx" || ext == ".xlsx" || ext == ".pptx" {
		office = true
	}
	var total uint64
	for _, entry := range reader.File {
		if entry.Flags&1 != 0 {
			return "", bizerr.NewCode(CodeAttachmentRejected)
		}
		total += entry.UncompressedSize64
		if total > uint64(maxTicketUploadBytes) {
			return "", bizerr.NewCode(CodeAttachmentRejected)
		}
		entryExt := strings.ToLower(filepath.Ext(entry.Name))
		if map[string]bool{".exe": true, ".msi": true, ".com": true, ".bat": true, ".cmd": true, ".ps1": true, ".sh": true, ".js": true, ".mjs": true, ".py": true, ".php": true, ".pl": true, ".rb": true, ".jar": true, ".scr": true, ".dll": true}[entryExt] {
			return "", bizerr.NewCode(CodeAttachmentRejected)
		}
	}
	if office {
		return "office", nil
	}
	if ext != ".zip" {
		return "", bizerr.NewCode(CodeAttachmentRejected)
	}
	return "zip", nil
}

func (s *serviceImpl) consumeUploads(ctx context.Context, ticketNo, messageNo string, uploadNos []string, actorUserID int, visibility, expectedContext string, now time.Time) error {
	if len(uploadNos) > 20 {
		return bizInvalid()
	}
	seen := make(map[string]struct{}, len(uploadNos))
	var selectedBytes int64
	for _, uploadNo := range uploadNos {
		if !businessNoPattern.MatchString(uploadNo) {
			return bizInvalid()
		}
		if _, exists := seen[uploadNo]; exists {
			return bizInvalid()
		}
		seen[uploadNo] = struct{}{}
	}
	for _, uploadNo := range uploadNos {
		var rows []struct {
			FileName, MediaType, SHA256, ObjectRef, Context string
			SizeBytes, UploaderUserID, Version              int64
		}
		if err := s.database.GetScan(ctx, &rows, `SELECT file_name,media_type,sha256_hex AS sha256,temporary_object_ref AS object_ref,context_code AS context,declared_size_bytes AS size_bytes,uploader_user_id,version FROM plugin_idc_core_support_ticket_attachment_upload WHERE tenant_id=0 AND upload_no=? AND (ticket_no=? OR (ticket_no IS NULL AND context_code=?)) AND status_code='ready' AND expires_at>? FOR UPDATE`, uploadNo, ticketNo, expectedContext, now); err != nil {
			return err
		}
		if len(rows) != 1 || rows[0].UploaderUserID != int64(actorUserID) || rows[0].Context != expectedContext {
			return bizerr.NewCode(CodeConflict)
		}
		expectedVisibility := "customer_visible"
		if rows[0].Context == "internal_note" || rows[0].Context == "internal_initial" {
			expectedVisibility = "internal_only"
		}
		if expectedVisibility != visibility {
			return bizerr.NewCode(CodeConflict)
		}
		selectedBytes += rows[0].SizeBytes
		if selectedBytes > maxTicketUploadBytes {
			return bizerr.NewCode(CodeAttachmentRejected)
		}
		attachmentNo, err := s.newNo("ATT_")
		if err != nil {
			return err
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_attachment (tenant_id,attachment_no,ticket_no,message_no,visibility_code,file_name,media_type,size_bytes,sha256_hex,object_ref,inspection_code,created_by,created_at) VALUES (0,?,?,?,?,?,?,?,?,?,'passed',?,?)`, attachmentNo, ticketNo, messageNo, visibility, rows[0].FileName, rows[0].MediaType, rows[0].SizeBytes, rows[0].SHA256, rows[0].ObjectRef, actorUserID, now); err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_attachment_upload SET ticket_no=?,status_code='consumed',consumed_at=?,version=version+1,updated_at=? WHERE tenant_id=0 AND upload_no=? AND status_code='ready' AND version=?`, ticketNo, now, now, uploadNo, rows[0].Version)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
	}
	return nil
}
