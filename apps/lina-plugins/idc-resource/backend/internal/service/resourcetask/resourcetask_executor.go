// This file coordinates bounded Provider writes and read-only task observations.

package resourcetask

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/providerhealth"
)

const (
	providerUnavailableCode        = "provider_unavailable"
	providerUnavailableSummary     = "provider construction is temporarily unavailable"
	resourceResolutionCode         = "resource_resolution_unavailable"
	resourceResolutionSummary      = "upstream resource identity is temporarily unavailable"
	invalidProviderResponseCode    = "invalid_response"
	invalidProviderResponseSummary = "provider response requires reconciliation"
	providerCallFailureSummary     = "provider write was not accepted"
	providerCallUnknownSummary     = "provider write result requires reconciliation"
)

// Service is the complete durable task boundary consumed by the executor.
// Implementations must preserve claim and transition lease fences, use a
// shared authoritative database clock, and never expose secret references in
// ordinary task projections.
type Service interface {
	// Claim leases one bounded batch of Provider writes.
	Claim(ctx context.Context, input ClaimInput) ([]ClaimedTask, error)
	// Transition persists one exact leased Provider write outcome.
	Transition(ctx context.Context, input TransitionInput) error
	// DeferWriteForProtection releases one exact lease without consuming its
	// Provider attempt because no upstream write crossed the call boundary.
	DeferWriteForProtection(ctx context.Context, lease LeaseRef, nextAt time.Time) error
	// ClaimObservations leases one bounded batch of read-only task queries.
	ClaimObservations(ctx context.Context, input ObservationClaimInput) ([]ClaimedObservation, error)
	// ObserveTask persists one validated upstream task observation.
	ObserveTask(ctx context.Context, input ObservationInput) error
	// RescheduleObservation releases an observation after a safe query failure.
	RescheduleObservation(ctx context.Context, input ObservationRescheduleInput) error
	// ClaimResourceObservations leases unknown tasks that require QueryResource.
	ClaimResourceObservations(
		ctx context.Context,
		input ObservationClaimInput,
	) ([]ClaimedResourceObservation, error)
	// ObserveResource closes one unknown task from an exact authoritative snapshot.
	ObserveResource(ctx context.Context, input ResourceObservationInput) error
}

var _ Service = (*Store)(nil)

// ProviderConstructor constructs one Provider session for the exact enabled
// channel. The executor always releases every successfully constructed session.
type ProviderConstructor func(
	ctx context.Context,
	channelRef string,
	configurationRevision string,
) (resourcecap.Provider, func(), error)

// ResourceResolutionRequest identifies one owner mapping lookup without
// exposing Provider or secret material.
type ResourceResolutionRequest struct {
	TaskNo          string
	Capability      resourcecap.CapabilityCode
	LocalResourceNo string
	ChannelNo       string
	PoolNo          string
}

// ResourceResolver resolves one claimed batch with a single bounded owner
// call. Provision entries may be absent; every other task must have one exact
// upstream reference before its Provider is called.
type ResourceResolver func(
	ctx context.Context,
	requests []ResourceResolutionRequest,
) (map[string]string, error)

// Executor coordinates durable owner tasks with injected Provider runtime seams.
type Executor struct {
	tasks     Service
	construct ProviderConstructor
	resolve   ResourceResolver
	health    providerhealth.Service
}

// BatchReport summarizes one bounded worker pass without exposing task,
// resource, Provider, or secret identifiers.
type BatchReport struct {
	Claimed             int
	Persisted           int
	PersistenceFailures int
}

// NewExecutor constructs the resource task execution coordinator. Runtime
// composition must inject the startup-owned task service, frozen Provider
// runtime constructor, and resource-owner mapping resolver.
func NewExecutor(
	tasks Service,
	construct ProviderConstructor,
	resolve ResourceResolver,
	health providerhealth.Service,
) (*Executor, error) {
	if tasks == nil || construct == nil || resolve == nil || health == nil {
		return nil, gerror.New("resource task executor dependencies are invalid")
	}
	return &Executor{tasks: tasks, construct: construct, resolve: resolve, health: health}, nil
}

