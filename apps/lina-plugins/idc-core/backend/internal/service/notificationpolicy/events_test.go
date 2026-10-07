package notificationpolicy

import (
	"testing"

	"lina-plugin-idc-core/backend/cap/notificationcap"
)

func TestValidateNotificationEventRequestRequiresOneTypedRecipient(t *testing.T) {
	base := notificationcap.EventRequest{
		EventCode:          "support.ticket_updated",
		BusinessObjectType: notificationcap.BusinessObjectSupportTicket,
		BusinessObjectNo:   "TKT_event",
		Variables:          map[string]string{"ticket_no": "TKT_event"},
		IdempotencyKey:     "support-event-1",
	}
	tests := []struct {
		name      string
		recipient notificationcap.Recipient
		valid     bool
	}{
		{name: "customer contact", recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact, CustomerNo: "customer-1", ContactNo: "contact-1"}, valid: true},
		{name: "staff user", recipient: notificationcap.Recipient{Kind: notificationcap.RecipientStaffUser, StaffUserID: 7}, valid: true},
		{name: "verified destination", recipient: notificationcap.Recipient{Kind: notificationcap.RecipientVerifiedDestination, DestinationNo: "destination-1"}, valid: true},
		{name: "mixed authority", recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact, CustomerNo: "customer-1", ContactNo: "contact-1", StaffUserID: 7}},
		{name: "raw-looking destination without kind", recipient: notificationcap.Recipient{DestinationNo: "destination-1"}},
		{name: "missing contact", recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact, CustomerNo: "customer-1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := base
			request.Recipient = test.recipient
			err := validateNotificationEventRequest(request)
			if test.valid && err != nil {
				t.Fatalf("valid recipient was rejected: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("invalid recipient was accepted")
			}
		})
	}
}
