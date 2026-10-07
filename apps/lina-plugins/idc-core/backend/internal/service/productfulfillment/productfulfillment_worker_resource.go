// This file routes idc-resource-owned fulfillment through its typed source
// while keeping server configuration interpretation outside the generic core.

package productfulfillment

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

type resourceInvocation struct {
	result                  *productfulfillmentcap.Result
	rawResult               []byte
	failed                  bool
	providerBoundaryCrossed bool
	manualReview            bool
	outcomeRequest          *provisionresourcecap.TaskRequest
	providerTaskNo          string
	taskEvidenceDigest      string
}

func (w *fulfillmentWorker) invokeResourceOwner(ctx context.Context, claimed *claimedFulfillment, request *productfulfillmentcap.Request) resourceInvocation {
	if w.resourceSource == nil {
		return resourceInvocation{failed: true}
	}
	resourceRequest, err := frozenResourceTaskRequest(claimed, request)
	if err != nil {
		return resourceInvocation{failed: true}
	}
	if claimed.Action == productfulfillmentcap.ActionQuery && claimed.PreviousExternalReference == "" {
		return marshalResourceInvocation(&productfulfillmentcap.Result{
			Version: productfulfillmentcap.RequestVersion,
			Status:  productfulfillmentcap.StatusNotFound,
		}, false)
	}
	// Creating or reading the resource owner's durable task is itself an
	// idempotent owner operation. An error without a returned task reference
	// does not prove that the server Provider boundary was crossed, so core must
	// keep the query-first bounded retry path available. The resource owner is
	// responsible for reconciling any upstream-principal task before it creates
	// the server task.
	invocation := resourceInvocation{}
	var task *provisionresourcecap.Task
	if claimed.Action == productfulfillmentcap.ActionSubmit {
		task, err = w.resourceSource.CreateTask(ctx, resourceRequest)
	} else {
		task, err = w.resourceSource.ReadTask(ctx, provisionresourcecap.TaskReadRequest{
			ProviderTaskNo: claimed.PreviousExternalReference,
			Request:        resourceRequest,
		})
	}
	if err != nil || task == nil || provisionresourcecap.ValidateTask(*task) != nil {
		invocation.failed = true
		return invocation
	}
	invocation.providerBoundaryCrossed = true
	if claimed.Action == productfulfillmentcap.ActionQuery && task.ProviderTaskNo != claimed.PreviousExternalReference {
		invocation.failed = true
		return invocation
	}
	result, manualReview, err := projectResourceTask(*task)
	if err != nil || productfulfillmentcap.ValidateResult(claimed.Action, result) != nil {
		invocation.failed = true
		return invocation
	}
	invocation = marshalResourceInvocation(result, true)
	invocation.manualReview = manualReview
	if task.Status == provisionresourcecap.TaskStatusSucceeded ||
		task.Status == provisionresourcecap.TaskStatusFailed ||
		task.Status == provisionresourcecap.TaskStatusManualReview {
		invocation.outcomeRequest = &resourceRequest
		invocation.providerTaskNo = task.ProviderTaskNo
		invocation.taskEvidenceDigest = task.EvidenceDigest
	}
	return invocation
}

func frozenResourceTaskRequest(claimed *claimedFulfillment, request *productfulfillmentcap.Request) (provisionresourcecap.TaskRequest, error) {
	if claimed == nil || request == nil {
		return provisionresourcecap.TaskRequest{}, gerror.New("resource fulfillment request is unavailable")
	}
	resourceRequest := provisionresourcecap.TaskRequest{
		OperationNo: request.IntentNo, CustomerNo: claimed.CustomerNo,
		ConfigurationSnapshotNo:        request.CustomerConfigurationID,
		InterfaceInstanceNo:            claimed.InterfaceInstanceNo,
		InterfaceConfigurationRevision: claimed.ResourceInterfaceConfigurationRevision,
		Fulfillment: provisionresourcecap.FrozenFulfillmentIdentity{
			IntentNo: request.IntentNo, OrderNo: request.OrderNo, OrderItemNo: request.OrderItemNo,
			ProductNo: request.ProductNo, ProductVersionNo: request.ProductVersionNo,
			ConfigurationRevision: request.ConfigurationRevision,
			UnitSequence:          request.UnitSequence, Quantity: request.Quantity,
		},
		ProductConfiguration:       append(json.RawMessage(nil), request.ProductConfiguration...),
		ProductConfigurationDigest: request.ProductConfigurationDigest,
		CustomerConfiguration:      append(json.RawMessage(nil), request.CustomerConfiguration...),
	}
	digest, err := provisionresourcecap.TaskRequestDigest(resourceRequest)
	if err != nil {
		return provisionresourcecap.TaskRequest{}, err
	}
	resourceRequest.RequestDigest = digest
	return resourceRequest, provisionresourcecap.ValidateTaskRequest(resourceRequest)
}

func projectResourceTask(task provisionresourcecap.Task) (*productfulfillmentcap.Result, bool, error) {
	result := &productfulfillmentcap.Result{
		Version:           productfulfillmentcap.RequestVersion,
		ExternalReference: task.ProviderTaskNo,
	}
	manualReview := false
	switch task.Status {
	case provisionresourcecap.TaskStatusPending:
		result.Status = productfulfillmentcap.StatusAccepted
	case provisionresourcecap.TaskStatusRunning:
		result.Status = productfulfillmentcap.StatusRunning
	case provisionresourcecap.TaskStatusSucceeded:
		result.Status = productfulfillmentcap.StatusSucceeded
	case provisionresourcecap.TaskStatusFailed:
		result.Status = productfulfillmentcap.StatusFailed
	case provisionresourcecap.TaskStatusResultUnknown:
		result.Status = productfulfillmentcap.StatusResultUnknown
	case provisionresourcecap.TaskStatusManualReview:
		result.Status = productfulfillmentcap.StatusResultUnknown
		manualReview = true
	default:
		return nil, false, gerror.New("resource fulfillment task status is invalid")
	}
	evidence, err := json.Marshal(struct {
		ChannelNo                string `json:"channelNo"`
		FailureCode              string `json:"failureCode,omitempty"`
		CapabilityEvidenceNo     string `json:"capabilityEvidenceNo"`
		CapabilityEvidenceDigest string `json:"capabilityEvidenceDigest"`
		EvidenceDigest           string `json:"evidenceDigest"`
	}{
		ChannelNo: task.ChannelNo, FailureCode: task.FailureCode,
		CapabilityEvidenceNo:     task.CapabilityEvidenceNo,
		CapabilityEvidenceDigest: task.CapabilityEvidenceDigest,
		EvidenceDigest:           task.EvidenceDigest,
	})
	if err != nil {
		return nil, false, err
	}
	result.Evidence = evidence
	return result, manualReview, nil
}

func marshalResourceInvocation(result *productfulfillmentcap.Result, crossed bool) resourceInvocation {
	raw, err := json.Marshal(result)
	if err != nil {
		return resourceInvocation{failed: true, providerBoundaryCrossed: crossed}
	}
	return resourceInvocation{result: result, rawResult: raw, providerBoundaryCrossed: crossed}
}
