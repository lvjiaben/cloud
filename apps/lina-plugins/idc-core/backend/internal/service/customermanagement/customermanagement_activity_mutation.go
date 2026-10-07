package customermanagement

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

type NoteInput struct {
	CustomerNo     string
	Category       string
	Content        string
	SupersedesNo   string
	IdempotencyKey string
}

type NoteResult struct {
	NoteNo       string
	CustomerNo   string
	Category     string
	Status       string
	SupersedesNo string
	CreatedAt    time.Time
}

type FollowUpPlanInput struct {
	CustomerNo     string
	Channel        string
	Subject        string
	Detail         string
	AssignedUserID int
	PlannedAt      time.Time
	IdempotencyKey string
}

type FollowUpTransitionInput struct {
	CustomerNo     string
	FollowUpNo     string
	Action         string
	Outcome        string
	IdempotencyKey string
}

type FollowUpResult struct {
	FollowUpNo string
	CustomerNo string
	Status     string
	PlannedAt  time.Time
	UpdatedAt  time.Time
}

type BlacklistCreateInput struct {
	CustomerNo        string
	Scope             string
	ReasonCode        string
	ReasonDetail      string
	EvidenceReference string
	RiskLevel         string
	StartsAt          time.Time
	ExpiresAt         *time.Time
	IdempotencyKey    string
}

type BlacklistDecisionInput struct {
	CustomerNo     string
	EntryNo        string
	Action         string
	IdempotencyKey string
}

type BlacklistResult struct {
	EntryNo    string
	CustomerNo string
	Status     string
	UpdatedAt  time.Time
}

func (s *mutationServiceImpl) AddNote(ctx context.Context, input NoteInput) (*NoteResult, error) {
	actor, err := s.authorizeActivity(ctx)
	if err != nil {
		return nil, err
	}
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.Category = strings.TrimSpace(input.Category)
	input.Content = strings.TrimSpace(input.Content)
	input.SupersedesNo = strings.TrimSpace(input.SupersedesNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validPersistedCustomerNo(input.CustomerNo) || !validOneOf(input.Category,
		"general", "billing", "service", "sales", "risk", "compliance") ||
		input.Content == "" || !utf8.ValidString(input.Content) || utf8.RuneCountInString(input.Content) > 8192 ||
		!validActivityIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	noteNo, err := newActivityNo("NTE_")
	if err != nil {
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	result := &NoteResult{NoteNo: noteNo, CustomerNo: input.CustomerNo, Category: input.Category,
		Status: "active", SupersedesNo: input.SupersedesNo}
	err = dao.CustomerNote.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if err := lockActivityCustomer(txCtx, input.CustomerNo); err != nil {
			return err
		}
		reservation, reserveErr := reserveActivity(txCtx, "customer.note.add", actor, input.CustomerNo,
			input.IdempotencyKey, struct {
				Category, Content, SupersedesNo string
			}{input.Category, input.Content, input.SupersedesNo}, "customer_note", noteNo)
		if reserveErr != nil {
			return reserveErr
		}
		result.NoteNo = reservation.resourceNo
		if reservation.replayed {
			loaded, loadErr := loadNoteResult(txCtx, input.CustomerNo, reservation.resourceNo)
			if loadErr == nil {
				result = loaded
			}
			return loadErr
		}
		noteNo = reservation.resourceNo
		if input.SupersedesNo != "" {
			value, lockErr := g.DB().GetValue(txCtx, `SELECT id FROM plugin_idc_core_customer_note WHERE tenant_id=0 AND customer_no=? AND note_no=? AND status_code='active' FOR UPDATE`, input.CustomerNo, input.SupersedesNo)
			if lockErr != nil || value.Int64() <= 0 {
				return bizerr.NewCode(customer.CodeNotAccessible)
			}
		}
		value, timeErr := g.DB().GetValue(txCtx, `SELECT CURRENT_TIMESTAMP`)
		if timeErr != nil {
			return gerror.Wrap(timeErr, "read note database time")
		}
		result.CreatedAt = value.Time()
		if _, insertErr := g.DB().Exec(txCtx, `INSERT INTO plugin_idc_core_customer_note (tenant_id,note_no,customer_no,category_code,content,status_code,supersedes_note_no,author_user_id,created_at,updated_at) VALUES (0,?,?,?,?, 'active',?,?,?,?)`, noteNo, input.CustomerNo, input.Category, input.Content, input.SupersedesNo, actor, result.CreatedAt, result.CreatedAt); insertErr != nil {
			return gerror.Wrap(insertErr, "insert customer note")
		}
		if input.SupersedesNo != "" {
			if _, updateErr := g.DB().Exec(txCtx, `UPDATE plugin_idc_core_customer_note SET status_code='withdrawn',withdrawn_by=?,withdrawn_at=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND note_no=? AND status_code='active'`, actor, result.CreatedAt, result.CreatedAt, input.CustomerNo, input.SupersedesNo); updateErr != nil {
				return gerror.Wrap(updateErr, "withdraw superseded customer note")
			}
		}
		if auditErr := s.auditWriter.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "customer.note.add", IdempotencyKey: input.IdempotencyKey,
			OperationCode: "customer.note.add", TargetType: "customer_note", TargetNo: noteNo,
			CustomerNo: input.CustomerNo, ResultCode: "succeeded", ReasonCode: "customer_note_recorded",
			RiskCode: "low", RequiredPermission: permissionCustomerUpdate,
			CorrelationNo:  noteNo,
			BeforeSnapshot: map[string]any{"supersedes_note_no": input.SupersedesNo},
			AfterSnapshot:  map[string]any{"category": input.Category, "status": "active"},
		}); auditErr != nil {
			return auditErr
		}
		return completeActivity(txCtx, reservation)
	})
	if err != nil {
		return nil, wrapActivityError(err)
	}
	return result, nil
}

