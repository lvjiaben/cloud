// This file verifies dynamic secret, confidential output, and safe error contracts.

package resourcecap

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

// TestOperationRequestValidatesDynamicSecretBindings verifies per-operation
// credentials use the same bounded opaque-handle boundary as static settings.
func TestOperationRequestValidatesDynamicSecretBindings(t *testing.T) {
	handle := mustSecurityContractHandle(t, "sec_operation_password")
	request := OperationRequest{
		RequestID:   "operation-reset-password",
		Capability:  CapabilityResetPassword,
		ResourceRef: "resource-test",
		Parameters:  []byte(`{"mode":"generated"}`),
		SecretBindings: []SecretBinding{
			{Name: "newPassword", Handle: handle},
		},
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid operation secret binding rejected: %v", err)
	}

	duplicate := request
	duplicate.SecretBindings = append([]SecretBinding(nil), request.SecretBindings...)
	duplicate.SecretBindings = append(duplicate.SecretBindings, request.SecretBindings[0])
	if err := duplicate.Validate(); err == nil {
		t.Fatal("expected duplicate operation secret binding to fail")
	}

	zero := request
	zero.SecretBindings = []SecretBinding{{Name: "newPassword"}}
	if err := zero.Validate(); err == nil {
		t.Fatal("expected zero operation secret handle to fail")
	}

	tooMany := request
	tooMany.SecretBindings = make([]SecretBinding, 0, MaximumSecretBindingCount+1)
	for index := 0; index <= MaximumSecretBindingCount; index++ {
		tooMany.SecretBindings = append(tooMany.SecretBindings, SecretBinding{
			Name: fmt.Sprintf("credential%d", index), Handle: handle,
		})
	}
	if err := tooMany.Validate(); err == nil {
		t.Fatal("expected operation secret binding budget to fail")
	}
}

// TestOperationResultConfidentialOutputsRequireRequestContext verifies
// plaintext never enters ordinary results and output handles remain bound to
// the operation capability and policy that produced them.
func TestOperationResultConfidentialOutputsRequireRequestContext(t *testing.T) {
	handle := mustSecurityContractHandle(t, "sec_console_ticket")
	expiresAt := validationReferenceTime().Add(10 * time.Minute)
	consoleRequest := OperationRequest{
		RequestID: "operation-console", Capability: CapabilityConsole, ResourceRef: "resource-test",
	}
	consoleResult := OperationResult{
		Status:              TaskStatusSucceeded,
		ExternalResourceRef: "resource-test",
		SecretOutputs: []SecretOutput{{
			Name: "consoleTicket", Policy: SecretOutputPolicyShortLivedTicket,
			Handle: handle, ExpiresAt: expiresAt,
		}},
	}
	if err := consoleResult.Validate(); err == nil {
		t.Fatal("expected context-free validation of confidential output to fail closed")
	}
	if err := consoleResult.ValidateFor(consoleRequest); err != nil {
		t.Fatalf("valid console output rejected: %v", err)
	}
	missingExpiry := consoleResult
	missingExpiry.SecretOutputs = append([]SecretOutput(nil), consoleResult.SecretOutputs...)
	missingExpiry.SecretOutputs[0].ExpiresAt = time.Time{}
	if err := missingExpiry.ValidateFor(consoleRequest); err == nil {
		t.Fatal("expected short-lived ticket without expiry to fail")
	}
	duplicateOutput := consoleResult
	duplicateOutput.SecretOutputs = append(
		append([]SecretOutput(nil), consoleResult.SecretOutputs...),
		consoleResult.SecretOutputs[0],
	)
	if err := duplicateOutput.ValidateFor(consoleRequest); err == nil {
		t.Fatal("expected duplicate confidential output to fail")
	}

	wrongPolicy := consoleResult
	wrongPolicy.SecretOutputs = []SecretOutput{{
		Name: "consoleTicket", Policy: SecretOutputPolicyOneTimeCredential, Handle: handle,
	}}
	if err := wrongPolicy.ValidateFor(consoleRequest); err == nil {
		t.Fatal("expected console credential policy mismatch to fail")
	}

	reinstallRequest := OperationRequest{
		RequestID: "operation-reinstall", Capability: CapabilityReinstall, ResourceRef: "resource-test",
	}
	reinstallResult := OperationResult{
		Status:              TaskStatusSucceeded,
		ExternalResourceRef: "resource-test",
		SecretOutputs: []SecretOutput{{
			Name: "generatedCredential", Policy: SecretOutputPolicyOneTimeCredential, Handle: handle,
		}},
	}
	if err := reinstallResult.ValidateFor(reinstallRequest); err != nil {
		t.Fatalf("valid reinstall credential rejected: %v", err)
	}
	oneTimeWithExpiry := reinstallResult
	oneTimeWithExpiry.SecretOutputs = append([]SecretOutput(nil), reinstallResult.SecretOutputs...)
	oneTimeWithExpiry.SecretOutputs[0].ExpiresAt = expiresAt
	if err := oneTimeWithExpiry.ValidateFor(reinstallRequest); err == nil {
		t.Fatal("expected one-time credential with expiry to fail")
	}

	failed := reinstallResult
	failed.Status = TaskStatusFailed
	failed.Failure = &ProviderFailure{Code: "UPSTREAM_FAILED", Message: "upstream request failed"}
	if err := failed.ValidateFor(reinstallRequest); err == nil {
		t.Fatal("expected failed result with confidential output to fail")
	}

	unknown := reinstallResult
	unknown.Status = TaskStatusUnknown
	unknown.Failure = &ProviderFailure{
		Code: "RESULT_UNKNOWN", Message: "upstream result is unknown", ResultUnknown: true,
	}
	if err := unknown.ValidateFor(reinstallRequest); err == nil {
		t.Fatal("expected unknown result with confidential output to fail")
	}

	unrelatedRequest := OperationRequest{
		RequestID: "operation-start", Capability: CapabilityStart, ResourceRef: "resource-test",
	}
	if err := reinstallResult.ValidateFor(unrelatedRequest); err == nil {
		t.Fatal("expected unsupported output capability to fail")
	}
}

