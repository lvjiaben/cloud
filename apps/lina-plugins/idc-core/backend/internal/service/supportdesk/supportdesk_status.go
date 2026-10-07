package supportdesk

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type statusActionGrants struct {
	reply, resolve, close, reopen bool
}

func (s *serviceImpl) TransitionStatus(ctx context.Context, input StatusTransitionInput) (*Detail, error) {
	if !businessNoPattern.MatchString(input.TicketNo) ||
		!businessNoPattern.MatchString(input.CustomStatusNo) || input.TicketVersion <= 0 {
		return nil, bizInvalid()
	}
	reason := strings.TrimSpace(input.Reason)
	if len([]byte(reason)) > 512 {
		return nil, bizInvalid()
	}
	staff, err := s.staffActor(ctx, PermissionQuery)
	if err != nil {
		return nil, err
	}
	grants, err := s.loadStatusActionGrants(ctx)
	if err != nil {
		return nil, err
	}
	if s.audit == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		ticket, loadErr := s.lockAccessibleTicket(ctx, input.TicketNo, staff, "")
		if loadErr != nil {
			return loadErr
		}
		if ticket.Version != input.TicketVersion {
			return bizerr.NewCode(CodeConflict)
		}
		status, statusErr := s.lockActiveCustomStatus(ctx, input.CustomStatusNo)
		if statusErr != nil {
			return statusErr
		}
		allowed, requiresReason, requiredPermission, policyErr := s.statusTransitionAllowed(ctx, ticket, status.BaseStage, staff, grants)
		if policyErr != nil {
			return policyErr
		}
		if !allowed {
			return bizNotAccessible()
		}
		if requiresReason && reason == "" {
			return bizInvalid()
		}
		if ticket.CustomStatusNo == status.StatusNo {
			return bizerr.NewCode(CodeConflict)
		}
		return s.applyStatusTransition(ctx, ticket, status, staff, requiredPermission, reason)
	})
	if err != nil {
		return nil, wrapUnavailable(err, "transition support ticket custom status")
	}
	return s.DetailStaff(ctx, input.TicketNo)
}

func (s *serviceImpl) loadStatusActionGrants(ctx context.Context) (statusActionGrants, error) {
	keys := []authz.PermissionKey{
		PermissionReply, PermissionResolve, PermissionClose, PermissionReopen,
		idccap.HostPermissionWildcard,
	}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return statusActionGrants{}, bizerr.WrapCode(err, CodeNotAccessible)
	}
	current := s.bizCtx.Current(ctx)
	platformAdmin, err := s.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil {
		return statusActionGrants{}, bizerr.WrapCode(err, CodeNotAccessible)
	}
	wildcard := grants[authz.PermissionKey(idccap.HostPermissionWildcard)] || platformAdmin
	return statusActionGrants{
		reply:   wildcard || grants[authz.PermissionKey(PermissionReply)],
		resolve: wildcard || grants[authz.PermissionKey(PermissionResolve)],
		close:   wildcard || grants[authz.PermissionKey(PermissionClose)],
		reopen:  wildcard || grants[authz.PermissionKey(PermissionReopen)],
	}, nil
}

func (s *serviceImpl) statusTransitionAllowed(
	ctx context.Context,
	ticket *Ticket,
	targetStage string,
	staff *actor,
	grants statusActionGrants,
) (allowed, requiresReason bool, requiredPermission string, err error) {
	if ticket == nil || staff == nil {
		return false, false, "", nil
	}
	leader := staff.Global || s.isDepartmentLeader(ctx, staff.UserID, ticket.DepartmentNo)
	reopenAllowed := false
	if ticket.BaseStage == StageClosed && targetStage == StageInProgress &&
		grants.reopen && leader && ticket.AssigneeUserID == 0 {
		reopenAllowed, err = s.withinReopenWindow(ctx, ticket)
		if err != nil {
			return false, false, "", err
		}
	}
	allowed, requiresReason, requiredPermission = statusTransitionAllowedWithFacts(
		ticket, targetStage, grants, leader, reopenAllowed,
	)
	return allowed, requiresReason, requiredPermission, nil
}

