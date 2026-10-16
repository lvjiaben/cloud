package supportdesk

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
)

func TestAnonymousCloseReopenAndRateLifecyclePostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	service := newAuthorizedSupportDeskTestService(database)
	destinationDigest := strings.Repeat("a", 64)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,anonymous_destination_digest,anonymous_destination_kind,subject,base_stage,priority_code,current_round,last_activity_at,created_by,updated_by) VALUES (0,'TKT_anonymous','presales','anonymous',?,'email','Anonymous','in_progress','normal',1,CURRENT_TIMESTAMP,0,0)`, destinationDigest); err != nil {
		t.Fatal(err)
	}
	access, err := service.IssueVerifiedAnonymousLink(ctx, "TKT_anonymous", "email", destinationDigest)
	if err != nil {
		t.Fatal(err)
	}

	closed, err := service.CloseAnonymous(ctx, access.Token, 1)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Ticket.BaseStage != "closed" || closed.Ticket.Version != 2 || !hasAction(closed.AvailableActions, "reopen") || !hasAction(closed.AvailableActions, "rate") || hasAction(closed.AvailableActions, "reply") {
		t.Fatalf("unexpected closed anonymous detail: ticket=%+v actions=%v", closed.Ticket, closed.AvailableActions)
	}
	if _, err = service.CloseAnonymous(ctx, access.Token, 1); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("stale anonymous close error=%v, want conflict", err)
	}

	rated, err := service.RateAnonymous(ctx, access.Token, RatingInput{TicketVersion: closed.Ticket.Version, RoundNo: 1, Stars: 2, Feedback: "Need follow-up"})
	if err != nil {
		t.Fatal(err)
	}
	if rated.Rating == nil || rated.Rating.Stars != 2 || !rated.Ticket.FollowUpRequired || hasAction(rated.AvailableActions, "rate") {
		t.Fatalf("unexpected anonymous rating projection: rating=%+v actions=%v", rated.Rating, rated.AvailableActions)
	}
	if _, err = service.RateAnonymous(ctx, access.Token, RatingInput{TicketVersion: rated.Ticket.Version, RoundNo: 1, Stars: 5}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("duplicate anonymous rating error=%v, want conflict", err)
	}

	reopened, err := service.ReopenAnonymous(ctx, access.Token, rated.Ticket.Version)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Ticket.BaseStage != "in_progress" || reopened.Ticket.CurrentRound != 2 || reopened.Ticket.Version != rated.Ticket.Version+1 || !hasAction(reopened.AvailableActions, "reply") || !hasAction(reopened.AvailableActions, "close") || hasAction(reopened.AvailableActions, "rate") {
		t.Fatalf("unexpected reopened anonymous detail: ticket=%+v actions=%v", reopened.Ticket, reopened.AvailableActions)
	}
	rounds, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_sla_round WHERE tenant_id=0 AND ticket_no='TKT_anonymous' AND round_no=2`)
	if err != nil || rounds != 1 {
		t.Fatalf("new anonymous SLA round count=%d err=%v", rounds, err)
	}
	var historySources []struct {
		Source string `orm:"source_code"`
	}
	if err = database.GetScan(ctx, &historySources, `SELECT source_code FROM plugin_idc_core_support_ticket_status_history WHERE tenant_id=0 AND ticket_no='TKT_anonymous' ORDER BY occurred_at,id`); err != nil {
		t.Fatal(err)
	}
	if len(historySources) != 2 || historySources[0].Source != "anonymous" || historySources[1].Source != "anonymous" {
		t.Fatalf("anonymous status history sources=%v", historySources)
	}
}

