package supportdesk

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

func (s *serviceImpl) MergeCandidates(ctx context.Context, ticketNo, cursor string, limit int) (*MergeCandidatePage, error) {
	return s.mergeCandidates(ctx, ticketNo, cursor, limit, PermissionMerge, false)
}

func (s *serviceImpl) MergeAllCandidates(ctx context.Context, ticketNo, cursor string, limit int) (*MergeCandidatePage, error) {
	return s.mergeCandidates(ctx, ticketNo, cursor, limit, PermissionMergeAll, true)
}

func (s *serviceImpl) mergeCandidates(ctx context.Context, ticketNo, cursor string, limit int, permission string, requireGlobal bool) (*MergeCandidatePage, error) {
	a, err := s.staffActor(ctx, permission)
	if err != nil {
		return nil, err
	}
	if !businessNoPattern.MatchString(ticketNo) {
		return nil, bizInvalid()
	}
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return nil, bizInvalid()
	}
	cursorID, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	current, err := s.detail(ctx, ticketNo, a, "", true)
	if err != nil {
		return nil, err
	}
	if (requireGlobal && !a.Global) || (!requireGlobal && !a.Global && !s.isDepartmentLeader(ctx, a.UserID, current.Ticket.DepartmentNo)) {
		return nil, bizNotAccessible()
	}
	var where strings.Builder
	where.WriteString(` WHERE t.tenant_id=0 AND t.ticket_no<>? AND t.base_stage<>'closed' AND t.ticket_type=? AND t.visibility_code=? AND t.customer_no IS NOT DISTINCT FROM ? AND t.anonymous_destination_kind IS NOT DISTINCT FROM ? AND t.anonymous_destination_digest IS NOT DISTINCT FROM ?`)
	args := []any{ticketNo, current.Ticket.TicketType, current.Ticket.Visibility, nullable(current.Ticket.CustomerNo), nullable(current.Ticket.AnonymousDestinationKind), nullable(current.Ticket.AnonymousDestinationDigest)}
	if cursorID > 0 {
		where.WriteString(` AND t.id<?`)
		args = append(args, cursorID)
	}
	appendStaffAccess(&where, &args, a, "t")
	args = append(args, limit+1)
	var rows []struct {
		ID                       int64
		TicketNo, Subject, Stage string
		Version                  int64
	}
	if err = s.database.GetScan(ctx, &rows, `SELECT t.id,t.ticket_no,t.subject,t.base_stage AS stage,t.version FROM plugin_idc_core_support_ticket t`+where.String()+` ORDER BY t.id DESC LIMIT ?`, args...); err != nil {
		return nil, wrapUnavailable(err, "load merge candidates")
	}
	next := ""
	if len(rows) > limit {
		next = encodeCursor(rows[limit-1].ID)
		rows = rows[:limit]
	}
	items := make([]MergeCandidate, 0, len(rows))
	for _, row := range rows {
		items = append(items, MergeCandidate{TicketNo: row.TicketNo, Subject: row.Subject, Stage: row.Stage, Version: row.Version})
	}
	return &MergeCandidatePage{Items: items, NextCursor: next}, nil
}

func (s *serviceImpl) Merge(ctx context.Context, input MergeInput, global bool) (*Detail, error) {
	permission := PermissionMerge
	if global {
		permission = PermissionMergeAll
	}
	a, err := s.staffActor(ctx, permission)
	if err != nil {
		return nil, err
	}
	reason, ok := normalizeText(input.Reason, 512)
	if !ok || !businessNoPattern.MatchString(input.Main.TicketNo) || input.Main.Version <= 0 || len(input.Sources) == 0 || len(input.Sources) > 20 {
		return nil, bizInvalid()
	}
	seen := map[string]struct{}{input.Main.TicketNo: {}}
	for _, source := range input.Sources {
		if !businessNoPattern.MatchString(source.TicketNo) || source.Version <= 0 {
			return nil, bizInvalid()
		}
		if _, exists := seen[source.TicketNo]; exists {
			return nil, bizInvalid()
		}
		seen[source.TicketNo] = struct{}{}
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return s.mergeLocked(ctx, input, a, global, reason)
	})
	if err != nil {
		return nil, wrapUnavailable(err, "merge support tickets")
	}
	return s.DetailStaff(ctx, input.Main.TicketNo)
}

