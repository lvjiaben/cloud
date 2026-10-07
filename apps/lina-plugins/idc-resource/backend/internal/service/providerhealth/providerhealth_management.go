// This file owns the bounded, audit-backed Provider health administration projection.

package providerhealth

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
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
	permissionHealthList        = "idc:admin:resource:health:list"
	permissionHealthPolicyWrite = "idc:admin:resource:health-policy:write"
	defaultHealthPageSize       = 25
	maximumHealthPageSize       = 100
	auditPolicyUpdateOperation  = "resource.provider-health-policy.update"
	auditPolicyUpdateResult     = "succeeded"
	auditPolicyUpdateReason     = "merchant_policy_update"
	auditPolicyUpdateRisk       = "medium"
	auditPolicyUpdateTargetType = "resource_channel"
)

// HealthListInput is one bounded keyset health query.
type HealthListInput struct {
	Page      int
	ChannelNo string
	Cursor    string
	Limit     int
}

// HealthPolicy is the merchant-managed automatic write-protection policy.
type HealthPolicy struct {
	WindowSampleLimit         int
	MinimumSamplesToProtect   int
	FailureLimit              int
	UnknownResultLimit        int
	ConsecutiveFailureLimit   int
	ProtectionDurationSeconds int
	Version                   int64
	UpdatedAt                 *time.Time
}

// HealthObservation is one non-sensitive latest operation projection.
type HealthObservation struct {
	HealthCode  HealthCode
	SummaryCode string
	LatencyMS   int
	ObservedAt  *time.Time
}

// ChannelHealth is one current local Provider-channel health projection.
type ChannelHealth struct {
	ChannelNo           string
	ChannelName         string
	ProviderID          string
	ChannelStatus       string
	HealthCode          HealthCode
	CircuitOpen         bool
	WriteProtectedUntil *time.Time
	SampleCount         int
	FailureCount        int
	UnknownCount        int
	ConsecutiveFailures int
	LastSummaryCode     string
	LastObservedAt      *time.Time
	Policy              HealthPolicy
	Authentication      HealthObservation
	Query               HealthObservation
	Write               HealthObservation
	ProviderCapacity    HealthObservation
}

// HealthPage is one bounded keyset page of Provider-channel health.
type HealthPage struct {
	Items      []ChannelHealth
	Total      int
	NextCursor string
}

// HealthPolicyUpdateInput carries one exact optimistic policy update.
type HealthPolicyUpdateInput struct {
	ChannelNo                 string
	WindowSampleLimit         int
	MinimumSamplesToProtect   int
	FailureLimit              int
	UnknownResultLimit        int
	ConsecutiveFailureLimit   int
	ProtectionDurationSeconds int
	ExpectedVersion           int64
}

// ManagementService owns staff health projection reads and audited policy updates.
type ManagementService interface {
	// List returns at most 100 locally projected channels without Provider calls.
	// Invalid cursors or bounds return a coded validation error; inaccessible
	// callers and storage failures return coded authorization or availability errors.
	List(ctx context.Context, input HealthListInput) (*HealthPage, error)
	// UpdatePolicy applies one safe exact-channel policy update. ExpectedVersion
	// is mandatory; active write protection is never manually closed or shortened.
	UpdatePolicy(ctx context.Context, input HealthPolicyUpdateInput) (*ChannelHealth, error)
}

type managementService struct {
	database gdb.DB
	runtime  managementcap.Runtime
}

var _ ManagementService = (*managementService)(nil)

// NewManagement constructs the staff health service from the startup-owned
// authorization and audit runtime; database access remains on the generated DAO boundary.
func NewManagement(database gdb.DB, runtime managementcap.Runtime) (ManagementService, error) {
	if database == nil || managementcap.ValidateRuntime(runtime) != nil {
		return nil, gerror.New("resource Provider health management dependencies are invalid")
	}
	return &managementService{database: database, runtime: runtime}, nil
}