// RunWrites claims and executes at most the requested fixed-size batch. A
// Provider call occurs only after the owner resolves the upstream resource;
// all untrusted call errors and malformed responses are reduced to fixed safe
// classifications before persistence.
func (e *Executor) RunWrites(ctx context.Context, input ClaimInput) (BatchReport, error) {
	if e == nil || e.tasks == nil || e.construct == nil || e.resolve == nil || e.health == nil ||
		ctx == nil || ctx.Err() != nil {
		return BatchReport{}, gerror.New("resource task executor is unavailable")
	}
	tasks, err := e.tasks.Claim(ctx, input)
	if err != nil {
		return BatchReport{}, gerror.Wrap(err, "claim resource task execution batch")
	}
	report := BatchReport{Claimed: len(tasks)}
	gates, gateErr := e.health.CheckWrites(ctx, claimedTaskChannels(tasks))
	resolutions, resolutionErr := e.resolve(ctx, resourceResolutionRequests(tasks))
	providers := make(map[string]providerConstruction, len(tasks))
	defer releaseProviderConstructions(providers)
	for index := range tasks {
		if ctx.Err() != nil {
			return report, gerror.New("resource task execution context ended")
		}
		resourceRef, resolved := resolutions[tasks[index].TaskNo]
		gate, protected := gates[tasks[index].ChannelNo]
		if gateErr != nil || (protected && !gate.AllowsWrite()) {
			nextAt := tasks[index].LeaseExpiresAt
			if gateErr == nil && gate.ProtectedUntil.After(nextAt) {
				nextAt = gate.ProtectedUntil
			}
			err = e.tasks.DeferWriteForProtection(ctx, LeaseRef{
				TaskNo: tasks[index].TaskNo, Token: tasks[index].LeaseToken,
				AttemptNo: tasks[index].AttemptNo,
			}, nextAt)
		} else if resolutionErr != nil ||
			(!resolved && resourcecap.CapabilityCode(tasks[index].CapabilityCode) != resourcecap.CapabilityProvision) {
			err = e.persistBeforeCallFailure(
				ctx,
				tasks[index],
				LeaseRef{
					TaskNo:    tasks[index].TaskNo,
					Token:     tasks[index].LeaseToken,
					AttemptNo: tasks[index].AttemptNo,
				},
				resourceResolutionCode,
				resourceResolutionSummary,
			)
		} else {
			providerKey := tasks[index].ChannelNo + "\x00" + tasks[index].ConfigurationRevision
			construction, exists := providers[providerKey]
			if !exists {
				construction.provider, construction.cleanup, construction.err =
					e.construct(ctx, tasks[index].ChannelNo, tasks[index].ConfigurationRevision)
				providers[providerKey] = construction
			}
			err = e.executeWrite(ctx, tasks[index], resourceRef, construction)
		}
		if err != nil {
			report.PersistenceFailures++
			continue
		}
		report.Persisted++
	}
	if report.PersistenceFailures != 0 {
		return report, gerror.Newf(
			"resource task execution could not persist %d outcomes",
			report.PersistenceFailures,
		)
	}
	return report, nil
}

