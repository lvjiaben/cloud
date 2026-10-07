// This file implements bounded, lease-fenced reconciliation of accepted upstream tasks.

package resourcetask

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

const (
	observationUnavailableCode    = "OBSERVATION_UNAVAILABLE"
	observationUnavailableSummary = "provider task observation is temporarily unavailable"
	upstreamTerminatedCode        = "UPSTREAM_TERMINATED"
	upstreamTerminatedSummary     = "upstream task terminated before successful completion"
	minimumObservationBackoff     = 5 * time.Second
	maximumObservationBackoff     = 5 * time.Minute
	maximumObservationExponent    = 6
)

// ObservationClaimInput bounds one transactional read-only observation claim.
type ObservationClaimInput struct {
	WorkerID      string
	Limit         int
	LeaseDuration time.Duration
}

// ObservationLeaseRef identifies one exact observation ownership version.
// Version is incremented when the observation is claimed and fences stale workers.
type ObservationLeaseRef struct {
	TaskNo  string
	Token   string
	Version int64
}

// ClaimedObservation contains the exact upstream task identity required for
// one read-only Provider query. It never contains task input secret handles.
type ClaimedObservation struct {
	TaskNo                string
	ChannelNo             string
	ConfigurationRevision string
	LocalResourceNo       string
	CurrentStatus         Status
	ExternalTaskRef       string
	ExternalResourceRef   string
	LeaseToken            string
	LeaseVersion          int64
	ObservationAttempt    int
	LeaseExpiresAt        time.Time
}

// ObservationInput applies one validated QueryTask result while the exact
// observation lease remains live. NextAt is required for non-terminal results.
type ObservationInput struct {
	Lease  ObservationLeaseRef
	Result resourcecap.OperationResult
	NextAt *time.Time
}

// ObservationRescheduleInput releases a read-only query lease after a
// normalized call failure without changing the last known task state.
type ObservationRescheduleInput struct {
	Lease  ObservationLeaseRef
	NextAt time.Time
}

// ClaimObservations leases a bounded due batch with one task query and one
// latest-binding query. Expired observation leases are safe to reclaim because
// QueryTask is read-only; Provider writes are never repeated by this path.
func (s *Store) ClaimObservations(
	ctx context.Context,
	input ObservationClaimInput,
) ([]ClaimedObservation, error) {
	if s == nil || s.database == nil || s.random == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("resource task store is unavailable")
	}
	if err := validateObservationClaimInput(input); err != nil {
		return nil, err
	}

	claimed := make([]ClaimedObservation, 0)
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := resourceDatabaseTime(tx)
		if err != nil {
			return err
		}
		rows, err := loadDueObservationTasks(ctx, tx, now, input.Limit)
		if err != nil || len(rows) == 0 {
			return err
		}
		taskNos := make([]string, len(rows))
		ids := make([]int64, len(rows))
		for index := range rows {
			ids[index] = rows[index].Id
			taskNos[index] = rows[index].TaskNo
		}
		bindings, err := loadLatestUpstreamBindings(ctx, tx, taskNos)
		if err != nil {
			return err
		}
		if len(bindings) != len(rows) {
			return gerror.New("resource task observation binding cardinality is invalid")
		}
		leaseToken, err := newLeaseToken(s.random, input.WorkerID)
		if err != nil {
			return err
		}
		leaseExpiresAt := now.Add(input.LeaseDuration)
		columns := dao.Task.Columns()
		result, err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
			Where(do.Task{TenantId: idccap.DefaultTenantID}).
			WhereIn(columns.Id, ids).
			WhereIn(columns.StatusCode, observationStatuses()).
			Data(do.Task{
				LockedBy:       leaseToken,
				LeaseExpiresAt: &leaseExpiresAt,
				ObservationAttemptCount: gdb.Raw(
					columns.ObservationAttemptCount + " + 1",
				),
				Version: gdb.Raw(columns.Version + " + 1"),
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "lease resource task observation batch")
		}
		if err = requireAffected(result, len(rows), "lease resource task observation batch"); err != nil {
			return err
		}
		claimed = make([]ClaimedObservation, 0, len(rows))
		for index := range rows {
			binding, exists := bindings[rows[index].TaskNo]
			if !exists {
				return gerror.New("resource task observation binding is unavailable")
			}
			query := resourcecap.TaskQuery{
				ExternalTaskRef: binding.ExternalTaskRef,
				ResourceRef:     binding.ExternalResourceRef,
			}
			if query.Validate() != nil || binding.ChannelNo != rows[index].ChannelNo {
				return gerror.New("resource task observation binding is invalid")
			}
			claimed = append(claimed, ClaimedObservation{
				TaskNo:                rows[index].TaskNo,
				ChannelNo:             rows[index].ChannelNo,
				ConfigurationRevision: taskConfigurationRevision(rows[index].OwnerMetadata),
				LocalResourceNo:       rows[index].LocalResourceNo,
				CurrentStatus:         Status(rows[index].StatusCode),
				ExternalTaskRef:       binding.ExternalTaskRef,
				ExternalResourceRef:   binding.ExternalResourceRef,
				LeaseToken:            leaseToken,
				LeaseVersion:          rows[index].Version + 1,
				ObservationAttempt:    rows[index].ObservationAttemptCount + 1,
				LeaseExpiresAt:        leaseExpiresAt,
			})
		}
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "claim resource task observations")
	}
	return claimed, nil
}

