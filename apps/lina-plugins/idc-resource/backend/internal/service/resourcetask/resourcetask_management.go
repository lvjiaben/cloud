// This file owns bounded Provider task reads and evidence-backed manual takeover.

package resourcetask

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

const (
	permissionTaskList           = "idc:admin:resource:task:list"
	permissionTaskQuery          = "idc:admin:resource:task:query"
	permissionTaskManualTakeover = "idc:admin:resource:task:manual-takeover"
	defaultTaskPageSize          = 25
	maximumTaskPageSize          = 100
	maximumDetailEvidenceCount   = 50
	maximumTakeoverEvidenceCount = 16
	takeoverAuditOperation       = "resource.provider-task.manual-takeover"
)

var evidenceNumberPattern = regexp.MustCompile(`^evidence_[0-9a-f]{32}$`)

// Manual takeover reason codes. Free text is intentionally unsupported.
const (
	TakeoverReasonProviderResultUnresolved     = "provider_result_unresolved"
	TakeoverReasonProviderTaskStalled          = "provider_task_stalled"
	TakeoverReasonResourceStateConflict        = "resource_state_conflict"
	TakeoverReasonExternalCoordinationRequired = "external_coordination_required"
	TakeoverReasonCompensationRequired         = "compensation_required"
)

// TaskListInput is one bounded task keyset query.
type TaskListInput struct {
	ServiceNo string
	Page      int
	ChannelNo string
	Status    Status
	Cursor    string
	Limit     int
}

// TaskScope fixes optional product and interface context without granting access.
type TaskScope struct{ ServiceNo, ChannelNo string }

// TaskSummary is the non-secret management projection of one Provider task.
type TaskSummary struct {
	TaskNo                  string
	CapabilityCode          string
	LocalResourceNo         string
	ChannelNo               string
	ChannelName             string
	Status                  Status
	AttemptCount            int
	MaxAttempts             int
	ObservationAttemptCount int
	ResultUnknown           bool
	Version                 int64
	NextObserveAt           *time.Time
	CreatedAt               *time.Time
	UpdatedAt               *time.Time
	Takeover                *ManualTakeover
}

// TaskPage is one bounded keyset page.
type TaskPage struct {
	Items      []TaskSummary
	Total      int
	NextCursor string
}

// TaskEvidence is one immutable, credential-scanned owner evidence projection.
type TaskEvidence struct {
	EvidenceNo   string
	SourceKind   string
	EvidenceKind string
	Reference    string
	Summary      string
	ObservedAt   *time.Time
}

// UpstreamTaskProjection is the latest normalized upstream task binding.
type UpstreamTaskProjection struct {
	ExternalTaskRef     string
	ExternalResourceRef string
	StatusCode          string
	Progress            int
	ObservedAt          *time.Time
}

// ManualTakeover is the immutable responsibility assignment projection.
type ManualTakeover struct {
	TakeoverNo     string
	FromStatus     Status
	TaskVersion    int64
	ReasonCode     string
	EvidenceCount  int
	AssignedUserID int64
	CreatedAt      *time.Time
}

// TaskDetail contains only local, bounded and non-secret reconciliation facts.
type TaskDetail struct {
	Task              TaskSummary
	Upstream          *UpstreamTaskProjection
	Evidence          []TaskEvidence
	ResidualResources []resourcecap.ResidualResource
}

// ManualTakeoverInput transfers automation responsibility without deciding an upstream result.
type ManualTakeoverInput struct {
	Scope              TaskScope
	TaskNo             string
	ExpectedVersion    int64
	ReasonCode         string
	EvidenceNos        []string
	TransportViolation bool
}

// ManagementService owns Provider task management reads and safe takeover.
type ManagementService interface {
	// ListAPILogs returns a bounded body-free invocation page within staff scope.
	ListAPILogs(ctx context.Context, input APILogListInput) (*APILogPage, error)
	List(ctx context.Context, input TaskListInput) (*TaskPage, error)
	Detail(ctx context.Context, taskNo string, scopes ...TaskScope) (*TaskDetail, error)
	TakeOver(ctx context.Context, input ManualTakeoverInput) (*TaskDetail, error)
}