func TestAnonymousActionsRejectExpiredAccessAndLateReopenPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	service := newAuthorizedSupportDeskTestService(database)
	destinationDigest := strings.Repeat("b", 64)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,anonymous_destination_digest,anonymous_destination_kind,subject,base_stage,priority_code,current_round,closed_at,closed_reason_code,closed_by_type,last_activity_at,created_by,updated_by) VALUES (0,'TKT_anonymous_expired','presales','anonymous',?,'sms','Expired','closed','normal',1,CURRENT_TIMESTAMP-INTERVAL '8 days','anonymous_confirmed','anonymous',CURRENT_TIMESTAMP,0,0)`, destinationDigest); err != nil {
		t.Fatal(err)
	}
	access, err := service.IssueVerifiedAnonymousLink(ctx, "TKT_anonymous_expired", "sms", destinationDigest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ReopenAnonymous(ctx, access.Token, 1); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("late anonymous reopen error=%v, want conflict", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_access_link SET created_at=CURRENT_TIMESTAMP-INTERVAL '3 hours',expires_at=CURRENT_TIMESTAMP-INTERVAL '1 hour' WHERE link_no=?`, access.LinkNo); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RateAnonymous(ctx, access.Token, RatingInput{TicketVersion: 1, RoundNo: 1, Stars: 4}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("expired anonymous rating error=%v, want not accessible", err)
	}
}