// ObserveTask reconciles one normalized task observation atomically with its
// upstream binding. Same-state refreshes are allowed only through this method,
// which requires exact observation identity and a live lease.
func (s *Store) ObserveTask(ctx context.Context, input ObservationInput) error {
	if s == nil || s.database == nil || s.random == nil || ctx == nil || ctx.Err() != nil {
		return gerror.New("resource task store is unavailable")
	}
	if err := validateObservationLeaseRef(input.Lease); err != nil {
		return err
	}
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := resourceDatabaseTime(tx)
		if err != nil {
			return err
		}
		row, err := loadLiveObservedTask(ctx, tx, input.Lease, now)
		if err != nil {
			return err
		}
		if row == nil {
			return gerror.New("resource task observation lease is unavailable")
		}
		bindings, err := loadLatestUpstreamBindings(ctx, tx, []string{row.TaskNo})
		if err != nil {
			return err
		}
		binding, exists := bindings[row.TaskNo]
		if !exists {
			return gerror.New("resource task observation binding is unavailable")
		}
		to, err := validateObservationResult(input, row, &binding, now)
		if err != nil {
			return err
		}
		if err = persistObservedBinding(ctx, tx, &binding, input.Result, now); err != nil {
			return err
		}
		if to == StatusSucceeded {
			resourceRef := operationResultResourceRefOrExpected(input.Result, binding.ExternalResourceRef)
			if err = convergeSuccessfulProvision(ctx, tx, row, resourceRef, s.random); err != nil {
				return err
			}
		}
		providerFacts, err := AppendResidualResourceEvidence(
			input.Result.Evidence, input.Result.ResidualResources, now)
		if err != nil {
			return err
		}
		if err = insertTaskEvidenceBatch(
			ctx,
			tx,
			row.TaskNo,
			taskEvidenceSourceProviderTask,
			providerFacts,
			s.random,
		); err != nil {
			return err
		}
		return persistObservedTask(ctx, tx, row, input, to, now)
	})
}

