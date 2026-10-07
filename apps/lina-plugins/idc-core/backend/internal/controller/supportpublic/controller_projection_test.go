package supportpublic

import (
	"context"
	"testing"
	"time"

	v1 "lina-plugin-idc-core/backend/api/supportpublic/v1"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

func TestAnonymousExpiresAtUsesAuthorityTimestamp(t *testing.T) {
	expiresAt := time.Date(2026, time.August, 15, 3, 4, 5, 0, time.UTC)
	if got := anonymousExpiresAt(&supportdesk.Detail{AnonymousAccessExpiresAt: &expiresAt}); got != expiresAt.UnixMilli() {
		t.Fatalf("anonymous expiry = %d, want %d", got, expiresAt.UnixMilli())
	}
	if got := anonymousExpiresAt(nil); got != 0 {
		t.Fatalf("missing anonymous expiry = %d, want 0", got)
	}
}

func TestPresalesAttachmentReservationUsesTokenBoundServiceContract(t *testing.T) {
	expiresAt := time.Date(2026, time.August, 15, 6, 7, 8, 0, time.UTC)
	service := &publicAttachmentServiceStub{reservation: &supportdesk.AttachmentReservation{
		UploadNo: "TUP_safe", TicketNo: "TKT_hidden", FileName: "note.txt",
		Context: "anonymous_reply", SizeBytes: 5, ExpiresAt: expiresAt, Version: 1,
	}}
	controller, err := NewV1(service)
	if err != nil {
		t.Fatal(err)
	}
	response, err := controller.PresalesAttachmentUploadCreate(context.Background(), &v1.PresalesAttachmentUploadCreateReq{
		AccessToken: "opaque-token", FileName: "note.txt", SizeBytes: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if service.accessToken != "opaque-token" || service.input.FileName != "note.txt" || service.input.SizeBytes != 5 {
		t.Fatalf("unexpected reservation input token=%q input=%+v", service.accessToken, service.input)
	}
	if response.UploadNo != "TUP_safe" || response.NextOffset != 0 || response.ExpiresAt != expiresAt.UnixMilli() || response.Version != 1 {
		t.Fatalf("unexpected reservation response: %+v", response)
	}
}

type publicAttachmentServiceStub struct {
	supportdesk.Service
	accessToken string
	input       supportdesk.AttachmentUploadInput
	reservation *supportdesk.AttachmentReservation
}

func (s *publicAttachmentServiceStub) ReserveAnonymousAttachment(_ context.Context, accessToken string, input supportdesk.AttachmentUploadInput) (*supportdesk.AttachmentReservation, error) {
	s.accessToken = accessToken
	s.input = input
	return s.reservation, nil
}
