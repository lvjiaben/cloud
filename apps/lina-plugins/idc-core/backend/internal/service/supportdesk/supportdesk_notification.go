package supportdesk

import (
	"context"
	"fmt"
	"strconv"

	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-staff-access/backend/cap/staffaccesscap"
)

const (
	supportEventTicketCreated       = "support.ticket_created"
	supportEventTicketUpdated       = "support.ticket_updated"
	supportEventTicketPublicUpdated = "support.ticket_public_updated"
	supportEventTicketEscalated     = "support.ticket_escalated"
	supportEventTicketRatingLow     = "support.ticket_rating_low"
	supportEventTicketRatingNeutral = "support.ticket_rating_neutral"
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

func (s *serviceImpl) enqueueStaffTicketEvent(ctx context.Context, ticket *Ticket, eventCode string, recipient int64, businessVersion int64, variables map[string]string) error {
	if s.notifications == nil || ticket == nil || recipient <= 0 {
		return nil
	}
	values := map[string]string{"ticket_no": ticket.TicketNo, "title": ticket.Subject}
	for key, value := range variables {
		values[key] = value
	}
	_, err := s.notifications.Enqueue(ctx, notificationcap.EventRequest{
		EventCode: eventCode, Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientStaffUser, StaffUserID: int(recipient)},
		BusinessObjectType: notificationcap.BusinessObjectSupportTicket, BusinessObjectNo: ticket.TicketNo,
		BusinessObjectVersion: businessVersion, Variables: values, CorrelationNo: ticket.TicketNo,
		IdempotencyKey: fmt.Sprintf("%s:%s:v%d:u%s", eventCode, ticket.TicketNo, businessVersion, strconv.FormatInt(recipient, 10)),
	})
	return err
}

func (s *serviceImpl) eligibleRatingRecipients(ctx context.Context, ticket *Ticket) ([]int64, error) {
	if ticket == nil || ticket.DepartmentNo == "" || s.staffAccess == nil {
		return nil, nil
	}
	ids := make([]int64, 0, 16)
	seen := map[int64]struct{}{}
	if ticket.AssigneeUserID > 0 {
		ids = append(ids, ticket.AssigneeUserID)
		seen[ticket.AssigneeUserID] = struct{}{}
	}
	var leaders []struct {
		UserID int64 `orm:"user_id"`
	}
	if err := s.database.GetScan(ctx, &leaders, `SELECT DISTINCT user_id FROM plugin_idc_core_support_department_member WHERE tenant_id=0 AND department_no=? AND role_code='leader' AND status_code='active' ORDER BY user_id LIMIT 100`, ticket.DepartmentNo); err != nil {
		return nil, err
	}
	for _, leader := range leaders {
		if _, exists := seen[leader.UserID]; !exists {
			ids = append(ids, leader.UserID)
			seen[leader.UserID] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	decisions, err := s.staffAccess.BatchCheck(ctx, staffaccesscap.BatchCheckRequest{UserIDs: ids, CapabilityCode: staffCapabilityInternalNotify, ScopeType: staffaccesscap.ScopeDepartment, ScopeKey: ticket.DepartmentNo})
	if err != nil {
		return nil, err
	}
	result := make([]int64, 0, len(decisions))
	for _, decision := range decisions {
		if decision.Eligible {
			result = append(result, decision.UserID)
		}
	}
	return result, nil
}

// onlyRecipient preserves one eligible target without broadening the original
// ordered qualification result.
func onlyRecipient(recipients []int64, target int64) []int64 {
	for _, recipient := range recipients {
		if recipient == target {
			return []int64{recipient}
		}
	}
	return nil
}