// List returns one authorized, bounded, database-projected health page.
func (s *managementService) List(ctx context.Context, input HealthListInput) (*HealthPage, error) {
	if _, err := s.authorize(ctx, permissionHealthList, "provider-health-list"); err != nil {
		return nil, err
	}
	input.ChannelNo = strings.TrimSpace(input.ChannelNo)
	input.Cursor = strings.TrimSpace(input.Cursor)
	if input.Limit == 0 {
		input.Limit = defaultHealthPageSize
	}
	if input.Limit < 1 || input.Limit > maximumHealthPageSize ||
		input.Page < 0 || input.Page > 1_000_000 || (input.Page > 0 && input.Cursor != "") ||
		(input.ChannelNo != "" && resourcecap.ValidateChannelRef(input.ChannelNo) != nil) ||
		(input.Cursor != "" && resourcecap.ValidateChannelRef(input.Cursor) != nil) {
		return nil, bizerr.NewCode(CodeHealthManagementInvalid)
	}
	model := s.healthListModel(ctx)
	channelColumn := dao.Channel.Table() + "." + dao.Channel.Columns().ChannelNo
	if input.ChannelNo != "" {
		model = model.Where(channelColumn, input.ChannelNo)
	}
	total, err := model.Clone().Count()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeHealthManagementUnavailable)
	}
	if input.Page > 0 {
		model = model.Offset((input.Page - 1) * input.Limit)
	}
	if input.Cursor != "" {
		model = model.WhereGT(channelColumn, input.Cursor)
	}
	rows := make([]healthManagementRow, 0, input.Limit+1)
	if err := model.Fields(healthListFields()...).OrderAsc(channelColumn).Limit(input.Limit + 1).Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(err, CodeHealthManagementUnavailable)
	}
	hasMore := len(rows) > input.Limit
	if hasMore {
		rows = rows[:input.Limit]
	}
	page := &HealthPage{Items: make([]ChannelHealth, len(rows)), Total: total}
	for index := range rows {
		page.Items[index] = rows[index].projection()
	}
	if hasMore && len(page.Items) > 0 {
		page.NextCursor = page.Items[len(page.Items)-1].ChannelNo
	}
	return page, nil
}

