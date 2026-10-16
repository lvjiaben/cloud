// This file executes the protocol selected on the exact interface revision.

package dcim

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var safeResponseFields = []string{
	"code", "status", "msg", "message", "data", "list", "rows", "groups", "os",
	"id", "value", "group_id", "os_id", "name", "title", "label", "show_name", "os_name",
}

// ExecuteDynamic runs one owner-authorized operation without accepting an endpoint or credential.
func ExecuteDynamic(executor confidentialoutboundcap.Service, contextID string, settings json.RawMessage, method string, payload json.RawMessage) (json.RawMessage, error) {
	protocol, err := decodeProtocol(settings)
	if err != nil || executor == nil || strings.TrimSpace(contextID) == "" {
		return nil, errInvalidConfiguration
	}
	switch method {
	case resourcecap.DynamicProviderMethodVerifyConnection:
		call := candidateCalls(protocol, candidateRequest{Requests: []candidateQuery{{FieldKey: "server_group"}}})[0]
		response, callErr := businessCall(executor, contextID, 1, call.Method, call.Path, call.Query, call.Form, nil)
		if callErr != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, errInvalidResponse
		}
		return json.Marshal(resourcecap.ConnectionVerification{Verified: true, ObservedAt: time.Now().UTC()})
	case resourcecap.DynamicProviderMethodDiscoverCapabilities:
		return json.Marshal(declaredCapabilitySnapshot(protocol, time.Now().UTC()))
	case resourcecap.DynamicProviderMethodBuildProvisionParameters:
		var plan resourcecap.ServerProvisionPlan
		if json.Unmarshal(payload, &plan) != nil {
			return nil, errInvalidConfiguration
		}
		return resourcecap.BuildProvisionParameters(context.Background(), func(_ context.Context, plan resourcecap.ServerProvisionPlan) (json.RawMessage, error) {
			return buildProvisionParameters(protocol, plan)
		}, plan)
	case resourcecap.DynamicProviderMethodConfigurationCandidates:
		return executeCandidates(executor, contextID, protocol, payload)
	case resourcecap.DynamicProviderMethodListProviderCapacity:
		return executeCapacity(executor, contextID, protocol, payload)
	case resourcecap.DynamicProviderMethodExecute:
		return executeOperation(executor, contextID, protocol, payload)
	case resourcecap.DynamicProviderMethodQueryResource:
		return executeResourceQuery(executor, contextID, protocol, payload)
	case resourcecap.DynamicProviderMethodQueryTask:
		return nil, errUnsupported
	default:
		return nil, errUnsupported
	}
}

func executeCandidates(executor confidentialoutboundcap.Service, contextID string, protocol Protocol, payload json.RawMessage) (json.RawMessage, error) {
	request, err := decodeCandidateRequest(payload)
	if err != nil {
		return nil, err
	}
	fields := make([]resourceinterfaceprojectioncap.CandidateField, 0, len(request.Requests))
	for index, call := range candidateCalls(protocol, request) {
		response, callErr := businessCall(executor, contextID, uint8(index+1), call.Method, call.Path, call.Query, call.Form, nil)
		if callErr != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, errInvalidResponse
		}
		items, projectErr := projectCandidates(protocol, call.FieldKey, response.Data)
		if projectErr != nil {
			return nil, projectErr
		}
		fields = append(fields, resourceinterfaceprojectioncap.CandidateField{FieldKey: call.FieldKey, Items: items})
	}
	return json.Marshal(resourceinterfaceprojectioncap.CandidateResult{Fields: fields})
}

