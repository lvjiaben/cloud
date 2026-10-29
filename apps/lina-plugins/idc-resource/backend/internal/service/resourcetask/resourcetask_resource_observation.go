// This file reconciles result-unknown tasks by read-only authoritative resource observations.

package resourcetask

import (
	"context"
	"encoding/hex"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

// ClaimedResourceObservation contains one exact active mapping for a
// result-unknown task that has no upstream task reference.
type ClaimedResourceObservation struct {
	TaskNo                string
	CapabilityCode        string
	ChannelNo             string
	ConfigurationRevision string
	LocalResourceNo       string
	SanitizedParameters   string
	ExternalResourceRef   string
	LeaseToken            string
	LeaseVersion          int64
	ObservationAttempt    int
	ClaimedAt             time.Time
	LeaseExpiresAt        time.Time
}

// ResourceObservationInput applies one exact resource snapshot to the leased
// unknown task. Only idempotent state capabilities with a proven target state
// can converge through this boundary.
type ResourceObservationInput struct {
	Lease    ObservationLeaseRef
	Snapshot resourcecap.ResourceSnapshot
}

// ClaimResourceObservations leases a bounded batch of result-unknown tasks
// that lack an upstream task binding but have one exact active resource mapping.
func (s *Store) ClaimResourceObservations(
	ctx context.Context,
	input ObservationClaimInput,
) ([]ClaimedResourceObservation, error) {
	if s == nil || s.database == nil || s.random == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("resource task store is unavailable")
	}
	if err := validateObservationClaimInput(input); err != nil {
		return nil, err
	}
	claimed := make([]ClaimedResourceObservation, 0)
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := resourceDatabaseTime(tx)
		if err != nil {
			return err
		}
		rows, err := loadDueResourceObservationTasks(ctx, tx, now, input.Limit)
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
			Where(do.Task{TenantId: idccap.DefaultTenantID, StatusCode: StatusResultUnknown}).
			WhereIn(columns.Id, ids).
			Data(do.Task{
				LockedBy:             leaseToken,
				LeaseExpiresAt:       &leaseExpiresAt,
				ObservationClaimedAt: &now,
				ObservationAttemptCount: gdb.Raw(
					columns.ObservationAttemptCount + " + 1",
				),
				Version: gdb.Raw(columns.Version + " + 1"),
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "lease resource snapshot observation batch")
		}
		if err = requireAffected(result, len(rows), "lease resource snapshot observation batch"); err != nil {
			return err
		}
		claimed = make([]ClaimedResourceObservation, 0, len(rows))
		for index := range rows {
			query := resourcecap.ResourceQuery{ExternalResourceRef: rows[index].ExternalResourceRef}
			if query.Validate() != nil {
				return gerror.New("resource snapshot observation mapping is invalid")
			}
			claimed = append(claimed, ClaimedResourceObservation{
				TaskNo:                rows[index].TaskNo,
				CapabilityCode:        rows[index].CapabilityCode,
				ChannelNo:             rows[index].ChannelNo,
				ConfigurationRevision: taskConfigurationRevision(rows[index].OwnerMetadata),
				LocalResourceNo:       rows[index].LocalResourceNo,
				SanitizedParameters:   rows[index].SanitizedParameters,
				ExternalResourceRef:   rows[index].ExternalResourceRef,
				LeaseToken:            leaseToken,
				LeaseVersion:          rows[index].Version + 1,
				ObservationAttempt:    rows[index].ObservationAttemptCount + 1,
				ClaimedAt:             now,
				LeaseExpiresAt:        leaseExpiresAt,
			})
		}
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "claim resource snapshot observations")
	}
	return claimed, nil
}

// ObserveResource closes an unknown task only when the exact active mapping
// and a capability-specific target state are both authoritatively observed.
func (s *Store) ObserveResource(ctx context.Context, input ResourceObservationInput) error {
	if s == nil || s.database == nil || s.random == nil || ctx == nil || ctx.Err() != nil {
		return gerror.New("resource task store is unavailable")
	}
	if err := validateObservationLeaseRef(input.Lease); err != nil || input.Snapshot.Validate() != nil {
		return gerror.New("resource snapshot observation is invalid")
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
		if row == nil || Status(row.StatusCode) != StatusResultUnknown ||
			!resourceObservationConverges(
				resourcecap.CapabilityCode(row.CapabilityCode),
				row.SanitizedParameters,
				input.Snapshot,
			) {
			return gerror.New("resource snapshot does not resolve the unknown task")
		}
		if row.ObservationClaimedAt == nil ||
			input.Snapshot.ObservedAt.Before(row.ObservationClaimedAt.UTC()) {
			return gerror.New("resource snapshot predates the observation lease")
		}
		mapping, err := loadExactActiveMapping(ctx, tx, row)
		if err != nil {
			return err
		}
		if mapping == nil || mapping.ExternalResourceRef != input.Snapshot.ExternalResourceRef {
			return gerror.New("resource snapshot identity does not match the active mapping")
		}
		if err = ValidateTransition(StatusResultUnknown, StatusSucceeded, TransitionEvidence{
			ObservationProven: true,
		}); err != nil {
			return err
		}
		if err = insertResourceObservationEvidence(ctx, tx, row, input.Snapshot, s.random); err != nil {
			return err
		}
		columns := dao.Task.Columns()
		result, err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
			Where(do.Task{
				TenantId: idccap.DefaultTenantID, Id: row.Id, TaskNo: row.TaskNo,
				StatusCode: StatusResultUnknown, LockedBy: input.Lease.Token, Version: input.Lease.Version,
			}).
			WhereNotNull(columns.LeaseExpiresAt).
			WhereGT(columns.LeaseExpiresAt, now).
			Data(do.Task{
				StatusCode: StatusSucceeded, LockedBy: "", ResultUnknown: false,
				FailureCode: "", FailureSummary: "", FinishedAt: &now,
				ObservationAttemptCount: 0,
				Version:                 gdb.Raw(columns.Version + " + 1"),
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "resolve unknown task from resource snapshot")
		}
		if err = requireAffected(result, 1, "resolve unknown task from resource snapshot"); err != nil {
			return err
		}
		if err = clearTaskLeaseExpiry(ctx, tx, row.Id); err != nil {
			return err
		}
		return clearTaskNextObservation(ctx, tx, row.Id)
	})
}