// UpdatePolicy commits one authorized optimistic update and its business audit atomically.
func (s *managementService) UpdatePolicy(
	ctx context.Context,
	input HealthPolicyUpdateInput,
) (*ChannelHealth, error) {
	actor, err := s.authorize(ctx, permissionHealthPolicyWrite, input.ChannelNo)
	if err != nil {
		return nil, err
	}
	input.ChannelNo = strings.TrimSpace(input.ChannelNo)
	if validatePolicyUpdate(input) != nil {
		return nil, bizerr.NewCode(CodeHealthManagementInvalid)
	}
	var result *ChannelHealth
	err = dao.ChannelHealthState.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, timeErr := databaseTime(ctx)
		if timeErr != nil {
			return timeErr
		}
		if accessible, accessErr := s.lockChannel(ctx, input.ChannelNo); accessErr != nil {
			return accessErr
		} else if !accessible {
			return bizerr.NewCode(CodeHealthManagementNotAccessible)
		}
		row, loadErr := loadHealthState(ctx, input.ChannelNo)
		if loadErr != nil {
			return loadErr
		}
		if row == nil {
			if input.ExpectedVersion != 1 {
				return bizerr.NewCode(CodeHealthManagementConflict)
			}
			if _, loadErr = dao.ChannelHealthState.Ctx(ctx).Data(do.ChannelHealthState{
				TenantId: idccap.DefaultTenantID, ChannelNo: input.ChannelNo,
				WindowStartedAt: &now,
			}).InsertIgnore(); loadErr != nil {
				return gerror.Wrap(loadErr, "initialize resource Provider health policy")
			}
			row, loadErr = loadHealthState(ctx, input.ChannelNo)
			if loadErr != nil || row == nil {
				return gerror.Wrap(loadErr, "load initialized resource Provider health policy")
			}
		}
		if row.PolicyVersion != input.ExpectedVersion {
			return bizerr.NewCode(CodeHealthManagementConflict)
		}
		before := healthPolicyFromState(row)
		resetExpiredState(row, now)
		if input.WindowSampleLimit < row.SampleCount {
			return bizerr.NewCode(CodeHealthManagementConflict)
		}
		applyPolicyInput(row, input)
		if policyProtects(row) {
			candidate := now.Add(time.Duration(row.ProtectionDurationSeconds) * time.Second)
			if row.WriteProtectedUntil == nil || row.WriteProtectedUntil.Before(candidate) {
				row.WriteProtectedUntil = &candidate
			}
			row.CircuitOpen = true
		}
		columns := dao.ChannelHealthState.Columns()
		update, updateErr := dao.ChannelHealthState.Ctx(ctx).
			Where(do.ChannelHealthState{TenantId: idccap.DefaultTenantID, ChannelNo: input.ChannelNo}).
			Where(columns.PolicyVersion, input.ExpectedVersion).
			Data(do.ChannelHealthState{
				WindowSampleLimit:         row.WindowSampleLimit,
				MinimumSamplesToProtect:   row.MinimumSamplesToProtect,
				FailureLimit:              row.FailureLimit,
				UnknownResultLimit:        row.UnknownResultLimit,
				ConsecutiveFailureLimit:   row.ConsecutiveFailureLimit,
				ProtectionDurationSeconds: row.ProtectionDurationSeconds,
				PolicyUpdatedBy:           actor,
				PolicyUpdatedAt:           &now,
				PolicyVersion:             gdb.Raw(columns.PolicyVersion + " + 1"),
				Version:                   gdb.Raw(columns.Version + " + 1"),
				SampleCount:               row.SampleCount,
				FailureCount:              row.FailureCount,
				UnknownCount:              row.UnknownCount,
				ConsecutiveFailures:       row.ConsecutiveFailures,
				WindowStartedAt:           row.WindowStartedAt,
				CircuitOpen:               row.CircuitOpen,
				WriteProtectedUntil:       row.WriteProtectedUntil,
			}).Update()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "update resource Provider health policy")
		}
		if affected, affectedErr := update.RowsAffected(); affectedErr != nil || affected != 1 {
			return bizerr.NewCode(CodeHealthManagementConflict)
		}
		if row.WriteProtectedUntil == nil {
			if _, updateErr = dao.ChannelHealthState.Ctx(ctx).
				Where(do.ChannelHealthState{TenantId: idccap.DefaultTenantID, ChannelNo: input.ChannelNo}).
				Data(columns.WriteProtectedUntil, nil).Update(); updateErr != nil {
				return gerror.Wrap(updateErr, "clear resource Provider protection deadline")
			}
		}
		after := healthPolicyFromState(row)
		after.Version = input.ExpectedVersion + 1
		after.UpdatedAt = &now
		if auditErr := s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: auditPolicyUpdateOperation,
			IdempotencyKey:   input.ChannelNo + ":" + stringInt64(after.Version),
			OperationCode:    auditPolicyUpdateOperation,
			TargetType:       auditPolicyUpdateTargetType, TargetNo: input.ChannelNo,
			ResultCode: auditPolicyUpdateResult, ReasonCode: auditPolicyUpdateReason,
			RiskCode: auditPolicyUpdateRisk, RequiredPermission: permissionHealthPolicyWrite,
			BeforeSnapshot: before.auditSnapshot(), AfterSnapshot: after.auditSnapshot(),
		}); auditErr != nil {
			return auditErr
		}
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeHealthManagementInvalid) || bizerr.Is(err, CodeHealthManagementConflict) ||
			bizerr.Is(err, CodeHealthManagementNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeHealthManagementUnavailable)
	}
	result, err = s.loadExactProjection(ctx, input.ChannelNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeHealthManagementUnavailable)
	}
	return result, nil
}

// authorize performs the required exact permission check before input-sensitive work.
func (s *managementService) authorize(ctx context.Context, permission, target string) (int, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return 0, bizerr.NewCode(CodeHealthManagementNotAccessible)
	}
	actor, err := s.runtime.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: permission, TargetNo: strings.TrimSpace(target),
	})
	if err != nil || actor <= 0 {
		return 0, bizerr.NewCode(CodeHealthManagementNotAccessible)
	}
	return actor, nil
}