func executeCapacity(executor confidentialoutboundcap.Service, contextID string, protocol Protocol, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.ProviderCapacityRequest
	if json.Unmarshal(payload, &request) != nil || request.Validate() != nil || request.Kind != resourcecap.ProviderCapacityKindPlan {
		return nil, errInvalidConfiguration
	}
	if trimmed := bytes.TrimSpace(request.Filters); len(trimmed) != 0 && !bytes.Equal(trimmed, []byte("{}")) {
		return nil, errUnsupported
	}
	offset := 0
	if request.Cursor != "" {
		if !strings.HasPrefix(request.Cursor, "group:") {
			return nil, errInvalidConfiguration
		}
		var err error
		offset, err = strconv.Atoi(strings.TrimPrefix(request.Cursor, "group:"))
		if err != nil || offset < 1 {
			return nil, errInvalidConfiguration
		}
	}
	requestPayload, err := json.Marshal(candidateRequest{SchemaVersion: "v1", Requests: []candidateQuery{{FieldKey: "server_group"}}})
	if err != nil {
		return nil, errInvalidConfiguration
	}
	projected, err := executeCandidates(executor, contextID, protocol, requestPayload)
	if err != nil {
		return nil, err
	}
	var candidates resourceinterfaceprojectioncap.CandidateResult
	if json.Unmarshal(projected, &candidates) != nil || len(candidates.Fields) != 1 {
		return nil, errInvalidResponse
	}
	all := candidates.Fields[0].Items
	if offset > len(all) || offset == len(all) && offset != 0 {
		return nil, errInvalidConfiguration
	}
	end := offset + request.Limit
	if end > len(all) {
		end = len(all)
	}
	observedAt := time.Now().UTC()
	items := make([]resourcecap.ProviderCapacityItem, 0, end-offset)
	for _, candidate := range all[offset:end] {
		data, marshalErr := json.Marshal(map[string]string{"name": candidate.LabelZhCN})
		if marshalErr != nil {
			return nil, errInvalidResponse
		}
		items = append(items, resourcecap.ProviderCapacityItem{Kind: request.Kind, ExternalRef: candidate.Value, Data: data, ObservedAt: observedAt})
	}
	next := ""
	if end < len(all) {
		next = "group:" + strconv.Itoa(end)
	}
	return json.Marshal(resourcecap.ProviderCapacityPage{Items: items, NextCursor: next, Complete: end == len(all), Evidence: []resourcecap.Evidence{{Kind: "dcim_server_groups", Reference: string(protocol), Summary: "field-whitelisted DCIM server groups observed", ObservedAt: observedAt}}})
}