// RunObservations claims and queries at most one fixed-size due batch. Query
// failures and invalid responses only delay the next read; they never regress
// an accepted, running, or result-unknown write into a replayable state.
func (e *Executor) RunObservations(
	ctx context.Context,
	input ObservationClaimInput,
) (BatchReport, error) {
	if e == nil || e.tasks == nil || e.construct == nil ||
		ctx == nil || ctx.Err() != nil {
		return BatchReport{}, gerror.New("resource task executor is unavailable")
	}
	observations, err := e.tasks.ClaimObservations(ctx, input)
	if err != nil {
		return BatchReport{}, gerror.Wrap(err, "claim resource task observation batch")
	}
	report := BatchReport{Claimed: len(observations)}
	providers := make(map[string]providerConstruction, len(observations))
	defer releaseProviderConstructions(providers)
	for index := range observations {
		if ctx.Err() != nil {
			return report, gerror.New("resource task observation context ended")
		}
		providerKey := observations[index].ChannelNo + "\x00" + observations[index].ConfigurationRevision
		construction, exists := providers[providerKey]
		if !exists {
			construction.provider, construction.cleanup, construction.err =
				e.construct(ctx, observations[index].ChannelNo, observations[index].ConfigurationRevision)
			providers[providerKey] = construction
		}
		if err = e.executeObservation(ctx, observations[index], construction); err != nil {
			report.PersistenceFailures++
			continue
		}
		report.Persisted++
	}
	if report.PersistenceFailures != 0 {
		return report, gerror.Newf(
			"resource task observation could not persist %d outcomes",
			report.PersistenceFailures,
		)
	}
	return report, nil
}

// RunResourceObservations queries mapped resources for result-unknown tasks
// without an upstream task reference. Non-converging or unavailable snapshots
// remain unknown and are only rescheduled for another read.
func (e *Executor) RunResourceObservations(
	ctx context.Context,
	input ObservationClaimInput,
) (BatchReport, error) {
	if e == nil || e.tasks == nil || e.construct == nil ||
		ctx == nil || ctx.Err() != nil {
		return BatchReport{}, gerror.New("resource task executor is unavailable")
	}
	observations, err := e.tasks.ClaimResourceObservations(ctx, input)
	if err != nil {
		return BatchReport{}, gerror.Wrap(err, "claim resource snapshot observation batch")
	}
	report := BatchReport{Claimed: len(observations)}
	providers := make(map[string]providerConstruction, len(observations))
	defer releaseProviderConstructions(providers)
	for index := range observations {
		if ctx.Err() != nil {
			return report, gerror.New("resource snapshot observation context ended")
		}
		providerKey := observations[index].ChannelNo + "\x00" + observations[index].ConfigurationRevision
		construction, exists := providers[providerKey]
		if !exists {
			construction.provider, construction.cleanup, construction.err =
				e.construct(ctx, observations[index].ChannelNo, observations[index].ConfigurationRevision)
			providers[providerKey] = construction
		}
		if err = e.executeResourceObservation(ctx, observations[index], construction); err != nil {
			report.PersistenceFailures++
			continue
		}
		report.Persisted++
	}
	if report.PersistenceFailures != 0 {
		return report, gerror.Newf(
			"resource snapshot observation could not persist %d outcomes",
			report.PersistenceFailures,
		)
	}
	return report, nil
}

type providerConstruction struct {
	provider resourcecap.Provider
	cleanup  func()
	err      error
}

func releaseProviderConstructions(providers map[string]providerConstruction) {
	for _, construction := range providers {
		if construction.cleanup != nil {
			construction.cleanup()
		}
	}
}

