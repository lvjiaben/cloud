package supportdesk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

func (s *serviceImpl) CreateStaff(ctx context.Context, input CreateInput) (*Detail, error) {
	a, err := s.staffActor(ctx, PermissionCreate)
	if err != nil {
		return nil, err
	}
	input.TicketType = "internal"
	input.CustomerNo = ""
	if !businessNoPattern.MatchString(input.DepartmentNo) || (!a.Global && !contains(a.Departments, input.DepartmentNo)) {
		return nil, bizNotAccessible()
	}
	return s.create(ctx, input, a, "")
}

func (s *serviceImpl) CreateAccount(ctx context.Context, input CreateInput) (*Detail, error) {
	a, err := s.customerActor(ctx)
	if err != nil {
		return nil, err
	}
	input.TicketType, input.CustomerNo, input.DepartmentNo = "customer", a.CustomerNo, ""
	return s.create(ctx, input, nil, a.CustomerNo)
}

func (s *serviceImpl) create(ctx context.Context, input CreateInput, staff *actor, customerNo string) (*Detail, error) {
	subject, subjectOK := normalizeText(input.Subject, 256)
	body, bodyOK := normalizeText(input.Body, maxTicketBodyBytes)
	if !subjectOK || !bodyOK || !validPriority(input.Priority) {
		return nil, bizInvalid()
	}
	if input.Priority == "" {
		input.Priority = "normal"
	}
	if input.TicketType == "customer" && !businessNoPattern.MatchString(input.CategoryNo) {
		return nil, bizInvalid()
	}
	if input.TicketType == "internal" && !businessNoPattern.MatchString(input.DepartmentNo) {
		return nil, bizInvalid()
	}
	ticketNo, err := s.newNo("TKT_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate support ticket")
	}
	messageNo, err := s.newNo("MSG_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate support message")
	}
	historyNo, err := s.newNo("TSH_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate support status history")
	}
	assignmentNo, err := s.newNo("TAH_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate support assignment history")
	}
	actorUserID := 0
	if staff != nil {
		actorUserID = staff.UserID
	}
	visibility, authorType, messageVisibility := "customer", "customer", "customer_visible"
	if input.TicketType == "internal" {
		visibility, authorType, messageVisibility = "internal", "staff", "internal_only"
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, loadErr := databaseNow(ctx, s.database)
		if loadErr != nil {
			return loadErr
		}
		departmentNo, priority, routeErr := s.resolveRoute(ctx, input)
		if routeErr != nil {
			return routeErr
		}
		assignee, assignmentMode, assignErr := s.resolveInitialAssignee(ctx, departmentNo)
		if assignErr != nil {
			return assignErr
		}
		customFields, fieldErr := s.prepareCustomFieldSnapshots(ctx, input.TicketType, input.CustomFields)
		if fieldErr != nil {
			return fieldErr
		}
		associationReference := input.AssociationReference
		if associationReference == "" && input.TicketType != "customer" {
			associationReference = input.EntitlementNo
		}
		relatedEntitlement, entitlementErr := s.resolveRelatedEntitlement(ctx, input.TicketType, customerNo, associationReference)
		if entitlementErr != nil {
			return entitlementErr
		}
		var related EntitlementCandidate
		if relatedEntitlement != nil {
			related = *relatedEntitlement
		}
		_, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,customer_no,related_entitlement_no,related_entitlement_version,related_entitlement_name_zh_cn,related_entitlement_name_en_us,related_entitlement_status,category_no,department_no,assignee_user_id,subject,base_stage,priority_code,current_round,last_activity_at,created_by,updated_by,created_at,updated_at) VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,?,'new',?,1,?,?,?,?,?)`,
			ticketNo, input.TicketType, visibility, nullable(customerNo), nullable(related.EntitlementNo), nullableInt(related.Version), nullable(related.NameZhCN), nullable(related.NameEnUS), nullable(related.Status), nullable(input.CategoryNo), nullable(departmentNo), nullableInt(assignee), subject, priority, now, actorUserID, actorUserID, now, now)
		if insertErr != nil {
			return insertErr
		}
		if insertErr = s.saveCustomFieldSnapshots(ctx, ticketNo, customFields); insertErr != nil {
			return insertErr
		}
		_, insertErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_message (tenant_id,message_no,ticket_no,message_type,visibility_code,author_type,author_user_id,body,automated,created_at) VALUES (0,?,?,?,?,?,?,?,FALSE,?)`, messageNo, ticketNo, "initial", messageVisibility, authorType, actorUserID, body, now)
		if insertErr != nil {
			return insertErr
		}
		_, insertErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_status_history (tenant_id,history_no,ticket_no,from_base_stage,to_base_stage,source_code,actor_user_id,reason_code,occurred_at) VALUES (0,?,?,NULL,'new',?,?, 'created',?)`, historyNo, ticketNo, sourceCode(staff), actorUserID, now)
		if insertErr != nil {
			return insertErr
		}
		action := "created"
		if assignee > 0 {
			action = "auto_assigned"
		}
		_, insertErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_assignment_history (tenant_id,history_no,ticket_no,action_code,to_department_no,to_user_id,actor_user_id,reason_code,occurred_at) VALUES (0,?,?,?,?,?,?,?,?)`, assignmentNo, ticketNo, action, nullable(departmentNo), nullableInt(assignee), actorUserID, assignmentMode, now)
		if insertErr != nil {
			return insertErr
		}
		if snapshotErr := s.createSLARound(ctx, ticketNo, 1, departmentNo, input.CategoryNo, priority, now); snapshotErr != nil {
			return snapshotErr
		}
		uploadContext := "customer_initial"
		if input.TicketType == "internal" {
			uploadContext = "internal_initial"
		}
		if consumeErr := s.consumeUploads(ctx, ticketNo, messageNo, input.UploadNos, actorUserID, messageVisibility, uploadContext, now); consumeErr != nil {
			return consumeErr
		}
		return s.enqueueCustomerTicketEvent(ctx, &Ticket{
			TicketNo: ticketNo, CustomerNo: customerNo, Subject: subject,
		}, supportEventTicketCreated, 1)
	})
	if err != nil {
		return nil, wrapUnavailable(err, "create support ticket")
	}
	if staff != nil {
		return s.DetailStaff(ctx, ticketNo)
	}
	return s.DetailAccount(ctx, ticketNo)
}

