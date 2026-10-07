// This file processes bounded lifecycle tasks and owns local or Provider dispatch orchestration.

package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-core/backend/internal/service/outbox"
)

// Processor owns local state convergence and the versioned resource action
// boundary. Scanner and processor remain separate so discovery has no effects.
type Processor interface {
	// RunTasks processes one bounded due-task batch and reports persisted outcomes.
	RunTasks(ctx context.Context, limit int) (*ProcessBatch, error)
	// ProcessTask idempotently advances one immutable lifecycle task.
	ProcessTask(ctx context.Context, taskNo string) error
}

// ProcessBatch summarizes one bounded lifecycle processing invocation.
type ProcessBatch struct {
	Selected  int
	Processed int
	Failures  int
}

type processor struct {
	database gdb.DB
	resource lifecycleresourcecap.Source
	outbox   outbox.Enqueuer
}

type taskState struct {
	TaskNo                        string             `orm:"task_no"`
	ServiceNo                     string             `orm:"service_no"`
	CustomerNo                    string             `orm:"customer_no"`
	CurrencyCode                  string             `orm:"currency_code"`
	PolicyNo                      string             `orm:"policy_no"`
	PolicyVersion                 int64              `orm:"policy_version"`
	ActionCode                    actionCode         `orm:"action_code"`
	ReminderSequence              *int               `orm:"reminder_sequence"`
	ExpectedServiceVersion        int64              `orm:"expected_service_version"`
	ExpectedEntitlementExpiresAt  time.Time          `orm:"expected_entitlement_expires_at"`
	DueAt                         time.Time          `orm:"due_at"`
	StatusCode                    taskStatus         `orm:"status_code"`
	BusinessOperationNo           *string            `orm:"business_operation_no"`
	PreviousRuntimeStatusCode     *runtimeStatus     `orm:"previous_runtime_status_code"`
	IdempotencyDigest             string             `orm:"idempotency_digest"`
	TaskVersion                   int64              `orm:"task_version"`
	ServiceVersion                int64              `orm:"service_version"`
	EntitlementExpiresAt          *time.Time         `orm:"entitlement_expires_at"`
	EntitlementStatusCode         entitlementStatus  `orm:"entitlement_status_code"`
	RuntimeStatusCode             runtimeStatus      `orm:"runtime_status_code"`
	ProviderTaskStatusCode        providerTaskStatus `orm:"provider_task_status_code"`
	LifecycleSuspended            bool               `orm:"lifecycle_suspended"`
	CurrentLifecyclePolicyNo      *string            `orm:"current_lifecycle_policy_no"`
	CurrentLifecyclePolicyVersion *int64             `orm:"current_lifecycle_policy_version"`
	ChannelNo                     *string            `orm:"channel_no"`
	PoolNo                        string             `orm:"pool_no"`
}

// NewProcessor binds authoritative storage, the resource owner, and the shared Outbox writer.
func NewProcessor(
	database gdb.DB,
	resource lifecycleresourcecap.Source,
	outboxWriter outbox.Enqueuer,
) (Processor, error) {
	if database == nil || resource == nil || outboxWriter == nil {
		return nil, gerror.New("service lifecycle processor dependencies are unavailable")
	}
	return &processor{database: database, resource: resource, outbox: outboxWriter}, nil
}

func (p *processor) RunTasks(ctx context.Context, limit int) (*ProcessBatch, error) {
	if p == nil || p.database == nil || p.resource == nil || ctx == nil || ctx.Err() != nil ||
		limit <= 0 || limit > MaximumScheduleBatch {
		return nil, gerror.New("invalid service lifecycle processing request")
	}
	rows := make([]struct {
		TaskNo string `orm:"task_no"`
	}, 0, limit)
	if err := p.database.GetScan(ctx, &rows, `
SELECT task.task_no
FROM plugin_idc_core_service_lifecycle_task task
WHERE task.tenant_id = 0
  AND task.status_code IN ('pending', 'claimed', 'dispatched')
  AND task.due_at <= CURRENT_TIMESTAMP
ORDER BY task.due_at ASC, task.id ASC
LIMIT ?`, limit); err != nil {
		return nil, gerror.Wrap(err, "list service lifecycle work")
	}
	report := &ProcessBatch{Selected: len(rows)}
	failures := make([]error, 0)
	for _, row := range rows {
		if ctx.Err() != nil {
			return report, ctx.Err()
		}
		if err := p.ProcessTask(ctx, row.TaskNo); err != nil {
			report.Failures++
			failures = append(failures, gerror.Wrapf(err, "process lifecycle task %s", row.TaskNo))
			continue
		}
		report.Processed++
	}
	if report.Failures != 0 {
		return report, errors.Join(failures...)
	}
	return report, nil
}

