// This file implements the isolated resource-provider bridge. The guest only
// validates normalized, secret-free resource contracts and calls the exact
// one-use confidential outbound context. Supplier credentials, tokens,
// endpoints, raw responses, and secret handles never enter WASM memory.

package backend

import (
	"bytes"
	"encoding/json"
	"io"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-zjmfcloud/backend/internal/provider/zjmfcloud"
)

func validResourceProviderInvocation(invocation *capmodel.BusinessExtensionBridgeRequest) bool {
	if invocation.OwnerPluginID != resourcecap.OwnerPluginID ||
		invocation.ContractVersion != resourcecap.DynamicProviderContractVersion {
		return false
	}
	if invocation.Method == resourcecap.DynamicProviderMethodConfigurationCandidates {
		return invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeNewBusiness
	}
	if invocation.Purpose != capmodel.BusinessExtensionInvocationPurposeExistingBusiness {
		return false
	}
	if invocation.Method == resourcecap.DynamicProviderMethodLookupPrincipal || invocation.Method == resourcecap.DynamicProviderMethodCreatePrincipal {
		return true
	}
	for _, method := range resourcecap.DynamicProviderMethods() {
		if invocation.Method == method {
			return true
		}
	}
	return false
}

func executeResourceProviderInvocation(
	executor confidentialoutboundcap.Service,
	invocation *capmodel.BusinessExtensionBridgeRequest,
) (json.RawMessage, error) {
	if err := validateResourceProviderRequest(invocation.Method, invocation.Payload); err != nil {
		return nil, err
	}
	projection, err := zjmfcloud.ExecuteDynamicResource(
		executor, invocation.RuntimeContext.ContextID, invocation.Method, invocation.Payload,
	)
	if err != nil {
		return nil, err
	}
	if err = validateResourceProviderResult(invocation.Method, projection, invocation.Payload); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), projection...), nil
}

func validateResourceProviderRequest(method string, payload json.RawMessage) error {
	switch method {
	case resourcecap.DynamicProviderMethodVerifyConnection:
		var request struct{}
		return decodeStrictResourcePayload(payload, &request)
	case resourcecap.DynamicProviderMethodDiscoverCapabilities:
		var request struct{}
		return decodeStrictResourcePayload(payload, &request)
	case resourcecap.DynamicProviderMethodListProviderCapacity:
		var request resourcecap.ProviderCapacityRequest
		if err := decodeStrictResourcePayload(payload, &request); err != nil {
			return err
		}
		return request.Validate()
	case resourcecap.DynamicProviderMethodConfigurationCandidates:
		_, err := decodeResourceCandidateRequest(payload)
		return err
	case resourcecap.DynamicProviderMethodBuildProvisionParameters:
		var request resourcecap.ServerProvisionPlan
		if err := decodeStrictResourcePayload(payload, &request); err != nil {
			return err
		}
		return request.Validate()
	case resourcecap.DynamicProviderMethodLookupPrincipal, resourcecap.DynamicProviderMethodCreatePrincipal:
		var request resourcecap.DynamicPrincipalRequest
		if err := decodeStrictResourcePayload(payload, &request); err != nil {
			return err
		}
		return request.Validate(method == resourcecap.DynamicProviderMethodCreatePrincipal)
	case resourcecap.DynamicProviderMethodExecute:
		var request resourcecap.DynamicOperationRequest
		if err := decodeStrictResourcePayload(payload, &request); err != nil {
			return err
		}
		if err := request.Validate(); err != nil {
			return err
		}
		if request.Capability == resourcecap.CapabilityProvision &&
			(len(request.ConfidentialInputNames) != 1 || request.ConfidentialInputNames[0] != "password") {
			return io.ErrUnexpectedEOF
		}
		return nil
	case resourcecap.DynamicProviderMethodQueryTask:
		var request resourcecap.TaskQuery
		if err := decodeStrictResourcePayload(payload, &request); err != nil {
			return err
		}
		return request.Validate()
	case resourcecap.DynamicProviderMethodQueryResource:
		var request resourcecap.ResourceQuery
		if err := decodeStrictResourcePayload(payload, &request); err != nil {
			return err
		}
		return request.Validate()
	default:
		return io.ErrUnexpectedEOF
	}
}

func validateResourceProviderResult(method string, payload json.RawMessage, requestPayload json.RawMessage) error {
	switch method {
	case resourcecap.DynamicProviderMethodVerifyConnection:
		var result resourcecap.ConnectionVerification
		if err := decodeStrictResourcePayload(payload, &result); err != nil {
			return err
		}
		prepareGuestShapeObservation(&result)
		return result.ValidateAt(guestShapeValidationTime())
	case resourcecap.DynamicProviderMethodDiscoverCapabilities:
		var result resourcecap.CapabilitySnapshot
		if err := decodeStrictResourcePayload(payload, &result); err != nil {
			return err
		}
		// Freshness belongs to the trusted resource owner, which validates this
		// observation against its own clock after the isolated call returns.
		return nil
	case resourcecap.DynamicProviderMethodListProviderCapacity:
		var (
			request resourcecap.ProviderCapacityRequest
			result  resourcecap.ProviderCapacityPage
		)
		if err := decodeStrictResourcePayload(requestPayload, &request); err != nil {
			return err
		}
		if err := decodeStrictResourcePayload(payload, &result); err != nil {
			return err
		}
		prepareGuestShapeObservation(&result)
		return result.Validate(request)
	case resourcecap.DynamicProviderMethodConfigurationCandidates:
		return validateResourceCandidateResult(payload, requestPayload)
	case resourcecap.DynamicProviderMethodBuildProvisionParameters:
		return validateStrictJSONObject(payload)
	case resourcecap.DynamicProviderMethodLookupPrincipal:
		var result resourcecap.PrincipalLookupResult
		if err := decodeStrictResourcePayload(payload, &result); err != nil {
			return err
		}
		prepareGuestShapeObservation(&result)
		return result.Validate()
	case resourcecap.DynamicProviderMethodCreatePrincipal:
		var result resourcecap.PrincipalCreateResult
		if err := decodeStrictResourcePayload(payload, &result); err != nil {
			return err
		}
		prepareGuestShapeObservation(&result)
		return result.Validate()
	case resourcecap.DynamicProviderMethodExecute:
		var result resourcecap.OperationResult
		if err := decodeStrictResourcePayload(payload, &result); err != nil {
			return err
		}
		// Secret outputs never traverse WASM. The trusted owner retains any
		// handles beside this safe acknowledgement and merges them locally.
		prepareGuestShapeObservation(&result)
		return result.Validate()
	case resourcecap.DynamicProviderMethodQueryTask:
		var result resourcecap.OperationResult
		if err := decodeStrictResourcePayload(payload, &result); err != nil {
			return err
		}
		prepareGuestShapeObservation(&result)
		return result.Validate()
	case resourcecap.DynamicProviderMethodQueryResource:
		var result resourcecap.ResourceSnapshot
		if err := decodeStrictResourcePayload(payload, &result); err != nil {
			return err
		}
		prepareGuestShapeObservation(&result)
		return result.Validate()
	default:
		return io.ErrUnexpectedEOF
	}
}

func validateStrictJSONObject(payload json.RawMessage) error {
	if len(payload) == 0 || len(payload) > maximumCandidateResponse {
		return io.ErrUnexpectedEOF
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil || value == nil {
		return io.ErrUnexpectedEOF
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func decodeStrictResourcePayload(content []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return err
	}
	return nil
}
