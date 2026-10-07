package providerhealth

import (
	"testing"
	"time"
)

func TestWriteWindowOpensOnlyAfterBoundedThreshold(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	state := &healthState{WindowStartedAt: &now}
	applyDefaultPolicy(state)
	for index, outcome := range []Outcome{
		OutcomeSucceeded,
		OutcomeSucceeded,
		OutcomeFailed,
		OutcomeResultUnknown,
		OutcomeResultUnknown,
	} {
		applyObservation(state, Observation{Operation: OperationWrite, Outcome: outcome}, now)
		if index < 4 && state.CircuitOpen {
			t.Fatalf("write protection opened after only %d samples: %+v", index+1, state)
		}
	}
	if !state.CircuitOpen || state.WriteProtectedUntil == nil ||
		!state.WriteProtectedUntil.Equal(now.Add(defaultProtectionDurationSeconds*time.Second)) ||
		state.SampleCount != 5 || state.FailureCount != 3 || state.UnknownCount != 2 {
		t.Fatalf("unexpected protected write window: %+v", state)
	}
}

func TestSuccessResetsConsecutiveFailuresWithoutErasingWindowRate(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	state := &healthState{
		SampleCount: 4, FailureCount: 2, UnknownCount: 1,
		ConsecutiveFailures: 2, WindowStartedAt: &now,
	}
	applyDefaultPolicy(state)
	applyObservation(state, Observation{
		Operation: OperationWrite, Outcome: OutcomeSucceeded,
	}, now)
	if state.ConsecutiveFailures != 0 || state.SampleCount != 5 ||
		state.FailureCount != 2 || state.UnknownCount != 1 || state.CircuitOpen {
		t.Fatalf("success corrupted the bounded health window: %+v", state)
	}
}

func TestHealthObservationValidationFailsClosed(t *testing.T) {
	t.Parallel()
	valid := Observation{
		ChannelNo: "channel-1", Operation: OperationWrite,
		Outcome: OutcomeSucceeded, SummaryCode: "succeeded", Latency: time.Second,
	}
	if err := validateObservation(valid); err != nil {
		t.Fatalf("valid health observation rejected: %v", err)
	}
	invalid := []Observation{
		{ChannelNo: "", Operation: OperationWrite, Outcome: OutcomeSucceeded, SummaryCode: "ok"},
		{ChannelNo: "channel-1", Operation: "delete", Outcome: OutcomeSucceeded, SummaryCode: "ok"},
		{ChannelNo: "channel-1", Operation: OperationWrite, Outcome: "maybe", SummaryCode: "ok"},
		{ChannelNo: "channel-1", Operation: OperationWrite, Outcome: OutcomeFailed, SummaryCode: "password=leak"},
		{ChannelNo: "channel-1", Operation: OperationWrite, Outcome: OutcomeFailed, SummaryCode: "ok", Latency: -1},
		{ChannelNo: "channel-1", Operation: OperationWrite, Outcome: OutcomeSucceeded, SummaryCode: "authentication", AuthenticationFailure: true},
		{ChannelNo: "channel-1", Operation: OperationAuthentication, Outcome: OutcomeFailed, SummaryCode: "authentication", AuthenticationFailure: true},
		{ChannelNo: "channel-1", Operation: OperationQuery, Outcome: OutcomeFailed, SummaryCode: "timeout", AuthenticationFailure: true},
	}
	for _, input := range invalid {
		if err := validateObservation(input); err == nil {
			t.Fatalf("invalid health observation accepted: %+v", input)
		}
	}
}

func TestGateChannelValidationDeduplicatesAndBounds(t *testing.T) {
	t.Parallel()
	channels, err := validateGateChannels([]string{"channel-1", "channel-1", "channel-2"})
	if err != nil || len(channels) != 2 || channels[0] != "channel-1" || channels[1] != "channel-2" {
		t.Fatalf("unexpected validated health channels=%v err=%v", channels, err)
	}
	tooMany := make([]string, maximumGateChannelCount+1)
	for index := range tooMany {
		tooMany[index] = "channel-" + string(rune('a'+index))
	}
	if _, err = validateGateChannels(tooMany); err == nil {
		t.Fatal("oversized health gate batch was accepted")
	}
	if _, err = validateGateChannels([]string{""}); err == nil {
		t.Fatal("invalid health channel was accepted")
	}
}