func (s *mutationServiceImpl) PlanFollowUp(ctx context.Context, input FollowUpPlanInput) (*FollowUpResult, error) {
	actor, err := s.authorizeActivity(ctx)
	if err != nil {
		return nil, err
	}
	input.CustomerNo, input.Channel = strings.TrimSpace(input.CustomerNo), strings.TrimSpace(input.Channel)
	input.Subject, input.Detail = strings.TrimSpace(input.Subject), strings.TrimSpace(input.Detail)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validPersistedCustomerNo(input.CustomerNo) || !validOneOf(input.Channel,
		"phone", "email", "sms", "wechat", "meeting", "ticket", "other") ||
		input.Subject == "" || utf8.RuneCountInString(input.Subject) > 512 ||
		!utf8.ValidString(input.Detail) || utf8.RuneCountInString(input.Detail) > 8192 || input.PlannedAt.IsZero() || input.AssignedUserID < 0 ||
		!validActivityIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	no, err := newActivityNo("FUP_")
	if err != nil {
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	result := &FollowUpResult{FollowUpNo: no, CustomerNo: input.CustomerNo, Status: "planned", PlannedAt: input.PlannedAt}
	err = dao.CustomerFollowUp.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if err := lockActivityCustomer(txCtx, input.CustomerNo); err != nil {
			return err
		}
		reservation, reserveErr := reserveActivity(txCtx, "customer.follow_up.plan", actor, input.CustomerNo,
			input.IdempotencyKey, struct {
				Channel, Subject, Detail string
				AssignedUserID           int
				PlannedAt                time.Time
			}{input.Channel, input.Subject, input.Detail, input.AssignedUserID, input.PlannedAt}, "customer_follow_up", no)
		if reserveErr != nil {
			return reserveErr
		}
		result.FollowUpNo = reservation.resourceNo
		if reservation.replayed {
			loaded, loadErr := loadFollowUpResult(txCtx, input.CustomerNo, reservation.resourceNo)
			if loadErr == nil {
				result = loaded
			}
			return loadErr
		}
		no = reservation.resourceNo
		value, timeErr := g.DB().GetValue(txCtx, `SELECT CURRENT_TIMESTAMP`)
		if timeErr != nil {
			return timeErr
		}
		result.UpdatedAt = value.Time()
		if _, insertErr := g.DB().Exec(txCtx, `INSERT INTO plugin_idc_core_customer_follow_up (tenant_id,follow_up_no,customer_no,kind_code,channel_code,status_code,subject,detail,assigned_user_id,planned_at,created_by,updated_by,created_at,updated_at) VALUES (0,?,?, 'plan',?,'planned',?,?,?,?,?,?,?,?)`, no, input.CustomerNo, input.Channel, input.Subject, input.Detail, input.AssignedUserID, input.PlannedAt, actor, actor, result.UpdatedAt, result.UpdatedAt); insertErr != nil {
			return gerror.Wrap(insertErr, "insert customer follow-up")
		}
		if auditErr := s.auditWriter.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "customer.follow_up.plan", IdempotencyKey: input.IdempotencyKey,
			OperationCode: "customer.follow_up.plan", TargetType: "customer_follow_up", TargetNo: no,
			CustomerNo: input.CustomerNo, ResultCode: "succeeded", ReasonCode: "customer_follow_up_planned",
			RiskCode: "low", RequiredPermission: permissionCustomerUpdate,
			CorrelationNo: no, BeforeSnapshot: map[string]any{"status": "absent"},
			AfterSnapshot: map[string]any{"channel": input.Channel, "status": "planned", "assigned_user_id": input.AssignedUserID},
		}); auditErr != nil {
			return auditErr
		}
		return completeActivity(txCtx, reservation)
	})
	if err != nil {
		return nil, wrapActivityError(err)
	}
	return result, nil
}

