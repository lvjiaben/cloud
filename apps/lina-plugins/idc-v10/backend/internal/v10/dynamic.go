package v10

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var (
	errInvalidRequest  = errors.New("V10 connector request is invalid")
	errInvalidResponse = errors.New("V10 supplier response is invalid")
	errUnsupported     = errors.New("V10 connector method is unsupported")
)

var safeResponseFields = []string{
	"status", "code", "msg", "message", "data", "list", "product", "id", "name", "description",
	"price", "cycle", "currency_code", "order_id", "host_ids", "amount", "domainstatus", "dedicatedip",
	"assignedips", "regdate", "nextduedate", "upstream_status", "content", "product_name",
}

func ExecuteDynamic(executor confidentialoutboundcap.Service, contextID, method string, payload json.RawMessage) (json.RawMessage, error) {
	if executor == nil || strings.TrimSpace(contextID) == "" {
		return nil, errInvalidRequest
	}
	switch method {
	case resourcecap.DynamicProviderMethodVerifyConnection:
		if err := authenticate(executor, contextID); err != nil {
			return nil, err
		}
		return json.Marshal(resourcecap.ConnectionVerification{Verified: true, ObservedAt: time.Now().UTC()})
	case resourcecap.DynamicProviderMethodDiscoverCapabilities:
		return json.Marshal(capabilities(time.Now().UTC()))
	case resourcecap.DynamicProviderMethodBuildProvisionParameters:
		var plan resourcecap.ServerProvisionPlan
		if strictDecode(payload, &plan) != nil || plan.Validate() != nil {
			return nil, errInvalidPlan
		}
		return BuildProvisionParameters(plan)
	}
	if err := authenticate(executor, contextID); err != nil {
		return nil, err
	}
	switch method {
	case resourcecap.DynamicProviderMethodListCatalog:
		return executeCatalogList(executor, contextID, payload)
	case resourcecap.DynamicProviderMethodReadCatalogProduct:
		return executeCatalogDetail(executor, contextID, payload)
	case resourcecap.DynamicProviderMethodConfigurationCandidates:
		return executeCandidates(executor, contextID, payload)
	case resourcecap.DynamicProviderMethodListProviderCapacity:
		return executeCapacity(executor, contextID, payload)
	case resourcecap.DynamicProviderMethodExecute:
		return executeOperation(executor, contextID, payload)
	case resourcecap.DynamicProviderMethodQueryResource:
		return executeResourceQuery(executor, contextID, payload)
	case resourcecap.DynamicProviderMethodQueryTask:
		return nil, errUnsupported
	default:
		return nil, errUnsupported
	}
}

func authenticate(executor confidentialoutboundcap.Service, contextID string) error {
	response, err := confidentialCall(executor, contextID, 1, resourcecap.ConfidentialHTTPRequest{
		Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/api/v1/auth",
		CredentialFields: map[string]string{"username": "username", "password": "password"},
	})
	if err != nil || !response.Authenticated {
		return errInvalidResponse
	}
	return nil
}

func executeCapacity(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.ProviderCapacityRequest
	if json.Unmarshal(payload, &request) != nil || request.Validate() != nil || request.Kind != resourcecap.ProviderCapacityKindPlan {
		return nil, errInvalidRequest
	}
	response, err := businessCall(executor, contextID, 2, http.MethodGet, "/api/v1/product", nil, nil, nil)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errInvalidResponse
	}
	products, err := projectProducts(response.Data)
	if err != nil {
		return nil, err
	}
	offset := 0
	if request.Cursor != "" {
		if !strings.HasPrefix(request.Cursor, "product:") {
			return nil, errInvalidRequest
		}
		offset, err = strconv.Atoi(strings.TrimPrefix(request.Cursor, "product:"))
		if err != nil || offset < 1 || offset >= len(products) {
			return nil, errInvalidRequest
		}
	}
	end := offset + request.Limit
	if end > len(products) {
		end = len(products)
	}
	now := time.Now().UTC()
	items := make([]resourcecap.ProviderCapacityItem, 0, end-offset)
	for _, product := range products[offset:end] {
		data, marshalErr := json.Marshal(map[string]string{"name": product.LabelZhCN})
		if marshalErr != nil {
			return nil, errInvalidResponse
		}
		items = append(items, resourcecap.ProviderCapacityItem{Kind: request.Kind, ExternalRef: product.Value, Data: data, ObservedAt: now})
	}
	next := ""
	if end < len(products) {
		next = "product:" + strconv.Itoa(end)
	}
	return json.Marshal(resourcecap.ProviderCapacityPage{Items: items, NextCursor: next, Complete: next == "", Evidence: []resourcecap.Evidence{{Kind: "upstream_products", Reference: "api/v1/product", Summary: "V10 supplier products observed", ObservedAt: now}}})
}