func (p *processor) ProcessTask(ctx context.Context, taskNo string) error {
	if p == nil || p.database == nil || p.resource == nil || ctx == nil || ctx.Err() != nil ||
		taskNo == "" {
		return gerror.New("invalid service lifecycle task request")
	}
	state, err := loadTaskState(ctx, p.database, taskNo, false)
	if err != nil || state == nil {
		return err
	}
	switch state.StatusCode {
	case taskStatusPending:
		if !providerAction(state.ActionCode) {
			return p.convergeLocal(ctx, taskNo)
		}
		request, claimErr := p.claimProviderTask(ctx, taskNo)
		if claimErr != nil || request == nil {
			return claimErr
		}
		return p.createAndPersistProviderTask(ctx, *request)
	case taskStatusClaimed:
		request, requestErr := resourceRequest(state)
		if requestErr != nil {
			return requestErr
		}
		return p.createAndPersistProviderTask(ctx, *request)
	case taskStatusDispatched:
		return p.reconcileProviderTask(ctx, state)
	default:
		return nil
	}
}

func (p *processor) createAndPersistProviderTask(
	ctx context.Context,
	request lifecycleresourcecap.TaskRequest,
) error {
	result, err := p.resource.CreateTask(ctx, request)
	if err != nil {
		if transitionErr := p.freezeProviderUncertainty(ctx, request.LifecycleTaskNo, err); transitionErr != nil {
			return errors.Join(err, transitionErr)
		}
		if errors.Is(err, lifecycleresourcecap.ErrCapabilityUnavailable) {
			return nil
		}
		return err
	}
	if result == nil || lifecycleresourcecap.ValidateTask(*result) != nil {
		return p.freezeProviderUncertainty(ctx, request.LifecycleTaskNo,
			lifecycleresourcecap.ErrTaskConflict)
	}
	if err = p.persistProviderDispatch(ctx, request, result); err != nil {
		return err
	}
	if result.Status == lifecycleresourcecap.TaskStatusPending ||
		result.Status == lifecycleresourcecap.TaskStatusRunning {
		return nil
	}
	state, err := loadTaskState(ctx, p.database, request.LifecycleTaskNo, false)
	if err != nil {
		return err
	}
	return p.convergeProviderResult(ctx, state, result)
}

func (p *processor) reconcileProviderTask(ctx context.Context, state *taskState) error {
	if state == nil || state.BusinessOperationNo == nil {
		return gerror.New("dispatched lifecycle task is missing its business operation")
	}
	request, err := resourceRequest(state)
	if err != nil {
		return err
	}
	providerTaskNo, err := loadProviderTaskNo(ctx, p.database, *state.BusinessOperationNo)
	if err != nil {
		return err
	}
	result, err := p.resource.ReadTask(ctx, lifecycleresourcecap.TaskReadRequest{
		ProviderTaskNo: providerTaskNo, Request: *request,
	})
	if err != nil {
		return err
	}
	return p.convergeProviderResult(ctx, state, result)
}

func providerAction(action actionCode) bool {
	return action == actionSuspend || action == actionResume || action == actionTerminate
}