func TestAnonymousReopenReturnsAssignedTicketToQueuePostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	service := newAuthorizedSupportDeskTestService(database)
	destinationDigest := strings.Repeat("c", 64)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,anonymous_destination_digest,anonymous_destination_kind,subject,base_stage,priority_code,current_round,assignee_user_id,closed_at,closed_reason_code,closed_by_type,last_activity_at,created_by,updated_by) VALUES (0,'TKT_anonymous_assigned','presales','anonymous',?,'email','Assigned','closed','normal',1,77,CURRENT_TIMESTAMP,'anonymous_confirmed','anonymous',CURRENT_TIMESTAMP,0,0)`, destinationDigest); err != nil {
		t.Fatal(err)
	}
	access, err := service.IssueVerifiedAnonymousLink(ctx, "TKT_anonymous_assigned", "email", destinationDigest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ReopenAnonymous(ctx, access.Token, 1); err != nil {
		t.Fatalf("anonymous reopen: %v", err)
	}
	var current struct {
		Stage          string `orm:"base_stage"`
		Version        int64  `orm:"version"`
		AssigneeUserID int64  `orm:"assignee_user_id"`
	}
	if err = database.GetScan(ctx, &current, `SELECT base_stage,version,assignee_user_id FROM plugin_idc_core_support_ticket WHERE tenant_id=0 AND ticket_no='TKT_anonymous_assigned'`); err != nil {
		t.Fatal(err)
	}
	if current.Stage != StageInProgress || current.Version != 2 || current.AssigneeUserID != 0 {
		t.Fatalf("reopened ticket did not return to queue: %+v", current)
	}
}

func TestAnonymousAttachmentRequiresSameLiveTicketTokenAtReserveUploadAndReplyPostgreSQL(t *testing.T) {
	if mediaType, inspectErr := inspectTicketAttachment("note.txt", []byte("hello")); inspectErr != nil || mediaType != "text/plain" {
		t.Fatalf("test attachment fixture is not accepted: media=%q err=%v", mediaType, inspectErr)
	}
	ctx, database := newSupportDeskTestDatabase(t)
	service := newAuthorizedSupportDeskTestService(database)
	storage := &anonymousAttachmentStorage{objects: map[string][]byte{}}
	service.storage = storage
	for _, statement := range []string{
		`INSERT INTO plugin_idc_core_support_ticket
         (tenant_id,ticket_no,ticket_type,visibility_code,anonymous_destination_digest,
          anonymous_destination_kind,subject,base_stage,priority_code,current_round,
          last_activity_at,created_by,updated_by)
         VALUES (0,'TKT_attachment','presales','anonymous',repeat('d',64),'email',
                 'Attachment','in_progress','normal',1,CURRENT_TIMESTAMP,0,0)`,
		`INSERT INTO plugin_idc_core_support_ticket
         (tenant_id,ticket_no,ticket_type,visibility_code,anonymous_destination_digest,
          anonymous_destination_kind,subject,base_stage,priority_code,current_round,
          last_activity_at,created_by,updated_by)
         VALUES (0,'TKT_attachment_other','presales','anonymous',repeat('e',64),'email',
                 'Other','in_progress','normal',1,CURRENT_TIMESTAMP,0,0)`,
	} {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	access, err := service.IssueVerifiedAnonymousLink(ctx, "TKT_attachment", "email", strings.Repeat("d", 64))
	if err != nil {
		t.Fatal(err)
	}
	otherAccess, err := service.IssueVerifiedAnonymousLink(ctx, "TKT_attachment_other", "email", strings.Repeat("e", 64))
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := service.ReserveAnonymousAttachment(ctx, access.Token, AttachmentUploadInput{FileName: "note.txt", SizeBytes: 5})
	if err != nil {
		t.Fatalf("reserve anonymous attachment: %v", err)
	}
	if reservation.TicketNo != "TKT_attachment" || reservation.Context != "anonymous_reply" {
		t.Fatalf("unexpected anonymous reservation: %+v", reservation)
	}
	if _, err = service.UploadAnonymousAttachment(ctx, otherAccess.Token, reservation.UploadNo, "text/plain", bytes.NewReader([]byte("hello"))); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("cross-ticket anonymous upload error=%v, want not accessible", err)
	}
	uploaded, err := service.UploadAnonymousAttachment(ctx, access.Token, reservation.UploadNo, "text/plain", bytes.NewReader([]byte("hello")))
	if err != nil || !uploaded.Ready {
		t.Fatalf("upload anonymous attachment: item=%+v err=%v", uploaded, err)
	}
	if _, err = service.ReplyAnonymous(ctx, otherAccess.Token, MessageInput{Body: "wrong ticket", UploadNos: []string{reservation.UploadNo}, TicketVersion: 1}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("cross-ticket anonymous consume error=%v, want conflict", err)
	}
	otherVersion, err := database.GetValue(ctx, `SELECT version FROM plugin_idc_core_support_ticket WHERE ticket_no='TKT_attachment_other'`)
	if err != nil || otherVersion.Int64() != 1 {
		t.Fatalf("cross-ticket consume did not roll back: version=%d err=%v", otherVersion.Int64(), err)
	}
	detail, err := service.ReplyAnonymous(ctx, access.Token, MessageInput{Body: "with attachment", UploadNos: []string{reservation.UploadNo}, TicketVersion: 1})
	if err != nil {
		t.Fatalf("consume anonymous attachment: %v", err)
	}
	if len(detail.Messages) != 1 || len(detail.Messages[0].Attachments) != 1 || detail.Messages[0].Attachments[0].Visibility != "customer_visible" {
		t.Fatalf("unsafe anonymous attachment projection: %+v", detail.Messages)
	}
	var persisted struct {
		UploaderType string `orm:"uploader_type"`
		UserID       int64  `orm:"uploader_user_id"`
		Context      string `orm:"context_code"`
		Status       string `orm:"status_code"`
	}
	if err = database.GetScan(ctx, &persisted, `SELECT uploader_type,uploader_user_id,context_code,status_code
        FROM plugin_idc_core_support_ticket_attachment_upload WHERE upload_no=?`, reservation.UploadNo); err != nil {
		t.Fatal(err)
	}
	if persisted.UploaderType != "anonymous" || persisted.UserID != 0 || persisted.Context != "anonymous_reply" || persisted.Status != "consumed" {
		t.Fatalf("unexpected anonymous upload facts: %+v", persisted)
	}

	expiring, err := service.ReserveAnonymousAttachment(ctx, access.Token, AttachmentUploadInput{FileName: "expire.txt", SizeBytes: 5})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_access_link
		SET created_at=CURRENT_TIMESTAMP-INTERVAL '3 hours',
		    expires_at=CURRENT_TIMESTAMP-INTERVAL '1 hour' WHERE link_no=?`, access.LinkNo); err != nil {
		t.Fatal(err)
	}
	if _, err = service.UploadAnonymousAttachment(ctx, access.Token, expiring.UploadNo, "text/plain", bytes.NewReader([]byte("hello"))); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("expired token upload error=%v, want not accessible", err)
	}
	if _, err = service.ReserveAnonymousAttachment(ctx, access.Token, AttachmentUploadInput{FileName: "old.txt", SizeBytes: 5}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("expired token reservation error=%v, want not accessible", err)
	}

	renewed, err := service.IssueVerifiedAnonymousLink(ctx, "TKT_attachment", "email", strings.Repeat("d", 64))
	if err != nil {
		t.Fatal(err)
	}
	late, err := service.ReserveAnonymousAttachment(ctx, renewed.Token, AttachmentUploadInput{FileName: "late.txt", SizeBytes: 5})
	if err != nil {
		t.Fatal(err)
	}
	storage.putHook = func() {
		if _, hookErr := database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_access_link
			SET created_at=CURRENT_TIMESTAMP-INTERVAL '3 hours',
			    expires_at=CURRENT_TIMESTAMP-INTERVAL '1 hour' WHERE link_no=?`, renewed.LinkNo); hookErr != nil {
			t.Errorf("expire token during upload: %v", hookErr)
		}
	}
	if _, err = service.UploadAnonymousAttachment(ctx, renewed.Token, late.UploadNo, "text/plain", bytes.NewReader([]byte("hello"))); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("token expiring during upload error=%v, want not accessible", err)
	}
	if _, exists := storage.objects["support/attachment-uploads/"+late.UploadNo]; exists {
		t.Fatal("object remained after token expired during upload")
	}
}

func TestAnonymousAttachmentConstraintMigrationIsIdempotentPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "manifest", "sql"))
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join(root, "042-support-content.sql"))
	if err != nil {
		t.Fatal(err)
	}
	entitlementMigration, err := os.ReadFile(filepath.Join(root, "091-support-entitlement-association.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for pass := 1; pass <= 2; pass++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("reapply support/content migration pass %d: %v", pass, err)
		}
		if _, err = database.Exec(ctx, string(entitlementMigration)); err != nil {
			t.Fatalf("reapply support entitlement migration pass %d: %v", pass, err)
		}
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket
        (tenant_id,ticket_no,ticket_type,visibility_code,anonymous_destination_digest,
         anonymous_destination_kind,subject,base_stage,priority_code,current_round,
         last_activity_at,created_by,updated_by)
        VALUES (0,'TKT_constraint','presales','anonymous',repeat('f',64),'email',
                'Constraint','new','normal',1,CURRENT_TIMESTAMP,0,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_attachment_upload
        (tenant_id,upload_no,ticket_no,uploader_user_id,uploader_type,context_code,
         file_name,declared_size_bytes,temporary_object_ref,status_code,expires_at)
        VALUES (0,'TUP_constraint','TKT_constraint',0,'anonymous','anonymous_reply',
                'note.txt',5,'support/attachment-uploads/TUP_constraint','reserved',CURRENT_TIMESTAMP+INTERVAL '1 hour')`); err != nil {
		t.Fatalf("anonymous upload constraint after idempotent migration: %v", err)
	}
}

type anonymousAttachmentStorage struct {
	storagecap.Service
	objects map[string][]byte
	putHook func()
}

func (s *anonymousAttachmentStorage) Put(_ context.Context, input storagecap.PutInput) (*storagecap.PutOutput, error) {
	content, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	s.objects[input.Path] = content
	if s.putHook != nil {
		s.putHook()
		s.putHook = nil
	}
	return &storagecap.PutOutput{}, nil
}

func (s *anonymousAttachmentStorage) Delete(_ context.Context, input storagecap.DeleteInput) error {
	delete(s.objects, input.Path)
	return nil
}

func hasAction(actions []string, expected string) bool {
	for _, action := range actions {
		if action == expected {
			return true
		}
	}
	return false
}