func executeOperation(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.DynamicOperationRequest
	if json.Unmarshal(payload, &request) != nil || request.Validate() != nil {
		return nil, errInvalidRequest
	}
	if request.Capability == resourcecap.CapabilityProvision {
		return executeProvision(executor, contextID, request)
	}
	if len(request.ConfidentialInputNames) != 0 || request.ProviderCorrelation != nil || !positiveDecimal(request.ResourceRef) {
		return nil, errInvalidRequest
	}
	var path string
	form := map[string]string{}
	switch request.Capability {
	case resourcecap.CapabilitySuspend:
		path = "/console/v1/host/" + request.ResourceRef + "/module/suspend"
		form = map[string]string{"suspend_type": "downstream", "suspend_reason": "downstream request"}
	case resourcecap.CapabilityResume:
		path = "/console/v1/host/" + request.ResourceRef + "/module/unsuspend"
	case resourcecap.CapabilityTerminate:
		path = "/console/v1/refund"
		form = map[string]string{"host_id": request.ResourceRef, "suspend_reason": "downstream cancellation", "type": "Immediate"}
	default:
		return nil, errUnsupported
	}
	response, err := businessCall(executor, contextID, 2, http.MethodPost, path, nil, form, nil)
	if err != nil {
		return json.Marshal(unknownResult(request.RequestID, request.ResourceRef))
	}
	if !successful(response) {
		return json.Marshal(failedResult(request.RequestID, request.ResourceRef))
	}
	return json.Marshal(succeededResult(request.RequestID, request.ResourceRef, "V10 supplier action acknowledged"))
}

func executeProvision(executor confidentialoutboundcap.Service, contextID string, request resourcecap.DynamicOperationRequest) (json.RawMessage, error) {
	if len(request.ConfidentialInputNames) != 1 || request.ConfidentialInputNames[0] != "downstream_token" || request.ProviderCorrelation == nil {
		return nil, errInvalidRequest
	}
	var parameters struct {
		PID                string                     `json:"pid"`
		BillingCycle       string                     `json:"billingcycle"`
		ConfigOption       map[string]json.RawMessage `json:"configoption"`
		DownstreamClientID string                     `json:"downstream_client_id"`
		CreateSubAccount   int                        `json:"create_sub_account"`
		Hostname           string                     `json:"hostname"`
	}
	if strictDecode(request.Parameters, &parameters) != nil || !positiveDecimal(parameters.PID) ||
		!positiveDecimal(parameters.DownstreamClientID) || parameters.BillingCycle == "" || len(parameters.ConfigOption) == 0 {
		return nil, errInvalidRequest
	}
	cleanupForm := map[string]string{
		"downstream_url":       request.ProviderCorrelation.DownstreamURL,
		"downstream_host_id":   strconv.FormatInt(request.ProviderCorrelation.DownstreamID, 10),
		"downstream_client_id": parameters.DownstreamClientID,
	}
	response, err := businessCall(executor, contextID, 2, http.MethodDelete, "/console/v1/cart", nil, cleanupForm, map[string]string{"downstream_token": "downstream_token"})
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return json.Marshal(unknownResult(request.RequestID, ""))
	}
	if orderID := nestedString(response.Data, "data", "order_id"); positiveDecimal(orderID) {
		return payOrder(executor, contextID, 3, request.RequestID, orderID, "")
	}
	cart := map[string]string{"product_id": parameters.PID, "qty": "1"}
	for optionID, raw := range parameters.ConfigOption {
		value := scalar(raw)
		if !positiveDecimal(optionID) || value == "" || len(value) > 256 {
			return nil, errInvalidRequest
		}
		cart["config_options[configoption]["+optionID+"]"] = value
	}
	response, err = businessCall(executor, contextID, 3, http.MethodPost, "/console/v1/cart", nil, cart, nil)
	if err != nil || !successful(response) {
		return json.Marshal(failedResult(request.RequestID, ""))
	}
	settle := map[string]string{
		"positions[0]": "0", "downstream_url": request.ProviderCorrelation.DownstreamURL,
		"downstream_host_id":   strconv.FormatInt(request.ProviderCorrelation.DownstreamID, 10),
		"downstream_client_id": parameters.DownstreamClientID,
	}
	response, err = businessCall(executor, contextID, 4, http.MethodPost, "/console/v1/cart/settle", nil, settle, map[string]string{"downstream_token": "downstream_token"})
	if err != nil || response.StatusCode >= 500 {
		return json.Marshal(unknownResult(request.RequestID, ""))
	}
	if !successful(response) {
		return json.Marshal(failedResult(request.RequestID, ""))
	}
	hostID := firstArrayScalar(response.Data, "data", "host_ids")
	orderID := nestedString(response.Data, "data", "order_id")
	amount := nestedString(response.Data, "data", "amount")
	if amount == "0" && positiveDecimal(hostID) {
		return json.Marshal(succeededResult(request.RequestID, hostID, "V10 supplier provisioned the server"))
	}
	if !positiveDecimal(orderID) {
		return json.Marshal(unknownResult(request.RequestID, hostID))
	}
	return payOrder(executor, contextID, 5, request.RequestID, orderID, hostID)
}

