package finance

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
	"status", "msg", "message", "data", "products", "count", "currency", "id", "name", "type", "description",
	"config_groups", "options", "sub", "option_name", "option_type", "upstream_id", "host_data", "host_id",
	"domainstatus", "power", "des", "invoiceid", "hostid", "orderid", "user",
}

func ExecuteDynamic(executor confidentialoutboundcap.Service, contextID, method string, payload json.RawMessage) (json.RawMessage, error) {
	switch method {
	case resourcecap.DynamicProviderMethodVerifyConnection:
		if err := authenticate(executor, contextID); err != nil {
			return nil, err
		}
		return json.Marshal(resourcecap.ConnectionVerification{Verified: true, ObservedAt: time.Now().UTC()})
	case resourcecap.DynamicProviderMethodDiscoverCapabilities:
		return json.Marshal(DeclaredCapabilitySnapshot(time.Now().UTC()))
	case resourcecap.DynamicProviderMethodBuildProvisionParameters:
		var plan resourcecap.ServerProvisionPlan
		if json.Unmarshal(payload, &plan) != nil {
			return nil, errInvalidConfig
		}
		return BuildProvisionParameters(context.Background(), plan)
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
	request := resourcecap.ConfidentialHTTPRequest{
		Kind: resourcecap.ConfidentialHTTPKindAuthenticate, Method: http.MethodPost, Path: "/zjmf_api_login",
	}
	result, err := confidentialCall(executor, contextID, 1, request)
	if err != nil || !result.Authenticated {
		return errInvalidResponse
	}
	return nil
}

func executeCandidates(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	request, err := DecodeCandidateRequest(payload)
	if err != nil {
		return nil, err
	}
	calls, err := CandidateCalls(request)
	if err != nil {
		return nil, err
	}
	var productPayload, configurationPayload json.RawMessage
	sequence := uint8(2)
	for _, call := range calls {
		if call.Path == "/cart/all" && productPayload != nil || call.Path == "/cart/get_product_config" && configurationPayload != nil {
			continue
		}
		response, callErr := businessCall(executor, contextID, sequence, call.Method, call.Path, call.Query, nil)
		if callErr != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, errInvalidResponse
		}
		sequence++
		if call.Path == "/cart/all" {
			productPayload = response.Data
		} else {
			configurationPayload = response.Data
		}
	}
	fields := make([]resourceinterfaceprojectioncap.CandidateField, 0, len(request.Requests))
	for _, query := range request.Requests {
		var items []resourceinterfaceprojectioncap.Candidate
		if query.FieldKey == "upstream_product_id" {
			items, err = ProjectProductCandidates(productPayload)
		} else {
			items, err = ProjectConfigurationCandidates(query.FieldKey, request.CurrentSelections, configurationPayload)
		}
		if err != nil {
			return nil, err
		}
		fields = append(fields, resourceinterfaceprojectioncap.CandidateField{FieldKey: query.FieldKey, Items: items})
	}
	return json.Marshal(resourceinterfaceprojectioncap.CandidateResult{Fields: fields})
}