func (s *serviceImpl) resolveRoute(ctx context.Context, input CreateInput) (string, string, error) {
	if input.TicketType == "internal" {
		if err := s.requireActiveOrganizationDepartment(ctx, input.DepartmentNo); err != nil {
			return "", "", err
		}
		return input.DepartmentNo, input.Priority, nil
	}
	var rows []struct {
		DepartmentNo *string `orm:"department_no"`
		Priority     string  `orm:"priority_code"`
	}
	if err := s.database.GetScan(ctx, &rows, `SELECT c.department_no,c.priority_code FROM plugin_idc_core_support_issue_category c LEFT JOIN plugin_idc_core_support_department d ON d.tenant_id=0 AND d.department_no=c.department_no AND d.status_code='active' WHERE c.tenant_id=0 AND c.category_no=? AND c.status_code='active' AND (c.department_no IS NULL OR d.department_no IS NOT NULL) LIMIT 1`, input.CategoryNo); err != nil {
		return "", "", err
	}
	if len(rows) != 1 {
		return "", "", bizInvalid()
	}
	department := ""
	if rows[0].DepartmentNo != nil {
		department = *rows[0].DepartmentNo
	}
	priority := input.Priority
	if priority == "" {
		priority = rows[0].Priority
	}
	return department, priority, nil
}

// resolveInitialAssignee keeps every newly created or split ticket in its queue.
// Legacy department assignment settings must never silently assign a person.
func (s *serviceImpl) resolveInitialAssignee(_ context.Context, departmentNo string) (int64, string, error) {
	if departmentNo == "" {
		return 0, "public_queue", nil
	}
	return 0, "manual_claim", nil
}

func (s *serviceImpl) ReplyStaff(ctx context.Context, input MessageInput) (*Detail, error) {
	a, err := s.staffActor(ctx, PermissionReply)
	if err != nil {
		return nil, err
	}
	if err = s.addMessage(ctx, input, a, "", "public_reply", "customer_visible", "staff"); err != nil {
		return nil, err
	}
	return s.DetailStaff(ctx, input.TicketNo)
}