func executeOperation(executor confidentialoutboundcap.Service, contextID string, protocol Protocol, payload json.RawMessage) (json.RawMessage, error) {
	if protocol != ProtocolBMS {
		return nil, errUnsupported
	}
	var request resourcecap.DynamicOperationRequest
	if json.Unmarshal(payload, &request) != nil || request.Validate() != nil {
		return nil, errInvalidConfiguration
	}
	method, path, form, secretFields, err := bmsOperationCall(request)
	if err != nil {
		return nil, err
	}
	sequence := uint8(1)
	if request.ResourceRef != "" {
		owned, ownershipErr := businessCall(executor, contextID, sequence, http.MethodGet, "/bms/source/"+request.ResourceRef, nil, nil, nil)
		if ownershipErr != nil || owned.StatusCode < 200 || owned.StatusCode >= 300 || !resourceMatches(owned.Data, request.ResourceRef) {
			return nil, errInvalidResponse
		}
		sequence++
	}
	response, callErr := businessCall(executor, contextID, sequence, method, path, nil, form, secretFields)
	if callErr != nil {
		return json.Marshal(unknownResult(request))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || !businessSucceeded(response.Data) {
		return json.Marshal(failedResult(request))
	}
	resourceRef := request.ResourceRef
	if request.Capability == resourcecap.CapabilityProvision {
		resourceRef = responseReference(response.Data)
		if !positiveDecimal(resourceRef) {
			return json.Marshal(unknownResult(request))
		}
	}
	return json.Marshal(resourcecap.OperationResult{Status: resourcecap.TaskStatusSucceeded, ExternalResourceRef: resourceRef, Evidence: []resourcecap.Evidence{{Kind: "dcim_acknowledgement", Reference: request.RequestID, Summary: "field-whitelisted BMS operation acknowledgement observed", ObservedAt: time.Now().UTC()}}})
}

func bmsOperationCall(request resourcecap.DynamicOperationRequest) (string, string, url.Values, map[string]string, error) {
	if request.Capability == resourcecap.CapabilityProvision {
		if len(request.ConfidentialInputNames) != 1 || request.ConfidentialInputNames[0] != "password" {
			return "", "", nil, nil, errInvalidConfiguration
		}
		var parameters bmsProvisionParameters
		decoder := json.NewDecoder(bytes.NewReader(request.Parameters))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&parameters) != nil || !positiveDecimal(parameters.GroupID) || !positiveDecimal(parameters.OSID) || parameters.Size < 1 || parameters.Type != 2 ||
			parameters.Name == "" || len(parameters.Name) > 255 || parameters.ExternalPrincipalRef == "" || len(parameters.ExternalPrincipalRef) > 512 ||
			parameters.IPCount < 1 || parameters.IPCount > 256 || parameters.DataSize < 0 || parameters.BackupCount < 0 || parameters.SnapshotCount < 0 {
			return "", "", nil, nil, errInvalidConfiguration
		}
		encoded := url.Values{
			"name": {parameters.Name}, "type": {strconv.Itoa(parameters.Type)}, "size": {strconv.FormatInt(parameters.Size, 10)},
			"name3": {parameters.ExternalPrincipalRef}, "group_id": {parameters.GroupID}, "os_id": {parameters.OSID},
			"ip_num": {strconv.FormatInt(parameters.IPCount, 10)}, "data_size": {strconv.FormatInt(parameters.DataSize, 10)},
			"backup_num": {strconv.FormatInt(parameters.BackupCount, 10)}, "snap_num": {strconv.FormatInt(parameters.SnapshotCount, 10)},
		}
		for key, value := range map[string]string{"ip_group_id": parameters.IPGroupID, "ceph_id": parameters.StoragePoolID} {
			if value != "" {
				encoded.Set(key, value)
			}
		}
		return http.MethodPost, "/bms/source", encoded, map[string]string{"password": "password"}, nil
	}
	if len(request.ConfidentialInputNames) != 0 || !positiveDecimal(request.ResourceRef) || len(bytes.TrimSpace(request.Parameters)) > 0 && !bytes.Equal(bytes.TrimSpace(request.Parameters), []byte("{}")) {
		return "", "", nil, nil, errInvalidConfiguration
	}
	actions := map[resourcecap.CapabilityCode]string{
		resourcecap.CapabilityStart: "boot", resourcecap.CapabilityStop: "shutdown", resourcecap.CapabilityReboot: "reboot",
		resourcecap.CapabilitySuspend: "suspend", resourcecap.CapabilityResume: "unsuspend",
	}
	if request.Capability == resourcecap.CapabilityTerminate {
		return http.MethodDelete, "/bms/source/" + request.ResourceRef, nil, nil, nil
	}
	action, ok := actions[request.Capability]
	if !ok {
		return "", "", nil, nil, errUnsupported
	}
	return http.MethodPost, "/bms/source/" + request.ResourceRef + "/" + action, nil, nil, nil
}

func executeResourceQuery(executor confidentialoutboundcap.Service, contextID string, protocol Protocol, payload json.RawMessage) (json.RawMessage, error) {
	if protocol != ProtocolBMS {
		return nil, errUnsupported
	}
	var request resourcecap.ResourceQuery
	if json.Unmarshal(payload, &request) != nil || request.Validate() != nil || !positiveDecimal(request.ExternalResourceRef) {
		return nil, errInvalidConfiguration
	}
	response, err := businessCall(executor, contextID, 1, http.MethodGet, "/bms/source/"+request.ExternalResourceRef+"/status", nil, nil, nil)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 || !businessSucceeded(response.Data) {
		return nil, errInvalidResponse
	}
	status := responseStatus(response.Data)
	state := normalizeBMSState(status)
	data, err := json.Marshal(map[string]string{"upstream_status": status})
	if err != nil {
		return nil, errInvalidResponse
	}
	observedAt := time.Now().UTC()
	return json.Marshal(resourcecap.ResourceSnapshot{ExternalResourceRef: request.ExternalResourceRef, State: state, Data: data, ObservedAt: observedAt, Evidence: []resourcecap.Evidence{{Kind: "dcim_resource", Reference: request.ExternalResourceRef, Summary: "field-whitelisted BMS resource status observed", ObservedAt: observedAt}}})
}

