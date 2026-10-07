// This file validates providerCapacity, operation, resource, failure, and evidence values.

package resourcecap

import (
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// ValidateOwnerMetadata validates bounded, secret-free resource-owner facts
// that are persisted for replay checks and never sent to a Provider.
func ValidateOwnerMetadata(metadata json.RawMessage) error {
	return validateJSONObject("resource owner metadata", metadata, false)
}

// Validate verifies one bounded providerCapacity request before an upstream call.
func (r ProviderCapacityRequest) Validate() error {
	if err := validateProviderCapacityKind(r.Kind); err != nil {
		return err
	}
	if r.Limit <= 0 || r.Limit > MaximumProviderCapacityPageSize {
		return gerror.Newf("resource Provider providerCapacity limit must be between 1 and %d", MaximumProviderCapacityPageSize)
	}
	if err := validateOptionalText("providerCapacity cursor", r.Cursor, maximumCursorBytes); err != nil {
		return err
	}
	return validateJSONObject("providerCapacity filters", r.Filters, false)
}

// Validate verifies one providerCapacity page against the request that produced it.
// The owner must stage all pages and publish only after the terminal Complete
// page has been validated.
func (p ProviderCapacityPage) Validate(request ProviderCapacityRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if len(p.Items) > request.Limit || len(p.Items) > MaximumProviderCapacityPageSize {
		return gerror.New("resource Provider providerCapacity page exceeds the requested limit")
	}
	if err := validateOptionalText("providerCapacity next cursor", p.NextCursor, maximumCursorBytes); err != nil {
		return err
	}
	if p.Complete && p.NextCursor != "" {
		return gerror.New("resource Provider complete providerCapacity page must not contain a next cursor")
	}
	if !p.Complete && p.NextCursor == "" {
		return gerror.New("resource Provider incomplete providerCapacity page requires a next cursor")
	}
	if !p.Complete && p.NextCursor == request.Cursor {
		return gerror.New("resource Provider providerCapacity page must advance the cursor")
	}
	if len(p.Evidence) > MaximumEvidenceCount {
		return gerror.Newf("resource Provider providerCapacity evidence exceeds %d entries", MaximumEvidenceCount)
	}
	seen := make(map[string]struct{}, len(p.Items))
	totalJSONBytes := 0
	for _, item := range p.Items {
		if err := item.Validate(); err != nil {
			return err
		}
		if item.Kind != request.Kind {
			return gerror.New("resource Provider providerCapacity item kind does not match the request")
		}
		if _, exists := seen[item.ExternalRef]; exists {
			return gerror.New("resource Provider providerCapacity reference is duplicated")
		}
		seen[item.ExternalRef] = struct{}{}
		totalJSONBytes += len(item.Data)
		if totalJSONBytes > maximumPageJSONBytes {
			return gerror.Newf("resource Provider providerCapacity page JSON exceeds %d bytes", maximumPageJSONBytes)
		}
	}
	for _, evidence := range p.Evidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate verifies one normalized providerCapacity item is bounded and secret-free.
func (i ProviderCapacityItem) Validate() error {
	if err := validateProviderCapacityKind(i.Kind); err != nil {
		return err
	}
	if err := validateRequiredReference("providerCapacity external reference", i.ExternalRef); err != nil {
		return err
	}
	if err := validateJSONObject("providerCapacity item data", i.Data, true); err != nil {
		return err
	}
	return validateObservationTime("providerCapacity observation time", i.ObservedAt)
}

// Validate verifies an idempotent operation request before Execute is called.
func (r OperationRequest) Validate() error {
	if err := validateRequiredReference("operation request ID", r.RequestID); err != nil {
		return err
	}
	if err := validateCapabilityCode(r.Capability); err != nil {
		return err
	}
	if !isExecutableCapability(r.Capability) {
		return gerror.New("resource Provider capability cannot be executed")
	}
	if r.Capability == CapabilityProvision {
		if err := validateOptionalReference("operation resource reference", r.ResourceRef); err != nil {
			return err
		}
	} else if err := validateRequiredReference("operation resource reference", r.ResourceRef); err != nil {
		return err
	}
	if err := validateJSONObject("operation parameters", r.Parameters, false); err != nil {
		return err
	}
	if r.ProviderCorrelation != nil {
		if r.Capability != CapabilityProvision || ValidateProviderCorrelation(r.ProviderCorrelation) != nil {
			return gerror.New("resource Provider correlation is invalid")
		}
		found := false
		for _, binding := range r.SecretBindings {
			found = found || binding.Name == "downstream_token"
		}
		if !found {
			return gerror.New("resource Provider correlation token binding is missing")
		}
	}
	return validateSecretBindings(r.SecretBindings)
}

// Validate verifies one external-task query has a governed task reference.
func (q TaskQuery) Validate() error {
	if err := validateRequiredReference("external task reference", q.ExternalTaskRef); err != nil {
		return err
	}
	return validateOptionalReference("task resource reference", q.ResourceRef)
}

// Validate verifies one external-resource query has a governed resource reference.
func (q ResourceQuery) Validate() error {
	return validateRequiredReference("external resource reference", q.ExternalResourceRef)
}

// Validate verifies one operation observation before it can affect the local
// resource task or service state machine.
func (r OperationResult) Validate() error {
	return r.validateFor(nil)
}

// ValidateFor verifies one operation result using the exact request context
// required to authorize confidential output handles.
func (r OperationResult) ValidateFor(request OperationRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	return r.validateFor(&request)
}

// validateFor enforces result invariants and fails closed when confidential
// outputs are present without their originating request.
func (r OperationResult) validateFor(request *OperationRequest) error {
	if err := r.validateEnvelope(request); err != nil {
		return err
	}
	if len(r.ResidualResources) != 0 && request != nil && request.Capability != CapabilityTerminate {
		return gerror.New("resource Provider residual resources require a terminate operation")
	}
	return r.validateStatusSemantics()
}

// validateEnvelope verifies bounded references, evidence, progress, resource,
// and confidential-output projections independently of task-state semantics.
func (r OperationResult) validateEnvelope(request *OperationRequest) error {
	if err := validateTaskStatus(r.Status); err != nil {
		return err
	}
	if err := validateOptionalReference("result external task reference", r.ExternalTaskRef); err != nil {
		return err
	}
	if err := validateOptionalReference("result external resource reference", r.ExternalResourceRef); err != nil {
		return err
	}
	if r.Progress != nil && (*r.Progress < 0 || *r.Progress > 100) {
		return gerror.New("resource Provider operation progress must be between 0 and 100")
	}
	if err := r.validateSecretOutputs(request); err != nil {
		return err
	}
	if len(r.Evidence) > MaximumEvidenceCount {
		return gerror.Newf("resource Provider operation evidence exceeds %d entries", MaximumEvidenceCount)
	}
	for _, evidence := range r.Evidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
	}
	if len(r.ResidualResources) > MaximumResidualResourceCount {
		return gerror.Newf("resource Provider residual resources exceed %d entries", MaximumResidualResourceCount)
	}
	if len(r.Evidence)+len(r.ResidualResources) > MaximumEvidenceCount {
		return gerror.Newf("resource Provider combined operation evidence exceeds %d entries", MaximumEvidenceCount)
	}
	seenResiduals := make(map[string]struct{}, len(r.ResidualResources))
	for _, residual := range r.ResidualResources {
		if err := residual.Validate(); err != nil {
			return err
		}
		key := residual.ResourceKind + "\x00" + residual.ResourceRef
		if _, exists := seenResiduals[key]; exists {
			return gerror.New("resource Provider residual resource is duplicated")
		}
		seenResiduals[key] = struct{}{}
	}
	if r.Resource != nil {
		if err := r.Resource.Validate(); err != nil {
			return err
		}
		if r.ExternalResourceRef != "" && r.ExternalResourceRef != r.Resource.ExternalResourceRef {
			return gerror.New("resource Provider result resource references do not match")
		}
	}
	return nil
}

// validateStatusSemantics verifies reference and failure invariants for each
// normalized task state after the envelope has passed validation.
func (r OperationResult) validateStatusSemantics() error {
	if len(r.ResidualResources) != 0 && r.Status != TaskStatusFailed && r.Status != TaskStatusUnknown {
		return gerror.New("resource Provider residual resources require a failed or unknown result")
	}
	switch r.Status {
	case TaskStatusAccepted, TaskStatusRunning:
		if r.ExternalTaskRef == "" {
			return gerror.Newf("resource Provider %s result requires an external task reference", r.Status)
		}
		if r.Failure != nil {
			return gerror.Newf("resource Provider %s result must not contain a failure", r.Status)
		}
	case TaskStatusSucceeded:
		if r.Failure != nil {
			return gerror.New("resource Provider succeeded result must not contain a failure")
		}
		if r.ExternalTaskRef == "" && r.ExternalResourceRef == "" && r.Resource == nil {
			return gerror.New("resource Provider succeeded result requires a task or resource reference")
		}
	case TaskStatusFailed:
		if r.Failure == nil {
			return gerror.New("resource Provider failed result requires failure details")
		}
		if err := r.Failure.Validate(); err != nil {
			return err
		}
		if r.Failure.ResultUnknown {
			return gerror.New("resource Provider failed result cannot be marked result-unknown")
		}
	case TaskStatusUnknown:
		if r.Failure == nil {
			return gerror.New("resource Provider unknown result requires failure details")
		}
		if err := r.Failure.Validate(); err != nil {
			return err
		}
		if !r.Failure.ResultUnknown {
			return gerror.New("resource Provider unknown result must be marked result-unknown")
		}
	case TaskStatusCanceled, TaskStatusTerminated:
		if r.ExternalTaskRef == "" {
			return gerror.Newf("resource Provider %s result requires an external task reference", r.Status)
		}
		if r.Failure != nil {
			if err := r.Failure.Validate(); err != nil {
				return err
			}
			if r.Failure.Retryable || r.Failure.ResultUnknown {
				return gerror.New("resource Provider terminal cancellation failure cannot be retryable or result-unknown")
			}
		}
	default:
		return gerror.New("resource Provider task status is invalid")
	}
	return nil
}

// Validate verifies one bounded, normalized residual resource projection.
func (r ResidualResource) Validate() error {
	if err := validateSafeName("residual resource kind", r.ResourceKind, maximumNameBytes); err != nil {
		return err
	}
	if err := validateRequiredReference("residual resource reference", r.ResourceRef); err != nil {
		return err
	}
	if err := validateMachineValue("residual resource status", string(r.Status)); err != nil {
		return err
	}
	switch r.Status {
	case ResidualResourcePresent, ResidualResourceReleasePending,
		ResidualResourceReleaseFailed, ResidualResourceUnknown:
	default:
		return gerror.New("resource Provider residual resource status is invalid")
	}
	if err := validateSafeName("residual resource reason code", r.ReasonCode, maximumCodeBytes); err != nil {
		return err
	}
	if containsCredentialMaterial(r.ReasonCode) {
		return gerror.New("resource Provider residual resource reason code contains credential-like material")
	}
	return nil
}

// validateSecretOutputs verifies bounded encrypted outputs and their
// capability-specific reveal policy.
func (r OperationResult) validateSecretOutputs(request *OperationRequest) error {
	if len(r.SecretOutputs) == 0 {
		return nil
	}
	if request == nil {
		return gerror.New("resource Provider confidential outputs require operation request validation")
	}
	if len(r.SecretOutputs) > MaximumSecretOutputCount {
		return gerror.Newf("resource Provider confidential outputs exceed %d entries", MaximumSecretOutputCount)
	}
	switch r.Status {
	case TaskStatusAccepted, TaskStatusRunning, TaskStatusSucceeded:
	default:
		return gerror.New("resource Provider non-successful result must not contain confidential outputs")
	}
	seen := make(map[string]struct{}, len(r.SecretOutputs))
	for _, output := range r.SecretOutputs {
		if err := output.Validate(); err != nil {
			return err
		}
		if err := validateSecretOutputPolicyForCapability(request.Capability, output.Policy); err != nil {
			return err
		}
		if _, exists := seen[output.Name]; exists {
			return gerror.New("resource Provider confidential output is duplicated")
		}
		seen[output.Name] = struct{}{}
	}
	return nil
}

// Validate verifies confidential storage metadata before plaintext production.
func (r SecretStoreRequest) Validate() error {
	if err := validateRequiredReference("secret output request ID", r.RequestID); err != nil {
		return err
	}
	if err := validateCapabilityCode(r.Capability); err != nil {
		return err
	}
	if !isExecutableCapability(r.Capability) {
		return gerror.New("resource Provider secret output capability cannot be executed")
	}
	if err := validateSafeName("secret output name", r.Name, maximumNameBytes); err != nil {
		return err
	}
	if err := validateSecretOutputPolicyForCapability(r.Capability, r.Policy); err != nil {
		return err
	}
	return validateSecretOutputExpiry(r.Policy, r.ExpiresAt)
}

// Validate verifies one encrypted confidential output handle and policy.
func (o SecretOutput) Validate() error {
	if err := validateSafeName("secret output name", o.Name, maximumNameBytes); err != nil {
		return err
	}
	if o.Handle.IsZero() {
		return gerror.New("resource Provider confidential output requires a non-zero handle")
	}
	if err := validateSecretOutputPolicy(o.Policy); err != nil {
		return err
	}
	return validateSecretOutputExpiry(o.Policy, o.ExpiresAt)
}

// validateSecretOutputPolicyForCapability restricts confidential outputs to
// operations whose normalized semantics explicitly require them.
func validateSecretOutputPolicyForCapability(capability CapabilityCode, policy SecretOutputPolicy) error {
	if err := validateSecretOutputPolicy(policy); err != nil {
		return err
	}
	switch capability {
	case CapabilityConsole:
		if policy != SecretOutputPolicyShortLivedTicket {
			return gerror.New("resource Provider console output requires a short-lived ticket policy")
		}
	case CapabilityProvision, CapabilityReinstall:
		if policy != SecretOutputPolicyOneTimeCredential {
			return gerror.New("resource Provider generated credential requires a one-time policy")
		}
	default:
		return gerror.New("resource Provider capability must not emit confidential outputs")
	}
	return nil
}

// validateSecretOutputPolicy applies bounded machine-value validation before
// checking membership in the confidential-output enum.
func validateSecretOutputPolicy(policy SecretOutputPolicy) error {
	if err := validateMachineValue("secret output policy", string(policy)); err != nil {
		return err
	}
	switch policy {
	case SecretOutputPolicyOneTimeCredential, SecretOutputPolicyShortLivedTicket:
		return nil
	default:
		return gerror.New("resource Provider secret output policy is invalid")
	}
}

// validateSecretOutputExpiry keeps one-time credentials expiry-free and makes
// every short-lived ticket's lifetime explicit for owner enforcement.
func validateSecretOutputExpiry(policy SecretOutputPolicy, expiresAt time.Time) error {
	switch policy {
	case SecretOutputPolicyOneTimeCredential:
		if !expiresAt.IsZero() {
			return gerror.New("resource Provider one-time credential must not declare an expiry")
		}
	case SecretOutputPolicyShortLivedTicket:
		if err := validateObservationTime("secret output expiry", expiresAt); err != nil {
			return err
		}
	default:
		return gerror.New("resource Provider secret output policy is invalid")
	}
	return nil
}

// Validate verifies one normalized resource observation is stable, bounded,
// and secret-free.
func (r ResourceSnapshot) Validate() error {
	if err := validateRequiredReference("resource snapshot reference", r.ExternalResourceRef); err != nil {
		return err
	}
	if err := validateResourceState(r.State); err != nil {
		return err
	}
	if err := validateJSONObject("resource snapshot data", r.Data, false); err != nil {
		return err
	}
	if err := validateObservationTime("resource observation time", r.ObservedAt); err != nil {
		return err
	}
	if len(r.Evidence) > MaximumEvidenceCount {
		return gerror.Newf("resource Provider resource evidence exceeds %d entries", MaximumEvidenceCount)
	}
	for _, evidence := range r.Evidence {
		if err := evidence.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate verifies redacted failure details before they enter task evidence.
func (f ProviderFailure) Validate() error {
	if err := validateSafeName("failure code", f.Code, maximumCodeBytes); err != nil {
		return err
	}
	if err := validateSafeDiagnostic("failure message", f.Message, maximumMessageBytes, true); err != nil {
		return err
	}
	if f.ResultUnknown && f.Retryable {
		return gerror.New("resource Provider result-unknown failure cannot be retryable")
	}
	return nil
}

// Validate verifies one redacted evidence item is bounded, attributable, and
// safe for persistence and operator diagnostics.
func (e Evidence) Validate() error {
	if err := validateSafeName("evidence kind", e.Kind, maximumNameBytes); err != nil {
		return err
	}
	if err := validateOptionalReference("evidence reference", e.Reference); err != nil {
		return err
	}
	if err := validateSafeDiagnostic("evidence summary", e.Summary, maximumEvidenceSummaryBytes, false); err != nil {
		return err
	}
	if e.Reference == "" && e.Summary == "" {
		return gerror.New("resource Provider evidence requires a reference or summary")
	}
	return validateObservationTime("evidence observation time", e.ObservedAt)
}

// isProviderCapacityKind reports whether an providerCapacity projection kind is recognized.
func isProviderCapacityKind(kind ProviderCapacityKind) bool {
	switch kind {
	case ProviderCapacityKindRegion,
		ProviderCapacityKindLine,
		ProviderCapacityKindPlan,
		ProviderCapacityKindImage,
		ProviderCapacityKindStock,
		ProviderCapacityKindCost,
		ProviderCapacityKindBalance,
		ProviderCapacityKindExpiry:
		return true
	default:
		return false
	}
}

// validateProviderCapacityKind applies machine-value bounds before enum membership.
func validateProviderCapacityKind(kind ProviderCapacityKind) error {
	if err := validateMachineValue("providerCapacity kind", string(kind)); err != nil {
		return err
	}
	if !isProviderCapacityKind(kind) {
		return gerror.New("resource Provider providerCapacity kind is invalid")
	}
	return nil
}

// isExecutableCapability reports whether Execute may accept a capability.
func isExecutableCapability(code CapabilityCode) bool {
	switch code {
	case CapabilityProvision,
		CapabilityStart,
		CapabilityStop,
		CapabilityReboot,
		CapabilitySuspend,
		CapabilityResume,
		CapabilityRenew,
		CapabilityResetPassword,
		CapabilityConsole,
		CapabilityReinstall,
		CapabilityEnterRescue,
		CapabilityExitRescue,
		CapabilityResize,
		CapabilityChangeIP,
		CapabilityManageSnapshot,
		CapabilityBackup,
		CapabilitySecurityGroup,
		CapabilityNAT,
		CapabilityVirtualMedia,
		CapabilityTrafficPackage,
		CapabilityUpstreamTicket,
		CapabilityTerminate:
		return true
	default:
		return false
	}
}

// isResourceState reports whether a normalized resource state is recognized.
func isResourceState(state ResourceState) bool {
	switch state {
	case ResourceStatePending,
		ResourceStateRunning,
		ResourceStateActive,
		ResourceStateStopped,
		ResourceStateSuspended,
		ResourceStateFailed,
		ResourceStateTerminated,
		ResourceStateUnknown:
		return true
	default:
		return false
	}
}

// validateTaskStatus applies machine-value bounds before enum membership.
func validateTaskStatus(status TaskStatus) error {
	if err := validateMachineValue("task status", string(status)); err != nil {
		return err
	}
	switch status {
	case TaskStatusAccepted,
		TaskStatusRunning,
		TaskStatusSucceeded,
		TaskStatusFailed,
		TaskStatusUnknown,
		TaskStatusCanceled,
		TaskStatusTerminated:
		return nil
	default:
		return gerror.New("resource Provider task status is invalid")
	}
}

// validateResourceState applies machine-value bounds before enum membership.
func validateResourceState(state ResourceState) error {
	if err := validateMachineValue("resource state", string(state)); err != nil {
		return err
	}
	if !isResourceState(state) {
		return gerror.New("resource Provider resource state is invalid")
	}
	return nil
}
