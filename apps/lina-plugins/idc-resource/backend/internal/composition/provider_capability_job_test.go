// This file verifies managed primary-node registration and bounded collection.
package composition

import (
	"context"
	"errors"
	"lina-plugin-idc-resource/backend/internal/service/providercapability"
	"testing"
)

func TestProviderCapabilityJobUsesPrimaryAndBoundedBatch(t *testing.T) {
	secondary := &jobRegistrarFake{}
	collector := &accessoryRefreshWorkerFake{remaining: 12}
	if err := RegisterProviderCapabilityJob(t.Context(), secondary, collector); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary registration err=%v", err)
	}
	primary := &jobRegistrarFake{primary: true}
	if err := RegisterProviderCapabilityJob(t.Context(), primary, collector); err != nil {
		t.Fatal(err)
	}
	if primary.name != "idc-resource-provider-capability-refresh" || primary.pattern != "@every 30s" {
		t.Fatal("capability job registration changed")
	}
	if err := primary.handler(t.Context()); err != nil || collector.calls != providerCapabilityBatchLimit {
		t.Fatalf("collector calls=%d err=%v", collector.calls, err)
	}
	collector.err = errors.New("discovery failed")
	if err := primary.handler(t.Context()); err == nil {
		t.Fatal("collector failure hidden")
	}
	if err := RegisterProviderCapabilityJob(t.Context(), primary, nil); err == nil {
		t.Fatal("missing collector accepted")
	}
}

type capabilityCollectionStep struct {
	worked bool
	err    error
}
type capabilityCollectionSequence struct {
	steps []capabilityCollectionStep
	calls int
}

func (s *capabilityCollectionSequence) RunOne(context.Context) (bool, error) {
	index := s.calls
	s.calls++
	if index >= len(s.steps) {
		return false, nil
	}
	return s.steps[index].worked, s.steps[index].err
}
func TestProviderCapabilityBatchContinuesOnlyCommittedFailures(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		steps     []capabilityCollectionStep
		wantCalls int
	}{
		{name: "committed-failure-does-not-block-healthy-channel", steps: []capabilityCollectionStep{{true, providercapability.ErrDiscoveryFailed}, {true, nil}, {false, nil}}, wantCalls: 3},
		{name: "uncommitted-store-error-stops", steps: []capabilityCollectionStep{{true, errors.New("database unavailable")}, {true, nil}}, wantCalls: 1},
		{name: "failed-fact-not-committed-stops", steps: []capabilityCollectionStep{{false, providercapability.ErrDiscoveryFailed}, {true, nil}}, wantCalls: 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			registrar := &jobRegistrarFake{primary: true}
			collector := &capabilityCollectionSequence{steps: scenario.steps}
			if err := RegisterProviderCapabilityJob(t.Context(), registrar, collector); err != nil {
				t.Fatal(err)
			}
			if err := registrar.handler(t.Context()); err == nil {
				t.Fatal("capability failure disappeared from job result")
			}
			if collector.calls != scenario.wantCalls {
				t.Fatalf("capability calls=%d want=%d", collector.calls, scenario.wantCalls)
			}
		})
	}
	registrar := &jobRegistrarFake{primary: true}
	collector := &capabilityCollectionSequence{}
	for range providerCapabilityBatchLimit + 4 {
		collector.steps = append(collector.steps, capabilityCollectionStep{true, providercapability.ErrDiscoveryFailed})
	}
	if err := RegisterProviderCapabilityJob(t.Context(), registrar, collector); err != nil {
		t.Fatal(err)
	}
	if err := registrar.handler(t.Context()); err == nil || collector.calls != providerCapabilityBatchLimit {
		t.Fatalf("failed channels escaped batch limit: calls=%d err=%v", collector.calls, err)
	}
}