type managementService struct {
	store   *Store
	runtime managementcap.Runtime
}

type modeler interface {
	Model(tableNameQueryOrStruct ...any) *gdb.Model
}

var _ ManagementService = (*managementService)(nil)

// NewManagement constructs task administration from explicit owner dependencies.
func NewManagement(store *Store, runtime managementcap.Runtime) (ManagementService, error) {
	if store == nil || store.database == nil || store.random == nil || managementcap.ValidateRuntime(runtime) != nil {
		return nil, gerror.New("resource Provider task management dependencies are invalid")
	}
	return &managementService{store: store, runtime: runtime}, nil
}

// List returns one authorized local task page without Provider calls.
func (s *managementService) List(ctx context.Context, input TaskListInput) (*TaskPage, error) {
	if _, err := s.authorize(ctx, permissionTaskList, "provider-task-list"); err != nil {
		return nil, err
	}
	input.ServiceNo = strings.TrimSpace(input.ServiceNo)
	input.ChannelNo = strings.TrimSpace(input.ChannelNo)
	input.Cursor = strings.TrimSpace(input.Cursor)
	if input.Limit == 0 {
		input.Limit = defaultTaskPageSize
	}
	if input.Limit < 1 || input.Limit > maximumTaskPageSize ||
		input.Page < 0 || input.Page > 1_000_000 || (input.Page > 0 && input.Cursor != "") ||
		(input.ServiceNo != "" && (len(input.ServiceNo) > 40 || !workerIDPattern.MatchString(input.ServiceNo))) ||
		(input.ChannelNo != "" && resourcecap.ValidateChannelRef(input.ChannelNo) != nil) ||
		(input.Cursor != "" && !workerIDPattern.MatchString(input.Cursor)) ||
		(input.Status != "" && !validStatus(input.Status)) {
		return nil, bizerr.NewCode(CodeTaskManagementInvalid)
	}
	taskTable := dao.Task.Table()
	taskColumns := dao.Task.Columns()
	model := s.store.database.Model(taskTable).Safe().Ctx(ctx).As("task").
		LeftJoin(dao.Channel.Table()+" channel", "channel.channel_no=task.channel_no AND channel.tenant_id=task.tenant_id").
		LeftJoin(dao.TaskManualTakeover.Table()+" takeover", "takeover.task_no=task.task_no AND takeover.tenant_id=task.tenant_id").
		Where("task.tenant_id", idccap.DefaultTenantID)
	if input.ChannelNo != "" {
		model = model.Where("task."+taskColumns.ChannelNo, input.ChannelNo)
	}
	if input.ServiceNo != "" {
		// The ownership projection binds provisioning and later operations to
		// one entitlement. Restrict the count and page with the same predicate.
		model = model.Where(`(task.local_resource_no = ? OR EXISTS (SELECT 1 FROM plugin_idc_resource_server_entitlement_projection ownership
WHERE ownership.tenant_id = task.tenant_id AND ownership.entitlement_no = ?
AND (task.local_resource_no = ownership.local_resource_no
OR task.local_resource_no = ownership.entitlement_no OR task.task_no = ownership.provider_task_no)))`, input.ServiceNo, input.ServiceNo)
	}

	if input.Status != "" {
		model = model.Where("task."+taskColumns.StatusCode, input.Status)
	}
	total, err := model.Clone().Count()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeTaskManagementUnavailable)
	}
	if input.Page > 0 {
		model = model.Offset((input.Page - 1) * input.Limit)
	}
	if input.Cursor != "" {
		model = model.WhereGT("task."+taskColumns.TaskNo, input.Cursor)
	}
	rows := make([]taskManagementRow, 0, input.Limit+1)
	if err := model.Fields(
		"task.task_no", "task.capability_code", "task.local_resource_no", "task.channel_no",
		"channel.display_name AS channel_name", "task.status_code", "task.attempt_count",
		"task.max_attempts", "task.observation_attempt_count", "task.result_unknown",
		"task.version", "task.next_observe_at", "task.created_at", "task.updated_at",
		"takeover.takeover_no", "takeover.from_status_code", "takeover.task_version",
		"takeover.reason_code", "takeover.evidence_count", "takeover.assigned_user_id",
		"takeover.created_at AS takeover_created_at",
	).OrderAsc("task." + taskColumns.TaskNo).Limit(input.Limit + 1).Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(err, CodeTaskManagementUnavailable)
	}
	hasMore := len(rows) > input.Limit
	if hasMore {
		rows = rows[:input.Limit]
	}
	page := &TaskPage{Items: make([]TaskSummary, len(rows)), Total: total}
	for index := range rows {
		page.Items[index] = rows[index].summary()
	}
	if hasMore && len(page.Items) > 0 {
		page.NextCursor = page.Items[len(page.Items)-1].TaskNo
	}
	return page, nil
}