func statusTransitionAllowedWithFacts(
	ticket *Ticket,
	targetStage string,
	grants statusActionGrants,
	leader,
	reopenAllowed bool,
) (allowed, requiresReason bool, requiredPermission string) {
	if ticket.BaseStage == targetStage {
		switch targetStage {
		case StageNew, StageInProgress, StageWaitingCustomer, StageWaitingUpstream:
			return grants.reply, false, PermissionReply
		case StageResolved:
			return grants.resolve, false, PermissionResolve
		case StageClosed:
			return grants.close && leader, false, PermissionClose
		default:
			return false, false, ""
		}
	}
	if ticket.BaseStage == StageClosed {
		if targetStage != StageInProgress || !reopenAllowed {
			return false, false, PermissionReopen
		}
		return true, true, PermissionReopen
	}
	if ticket.BaseStage == StageResolved {
		if targetStage == StageClosed {
			return grants.close && leader, true, PermissionClose
		}
		return false, false, ""
	}
	switch targetStage {
	case StageNew:
		return false, false, ""
	case StageInProgress, StageWaitingCustomer, StageWaitingUpstream:
		return grants.reply, false, PermissionReply
	case StageResolved:
		return grants.resolve, false, PermissionResolve
	case StageClosed:
		return grants.close && leader, true, PermissionClose
	default:
		return false, false, ""
	}
}

func (s *serviceImpl) lockActiveCustomStatus(ctx context.Context, statusNo string) (*CustomStatusSetting, error) {
	var rows []CustomStatusSetting
	if err := s.database.GetScan(ctx, &rows, `SELECT status_no,name_zh_cn,name_en_us,base_stage,sort_order,status_code AS status,version FROM plugin_idc_core_support_custom_status WHERE tenant_id=0 AND status_no=? AND status_code='active' FOR SHARE`, statusNo); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizInvalid()
	}
	return &rows[0], nil
}

func (s *serviceImpl) applyStatusTransition(
	ctx context.Context,
	ticket *Ticket,
	status *CustomStatusSetting,
	staff *actor,
	requiredPermission,
	reason string,
) error {
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return err
	}
	if ticket.BaseStage == StageWaitingCustomer && status.BaseStage != StageWaitingCustomer {
		if err = s.finishPause(ctx, ticket, now); err != nil {
			return err
		}
	}
	newRound := ticket.CurrentRound
	if ticket.BaseStage == StageClosed && status.BaseStage == StageInProgress {
		newRound++
	}
	const updateTicketSQL = `UPDATE plugin_idc_core_support_ticket SET
base_stage=?,custom_status_no=?,current_round=?,
sla_risk_code=CASE WHEN ?='in_progress' AND base_stage='closed' THEN 'none'
    WHEN ? IN ('resolved','closed') AND sla_risk_code='at_risk' THEN 'none' ELSE sla_risk_code END,
resolved_at=CASE WHEN ?='resolved' AND base_stage<>'resolved' THEN ?
    WHEN ?='in_progress' AND base_stage='closed' THEN NULL ELSE resolved_at END,
closed_at=CASE WHEN ?='closed' AND base_stage<>'closed' THEN ?
    WHEN ?='in_progress' AND base_stage='closed' THEN NULL ELSE closed_at END,
closed_reason_code=CASE WHEN ?='closed' AND base_stage<>'closed' THEN 'custom_status_transition'
    WHEN ?='in_progress' AND base_stage='closed' THEN NULL ELSE closed_reason_code END,
closed_by_type=CASE WHEN ?='closed' AND base_stage<>'closed' THEN 'leader'
    WHEN ?='in_progress' AND base_stage='closed' THEN NULL ELSE closed_by_type END,
last_activity_at=?,version=version+1,updated_by=?,updated_at=?
WHERE tenant_id=0 AND ticket_no=? AND version=?`
	result, err := s.database.Exec(
		ctx,
		updateTicketSQL,
		status.BaseStage, status.StatusNo, newRound,
		status.BaseStage, status.BaseStage,
		status.BaseStage, now, status.BaseStage,
		status.BaseStage, now, status.BaseStage,
		status.BaseStage, status.BaseStage,
		status.BaseStage, status.BaseStage,
		now, staff.UserID, now, ticket.TicketNo, ticket.Version,
	)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	if ticket.BaseStage != StageWaitingCustomer && status.BaseStage == StageWaitingCustomer {
		if err = s.startPause(ctx, ticket.TicketNo, ticket.CurrentRound, now); err != nil {
			return err
		}
	}
	if status.BaseStage == StageResolved || status.BaseStage == StageClosed {
		if _, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_sla_round SET resolved_at=COALESCE(resolved_at,?),stopped_at=CASE WHEN ?='closed' THEN ? ELSE stopped_at END WHERE tenant_id=0 AND ticket_no=? AND round_no=?`, now, status.BaseStage, now, ticket.TicketNo, ticket.CurrentRound); err != nil {
			return err
		}
	}
	if ticket.BaseStage == StageClosed && status.BaseStage == StageInProgress {
		if err = s.createSLARound(ctx, ticket.TicketNo, newRound, ticket.DepartmentNo, ticket.CategoryNo, ticket.Priority, now); err != nil {
			return err
		}
	}
	if err = s.appendCustomStatusHistory(ctx, ticket, status, staff.UserID, reason, now); err != nil {
		return err
	}
	if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "support.ticket.status-transition",
		IdempotencyKey:   ticket.TicketNo + ":" + strconv.FormatInt(ticket.Version, 10) + ":" + status.StatusNo,
		OperationCode:    "support.ticket.status-transition",
		TargetType:       "support_ticket", TargetNo: ticket.TicketNo, CustomerNo: ticket.CustomerNo,
		ResultCode: "succeeded", ReasonCode: "custom_status_transition", RiskCode: "medium",
		RequiredPermission: requiredPermission, CorrelationNo: status.StatusNo,
		BeforeSnapshot: map[string]any{
			"base_stage": ticket.BaseStage, "custom_status_no": ticket.CustomStatusNo, "version": ticket.Version,
		},
		AfterSnapshot: map[string]any{
			"base_stage": status.BaseStage, "custom_status_no": status.StatusNo, "version": ticket.Version + 1,
		},
	}); err != nil {
		return gerror.Wrap(err, "append support ticket status transition audit")
	}
	if customerStage(ticket.BaseStage) != customerStage(status.BaseStage) {
		return s.enqueueCustomerTicketEvent(ctx, ticket, supportEventTicketUpdated, ticket.Version+1)
	}
	return nil
}

func (s *serviceImpl) startPause(ctx context.Context, ticketNo string, round int, now time.Time) error {
	_, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_sla_pause (tenant_id,ticket_no,round_no,started_at,reason_code) SELECT 0,ticket_no,round_no,?,'waiting_customer' FROM plugin_idc_core_support_ticket_sla_round WHERE tenant_id=0 AND ticket_no=? AND round_no=? ON CONFLICT (ticket_no,round_no) WHERE ended_at IS NULL DO NOTHING`, now, ticketNo, round)
	return err
}