// lockChannel locks one automatically soft-delete-filtered channel row.
func (s *managementService) lockChannel(ctx context.Context, channelNo string) (bool, error) {
	var row struct {
		ChannelNo string `json:"channelNo"`
	}
	err := dao.Channel.Ctx(ctx).
		Fields(dao.Channel.Columns().ChannelNo).
		Where(do.Channel{TenantId: idccap.DefaultTenantID, ChannelNo: channelNo}).
		LockUpdate().Scan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return row.ChannelNo != "", err
}

// validatePolicyUpdate rejects unsafe bounds and inconsistent thresholds.
func validatePolicyUpdate(input HealthPolicyUpdateInput) error {
	if resourcecap.ValidateChannelRef(strings.TrimSpace(input.ChannelNo)) != nil ||
		input.ExpectedVersion <= 0 || input.WindowSampleLimit < 5 || input.WindowSampleLimit > 100 ||
		input.MinimumSamplesToProtect < 3 || input.MinimumSamplesToProtect > input.WindowSampleLimit ||
		input.FailureLimit < 1 || input.FailureLimit > input.WindowSampleLimit ||
		input.UnknownResultLimit < 1 || input.UnknownResultLimit > input.FailureLimit ||
		input.ConsecutiveFailureLimit < 1 || input.ConsecutiveFailureLimit > input.WindowSampleLimit ||
		input.ProtectionDurationSeconds < 60 || input.ProtectionDurationSeconds > 86400 {
		return gerror.New("resource Provider health policy is invalid")
	}
	return nil
}

// applyPolicyInput copies validated policy values into the locked state.
func applyPolicyInput(row *healthState, input HealthPolicyUpdateInput) {
	row.WindowSampleLimit = input.WindowSampleLimit
	row.MinimumSamplesToProtect = input.MinimumSamplesToProtect
	row.FailureLimit = input.FailureLimit
	row.UnknownResultLimit = input.UnknownResultLimit
	row.ConsecutiveFailureLimit = input.ConsecutiveFailureLimit
	row.ProtectionDurationSeconds = input.ProtectionDurationSeconds
}

// healthPolicyFromState returns a detached management projection.
func healthPolicyFromState(row *healthState) HealthPolicy {
	return HealthPolicy{
		WindowSampleLimit: row.WindowSampleLimit, MinimumSamplesToProtect: row.MinimumSamplesToProtect,
		FailureLimit: row.FailureLimit, UnknownResultLimit: row.UnknownResultLimit,
		ConsecutiveFailureLimit:   row.ConsecutiveFailureLimit,
		ProtectionDurationSeconds: row.ProtectionDurationSeconds,
		Version:                   row.PolicyVersion, UpdatedAt: utcTimePointer(row.PolicyUpdatedAt),
	}
}

// auditSnapshot returns the bounded, non-sensitive policy audit payload.
func (p HealthPolicy) auditSnapshot() map[string]any {
	return map[string]any{
		"window_sample_limit":         p.WindowSampleLimit,
		"minimum_samples_to_protect":  p.MinimumSamplesToProtect,
		"failure_limit":               p.FailureLimit,
		"unknown_result_limit":        p.UnknownResultLimit,
		"consecutive_failure_limit":   p.ConsecutiveFailureLimit,
		"protection_duration_seconds": p.ProtectionDurationSeconds,
		"version":                     p.Version,
	}
}

type healthManagementRow struct {
	entity.ChannelHealthState
	ChannelName   string `json:"channelName"`
	ProviderID    string `json:"providerId"`
	ChannelStatus string `json:"channelStatus"`
}

// healthListModel builds one joined projection with automatic channel soft-delete filtering.
func (s *managementService) healthListModel(ctx context.Context) *gdb.Model {
	table := dao.Channel.Table()
	return s.database.Model(dao.Channel.Table()).Safe().Ctx(ctx).
		LeftJoin(
			dao.ChannelHealthState.Table()+" h",
			"h.channel_no="+table+".channel_no AND h.tenant_id="+table+".tenant_id",
		).
		Where(do.Channel{TenantId: idccap.DefaultTenantID})
}