// Detail returns fixed, bounded local projections for one exact task.
func (s *managementService) Detail(ctx context.Context, taskNo string, scopes ...TaskScope) (*TaskDetail, error) {
	taskNo = strings.TrimSpace(taskNo)
	if _, err := s.authorize(ctx, permissionTaskQuery, taskNo); err != nil {
		return nil, err
	}
	if !workerIDPattern.MatchString(taskNo) {
		return nil, bizerr.NewCode(CodeTaskManagementInvalid)
	}
	if len(scopes) > 1 {
		return nil, bizerr.NewCode(CodeTaskManagementInvalid)
	}
	if len(scopes) == 1 {
		if err := s.ensureTaskScope(ctx, taskNo, scopes[0]); err != nil {
			return nil, err
		}
	}
	result, err := s.loadDetail(ctx, s.store.database, taskNo)
	if err != nil {
		if bizerr.Is(err, CodeTaskManagementNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeTaskManagementUnavailable)
	}
	return result, nil
}

// TakeOver atomically closes automation and records one evidence-backed assignment.
func (s *managementService) TakeOver(ctx context.Context, input ManualTakeoverInput) (*TaskDetail, error) {
	input.TaskNo = strings.TrimSpace(input.TaskNo)
	actor, err := s.authorize(ctx, permissionTaskManualTakeover, input.TaskNo)
	if err != nil {
		return nil, err
	}
	evidenceNos, err := validateTakeoverInput(input)
	if err != nil {
		return nil, bizerr.NewCode(CodeTaskManagementInvalid)
	}
	if err := s.ensureTaskScope(ctx, input.TaskNo, input.Scope); err != nil {
		return nil, err
	}
	requestDigest, err := canonicalDigest(struct {
		Actor           int      `json:"actor"`
		TaskNo          string   `json:"taskNo"`
		ExpectedVersion int64    `json:"expectedVersion"`
		ReasonCode      string   `json:"reasonCode"`
		EvidenceNos     []string `json:"evidenceNos"`
	}{actor, input.TaskNo, input.ExpectedVersion, input.ReasonCode, evidenceNos})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeTaskManagementUnavailable)
	}

	err = s.store.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, loadErr := lockManagedTask(ctx, tx, input.TaskNo)
		if loadErr != nil {
			return loadErr
		}
		if row == nil {
			return bizerr.NewCode(CodeTaskManagementNotAccessible)
		}
		existing, loadErr := loadManualTakeover(ctx, tx, input.TaskNo)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.RequestDigest == requestDigest && existing.AssignedUserId == int64(actor) &&
				existing.TaskVersion == input.ExpectedVersion {
				return nil
			}
			return bizerr.NewCode(CodeTaskManagementConflict)
		}
		from := Status(row.StatusCode)
		if row.Version != input.ExpectedVersion ||
			ValidateTransition(from, StatusManualTakeover, TransitionEvidence{ManualAuthorized: true}) != nil {
			return bizerr.NewCode(CodeTaskManagementConflict)
		}
		evidence, loadErr := loadSelectedEvidence(ctx, tx, input.TaskNo, evidenceNos)
		if loadErr != nil {
			return loadErr
		}
		if len(evidence) != len(evidenceNos) {
			return bizerr.NewCode(CodeTaskManagementConflict)
		}
		now, timeErr := resourceDatabaseTime(tx)
		if timeErr != nil {
			return timeErr
		}
		takeoverNo, numberErr := newTakeoverNumber(s.store.random)
		if numberErr != nil {
			return numberErr
		}
		evidenceDigest, digestErr := digestEvidence(evidence)
		if digestErr != nil {
			return digestErr
		}
		if _, insertErr := tx.Model(dao.TaskManualTakeover.Table()).Safe().Ctx(ctx).Data(do.TaskManualTakeover{
			TenantId: idccap.DefaultTenantID, TakeoverNo: takeoverNo, TaskNo: input.TaskNo,
			FromStatusCode: from, TaskVersion: input.ExpectedVersion, ReasonCode: input.ReasonCode,
			RequestDigest: requestDigest, EvidenceDigest: evidenceDigest, EvidenceCount: len(evidence),
			AssignedUserId: actor, CreatedAt: &now,
		}).Insert(); insertErr != nil {
			return gerror.Wrap(insertErr, "insert resource task manual takeover")
		}
		if revokeErr := revokeTaskInputBindings(ctx, tx, []string{input.TaskNo}, now); revokeErr != nil {
			return revokeErr
		}
		columns := dao.Task.Columns()
		update, updateErr := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
			Where(do.Task{TenantId: idccap.DefaultTenantID, Id: row.Id, TaskNo: row.TaskNo,
				StatusCode: from, Version: input.ExpectedVersion}).
			Data(do.Task{
				StatusCode: StatusManualTakeover, ResultUnknown: false, LockedBy: "",
				FinishedAt: &now, Version: gdb.Raw(columns.Version + " + 1"),
			}).Update()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "close resource task for manual takeover")
		}
		if affected, affectedErr := update.RowsAffected(); affectedErr != nil || affected != 1 {
			return bizerr.NewCode(CodeTaskManagementConflict)
		}
		for _, column := range []string{
			columns.LeaseExpiresAt, columns.NextObserveAt, columns.ObservationClaimedAt,
		} {
			clearResult, clearErr := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
				Where(do.Task{TenantId: idccap.DefaultTenantID, Id: row.Id, TaskNo: row.TaskNo,
					StatusCode: StatusManualTakeover, Version: input.ExpectedVersion + 1}).
				Data(column, nil).Update()
			if clearErr != nil {
				return gerror.Wrap(clearErr, "clear resource task takeover observation lease")
			}
			if affected, affectedErr := clearResult.RowsAffected(); affectedErr != nil || affected != 1 {
				return bizerr.NewCode(CodeTaskManagementConflict)
			}
		}
		if auditErr := s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: takeoverAuditOperation, IdempotencyKey: requestDigest,
			OperationCode: takeoverAuditOperation, TargetType: "resource_task", TargetNo: input.TaskNo,
			ResultCode: "succeeded", ReasonCode: input.ReasonCode, RiskCode: "high",
			RequiredPermission: permissionTaskManualTakeover,
			BeforeSnapshot:     map[string]any{"status": from, "version": input.ExpectedVersion},
			AfterSnapshot: map[string]any{"status": StatusManualTakeover, "version": input.ExpectedVersion + 1,
				"takeoverNo": takeoverNo, "evidenceDigest": evidenceDigest, "evidenceCount": len(evidence)},
		}); auditErr != nil {
			return auditErr
		}
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeTaskManagementInvalid) || bizerr.Is(err, CodeTaskManagementConflict) ||
			bizerr.Is(err, CodeTaskManagementNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeTaskManagementUnavailable)
	}
	result, err := s.loadDetail(ctx, s.store.database, input.TaskNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeTaskManagementUnavailable)
	}
	return result, nil
}