func (s *mutationServiceImpl) TransitionFollowUp(ctx context.Context, input FollowUpTransitionInput) (*FollowUpResult, error) {
	actor, err := s.authorizeActivity(ctx)
	if err != nil {
		return nil, err
	}
	input.CustomerNo, input.FollowUpNo = strings.TrimSpace(input.CustomerNo), strings.TrimSpace(input.FollowUpNo)
	input.Action, input.Outcome = strings.TrimSpace(input.Action), strings.TrimSpace(input.Outcome)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validPersistedCustomerNo(input.CustomerNo) || !validOneOf(input.Action, "complete", "cancel") ||
		utf8.RuneCountInString(input.Outcome) > 32 || !validActivityIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	status, timeColumn := "completed", "completed_at"
	if input.Action == "cancel" {
		status, timeColumn = "cancelled", "cancelled_at"
	}
	result := &FollowUpResult{FollowUpNo: input.FollowUpNo, CustomerNo: input.CustomerNo, Status: status}
	err = dao.CustomerFollowUp.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		reservation, reserveErr := reserveActivity(txCtx, "customer.follow_up."+input.Action, actor, input.CustomerNo,
			input.IdempotencyKey, struct{ FollowUpNo, Action, Outcome string }{input.FollowUpNo, input.Action, input.Outcome},
			"customer_follow_up", input.FollowUpNo)
		if reserveErr != nil {
			return reserveErr
		}
		if reservation.replayed {
			loaded, loadErr := loadFollowUpResult(txCtx, input.CustomerNo, reservation.resourceNo)
			if loadErr == nil {
				result = loaded
			}
			return loadErr
		}
		var row struct {
			PlannedAt time.Time `orm:"planned_at"`
		}
		if scanErr := g.DB().GetScan(txCtx, &row, `SELECT planned_at FROM plugin_idc_core_customer_follow_up WHERE tenant_id=0 AND customer_no=? AND follow_up_no=? AND status_code='planned' FOR UPDATE`, input.CustomerNo, input.FollowUpNo); scanErr != nil || row.PlannedAt.IsZero() {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		result.PlannedAt = row.PlannedAt
		value, timeErr := g.DB().GetValue(txCtx, `SELECT CURRENT_TIMESTAMP`)
		if timeErr != nil {
			return timeErr
		}
		result.UpdatedAt = value.Time()
		query := `UPDATE plugin_idc_core_customer_follow_up SET status_code=?,outcome_code=?,` + timeColumn + `=?,updated_by=?,updated_at=? WHERE tenant_id=0 AND customer_no=? AND follow_up_no=? AND status_code='planned'`
		update, updateErr := g.DB().Exec(txCtx, query, status, input.Outcome, result.UpdatedAt, actor, result.UpdatedAt, input.CustomerNo, input.FollowUpNo)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "transition customer follow-up")
		}
		if affected, _ := update.RowsAffected(); affected != 1 {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		if auditErr := s.auditWriter.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "customer.follow_up." + input.Action, IdempotencyKey: input.IdempotencyKey,
			OperationCode: "customer.follow_up." + input.Action, TargetType: "customer_follow_up", TargetNo: input.FollowUpNo,
			CustomerNo: input.CustomerNo, ResultCode: "succeeded", ReasonCode: "customer_follow_up_" + status,
			RiskCode: "low", RequiredPermission: permissionCustomerUpdate,
			CorrelationNo:  input.FollowUpNo,
			BeforeSnapshot: map[string]any{"status": "planned"}, AfterSnapshot: map[string]any{"status": status, "outcome": input.Outcome},
		}); auditErr != nil {
			return auditErr
		}
		return completeActivity(txCtx, reservation)
	})
	if err != nil {
		return nil, wrapActivityError(err)
	}
	return result, nil
}