func (s *serviceImpl) NoteStaff(ctx context.Context, input MessageInput) (*Detail, error) {
	a, err := s.staffActor(ctx, PermissionNote)
	if err != nil {
		return nil, err
	}
	if err = s.addMessage(ctx, input, a, "", "internal_note", "internal_only", "staff"); err != nil {
		return nil, err
	}
	return s.DetailStaff(ctx, input.TicketNo)
}

func (s *serviceImpl) ReplyAccount(ctx context.Context, input MessageInput) (*Detail, error) {
	a, err := s.customerActor(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.addMessage(ctx, input, nil, a.CustomerNo, "public_reply", "customer_visible", "customer"); err != nil {
		return nil, err
	}
	return s.DetailAccount(ctx, input.TicketNo)
}

func (s *serviceImpl) addMessage(ctx context.Context, input MessageInput, staff *actor, customerNo, messageType, visibility, authorType string) error {
	body, ok := normalizeText(input.Body, maxTicketBodyBytes)
	if !ok || !businessNoPattern.MatchString(input.TicketNo) || input.TicketVersion <= 0 {
		return bizInvalid()
	}
	messageNo, err := s.newNo("MSG_")
	if err != nil {
		return wrapUnavailable(err, "generate support message")
	}
	actorUserID := 0
	if staff != nil {
		actorUserID = staff.UserID
	}
	if customerNo != "" {
		current, actorErr := s.customerActor(ctx)
		if actorErr != nil {
			return actorErr
		}
		actorUserID = current.UserID
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		ticket, loadErr := s.lockAccessibleTicket(ctx, input.TicketNo, staff, customerNo)
		if loadErr != nil {
			return loadErr
		}
		if ticket.Version != input.TicketVersion || ticket.BaseStage == "closed" {
			return bizerr.NewCode(CodeConflict)
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		stage := ticket.BaseStage
		if customerNo != "" && (stage == "waiting_customer" || stage == "resolved") {
			stage = "in_progress"
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET base_stage=?,customer_last_replied_at=CASE WHEN ?='customer' THEN ? ELSE customer_last_replied_at END,last_activity_at=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`, stage, authorType, now, now, actorUserID, now, input.TicketNo, input.TicketVersion)
		if updateErr != nil {
			return updateErr
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_message (tenant_id,message_no,ticket_no,message_type,visibility_code,author_type,author_user_id,body,automated,created_at) VALUES (0,?,?,?,?,?,?,?,FALSE,?)`, messageNo, input.TicketNo, messageType, visibility, authorType, actorUserID, body, now); insertErr != nil {
			return insertErr
		}
		if customerNo != "" && stage != ticket.BaseStage {
			if historyErr := s.appendStatusHistory(ctx, input.TicketNo, ticket.BaseStage, stage, "customer", actorUserID, "customer_reply", "", now); historyErr != nil {
				return historyErr
			}
			if ticket.BaseStage == "waiting_customer" {
				if pauseErr := s.finishPause(ctx, ticket, now); pauseErr != nil {
					return pauseErr
				}
			}
		}
		if staff != nil && visibility == "customer_visible" {
			if firstErr := s.markFirstResponse(ctx, ticket, now); firstErr != nil {
				return firstErr
			}
		}
		uploadContext := messageType
		if authorType == "customer" {
			uploadContext = "customer_reply"
		}
		if consumeErr := s.consumeUploads(ctx, input.TicketNo, messageNo, input.UploadNos, actorUserID, visibility, uploadContext, now); consumeErr != nil {
			return consumeErr
		}
		if staff != nil && visibility == "customer_visible" {
			return s.enqueueCustomerTicketEvent(ctx, ticket, supportEventTicketPublicUpdated, ticket.Version+1)
		}
		return nil
	})
	return wrapUnavailable(err, "append support message")
}

func (s *serviceImpl) Claim(ctx context.Context, input ClaimInput) (*Detail, error) {
	a, err := s.staffActor(ctx, PermissionClaim)
	if err != nil {
		return nil, err
	}
	if !businessNoPattern.MatchString(input.TicketNo) || input.TicketVersion <= 0 {
		return nil, bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		ticket, lockErr := s.lockAccessibleTicket(ctx, input.TicketNo, a, "")
		if lockErr != nil {
			return lockErr
		}
		if ticket.Version != input.TicketVersion || ticket.AssigneeUserID != 0 || (ticket.BaseStage == StageClosed || ticket.BaseStage == StageResolved) || ticket.DepartmentNo == "" || !contains(a.Departments, ticket.DepartmentNo) {
			return bizerr.NewCode(CodeConflict)
		}
		allowed, allowErr := s.targetStaffAllowed(ctx, int64(a.UserID), PermissionClaim, ticket.DepartmentNo, false)
		if allowErr != nil {
			return allowErr
		}
		if !allowed {
			return bizNotAccessible()
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET assignee_user_id=?,base_stage=CASE WHEN base_stage='new' THEN 'in_progress' ELSE base_stage END,last_activity_at=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=? AND assignee_user_id IS NULL`, a.UserID, now, a.UserID, now, input.TicketNo, input.TicketVersion)
		if updateErr != nil {
			return updateErr
		}
		changed, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return affectedErr
		}
		if changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return s.appendAssignmentHistory(ctx, input.TicketNo, "claimed", ticket.DepartmentNo, ticket.DepartmentNo, 0, int64(a.UserID), a.UserID, "manual_claim", "", now)
	})
	if err != nil {
		return nil, wrapUnavailable(err, "claim support ticket")
	}
	return s.DetailStaff(ctx, input.TicketNo)
}

func (s *serviceImpl) Transfer(ctx context.Context, input TransferInput) error {
	a, err := s.staffActor(ctx, PermissionTransfer)
	if err != nil {
		return err
	}
	reason, ok := normalizeText(input.Reason, 512)
	if !ok || !businessNoPattern.MatchString(input.TicketNo) || !businessNoPattern.MatchString(input.DepartmentNo) || input.TicketVersion <= 0 || input.AssigneeUserID != 0 {
		return bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		ticket, lockErr := s.lockAccessibleTicket(ctx, input.TicketNo, a, "")
		if lockErr != nil {
			return lockErr
		}
		if ticket.Version != input.TicketVersion || ticket.BaseStage == "closed" {
			return bizerr.NewCode(CodeConflict)
		}
		leader := s.isDepartmentLeader(ctx, a.UserID, ticket.DepartmentNo)
		if !a.Global && !leader && ticket.AssigneeUserID != int64(a.UserID) {
			return bizNotAccessible()
		}
		if input.DepartmentNo != ticket.DepartmentNo && !a.Global && !leader {
			return bizNotAccessible()
		}
		if err := s.requireActiveOrganizationDepartment(ctx, input.DepartmentNo); err != nil {
			return err
		}

		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET department_no=?,assignee_user_id=NULL,last_activity_at=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`, input.DepartmentNo, now, a.UserID, now, input.TicketNo, input.TicketVersion)
		if updateErr != nil {
			return updateErr
		}
		changed, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return affectedErr
		}
		if changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return s.appendAssignmentHistory(ctx, input.TicketNo, "transferred", ticket.DepartmentNo, input.DepartmentNo, ticket.AssigneeUserID, 0, a.UserID, "manual_transfer", reason, now)
	})
	if err != nil {
		return wrapUnavailable(err, "transfer support ticket")
	}
	return nil
}