func (e *Executor) executeWrite(
	ctx context.Context,
	task ClaimedTask,
	resourceRef string,
	construction providerConstruction,
) error {
	lease := LeaseRef{TaskNo: task.TaskNo, Token: task.LeaseToken, AttemptNo: task.AttemptNo}
	capability := resourcecap.CapabilityCode(task.CapabilityCode)
	request := resourcecap.OperationRequest{
		RequestID:           task.TaskNo,
		Capability:          capability,
		ResourceRef:         resourceRef,
		Parameters:          []byte(task.SanitizedParameters),
		SecretBindings:      task.SecretBindings,
		ProviderCorrelation: task.ProviderCorrelation,
	}
	if request.Validate() != nil {
		return e.persistBeforeCallFailure(
			ctx,
			task,
			lease,
			resourceResolutionCode,
			resourceResolutionSummary,
		)
	}
	if construction.err != nil || construction.provider == nil || construction.cleanup == nil {
		return e.persistBeforeCallFailure(
			ctx,
			task,
			lease,
			providerUnavailableCode,
			providerUnavailableSummary,
		)
	}
	startedAt := time.Now()
	result, callErr := construction.provider.Execute(ctx, request)
	latency := time.Since(startedAt)
	if callErr != nil {
		failure := resourcecap.NormalizeProviderError(callErr)
		if err := e.persistCallFailure(ctx, task, lease, failure); err != nil {
			return err
		}
		return e.recordWriteHealth(
			ctx, task.ChannelNo, outcomeForCallFailure(failure), string(failure.Code), latency,
		)
	}
	if result == nil || result.ValidateFor(request) != nil ||
		resultResourceMismatch(request.ResourceRef, result) {
		if err := e.persistResultUnknown(
			ctx,
			lease,
			task.LeaseExpiresAt,
			invalidProviderResponseCode,
			invalidProviderResponseSummary,
			nil,
			"",
			nil,
		); err != nil {
			return err
		}
		return e.recordWriteHealth(
			ctx, task.ChannelNo, providerhealth.OutcomeResultUnknown, invalidProviderResponseCode,
			latency,
		)
	}
	if err := e.persistResult(ctx, task, lease, request.ResourceRef, *result); err != nil {
		return err
	}
	return e.recordWriteHealth(
		ctx, task.ChannelNo, outcomeForResult(result.Status), summaryForResult(*result), latency,
	)
}

func (e *Executor) recordWriteHealth(
	ctx context.Context,
	channelNo string,
	outcome providerhealth.Outcome,
	summaryCode string,
	latency time.Duration,
) error {
	return e.health.Record(ctx, providerhealth.Observation{
		ChannelNo: channelNo, Operation: providerhealth.OperationWrite,
		Outcome: outcome, SummaryCode: summaryCode, Latency: latency,
		AuthenticationFailure: summaryCode ==
			string(resourcecap.ProviderCallErrorCodeAuthentication),
	})
}

func outcomeForCallFailure(failure *resourcecap.ProviderCallError) providerhealth.Outcome {
	if failure == nil || failure.ResultUnknown {
		return providerhealth.OutcomeResultUnknown
	}
	return providerhealth.OutcomeFailed
}

func outcomeForResult(status resourcecap.TaskStatus) providerhealth.Outcome {
	if status == resourcecap.TaskStatusUnknown {
		return providerhealth.OutcomeResultUnknown
	}
	if status == resourcecap.TaskStatusFailed || status == resourcecap.TaskStatusCanceled ||
		status == resourcecap.TaskStatusTerminated {
		return providerhealth.OutcomeFailed
	}
	return providerhealth.OutcomeSucceeded
}

func summaryForResult(result resourcecap.OperationResult) string {
	if result.Failure != nil && result.Failure.Code != "" {
		return result.Failure.Code
	}
	return string(result.Status)
}