func executeCapacity(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.ProviderCapacityRequest
	if json.Unmarshal(payload, &request) != nil || request.Validate() != nil || request.Kind != resourcecap.ProviderCapacityKindPlan {
		return nil, errInvalidConfig
	}
	offset := 0
	if request.Cursor != "" {
		if !strings.HasPrefix(request.Cursor, "product:") {
			return nil, errInvalidConfig
		}
		parsed, parseErr := strconv.Atoi(strings.TrimPrefix(request.Cursor, "product:"))
		if parseErr != nil || parsed < 1 {
			return nil, errInvalidConfig
		}
		offset = parsed
	}
	response, err := businessCall(executor, contextID, 2, http.MethodGet, "/cart/all", nil, nil)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errInvalidResponse
	}
	candidates, err := ProjectProductCandidates(response.Data)
	if err != nil {
		return nil, err
	}
	total := len(candidates)
	if offset >= total && offset != 0 {
		return nil, errInvalidConfig
	}
	end := offset + request.Limit
	if end > total {
		end = total
	}
	candidates = candidates[offset:end]
	now := time.Now().UTC()
	items := make([]resourcecap.ProviderCapacityItem, 0, len(candidates))
	for _, item := range candidates {
		data, marshalErr := json.Marshal(map[string]string{"name": item.LabelZhCN})
		if marshalErr != nil {
			return nil, errInvalidResponse
		}
		items = append(items, resourcecap.ProviderCapacityItem{Kind: request.Kind, ExternalRef: item.Value, Data: data, ObservedAt: now})
	}
	next := ""
	if end < total {
		next = "product:" + strconv.Itoa(end)
	}
	page := resourcecap.ProviderCapacityPage{Items: items, NextCursor: next, Complete: end == total, Evidence: []resourcecap.Evidence{{Kind: "upstream_products", Reference: "cart/all", Summary: "field-whitelisted native finance products observed", ObservedAt: now}}}
	return json.Marshal(page)
}

func executeOperation(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.DynamicOperationRequest
	if json.Unmarshal(payload, &request) != nil || request.Validate() != nil {
		return nil, errInvalidConfig
	}
	if request.Capability == resourcecap.CapabilityProvision {
		if len(request.ConfidentialInputNames) != 1 || request.ConfidentialInputNames[0] != "downstream_token" || request.ProviderCorrelation == nil {
			return nil, errInvalidConfig
		}
		return executeProvision(executor, contextID, request)
	}
	if len(request.ConfidentialInputNames) != 0 || request.ProviderCorrelation != nil {
		return nil, errInvalidConfig
	}
	call, err := LifecycleCall(request)
	if err != nil {
		return nil, err
	}
	response, err := businessCall(executor, contextID, 2, call.Method, call.Path, call.Query, call.Form)
	if err != nil {
		return json.Marshal(unknownResult(request.RequestID, request.ResourceRef))
	}
	if !successfulBusinessResponse(response.Data) {
		return json.Marshal(failedResult(request.RequestID, request.ResourceRef))
	}
	now := time.Now().UTC()
	return json.Marshal(resourcecap.OperationResult{Status: resourcecap.TaskStatusSucceeded, ExternalResourceRef: request.ResourceRef, Evidence: []resourcecap.Evidence{{Kind: "upstream_acknowledgement", Reference: request.RequestID, Summary: "native finance lifecycle action acknowledged", ObservedAt: now}}})
}