func resourceRequest(state *taskState) (*lifecycleresourcecap.TaskRequest, error) {
	if state == nil || state.ChannelNo == nil || *state.ChannelNo == "" || state.PoolNo == "" ||
		!providerAction(state.ActionCode) {
		return nil, gerror.New("service lifecycle task has no valid resource route")
	}
	request := &lifecycleresourcecap.TaskRequest{
		LifecycleTaskNo: state.TaskNo, ServiceNo: state.ServiceNo,
		ChannelNo: *state.ChannelNo, PoolNo: state.PoolNo,
		Action:                 lifecycleresourcecap.Action(state.ActionCode),
		ExpectedServiceVersion: state.ExpectedServiceVersion,
		ExpectedExpiresAt:      state.ExpectedEntitlementExpiresAt,
		IdempotencyDigest:      state.IdempotencyDigest,
	}
	if err := lifecycleresourcecap.ValidateTaskRequest(*request); err != nil {
		return nil, err
	}
	return request, nil
}

func loadTaskState(
	ctx context.Context,
	database gdb.DB,
	taskNo string,
	lock bool,
) (*taskState, error) {
	query := lifecycleTaskStateSQL
	if lock {
		query += " FOR UPDATE OF task, service"
	}
	var state *taskState
	if err := database.GetScan(ctx, &state, query, taskNo); err != nil {
		return nil, gerror.Wrap(err, "load service lifecycle task")
	}
	return state, nil
}

const lifecycleTaskStateSQL = `
SELECT task.task_no, task.service_no, task.customer_no, task.currency_code,
       task.policy_no, task.policy_version, task.action_code,
       task.reminder_sequence, task.expected_service_version,
       task.expected_entitlement_expires_at, task.due_at, task.status_code,
       task.business_operation_no, task.previous_runtime_status_code,
       task.idempotency_digest, task.version AS task_version,
       service.version AS service_version, service.entitlement_expires_at,
       service.entitlement_status_code, service.runtime_status_code,
       service.provider_task_status_code,
       EXISTS (
           SELECT 1
           FROM plugin_idc_core_service_lifecycle_task prior
           WHERE prior.tenant_id = service.tenant_id
             AND prior.service_no = service.service_no
             AND prior.action_code = 'suspend'
             AND prior.status_code = 'succeeded'
             AND prior.business_operation_no IS NOT NULL
             AND prior.business_operation_no = service.last_business_operation_no
       ) AS lifecycle_suspended,
       service.current_lifecycle_policy_no,
       service.current_lifecycle_policy_version, service.channel_no, service.pool_no
FROM plugin_idc_core_service_lifecycle_task task
INNER JOIN plugin_idc_core_service_instance service
    ON service.tenant_id = task.tenant_id AND service.service_no = task.service_no
   AND service.customer_no = task.customer_no AND service.currency_code = task.currency_code
WHERE task.tenant_id = 0 AND task.task_no = ?`

func lifecycleFactsCurrent(state *taskState) bool {
	return state != nil && state.EntitlementExpiresAt != nil &&
		state.EntitlementExpiresAt.Equal(state.ExpectedEntitlementExpiresAt) &&
		state.CurrentLifecyclePolicyNo != nil &&
		*state.CurrentLifecyclePolicyNo == state.PolicyNo &&
		state.CurrentLifecyclePolicyVersion != nil &&
		*state.CurrentLifecyclePolicyVersion == state.PolicyVersion &&
		state.ServiceVersion >= state.ExpectedServiceVersion
}

func encodeReminderPayload(state *taskState) (json.RawMessage, json.RawMessage, error) {
	payload, err := json.Marshal(struct {
		ServiceNo, CustomerNo, ExpiresAt string
		ReminderSequence                 int
	}{state.ServiceNo, state.CustomerNo,
		state.ExpectedEntitlementExpiresAt.UTC().Format(time.RFC3339Nano),
		*state.ReminderSequence})
	if err != nil {
		return nil, nil, gerror.Wrap(err, "encode service lifecycle reminder")
	}
	metadata, err := json.Marshal(struct {
		Source, LifecycleTaskNo string
	}{"service-lifecycle", state.TaskNo})
	if err != nil {
		return nil, nil, gerror.Wrap(err, "encode service lifecycle reminder metadata")
	}
	return payload, metadata, nil
}