func (s *serviceImpl) Resolve(ctx context.Context, input TransitionInput) (*Detail, error) {
	a, err := s.staffActor(ctx, PermissionResolve)
	if err != nil {
		return nil, err
	}
	if err = s.transition(ctx, input, a, "", "resolved", "agent", "resolved"); err != nil {
		return nil, err
	}
	return s.DetailStaff(ctx, input.TicketNo)
}

func (s *serviceImpl) CloseStaff(ctx context.Context, input TransitionInput) (*Detail, error) {
	a, err := s.staffActor(ctx, PermissionClose)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Reason) == "" {
		return nil, bizInvalid()
	}
	if err = s.transition(ctx, input, a, "", "closed", "leader", "forced"); err != nil {
		return nil, err
	}
	return s.DetailStaff(ctx, input.TicketNo)
}

func (s *serviceImpl) ReopenStaff(ctx context.Context, input TransitionInput) (*Detail, error) {
	a, err := s.staffActor(ctx, PermissionReopen)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Reason) == "" {
		return nil, bizInvalid()
	}
	if err = s.transition(ctx, input, a, "", "in_progress", "leader", "staff_reopen"); err != nil {
		return nil, err
	}
	return s.DetailStaff(ctx, input.TicketNo)
}

func (s *serviceImpl) CloseAccount(ctx context.Context, input TransitionInput) (*Detail, error) {
	a, err := s.customerActor(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.transition(ctx, input, nil, a.CustomerNo, "closed", "customer", "customer_confirmed"); err != nil {
		return nil, err
	}
	return s.DetailAccount(ctx, input.TicketNo)
}

func (s *serviceImpl) ReopenAccount(ctx context.Context, input TransitionInput) (*Detail, error) {
	a, err := s.customerActor(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.transition(ctx, input, nil, a.CustomerNo, "in_progress", "customer", "customer_reopen"); err != nil {
		return nil, err
	}
	return s.DetailAccount(ctx, input.TicketNo)
}

func (s *serviceImpl) transition(ctx context.Context, input TransitionInput, staff *actor, customerNo, toStage, source, reasonCode string) error {
	if !businessNoPattern.MatchString(input.TicketNo) || input.TicketVersion <= 0 {
		return bizInvalid()
	}
	reason := strings.TrimSpace(input.Reason)
	if len([]byte(reason)) > 512 {
		return bizInvalid()
	}
	actorUserID := 0
	if staff != nil {
		actorUserID = staff.UserID
	} else if customerNo != "" {
		a, _ := s.customerActor(ctx)
		if a != nil {
			actorUserID = a.UserID
		}
	}
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		ticket, loadErr := s.lockAccessibleTicket(ctx, input.TicketNo, staff, customerNo)
		if loadErr != nil {
			return loadErr
		}
		if ticket.Version != input.TicketVersion {
			return bizerr.NewCode(CodeConflict)
		}
		clearAssignee, validationErr := s.validateTransition(ctx, ticket, staff, toStage, reasonCode)
		if validationErr != nil {
			return validationErr
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		newRound := ticket.CurrentRound
		if toStage == "in_progress" {
			newRound++
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET base_stage=?,current_round=?,assignee_user_id=CASE WHEN ? THEN NULL ELSE assignee_user_id END,sla_risk_code=CASE WHEN ?='in_progress' THEN 'none' WHEN ? IN ('resolved','closed') AND sla_risk_code='at_risk' THEN 'none' ELSE sla_risk_code END,resolved_at=CASE WHEN ?='resolved' THEN ? WHEN ?='in_progress' THEN NULL ELSE resolved_at END,closed_at=CASE WHEN ?='closed' THEN ? WHEN ?='in_progress' THEN NULL ELSE closed_at END,closed_reason_code=CASE WHEN ?='closed' THEN ? WHEN ?='in_progress' THEN NULL ELSE closed_reason_code END,closed_by_type=CASE WHEN ?='closed' THEN ? WHEN ?='in_progress' THEN NULL ELSE closed_by_type END,last_activity_at=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`,
			toStage, newRound, clearAssignee,
			toStage, toStage,
			toStage, now, toStage,
			toStage, now, toStage,
			toStage, reasonCode, toStage,
			toStage, source, toStage,
			now, actorUserID, now, input.TicketNo, input.TicketVersion,
		)
		if updateErr != nil {
			return updateErr
		}
		changed, affectedErr := result.RowsAffected()
		if affectedErr != nil {
			return affectedErr
		}
		if changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if historyErr := s.appendStatusHistory(ctx, input.TicketNo, ticket.BaseStage, toStage, source, actorUserID, reasonCode, reason, now); historyErr != nil {
			return historyErr
		}
		if toStage == "resolved" || toStage == "closed" {
			if _, stopErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_sla_round SET resolved_at=COALESCE(resolved_at,?),stopped_at=CASE WHEN ?='closed' THEN ? ELSE stopped_at END WHERE tenant_id=0 AND ticket_no=? AND round_no=?`, now, toStage, now, input.TicketNo, ticket.CurrentRound); stopErr != nil {
				return stopErr
			}
		}
		if toStage == "in_progress" {
			if clearAssignee {
				if historyErr := s.appendAssignmentHistory(ctx, input.TicketNo, "unassigned", ticket.DepartmentNo, ticket.DepartmentNo, ticket.AssigneeUserID, 0, actorUserID, "qualification_revoked", reason, now); historyErr != nil {
					return historyErr
				}
			}
			if slaErr := s.createSLARound(ctx, input.TicketNo, newRound, ticket.DepartmentNo, ticket.CategoryNo, ticket.Priority, now); slaErr != nil {
				return slaErr
			}
		}
		if staff != nil {
			return s.enqueueCustomerTicketEvent(ctx, ticket, supportEventTicketUpdated, ticket.Version+1)
		}
		return nil
	})
	return wrapUnavailable(err, "transition support ticket")
}