// RescheduleObservation records only a fixed owner-safe query outage summary,
// releases the lease, and retains accepted/running/result_unknown exactly.
func (s *Store) RescheduleObservation(
	ctx context.Context,
	input ObservationRescheduleInput,
) error {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil {
		return gerror.New("resource task store is unavailable")
	}
	if err := validateObservationLeaseRef(input.Lease); err != nil {
		return err
	}
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := resourceDatabaseTime(tx)
		if err != nil {
			return err
		}
		next := input.NextAt.UTC()
		if !next.After(now) || next.After(now.Add(maximumTransitionScheduleDelay)) {
			return gerror.New("resource task observation schedule is invalid")
		}
		row, err := loadLiveObservedTask(ctx, tx, input.Lease, now)
		if err != nil {
			return err
		}
		if row == nil {
			return gerror.New("resource task observation lease is unavailable")
		}
		columns := dao.Task.Columns()
		result, err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
			Where(do.Task{
				TenantId:   idccap.DefaultTenantID,
				Id:         row.Id,
				TaskNo:     input.Lease.TaskNo,
				LockedBy:   input.Lease.Token,
				Version:    input.Lease.Version,
				StatusCode: row.StatusCode,
			}).
			WhereNotNull(columns.LeaseExpiresAt).
			WhereGT(columns.LeaseExpiresAt, now).
			Data(do.Task{
				LockedBy:       "",
				NextObserveAt:  &next,
				FailureCode:    observationUnavailableCode,
				FailureSummary: observationUnavailableSummary,
				Version:        gdb.Raw(columns.Version + " + 1"),
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "reschedule resource task observation")
		}
		if err = requireAffected(result, 1, "reschedule resource task observation"); err != nil {
			return err
		}
		return clearTaskLeaseExpiry(ctx, tx, row.Id)
	})
}

// validateObservationClaimInput applies the same worker, batch, and lease
// bounds used by write claims without converting the operation into a write.
func validateObservationClaimInput(input ObservationClaimInput) error {
	return validateClaimInput(ClaimInput(input))
}

// validateObservationLeaseRef rejects malformed or pre-claim observation identities.
func validateObservationLeaseRef(lease ObservationLeaseRef) error {
	if !workerIDPattern.MatchString(lease.TaskNo) || !leaseTokenPattern.MatchString(lease.Token) || lease.Version <= 1 {
		return gerror.New("resource task observation lease reference is invalid")
	}
	return nil
}

// observationStatuses returns the only non-terminal states eligible for task queries.
func observationStatuses() []Status {
	return []Status{StatusAccepted, StatusRunning, StatusResultUnknown}
}

// loadDueObservationTasks locks one bounded, deterministically ordered due batch.
func loadDueObservationTasks(
	ctx context.Context,
	tx gdb.TX,
	now time.Time,
	limit int,
) ([]entity.Task, error) {
	var rows []entity.Task
	taskColumns := dao.Task.Columns()
	bindingColumns := dao.UpstreamTaskBinding.Columns()
	taskModel := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).As("task")
	leaseAvailable := taskModel.Builder().
		WhereNull("task."+taskColumns.LeaseExpiresAt).
		WhereOrLTE("task."+taskColumns.LeaseExpiresAt, now)
	bindingExists := tx.Model(dao.UpstreamTaskBinding.Table()).Safe().Ctx(ctx).As("binding").
		Fields("binding." + bindingColumns.Id).
		Where(do.UpstreamTaskBinding{TenantId: idccap.DefaultTenantID}).
		Where("binding." + bindingColumns.TaskNo + " = task." + taskColumns.TaskNo)
	err := taskModel.
		Fields(
			"task."+taskColumns.Id,
			"task."+taskColumns.TaskNo,
			"task."+taskColumns.LocalResourceNo,
			"task."+taskColumns.ChannelNo,
			"task."+taskColumns.OwnerMetadata,
			"task."+taskColumns.StatusCode,
			"task."+taskColumns.Version,
			"task."+taskColumns.ObservationAttemptCount,
		).
		Where(do.Task{TenantId: idccap.DefaultTenantID}).
		WhereIn("task."+taskColumns.StatusCode, observationStatuses()).
		WhereNotNull("task."+taskColumns.NextObserveAt).
		WhereLTE("task."+taskColumns.NextObserveAt, now).
		Where(leaseAvailable).
		WhereExists(bindingExists).
		OrderAsc("task." + taskColumns.NextObserveAt).
		OrderAsc("task." + taskColumns.Priority).
		OrderAsc("task." + taskColumns.Id).
		Limit(limit).
		LockUpdateSkipLocked().
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "load due resource task observations")
	}
	return rows, nil
}

