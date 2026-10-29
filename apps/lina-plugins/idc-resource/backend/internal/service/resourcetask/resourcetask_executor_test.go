// This file verifies Provider execution orchestration and safe task convergence.

package resourcetask

import (
	"context"
	"testing"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/providerhealth"
)

// TestExecutorRunWritesPersistsAcceptedOutput verifies request construction,
// accepted-task binding, and encrypted output forwarding in one worker pass.
func TestExecutorRunWritesPersistsAcceptedOutput(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	inputHandle, err := secretcap.NewHandle("input-secret-ref")
	if err != nil {
		t.Fatalf("construct input handle failed: %v", err)
	}
	outputHandle, err := secretcap.NewHandle("output-secret-ref")
	if err != nil {
		t.Fatalf("construct output handle failed: %v", err)
	}
	task := ClaimedTask{
		TaskNo: "task-write-accepted", CapabilityCode: string(resourcecap.CapabilityReinstall),
		LocalResourceNo: "local-resource-1", ChannelNo: "channel-1", PoolNo: "pool-1",
		SanitizedParameters: `{"image":"debian"}`, AttemptNo: 1, MaxAttempts: 4,
		LeaseToken:     "worker:0123456789abcdef0123456789abcdef",
		LeaseExpiresAt: now.Add(time.Minute),
		SecretBindings: []resourcecap.SecretBinding{{Name: "password", Handle: inputHandle}},
	}
	store := &executorTaskService{claimedWrites: []ClaimedTask{task}}
	provider := &executorProvider{}
	provider.execute = func(_ context.Context, request resourcecap.OperationRequest) (*resourcecap.OperationResult, error) {
		if request.RequestID != task.TaskNo || request.ResourceRef != "external-resource-1" ||
			request.Capability != resourcecap.CapabilityReinstall || len(request.SecretBindings) != 1 {
			t.Fatalf("unexpected Provider request: %+v", request)
		}
		return &resourcecap.OperationResult{
			Status:          resourcecap.TaskStatusAccepted,
			ExternalTaskRef: "external-task-1",
			SecretOutputs: []resourcecap.SecretOutput{{
				Name:   "generatedCredential",
				Policy: resourcecap.SecretOutputPolicyOneTimeCredential,
				Handle: outputHandle,
			}},
		}, nil
	}
	executor := newExecutorFixture(t, store, provider)
	report, err := executor.RunWrites(context.Background(), ClaimInput{
		WorkerID: "worker", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || report.Claimed != 1 || report.Persisted != 1 || len(store.transitions) != 1 {
		t.Fatalf("run writes failed: report=%+v transitions=%+v err=%v", report, store.transitions, err)
	}
	transition := store.transitions[0]
	if transition.NextAt != nil {
		t.Fatal("accepted task must use owner database time rather than the future write lease")
	}
	if transition.To != StatusAccepted || !transition.Evidence.AcceptanceProven ||
		transition.Upstream == nil || transition.Upstream.TaskRef != "external-task-1" ||
		transition.Upstream.ResourceRef != "external-resource-1" ||
		len(transition.SecretOutputs) != 1 ||
		transition.SecretOutputs[0].Handle.OpaqueRef() != outputHandle.OpaqueRef() {
		t.Fatalf("accepted Provider result was not preserved: %+v", transition)
	}
}

// TestExecutorRunWritesNormalizesUnknownError verifies raw Provider text can
// neither enter the task transition nor make an uncertain write replayable.
func TestExecutorRunWritesNormalizesUnknownError(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	store := &executorTaskService{claimedWrites: []ClaimedTask{{
		TaskNo: "task-write-unknown", CapabilityCode: string(resourcecap.CapabilityStart),
		LocalResourceNo: "local-resource-1", ChannelNo: "channel-1",
		SanitizedParameters: `{}`, AttemptNo: 1, MaxAttempts: 4,
		LeaseToken:     "worker:0123456789abcdef0123456789abcdef",
		LeaseExpiresAt: now.Add(time.Minute),
	}}}
	provider := &executorProvider{execute: func(
		context.Context,
		resourcecap.OperationRequest,
	) (*resourcecap.OperationResult, error) {
		return nil, gerror.New("password=must-never-be-persisted")
	}}
	executor := newExecutorFixture(t, store, provider)
	if _, err := executor.RunWrites(context.Background(), ClaimInput{
		WorkerID: "worker", Limit: 1, LeaseDuration: time.Minute,
	}); err != nil {
		t.Fatalf("run writes failed: %v", err)
	}
	transition := store.transitions[0]
	if transition.To != StatusResultUnknown || transition.FailureCode != "unknown" ||
		transition.FailureText != providerCallUnknownSummary || transition.Evidence.NonAcceptanceProven {
		t.Fatalf("raw Provider error was not safely normalized: %+v", transition)
	}
}

// TestExecutorRunWritesRetriesKnownNonAcceptance verifies retry budget is used
// only for a typed call failure that proves the write result is known.
func TestExecutorRunWritesRetriesKnownNonAcceptance(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	store := &executorTaskService{claimedWrites: []ClaimedTask{{
		TaskNo: "task-write-retry", CapabilityCode: string(resourcecap.CapabilityStop),
		LocalResourceNo: "local-resource-1", ChannelNo: "channel-1",
		SanitizedParameters: `{}`, AttemptNo: 1, MaxAttempts: 2,
		LeaseToken:     "worker:0123456789abcdef0123456789abcdef",
		LeaseExpiresAt: now.Add(time.Minute),
	}}}
	callError, err := resourcecap.NewProviderCallError(
		resourcecap.ProviderCallErrorCodeTransport,
		true,
		false,
	)
	if err != nil {
		t.Fatalf("construct call error failed: %v", err)
	}
	provider := &executorProvider{execute: func(
		context.Context,
		resourcecap.OperationRequest,
	) (*resourcecap.OperationResult, error) {
		return nil, callError
	}}
	executor := newExecutorFixture(t, store, provider)
	if _, err = executor.RunWrites(context.Background(), ClaimInput{
		WorkerID: "worker", Limit: 1, LeaseDuration: time.Minute,
	}); err != nil {
		t.Fatalf("run writes failed: %v", err)
	}
	transition := store.transitions[0]
	if transition.To != StatusRetryWait || !transition.Evidence.NonAcceptanceProven || transition.NextAt == nil {
		t.Fatalf("known non-acceptance did not enter retry wait: %+v", transition)
	}
}

// TestExecutorRunWritesBatchesMappingsAndProviders verifies a worker pass uses
// one bounded mapping projection and one Provider construction per channel.
func TestExecutorRunWritesBatchesMappingsAndProviders(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	store := &executorTaskService{claimedWrites: []ClaimedTask{
		{
			TaskNo: "task-batch-1", CapabilityCode: string(resourcecap.CapabilityStart),
			LocalResourceNo: "local-resource-1", ChannelNo: "channel-1", PoolNo: "pool-1",
			SanitizedParameters: `{}`, AttemptNo: 1, MaxAttempts: 4,
			LeaseToken: "worker:0123456789abcdef0123456789abcdef", LeaseExpiresAt: now.Add(time.Minute),
		},
		{
			TaskNo: "task-batch-2", CapabilityCode: string(resourcecap.CapabilityStop),
			LocalResourceNo: "local-resource-2", ChannelNo: "channel-1", PoolNo: "pool-1",
			SanitizedParameters: `{}`, AttemptNo: 1, MaxAttempts: 4,
			LeaseToken: "worker:0123456789abcdef0123456789abcdef", LeaseExpiresAt: now.Add(time.Minute),
		},
	}}
	provider := &executorProvider{execute: func(
		_ context.Context,
		request resourcecap.OperationRequest,
	) (*resourcecap.OperationResult, error) {
		return &resourcecap.OperationResult{
			Status: resourcecap.TaskStatusSucceeded, ExternalResourceRef: request.ResourceRef,
		}, nil
	}}
	resolverCalls := 0
	constructorCalls := 0
	cleanupCalls := 0
	executor, err := NewExecutor(
		store,
		func(context.Context, string, string) (resourcecap.Provider, func(), error) {
			constructorCalls++
			return provider, func() { cleanupCalls++ }, nil
		},
		func(
			_ context.Context,
			requests []ResourceResolutionRequest,
		) (map[string]string, error) {
			resolverCalls++
			if len(requests) != 2 {
				t.Fatalf("mapping request count=%d want=2", len(requests))
			}
			return map[string]string{
				"task-batch-1": "external-resource-1",
				"task-batch-2": "external-resource-2",
			}, nil
		},
		&executorHealthService{}, allowTestWrite,
	)
	if err != nil {
		t.Fatalf("construct executor failed: %v", err)
	}
	report, err := executor.RunWrites(context.Background(), ClaimInput{
		WorkerID: "worker", Limit: 2, LeaseDuration: time.Minute,
	})
	if err != nil || report.Persisted != 2 || len(store.transitions) != 2 {
		t.Fatalf("run batched writes failed: report=%+v transitions=%+v err=%v", report, store.transitions, err)
	}
	if resolverCalls != 1 || constructorCalls != 1 || cleanupCalls != 1 {
		t.Fatalf("unexpected batch dependency calls: resolver=%d constructor=%d cleanup=%d",
			resolverCalls, constructorCalls, cleanupCalls)
	}
	if executor.health.(*executorHealthService).gateCalls != 1 {
		t.Fatalf("health protection lookup was not batched once")
	}
}

func TestExecutorRunWritesHonorsSharedHealthProtectionWithoutProviderCall(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	store := &executorTaskService{claimedWrites: []ClaimedTask{{
		TaskNo: "task-health-protected", CapabilityCode: string(resourcecap.CapabilityStart),
		LocalResourceNo: "local-resource-1", ChannelNo: "channel-1", PoolNo: "pool-1",
		SanitizedParameters: `{}`, AttemptNo: 1, MaxAttempts: 4,
		LeaseToken: "worker:0123456789abcdef0123456789abcdef", LeaseExpiresAt: now.Add(time.Minute),
	}}}
	providerCalls := 0
	constructorCalls := 0
	health := &executorHealthService{gate: providerhealth.Gate{ProtectedUntil: now.Add(time.Minute)}}
	executor, err := NewExecutor(
		store,
		func(context.Context, string, string) (resourcecap.Provider, func(), error) {
			constructorCalls++
			return &executorProvider{execute: func(
				context.Context, resourcecap.OperationRequest,
			) (*resourcecap.OperationResult, error) {
				providerCalls++
				return nil, nil
			}}, func() {}, nil
		},
		func(
			context.Context, []ResourceResolutionRequest,
		) (map[string]string, error) {
			return map[string]string{"task-health-protected": "external-resource-1"}, nil
		},
		health, allowTestWrite,
	)
	if err != nil {
		t.Fatalf("construct protected executor failed: %v", err)
	}
	report, err := executor.RunWrites(context.Background(), ClaimInput{
		WorkerID: "worker", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || report.Persisted != 1 || len(store.transitions) != 0 ||
		len(store.protectedDeferrals) != 1 {
		t.Fatalf("protected write did not persist retry state: report=%+v store=%+v err=%v", report, store, err)
	}
	if providerCalls != 0 || constructorCalls != 0 || health.gateCalls != 1 ||
		len(health.observations) != 0 {
		t.Fatalf("protected write crossed Provider boundary: constructor=%d provider=%d observations=%d",
			constructorCalls, providerCalls, len(health.observations))
	}
	deferred := store.protectedDeferrals[0]
	if deferred.TaskNo != "task-health-protected" || deferred.AttemptNo != 1 {
		t.Fatalf("unexpected protected deferral: %+v", deferred)
	}
}

// TestExecutorRunObservationsReschedulesAndConverges verifies query failures
// retain state while a validated terminal observation reaches the owner store.
func TestExecutorRunObservationsReschedulesAndConverges(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	base := ClaimedObservation{
		TaskNo: "task-observe-1", ChannelNo: "channel-1", CurrentStatus: StatusRunning,
		ExternalTaskRef: "external-task-1", ExternalResourceRef: "external-resource-1",
		LeaseToken: "worker:0123456789abcdef0123456789abcdef", LeaseVersion: 2,
		ObservationAttempt: 3, LeaseExpiresAt: now.Add(time.Minute),
	}
	store := &executorTaskService{claimedObservations: []ClaimedObservation{base}}
	provider := &executorProvider{queryTask: func(
		context.Context,
		resourcecap.TaskQuery,
	) (*resourcecap.OperationResult, error) {
		return nil, gerror.New("temporary upstream detail")
	}}
	health := &executorHealthService{}
	executor := newExecutorFixtureWithHealth(t, store, provider, health)
	if _, err := executor.RunObservations(context.Background(), ObservationClaimInput{
		WorkerID: "worker", Limit: 1, LeaseDuration: time.Minute,
	}); err != nil || len(store.reschedules) != 1 || len(store.observations) != 0 {
		t.Fatalf("query failure did not reschedule safely: err=%v store=%+v", err, store)
	}
	if !store.reschedules[0].NextAt.IsZero() {
		t.Fatal("query failure must request bounded owner-clock scheduling instead of adding the observation lease")
	}
	if len(health.observations) != 1 ||
		health.observations[0].Operation != providerhealth.OperationQuery ||
		health.observations[0].Outcome != providerhealth.OutcomeResultUnknown ||
		health.observations[0].SummaryCode != string(resourcecap.ProviderCallErrorCodeUnknown) {
		t.Fatalf("query failure health observation is invalid: %+v", health.observations)
	}

	store = &executorTaskService{claimedObservations: []ClaimedObservation{base}}
	provider = &executorProvider{queryTask: func(
		context.Context,
		resourcecap.TaskQuery,
	) (*resourcecap.OperationResult, error) {
		return &resourcecap.OperationResult{
			Status:              resourcecap.TaskStatusSucceeded,
			ExternalTaskRef:     "external-task-1",
			ExternalResourceRef: "external-resource-1",
		}, nil
	}}
	health = &executorHealthService{}
	executor = newExecutorFixtureWithHealth(t, store, provider, health)
	if _, err := executor.RunObservations(context.Background(), ObservationClaimInput{
		WorkerID: "worker", Limit: 1, LeaseDuration: time.Minute,
	}); err != nil || len(store.observations) != 1 || store.observations[0].NextAt != nil {
		t.Fatalf("terminal query did not converge: err=%v store=%+v", err, store)
	}
	if len(health.observations) != 1 ||
		health.observations[0].Outcome != providerhealth.OutcomeSucceeded ||
		health.observations[0].SummaryCode != "succeeded" {
		t.Fatalf("successful query health observation is invalid: %+v", health.observations)
	}
}

// TestExecutorRunResourceObservationsRequiresExactTargetState verifies an
// unknown task closes only after QueryResource proves its idempotent outcome.
func TestExecutorRunResourceObservationsRequiresExactTargetState(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	base := ClaimedResourceObservation{
		TaskNo: "task-resource-observe-1", CapabilityCode: string(resourcecap.CapabilityStart),
		ChannelNo: "channel-1", LocalResourceNo: "local-resource-1",
		ExternalResourceRef: "external-resource-1",
		LeaseToken:          "worker:0123456789abcdef0123456789abcdef", LeaseVersion: 2,
		ObservationAttempt: 4,
		ClaimedAt:          now.Add(-time.Millisecond),
		LeaseExpiresAt:     now.Add(time.Minute),
	}
	store := &executorTaskService{claimedResourceObservations: []ClaimedResourceObservation{base}}
	provider := &executorProvider{queryResource: func(
		context.Context,
		resourcecap.ResourceQuery,
	) (*resourcecap.ResourceSnapshot, error) {
		return &resourcecap.ResourceSnapshot{
			ExternalResourceRef: "external-resource-1",
			State:               resourcecap.ResourceStateStopped, ObservedAt: now,
		}, nil
	}}
	executor := newExecutorFixture(t, store, provider)
	if _, err := executor.RunResourceObservations(context.Background(), ObservationClaimInput{
		WorkerID: "worker", Limit: 1, LeaseDuration: time.Minute,
	}); err != nil || len(store.reschedules) != 1 || len(store.resourceObservations) != 0 {
		t.Fatalf("non-converging resource snapshot did not remain unknown: err=%v store=%+v", err, store)
	}
	if !store.reschedules[0].NextAt.IsZero() {
		t.Fatal("resource query failure must request owner-clock scheduling without the lease duration")
	}

	store = &executorTaskService{claimedResourceObservations: []ClaimedResourceObservation{base}}
	provider = &executorProvider{queryResource: func(
		context.Context,
		resourcecap.ResourceQuery,
	) (*resourcecap.ResourceSnapshot, error) {
		return &resourcecap.ResourceSnapshot{
			ExternalResourceRef: "external-resource-1",
			State:               resourcecap.ResourceStateRunning, ObservedAt: now,
		}, nil
	}}
	executor = newExecutorFixture(t, store, provider)
	if _, err := executor.RunResourceObservations(context.Background(), ObservationClaimInput{
		WorkerID: "worker", Limit: 1, LeaseDuration: time.Minute,
	}); err != nil || len(store.resourceObservations) != 1 || len(store.reschedules) != 0 {
		t.Fatalf("converging resource snapshot did not close unknown task: err=%v store=%+v", err, store)
	}
}

func newExecutorFixture(
	t *testing.T,
	store Service,
	provider resourcecap.Provider,
) *Executor {
	return newExecutorFixtureWithHealth(t, store, provider, &executorHealthService{})
}

func newExecutorFixtureWithHealth(
	t *testing.T,
	store Service,
	provider resourcecap.Provider,
	health providerhealth.Service,
) *Executor {
	t.Helper()
	executor, err := NewExecutor(
		store,
		func(context.Context, string, string) (resourcecap.Provider, func(), error) {
			return provider, func() {}, nil
		},
		func(
			context.Context,
			[]ResourceResolutionRequest,
		) (map[string]string, error) {
			return map[string]string{
				"task-write-accepted": "external-resource-1",
				"task-write-unknown":  "external-resource-1",
				"task-write-retry":    "external-resource-1",
			}, nil
		},
		health, allowTestWrite,
	)
	if err != nil {
		t.Fatalf("construct executor failed: %v", err)
	}
	return executor
}

type executorTaskService struct {
	claimedWrites               []ClaimedTask
	claimedObservations         []ClaimedObservation
	claimedResourceObservations []ClaimedResourceObservation
	transitions                 []TransitionInput
	protectedDeferrals          []LeaseRef
	observations                []ObservationInput
	resourceObservations        []ResourceObservationInput
	reschedules                 []ObservationRescheduleInput
}

type executorHealthService struct {
	gate         providerhealth.Gate
	gateCalls    int
	observations []providerhealth.Observation
}

func (s *executorHealthService) CheckWrites(
	_ context.Context,
	channelNos []string,
) (map[string]providerhealth.Gate, error) {
	s.gateCalls++
	gates := make(map[string]providerhealth.Gate, len(channelNos))
	if !s.gate.AllowsWrite() {
		for _, channelNo := range channelNos {
			gates[channelNo] = s.gate
		}
	}
	return gates, nil
}

func (s *executorHealthService) CheckWrite(
	_ context.Context,
	_ string,
) (providerhealth.Gate, error) {
	return s.gate, nil
}

func (s *executorHealthService) Record(_ context.Context, input providerhealth.Observation) error {
	s.observations = append(s.observations, input)
	return nil
}

func (s *executorTaskService) Claim(context.Context, ClaimInput) ([]ClaimedTask, error) {
	return append([]ClaimedTask(nil), s.claimedWrites...), nil
}

func (s *executorTaskService) Transition(_ context.Context, input TransitionInput) error {
	s.transitions = append(s.transitions, input)
	return nil
}

func (s *executorTaskService) DeferWriteForProtection(
	_ context.Context,
	lease LeaseRef,
	_ time.Time,
) error {
	s.protectedDeferrals = append(s.protectedDeferrals, lease)
	return nil
}

func (s *executorTaskService) ClaimObservations(
	context.Context,
	ObservationClaimInput,
) ([]ClaimedObservation, error) {
	return append([]ClaimedObservation(nil), s.claimedObservations...), nil
}

func (s *executorTaskService) ObserveTask(_ context.Context, input ObservationInput) error {
	s.observations = append(s.observations, input)
	return nil
}

func (s *executorTaskService) RescheduleObservation(
	_ context.Context,
	input ObservationRescheduleInput,
) error {
	s.reschedules = append(s.reschedules, input)
	return nil
}

func (s *executorTaskService) ClaimResourceObservations(
	context.Context,
	ObservationClaimInput,
) ([]ClaimedResourceObservation, error) {
	return append([]ClaimedResourceObservation(nil), s.claimedResourceObservations...), nil
}

func (s *executorTaskService) ObserveResource(_ context.Context, input ResourceObservationInput) error {
	s.resourceObservations = append(s.resourceObservations, input)
	return nil
}

type executorProvider struct {
	execute       func(context.Context, resourcecap.OperationRequest) (*resourcecap.OperationResult, error)
	queryTask     func(context.Context, resourcecap.TaskQuery) (*resourcecap.OperationResult, error)
	queryResource func(context.Context, resourcecap.ResourceQuery) (*resourcecap.ResourceSnapshot, error)
}

func (*executorProvider) DiscoverCapabilities(context.Context) (*resourcecap.CapabilitySnapshot, error) {
	return nil, gerror.New("not implemented by executor test")
}

func (*executorProvider) ListProviderCapacity(
	context.Context,
	resourcecap.ProviderCapacityRequest,
) (*resourcecap.ProviderCapacityPage, error) {
	return nil, gerror.New("not implemented by executor test")
}

func (p *executorProvider) Execute(
	ctx context.Context,
	request resourcecap.OperationRequest,
) (*resourcecap.OperationResult, error) {
	return p.execute(ctx, request)
}

func (p *executorProvider) QueryTask(
	ctx context.Context,
	request resourcecap.TaskQuery,
) (*resourcecap.OperationResult, error) {
	return p.queryTask(ctx, request)
}

func (p *executorProvider) QueryResource(
	ctx context.Context,
	request resourcecap.ResourceQuery,
) (*resourcecap.ResourceSnapshot, error) {
	return p.queryResource(ctx, request)
}

func allowTestWrite(ctx context.Context, _ ClaimedTask, _ resourcecap.OperationRequest, _ resourcecap.Provider, invoke func(context.Context) error) error {
	return invoke(ctx)
}

// Task success must not make an unready or unrelated cloud deliverable.
func TestExecutorProvisionObservationRequiresActualReadyResource(t *testing.T) {
	for _, test := range []struct {
		name, reference string
		state           resourcecap.ResourceState
		fail            bool
		delivered       bool
	}{
		{"ready", "cloud-41", resourcecap.ResourceStateActive, false, true},
		{"stopped ready", "cloud-41", resourcecap.ResourceStateStopped, false, true},
		{"still creating", "cloud-41", resourcecap.ResourceStatePending, false, false},
		{"wrong resource", "cloud-42", resourcecap.ResourceStateActive, false, false},
		{"query unavailable", "cloud-41", resourcecap.ResourceStateActive, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &executorTaskService{claimedObservations: []ClaimedObservation{{TaskNo: "task-provision-confirm", CapabilityCode: string(resourcecap.CapabilityProvision), ChannelNo: "channel-1", CurrentStatus: StatusAccepted, ExternalTaskRef: "task-71", ExternalResourceRef: "cloud-41", LeaseToken: "worker:0123456789abcdef0123456789abcdef", LeaseVersion: 2, LeaseExpiresAt: time.Now().Add(time.Minute)}}}
			provider := &executorProvider{queryTask: func(context.Context, resourcecap.TaskQuery) (*resourcecap.OperationResult, error) {
				return &resourcecap.OperationResult{Status: resourcecap.TaskStatusSucceeded, ExternalTaskRef: "task-71", ExternalResourceRef: "cloud-41"}, nil
			}, queryResource: func(_ context.Context, q resourcecap.ResourceQuery) (*resourcecap.ResourceSnapshot, error) {
				if q.ExternalResourceRef != "cloud-41" {
					t.Fatal("incorrect resource query")
				}
				if test.fail {
					return nil, gerror.New("unavailable")
				}
				return &resourcecap.ResourceSnapshot{ExternalResourceRef: test.reference, State: test.state, Data: []byte(`{}`), ObservedAt: time.Now().UTC()}, nil
			}}
			executor := newExecutorFixture(t, store, provider)
			if _, err := executor.RunObservations(context.Background(), ObservationClaimInput{WorkerID: "worker", Limit: 1, LeaseDuration: time.Minute}); err != nil {
				t.Fatal(err)
			}
			if test.delivered {
				if len(store.observations) != 1 || store.observations[0].Result.Resource == nil || len(store.reschedules) != 0 {
					t.Fatal("ready server did not converge")
				}
			} else if len(store.observations) != 0 || len(store.reschedules) != 1 {
				t.Fatal("unproven server delivered")
			}
		})
	}
}