func (e *Executor) executeObservation(
	ctx context.Context,
	task ClaimedObservation,
	construction providerConstruction,
) error {
	lease := ObservationLeaseRef{
		TaskNo:  task.TaskNo,
		Token:   task.LeaseToken,
		Version: task.LeaseVersion,
	}
	if construction.err != nil || construction.provider == nil || construction.cleanup == nil {
		return e.rescheduleTaskObservation(ctx, task, lease)
	}
	query := resourcecap.TaskQuery{
		ExternalTaskRef: task.ExternalTaskRef,
		ResourceRef:     task.ExternalResourceRef,
	}
	startedAt := time.Now()
	result, callErr := construction.provider.QueryTask(ctx, query)
	latency := time.Since(startedAt)
	if callErr != nil {
		failure := resourcecap.NormalizeProviderError(callErr)
		if err := e.rescheduleTaskObservation(ctx, task, lease); err != nil {
			return err
		}
		return e.recordQueryHealth(
			ctx, task.ChannelNo, outcomeForCallFailure(failure), string(failure.Code), latency,
		)
	}
	if result == nil || result.Validate() != nil ||
		result.ExternalTaskRef != task.ExternalTaskRef ||
		(result.ExternalResourceRef != "" &&
			result.ExternalResourceRef != task.ExternalResourceRef) {
		if err := e.rescheduleTaskObservation(ctx, task, lease); err != nil {
			return err
		}
		return e.recordQueryHealth(
			ctx, task.ChannelNo, providerhealth.OutcomeResultUnknown,
			invalidProviderResponseCode, latency,
		)
	}
	nextAt := (*time.Time)(nil)
	if !result.Status.IsTerminal() {
		next := nextObservationAt(task.TaskNo, task.ObservationAttempt, task.LeaseExpiresAt)
		nextAt = &next
	}
	if err := e.tasks.ObserveTask(ctx, ObservationInput{
		Lease:  lease,
		Result: *result,
		NextAt: nextAt,
	}); err != nil {
		return err
	}
	return e.recordQueryHealth(
		ctx, task.ChannelNo, providerhealth.OutcomeSucceeded, "succeeded", latency,
	)
}

func (e *Executor) executeResourceObservation(
	ctx context.Context,
	task ClaimedResourceObservation,
	construction providerConstruction,
) error {
	lease := ObservationLeaseRef{
		TaskNo: task.TaskNo, Token: task.LeaseToken, Version: task.LeaseVersion,
	}
	reschedule := func() error {
		return e.tasks.RescheduleObservation(ctx, ObservationRescheduleInput{
			Lease: lease,
			NextAt: nextObservationAt(
				task.TaskNo,
				task.ObservationAttempt,
				task.LeaseExpiresAt,
			),
		})
	}
	if construction.err != nil || construction.provider == nil || construction.cleanup == nil {
		return reschedule()
	}
	query := resourcecap.ResourceQuery{ExternalResourceRef: task.ExternalResourceRef}
	startedAt := time.Now()
	snapshot, callErr := construction.provider.QueryResource(ctx, query)
	latency := time.Since(startedAt)
	if callErr != nil {
		failure := resourcecap.NormalizeProviderError(callErr)
		if err := reschedule(); err != nil {
			return err
		}
		return e.recordQueryHealth(
			ctx, task.ChannelNo, outcomeForCallFailure(failure), string(failure.Code), latency,
		)
	}
	if snapshot == nil || snapshot.Validate() != nil ||
		snapshot.ExternalResourceRef != task.ExternalResourceRef ||
		snapshot.ObservedAt.Before(task.ClaimedAt) {
		if err := reschedule(); err != nil {
			return err
		}
		return e.recordQueryHealth(
			ctx, task.ChannelNo, providerhealth.OutcomeResultUnknown,
			invalidProviderResponseCode, latency,
		)
	}
	if !resourceObservationConverges(
		resourcecap.CapabilityCode(task.CapabilityCode),
		task.SanitizedParameters,
		*snapshot,
	) {
		if err := reschedule(); err != nil {
			return err
		}
		return e.recordQueryHealth(
			ctx, task.ChannelNo, providerhealth.OutcomeSucceeded, "succeeded", latency,
		)
	}
	if err := e.tasks.ObserveResource(ctx, ResourceObservationInput{
		Lease: lease, Snapshot: *snapshot,
	}); err != nil {
		return err
	}
	return e.recordQueryHealth(
		ctx, task.ChannelNo, providerhealth.OutcomeSucceeded, "succeeded", latency,
	)
}

func (e *Executor) recordQueryHealth(
	ctx context.Context,
	channelNo string,
	outcome providerhealth.Outcome,
	summaryCode string,
	latency time.Duration,
) error {
	return e.health.Record(ctx, providerhealth.Observation{
		ChannelNo: channelNo, Operation: providerhealth.OperationQuery,
		Outcome: outcome, SummaryCode: summaryCode, Latency: latency,
		AuthenticationFailure: summaryCode ==
			string(resourcecap.ProviderCallErrorCodeAuthentication),
	})
}