// loadLatestUpstreamBindings returns at most one highest-step binding per task
// with one PostgreSQL projection, avoiding per-task database queries.
func loadLatestUpstreamBindings(
	ctx context.Context,
	tx gdb.TX,
	taskNos []string,
) (map[string]entity.UpstreamTaskBinding, error) {
	bindings := make(map[string]entity.UpstreamTaskBinding, len(taskNos))
	if len(taskNos) == 0 {
		return bindings, nil
	}
	columns := dao.UpstreamTaskBinding.Columns()
	var rows []entity.UpstreamTaskBinding
	err := tx.Model(dao.UpstreamTaskBinding.Table()).Safe().Ctx(ctx).
		Fields(
			"DISTINCT ON ("+columns.TaskNo+") "+columns.Id,
			columns.TaskNo,
			columns.StepNo,
			columns.ChannelNo,
			columns.ExternalTaskRef,
			columns.ExternalResourceRef,
			columns.StatusCode,
			columns.LastProgress,
		).
		Where(do.UpstreamTaskBinding{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.TaskNo, taskNos).
		OrderAsc(columns.TaskNo).
		OrderDesc(columns.StepNo).
		Limit(len(taskNos)).
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "load latest upstream resource task bindings")
	}
	for index := range rows {
		if _, duplicate := bindings[rows[index].TaskNo]; duplicate {
			return nil, gerror.New("resource task observation binding is duplicated")
		}
		bindings[rows[index].TaskNo] = rows[index]
	}
	return bindings, nil
}

// loadLiveObservedTask locks the exact still-live observation lease for reconciliation.
func loadLiveObservedTask(
	ctx context.Context,
	tx gdb.TX,
	lease ObservationLeaseRef,
	now time.Time,
) (*entity.Task, error) {
	columns := dao.Task.Columns()
	var row entity.Task
	err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
		Fields(
			columns.Id,
			columns.TaskNo,
			columns.CapabilityCode,
			columns.LocalResourceNo,
			columns.ChannelNo,
			columns.PoolNo,
			columns.StatusCode,
			columns.SanitizedParameters,
			columns.AcceptedAt,
			columns.ObservationAttemptCount,
			columns.ObservationClaimedAt,
		).
		Where(do.Task{
			TenantId: idccap.DefaultTenantID,
			TaskNo:   lease.TaskNo,
			LockedBy: lease.Token,
			Version:  lease.Version,
		}).
		WhereIn(columns.StatusCode, observationStatuses()).
		WhereNotNull(columns.LeaseExpiresAt).
		WhereGT(columns.LeaseExpiresAt, now).
		LockUpdate().
		Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "load live resource task observation lease")
	}
	if row.TaskNo == "" {
		return nil, nil
	}
	return &row, nil
}

// validateObservationResult verifies the normalized envelope, exact upstream
// identity, local transition, and terminal or non-terminal schedule semantics.
func validateObservationResult(
	input ObservationInput,
	row *entity.Task,
	binding *entity.UpstreamTaskBinding,
	now time.Time,
) (Status, error) {
	if err := input.Result.Validate(); err != nil || len(input.Result.SecretOutputs) != 0 {
		return "", gerror.New("resource task observation result is invalid")
	}
	if len(input.Result.ResidualResources) != 0 &&
		resourcecap.CapabilityCode(row.CapabilityCode) != resourcecap.CapabilityTerminate {
		return "", gerror.New("resource task residual resources require terminate capability")
	}
	if input.Result.ExternalTaskRef != binding.ExternalTaskRef {
		return "", gerror.New("resource task observation task reference does not match")
	}
	observedResourceRef := input.Result.ExternalResourceRef
	if input.Result.Resource != nil {
		observedResourceRef = input.Result.Resource.ExternalResourceRef
	}
	if binding.ExternalResourceRef != "" && observedResourceRef != "" &&
		binding.ExternalResourceRef != observedResourceRef {
		return "", gerror.New("resource task observation resource reference does not match")
	}
	to, err := localStatusForObservation(Status(row.StatusCode), input.Result.Status)
	if err != nil {
		return "", err
	}
	terminal := to == StatusSucceeded || to == StatusFailed || to == StatusCanceled
	if terminal {
		if input.NextAt != nil {
			return "", gerror.New("terminal resource task observation does not accept a schedule")
		}
	} else {
		if input.NextAt == nil {
			return "", gerror.New("resource task observation schedule is required")
		}
		next := input.NextAt.UTC()
		if !next.After(now) || next.After(now.Add(maximumTransitionScheduleDelay)) {
			return "", gerror.New("resource task observation schedule is invalid")
		}
	}
	if to != Status(row.StatusCode) {
		if err = ValidateTransition(Status(row.StatusCode), to, TransitionEvidence{ObservationProven: true}); err != nil {
			return "", err
		}
	}
	return to, nil
}

