// This file implements bounded Provider-write task claiming and crash recovery.

package resourcetask

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"io"
	"regexp"
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
	maximumClaimBatchSize = 32
	minimumLeaseDuration  = time.Second
	maximumLeaseDuration  = 5 * time.Minute
	expiredWriteCode      = "WRITE_LEASE_EXPIRED"
	expiredWriteSummary   = "provider write ownership expired before a result was persisted"
	invalidTaskCode       = "INVALID_PERSISTED_TASK"
	invalidTaskSummary    = "resource task failed owner validation before provider submission"
)

var workerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Store owns durable Provider task claims against an explicitly injected database.
type Store struct {
	database gdb.DB
	random   io.Reader
}

// NewStore constructs a task store. Runtime composition must inject the owner database.
func NewStore(database gdb.DB) (*Store, error) {
	if database == nil {
		return nil, gerror.New("resource task database is required")
	}
	return &Store{database: database, random: rand.Reader}, nil
}

// ClaimInput bounds one transactional task-claim batch.
type ClaimInput struct {
	WorkerID      string
	Limit         int
	LeaseDuration time.Duration
}

// ClaimedTask is the non-secret execution projection returned to the owner worker.
type ClaimedTask struct {
	ID                    int64
	TaskNo                string
	RequestIDDigest       string
	CapabilityCode        string
	LocalResourceNo       string
	ChannelNo             string
	ConfigurationRevision string
	PoolNo                string
	OwnerMetadata         string
	SanitizedParameters   string
	AttemptNo             int
	MaxAttempts           int
	LeaseToken            string
	LeaseExpiresAt        time.Time
	SecretBindings        []resourcecap.SecretBinding
	ProviderCorrelation   *resourcecap.ProviderCorrelation
}

// Claim first freezes expired in-flight writes as result_unknown, then leases
// only pending/retry_wait tasks. An interrupted external write is never made
// claimable again because its acceptance outcome cannot be inferred safely.
func (s *Store) Claim(ctx context.Context, input ClaimInput) ([]ClaimedTask, error) {
	if s == nil || s.database == nil || s.random == nil ||
		ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("resource task store is unavailable")
	}
	if err := validateClaimInput(input); err != nil {
		return nil, err
	}

	claimed := make([]ClaimedTask, 0)
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := resourceDatabaseTime(tx)
		if err != nil {
			return err
		}
		if err := freezeExpiredWrites(ctx, tx, now, input.Limit); err != nil {
			return err
		}
		rows, err := loadClaimableTasks(ctx, tx, now, input.Limit)
		if err != nil || len(rows) == 0 {
			return err
		}
		rows, err = rejectInvalidClaimableTasks(ctx, tx, rows, now)
		if err != nil || len(rows) == 0 {
			return err
		}
		leaseToken, err := newLeaseToken(s.random, input.WorkerID)
		if err != nil {
			return err
		}
		leaseExpiresAt := now.Add(input.LeaseDuration)
		ids := make([]int64, len(rows))
		for index := range rows {
			ids[index] = rows[index].Id
		}
		columns := dao.Task.Columns()
		result, err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
			Where(do.Task{TenantId: idccap.DefaultTenantID}).
			WhereIn(columns.Id, ids).
			WhereIn(columns.StatusCode, []Status{StatusPending, StatusRetryWait}).
			WhereLT(columns.AttemptCount, gdb.Raw(columns.MaxAttempts)).
			Data(do.Task{
				StatusCode:     StatusSubmitting,
				LockedBy:       leaseToken,
				LeaseExpiresAt: &leaseExpiresAt,
				AttemptCount:   gdb.Raw(columns.AttemptCount + " + 1"),
				Version:        gdb.Raw(columns.Version + " + 1"),
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "lease resource task batch")
		}
		if err = requireAffected(result, len(rows), "lease resource task batch"); err != nil {
			return err
		}
		taskNos := make([]string, len(rows))
		for index := range rows {
			taskNos[index] = rows[index].TaskNo
		}
		secretBindings, err := loadInputTaskSecretBindings(ctx, tx, taskNos)
		if err != nil {
			return err
		}
		correlations, err := loadTaskProviderCorrelations(tx, taskNos, secretBindings)
		if err != nil {
			return err
		}
		for index := range rows {
			request := resourcecap.OperationRequest{
				RequestID:           rows[index].TaskNo,
				Capability:          resourcecap.CapabilityCode(rows[index].CapabilityCode),
				ResourceRef:         rows[index].LocalResourceNo,
				Parameters:          []byte(rows[index].SanitizedParameters),
				SecretBindings:      secretBindings[rows[index].TaskNo],
				ProviderCorrelation: correlations[rows[index].TaskNo],
			}
			if request.Validate() != nil {
				return gerror.New("persisted resource task secret bindings are invalid")
			}
		}
		claimed = make([]ClaimedTask, 0, len(rows))
		for index := range rows {
			claimed = append(claimed, ClaimedTask{
				ID:                    rows[index].Id,
				TaskNo:                rows[index].TaskNo,
				RequestIDDigest:       rows[index].RequestIdDigest,
				CapabilityCode:        rows[index].CapabilityCode,
				LocalResourceNo:       rows[index].LocalResourceNo,
				ChannelNo:             rows[index].ChannelNo,
				ConfigurationRevision: taskConfigurationRevision(rows[index].OwnerMetadata),
				PoolNo:                rows[index].PoolNo,
				OwnerMetadata:         rows[index].OwnerMetadata,
				SanitizedParameters:   rows[index].SanitizedParameters,
				AttemptNo:             rows[index].AttemptCount + 1,
				MaxAttempts:           rows[index].MaxAttempts,
				LeaseToken:            leaseToken,
				LeaseExpiresAt:        leaseExpiresAt,
				SecretBindings:        secretBindings[rows[index].TaskNo],
				ProviderCorrelation:   correlations[rows[index].TaskNo],
			})
		}
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "claim resource tasks")
	}
	return claimed, nil
}

