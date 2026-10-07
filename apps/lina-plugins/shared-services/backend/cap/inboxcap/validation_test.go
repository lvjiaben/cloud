package inboxcap

import "testing"

func TestValidatePublishRequestRejectsExternalActionURL(t *testing.T) {
	valid := PublishRequest{RecipientUserID: 7, SourceType: "support_ticket", SourceNo: "ticket_1",
		IdempotencyKey: "event_12345678", Title: "Ticket updated", Body: "A reply is available.",
		Category: "support", ActionURL: "/support/tickets/ticket_1"}
	if err := ValidatePublishRequest(valid); err != nil {
		t.Fatalf("valid inbox request rejected: %v", err)
	}
	for _, target := range []string{
		"https://example.com", "//example.com/path", "/\\example.com/path", "/safe\nLocation: https://example.com", "/safe\tpath",
	} {
		candidate := valid
		candidate.ActionURL = target
		if err := ValidatePublishRequest(candidate); err == nil {
			t.Fatalf("unsafe action URL accepted: %q", target)
		}
	}
}
