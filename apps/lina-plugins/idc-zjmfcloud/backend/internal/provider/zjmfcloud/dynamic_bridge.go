package zjmfcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type dynamicHTTPResponse struct {
	StatusCode    int             `json:"StatusCode"`
	Data          json.RawMessage `json:"Data"`
	Authenticated bool            `json:"Authenticated"`
}

var dynamicSafeResponseFields = []string{
	"data", "meta", "id", "cloud_id", "taskid", "task_id", "status", "current_page", "last_page", "total", "per_page",
	"name", "country", "used", "free", "locked", "ip_total", "ip_used", "ip_free", "ip_lock",
	"area", "area_id", "node_id", "node_group", "node_group_id", "type", "cpu_total", "cpu_used",
	"memory_total", "memory_used", "disk_total", "disk_used", "group", "group_id", "boot_mode", "os_type", "arch",
	"node_type", "node", "store", "show_name",
}

// ExecuteDynamicResource runs the managed connector protocol using only the
// invocation-scoped confidential HTTP proxy. Endpoint, credentials and token
// never enter this package or WASM memory.
func ExecuteDynamicResource(
	executor confidentialoutboundcap.Service,
	contextID string,
	method string,
	payload json.RawMessage,
) (json.RawMessage, error) {
	if method == resourcecap.DynamicProviderMethodVerifyConnection {
		if err := dynamicAuthenticate(executor, contextID); err != nil {
			return nil, err
		}
		return json.Marshal(resourcecap.ConnectionVerification{Verified: true, ObservedAt: time.Now().UTC()})
	}
	if method == resourcecap.DynamicProviderMethodDiscoverCapabilities {
		return json.Marshal(DeclaredCapabilitySnapshot(time.Now().UTC()))
	}
	if method == resourcecap.DynamicProviderMethodBuildProvisionParameters {
		var plan resourcecap.ServerProvisionPlan
		if err := json.Unmarshal(payload, &plan); err != nil {
			return nil, errInvalidConfig
		}
		return BuildProvisionParameters(context.Background(), plan)
	}
	if err := dynamicAuthenticate(executor, contextID); err != nil {
		return nil, err
	}
	switch method {
	case resourcecap.DynamicProviderMethodConfigurationCandidates:
		return dynamicConfigurationCandidates(executor, contextID, payload)
	case resourcecap.DynamicProviderMethodListProviderCapacity:
		return dynamicListProviderCapacity(executor, contextID, payload)
	case resourcecap.DynamicProviderMethodQueryTask:
		return dynamicQueryTask(executor, contextID, payload)
	case resourcecap.DynamicProviderMethodQueryResource:
		return dynamicQueryResource(executor, contextID, payload)
	case resourcecap.DynamicProviderMethodExecute:
		return dynamicExecute(executor, contextID, payload)
	default:
		return nil, errUnsupported
	}
}

func dynamicAuthenticate(executor confidentialoutboundcap.Service, contextID string) error {
	request := resourcecap.ConfidentialHTTPRequest{
		Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/login",
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return errInvalidConfig
	}
	result, err := executor.Execute(contextID, 1, payload)
	if err != nil {
		return err
	}
	var response dynamicHTTPResponse
	if json.Unmarshal(result, &response) != nil || !response.Authenticated {
		return errAuthentication
	}
	return nil
}

func dynamicCall(executor confidentialoutboundcap.Service, contextID string, sequence uint8, method, path string, query url.Values, form url.Values, secretFormFields ...map[string]string) (dynamicHTTPResponse, error) {
	request := resourcecap.ConfidentialHTTPRequest{
		Kind: resourcecap.ConfidentialHTTPKindRequest, Method: method, Path: path,
		Query: map[string][]string(query), Form: firstFormValues(form), ResponseFields: dynamicSafeResponseFields, UseAuthentication: true,
	}
	if len(secretFormFields) == 1 {
		request.SecretFormBindings = resourcecap.NewConfidentialSecretFormBindings(secretFormFields[0])
	} else if len(secretFormFields) > 1 {
		return dynamicHTTPResponse{}, errInvalidConfig
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return dynamicHTTPResponse{}, errInvalidConfig
	}
	result, err := executor.Execute(contextID, sequence, payload)
	if err != nil {
		return dynamicHTTPResponse{}, err
	}
	var response dynamicHTTPResponse
	if json.Unmarshal(result, &response) != nil {
		return dynamicHTTPResponse{}, errInvalidResponse
	}
	return response, nil
}

