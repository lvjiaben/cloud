// This file reads scoped support projections with shared filters for row queries and accurate pagination counts.

package supportdesk

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-plugin-idc-core/backend/cap/idccap"
)

func (s *serviceImpl) SearchStaff(ctx context.Context, input SearchInput) (*Page, error) {
	a, err := s.staffActor(ctx, PermissionList)
	if err != nil {
		return nil, err
	}
	return s.search(ctx, input, a, "")
}

func (s *serviceImpl) SearchAllStaff(ctx context.Context, input SearchInput) (*Page, error) {
	a, err := s.staffActor(ctx, PermissionViewAll)
	if err != nil {
		return nil, err
	}
	return s.search(ctx, input, a, "")
}

func (s *serviceImpl) SearchAccount(ctx context.Context, input SearchInput) (*Page, error) {
	a, err := s.customerActor(ctx)
	if err != nil {
		return nil, err
	}
	return s.search(ctx, input, nil, a.CustomerNo)
}

func (s *serviceImpl) search(ctx context.Context, input SearchInput, staff *actor, customerNo string) (*Page, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 || input.Page < 0 || (input.Page > 0 && strings.TrimSpace(input.Cursor) != "") || int64(input.Page) > math.MaxInt64/int64(limit) {
		return nil, bizInvalid()
	}
	cursorID, err := decodeCursor(strings.TrimSpace(input.Cursor))
	if err != nil {
		return nil, err
	}
	var where strings.Builder
	where.WriteString(" WHERE t.tenant_id = 0")
	args := make([]any, 0, 24)
	if customerNo != "" {
		where.WriteString(" AND t.ticket_type = 'customer' AND t.visibility_code = 'customer' AND t.customer_no = ?")
		args = append(args, customerNo)
		switch strings.TrimSpace(input.Stage) {
		case "":
		case "in_progress":
			where.WriteString(" AND t.base_stage IN ('in_progress','waiting_upstream')")
		case "new", "waiting_customer", "resolved", "closed":
			where.WriteString(" AND t.base_stage = ?")
			args = append(args, strings.TrimSpace(input.Stage))
		default:
			return nil, bizInvalid()
		}
	} else {
		appendStaffAccess(&where, &args, staff, "t")
		if stage := strings.TrimSpace(input.Stage); stage != "" {
			where.WriteString(" AND t.base_stage = ?")
			args = append(args, stage)
		}
	}
	for _, filter := range []struct{ value, column string }{
		{strings.TrimSpace(input.Status), "t.custom_status_no"},
		{strings.TrimSpace(input.Priority), "t.priority_code"},
		{strings.TrimSpace(input.Type), "t.ticket_type"}, {strings.TrimSpace(input.DepartmentNo), "t.department_no"},
	} {
		value, column := filter.value, filter.column
		if value != "" {
			where.WriteString(" AND " + column + " = ?")
			args = append(args, value)
		}
	}
	if input.AssigneeUserID > 0 {
		where.WriteString(" AND t.assignee_user_id = ?")
		args = append(args, input.AssigneeUserID)
	}
	if input.FollowUp != nil {
		where.WriteString(" AND t.follow_up_required = ?")
		args = append(args, *input.FollowUp)
	}
	queryText := strings.TrimSpace(input.Query)
	if queryText != "" {
		if len([]byte(queryText)) > 128 {
			return nil, bizInvalid()
		}
		where.WriteString(` AND (t.ticket_no = ? OR t.subject ILIKE ? ESCAPE '\')`)
		args = append(args, queryText, escapeLike(queryText)+"%")
	}
	// Count before the cursor constraint, using exactly the same ownership and
	// staff scope. This is one aggregate query, never one query per ticket.
	countQuery := "SELECT COUNT(*) FROM plugin_idc_core_support_ticket t" + where.String()
	countArgs := append([]any(nil), args...)
	if cursorID > 0 {
		where.WriteString(" AND t.id < ?")
		args = append(args, cursorID)
	}
	var offset, total int64
	if input.Page > 0 {
		offset = int64(input.Page-1) * int64(limit)
	}
	rows := make([]*Ticket, 0)
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		// The GoFrame transaction context keeps both reads on the same snapshot.
		if _, readErr := s.database.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); readErr != nil {
			return readErr
		}
		count, readErr := s.database.GetValue(ctx, countQuery, countArgs...)
		if readErr != nil {
			return readErr
		}
		total = count.Int64()
		if offset >= total {
			return nil
		}
		args = append(args, limit+1, offset)
		rows, readErr = scanTickets(ctx, s.database, "SELECT "+ticketProjection("t", "c", "customer")+" FROM plugin_idc_core_support_ticket t LEFT JOIN plugin_idc_core_support_issue_category c ON c.tenant_id=0 AND c.category_no=t.category_no LEFT JOIN plugin_idc_core_customer_subject customer ON customer.tenant_id=0 AND customer.customer_no=t.customer_no"+where.String()+" ORDER BY t.id DESC LIMIT ? OFFSET ?", args...)
		return readErr
	})
	if err != nil {
		return nil, wrapUnavailable(err, "search support tickets")
	}
	next := ""
	if len(rows) > limit {
		next = encodeCursor(rows[limit-1].ID)
		rows = rows[:limit]
	}
	if err = s.loadCurrentEntitlementProjections(ctx, rows); err != nil {
		return nil, err
	}
	page := &Page{Tickets: rows, NextCursor: next, Total: total, Actions: []string{}}
	if staff != nil {
		page.Actions = s.staffPageActions(ctx, staff)
	}
	return page, nil
}