func (s *managementService) authorize(ctx context.Context, permission, target string) (int, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return 0, bizerr.NewCode(CodeTaskManagementNotAccessible)
	}
	actor, err := s.runtime.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: permission, TargetNo: strings.TrimSpace(target),
	})
	if err != nil || actor <= 0 {
		return 0, bizerr.NewCode(CodeTaskManagementNotAccessible)
	}
	return actor, nil
}

type taskManagementRow struct {
	entity.Task
	ChannelName       string     `orm:"channel_name"`
	TakeoverNo        string     `orm:"takeover_no"`
	FromStatusCode    string     `orm:"from_status_code"`
	TaskVersion       int64      `orm:"task_version"`
	ReasonCode        string     `orm:"reason_code"`
	EvidenceCount     int        `orm:"evidence_count"`
	AssignedUserID    int64      `orm:"assigned_user_id"`
	TakeoverCreatedAt *time.Time `orm:"takeover_created_at"`
}

func (row taskManagementRow) summary() TaskSummary {
	result := TaskSummary{
		TaskNo: row.TaskNo, CapabilityCode: row.CapabilityCode, LocalResourceNo: row.LocalResourceNo,
		ChannelNo: row.ChannelNo, ChannelName: row.ChannelName, Status: Status(row.StatusCode),
		AttemptCount: row.AttemptCount, MaxAttempts: row.MaxAttempts,
		ObservationAttemptCount: row.ObservationAttemptCount, ResultUnknown: row.ResultUnknown,
		Version: row.Version, NextObserveAt: cloneTaskTime(row.NextObserveAt),
		CreatedAt: cloneTaskTime(row.CreatedAt), UpdatedAt: cloneTaskTime(row.UpdatedAt),
	}
	if row.TakeoverNo != "" {
		result.Takeover = &ManualTakeover{
			TakeoverNo: row.TakeoverNo, FromStatus: Status(row.FromStatusCode), TaskVersion: row.TaskVersion,
			ReasonCode: row.ReasonCode, EvidenceCount: row.EvidenceCount,
			AssignedUserID: row.AssignedUserID, CreatedAt: cloneTaskTime(row.TakeoverCreatedAt),
		}
	}
	return result
}