func dynamicListProviderCapacity(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.ProviderCapacityRequest
	if json.Unmarshal(payload, &request) != nil || request.Validate() != nil {
		return nil, errInvalidConfig
	}
	filters := bytes.TrimSpace(request.Filters)
	if len(filters) != 0 && !bytes.Equal(filters, []byte(`{}`)) {
		return nil, errUnsupported
	}
	if request.Kind == resourcecap.ProviderCapacityKindPlan {
		return dynamicListProvisionCandidates(executor, contextID, request)
	}
	projection, ok := providerCapacityProjections[request.Kind]
	if !ok {
		return nil, errUnsupported
	}
	pageNumber, err := decodeCursor(request.Cursor, request.Kind, request.Limit)
	if err != nil {
		return nil, err
	}
	query := url.Values{"page": {strconv.Itoa(pageNumber)}, "per_page": {strconv.Itoa(request.Limit)}}
	response, err := dynamicCall(executor, contextID, 2, http.MethodGet, projection.route, query, nil)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, readStatusError(response.StatusCode)
	}
	rawItems, meta, err := decodeObjectPage(response.Data)
	if err != nil || len(rawItems) > request.Limit {
		return nil, errInvalidResponse
	}
	observedAt := time.Now().UTC()
	items := make([]resourcecap.ProviderCapacityItem, 0, len(rawItems))
	for _, rawItem := range rawItems {
		reference, exists := firstReference(rawItem, projection.idKeys...)
		if !exists {
			return nil, errInvalidResponse
		}
		data, projectErr := projectScalars(rawItem, projection.fields)
		if projectErr != nil {
			return nil, projectErr
		}
		items = append(items, resourcecap.ProviderCapacityItem{Kind: request.Kind, ExternalRef: reference, Data: data, ObservedAt: observedAt})
	}
	complete, next, err := pageBoundary(request.Kind, pageNumber, request.Limit, len(items), meta)
	if err != nil {
		return nil, err
	}
	return json.Marshal(resourcecap.ProviderCapacityPage{Items: items, NextCursor: next, Complete: complete, Evidence: []resourcecap.Evidence{{Kind: "upstream_page", Reference: string(request.Kind) + ":" + strconv.Itoa(pageNumber), Summary: "field-whitelisted upstream capacity page observed", ObservedAt: observedAt}}})
}

func dynamicListProvisionCandidates(executor confidentialoutboundcap.Service, contextID string, request resourcecap.ProviderCapacityRequest) (json.RawMessage, error) {
	pageNumber, expectedFingerprint, err := decodeSnapshotCursor(request.Cursor, request.Kind, request.Limit)
	if err != nil {
		return nil, err
	}
	graph, err := dynamicCall(executor, contextID, 2, http.MethodGet, "/clouds/create_page", nil, nil)
	if err != nil || graph.StatusCode < 200 || graph.StatusCode >= 300 {
		if err != nil {
			return nil, err
		}
		return nil, readStatusError(graph.StatusCode)
	}
	graphRoot, err := decodeObject(graph.Data)
	if err != nil {
		return nil, errInvalidResponse
	}
	candidates, err := creationCandidates(graphRoot)
	if err != nil {
		return nil, err
	}
	defer clearCreationCandidates(candidates)
	stores, err := dynamicCall(executor, contextID, 3, http.MethodGet, "/stores", url.Values{"enable": {"1"}, "list_type": {"all"}}, nil)
	if err != nil || stores.StatusCode < 200 || stores.StatusCode >= 300 {
		if err != nil {
			return nil, err
		}
		return nil, readStatusError(stores.StatusCode)
	}
	storesRoot, err := decodeObject(stores.Data)
	if err != nil || appendGlobalStorageCandidates(&candidates, storesRoot) != nil {
		return nil, errInvalidResponse
	}
	sort.Slice(candidates, func(left, right int) bool { return candidates[left].reference < candidates[right].reference })
	fingerprint, err := creationCandidateFingerprint(candidates)
	if err != nil || (expectedFingerprint != "" && expectedFingerprint != fingerprint) {
		return nil, errInvalidResponse
	}
	start := (pageNumber - 1) * request.Limit
	if start > len(candidates) || (start == len(candidates) && pageNumber > 1) {
		return nil, errInvalidResponse
	}
	end := start + request.Limit
	if end > len(candidates) {
		end = len(candidates)
	}
	observedAt := time.Now().UTC()
	items := make([]resourcecap.ProviderCapacityItem, 0, end-start)
	for _, candidate := range candidates[start:end] {
		items = append(items, resourcecap.ProviderCapacityItem{Kind: request.Kind, ExternalRef: candidate.reference, Data: append(json.RawMessage(nil), candidate.data...), ObservedAt: observedAt})
	}
	complete := end == len(candidates)
	next := ""
	if !complete {
		next = encodeSnapshotCursor(request.Kind, request.Limit, pageNumber+1, fingerprint)
	}
	page := resourcecap.ProviderCapacityPage{Items: items, NextCursor: next, Complete: complete, Evidence: []resourcecap.Evidence{{Kind: "upstream_creation_capacity", Reference: string(request.Kind) + ":" + strconv.Itoa(pageNumber), Summary: "field-whitelisted storage candidates projected from bounded creation sources", ObservedAt: observedAt}}}
	if page.Validate(request) != nil {
		return nil, errInvalidResponse
	}
	return json.Marshal(page)
}