// taskEvidenceSource classifies the owner-controlled origin of one immutable
// reconciliation fact without exposing a raw Provider payload.
type taskEvidenceSource string

const (
	// taskEvidenceSourceProviderTask identifies validated upstream task-query evidence.
	taskEvidenceSourceProviderTask taskEvidenceSource = "provider_task"
	// taskEvidenceSourceProviderResource identifies validated Provider snapshot evidence.
	taskEvidenceSourceProviderResource taskEvidenceSource = "provider_resource"
	// taskEvidenceSourceOwnerState identifies the owner's exact target-state proof.
	taskEvidenceSourceOwnerState taskEvidenceSource = "owner_resource_state"
	// taskEvidenceKindOwnerState is the stable evidence type for owner state convergence.
	taskEvidenceKindOwnerState = "resourceState"
)

// insertResourceObservationEvidence appends bounded owner and Provider facts
// inside the same transaction that closes the task.
func insertResourceObservationEvidence(
	ctx context.Context,
	tx gdb.TX,
	task *entity.Task,
	snapshot resourcecap.ResourceSnapshot,
	random io.Reader,
) error {
	if err := insertTaskEvidence(ctx, tx, task.TaskNo, taskEvidenceSourceOwnerState, resourcecap.Evidence{
		Kind:       taskEvidenceKindOwnerState,
		Reference:  snapshot.ExternalResourceRef,
		Summary:    string(snapshot.State),
		ObservedAt: snapshot.ObservedAt,
	}, random); err != nil {
		return err
	}
	return insertTaskEvidenceBatch(
		ctx, tx, task.TaskNo, taskEvidenceSourceProviderResource, snapshot.Evidence, random,
	)
}

// insertTaskEvidenceBatch persists one fixed-size validated evidence collection.
func insertTaskEvidenceBatch(
	ctx context.Context,
	tx gdb.TX,
	taskNo string,
	source taskEvidenceSource,
	evidence []resourcecap.Evidence,
	random io.Reader,
) error {
	if len(evidence) > resourcecap.MaximumEvidenceCount {
		return gerror.New("resource task evidence batch is invalid")
	}
	for index := range evidence {
		if evidence[index].Validate() != nil {
			return gerror.New("resource task evidence is invalid")
		}
		if err := insertTaskEvidence(ctx, tx, taskNo, source, evidence[index], random); err != nil {
			return err
		}
	}
	return nil
}

// insertTaskEvidence persists one already validated, secret-free evidence item.
func insertTaskEvidence(
	ctx context.Context,
	tx gdb.TX,
	taskNo string,
	source taskEvidenceSource,
	evidence resourcecap.Evidence,
	random io.Reader,
) error {
	evidenceNo, err := newTaskEvidenceNumber(random)
	if err != nil {
		return err
	}
	_, err = tx.Model(dao.TaskEvidence.Table()).Safe().Ctx(ctx).Data(do.TaskEvidence{
		TenantId:     idccap.DefaultTenantID,
		EvidenceNo:   evidenceNo,
		TaskNo:       taskNo,
		SourceKind:   source,
		EvidenceKind: evidence.Kind,
		Reference:    evidence.Reference,
		Summary:      evidence.Summary,
		ObservedAt:   &evidence.ObservedAt,
	}).Insert()
	if err != nil {
		return gerror.Wrap(err, "persist resource task reconciliation evidence")
	}
	return nil
}

// newTaskEvidenceNumber creates an unpredictable immutable evidence number.
func newTaskEvidenceNumber(random io.Reader) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate resource task evidence number")
	}
	return "evidence_" + hex.EncodeToString(content[:]), nil
}

// resourceObservationRow joins the task projection with its exact external resource reference.
type resourceObservationRow struct {
	entity.Task
	ExternalResourceRef string `orm:"external_resource_ref"`
}