func (s *serviceImpl) validateTransition(ctx context.Context, ticket *Ticket, staff *actor, toStage, reasonCode string) (bool, error) {
	if staff != nil && (reasonCode == "forced" || reasonCode == "staff_reopen") && !staff.Global && !s.isDepartmentLeader(ctx, staff.UserID, ticket.DepartmentNo) {
		return false, bizNotAccessible()
	}
	if toStage == "resolved" && (ticket.BaseStage == "resolved" || ticket.BaseStage == "closed") {
		return false, bizerr.NewCode(CodeConflict)
	}
	if toStage == "closed" && ticket.BaseStage == "closed" {
		return false, bizerr.NewCode(CodeConflict)
	}
	if toStage != "in_progress" {
		return false, nil
	}
	if ticket.BaseStage != "closed" {
		return false, bizerr.NewCode(CodeConflict)
	}
	allowed, err := s.withinReopenWindow(ctx, ticket)
	if err != nil {
		return false, err
	}
	if !allowed {
		return false, bizerr.NewCode(CodeConflict)
	}
	// Reopened work is always returned to its original queue for a fresh claim.
	return ticket.AssigneeUserID != 0, nil
}

func (s *serviceImpl) RateAccount(ctx context.Context, input RatingInput) (*Detail, error) {
	a, err := s.customerActor(ctx)
	if err != nil {
		return nil, err
	}
	if input.Stars < 1 || input.Stars > 5 || input.RoundNo <= 0 || input.TicketVersion <= 0 || len([]byte(input.Feedback)) > 2000 {
		return nil, bizInvalid()
	}
	ratingNo, err := s.newNo("RAT_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate ticket rating")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		ticket, loadErr := s.lockAccessibleTicket(ctx, input.TicketNo, nil, a.CustomerNo)
		if loadErr != nil {
			return loadErr
		}
		if ticket.Version != input.TicketVersion || input.RoundNo != ticket.CurrentRound || (ticket.BaseStage != "resolved" && ticket.BaseStage != "closed") {
			return bizerr.NewCode(CodeConflict)
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if ticket.ClosedAt != nil && now.Sub(*ticket.ClosedAt) > 7*24*time.Hour {
			return bizerr.NewCode(CodeConflict)
		}
		_, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_rating (tenant_id,rating_no,ticket_no,round_no,score,comment,author_type,created_at) VALUES (0,?,?,?,?,?,'customer',?)`, ratingNo, input.TicketNo, input.RoundNo, input.Stars, nullable(strings.TrimSpace(input.Feedback)), now)
		if insertErr != nil {
			return bizerr.NewCode(CodeConflict)
		}
		if input.Stars <= 2 {
			result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET follow_up_required=TRUE,version=version+1,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`, now, ticket.TicketNo, ticket.Version)
			if updateErr != nil {
				return updateErr
			}
			if changed, _ := result.RowsAffected(); changed != 1 {
				return bizerr.NewCode(CodeConflict)
			}
		}
		return nil
	})
	if err != nil {
		return nil, wrapUnavailable(err, "rate support ticket")
	}
	return s.DetailAccount(ctx, input.TicketNo)
}