func businessCall(executor confidentialoutboundcap.Service, contextID string, sequence uint8, method, path string, query, form url.Values, secretFields map[string]string) (resourcecap.ConfidentialHTTPResponse, error) {
	request := resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: method, Path: path, Query: map[string][]string(query), Form: firstValues(form), SecretFormBindings: resourcecap.NewConfidentialSecretFormBindings(secretFields), ResponseFields: safeResponseFields, UseAuthentication: true}
	encoded, err := json.Marshal(request)
	if err != nil {
		return resourcecap.ConfidentialHTTPResponse{}, errInvalidConfiguration
	}
	raw, err := executor.Execute(contextID, sequence, encoded)
	if err != nil {
		return resourcecap.ConfidentialHTTPResponse{}, err
	}
	var response resourcecap.ConfidentialHTTPResponse
	if json.Unmarshal(raw, &response) != nil {
		return response, errInvalidResponse
	}
	return response, nil
}

func firstValues(values url.Values) map[string]string {
	result := make(map[string]string, len(values))
	for key := range values {
		result[key] = values.Get(key)
	}
	return result
}

func responseObject(raw json.RawMessage) map[string]any {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var root map[string]any
	if decoder.Decode(&root) != nil {
		return nil
	}
	if data, ok := root["data"].(map[string]any); ok {
		return data
	}
	return root
}

func responseReference(raw json.RawMessage) string {
	var envelope map[string]json.RawMessage
	if json.Unmarshal(raw, &envelope) == nil {
		if data, exists := envelope["data"]; exists {
			var text string
			if json.Unmarshal(data, &text) == nil && positiveDecimal(strings.TrimSpace(text)) {
				return strings.TrimSpace(text)
			}
			var number json.Number
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.UseNumber()
			if decoder.Decode(&number) == nil && positiveDecimal(number.String()) {
				return number.String()
			}
		}
	}
	return firstScalar(responseObject(raw), "id")
}
func responseStatus(raw json.RawMessage) string { return firstScalar(responseObject(raw), "status") }
func resourceMatches(raw json.RawMessage, expected string) bool {
	return responseReference(raw) == expected
}

func normalizeBMSState(status string) resourcecap.ResourceState {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "on":
		return resourcecap.ResourceStateRunning
	case "off":
		return resourcecap.ResourceStateStopped
	default:
		return resourcecap.NormalizeResourceState(status)
	}
}

func businessSucceeded(raw json.RawMessage) bool {
	object := responseObject(raw)
	code := firstScalar(object, "code", "status")
	if code == "" {
		var root map[string]any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		_ = decoder.Decode(&root)
		code = firstScalar(root, "code", "status")
	}
	return code == "200" || strings.EqualFold(code, "success")
}

func unknownResult(request resourcecap.DynamicOperationRequest) resourcecap.OperationResult {
	return resourcecap.OperationResult{Status: resourcecap.TaskStatusUnknown, ExternalResourceRef: request.ResourceRef, Failure: &resourcecap.ProviderFailure{Code: "UPSTREAM_RESULT_UNKNOWN", Message: "DCIM operation requires manual reconciliation", ResultUnknown: true}, Evidence: []resourcecap.Evidence{{Kind: "dcim_result_unknown", Reference: request.RequestID, Summary: "BMS operation outcome could not be proven", ObservedAt: time.Now().UTC()}}}
}

func failedResult(request resourcecap.DynamicOperationRequest) resourcecap.OperationResult {
	return resourcecap.OperationResult{Status: resourcecap.TaskStatusFailed, ExternalResourceRef: request.ResourceRef, Failure: &resourcecap.ProviderFailure{Code: "UPSTREAM_REJECTED", Message: "DCIM rejected the operation"}, Evidence: []resourcecap.Evidence{{Kind: "dcim_rejection", Reference: request.RequestID, Summary: "BMS operation was explicitly rejected", ObservedAt: time.Now().UTC()}}}
}