// nextObservationAt returns a deterministic, bounded exponential schedule for
// read-only reconciliation. It never changes write attempt counters or status.
func nextObservationAt(taskNo string, attempt int, now time.Time) time.Time {
	if attempt < 1 {
		attempt = 1
	}
	exponent := attempt - 1
	if exponent > maximumObservationExponent {
		exponent = maximumObservationExponent
	}
	delay := minimumObservationBackoff * time.Duration(1<<exponent)
	if delay > maximumObservationBackoff {
		delay = maximumObservationBackoff
	}
	digest := sha256.Sum256([]byte(taskNo))
	jitterRange := delay / 5
	if jitterRange > 0 {
		delay += time.Duration(binary.BigEndian.Uint64(digest[:8]) % uint64(jitterRange))
	}
	if delay > maximumObservationBackoff {
		delay = maximumObservationBackoff
	}
	return now.UTC().Add(delay)
}

// localStatusForObservation maps a normalized upstream state without allowing
// an unknown observation to erase the last known local state.
func localStatusForObservation(current Status, observed resourcecap.TaskStatus) (Status, error) {
	switch observed {
	case resourcecap.TaskStatusAccepted:
		if current == StatusRunning {
			return StatusRunning, nil
		}
		return StatusAccepted, nil
	case resourcecap.TaskStatusRunning:
		return StatusRunning, nil
	case resourcecap.TaskStatusSucceeded:
		return StatusSucceeded, nil
	case resourcecap.TaskStatusFailed, resourcecap.TaskStatusTerminated:
		return StatusFailed, nil
	case resourcecap.TaskStatusCanceled:
		return StatusCanceled, nil
	case resourcecap.TaskStatusUnknown:
		return current, nil
	default:
		return "", gerror.New("resource task observation status is invalid")
	}
}

// persistObservedBinding updates only the authoritative latest binding selected
// for the claimed task and clears absent progress to a real SQL NULL.
func persistObservedBinding(
	ctx context.Context,
	tx gdb.TX,
	binding *entity.UpstreamTaskBinding,
	result resourcecap.OperationResult,
	now time.Time,
) error {
	resourceRef := binding.ExternalResourceRef
	if result.ExternalResourceRef != "" {
		resourceRef = result.ExternalResourceRef
	}
	if result.Resource != nil {
		resourceRef = result.Resource.ExternalResourceRef
	}
	columns := dao.UpstreamTaskBinding.Columns()
	update, err := tx.Model(dao.UpstreamTaskBinding.Table()).Safe().Ctx(ctx).
		Where(do.UpstreamTaskBinding{
			TenantId:        idccap.DefaultTenantID,
			Id:              binding.Id,
			TaskNo:          binding.TaskNo,
			StepNo:          binding.StepNo,
			ChannelNo:       binding.ChannelNo,
			ExternalTaskRef: binding.ExternalTaskRef,
		}).
		Data(do.UpstreamTaskBinding{
			ExternalResourceRef: resourceRef,
			StatusCode:          result.Status,
			LastProgress:        result.Progress,
			LastObservedAt:      &now,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "persist upstream resource task observation")
	}
	if err = requireAffected(update, 1, "persist upstream resource task observation"); err != nil {
		return err
	}
	if result.Progress == nil {
		clearResult, clearErr := tx.Model(dao.UpstreamTaskBinding.Table()).Safe().Ctx(ctx).
			Where(do.UpstreamTaskBinding{TenantId: idccap.DefaultTenantID, Id: binding.Id}).
			Data(columns.LastProgress, nil).
			Update()
		if clearErr != nil {
			return gerror.Wrap(clearErr, "clear upstream resource task observation progress")
		}
		return requireAffected(clearResult, 1, "clear upstream resource task observation progress")
	}
	return nil
}