func (s *serviceImpl) mergeLocked(ctx context.Context, input MergeInput, a *actor, global bool, reason string) error {
	main, loadErr := s.lockAccessibleTicket(ctx, input.Main.TicketNo, a, "")
	if loadErr != nil {
		return loadErr
	}
	if main.Version != input.Main.Version || main.BaseStage == "closed" {
		return bizerr.NewCode(CodeConflict)
	}
	if !global && !s.isDepartmentLeader(ctx, a.UserID, main.DepartmentNo) {
		return bizNotAccessible()
	}
	sources := make([]*Ticket, 0, len(input.Sources))
	for _, requested := range input.Sources {
		source, sourceErr := s.lockAccessibleTicket(ctx, requested.TicketNo, a, "")
		if sourceErr != nil {
			return sourceErr
		}
		if source.Version != requested.Version || source.BaseStage == "closed" ||
			source.TicketType != main.TicketType || source.Visibility != main.Visibility || source.CustomerNo != main.CustomerNo ||
			source.AnonymousDestinationKind != main.AnonymousDestinationKind || source.AnonymousDestinationDigest != main.AnonymousDestinationDigest {
			return bizerr.NewCode(CodeConflict)
		}
		if !global && !s.isDepartmentLeader(ctx, a.UserID, source.DepartmentNo) {
			return bizNotAccessible()
		}
		sources = append(sources, source)
	}
	// Anonymous merges require delivery of a fresh main-ticket link. Until the
	// verified-destination sender is installed, fail before any state change.
	if main.TicketType == "presales" {
		return bizerr.NewCode(CodeAnonymousLinkUnconfigured)
	}
	now, nowErr := databaseNow(ctx, s.database)
	if nowErr != nil {
		return nowErr
	}
	mainResult, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET last_activity_at=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`, now, a.UserID, now, main.TicketNo, input.Main.Version)
	if updateErr != nil {
		return updateErr
	}
	if changed, _ := mainResult.RowsAffected(); changed != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	for i, source := range sources {
		relationNo, noErr := s.newNo("REL_")
		if noErr != nil {
			return noErr
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET base_stage='closed',closed_at=?,closed_reason_code='merged',closed_by_type='leader',last_activity_at=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`, now, now, a.UserID, now, source.TicketNo, input.Sources[i].Version)
		if updateErr != nil {
			return updateErr
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if _, updateErr = s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_sla_round SET stopped_at=COALESCE(stopped_at,?) WHERE tenant_id=0 AND ticket_no=? AND round_no=?`, now, source.TicketNo, source.CurrentRound); updateErr != nil {
			return updateErr
		}
		if updateErr = s.appendStatusHistory(ctx, source.TicketNo, source.BaseStage, "closed", "merge", a.UserID, "merged", reason, now); updateErr != nil {
			return updateErr
		}
		if _, updateErr = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_relation (tenant_id,relation_no,relation_type,source_ticket_no,target_ticket_no,actor_user_id,reason,created_at) VALUES (0,?,'merged_into',?,?,?,?,?)`, relationNo, source.TicketNo, main.TicketNo, a.UserID, reason, now); updateErr != nil {
			return updateErr
		}
	}
	return nil
}

func (s *serviceImpl) Split(ctx context.Context, input SplitInput) (*Detail, error) {
	a, err := s.staffActor(ctx, PermissionSplit)
	if err != nil {
		return nil, err
	}
	reason, ok := normalizeText(input.Reason, 512)
	if !ok || !businessNoPattern.MatchString(input.TicketNo) || !businessNoPattern.MatchString(input.CategoryNo) || input.TicketVersion <= 0 || len(input.MessageNos) > 100 || len(input.AttachmentNos) > 100 {
		return nil, bizInvalid()
	}
	messageNos, ok := uniqueBusinessNos(input.MessageNos)
	if !ok {
		return nil, bizInvalid()
	}
	attachmentNos, ok := uniqueBusinessNos(input.AttachmentNos)
	if !ok {
		return nil, bizInvalid()
	}
	newTicketNo, err := s.newNo("TKT_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate split ticket")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return s.splitLocked(ctx, input, a, reason, messageNos, attachmentNos, newTicketNo)
	})
	if err != nil {
		return nil, wrapUnavailable(err, "split support ticket")
	}
	return s.DetailStaff(ctx, newTicketNo)
}