// healthListFields applies the display projection only after the exact count query.
func healthListFields() []any {
	table := dao.Channel.Table()
	return []any{
		"h.*", table + ".channel_no", table + ".display_name AS channel_name", table + ".provider_id",
		table + ".status_code AS channel_status",
		"CASE WHEN h.circuit_open AND h.write_protected_until > CURRENT_TIMESTAMP THEN TRUE ELSE FALSE END AS circuit_open",
		"CASE WHEN h.circuit_open AND h.write_protected_until > CURRENT_TIMESTAMP THEN h.write_protected_until ELSE NULL END AS write_protected_until",
	}
}

// loadExactProjection reloads the committed channel and policy projection.
func (s *managementService) loadExactProjection(ctx context.Context, channelNo string) (*ChannelHealth, error) {
	var row healthManagementRow
	err := s.healthListModel(ctx).
		Fields(healthListFields()...).
		Where(dao.Channel.Table()+"."+dao.Channel.Columns().ChannelNo, channelNo).
		Limit(1).Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "load resource Provider health projection")
	}
	if row.ChannelNo == "" {
		return nil, gerror.New("resource Provider health projection is unavailable")
	}
	result := row.projection()
	return &result, nil
}

// projection converts one joined row without additional storage or Provider calls.
func (row healthManagementRow) projection() ChannelHealth {
	state := row.ChannelHealthState
	if state.WindowSampleLimit == 0 {
		applyDefaultPolicy(&state)
		state.PolicyVersion = 1
	}
	healthCode := overallHealthCode(&state)
	return ChannelHealth{
		ChannelNo: row.ChannelNo, ChannelName: row.ChannelName, ProviderID: row.ProviderID,
		ChannelStatus: row.ChannelStatus, HealthCode: healthCode,
		CircuitOpen: row.CircuitOpen, WriteProtectedUntil: utcTimePointer(row.WriteProtectedUntil),
		SampleCount: row.SampleCount, FailureCount: row.FailureCount,
		UnknownCount: row.UnknownCount, ConsecutiveFailures: row.ConsecutiveFailures,
		LastSummaryCode: row.LastSummaryCode, LastObservedAt: utcTimePointer(row.LastObservedAt),
		Policy:           healthPolicyFromState(&state),
		Authentication:   healthObservation(row.LastAuthenticationHealthCode, row.LastAuthenticationSummaryCode, row.LastAuthenticationLatencyMs, row.LastAuthenticationObservedAt),
		Query:            healthObservation(row.LastQueryHealthCode, row.LastQuerySummaryCode, row.LastQueryLatencyMs, row.LastQueryObservedAt),
		Write:            healthObservation(row.LastWriteHealthCode, row.LastWriteSummaryCode, row.LastWriteLatencyMs, row.LastWriteObservedAt),
		ProviderCapacity: healthObservation(row.LastProviderCapacityHealthCode, row.LastProviderCapacitySummaryCode, row.LastProviderCapacityLatencyMs, row.LastProviderCapacityObservedAt),
	}
}

// overallHealthCode derives a conservative current aggregate state.
func overallHealthCode(row *healthState) HealthCode {
	if row.CircuitOpen {
		return HealthUnavailable
	}
	codes := []string{row.LastAuthenticationHealthCode, row.LastQueryHealthCode, row.LastWriteHealthCode, row.LastProviderCapacityHealthCode}
	seenHealthy := false
	for _, code := range codes {
		if code == string(HealthDegraded) || code == string(HealthUnavailable) {
			return HealthDegraded
		}
		seenHealthy = seenHealthy || code == string(HealthHealthy)
	}
	if seenHealthy {
		return HealthHealthy
	}
	return HealthUnknown
}

// healthObservation normalizes an absent projection to unknown.
func healthObservation(code, summary string, latency int, observedAt *time.Time) HealthObservation {
	if code == "" {
		code = string(HealthUnknown)
	}
	return HealthObservation{HealthCode: HealthCode(code), SummaryCode: summary, LatencyMS: latency, ObservedAt: utcTimePointer(observedAt)}
}

// utcTimePointer detaches and normalizes one optional timestamp.
func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

// stringInt64 formats an optimistic version for the idempotency key.
func stringInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}
