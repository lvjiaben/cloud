// This file invokes optional pure provider validation on canonical merchant
// configuration. The temporary context grants no configuration or outbound view.

package productautomation

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"slices"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/provideradmissioncap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (s *serviceImpl) validateProviderProductConfiguration(ctx context.Context, exact *capmodel.BusinessExtensionProviderEvidence, values json.RawMessage, choices ...map[string][]json.RawMessage) error {
	if !slices.Contains(exact.AuthorizedMethods, productfulfillmentcap.MethodValidateProductConfiguration) {
		if len(choices) > 0 && len(choices[0]) > 0 {
			return bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		return nil
	}
	var alternatives map[string][]json.RawMessage
	if len(choices) > 0 {
		alternatives = choices[0]
	}
	payload, err := json.Marshal(productfulfillmentcap.ConfigurationValidationRequest{Version: productfulfillmentcap.ContractVersion, ProductConfiguration: values, ProductChoiceValues: alternatives})
	if err != nil {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	// Pure validation must not carry an interface identity: secret access uses
	// the identity independently from the public configuration view.
	runtimeContext, err := s.runtimeContexts.issue(exact, dynamicCandidateScope{}, capmodel.BusinessExtensionInvocationPurposeNewBusiness, productfulfillmentcap.MethodValidateProductConfiguration, nil)
	if err != nil {
		return providerValidationFailureError(ctx, validationContextUnavailable)
	}
	defer s.runtimeContexts.remove(runtimeContext.id)
	result, err := s.extensions.InvokeProviderMethod(ctx, pluginhost.BusinessExtensionMethodInvokeInput{
		PluginID: exact.PluginID, Point: exact.Point, ContractVersion: exact.ContractVersion,
		Method: productfulfillmentcap.MethodValidateProductConfiguration, ReleaseVersion: exact.ReleaseVersion,
		ReleaseDigest: exact.ReleaseDigest, DeclarationDigest: exact.DeclarationDigest,
		Purpose:        capmodel.BusinessExtensionInvocationPurposeNewBusiness,
		RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: runtimeContext.id}, Payload: payload,
	})
	if err != nil {
		if provideradmissioncap.IsBusy(err) {
			return providerValidationFailureError(ctx, validationAdmissionBusy)
		}
		return providerValidationFailureError(ctx, validationInvocationFailed)
	}
	if result == nil {
		return providerValidationFailureError(ctx, validationResultMissing)
	}
	if result.Version != productfulfillmentcap.ContractVersion {
		return providerValidationFailureError(ctx, validationProtocolInvalid)
	}
	if len(result.Payload) > productfulfillmentcap.MaximumEvidenceBytes {
		return providerValidationFailureError(ctx, validationReplyTooLarge)
	}
	// Presence distinguishes an explicit provider rejection from an incomplete reply.
	var validation struct {
		Version string                                              `json:"version"`
		Valid   *bool                                               `json:"valid"`
		Reason  productfulfillmentcap.ConfigurationValidationReason `json:"reason,omitempty"`
	}
	decoder := json.NewDecoder(bytes.NewReader(result.Payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&validation) != nil || decoder.Decode(&struct{}{}) != io.EOF || validation.Version != productfulfillmentcap.ContractVersion || validation.Valid == nil {
		return providerValidationFailureError(ctx, validationReplyInvalid)
	}
	if !*validation.Valid && validation.Reason == productfulfillmentcap.ConfigurationValidationSupplierCapabilitiesUnconfirmed {
		return bizerr.NewCode(codeSupplierCapabilitiesUnconfirmed)
	}
	if !*validation.Valid || validation.Reason != "" {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	return nil
}

// providerValidationFailure admits fixed technical categories only; no caller
// passes guest payloads, configuration values or underlying error messages.
type providerValidationFailure string

const (
	validationAdmissionBusy      providerValidationFailure = "host_admission_busy"
	validationContextUnavailable providerValidationFailure = "runtime_context_unavailable"
	validationInvocationFailed   providerValidationFailure = "invocation_failed"
	validationResultMissing      providerValidationFailure = "result_missing"
	validationProtocolInvalid    providerValidationFailure = "protocol_invalid"
	validationReplyTooLarge      providerValidationFailure = "reply_too_large"
	validationReplyInvalid       providerValidationFailure = "reply_invalid"
)

// providerValidationFailureError returns an owner-translated public code and
// logs only the fixed preflight stage. Untrusted replies and causes are omitted.
func providerValidationFailureError(ctx context.Context, reason providerValidationFailure) error {
	switch reason {
	case validationAdmissionBusy, validationContextUnavailable, validationInvocationFailed, validationResultMissing, validationProtocolInvalid, validationReplyTooLarge, validationReplyInvalid:
	default:
		reason = validationReplyInvalid
	}
	logger.Warningf(ctx, "provider configuration validation failed stage=pure_validation reason=%s", reason)
	code := codeProviderValidationResponseInvalid
	switch reason {
	case validationAdmissionBusy:
		code = codeProviderBusy
	case validationContextUnavailable, validationInvocationFailed:
		code = codeProviderValidationUnavailable
	}
	return bizerr.NewCode(code, bizerr.Param{Name: "reason", Value: string(reason)})
}
