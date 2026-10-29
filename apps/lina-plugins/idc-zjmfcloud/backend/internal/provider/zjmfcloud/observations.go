// This file projects upstream task and resource observations without raw payload retention.

package zjmfcloud

import (
	"context"
	"encoding/json"
	"net/http"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// QueryTask observes one upstream task without inferring success from messages.
func (p *Provider) QueryTask(
	ctx context.Context,
	query resourcecap.TaskQuery,
) (*resourcecap.OperationResult, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if !validNumericReference(query.ExternalTaskRef) ||
		(query.ResourceRef != "" && !validNumericReference(query.ResourceRef)) {
		return nil, errInvalidConfig
	}
	response, err := p.authenticatedRequest(
		ctx,
		http.MethodGet,
		"/tasks/"+query.ExternalTaskRef,
		nil,
		nil,
	)
	if err != nil {
		return nil, normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		if response.statusCode == http.StatusRequestTimeout ||
			response.statusCode == http.StatusTooEarly ||
			response.statusCode == http.StatusTooManyRequests ||
			response.statusCode >= http.StatusInternalServerError {
			return nil, readStatusError(response.statusCode)
		}
		return p.unknownTaskResult(query, "upstream task could not be observed")
	}
	object, err := decodeObject(response.body)
	if err != nil {
		return p.unknownTaskResult(query, "upstream task response was not usable")
	}
	object = unwrapObject(object)
	defer clearRawObject(object)
	observedTaskReference, ok := firstReference(object, "id")
	if !ok || observedTaskReference != query.ExternalTaskRef {
		return nil, errInvalidResponse
	}
	status, ok := metaInteger(object, "status")
	if !ok {
		return p.unknownTaskResult(query, "upstream task status was not recognized")
	}
	resourceReference := query.ResourceRef
	observedResourceReference, hasResourceReference := firstReference(object, "hostid", "host_id")
	if resourceReference != "" && (!hasResourceReference || observedResourceReference != resourceReference) {
		return nil, errInvalidResponse
	}
	if resourceReference == "" && hasResourceReference {
		resourceReference = observedResourceReference
	}

	result := &resourcecap.OperationResult{
		ExternalTaskRef:     query.ExternalTaskRef,
		ExternalResourceRef: resourceReference,
		Evidence: []resourcecap.Evidence{{
			Kind:       "upstream_task",
			Reference:  query.ExternalTaskRef,
			Summary:    "numeric upstream task status observed",
			ObservedAt: p.now(),
		}},
	}
	if progress, exists := metaInteger(object, "progress"); exists && progress <= 100 {
		result.Progress = &progress
	}
	switch status {
	case 0:
		result.Status = resourcecap.TaskStatusAccepted
	case 1:
		result.Status = resourcecap.TaskStatusRunning
	case 2:
		result.Status = resourcecap.TaskStatusSucceeded
	case 3:
		result.Status = resourcecap.TaskStatusFailed
		result.Failure = &resourcecap.ProviderFailure{
			Code: "UPSTREAM_TASK_FAILED", Message: "upstream task reported failure",
		}
	case 4:
		result.Status = resourcecap.TaskStatusTerminated
	case 5:
		result.Status = resourcecap.TaskStatusCanceled
	default:
		return p.unknownTaskResult(query, "upstream task status was outside the verified matrix")
	}
	if err := result.Validate(); err != nil {
		return p.unknownTaskResult(query, "upstream task projection failed validation")
	}
	return result, nil
}

// QueryResource observes the documented full instance endpoint so accepted
// changes can be reconciled. Only validated runtime, address and guest-display
// fields survive projection; provider credentials never enter the snapshot.
func (p *Provider) QueryResource(
	ctx context.Context,
	query resourcecap.ResourceQuery,
) (*resourcecap.ResourceSnapshot, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if !validNumericReference(query.ExternalResourceRef) {
		return nil, errInvalidConfig
	}
	response, err := p.authenticatedRequest(
		ctx,
		http.MethodGet,
		"/clouds/"+query.ExternalResourceRef,
		nil,
		nil,
	)
	if err != nil {
		return nil, normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode == http.StatusNotFound {
		return p.resourceSnapshot(
			query.ExternalResourceRef,
			resourcecap.ResourceStateUnknown,
			"unknown",
			nil,
			false,
		)
	}
	if response.statusCode < 200 || response.statusCode >= 300 {
		return nil, readStatusError(response.statusCode)
	}
	object, err := decodeObject(response.body)
	if err != nil {
		return nil, errInvalidResponse
	}
	object = unwrapObject(object)
	defer clearRawObject(object)
	status, ok := stringScalar(object["status"])
	if !ok {
		return p.resourceSnapshot(query.ExternalResourceRef, resourcecap.ResourceStateUnknown, "unknown", nil, true)
	}
	ipCount := normalizedIPCount(object)
	state, normalizedStatus := normalizeUpstreamResourceState(status)
	snapshot, snapshotErr := p.resourceSnapshot(query.ExternalResourceRef, state, normalizedStatus, ipCount, true)
	if snapshotErr != nil {
		return nil, snapshotErr
	}
	snapshot.Data, snapshotErr = json.Marshal(resourceObservationData(object, normalizedStatus))
	if snapshotErr != nil || snapshot.Validate() != nil {
		return nil, errInvalidResponse
	}
	return snapshot, nil
}

func (p *Provider) unknownTaskResult(
	query resourcecap.TaskQuery,
	summary string,
) (*resourcecap.OperationResult, error) {
	result := &resourcecap.OperationResult{
		Status:              resourcecap.TaskStatusUnknown,
		ExternalTaskRef:     query.ExternalTaskRef,
		ExternalResourceRef: query.ResourceRef,
		Failure: &resourcecap.ProviderFailure{
			Code:          "UPSTREAM_TASK_UNKNOWN",
			Message:       "upstream task result requires later reconciliation",
			ResultUnknown: true,
		},
		Evidence: []resourcecap.Evidence{{
			Kind:       "unknown_task",
			Reference:  query.ExternalTaskRef,
			Summary:    summary,
			ObservedAt: p.now(),
		}},
	}
	if err := result.Validate(); err != nil {
		return nil, errInvalidResponse
	}
	return result, nil
}

func (p *Provider) resourceSnapshot(
	reference string,
	state resourcecap.ResourceState,
	upstreamStatus string,
	ipCount *int,
	observed bool,
) (*resourcecap.ResourceSnapshot, error) {
	projection := map[string]any{
		"observed":        observed,
		"upstream_status": upstreamStatus,
	}
	if ipCount != nil {
		projection["ip_count"] = *ipCount
	}
	data, err := json.Marshal(projection)
	if err != nil {
		return nil, errInvalidResponse
	}
	snapshot := &resourcecap.ResourceSnapshot{
		ExternalResourceRef: reference,
		State:               state,
		Data:                data,
		ObservedAt:          p.now(),
		Evidence: []resourcecap.Evidence{{
			Kind:       "upstream_resource",
			Reference:  reference,
			Summary:    "field-whitelisted upstream resource status observed",
			ObservedAt: p.now(),
		}},
	}
	if err := snapshot.Validate(); err != nil {
		return nil, errInvalidResponse
	}
	return snapshot, nil
}

func normalizedIPCount(object rawObject) *int {
	count, ok := metaInteger(object, "ip_num")
	if !ok || count > 65535 {
		return nil
	}
	return &count
}

func stringScalar(raw json.RawMessage) (string, bool) {
	value, ok := decodeScalar(raw)
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

func normalizeUpstreamResourceState(status string) (resourcecap.ResourceState, string) {
	switch status {
	case "on":
		return resourcecap.ResourceStateActive, status
	case "off":
		return resourcecap.ResourceStateStopped, status
	case "suspend", "paused":
		return resourcecap.ResourceStateSuspended, status
	case "recycle":
		return resourcecap.ResourceStateTerminated, status
	case "wait_reboot":
		return resourcecap.ResourceStatePending, status
	case "task", "cold_migrate", "hot_migrate":
		return resourcecap.ResourceStatePending, status
	case "unknown":
		return resourcecap.ResourceStateUnknown, status
	default:
		return resourcecap.ResourceStateUnknown, "unknown"
	}
}