func (s *serviceImpl) staffPageActions(ctx context.Context, staff *actor) []string {
	if staff == nil {
		return []string{}
	}
	keys := []authz.PermissionKey{PermissionQuery, PermissionCreate, PermissionConfigure, idccap.HostPermissionWildcard}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return []string{}
	}
	wildcard := grants[authz.PermissionKey(idccap.HostPermissionWildcard)]
	actions := make([]string, 0, 3)
	if wildcard || grants[authz.PermissionKey(PermissionQuery)] {
		actions = append(actions, "query")
	}
	if wildcard || grants[authz.PermissionKey(PermissionCreate)] {
		actions = append(actions, "create")
	}
	if wildcard || grants[authz.PermissionKey(PermissionConfigure)] {
		actions = append(actions, "configure")
	}
	return actions
}

func (s *serviceImpl) DetailStaff(ctx context.Context, ticketNo string) (*Detail, error) {
	a, err := s.staffActor(ctx, PermissionQuery)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, ticketNo, a, "", true)
}

func (s *serviceImpl) DetailAccount(ctx context.Context, ticketNo string) (*Detail, error) {
	a, err := s.customerActor(ctx)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, ticketNo, nil, a.CustomerNo, false)
}

func (s *serviceImpl) detail(ctx context.Context, ticketNo string, staff *actor, customerNo string, includeInternal bool) (*Detail, error) {
	if !businessNoPattern.MatchString(ticketNo) {
		return nil, bizNotAccessible()
	}
	var where strings.Builder
	where.WriteString(" WHERE t.tenant_id = 0 AND t.ticket_no = ?")
	args := []any{ticketNo}
	if customerNo != "" {
		where.WriteString(" AND t.ticket_type = 'customer' AND t.visibility_code = 'customer' AND t.customer_no = ?")
		args = append(args, customerNo)
	} else {
		appendStaffAccess(&where, &args, staff, "t")
	}
	rows, err := scanTickets(ctx, s.database, "SELECT "+ticketProjection("t", "c", "customer")+" FROM plugin_idc_core_support_ticket t LEFT JOIN plugin_idc_core_support_issue_category c ON c.tenant_id=0 AND c.category_no=t.category_no LEFT JOIN plugin_idc_core_customer_subject customer ON customer.tenant_id=0 AND customer.customer_no=t.customer_no"+where.String()+" LIMIT 1", args...)
	if err != nil {
		return nil, wrapUnavailable(err, "load support ticket")
	}
	if len(rows) != 1 {
		return nil, bizNotAccessible()
	}
	if err = s.loadCurrentEntitlementProjections(ctx, rows); err != nil {
		return nil, err
	}
	ticket := rows[0]
	var messages []Message
	messageWhere := "ticket_no = ? AND tenant_id = 0"
	if !includeInternal {
		messageWhere += " AND visibility_code = 'customer_visible' AND hidden_at IS NULL"
	}
	if err = s.database.GetScan(ctx, &messages, `SELECT message_no,type_code AS type,visibility_code AS visibility,author_type,author_user_id,body,automated,(hidden_at IS NOT NULL) AS hidden,created_at FROM (SELECT message_no,message_type AS type_code,visibility_code,author_type,author_user_id,body,automated,hidden_at,created_at,ticket_no,tenant_id,id FROM plugin_idc_core_support_ticket_message) m WHERE `+messageWhere+` ORDER BY created_at,id`, ticketNo); err != nil {
		return nil, wrapUnavailable(err, "load support messages")
	}
	attachmentsByMessage, err := s.loadAttachments(ctx, ticketNo, includeInternal)
	if err != nil {
		return nil, err
	}
	for i := range messages {
		messages[i].Attachments = attachmentsByMessage[messages[i].MessageNo]
	}
	customFields, err := s.loadCustomFieldValues(ctx, ticketNo, includeInternal)
	if err != nil {
		return nil, err
	}
	detail := &Detail{Ticket: ticket, Messages: messages, PublicMessages: make([]Message, 0), InternalNotes: make([]Message, 0), Attachments: make([]Attachment, 0), CustomFields: customFields}
	for _, message := range messages {
		detail.Attachments = append(detail.Attachments, message.Attachments...)
		if message.Visibility == "customer_visible" {
			detail.PublicMessages = append(detail.PublicMessages, message)
		} else {
			detail.InternalNotes = append(detail.InternalNotes, message)
		}
	}
	detail.Attachments = append(detail.Attachments, attachmentsByMessage[""]...)
	if includeInternal {
		if err = s.loadStaffHistory(ctx, detail); err != nil {
			return nil, err
		}
		detail.AvailableActions = s.staffAvailableActions(ctx, ticket, staff)
		detail.AvailableStatusTransitions, err = s.availableStatusTransitions(ctx, ticket, staff)
		if err != nil {
			return nil, err
		}
		if len(detail.AvailableStatusTransitions) > 0 {
			detail.AvailableActions = append(detail.AvailableActions, ActionStatusTransition)
		}
	} else if err = s.loadCustomerActionState(ctx, detail); err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *serviceImpl) loadAttachments(ctx context.Context, ticketNo string, includeInternal bool) (map[string][]Attachment, error) {
	where := "tenant_id = 0 AND ticket_no = ?"
	if !includeInternal {
		where += " AND visibility_code = 'customer_visible' AND hidden_at IS NULL"
	}
	var rows []Attachment
	if err := s.database.GetScan(ctx, &rows, `SELECT attachment_no,COALESCE(message_no,'') AS message_no,visibility_code AS visibility,file_name,media_type,size_bytes,(hidden_at IS NOT NULL) AS hidden,created_at FROM plugin_idc_core_support_ticket_attachment WHERE `+where+` ORDER BY created_at,id`, ticketNo); err != nil {
		return nil, wrapUnavailable(err, "load support attachments")
	}
	result := make(map[string][]Attachment, len(rows))
	for _, item := range rows {
		result[item.MessageNo] = append(result[item.MessageNo], item)
	}
	return result, nil
}