func (s *serviceImpl) appendCustomStatusHistory(ctx context.Context, ticket *Ticket, status *CustomStatusSetting, actorUserID int, reason string, now time.Time) error {
	historyNo, err := s.newNo("TSH_")
	if err != nil {
		return err
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_status_history (tenant_id,history_no,ticket_no,from_base_stage,to_base_stage,custom_status_no,source_code,actor_user_id,reason_code,reason_text,occurred_at) VALUES (0,?,?,?,?,?,'agent',?,'custom_status_transition',?,?)`, historyNo, ticket.TicketNo, nullable(ticket.BaseStage), status.BaseStage, status.StatusNo, actorUserID, nullable(reason), now)
	return err
}

func (s *serviceImpl) availableStatusTransitions(ctx context.Context, ticket *Ticket, staff *actor) ([]StatusTransitionOption, error) {
	if ticket == nil || staff == nil || s.audit == nil {
		return []StatusTransitionOption{}, nil
	}
	grants, err := s.loadStatusActionGrants(ctx)
	if err != nil {
		return nil, err
	}
	var statuses []CustomStatusSetting
	if err = s.database.GetScan(ctx, &statuses, `SELECT status_no,name_zh_cn,name_en_us,base_stage,sort_order,status_code AS status,version FROM plugin_idc_core_support_custom_status WHERE tenant_id=0 AND status_code='active' ORDER BY sort_order,id LIMIT ?`, maxSupportSettings); err != nil {
		return nil, wrapUnavailable(err, "load available support ticket status transitions")
	}
	leader := staff.Global || s.isDepartmentLeader(ctx, staff.UserID, ticket.DepartmentNo)
	reopenAllowed := false
	if ticket.BaseStage == StageClosed && grants.reopen && leader && ticket.AssigneeUserID == 0 {
		reopenAllowed, err = s.withinReopenWindow(ctx, ticket)
		if err != nil {
			return nil, wrapUnavailable(err, "evaluate support ticket reopen window")
		}
	}
	items := make([]StatusTransitionOption, 0, len(statuses))
	for _, status := range statuses {
		if status.StatusNo == ticket.CustomStatusNo {
			continue
		}
		allowed, requiresReason, _ := statusTransitionAllowedWithFacts(
			ticket, status.BaseStage, grants, leader, reopenAllowed,
		)
		if !allowed {
			continue
		}
		items = append(items, StatusTransitionOption{
			StatusNo: status.StatusNo, NameZhCN: status.NameZhCN, NameEnUS: status.NameEnUS,
			BaseStage: status.BaseStage, Version: status.Version, RequiresReason: requiresReason,
		})
	}
	return items, nil
}