func (s *managementService) loadDetail(ctx context.Context, database gdb.DB, taskNo string) (*TaskDetail, error) {
	var row taskManagementRow
	err := database.Model(dao.Task.Table()).Safe().Ctx(ctx).As("task").
		LeftJoin(dao.Channel.Table()+" channel", "channel.channel_no=task.channel_no AND channel.tenant_id=task.tenant_id").
		LeftJoin(dao.TaskManualTakeover.Table()+" takeover", "takeover.task_no=task.task_no AND takeover.tenant_id=task.tenant_id").
		Fields(
			"task.task_no", "task.capability_code", "task.local_resource_no", "task.channel_no",
			"channel.display_name AS channel_name", "task.status_code", "task.attempt_count",
			"task.max_attempts", "task.observation_attempt_count", "task.result_unknown",
			"task.version", "task.next_observe_at", "task.created_at", "task.updated_at",
			"takeover.takeover_no", "takeover.from_status_code", "takeover.task_version",
			"takeover.reason_code", "takeover.evidence_count", "takeover.assigned_user_id",
			"takeover.created_at AS takeover_created_at",
		).
		Where("task.tenant_id", idccap.DefaultTenantID).Where("task.task_no", taskNo).Limit(1).Scan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, bizerr.NewCode(CodeTaskManagementNotAccessible)
	}
	if err != nil {
		return nil, gerror.Wrap(err, "load resource task management projection")
	}
	if row.TaskNo == "" {
		return nil, bizerr.NewCode(CodeTaskManagementNotAccessible)
	}
	result := &TaskDetail{Task: row.summary(), Evidence: make([]TaskEvidence, 0)}
	result.ResidualResources, err = LoadResidualResources(ctx, database, taskNo)
	if err != nil {
		return nil, err
	}
	var upstream entity.UpstreamTaskBinding
	if err = database.Model(dao.UpstreamTaskBinding.Table()).Safe().Ctx(ctx).
		Fields("external_task_ref", "external_resource_ref", "status_code", "last_progress", "last_observed_at").
		Where(do.UpstreamTaskBinding{TenantId: idccap.DefaultTenantID, TaskNo: taskNo}).
		OrderDesc(dao.UpstreamTaskBinding.Columns().StepNo).OrderDesc(dao.UpstreamTaskBinding.Columns().Id).
		Limit(1).Scan(&upstream); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, gerror.Wrap(err, "load resource task upstream projection")
		}
	}
	if upstream.ExternalTaskRef != "" {
		result.Upstream = &UpstreamTaskProjection{
			ExternalTaskRef: upstream.ExternalTaskRef, ExternalResourceRef: upstream.ExternalResourceRef,
			StatusCode: upstream.StatusCode, Progress: upstream.LastProgress,
			ObservedAt: cloneTaskTime(upstream.LastObservedAt),
		}
	}
	var evidence []entity.TaskEvidence
	if err = database.Model(dao.TaskEvidence.Table()).Safe().Ctx(ctx).
		Fields("evidence_no", "source_kind", "evidence_kind", "reference", "summary", "observed_at").
		Where(do.TaskEvidence{TenantId: idccap.DefaultTenantID, TaskNo: taskNo}).
		OrderDesc(dao.TaskEvidence.Columns().ObservedAt).OrderDesc(dao.TaskEvidence.Columns().Id).
		Limit(maximumDetailEvidenceCount).Scan(&evidence); err != nil {
		return nil, gerror.Wrap(err, "load resource task evidence projection")
	}
	result.Evidence = make([]TaskEvidence, len(evidence))
	for index := range evidence {
		result.Evidence[index] = TaskEvidence{
			EvidenceNo: evidence[index].EvidenceNo, SourceKind: evidence[index].SourceKind,
			EvidenceKind: evidence[index].EvidenceKind, Reference: evidence[index].Reference,
			Summary: evidence[index].Summary, ObservedAt: cloneTaskTime(evidence[index].ObservedAt),
		}
	}
	return result, nil
}