func (s *serviceImpl) loadStaffHistory(ctx context.Context, detail *Detail) error {
	ticketNo := detail.Ticket.TicketNo
	if err := s.database.GetScan(ctx, &detail.StatusHistory, `SELECT history_no,COALESCE(from_base_stage,'') AS from_stage,to_base_stage AS to_stage,COALESCE(custom_status_no,'') AS custom_status_no,source_code AS source,actor_user_id,COALESCE(reason_code,'') AS reason_code,COALESCE(reason_text,'') AS reason,COALESCE(rule_version,0) AS rule_version,occurred_at FROM plugin_idc_core_support_ticket_status_history WHERE tenant_id=0 AND ticket_no=? ORDER BY occurred_at,id`, ticketNo); err != nil {
		return wrapUnavailable(err, "load ticket status history")
	}
	if err := s.database.GetScan(ctx, &detail.AssignmentHistory, `SELECT history_no,action_code AS action,COALESCE(from_department_no,'') AS from_department_no,COALESCE(to_department_no,'') AS to_department_no,COALESCE(from_user_id,0) AS from_user_id,COALESCE(to_user_id,0) AS to_user_id,actor_user_id,COALESCE(reason_code,'') AS reason_code,COALESCE(reason_text,'') AS reason,occurred_at FROM plugin_idc_core_support_ticket_assignment_history WHERE tenant_id=0 AND ticket_no=? ORDER BY occurred_at,id`, ticketNo); err != nil {
		return wrapUnavailable(err, "load ticket assignment history")
	}
	if err := s.database.GetScan(ctx, &detail.Relations, `SELECT relation_no,relation_type AS type,source_ticket_no,target_ticket_no,actor_user_id,reason,created_at FROM plugin_idc_core_support_ticket_relation WHERE tenant_id=0 AND (source_ticket_no=? OR target_ticket_no=?) ORDER BY created_at,id`, ticketNo, ticketNo); err != nil {
		return wrapUnavailable(err, "load ticket relations")
	}
	if err := s.database.GetScan(ctx, &detail.Escalations, `SELECT escalation_no,from_user_id,to_user_id,reason,created_at FROM plugin_idc_core_support_ticket_escalation WHERE tenant_id=0 AND ticket_no=? ORDER BY created_at,id`, ticketNo); err != nil {
		return wrapUnavailable(err, "load ticket escalations")
	}
	if err := s.database.GetScan(ctx, &detail.SLARounds, `SELECT round_no,COALESCE(rule_no,'') AS rule_no,COALESCE(calendar_no,'') AS calendar_no,COALESCE(calendar_mode,'') AS calendar_mode,first_response_due_at,resolution_due_at,first_responded_at,resolved_at,stopped_at,paused_seconds,first_response_breached,resolution_breached FROM plugin_idc_core_support_ticket_sla_round WHERE tenant_id=0 AND ticket_no=? ORDER BY round_no`, ticketNo); err != nil {
		return wrapUnavailable(err, "load ticket SLA history")
	}
	return nil
}