func loadTaskProviderCorrelations(tx gdb.TX, taskNos []string, bindings map[string][]resourcecap.SecretBinding) (map[string]*resourcecap.ProviderCorrelation, error) {
	result := make(map[string]*resourcecap.ProviderCorrelation)
	if len(taskNos) == 0 {
		return result, nil
	}
	var rows []struct {
		TaskNo, DownstreamURL, TokenSecretRef string
		TaskID                                int64 `orm:"task_id"`
	}
	if err := tx.Model("plugin_idc_resource_provider_correlation").Safe().
		Fields("task_no,task_id,downstream_url,token_secret_ref").Where("tenant_id", idccap.DefaultTenantID).
		WhereIn("task_no", taskNos).Scan(&rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		value := &resourcecap.ProviderCorrelation{DownstreamID: row.TaskID, DownstreamURL: row.DownstreamURL}
		matched := false
		for _, binding := range bindings[row.TaskNo] {
			matched = matched || (binding.Name == "downstream_token" && binding.Handle.OpaqueRef() == row.TokenSecretRef)
		}
		if resourcecap.ValidateProviderCorrelation(value) != nil || !matched {
			return nil, gerror.New("persisted resource task Provider correlation is invalid")
		}
		result[row.TaskNo] = value
	}
	return result, nil
}

// resourceDatabaseTime uses PostgreSQL as the single clock for availability,
// leases, and expiry convergence. Application-node clock skew cannot change
// whether a task is claimable or an in-flight write is considered abandoned.
func resourceDatabaseTime(tx gdb.TX) (time.Time, error) {
	value, err := tx.GetValue("SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "read resource task database time")
	}
	now := value.Time()
	if now.IsZero() {
		return time.Time{}, gerror.New("resource task database time is unavailable")
	}
	return now.UTC(), nil
}

// validateClaimInput enforces bounded worker identity, batch size, and lease duration.
func validateClaimInput(input ClaimInput) error {
	if !workerIDPattern.MatchString(input.WorkerID) {
		return gerror.New("resource task worker identity is invalid")
	}
	if input.Limit <= 0 || input.Limit > maximumClaimBatchSize {
		return gerror.New("resource task claim limit is invalid")
	}
	if input.LeaseDuration < minimumLeaseDuration || input.LeaseDuration > maximumLeaseDuration {
		return gerror.New("resource task lease duration is invalid")
	}
	return nil
}

// freezeExpiredWrites moves interrupted external writes behind the result-unknown replay fence.
func freezeExpiredWrites(ctx context.Context, tx gdb.TX, now time.Time, limit int) error {
	columns := dao.Task.Columns()
	var rows []entity.Task
	err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
		Fields(columns.Id, columns.TaskNo).
		Where(do.Task{TenantId: idccap.DefaultTenantID, StatusCode: StatusSubmitting}).
		WhereNotNull(columns.LeaseExpiresAt).
		WhereLTE(columns.LeaseExpiresAt, now).
		OrderAsc(columns.LeaseExpiresAt).
		OrderAsc(columns.Id).
		Limit(limit).
		LockUpdateSkipLocked().
		Scan(&rows)
	if err != nil {
		return gerror.Wrap(err, "load expired resource writes")
	}
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, len(rows))
	taskNos := make([]string, len(rows))
	for index := range rows {
		ids[index] = rows[index].Id
		taskNos[index] = rows[index].TaskNo
	}
	result, err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
		Where(do.Task{TenantId: idccap.DefaultTenantID, StatusCode: StatusSubmitting}).
		WhereIn(columns.Id, ids).
		Data(do.Task{
			StatusCode:     StatusResultUnknown,
			ResultUnknown:  true,
			LockedBy:       "",
			FailureCode:    expiredWriteCode,
			FailureSummary: expiredWriteSummary,
			Version:        gdb.Raw(columns.Version + " + 1"),
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "freeze expired resource writes")
	}
	if err = requireAffected(result, len(rows), "freeze expired resource writes"); err != nil {
		return err
	}
	if err = clearTaskLeaseExpiries(ctx, tx, ids); err != nil {
		return err
	}
	return revokeTaskInputBindings(ctx, tx, taskNos, now)
}