func (s *serviceImpl) lockAccessibleTicket(ctx context.Context, ticketNo string, staff *actor, customerNo string) (*Ticket, error) {
	var where strings.Builder
	where.WriteString(" WHERE t.tenant_id=0 AND t.ticket_no=?")
	args := []any{ticketNo}
	if customerNo != "" {
		where.WriteString(" AND t.ticket_type='customer' AND t.visibility_code='customer' AND t.customer_no=?")
		args = append(args, customerNo)
	} else {
		appendStaffAccess(&where, &args, staff, "t")
	}
	rows, err := scanTickets(ctx, s.database, "SELECT "+ticketProjection("t", "c", "customer")+" FROM plugin_idc_core_support_ticket t LEFT JOIN plugin_idc_core_support_issue_category c ON c.tenant_id=0 AND c.category_no=t.category_no LEFT JOIN plugin_idc_core_customer_subject customer ON customer.tenant_id=0 AND customer.customer_no=t.customer_no"+where.String()+" FOR UPDATE OF t", args...)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizNotAccessible()
	}
	return rows[0], nil
}

func (s *serviceImpl) appendStatusHistory(ctx context.Context, ticketNo, from, to, source string, actorUserID int, reasonCode, reason string, now time.Time) error {
	no, err := s.newNo("TSH_")
	if err != nil {
		return err
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_status_history (tenant_id,history_no,ticket_no,from_base_stage,to_base_stage,source_code,actor_user_id,reason_code,reason_text,occurred_at) VALUES (0,?,?,?,?,?,?,?,?,?)`, no, ticketNo, nullable(from), to, source, actorUserID, nullable(reasonCode), nullable(reason), now)
	return err
}
func (s *serviceImpl) appendAssignmentHistory(ctx context.Context, ticketNo, action, fromDept, toDept string, fromUser, toUser int64, actorUser int, reasonCode, reason string, now time.Time) error {
	no, err := s.newNo("TAH_")
	if err != nil {
		return err
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_assignment_history (tenant_id,history_no,ticket_no,action_code,from_department_no,to_department_no,from_user_id,to_user_id,actor_user_id,reason_code,reason_text,occurred_at) VALUES (0,?,?,?,?,?,?,?,?,?,?,?)`, no, ticketNo, action, nullable(fromDept), nullable(toDept), nullableInt(fromUser), nullableInt(toUser), actorUser, nullable(reasonCode), nullable(reason), now)
	return err
}