func validateTakeoverInput(input ManualTakeoverInput) ([]string, error) {
	if input.TransportViolation || !workerIDPattern.MatchString(input.TaskNo) || input.ExpectedVersion <= 0 ||
		!validTakeoverReason(input.ReasonCode) || len(input.EvidenceNos) < 1 ||
		len(input.EvidenceNos) > maximumTakeoverEvidenceCount {
		return nil, gerror.New("resource task manual takeover is invalid")
	}
	result := append([]string(nil), input.EvidenceNos...)
	sort.Strings(result)
	for index := range result {
		if !evidenceNumberPattern.MatchString(result[index]) ||
			(index > 0 && result[index] == result[index-1]) {
			return nil, gerror.New("resource task manual takeover evidence is invalid")
		}
	}
	return result, nil
}

func validTakeoverReason(reason string) bool {
	switch reason {
	case TakeoverReasonProviderResultUnresolved, TakeoverReasonProviderTaskStalled,
		TakeoverReasonResourceStateConflict, TakeoverReasonExternalCoordinationRequired,
		TakeoverReasonCompensationRequired:
		return true
	default:
		return false
	}
}

func lockManagedTask(ctx context.Context, tx gdb.TX, taskNo string) (*entity.Task, error) {
	var row entity.Task
	err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
		Fields("id", "task_no", "status_code", "version").
		Where(do.Task{TenantId: idccap.DefaultTenantID, TaskNo: taskNo}).LockUpdate().Scan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, gerror.Wrap(err, "lock resource task for manual takeover")
	}
	if row.TaskNo == "" {
		return nil, nil
	}
	return &row, nil
}