// loadClaimableTasks locks one deterministic bounded write batch without waiting on peers.
func loadClaimableTasks(
	ctx context.Context,
	tx gdb.TX,
	now time.Time,
	limit int,
) ([]entity.Task, error) {
	columns := dao.Task.Columns()
	var rows []entity.Task
	err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
		Fields(
			columns.Id,
			columns.TaskNo,
			columns.RequestIdDigest,
			columns.CapabilityCode,
			columns.LocalResourceNo,
			columns.ChannelNo,
			columns.PoolNo,
			columns.SanitizedParameters,
			columns.OwnerMetadata,
			columns.AttemptCount,
			columns.MaxAttempts,
		).
		Where(do.Task{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.StatusCode, []Status{StatusPending, StatusRetryWait}).
		WhereLTE(columns.AvailableAt, now).
		WhereLT(columns.AttemptCount, gdb.Raw(columns.MaxAttempts)).
		Where(`NOT EXISTS (
SELECT 1
FROM plugin_idc_resource_channel_health_state AS health
WHERE health.tenant_id = plugin_idc_resource_task.tenant_id
  AND health.channel_no = plugin_idc_resource_task.channel_no
  AND health.circuit_open = TRUE
  AND health.write_protected_until > ?
)`, now).
		OrderAsc(columns.Priority).
		OrderAsc(columns.AvailableAt).
		OrderAsc(columns.Id).
		Limit(limit).
		LockUpdateSkipLocked().
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "load claimable resource tasks")
	}
	return rows, nil
}

// rejectInvalidClaimableTasks fails corrupted persisted requests before any Provider call.
func rejectInvalidClaimableTasks(
	ctx context.Context,
	tx gdb.TX,
	rows []entity.Task,
	now time.Time,
) ([]entity.Task, error) {
	valid := make([]entity.Task, 0, len(rows))
	invalidIDs := make([]int64, 0)
	invalidTaskNos := make([]string, 0)
	for index := range rows {
		request := resourcecap.OperationRequest{
			RequestID:   rows[index].TaskNo,
			Capability:  resourcecap.CapabilityCode(rows[index].CapabilityCode),
			ResourceRef: rows[index].LocalResourceNo,
			Parameters:  []byte(rows[index].SanitizedParameters),
		}
		if request.Validate() != nil || resourcecap.ValidateChannelRef(rows[index].ChannelNo) != nil {
			invalidIDs = append(invalidIDs, rows[index].Id)
			invalidTaskNos = append(invalidTaskNos, rows[index].TaskNo)
			continue
		}
		valid = append(valid, rows[index])
	}
	if len(invalidIDs) == 0 {
		return valid, nil
	}
	columns := dao.Task.Columns()
	result, err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
		Where(do.Task{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.Id, invalidIDs).
		WhereIn(columns.StatusCode, []Status{StatusPending, StatusRetryWait}).
		Data(do.Task{
			StatusCode:     StatusFailed,
			LockedBy:       "",
			FailureCode:    invalidTaskCode,
			FailureSummary: invalidTaskSummary,
			FinishedAt:     &now,
			Version:        gdb.Raw(columns.Version + " + 1"),
		}).Update()
	if err != nil {
		return nil, gerror.Wrap(err, "reject invalid resource tasks")
	}
	if err = requireAffected(result, len(invalidIDs), "reject invalid resource tasks"); err != nil {
		return nil, err
	}
	if err = revokeTaskInputBindings(ctx, tx, invalidTaskNos, now); err != nil {
		return nil, err
	}
	return valid, nil
}

// newLeaseToken creates one unpredictable worker-scoped token for a claimed batch.
func newLeaseToken(random io.Reader, workerID string) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate resource task lease")
	}
	return workerID + ":" + hex.EncodeToString(content[:]), nil
}

// requireAffected verifies that every row protected by an ownership fence was updated.
func requireAffected(result sql.Result, expected int, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read "+operation+" result")
	}
	if affected != int64(expected) {
		return gerror.Newf("%s affected %d rows, expected %d", operation, affected, expected)
	}
	return nil
}