// TestStoreSecretEnforcesSingleUseAndClearsPlaintext verifies the guarded
// helper rejects broken sinks, discards handles on failure, and invalidates the
// producer byte view immediately after synchronous storage.
func TestStoreSecretEnforcesSingleUseAndClearsPlaintext(t *testing.T) {
	handle := mustSecurityContractHandle(t, "sec_generated_credential")
	request := SecretStoreRequest{
		RequestID: "operation-reinstall", Capability: CapabilityReinstall,
		Name: "generatedCredential", Policy: SecretOutputPolicyOneTimeCredential,
	}
	plaintext := []byte("username=root;password=must-not-pass;port=22")
	const expectedPlaintext = "username=root;password=must-not-pass;port=22"
	var retainedView []byte
	sink := securityContractSecretSink(func(
		_ context.Context,
		got SecretStoreRequest,
		produce SecretProducer,
	) (secretcap.Handle, error) {
		if got != request {
			return secretcap.Handle{}, errors.New("unexpected request")
		}
		err := produce(func(value []byte) error {
			retainedView = value
			if string(value) != expectedPlaintext {
				return errors.New("unexpected plaintext")
			}
			return nil
		})
		return handle, err
	})
	stored, err := StoreSecret(context.Background(), sink, request, func(consume SecretConsumer) error {
		return consume(plaintext)
	})
	if err != nil {
		t.Fatalf("store valid secret: %v", err)
	}
	if !stored.Equal(handle) {
		t.Fatal("stored secret handle does not match sink result")
	}
	for index, value := range plaintext {
		if value != 0 || retainedView[index] != 0 {
			t.Fatal("plaintext view remained valid after StoreSecret returned")
		}
	}

	brokenSink := securityContractSecretSink(func(
		_ context.Context,
		_ SecretStoreRequest,
		produce SecretProducer,
	) (secretcap.Handle, error) {
		consume := func([]byte) error { return nil }
		_ = produce(consume)
		_ = produce(consume)
		return handle, nil
	})
	stored, err = StoreSecret(context.Background(), brokenSink, request, func(consume SecretConsumer) error {
		return consume([]byte("short-lived"))
	})
	if err == nil || !stored.IsZero() {
		t.Fatal("broken sink did not fail with a zero handle")
	}

	failingSink := securityContractSecretSink(func(
		context.Context,
		SecretStoreRequest,
		SecretProducer,
	) (secretcap.Handle, error) {
		return handle, errors.New("token=must-not-pass")
	})
	stored, err = StoreSecret(context.Background(), failingSink, request, func(SecretConsumer) error { return nil })
	if err == nil || !stored.IsZero() {
		t.Fatal("sink failure did not discard its handle")
	}
	if strings.Contains(err.Error(), "must-not-pass") {
		t.Fatal("StoreSecret reflected an untrusted sink error")
	}

	producerFailureSink := securityContractSecretSink(func(
		_ context.Context,
		_ SecretStoreRequest,
		produce SecretProducer,
	) (secretcap.Handle, error) {
		return handle, produce(func([]byte) error { return nil })
	})
	stored, err = StoreSecret(context.Background(), producerFailureSink, request, func(consume SecretConsumer) error {
		if err := consume([]byte("must-not-pass")); err != nil {
			return err
		}
		return errors.New("password=must-not-pass")
	})
	if err == nil || !stored.IsZero() || strings.Contains(err.Error(), "must-not-pass") {
		t.Fatal("producer failure was not normalized with a zero handle")
	}
}