func payOrder(executor confidentialoutboundcap.Service, contextID string, sequence uint8, requestID, orderID, hostID string) (json.RawMessage, error) {
	response, err := businessCall(executor, contextID, sequence, http.MethodPost, "/console/v1/pay", nil, map[string]string{"id": orderID, "gateway": "credit"}, nil)
	if err != nil || response.StatusCode >= 500 {
		return json.Marshal(unknownResult(requestID, hostID))
	}
	if !successful(response) {
		return json.Marshal(failedResult(requestID, hostID))
	}
	if !positiveDecimal(hostID) {
		hostID = firstArrayScalar(response.Data, "data", "host_ids")
	}
	if !positiveDecimal(hostID) {
		return json.Marshal(unknownResult(requestID, ""))
	}
	return json.Marshal(succeededResult(requestID, hostID, "V10 supplier payment and provisioning were acknowledged"))
}

func executeResourceQuery(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.ResourceQuery
	if json.Unmarshal(payload, &request) != nil || request.Validate() != nil || !positiveDecimal(request.ExternalResourceRef) {
		return nil, errInvalidRequest
	}
	response, err := businessCall(executor, contextID, 2, http.MethodGet, "/console/v1/host/"+request.ExternalResourceRef, nil, nil, nil)
	if err != nil || !successful(response) {
		return nil, errInvalidResponse
	}
	upstream := nestedString(response.Data, "data", "domainstatus")
	now := time.Now().UTC()
	data, err := json.Marshal(map[string]string{"upstream_status": upstream})
	if err != nil {
		return nil, errInvalidResponse
	}
	return json.Marshal(resourcecap.ResourceSnapshot{ExternalResourceRef: request.ExternalResourceRef, State: normalizeState(upstream), Data: data, ObservedAt: now, Evidence: []resourcecap.Evidence{{Kind: "upstream_status", Reference: request.ExternalResourceRef, Summary: "V10 server status observed", ObservedAt: now}}})
}

func businessCall(executor confidentialoutboundcap.Service, contextID string, sequence uint8, method, path string, query url.Values, form map[string]string, secrets map[string]string) (resourcecap.ConfidentialHTTPResponse, error) {
	return confidentialCall(executor, contextID, sequence, resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: method, Path: path, Query: map[string][]string(query), Form: form, SecretFormFields: secrets, ResponseFields: safeResponseFields, UseAuthentication: true})
}