func (s *mutationServiceImpl) CreateBlacklist(ctx context.Context, input BlacklistCreateInput) (*BlacklistResult, error) {
	actor, err := s.authorizeActivity(ctx)
	if err != nil {
		return nil, err
	}
	input.CustomerNo, input.Scope = strings.TrimSpace(input.CustomerNo), strings.TrimSpace(input.Scope)
	input.ReasonCode, input.ReasonDetail = strings.TrimSpace(input.ReasonCode), strings.TrimSpace(input.ReasonDetail)
	input.EvidenceReference, input.RiskLevel = strings.TrimSpace(input.EvidenceReference), strings.TrimSpace(input.RiskLevel)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	startsAtDefault := input.StartsAt.IsZero()
	if !validPersistedCustomerNo(input.CustomerNo) || !validOneOf(input.Scope,
		"registration", "purchase", "refund", "resource_operation", "all_business") ||
		!validOneOf(input.RiskLevel, "low", "medium", "high", "critical") ||
		!validReasonCode(input.ReasonCode) || utf8.RuneCountInString(input.ReasonDetail) > 2048 ||
		utf8.RuneCountInString(input.EvidenceReference) > 256 || (!startsAtDefault && input.ExpiresAt != nil && !input.ExpiresAt.After(input.StartsAt)) ||
		!validActivityIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	no, err := newActivityNo("BLK_")
	if err != nil {
		return nil, bizerr.WrapCode(err, customer.CodeUnavailable)
	}
	result := &BlacklistResult{EntryNo: no, CustomerNo: input.CustomerNo, Status: "pending"}
	err = dao.CustomerBlacklistEntry.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if err := lockActivityCustomer(txCtx, input.CustomerNo); err != nil {
			return err
		}
		reservation, reserveErr := reserveActivity(txCtx, "customer.blacklist.create", actor, input.CustomerNo,
			input.IdempotencyKey, struct {
				Scope, ReasonCode, ReasonDetail, EvidenceReference, RiskLevel string
				StartsAt                                                      time.Time
				ExpiresAt                                                     *time.Time
			}{input.Scope, input.ReasonCode, input.ReasonDetail, input.EvidenceReference, input.RiskLevel, input.StartsAt, input.ExpiresAt},
			"customer_blacklist", no)
		if reserveErr != nil {
			return reserveErr
		}
		result.EntryNo = reservation.resourceNo
		if reservation.replayed {
			loaded, loadErr := loadBlacklistResult(txCtx, input.CustomerNo, reservation.resourceNo)
			if loadErr == nil {
				result = loaded
			}
			return loadErr
		}
		no = reservation.resourceNo
		value, timeErr := g.DB().GetValue(txCtx, `SELECT CURRENT_TIMESTAMP`)
		if timeErr != nil {
			return timeErr
		}
		result.UpdatedAt = value.Time()
		if startsAtDefault {
			input.StartsAt = result.UpdatedAt
			if input.ExpiresAt != nil && !input.ExpiresAt.After(input.StartsAt) {
				return bizerr.NewCode(customer.CodeProfileInvalid)
			}
		}
		if _, insertErr := g.DB().Exec(txCtx, `INSERT INTO plugin_idc_core_customer_blacklist_entry (tenant_id,entry_no,customer_no,scope_code,reason_code,reason_detail,evidence_reference,risk_level_code,status_code,starts_at,expires_at,created_by,created_at,updated_at) VALUES (0,?,?,?,?,?,?,?,'pending',?,?,?,?,?)`, no, input.CustomerNo, input.Scope, input.ReasonCode, input.ReasonDetail, input.EvidenceReference, input.RiskLevel, input.StartsAt, input.ExpiresAt, actor, result.UpdatedAt, result.UpdatedAt); insertErr != nil {
			return gerror.Wrap(insertErr, "insert customer blacklist proposal")
		}
		if auditErr := s.auditWriter.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "customer.blacklist.create", IdempotencyKey: input.IdempotencyKey,
			OperationCode: "customer.blacklist.create", TargetType: "customer_blacklist", TargetNo: no,
			CustomerNo: input.CustomerNo, ResultCode: "succeeded", ReasonCode: input.ReasonCode,
			RiskCode: input.RiskLevel, RequiredPermission: permissionCustomerUpdate,
			CorrelationNo: no, BeforeSnapshot: map[string]any{"status": "absent"},
			AfterSnapshot: map[string]any{"scope": input.Scope, "status": "pending", "expires_at": input.ExpiresAt},
		}); auditErr != nil {
			return auditErr
		}
		return completeActivity(txCtx, reservation)
	})
	if err != nil {
		return nil, wrapActivityError(err)
	}
	return result, nil
}