// TestNormalizeProviderErrorNeverReflectsRawErrors verifies the call-error
// boundary carries only typed policy metadata into logs, persistence, or retry.
func TestNormalizeProviderErrorNeverReflectsRawErrors(t *testing.T) {
	raw := errors.New("request failed token=must-not-pass")
	normalized := NormalizeProviderError(raw)
	if normalized == nil {
		t.Fatal("raw error normalized to nil")
	}
	if normalized.Code != ProviderCallErrorCodeUnknown || normalized.Retryable || !normalized.ResultUnknown {
		t.Fatalf("raw error normalized unsafely: %#v", normalized)
	}
	if strings.Contains(normalized.Error(), "must-not-pass") {
		t.Fatal("normalized error reflected raw error text")
	}

	typed, err := NewProviderCallError(ProviderCallErrorCodeTimeout, false, true)
	if err != nil {
		t.Fatalf("create typed Provider call error: %v", err)
	}
	normalized = NormalizeProviderError(fmt.Errorf("safe wrapper: %w", typed))
	if normalized.Code != ProviderCallErrorCodeTimeout || normalized.Retryable || !normalized.ResultUnknown {
		t.Fatalf("typed error metadata was not retained: %#v", normalized)
	}
	if got := NormalizeProviderError(nil); got != nil {
		t.Fatalf("nil error normalized to %#v", got)
	}
}

// TestTaskStatusTerminalAndProgressSemantics verifies cancellation outcomes
// are explicit and bounded progress never changes terminal classification.
func TestTaskStatusTerminalAndProgressSemantics(t *testing.T) {
	progress := 35
	for _, status := range []TaskStatus{TaskStatusCanceled, TaskStatusTerminated} {
		result := OperationResult{Status: status, ExternalTaskRef: "task-test", Progress: &progress}
		if err := result.Validate(); err != nil {
			t.Fatalf("valid terminal status %q rejected: %v", status, err)
		}
		if !status.IsTerminal() {
			t.Fatalf("status %q is not terminal", status)
		}
	}
	for _, status := range []TaskStatus{TaskStatusAccepted, TaskStatusRunning, TaskStatusUnknown} {
		if status.IsTerminal() {
			t.Fatalf("status %q unexpectedly classified as terminal", status)
		}
	}
	for _, value := range []int{-1, 101} {
		candidate := value
		result := OperationResult{Status: TaskStatusRunning, ExternalTaskRef: "task-test", Progress: &candidate}
		if err := result.Validate(); err == nil {
			t.Fatalf("out-of-range progress %d was accepted", value)
		}
	}
}

// securityContractSecretSink adapts a test function to SecretSink.
type securityContractSecretSink func(
	context.Context,
	SecretStoreRequest,
	SecretProducer,
) (secretcap.Handle, error)

// Store invokes the configured test function.
func (sink securityContractSecretSink) Store(
	ctx context.Context,
	request SecretStoreRequest,
	produce SecretProducer,
) (secretcap.Handle, error) {
	return sink(ctx, request, produce)
}

// mustSecurityContractHandle creates one test-only opaque handle.
func mustSecurityContractHandle(t *testing.T, reference string) secretcap.Handle {
	t.Helper()
	handle, err := secretcap.NewHandle(reference)
	if err != nil {
		t.Fatalf("create secret handle: %v", err)
	}
	return handle
}