func (s *serviceImpl) markFirstResponse(ctx context.Context, ticket *Ticket, now time.Time) error {
	_, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_sla_round SET first_responded_at=COALESCE(first_responded_at,?) WHERE tenant_id=0 AND ticket_no=? AND round_no=?`, now, ticket.TicketNo, ticket.CurrentRound)
	return err
}
func (s *serviceImpl) finishPause(ctx context.Context, ticket *Ticket, now time.Time) error {
	var starts []struct {
		StartedAt time.Time `orm:"started_at"`
	}
	if err := s.database.GetScan(ctx, &starts, `UPDATE plugin_idc_core_support_ticket_sla_pause SET ended_at=? WHERE tenant_id=0 AND ticket_no=? AND round_no=? AND ended_at IS NULL RETURNING started_at`, now, ticket.TicketNo, ticket.CurrentRound); err != nil {
		return err
	}
	if len(starts) == 1 {
		seconds := int64(now.Sub(starts[0].StartedAt).Seconds())
		_, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_sla_round SET paused_seconds=paused_seconds+?,resolution_due_at=resolution_due_at+(?*INTERVAL '1 second') WHERE tenant_id=0 AND ticket_no=? AND round_no=?`, seconds, seconds, ticket.TicketNo, ticket.CurrentRound)
		return err
	}
	return nil
}

func (s *serviceImpl) withinReopenWindow(ctx context.Context, ticket *Ticket) (bool, error) {
	if ticket.ClosedAt == nil {
		return false, nil
	}
	daysValue, err := s.database.GetValue(ctx, `SELECT reopen_days FROM plugin_idc_core_support_automation_policy WHERE tenant_id=0 AND policy_no='support-default'`)
	if err != nil {
		return false, err
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return false, err
	}
	return !now.After(ticket.ClosedAt.Add(time.Duration(daysValue.Int()) * 24 * time.Hour)), nil
}

// isDepartmentLeader checks the current actor's explicit supervisor permission
// and live organization membership; historical queue-member rows grant nothing.
func (s *serviceImpl) isDepartmentLeader(ctx context.Context, userID int, departmentNo string) bool {
	allowed, err := s.targetDepartmentMember(ctx, int64(userID), departmentNo, true)
	return err == nil && allowed
}

func databaseNow(ctx context.Context, database gdb.DB) (time.Time, error) {
	value, err := database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return time.Time{}, err
	}
	return value.Time().UTC(), nil
}
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func nullableInt(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func validPriority(value string) bool {
	return value == "" || value == "low" || value == "normal" || value == "high" || value == "urgent"
}
func sourceCode(staff *actor) string {
	if staff == nil {
		return "customer"
	}
	return "agent"
}

var _ = fmt.Sprintf