func loadManualTakeover(ctx context.Context, database modeler, taskNo string) (*entity.TaskManualTakeover, error) {
	var row entity.TaskManualTakeover
	err := database.Model(dao.TaskManualTakeover.Table()).Safe().Ctx(ctx).
		Where(do.TaskManualTakeover{TenantId: idccap.DefaultTenantID, TaskNo: taskNo}).Limit(1).Scan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, gerror.Wrap(err, "load resource task manual takeover")
	}
	if row.TakeoverNo == "" {
		return nil, nil
	}
	return &row, nil
}

func loadSelectedEvidence(ctx context.Context, database modeler, taskNo string, evidenceNos []string) ([]entity.TaskEvidence, error) {
	rows := make([]entity.TaskEvidence, 0, len(evidenceNos))
	err := database.Model(dao.TaskEvidence.Table()).Safe().Ctx(ctx).
		Fields("evidence_no", "source_kind", "evidence_kind", "reference", "summary", "observed_at").
		Where(do.TaskEvidence{TenantId: idccap.DefaultTenantID, TaskNo: taskNo}).
		WhereIn(dao.TaskEvidence.Columns().EvidenceNo, evidenceNos).
		OrderAsc(dao.TaskEvidence.Columns().EvidenceNo).Limit(maximumTakeoverEvidenceCount).Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "load resource task takeover evidence")
	}
	return rows, nil
}

func newTakeoverNumber(random io.Reader) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate resource task manual takeover number")
	}
	return "takeover_" + hex.EncodeToString(content[:]), nil
}

func digestEvidence(rows []entity.TaskEvidence) (string, error) {
	type item struct {
		EvidenceNo   string `json:"evidenceNo"`
		SourceKind   string `json:"sourceKind"`
		EvidenceKind string `json:"evidenceKind"`
		Reference    string `json:"reference"`
		Summary      string `json:"summary"`
		ObservedAt   string `json:"observedAt"`
	}
	items := make([]item, len(rows))
	for index := range rows {
		observedAt := ""
		if rows[index].ObservedAt != nil {
			observedAt = rows[index].ObservedAt.UTC().Format(time.RFC3339Nano)
		}
		items[index] = item{rows[index].EvidenceNo, rows[index].SourceKind, rows[index].EvidenceKind,
			rows[index].Reference, rows[index].Summary, observedAt}
	}
	return canonicalDigest(items)
}

func canonicalDigest(value any) (string, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return "", gerror.Wrap(err, "encode resource task canonical digest")
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

func cloneTaskTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

// ensureTaskScope fails closed when a task is outside its current product or interface.
func (s *managementService) ensureTaskScope(ctx context.Context, taskNo string, scope TaskScope) error {
	if scope.ServiceNo == "" && scope.ChannelNo == "" {
		return nil
	}
	if (scope.ServiceNo != "" && (len(scope.ServiceNo) > 40 || !workerIDPattern.MatchString(scope.ServiceNo))) ||
		(scope.ChannelNo != "" && resourcecap.ValidateChannelRef(scope.ChannelNo) != nil) {
		return bizerr.NewCode(CodeTaskManagementInvalid)
	}
	model := s.store.database.Model(dao.Task.Table()).Safe().Ctx(ctx).As("task").Where("task.tenant_id", idccap.DefaultTenantID).Where("task.task_no", taskNo)
	if scope.ChannelNo != "" {
		model = model.Where("task.channel_no", scope.ChannelNo)
	}
	if scope.ServiceNo != "" {
		model = model.Where(`(task.local_resource_no = ? OR EXISTS (SELECT 1 FROM plugin_idc_resource_server_entitlement_projection ownership
WHERE ownership.tenant_id = task.tenant_id AND ownership.entitlement_no = ?
AND (task.local_resource_no = ownership.local_resource_no OR task.local_resource_no = ownership.entitlement_no OR task.task_no = ownership.provider_task_no)))`, scope.ServiceNo, scope.ServiceNo)
	}
	count, err := model.Count()
	if err != nil {
		return bizerr.WrapCode(err, CodeTaskManagementUnavailable)
	}
	if count != 1 {
		return bizerr.NewCode(CodeTaskManagementNotAccessible)
	}
	return nil
}