func confidentialCall(executor confidentialoutboundcap.Service, contextID string, sequence uint8, request resourcecap.ConfidentialHTTPRequest) (resourcecap.ConfidentialHTTPResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return resourcecap.ConfidentialHTTPResponse{}, err
	}
	result, err := executor.Execute(contextID, sequence, payload)
	if err != nil {
		return resourcecap.ConfidentialHTTPResponse{}, err
	}
	var response resourcecap.ConfidentialHTTPResponse
	if json.Unmarshal(result, &response) != nil {
		return response, errInvalidResponse
	}
	return response, nil
}

func successful(response resourcecap.ConfidentialHTTPResponse) bool {
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false
	}
	status := findString(response.Data, "status")
	return status == "200" || strings.EqualFold(status, "success")
}

func findString(raw json.RawMessage, key string) string {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var root any
	if decoder.Decode(&root) != nil {
		return ""
	}
	var visit func(any) string
	visit = func(value any) string {
		switch typed := value.(type) {
		case map[string]any:
			if found, ok := typed[key]; ok {
				if scalar := anyScalar(found); scalar != "" {
					return scalar
				}
			}
			for _, child := range typed {
				if found := visit(child); found != "" {
					return found
				}
			}
		case []any:
			for _, child := range typed {
				if found := visit(child); found != "" {
					return found
				}
			}
		}
		return ""
	}
	return visit(root)
}

func nestedString(raw json.RawMessage, path ...string) string {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var current any
	if decoder.Decode(&current) != nil {
		return ""
	}
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = object[key]
		if !ok {
			return ""
		}
	}
	return anyScalar(current)
}

func firstArrayScalar(raw json.RawMessage, path ...string) string {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var current any
	if decoder.Decode(&current) != nil {
		return ""
	}
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[key]
	}
	items, ok := current.([]any)
	if !ok || len(items) == 0 {
		return ""
	}
	return anyScalar(items[0])
}

func anyScalar(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func normalizeState(value string) resourcecap.ResourceState {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "active", "running", "on":
		return resourcecap.ResourceStateActive
	case "pending":
		return resourcecap.ResourceStatePending
	case "suspended":
		return resourcecap.ResourceStateSuspended
	case "stopped", "off":
		return resourcecap.ResourceStateStopped
	case "cancelled", "canceled", "terminated", "deleted":
		return resourcecap.ResourceStateTerminated
	case "failed", "error":
		return resourcecap.ResourceStateFailed
	default:
		return resourcecap.ResourceStateUnknown
	}
}

func succeededResult(requestID, resourceRef, summary string) resourcecap.OperationResult {
	return resourcecap.OperationResult{Status: resourcecap.TaskStatusSucceeded, ExternalResourceRef: resourceRef, Evidence: []resourcecap.Evidence{{Kind: "upstream_acknowledgement", Reference: requestID, Summary: summary, ObservedAt: time.Now().UTC()}}}
}

func failedResult(requestID, resourceRef string) resourcecap.OperationResult {
	return resourcecap.OperationResult{Status: resourcecap.TaskStatusFailed, ExternalResourceRef: resourceRef, Failure: &resourcecap.ProviderFailure{Code: "UPSTREAM_REJECTED", Message: "V10 supplier rejected the operation"}, Evidence: []resourcecap.Evidence{{Kind: "upstream_rejection", Reference: requestID, Summary: "V10 supplier explicitly rejected the operation", ObservedAt: time.Now().UTC()}}}
}

func unknownResult(requestID, resourceRef string) resourcecap.OperationResult {
	return resourcecap.OperationResult{Status: resourcecap.TaskStatusUnknown, ExternalResourceRef: resourceRef, Failure: &resourcecap.ProviderFailure{Code: "UPSTREAM_RESULT_UNKNOWN", Message: "V10 supplier result requires reconciliation", ResultUnknown: true}, Evidence: []resourcecap.Evidence{{Kind: "upstream_result_unknown", Reference: requestID, Summary: "V10 supplier result could not be proven", ObservedAt: time.Now().UTC()}}}
}