func validReasonCode(value string) bool {
	if value == "" || len(value) > 64 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, current := range value[1:] {
		if unicode.IsLower(current) || unicode.IsDigit(current) || current == '_' || current == '-' || current == '.' {
			continue
		}
		return false
	}
	return true
}

func (s *mutationServiceImpl) DecideBlacklist(ctx context.Context, input BlacklistDecisionInput) (*BlacklistResult, error) {
	actor, err := s.authorizeActivity(ctx)
	if err != nil {
		return nil, err
	}
	input.CustomerNo, input.EntryNo, input.Action = strings.TrimSpace(input.CustomerNo), strings.TrimSpace(input.EntryNo), strings.TrimSpace(input.Action)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validPersistedCustomerNo(input.CustomerNo) || !validOneOf(input.Action, "approve", "reject", "revoke") ||
		!validActivityIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	from, to := "pending", "active"
	if input.Action == "reject" {
		to = "rejected"
	} else if input.Action == "revoke" {
		from, to = "active", "revoked"
	}
	result := &BlacklistResult{EntryNo: input.EntryNo, CustomerNo: input.CustomerNo, Status: to}
	err = dao.CustomerBlacklistEntry.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		reservation, reserveErr := reserveActivity(txCtx, "customer.blacklist."+input.Action, actor, input.CustomerNo,
			input.IdempotencyKey, struct{ EntryNo, Action string }{input.EntryNo, input.Action},
			"customer_blacklist", input.EntryNo)
		if reserveErr != nil {
			return reserveErr
		}
		if reservation.replayed {
			loaded, loadErr := loadBlacklistResult(txCtx, input.CustomerNo, reservation.resourceNo)
			if loadErr == nil {
				result = loaded
			}
			return loadErr
		}
		value, timeErr := g.DB().GetValue(txCtx, `SELECT CURRENT_TIMESTAMP`)
		if timeErr != nil {
			return timeErr
		}
		result.UpdatedAt = value.Time()
		setClause := "status_code=?,updated_at=?"
		arguments := []any{to, result.UpdatedAt}
		if input.Action == "approve" {
			setClause += ",approved_by=?,approved_at=?"
			arguments = append(arguments, actor, result.UpdatedAt)
		} else if input.Action == "revoke" {
			setClause += ",revoked_by=?,revoked_at=?"
			arguments = append(arguments, actor, result.UpdatedAt)
		}
		arguments = append(arguments, input.CustomerNo, input.EntryNo, from)
		update, updateErr := g.DB().Exec(txCtx, `UPDATE plugin_idc_core_customer_blacklist_entry SET `+setClause+` WHERE tenant_id=0 AND customer_no=? AND entry_no=? AND status_code=?`, arguments...)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "decide customer blacklist entry")
		}
		if affected, _ := update.RowsAffected(); affected != 1 {
			return bizerr.NewCode(customer.CodeNotAccessible)
		}
		if auditErr := s.auditWriter.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "customer.blacklist." + input.Action, IdempotencyKey: input.IdempotencyKey,
			OperationCode: "customer.blacklist." + input.Action, TargetType: "customer_blacklist", TargetNo: input.EntryNo,
			CustomerNo: input.CustomerNo, ResultCode: "succeeded", ReasonCode: "customer_blacklist_" + to,
			RiskCode: "high", RequiredPermission: permissionCustomerUpdate,
			CorrelationNo:  input.EntryNo,
			BeforeSnapshot: map[string]any{"status": from}, AfterSnapshot: map[string]any{"status": to},
		}); auditErr != nil {
			return auditErr
		}
		return completeActivity(txCtx, reservation)
	})
	if err != nil {
		return nil, wrapActivityError(err)
	}
	return result, nil
}