func dynamicQueryTask(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var query resourcecap.TaskQuery
	if json.Unmarshal(payload, &query) != nil || query.Validate() != nil || !validNumericReference(query.ExternalTaskRef) {
		return nil, errInvalidConfig
	}
	response, err := dynamicCall(executor, contextID, 2, http.MethodGet, "/tasks/"+query.ExternalTaskRef, nil, nil)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, readStatusError(response.StatusCode)
	}
	object, err := decodeObject(response.Data)
	if err != nil {
		return nil, errInvalidResponse
	}
	object = unwrapObject(object)
	status, ok := metaInteger(object, "status")
	if !ok {
		return nil, errInvalidResponse
	}
	states := map[int]resourcecap.TaskStatus{0: resourcecap.TaskStatusAccepted, 1: resourcecap.TaskStatusRunning, 2: resourcecap.TaskStatusSucceeded, 3: resourcecap.TaskStatusFailed, 4: resourcecap.TaskStatusTerminated, 5: resourcecap.TaskStatusCanceled}
	state, ok := states[status]
	if !ok {
		state = resourcecap.TaskStatusUnknown
	}
	result := resourcecap.OperationResult{Status: state, ExternalTaskRef: query.ExternalTaskRef, ExternalResourceRef: query.ResourceRef, Evidence: []resourcecap.Evidence{{Kind: "upstream_task", Reference: query.ExternalTaskRef, Summary: "numeric upstream task status observed", ObservedAt: time.Now().UTC()}}}
	if state == resourcecap.TaskStatusFailed || state == resourcecap.TaskStatusUnknown {
		result.Failure = &resourcecap.ProviderFailure{Code: "UPSTREAM_TASK_FAILED", Message: "upstream task requires reconciliation", ResultUnknown: state == resourcecap.TaskStatusUnknown}
	}
	return json.Marshal(result)
}

func dynamicQueryResource(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var query resourcecap.ResourceQuery
	if json.Unmarshal(payload, &query) != nil || query.Validate() != nil || !validNumericReference(query.ExternalResourceRef) {
		return nil, errInvalidConfig
	}
	response, err := dynamicCall(executor, contextID, 2, http.MethodGet, "/clouds/"+query.ExternalResourceRef, nil, nil)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, readStatusError(response.StatusCode)
	}
	object, err := decodeObject(response.Data)
	if err != nil {
		return nil, errInvalidResponse
	}
	object = unwrapObject(object)
	status, ok := stringScalar(object["status"])
	if !ok {
		status = "unknown"
	}
	state, normalized := normalizeUpstreamResourceState(status)
	data, err := json.Marshal(struct {
		Observed       bool   `json:"observed"`
		UpstreamStatus string `json:"upstream_status"`
	}{Observed: true, UpstreamStatus: normalized})
	if err != nil {
		return nil, errInvalidResponse
	}
	observedAt := time.Now().UTC()
	return json.Marshal(resourcecap.ResourceSnapshot{ExternalResourceRef: query.ExternalResourceRef, State: state, Data: data, ObservedAt: observedAt, Evidence: []resourcecap.Evidence{{Kind: "upstream_resource", Reference: query.ExternalResourceRef, Summary: "field-whitelisted upstream resource status observed", ObservedAt: observedAt}}})
}