func TestHealthPolicyValidationAndProtectionEvaluation(t *testing.T) {
	t.Parallel()
	valid := HealthPolicyUpdateInput{
		ChannelNo: "channel-1", WindowSampleLimit: 10, MinimumSamplesToProtect: 3,
		FailureLimit: 2, UnknownResultLimit: 1, ConsecutiveFailureLimit: 2,
		ProtectionDurationSeconds: 600, ExpectedVersion: 1,
	}
	if err := validatePolicyUpdate(valid); err != nil {
		t.Fatalf("valid Provider health policy rejected: %v", err)
	}
	invalid := []HealthPolicyUpdateInput{
		{ChannelNo: "", WindowSampleLimit: 10, MinimumSamplesToProtect: 3, FailureLimit: 2, UnknownResultLimit: 1, ConsecutiveFailureLimit: 2, ProtectionDurationSeconds: 600, ExpectedVersion: 1},
		{ChannelNo: "channel-1", WindowSampleLimit: 4, MinimumSamplesToProtect: 3, FailureLimit: 2, UnknownResultLimit: 1, ConsecutiveFailureLimit: 2, ProtectionDurationSeconds: 600, ExpectedVersion: 1},
		{ChannelNo: "channel-1", WindowSampleLimit: 10, MinimumSamplesToProtect: 11, FailureLimit: 2, UnknownResultLimit: 1, ConsecutiveFailureLimit: 2, ProtectionDurationSeconds: 600, ExpectedVersion: 1},
		{ChannelNo: "channel-1", WindowSampleLimit: 10, MinimumSamplesToProtect: 3, FailureLimit: 2, UnknownResultLimit: 3, ConsecutiveFailureLimit: 2, ProtectionDurationSeconds: 600, ExpectedVersion: 1},
		{ChannelNo: "channel-1", WindowSampleLimit: 10, MinimumSamplesToProtect: 3, FailureLimit: 2, UnknownResultLimit: 1, ConsecutiveFailureLimit: 2, ProtectionDurationSeconds: 59, ExpectedVersion: 1},
		{ChannelNo: "channel-1", WindowSampleLimit: 10, MinimumSamplesToProtect: 3, FailureLimit: 2, UnknownResultLimit: 1, ConsecutiveFailureLimit: 2, ProtectionDurationSeconds: 600, ExpectedVersion: 0},
	}
	for _, input := range invalid {
		if err := validatePolicyUpdate(input); err == nil {
			t.Fatalf("invalid Provider health policy accepted: %+v", input)
		}
	}
	state := &healthState{SampleCount: 3, FailureCount: 2, ConsecutiveFailures: 1}
	applyPolicyInput(state, valid)
	if !policyProtects(state) {
		t.Fatalf("custom failure policy did not protect current window: %+v", state)
	}
}

func TestOverallHealthCodeIsConservative(t *testing.T) {
	t.Parallel()
	if got := overallHealthCode(&healthState{CircuitOpen: true}); got != "unavailable" {
		t.Fatalf("open circuit health=%q want unavailable", got)
	}
	if got := overallHealthCode(&healthState{LastQueryHealthCode: "degraded"}); got != "degraded" {
		t.Fatalf("degraded dimension health=%q want degraded", got)
	}
	if got := overallHealthCode(&healthState{LastProviderCapacityHealthCode: "healthy"}); got != "healthy" {
		t.Fatalf("healthy dimension health=%q want healthy", got)
	}
	if got := overallHealthCode(&healthState{}); got != "unknown" {
		t.Fatalf("unobserved health=%q want unknown", got)
	}
}
