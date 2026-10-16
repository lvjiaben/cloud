// This file emits customer-facing ticket notices from authoritative contact data.
// Internal queue work is presented only through permission-filtered ticket pages.

package supportdesk

import (
	"context"
	"fmt"

	"lina-plugin-idc-core/backend/cap/notificationcap"
)

const (
	supportEventTicketCreated       = "support.ticket_created"
	supportEventTicketUpdated       = "support.ticket_updated"
	supportEventTicketPublicUpdated = "support.ticket_public_updated"
)

// enqueueCustomerTicketEvent appends a pure business intent only when the
// customer has one authoritative active primary contact. Missing contact data
// is not guessed and produces no event. A configured sink failure aborts the
// caller's transaction so business state and its outbox cannot diverge.
func (s *serviceImpl) enqueueCustomerTicketEvent(
	ctx context.Context,
	ticket *Ticket,
	eventCode string,
	businessVersion int64,
) error {
	if s.notifications == nil || ticket == nil || ticket.CustomerNo == "" {
		return nil
	}
	var contacts []struct {
		ContactNo string `orm:"contact_no"`
	}
	if err := s.database.GetScan(ctx, &contacts, `SELECT c.contact_no
FROM plugin_idc_core_customer_contact_role r
JOIN plugin_idc_core_customer_contact c
  ON c.tenant_id=0 AND c.customer_no=r.customer_no AND c.contact_no=r.contact_no
WHERE r.tenant_id=0 AND r.customer_no=? AND r.role_code='primary'
  AND r.status_code='active' AND c.status_code='active'
ORDER BY c.id
LIMIT 2`, ticket.CustomerNo); err != nil {
		return err
	}
	if len(contacts) != 1 {
		return nil
	}
	_, err := s.notifications.Enqueue(ctx, notificationcap.EventRequest{
		EventCode: eventCode,
		Recipient: notificationcap.Recipient{
			Kind:       notificationcap.RecipientCustomerContact,
			CustomerNo: ticket.CustomerNo,
			ContactNo:  contacts[0].ContactNo,
		},
		BusinessObjectType:    notificationcap.BusinessObjectSupportTicket,
		BusinessObjectNo:      ticket.TicketNo,
		BusinessObjectVersion: businessVersion,
		Variables: map[string]string{
			"ticket_no": ticket.TicketNo,
			"title":     ticket.Subject,
		},
		CorrelationNo: ticket.TicketNo,
		IdempotencyKey: fmt.Sprintf(
			"%s:%s:v%d:%s", eventCode, ticket.TicketNo, businessVersion, contacts[0].ContactNo,
		),
	})
	return err
}