func (s *serviceImpl) splitLocked(
	ctx context.Context,
	input SplitInput,
	a *actor,
	reason string,
	messageNos []string,
	attachmentNos []string,
	newTicketNo string,
) error {
	source, loadErr := s.lockAccessibleTicket(ctx, input.TicketNo, a, "")
	if loadErr != nil {
		return loadErr
	}
	if source.Version != input.TicketVersion || source.BaseStage == "closed" {
		return bizerr.NewCode(CodeConflict)
	}
	if source.AssigneeUserID != int64(a.UserID) && !a.Global && !s.isDepartmentLeader(ctx, a.UserID, source.DepartmentNo) {
		return bizNotAccessible()
	}
	if source.TicketType == "presales" {
		return bizerr.NewCode(CodeAnonymousLinkUnconfigured)
	}
	if source.TicketType != "customer" {
		return bizInvalid()
	}
	messages, loadErr := s.selectedPublicMessages(ctx, source.TicketNo, messageNos)
	if loadErr != nil {
		return loadErr
	}
	attachments, loadErr := s.selectedPublicAttachments(ctx, source.TicketNo, attachmentNos)
	if loadErr != nil {
		return loadErr
	}
	route := CreateInput{TicketType: source.TicketType, CustomerNo: source.CustomerNo, CategoryNo: input.CategoryNo, Priority: source.Priority}
	departmentNo, priority, routeErr := s.resolveRoute(ctx, route)
	if routeErr != nil {
		return routeErr
	}
	assignee, assignmentMode, assignErr := s.resolveInitialAssignee(ctx, departmentNo)
	if assignErr != nil {
		return assignErr
	}
	now, nowErr := databaseNow(ctx, s.database)
	if nowErr != nil {
		return nowErr
	}
	sourceResult, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET last_activity_at=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`, now, a.UserID, now, source.TicketNo, input.TicketVersion)
	if updateErr != nil {
		return updateErr
	}
	if changed, _ := sourceResult.RowsAffected(); changed != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	insertResult, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket (
		tenant_id,ticket_no,ticket_type,visibility_code,customer_no,category_no,department_no,assignee_user_id,
		subject,base_stage,priority_code,current_round,last_activity_at,
		related_entitlement_no,related_entitlement_version,related_entitlement_name_zh_cn,related_entitlement_name_en_us,
		related_entitlement_status,
		created_by,updated_by,created_at,updated_at
	) SELECT 0,?,'customer','customer',customer_no,?,?,?,subject,'new',?,1,?,
		related_entitlement_no,related_entitlement_version,related_entitlement_name_zh_cn,related_entitlement_name_en_us,
		related_entitlement_status,?,?,?,?
	FROM plugin_idc_core_support_ticket WHERE tenant_id=0 AND ticket_no=?`,
		newTicketNo, input.CategoryNo, nullable(departmentNo), nullableInt(assignee), priority, now,
		a.UserID, a.UserID, now, now, source.TicketNo)
	if insertErr != nil {
		return insertErr
	}
	if changed, rowsErr := insertResult.RowsAffected(); rowsErr != nil || changed != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	if copyErr := s.copySplitCustomFieldSnapshots(ctx, source.TicketNo, newTicketNo); copyErr != nil {
		return copyErr
	}
	messageMap := make(map[string]string, len(messages))
	for _, message := range messages {
		newMessageNo, noErr := s.newNo("MSG_")
		if noErr != nil {
			return noErr
		}
		messageMap[message.MessageNo] = newMessageNo
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_message (tenant_id,message_no,ticket_no,message_type,visibility_code,author_type,author_user_id,body,automated,created_at) SELECT 0,?,? ,message_type,'customer_visible',author_type,author_user_id,body,automated,? FROM plugin_idc_core_support_ticket_message WHERE tenant_id=0 AND ticket_no=? AND message_no=? AND visibility_code='customer_visible' AND hidden_at IS NULL`, newMessageNo, newTicketNo, now, source.TicketNo, message.MessageNo); insertErr != nil {
			return insertErr
		}
	}
	for _, attachment := range attachments {
		newAttachmentNo, noErr := s.newNo("ATT_")
		if noErr != nil {
			return noErr
		}
		newMessageNo := nullable(messageMap[attachment.MessageNo])
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_attachment (tenant_id,attachment_no,ticket_no,message_no,visibility_code,file_name,media_type,size_bytes,sha256_hex,object_ref,inspection_code,created_by,created_at) SELECT 0,?,?,?,'customer_visible',file_name,media_type,size_bytes,sha256_hex,object_ref,inspection_code,?,? FROM plugin_idc_core_support_ticket_attachment WHERE tenant_id=0 AND ticket_no=? AND attachment_no=? AND visibility_code='customer_visible' AND hidden_at IS NULL`, newAttachmentNo, newTicketNo, newMessageNo, a.UserID, now, source.TicketNo, attachment.AttachmentNo); insertErr != nil {
			return insertErr
		}
	}
	if historyErr := s.appendStatusHistory(ctx, newTicketNo, "", "new", "agent", a.UserID, "split", reason, now); historyErr != nil {
		return historyErr
	}
	assignmentAction := "created"
	if assignee > 0 {
		assignmentAction = "auto_assigned"
	}
	if historyErr := s.appendAssignmentHistory(ctx, newTicketNo, assignmentAction, "", departmentNo, 0, assignee, a.UserID, assignmentMode, reason, now); historyErr != nil {
		return historyErr
	}
	if slaErr := s.createSLARound(ctx, newTicketNo, 1, departmentNo, input.CategoryNo, priority, now); slaErr != nil {
		return slaErr
	}
	relationNo, noErr := s.newNo("REL_")
	if noErr != nil {
		return noErr
	}
	if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_relation (tenant_id,relation_no,relation_type,source_ticket_no,target_ticket_no,actor_user_id,reason,created_at) VALUES (0,?,'split_from',?,?,?,?,?)`, relationNo, newTicketNo, source.TicketNo, a.UserID, reason, now); insertErr != nil {
		return insertErr
	}
	return s.enqueueCustomerTicketEvent(ctx, &Ticket{
		TicketNo: newTicketNo, CustomerNo: source.CustomerNo, Subject: source.Subject,
	}, supportEventTicketCreated, 1)
}

func (s *serviceImpl) copySplitCustomFieldSnapshots(ctx context.Context, sourceTicketNo, targetTicketNo string) error {
	if _, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_custom_field_value (
		tenant_id,ticket_no,field_no,definition_version,field_type,name_zh_cn,name_en_us,description_zh_cn,
		description_en_us,visibility_code,required,sort_order,normalized_value,selected_option_count,created_at
	) SELECT 0,?,field_no,definition_version,field_type,name_zh_cn,name_en_us,description_zh_cn,
		description_en_us,visibility_code,required,sort_order,normalized_value,selected_option_count,created_at
	FROM plugin_idc_core_support_ticket_custom_field_value
	WHERE tenant_id=0 AND ticket_no=? ORDER BY id`, targetTicketNo, sourceTicketNo); err != nil {
		return err
	}
	_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_custom_field_value_option (
		tenant_id,ticket_no,field_no,option_no,name_zh_cn,name_en_us,sort_order,created_at
	) SELECT 0,?,field_no,option_no,name_zh_cn,name_en_us,sort_order,created_at
	FROM plugin_idc_core_support_ticket_custom_field_value_option
	WHERE tenant_id=0 AND ticket_no=? ORDER BY id`, targetTicketNo, sourceTicketNo)
	return err
}

func (s *serviceImpl) Escalate(ctx context.Context, input EscalateInput) (*Detail, error) {
	a, err := s.staffActor(ctx, PermissionEscalate)
	if err != nil {
		return nil, err
	}
	reason, ok := normalizeText(input.Reason, 512)
	if !ok || !businessNoPattern.MatchString(input.TicketNo) || input.ToUserID <= 0 || input.ToUserID == int64(a.UserID) || input.TicketVersion <= 0 {
		return nil, bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		ticket, loadErr := s.lockAccessibleTicket(ctx, input.TicketNo, a, "")
		if loadErr != nil {
			return loadErr
		}
		if ticket.Version != input.TicketVersion || ticket.BaseStage == StageClosed {
			return bizerr.NewCode(CodeConflict)
		}
		if !a.Global && ticket.AssigneeUserID != int64(a.UserID) && !s.isDepartmentLeader(ctx, a.UserID, ticket.DepartmentNo) {
			return bizNotAccessible()
		}
		allowed, allowErr := s.targetStaffAllowed(ctx, input.ToUserID, staffCapabilityLead, ticket.DepartmentNo, true)
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
		escalationNo, noErr := s.newNo("ESC_")
		if noErr != nil {
			return noErr
		}
		if _, writeErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_escalation (tenant_id,escalation_no,ticket_no,from_user_id,to_user_id,reason,created_at) VALUES (0,?,?,?,?,?,?)`, escalationNo, input.TicketNo, a.UserID, input.ToUserID, reason, now); writeErr != nil {
			return writeErr
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET last_activity_at=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`, now, a.UserID, now, input.TicketNo, input.TicketVersion)
		if updateErr != nil {
			return updateErr
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return s.enqueueStaffTicketEvent(ctx, ticket, supportEventTicketEscalated, input.ToUserID, ticket.Version+1, map[string]string{"status": ticket.BaseStage, "message": reason})
	})
	if err != nil {
		return nil, wrapUnavailable(err, "escalate support ticket")
	}
	return s.DetailStaff(ctx, input.TicketNo)
}

func (s *serviceImpl) Hide(ctx context.Context, input HideInput) (*Detail, error) {
	a, err := s.staffActor(ctx, PermissionHide)
	if err != nil {
		return nil, err
	}
	reason, ok := normalizeText(input.Reason, 512)
	if !ok || !businessNoPattern.MatchString(input.TicketNo) || !businessNoPattern.MatchString(input.TargetNo) || input.TicketVersion <= 0 || (input.TargetType != "message" && input.TargetType != "attachment") {
		return nil, bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		ticket, loadErr := s.lockAccessibleTicket(ctx, input.TicketNo, a, "")
		if loadErr != nil {
			return loadErr
		}
		if ticket.Version != input.TicketVersion || (!a.Global && !s.isDepartmentLeader(ctx, a.UserID, ticket.DepartmentNo)) {
			return bizerr.NewCode(CodeConflict)
		}
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		table, column := "plugin_idc_core_support_ticket_message", "message_no"
		if input.TargetType == "attachment" {
			table, column = "plugin_idc_core_support_ticket_attachment", "attachment_no"
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE `+table+` SET hidden_at=?,hidden_by=?,hidden_reason=? WHERE tenant_id=0 AND ticket_no=? AND `+column+`=? AND hidden_at IS NULL`, now, a.UserID, reason, input.TicketNo, input.TargetNo)
		if updateErr != nil {
			return updateErr
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		result, updateErr = s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET last_activity_at=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND version=?`, now, a.UserID, now, input.TicketNo, input.TicketVersion)
		if updateErr != nil {
			return updateErr
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return nil
	})
	if err != nil {
		return nil, wrapUnavailable(err, "hide support content")
	}
	return s.DetailStaff(ctx, input.TicketNo)
}

func (s *serviceImpl) selectedPublicMessages(ctx context.Context, ticketNo string, messageNos []string) ([]Message, error) {
	if len(messageNos) == 0 {
		return []Message{}, nil
	}
	args := make([]any, 0, len(messageNos)+1)
	args = append(args, ticketNo)
	for _, value := range messageNos {
		args = append(args, value)
	}
	var rows []Message
	if err := s.database.GetScan(ctx, &rows, `SELECT message_no FROM plugin_idc_core_support_ticket_message WHERE tenant_id=0 AND ticket_no=? AND visibility_code='customer_visible' AND hidden_at IS NULL AND message_no IN (`+sqlPlaceholders(len(messageNos))+`) ORDER BY id`, args...); err != nil {
		return nil, err
	}
	if len(rows) != len(messageNos) {
		return nil, bizNotAccessible()
	}
	return rows, nil
}

func (s *serviceImpl) selectedPublicAttachments(ctx context.Context, ticketNo string, attachmentNos []string) ([]Attachment, error) {
	if len(attachmentNos) == 0 {
		return []Attachment{}, nil
	}
	args := make([]any, 0, len(attachmentNos)+1)
	args = append(args, ticketNo)
	for _, value := range attachmentNos {
		args = append(args, value)
	}
	var rows []Attachment
	if err := s.database.GetScan(ctx, &rows, `SELECT attachment_no,COALESCE(message_no,'') AS message_no FROM plugin_idc_core_support_ticket_attachment WHERE tenant_id=0 AND ticket_no=? AND visibility_code='customer_visible' AND hidden_at IS NULL AND attachment_no IN (`+sqlPlaceholders(len(attachmentNos))+`) ORDER BY id`, args...); err != nil {
		return nil, err
	}
	if len(rows) != len(attachmentNos) {
		return nil, bizNotAccessible()
	}
	return rows, nil
}

func uniqueBusinessNos(values []string) ([]string, bool) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !businessNoPattern.MatchString(value) {
			return nil, false
		}
		if _, exists := seen[value]; exists {
			return nil, false
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, true
}