func (e *Executor) rescheduleTaskObservation(
	ctx context.Context,
	task ClaimedObservation,
	lease ObservationLeaseRef,
) error {
	return e.tasks.RescheduleObservation(ctx, ObservationRescheduleInput{
		Lease:  lease,
		NextAt: nextObservationAt(task.TaskNo, task.ObservationAttempt, task.LeaseExpiresAt),
	})
}

func (e *Executor) persistBeforeCallFailure(
	ctx context.Context,
	task ClaimedTask,
	lease LeaseRef,
	code string,
	summary string,
) error {
	to := StatusFailed
	var nextAt *time.Time
	if task.AttemptNo < task.MaxAttempts {
		to = StatusRetryWait
		next := task.LeaseExpiresAt
		nextAt = &next
	}
	return e.tasks.Transition(ctx, TransitionInput{
		Lease:       lease,
		To:          to,
		Evidence:    TransitionEvidence{NonAcceptanceProven: true},
		FailureCode: code,
		FailureText: summary,
		NextAt:      nextAt,
	})
}

func (e *Executor) persistCallFailure(
	ctx context.Context,
	task ClaimedTask,
	lease LeaseRef,
	failure *resourcecap.ProviderCallError,
) error {
	if failure == nil || failure.ResultUnknown {
		code := string(resourcecap.ProviderCallErrorCodeUnknown)
		if failure != nil {
			code = string(failure.Code)
		}
		return e.persistResultUnknown(
			ctx,
			lease,
			task.LeaseExpiresAt,
			code,
			providerCallUnknownSummary,
			nil,
			"",
			nil,
		)
	}
	to := StatusFailed
	var nextAt *time.Time
	if failure.Retryable && task.AttemptNo < task.MaxAttempts {
		to = StatusRetryWait
		next := task.LeaseExpiresAt
		nextAt = &next
	}
	return e.tasks.Transition(ctx, TransitionInput{
		Lease:       lease,
		To:          to,
		Evidence:    TransitionEvidence{NonAcceptanceProven: true},
		FailureCode: string(failure.Code),
		FailureText: providerCallFailureSummary,
		NextAt:      nextAt,
	})
}

