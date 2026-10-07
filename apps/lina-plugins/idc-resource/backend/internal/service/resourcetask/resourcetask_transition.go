// This file persists lease-fenced Provider submission outcomes.

package resourcetask

import (
	"context"
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

const maximumTransitionScheduleDelay = 24 * time.Hour

var leaseTokenPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}:[0-9a-f]{32}$`)

// LeaseRef identifies one exact claimed attempt. It is not a user session token.
type LeaseRef struct {
	TaskNo    string
	Token     string
	AttemptNo int
}

// TransitionInput records a submission outcome while the exact lease is live.
type TransitionInput struct {
	Lease         LeaseRef
	To            Status
	Evidence      TransitionEvidence
	ResourceRef   string
	ProviderFacts []resourcecap.Evidence
	FailureCode   string
	FailureText   string
	NextAt        *time.Time
	Upstream      *UpstreamTask
	SecretOutputs []resourcecap.SecretOutput
}

// UpstreamTask is the exact external task identity accepted or observed while
// resolving one submission. It contains no raw response or credential data.
type UpstreamTask struct {
	TaskRef     string
	ResourceRef string
	Status      resourcecap.TaskStatus
	Progress    *int
}

// Transition applies one state change only while task, attempt, token, state,
// and database-time lease validity all match. Late workers cannot overwrite a
// task already frozen or reconciled by another owner process.
func (s *Store) Transition(ctx context.Context, input TransitionInput) error {
	if s == nil || s.database == nil || s.random == nil || ctx == nil || ctx.Err() != nil {
		return gerror.New("resource task store is unavailable")
	}
	if err := validateLeaseRef(input.Lease); err != nil {
		return err
	}
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := resourceDatabaseTime(tx)
		if err != nil {
			return err
		}
		row, err := loadLiveSubmittingTask(ctx, tx, input.Lease, now)
		if err != nil {
			return err
		}
		if row == nil {
			return gerror.New("resource task lease is unavailable")
		}
		if err = ValidateTransition(Status(row.StatusCode), input.To, input.Evidence); err != nil {
			return err
		}
		if err = validateTransitionDetails(input, row, now); err != nil {
			return err
		}
		if input.To == StatusSucceeded {
			if err = convergeSuccessfulProvision(ctx, tx, row, input.ResourceRef, s.random); err != nil {
				return err
			}
		}
		if input.To != StatusRetryWait {
			if err = revokeTaskInputBindings(ctx, tx, []string{row.TaskNo}, now); err != nil {
				return err
			}
		}
		if err = insertTaskOutputBindings(ctx, tx, row, input.SecretOutputs, s.random); err != nil {
			return err
		}
		if input.Upstream != nil {
			if err = insertUpstreamTaskBinding(ctx, tx, row, input.Upstream, now, s.random); err != nil {
				return err
			}
		}
		source := taskEvidenceSourceProviderResource
		if input.Upstream != nil {
			source = taskEvidenceSourceProviderTask
		}
		if err = insertTaskEvidenceBatch(ctx, tx, row.TaskNo, source, input.ProviderFacts, s.random); err != nil {
			return err
		}
		return persistLeasedTransition(ctx, tx, input, row, now)
	})
}

// validateLeaseRef verifies the exact claimed write attempt identity.
func validateLeaseRef(lease LeaseRef) error {
	if !workerIDPattern.MatchString(lease.TaskNo) || !leaseTokenPattern.MatchString(lease.Token) ||
		lease.AttemptNo <= 0 {
		return gerror.New("resource task lease reference is invalid")
	}
	return nil
}

// loadLiveSubmittingTask locks the exact unexpired write lease before transition.
func loadLiveSubmittingTask(
	ctx context.Context,
	tx gdb.TX,
	lease LeaseRef,
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
			columns.AttemptCount,
			columns.MaxAttempts,
		).
		Where(do.Task{
			TenantId: idccap.DefaultTenantID, TaskNo: lease.TaskNo,
			StatusCode: StatusSubmitting, LockedBy: lease.Token, AttemptCount: lease.AttemptNo,
		}).
		WhereNotNull(columns.LeaseExpiresAt).
		WhereGT(columns.LeaseExpiresAt, now).
		LockUpdate().
		Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "load live resource task lease")
	}
	if row.TaskNo == "" {
		return nil, nil
	}
	return &row, nil
}

// validateTransitionDetails enforces failure, schedule, retry, and upstream identity semantics.
func validateTransitionDetails(input TransitionInput, row *entity.Task, now time.Time) error {
	requiresFailure := input.To == StatusRetryWait || input.To == StatusFailed ||
		input.To == StatusResultUnknown
	if requiresFailure {
		failure := resourcecap.ProviderFailure{
			Code:          input.FailureCode,
			Message:       input.FailureText,
			Retryable:     input.To == StatusRetryWait,
			ResultUnknown: input.To == StatusResultUnknown,
		}
		if err := failure.Validate(); err != nil {
			return gerror.New("resource task failure details are invalid")
		}
	} else if input.FailureCode != "" || input.FailureText != "" {
		return gerror.New("resource task transition does not accept failure details")
	}

	requiresSchedule := input.To == StatusRetryWait || input.To == StatusAccepted ||
		input.To == StatusRunning || input.To == StatusResultUnknown
	if requiresSchedule {
		if input.NextAt == nil {
			return gerror.New("resource task transition schedule is required")
		}
		next := input.NextAt.UTC()
		if !next.After(now) || next.After(now.Add(maximumTransitionScheduleDelay)) {
			return gerror.New("resource task transition schedule is invalid")
		}
	} else if input.NextAt != nil {
		return gerror.New("resource task transition does not accept a schedule")
	}
	if input.To == StatusRetryWait && row.AttemptCount >= row.MaxAttempts {
		return gerror.New("resource task retry attempts are exhausted")
	}
	if err := validateUpstreamTask(input); err != nil {
		return err
	}
	if err := validateTransitionResourceOutcome(input, row); err != nil {
		return err
	}
	return validateTaskSecretOutputs(input, row)
}

// validateTransitionResourceOutcome verifies the authoritative resource
// identity and bounded Provider facts before they reach the owner transaction.
func validateTransitionResourceOutcome(input TransitionInput, row *entity.Task) error {
	if input.ResourceRef != "" &&
		(resourcecap.ResourceQuery{ExternalResourceRef: input.ResourceRef}).Validate() != nil {
		return gerror.New("resource task result resource identity is invalid")
	}
	if input.Upstream != nil && input.ResourceRef != "" && input.Upstream.ResourceRef != "" &&
		input.ResourceRef != input.Upstream.ResourceRef {
		return gerror.New("resource task result resource identities do not match")
	}
	if input.To == StatusSucceeded &&
		resourcecap.CapabilityCode(row.CapabilityCode) == resourcecap.CapabilityProvision &&
		input.ResourceRef == "" {
		return gerror.New("successful provision resource identity is required")
	}
	if len(input.ProviderFacts) > resourcecap.MaximumEvidenceCount {
		return gerror.New("resource task Provider evidence exceeds the owner limit")
	}
	for index := range input.ProviderFacts {
		if input.ProviderFacts[index].Validate() != nil {
			return gerror.New("resource task Provider evidence is invalid")
		}
	}
	return nil
}

// validateTaskSecretOutputs rechecks owner-managed output handles at the
// persistence boundary and limits them to successful submission states.
func validateTaskSecretOutputs(input TransitionInput, row *entity.Task) error {
	if len(input.SecretOutputs) == 0 {
		return nil
	}
	if input.To != StatusAccepted && input.To != StatusRunning && input.To != StatusSucceeded {
		return gerror.New("resource task transition does not accept confidential outputs")
	}
	if len(input.SecretOutputs) > resourcecap.MaximumSecretOutputCount {
		return gerror.New("resource task confidential outputs exceed the owner limit")
	}
	seen := make(map[string]struct{}, len(input.SecretOutputs))
	for _, output := range input.SecretOutputs {
		if _, exists := seen[output.Name]; exists {
			return gerror.New("resource task confidential output is duplicated")
		}
		seen[output.Name] = struct{}{}
		request := resourcecap.SecretStoreRequest{
			RequestID:  row.TaskNo,
			Capability: resourcecap.CapabilityCode(row.CapabilityCode),
			Name:       output.Name,
			Policy:     output.Policy,
			ExpiresAt:  output.ExpiresAt,
		}
		if err := request.Validate(); err != nil || output.Validate() != nil {
			return gerror.New("resource task confidential output is invalid")
		}
	}
	return nil
}

// insertTaskOutputBindings atomically attaches encrypted output handles to
// the task transition. Plaintext never crosses this persistence boundary.
func insertTaskOutputBindings(
	ctx context.Context,
	tx gdb.TX,
	row *entity.Task,
	outputs []resourcecap.SecretOutput,
	random io.Reader,
) error {
	for _, output := range outputs {
		bindingNo, err := newBindingNumber(random)
		if err != nil {
			return err
		}
		data := do.TaskSecretBinding{
			TenantId:         idccap.DefaultTenantID,
			BindingNo:        bindingNo,
			TaskNo:           row.TaskNo,
			BindingName:      output.Name,
			BindingDirection: "output",
			SecretRef:        output.Handle.OpaqueRef(),
			OutputPolicy:     output.Policy,
		}
		if !output.ExpiresAt.IsZero() {
			expiresAt := output.ExpiresAt.UTC()
			data.ExpiresAt = &expiresAt
		}
		if _, err = tx.Model(dao.TaskSecretBinding.Table()).Safe().Ctx(ctx).Data(data).Insert(); err != nil {
			return gerror.Wrap(err, "insert resource task confidential output")
		}
	}
	return nil
}

// validateUpstreamTask proves that normalized upstream state matches the requested transition.
func validateUpstreamTask(input TransitionInput) error {
	if input.Upstream == nil {
		if input.To == StatusAccepted || input.To == StatusRunning || input.To == StatusCanceled ||
			(input.To == StatusFailed && input.Evidence.ObservationProven) {
			return gerror.New("resource task upstream identity is required")
		}
		return nil
	}
	if input.To == StatusRetryWait ||
		(input.To == StatusFailed && input.Evidence.NonAcceptanceProven) {
		return gerror.New("resource task transition does not accept an upstream identity")
	}
	query := resourcecap.TaskQuery{
		ExternalTaskRef: input.Upstream.TaskRef,
		ResourceRef:     input.Upstream.ResourceRef,
	}
	if err := query.Validate(); err != nil {
		return gerror.New("resource task upstream identity is invalid")
	}
	if input.Upstream.Progress != nil &&
		(*input.Upstream.Progress < 0 || *input.Upstream.Progress > 100) {
		return gerror.New("resource task upstream progress is invalid")
	}
	allowed := false
	switch input.To {
	case StatusAccepted:
		allowed = input.Upstream.Status == resourcecap.TaskStatusAccepted
	case StatusRunning:
		allowed = input.Upstream.Status == resourcecap.TaskStatusRunning
	case StatusSucceeded:
		allowed = input.Upstream.Status == resourcecap.TaskStatusSucceeded
	case StatusFailed:
		allowed = input.Upstream.Status == resourcecap.TaskStatusFailed ||
			input.Upstream.Status == resourcecap.TaskStatusTerminated
	case StatusResultUnknown:
		allowed = input.Upstream.Status == resourcecap.TaskStatusUnknown
	case StatusCanceled:
		allowed = input.Upstream.Status == resourcecap.TaskStatusCanceled
	}
	if !allowed {
		return gerror.New("resource task upstream status does not match the transition")
	}
	return nil
}

// insertUpstreamTaskBinding records the first accepted upstream task in the local transaction.
func insertUpstreamTaskBinding(
	ctx context.Context,
	tx gdb.TX,
	row *entity.Task,
	upstream *UpstreamTask,
	now time.Time,
	random io.Reader,
) error {
	bindingNo, err := newUpstreamBindingNumber(random)
	if err != nil {
		return err
	}
	_, err = tx.Model(dao.UpstreamTaskBinding.Table()).Safe().Ctx(ctx).
		Data(do.UpstreamTaskBinding{
			TenantId:            idccap.DefaultTenantID,
			BindingNo:           bindingNo,
			TaskNo:              row.TaskNo,
			StepNo:              1,
			ChannelNo:           row.ChannelNo,
			ExternalTaskRef:     upstream.TaskRef,
			ExternalResourceRef: upstream.ResourceRef,
			StatusCode:          upstream.Status,
			LastProgress:        upstream.Progress,
			LastObservedAt:      &now,
		}).Insert()
	if err != nil {
		return gerror.Wrap(err, "insert upstream resource task binding")
	}
	return nil
}

// newUpstreamBindingNumber creates one unpredictable upstream binding number.
func newUpstreamBindingNumber(random io.Reader) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate upstream resource task binding number")
	}
	return "upstream_" + hex.EncodeToString(content[:]), nil
}

// persistLeasedTransition applies a write outcome only under the exact live lease fence.
func persistLeasedTransition(
	ctx context.Context,
	tx gdb.TX,
	input TransitionInput,
	row *entity.Task,
	now time.Time,
) error {
	columns := dao.Task.Columns()
	data := do.Task{
		StatusCode:     input.To,
		LockedBy:       "",
		ResultUnknown:  input.To == StatusResultUnknown,
		FailureCode:    input.FailureCode,
		FailureSummary: input.FailureText,
		Version:        gdb.Raw(columns.Version + " + 1"),
	}
	if input.NextAt != nil {
		next := input.NextAt.UTC()
		if input.To == StatusRetryWait {
			data.AvailableAt = &next
		} else {
			data.NextObserveAt = &next
		}
	}
	if input.To == StatusAccepted || input.To == StatusRunning || input.To == StatusSucceeded {
		data.AcceptedAt = &now
	}
	if input.To == StatusSucceeded || input.To == StatusFailed || input.To == StatusCanceled {
		data.FinishedAt = &now
	}
	result, err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
		Where(do.Task{
			TenantId: idccap.DefaultTenantID, Id: row.Id, TaskNo: input.Lease.TaskNo,
			StatusCode: StatusSubmitting, LockedBy: input.Lease.Token,
			AttemptCount: input.Lease.AttemptNo,
		}).
		Data(data).
		Update()
	if err != nil {
		return gerror.Wrap(err, "transition leased resource task")
	}
	if err = requireAffected(result, 1, "transition leased resource task"); err != nil {
		return err
	}
	return clearTaskLeaseExpiries(ctx, tx, []int64{row.Id})
}