// loadDueResourceObservationTasks locks one bounded due batch whose tasks have
// no upstream task binding and do have an exact active resource mapping.
func loadDueResourceObservationTasks(
	ctx context.Context,
	tx gdb.TX,
	now time.Time,
	limit int,
) ([]resourceObservationRow, error) {
	taskColumns := dao.Task.Columns()
	mappingColumns := dao.Mapping.Columns()
	bindingColumns := dao.UpstreamTaskBinding.Columns()
	model := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).As("task")
	leaseAvailable := model.Builder().
		WhereNull("task."+taskColumns.LeaseExpiresAt).
		WhereOrLTE("task."+taskColumns.LeaseExpiresAt, now)
	bindingExists := tx.Model(dao.UpstreamTaskBinding.Table()).Safe().Ctx(ctx).As("binding").
		Fields("binding." + bindingColumns.Id).
		Where(do.UpstreamTaskBinding{TenantId: idccap.DefaultTenantID}).
		Where("binding." + bindingColumns.TaskNo + " = task." + taskColumns.TaskNo)
	rows := make([]resourceObservationRow, 0, limit)
	err := model.
		InnerJoin(
			dao.Mapping.Table(),
			"mapping",
			"mapping."+mappingColumns.TenantId+" = task."+taskColumns.TenantId+
				" AND mapping."+mappingColumns.LocalResourceNo+" = task."+taskColumns.LocalResourceNo+
				" AND mapping."+mappingColumns.ChannelNo+" = task."+taskColumns.ChannelNo+
				" AND mapping."+mappingColumns.PoolNo+" = task."+taskColumns.PoolNo,
		).
		Fields(
			"task."+taskColumns.Id,
			"task."+taskColumns.TaskNo,
			"task."+taskColumns.CapabilityCode,
			"task."+taskColumns.LocalResourceNo,
			"task."+taskColumns.ChannelNo,
			"task."+taskColumns.SanitizedParameters,
			"task."+taskColumns.OwnerMetadata,
			"task."+taskColumns.Version,
			"task."+taskColumns.ObservationAttemptCount,
			"mapping."+mappingColumns.ExternalResourceRef,
		).
		Where("task."+taskColumns.TenantId, idccap.DefaultTenantID).
		Where("task."+taskColumns.StatusCode, StatusResultUnknown).
		Where("mapping."+mappingColumns.StatusCode, mappingStatusActive).
		WhereNotNull("task."+taskColumns.NextObserveAt).
		WhereLTE("task."+taskColumns.NextObserveAt, now).
		Where(leaseAvailable).
		WhereNotExists(bindingExists).
		OrderAsc("task." + taskColumns.NextObserveAt).
		OrderAsc("task." + taskColumns.Priority).
		OrderAsc("task." + taskColumns.Id).
		Limit(limit).
		LockUpdateSkipLocked().
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "load due resource snapshot observations")
	}
	return rows, nil
}

// loadExactActiveMapping locks the mapping matching all owner identity dimensions.
func loadExactActiveMapping(
	ctx context.Context,
	tx gdb.TX,
	task *entity.Task,
) (*entity.Mapping, error) {
	columns := dao.Mapping.Columns()
	var mapping entity.Mapping
	err := tx.Model(dao.Mapping.Table()).Safe().Ctx(ctx).
		Fields(
			columns.Id,
			columns.LocalResourceNo,
			columns.ChannelNo,
			columns.PoolNo,
			columns.ExternalResourceRef,
			columns.StatusCode,
		).
		Where(do.Mapping{
			TenantId: idccap.DefaultTenantID, LocalResourceNo: task.LocalResourceNo,
			ChannelNo: task.ChannelNo, PoolNo: task.PoolNo, StatusCode: mappingStatusActive,
		}).
		LockUpdate().
		Scan(&mapping)
	if err != nil {
		return nil, gerror.Wrap(err, "load exact active resource mapping")
	}
	if mapping.LocalResourceNo == "" {
		return nil, nil
	}
	return &mapping, nil
}

// resourceObservationConverges reports whether a generic resource state is
// sufficient to prove the idempotent outcome of the requested capability.
func resourceObservationConverges(
	capability resourcecap.CapabilityCode,
	_ string,
	snapshot resourcecap.ResourceSnapshot,
) bool {
	switch capability {
	case resourcecap.CapabilityStart, resourcecap.CapabilityResume:
		return snapshot.State == resourcecap.ResourceStateRunning ||
			snapshot.State == resourcecap.ResourceStateActive
	case resourcecap.CapabilityStop:
		return snapshot.State == resourcecap.ResourceStateStopped
	case resourcecap.CapabilitySuspend:
		return snapshot.State == resourcecap.ResourceStateSuspended
	case resourcecap.CapabilityTerminate:
		return snapshot.State == resourcecap.ResourceStateTerminated
	case resourcecap.CapabilityChangeIP:
		// Replacement preserves the requested count. The same count cannot prove
		// a changed address set or associate the change with this task. Only the
		// exact supplier task outcome may finish this non-idempotent operation.
		return false
	default:
		return false
	}
}