func (e *Executor) persistResult(
	ctx context.Context,
	task ClaimedTask,
	lease LeaseRef,
	requestedResourceRef string,
	result resourcecap.OperationResult,
) error {
	providerFacts, err := AppendResidualResourceEvidence(
		result.Evidence, result.ResidualResources, time.Now().UTC())
	if err != nil {
		return err
	}
	input := TransitionInput{
		Lease: lease, SecretOutputs: result.SecretOutputs,
		ResourceRef:   operationResultResourceRefOrExpected(result, requestedResourceRef),
		ProviderFacts: providerFacts,
	}
	if result.ExternalTaskRef != "" {
		input.Upstream = &UpstreamTask{
			TaskRef:     result.ExternalTaskRef,
			ResourceRef: operationResultResourceRefOrExpected(result, requestedResourceRef),
			Status:      result.Status,
			Progress:    result.Progress,
		}
	}
	switch result.Status {
	case resourcecap.TaskStatusAccepted:
		input.To = StatusAccepted
		input.Evidence.AcceptanceProven = true
		next := task.LeaseExpiresAt
		input.NextAt = &next
	case resourcecap.TaskStatusRunning:
		input.To = StatusRunning
		input.Evidence.AcceptanceProven = true
		next := task.LeaseExpiresAt
		input.NextAt = &next
	case resourcecap.TaskStatusSucceeded:
		input.To = StatusSucceeded
		input.Evidence.AcceptanceProven = true
	case resourcecap.TaskStatusFailed:
		if result.ExternalTaskRef == "" && operationResultResourceRef(result) == "" {
			if result.Failure.Retryable && task.AttemptNo < task.MaxAttempts {
				input.To = StatusRetryWait
				next := task.LeaseExpiresAt
				input.NextAt = &next
			} else {
				input.To = StatusFailed
			}
			input.Evidence.NonAcceptanceProven = true
		} else if result.ExternalTaskRef != "" {
			input.To = StatusFailed
			input.Evidence.ObservationProven = true
		} else {
			return e.persistResultUnknown(
				ctx,
				lease,
				task.LeaseExpiresAt,
				result.Failure.Code,
				providerCallUnknownSummary,
				nil,
				input.ResourceRef,
				input.ProviderFacts,
			)
		}
		input.FailureCode = result.Failure.Code
		input.FailureText = result.Failure.Message
	case resourcecap.TaskStatusUnknown:
		return e.persistResultUnknown(
			ctx,
			lease,
			task.LeaseExpiresAt,
			result.Failure.Code,
			result.Failure.Message,
			input.Upstream,
			input.ResourceRef,
			input.ProviderFacts,
		)
	case resourcecap.TaskStatusCanceled:
		input.To = StatusCanceled
		input.Evidence.ObservationProven = true
	case resourcecap.TaskStatusTerminated:
		input.To = StatusFailed
		input.Evidence.ObservationProven = true
		input.FailureCode = upstreamTerminatedCode
		input.FailureText = upstreamTerminatedSummary
	default:
		return e.persistResultUnknown(
			ctx,
			lease,
			task.LeaseExpiresAt,
			invalidProviderResponseCode,
			invalidProviderResponseSummary,
			nil,
			"",
			nil,
		)
	}
	return e.tasks.Transition(ctx, input)
}

func (e *Executor) persistResultUnknown(
	ctx context.Context,
	lease LeaseRef,
	nextAt time.Time,
	code string,
	summary string,
	upstream *UpstreamTask,
	resourceRef string,
	providerFacts []resourcecap.Evidence,
) error {
	return e.tasks.Transition(ctx, TransitionInput{
		Lease:         lease,
		To:            StatusResultUnknown,
		FailureCode:   code,
		FailureText:   summary,
		NextAt:        &nextAt,
		Upstream:      upstream,
		ResourceRef:   resourceRef,
		ProviderFacts: providerFacts,
	})
}

func resultResourceMismatch(requestRef string, result *resourcecap.OperationResult) bool {
	if requestRef == "" {
		return false
	}
	observed := operationResultResourceRef(*result)
	return observed != "" && observed != requestRef
}

func operationResultResourceRef(result resourcecap.OperationResult) string {
	if result.Resource != nil {
		return result.Resource.ExternalResourceRef
	}
	return result.ExternalResourceRef
}

func operationResultResourceRefOrExpected(
	result resourcecap.OperationResult,
	expected string,
) string {
	observed := operationResultResourceRef(result)
	if observed != "" {
		return observed
	}
	return expected
}

func resourceResolutionRequests(tasks []ClaimedTask) []ResourceResolutionRequest {
	requests := make([]ResourceResolutionRequest, len(tasks))
	for index := range tasks {
		requests[index] = ResourceResolutionRequest{
			TaskNo:          tasks[index].TaskNo,
			Capability:      resourcecap.CapabilityCode(tasks[index].CapabilityCode),
			LocalResourceNo: tasks[index].LocalResourceNo,
			ChannelNo:       tasks[index].ChannelNo,
			PoolNo:          tasks[index].PoolNo,
		}
	}
	return requests
}

func claimedTaskChannels(tasks []ClaimedTask) []string {
	seen := make(map[string]struct{}, len(tasks))
	channels := make([]string, 0, len(tasks))
	for index := range tasks {
		if _, exists := seen[tasks[index].ChannelNo]; exists {
			continue
		}
		seen[tasks[index].ChannelNo] = struct{}{}
		channels = append(channels, tasks[index].ChannelNo)
	}
	return channels
}