// persistObservedTask applies the lease-fenced local result and closes terminal schedules.
func persistObservedTask(
	ctx context.Context,
	tx gdb.TX,
	row *entity.Task,
	input ObservationInput,
	to Status,
	now time.Time,
) error {
	failureCode, failureSummary := observationFailure(input.Result)
	columns := dao.Task.Columns()
	data := do.Task{
		StatusCode:     to,
		LockedBy:       "",
		ResultUnknown:  to == StatusResultUnknown,
		FailureCode:    failureCode,
		FailureSummary: failureSummary,
		Version:        gdb.Raw(columns.Version + " + 1"),
	}
	if input.NextAt != nil {
		next := input.NextAt.UTC()
		data.NextObserveAt = &next
	}
	if row.AcceptedAt == nil && (to == StatusAccepted || to == StatusRunning || to == StatusSucceeded) {
		data.AcceptedAt = &now
	}
	terminal := to == StatusSucceeded || to == StatusFailed || to == StatusCanceled
	if to != Status(row.StatusCode) || terminal {
		data.ObservationAttemptCount = 0
	}
	if terminal {
		data.FinishedAt = &now
	}
	result, err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
		Where(do.Task{
			TenantId:   idccap.DefaultTenantID,
			Id:         row.Id,
			TaskNo:     input.Lease.TaskNo,
			LockedBy:   input.Lease.Token,
			Version:    input.Lease.Version,
			StatusCode: row.StatusCode,
		}).
		WhereNotNull(columns.LeaseExpiresAt).
		WhereGT(columns.LeaseExpiresAt, now).
		Data(data).
		Update()
	if err != nil {
		return gerror.Wrap(err, "reconcile observed resource task")
	}
	if err = requireAffected(result, 1, "reconcile observed resource task"); err != nil {
		return err
	}
	if err = clearTaskLeaseExpiry(ctx, tx, row.Id); err != nil {
		return err
	}
	if terminal {
		return clearTaskNextObservation(ctx, tx, row.Id)
	}
	return nil
}

// observationFailure projects only validated, redacted Provider failure metadata.
func observationFailure(result resourcecap.OperationResult) (string, string) {
	if result.Failure != nil {
		return result.Failure.Code, result.Failure.Message
	}
	if result.Status == resourcecap.TaskStatusTerminated {
		return upstreamTerminatedCode, upstreamTerminatedSummary
	}
	return "", ""
}

// clearTaskLeaseExpiry clears one observation lease expiry after its fenced update.
func clearTaskLeaseExpiry(ctx context.Context, tx gdb.TX, id int64) error {
	return clearTaskLeaseExpiries(ctx, tx, []int64{id})
}

// clearTaskLeaseExpiries is the narrow fixed-column exception required to
// persist SQL NULL because generated nullable time fields omit nil pointers.
func clearTaskLeaseExpiries(ctx context.Context, tx gdb.TX, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	columns := dao.Task.Columns()
	result, err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
		Where(do.Task{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.Id, ids).
		Data(columns.LeaseExpiresAt, nil).
		Update()
	if err != nil {
		return gerror.Wrap(err, "clear resource task observation lease expiry")
	}
	return requireAffected(result, len(ids), "clear resource task observation lease expiry")
}

// clearTaskNextObservation removes terminal tasks from the observation queue.
func clearTaskNextObservation(ctx context.Context, tx gdb.TX, id int64) error {
	columns := dao.Task.Columns()
	result, err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
		Where(do.Task{TenantId: idccap.DefaultTenantID, Id: id}).
		Data(columns.NextObserveAt, nil).
		Update()
	if err != nil {
		return gerror.Wrap(err, "clear resource task next observation")
	}
	return requireAffected(result, 1, "clear resource task next observation")
}