func executeProvision(executor confidentialoutboundcap.Service, contextID string, request resourcecap.DynamicOperationRequest) (json.RawMessage, error) {
	intent := ProvisionIntent{
		DownstreamURL: request.ProviderCorrelation.DownstreamURL,
		DownstreamID:  strconv.FormatInt(request.ProviderCorrelation.DownstreamID, 10),
		Parameters:    request.Parameters, DownstreamTokenBinding: "downstream_token",
	}
	clearCall, secrets, err := BuildClearCartCall(intent)
	if err != nil {
		return nil, err
	}
	response, err := businessCallWithSecrets(executor, contextID, 2, clearCall.Method, clearCall.Path, clearCall.Query, clearCall.Form, secrets)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return json.Marshal(failedProvisionResult(request.RequestID))
	}
	clearOutcome, err := ClassifyClearOutcome(response.Data)
	if err != nil {
		return json.Marshal(failedProvisionResult(request.RequestID))
	}
	if clearOutcome.ExistingInvoiceRef != "" {
		return applyProvisionCredit(executor, contextID, 3, request.RequestID, intent, clearOutcome.ExistingInvoiceRef)
	}
	if clearOutcome.ExistingResourceRef != "" {
		return json.Marshal(succeededProvisionResult(request.RequestID, clearOutcome.ExistingResourceRef, "upstream query recovered the existing server"))
	}

	user, err := businessCall(executor, contextID, 3, http.MethodGet, "/user_info", nil, nil)
	currencyID := nestedPathString(user.Data, "user", "currency")
	if currencyID == "" {
		currencyID = nestedPathString(user.Data, "data", "user", "currency")
	}
	if err != nil || user.StatusCode < 200 || user.StatusCode >= 300 || !successfulBusinessResponse(user.Data) || !positiveDecimal(currencyID) {
		return json.Marshal(failedProvisionResult(request.RequestID))
	}
	add, _, err := BuildAddToCartCall(intent, currencyID)
	if err != nil {
		return nil, err
	}
	added, err := businessCall(executor, contextID, 4, add.Method, add.Path, add.Query, add.Form)
	if err != nil || added.StatusCode < 200 || added.StatusCode >= 300 || !successfulBusinessResponse(added.Data) {
		return json.Marshal(failedProvisionResult(request.RequestID))
	}
	settle, settleSecrets, err := BuildSettleCall(intent)
	if err != nil {
		return nil, err
	}
	settled, err := businessCallWithSecrets(executor, contextID, 5, settle.Method, settle.Path, settle.Query, settle.Form, settleSecrets)
	if err != nil {
		return json.Marshal(unknownResult(request.RequestID, ""))
	}
	if settled.StatusCode >= 500 {
		return json.Marshal(unknownResult(request.RequestID, ""))
	}
	if settled.StatusCode < 200 || settled.StatusCode >= 300 {
		return json.Marshal(failedProvisionResult(request.RequestID))
	}
	if !successfulBusinessResponse(settled.Data) {
		if findString(settled.Data, "status") == "" {
			return json.Marshal(unknownResult(request.RequestID, ""))
		}
		return json.Marshal(failedProvisionResult(request.RequestID))
	}
	settlement := SettlementResult(request.RequestID, settled.Data)
	if settlement.Status == resourcecap.TaskStatusSucceeded {
		return json.Marshal(settlement)
	}
	invoiceID := findString(settled.Data, "invoiceid")
	if !positiveDecimal(invoiceID) {
		return json.Marshal(unknownResult(request.RequestID, ""))
	}
	return applyProvisionCredit(executor, contextID, 6, request.RequestID, intent, invoiceID)
}

func applyProvisionCredit(executor confidentialoutboundcap.Service, contextID string, sequence uint8, requestID string, intent ProvisionIntent, invoiceID string) (json.RawMessage, error) {
	apply, secrets, err := BuildApplyCreditCall(intent, invoiceID)
	if err != nil {
		return nil, err
	}
	response, err := businessCallWithSecrets(executor, contextID, sequence, apply.Method, apply.Path, apply.Query, apply.Form, secrets)
	if err != nil {
		return json.Marshal(unknownResult(requestID, ""))
	}
	if response.StatusCode >= 500 {
		return json.Marshal(unknownResult(requestID, ""))
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return json.Marshal(failedProvisionResult(requestID))
	}
	result := SettlementResult(requestID, response.Data)
	if result.Status == resourcecap.TaskStatusSucceeded {
		return json.Marshal(result)
	}
	status := findString(response.Data, "status")
	if status == "1001" || status == "200" || status == "success" || status == "" {
		return json.Marshal(unknownResult(requestID, ""))
	}
	return json.Marshal(failedProvisionResult(requestID))
}

func executeResourceQuery(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage) (json.RawMessage, error) {
	var query resourcecap.ResourceQuery
	if json.Unmarshal(payload, &query) != nil {
		return nil, errInvalidConfig
	}
	call, err := ResourceQueryCall(query)
	if err != nil {
		return nil, err
	}
	response, err := businessCall(executor, contextID, 2, call.Method, call.Path, call.Query, call.Form)
	if err != nil || !successfulBusinessResponse(response.Data) {
		return nil, errInvalidResponse
	}
	upstreamState := nestedDataString(response.Data, "status")
	state := NormalizePowerState(upstreamState)
	now := time.Now().UTC()
	data, err := json.Marshal(map[string]string{"upstream_status": upstreamState})
	if err != nil {
		return nil, errInvalidResponse
	}
	return json.Marshal(resourcecap.ResourceSnapshot{ExternalResourceRef: query.ExternalResourceRef, State: state, Data: data, ObservedAt: now, Evidence: []resourcecap.Evidence{{Kind: "upstream_status", Reference: query.ExternalResourceRef, Summary: "native finance server status observed", ObservedAt: now}}})
}