func dynamicExecute(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.DynamicOperationRequest
	if json.Unmarshal(payload, &request) != nil || request.Validate() != nil {
		return nil, errInvalidConfig
	}
	operation := resourcecap.OperationRequest{RequestID: request.RequestID, Capability: request.Capability, ResourceRef: request.ResourceRef, Parameters: request.Parameters}
	call, secretFields, err := dynamicOperationCall(operation, request.ConfidentialInputNames)
	if err != nil {
		return nil, err
	}
	sequence := uint8(2)
	if request.ResourceRef != "" {
		owned, ownershipErr := dynamicCall(executor, contextID, sequence, http.MethodGet, "/clouds/"+request.ResourceRef, nil, nil)
		if ownershipErr != nil {
			return nil, ownershipErr
		}
		if owned.StatusCode < 200 || owned.StatusCode >= 300 || !dynamicResourceMatches(owned.Data, request.ResourceRef) {
			return nil, errInvalidResponse
		}
		sequence++
	}
	response, err := dynamicCall(executor, contextID, sequence, call.method, call.route, call.query, call.form, secretFields)
	if err != nil {
		return json.Marshal(dynamicUnknownOperationResult(request))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, readStatusError(response.StatusCode)
	}
	object, _ := decodeObject(response.Data)
	object = unwrapObject(object)
	taskRef, _ := firstReference(object, "taskid", "task_id", "id")
	status := resourcecap.TaskStatusSucceeded
	if taskRef != "" {
		status = resourcecap.TaskStatusAccepted
	}
	result := resourcecap.OperationResult{Status: status, ExternalTaskRef: taskRef, ExternalResourceRef: request.ResourceRef, Evidence: []resourcecap.Evidence{{Kind: "upstream_response", Reference: request.RequestID, Summary: "field-whitelisted operation acknowledgement observed", ObservedAt: time.Now().UTC()}}}
	return json.Marshal(result)
}

func dynamicOperationCall(operation resourcecap.OperationRequest, confidentialNames []string) (operationCall, map[string]string, error) {
	exactNames := func(expected ...string) bool {
		if len(confidentialNames) != len(expected) {
			return false
		}
		seen := make(map[string]struct{}, len(confidentialNames))
		for _, name := range confidentialNames {
			seen[name] = struct{}{}
		}
		for _, name := range expected {
			if _, ok := seen[name]; !ok {
				return false
			}
		}
		return true
	}
	switch operation.Capability {
	case resourcecap.CapabilityResetPassword:
		parameters, err := decodeParameters(operation.Parameters)
		if err != nil || len(parameters) != 0 || !validNumericReference(operation.ResourceRef) || !exactNames(resetPasswordBinding) {
			return operationCall{}, nil, errInvalidConfig
		}
		return operationCall{method: http.MethodPut, route: "/clouds/" + operation.ResourceRef + "/password", requiresTask: true}, map[string]string{"password": resetPasswordBinding}, nil
	case resourcecap.CapabilityProvision:
		if !exactNames(resetPasswordBinding) {
			return operationCall{}, nil, errInvalidConfig
		}
		call, err := buildOperationCall(operation)
		return call, map[string]string{"rootpass": resetPasswordBinding}, err
	case resourcecap.CapabilityEnterRescue:
		if !exactNames(rescuePasswordBinding) {
			return operationCall{}, nil, errInvalidConfig
		}
		call, err := buildOperationCall(operation)
		return call, map[string]string{"temp_pass": rescuePasswordBinding}, err
	default:
		if !exactNames() {
			return operationCall{}, nil, errInvalidConfig
		}
		call, err := buildOperationCall(operation)
		return call, nil, err
	}
}

func dynamicResourceMatches(payload json.RawMessage, expected string) bool {
	object, err := decodeObject(payload)
	if err != nil {
		return false
	}
	object = unwrapObject(object)
	actual, ok := firstReference(object, "id", "cloud_id")
	return ok && actual == expected
}

func dynamicUnknownOperationResult(request resourcecap.DynamicOperationRequest) resourcecap.OperationResult {
	return resourcecap.OperationResult{
		Status: resourcecap.TaskStatusUnknown, ExternalResourceRef: request.ResourceRef,
		Failure:  &resourcecap.ProviderFailure{Code: "UPSTREAM_RESULT_UNKNOWN", Message: "upstream operation requires reconciliation", ResultUnknown: true},
		Evidence: []resourcecap.Evidence{{Kind: "upstream_result_unknown", Reference: request.RequestID, Summary: "operation submission outcome could not be proven", ObservedAt: time.Now().UTC()}},
	}
}

func firstFormValues(values url.Values) map[string]string {
	result := make(map[string]string, len(values))
	for key := range values {
		result[key] = values.Get(key)
	}
	return result
}