func (s *serviceImpl) staffAvailableActions(ctx context.Context, ticket *Ticket, staff *actor) []string {
	if ticket == nil {
		return []string{}
	}
	permissions := []authz.PermissionKey{PermissionReply, PermissionNote, PermissionClaim, PermissionTransfer, PermissionResolve, PermissionClose, PermissionReopen, PermissionMerge, PermissionMergeAll, PermissionSplit, PermissionEscalate, PermissionHide, PermissionUpstream, idccap.HostPermissionWildcard}
	grants, err := s.authorization.BatchHasPermissions(ctx, permissions)
	if err != nil {
		return []string{}
	}
	wildcard := grants[authz.PermissionKey(idccap.HostPermissionWildcard)]
	has := func(permission string) bool { return wildcard || grants[authz.PermissionKey(permission)] }
	leader := s.isDepartmentLeader(ctx, staff.UserID, ticket.DepartmentNo)
	canManageAssigned := ticket.AssigneeUserID == int64(staff.UserID) || staff.Global || leader
	canLead := staff.Global || leader
	actions := make([]string, 0, 8)
	if ticket.BaseStage != "closed" {
		if ticket.AssigneeUserID == 0 && ticket.DepartmentNo != "" && contains(staff.Departments, ticket.DepartmentNo) && has(PermissionClaim) {
			actions = append(actions, "claim")
		}
		if has(PermissionReply) {
			actions = append(actions, "public_reply")
		}
		if has(PermissionNote) {
			actions = append(actions, "internal_note")
		}
		if has(PermissionTransfer) && canManageAssigned {
			actions = append(actions, "transfer")
		}
		if has(PermissionSplit) && canManageAssigned {
			actions = append(actions, "split")
		}
		if has(PermissionUpstream) && canManageAssigned {
			actions = append(actions, "upstream")
		}
		if has(PermissionHide) && canLead {
			actions = append(actions, "hide")
		}
		if has(PermissionMergeAll) && staff.Global {
			actions = append(actions, "merge_all")
		} else if has(PermissionMerge) && leader {
			actions = append(actions, "merge")
		}
		if ticket.BaseStage != "resolved" && has(PermissionResolve) {
			actions = append(actions, "resolve")
		}
		if has(PermissionClose) && canLead {
			actions = append(actions, "close")
		}
	} else if ticket.AssigneeUserID == 0 && has(PermissionReopen) && canLead {
		actions = append(actions, "reopen")
	}
	return actions
}

func (s *serviceImpl) loadCustomerActionState(ctx context.Context, detail *Detail) error {
	if detail == nil || detail.Ticket == nil {
		return nil
	}
	ticket := detail.Ticket
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return wrapUnavailable(err, "load customer ticket action time")
	}
	var rows []RatingProjection
	if err = s.database.GetScan(ctx, &rows, `SELECT score AS stars,round_no,COALESCE(comment,'') AS feedback,created_at AS submitted_at FROM plugin_idc_core_support_ticket_rating WHERE tenant_id=0 AND ticket_no=? AND round_no=? LIMIT 1`, ticket.TicketNo, ticket.CurrentRound); err != nil {
		return wrapUnavailable(err, "load current ticket rating")
	}
	if len(rows) == 1 {
		detail.Rating = &rows[0]
	}
	actions := make([]string, 0, 3)
	if ticket.BaseStage != "closed" {
		actions = append(actions, "reply", "close")
	}
	if ticket.BaseStage == "closed" && ticket.ClosedAt != nil {
		daysValue, loadErr := s.database.GetValue(ctx, `SELECT reopen_days FROM plugin_idc_core_support_automation_policy WHERE tenant_id=0 AND policy_no='support-default' LIMIT 1`)
		if loadErr != nil {
			return wrapUnavailable(loadErr, "load ticket reopen policy")
		}
		deadline := ticket.ClosedAt.Add(time.Duration(daysValue.Int()) * 24 * time.Hour)
		detail.ReopenDeadline = &deadline
		if ticket.AssigneeUserID == 0 && !now.After(deadline) {
			actions = append(actions, "reopen")
		}
	}
	if detail.Rating == nil && (ticket.BaseStage == "resolved" || (ticket.BaseStage == "closed" && ticket.ClosedAt != nil && !now.After(ticket.ClosedAt.Add(7*24*time.Hour)))) {
		actions = append(actions, "rate")
	}
	detail.AvailableActions = actions
	return nil
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func bizInvalid() error       { return bizerr.NewCode(CodeInvalid) }
func bizNotAccessible() error { return bizerr.NewCode(CodeNotAccessible) }