func (s *mutationServiceImpl) authorizeActivity(ctx context.Context) (int, error) {
	if err := authorizeStaff(ctx, s.bizCtxSvc, s.principal, s.authorization, permissionCustomerUpdate); err != nil {
		return 0, err
	}
	return s.bizCtxSvc.Current(ctx).UserID, nil
}

func lockActivityCustomer(ctx context.Context, customerNo string) error {
	value, err := g.DB().GetValue(ctx, `SELECT id FROM plugin_idc_core_customer_subject WHERE tenant_id=0 AND customer_no=? AND status_code<>'anonymized' FOR UPDATE`, customerNo)
	if err != nil {
		return gerror.Wrap(err, "lock customer activity subject")
	}
	if value.Int64() <= 0 {
		return bizerr.NewCode(customer.CodeNotAccessible)
	}
	return nil
}

func newActivityNo(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(raw[:]), nil
}

func validOneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func validActivityIdempotencyKey(value string) bool {
	return len(value) >= 12 && len(value) <= 128 && utf8.ValidString(value)
}

func loadNoteResult(ctx context.Context, customerNo, noteNo string) (*NoteResult, error) {
	var row struct {
		NoteNo       string    `orm:"note_no"`
		CustomerNo   string    `orm:"customer_no"`
		Category     string    `orm:"category_code"`
		Status       string    `orm:"status_code"`
		SupersedesNo string    `orm:"supersedes_note_no"`
		CreatedAt    time.Time `orm:"created_at"`
	}
	if err := g.DB().GetScan(ctx, &row, `SELECT note_no,customer_no,category_code,status_code,supersedes_note_no,created_at FROM plugin_idc_core_customer_note WHERE tenant_id=0 AND customer_no=? AND note_no=?`, customerNo, noteNo); err != nil {
		return nil, gerror.Wrap(err, "load replayed customer note")
	}
	if row.NoteNo == "" || row.CustomerNo != customerNo {
		return nil, gerror.New("replayed customer note is missing")
	}
	return &NoteResult{NoteNo: row.NoteNo, CustomerNo: row.CustomerNo, Category: row.Category,
		Status: row.Status, SupersedesNo: row.SupersedesNo, CreatedAt: row.CreatedAt}, nil
}

func loadFollowUpResult(ctx context.Context, customerNo, followUpNo string) (*FollowUpResult, error) {
	var row struct {
		FollowUpNo string    `orm:"follow_up_no"`
		CustomerNo string    `orm:"customer_no"`
		Status     string    `orm:"status_code"`
		PlannedAt  time.Time `orm:"planned_at"`
		UpdatedAt  time.Time `orm:"updated_at"`
	}
	if err := g.DB().GetScan(ctx, &row, `SELECT follow_up_no,customer_no,status_code,planned_at,updated_at FROM plugin_idc_core_customer_follow_up WHERE tenant_id=0 AND customer_no=? AND follow_up_no=?`, customerNo, followUpNo); err != nil {
		return nil, gerror.Wrap(err, "load replayed customer follow-up")
	}
	if row.FollowUpNo == "" || row.CustomerNo != customerNo {
		return nil, gerror.New("replayed customer follow-up is missing")
	}
	return &FollowUpResult{FollowUpNo: row.FollowUpNo, CustomerNo: row.CustomerNo,
		Status: row.Status, PlannedAt: row.PlannedAt, UpdatedAt: row.UpdatedAt}, nil
}

func loadBlacklistResult(ctx context.Context, customerNo, entryNo string) (*BlacklistResult, error) {
	var row struct {
		EntryNo    string    `orm:"entry_no"`
		CustomerNo string    `orm:"customer_no"`
		Status     string    `orm:"status_code"`
		UpdatedAt  time.Time `orm:"updated_at"`
	}
	if err := g.DB().GetScan(ctx, &row, `SELECT entry_no,customer_no,status_code,updated_at FROM plugin_idc_core_customer_blacklist_entry WHERE tenant_id=0 AND customer_no=? AND entry_no=?`, customerNo, entryNo); err != nil {
		return nil, gerror.Wrap(err, "load replayed customer blacklist")
	}
	if row.EntryNo == "" || row.CustomerNo != customerNo {
		return nil, gerror.New("replayed customer blacklist is missing")
	}
	return &BlacklistResult{EntryNo: row.EntryNo, CustomerNo: row.CustomerNo,
		Status: row.Status, UpdatedAt: row.UpdatedAt}, nil
}

func wrapActivityError(err error) error {
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, customer.CodeUnavailable)
}
