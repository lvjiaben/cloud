// This file freezes the first reviewed installation-local guard policy.

package supplierguard

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultPolicyContract(t *testing.T) {
	policy := DefaultPolicy()
	if !validPolicy(policy) || policy.Version != 1 || policy.SampleWindow != 20 ||
		policy.MinimumSamples != 5 || policy.FailureThreshold != 3 ||
		policy.UnknownThreshold != 2 || policy.ConsecutiveFailureThreshold != 3 ||
		policy.OpenDuration != 5*time.Minute || policy.ProbeTimeout != time.Minute {
		t.Fatalf("unexpected default policy: %+v", policy)
	}
	if policy.InstallationRate != (RatePolicy{Window: time.Minute, Limit: 120}) ||
		policy.OperationRate != (RatePolicy{Window: time.Minute, Limit: 30}) {
		t.Fatalf("unexpected default rate policy: %+v", policy)
	}
}

func TestNewServiceRejectsInvalidBoundary(t *testing.T) {
	if _, err := NewService(nil, "invalid", DefaultPolicy()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid service error=%v", err)
	}
	policy := DefaultPolicy()
	policy.SampleWindow = 4
	if validPolicy(policy) {
		t.Fatal("undersized sample window was accepted")
	}
}