func businessCall(executor confidentialoutboundcap.Service, contextID string, sequence uint8, method, path string, query url.Values, form url.Values) (resourcecap.ConfidentialHTTPResponse, error) {
	return businessCallWithSecrets(executor, contextID, sequence, method, path, query, form, nil)
}

func businessCallWithSecrets(executor confidentialoutboundcap.Service, contextID string, sequence uint8, method, path string, query url.Values, form url.Values, secrets map[string]string) (resourcecap.ConfidentialHTTPResponse, error) {
	request := resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: method, Path: path, Query: map[string][]string(query), Form: firstValues(form), SecretFormBindings: resourcecap.NewConfidentialSecretFormBindings(secrets), ResponseFields: safeResponseFields, UseAuthentication: true}
	return confidentialCall(executor, contextID, sequence, request)
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

func firstValues(values url.Values) map[string]string {
	result := make(map[string]string, len(values))
	for key := range values {
		result[key] = values.Get(key)
	}
	return result
}

func successfulBusinessResponse(raw json.RawMessage) bool {
	status := findString(raw, "status")
	return status == "200" || status == "success"
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
				switch scalar := found.(type) {
				case string:
					return scalar
				case json.Number:
					return scalar.String()
				}
			}
			if data, ok := typed["data"]; ok {
				return visit(data)
			}
		}
		return ""
	}
	return visit(root)
}

func nestedDataString(raw json.RawMessage, key string) string {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var root map[string]any
	if decoder.Decode(&root) != nil {
		return ""
	}
	data, ok := root["data"].(map[string]any)
	if !ok {
		return ""
	}
	value, exists := data[key]
	if !exists {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func nestedPathString(raw json.RawMessage, path ...string) string {
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
	switch value := current.(type) {
	case string:
		return value
	case json.Number:
		return value.String()
	default:
		return ""
	}
}

func unknownResult(requestID, resourceRef string) resourcecap.OperationResult {
	return resourcecap.OperationResult{Status: resourcecap.TaskStatusUnknown, ExternalResourceRef: resourceRef, Failure: &resourcecap.ProviderFailure{Code: "UPSTREAM_RESULT_UNKNOWN", Message: "upstream action requires manual reconciliation", ResultUnknown: true}, Evidence: []resourcecap.Evidence{{Kind: "upstream_result_unknown", Reference: requestID, Summary: "native finance action outcome could not be proven", ObservedAt: time.Now().UTC()}}}
}

func failedResult(requestID, resourceRef string) resourcecap.OperationResult {
	return resourcecap.OperationResult{Status: resourcecap.TaskStatusFailed, ExternalResourceRef: resourceRef, Failure: &resourcecap.ProviderFailure{Code: "UPSTREAM_REJECTED", Message: "upstream rejected the lifecycle action"}, Evidence: []resourcecap.Evidence{{Kind: "upstream_rejection", Reference: requestID, Summary: "native finance action was explicitly rejected", ObservedAt: time.Now().UTC()}}}
}

func failedProvisionResult(requestID string) resourcecap.OperationResult {
	result := failedResult(requestID, "")
	result.Failure.Code = "UPSTREAM_PROVISION_REJECTED"
	result.Failure.Message = "upstream did not confirm server provisioning"
	return result
}

func succeededProvisionResult(requestID, resourceRef, summary string) resourcecap.OperationResult {
	return resourcecap.OperationResult{Status: resourcecap.TaskStatusSucceeded, ExternalResourceRef: resourceRef, Evidence: []resourcecap.Evidence{{Kind: "upstream_provision", Reference: requestID, Summary: summary, ObservedAt: time.Now().UTC()}}}
}
