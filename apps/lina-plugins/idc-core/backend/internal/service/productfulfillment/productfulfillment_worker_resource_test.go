// This file verifies the idc-resource fulfillment split without a database or
// supplier credentials.

package productfulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

type resourceSourceStub struct {
	created []provisionresourcecap.TaskRequest
	read    []provisionresourcecap.TaskReadRequest
	task    *provisionresourcecap.Task
	err     error
}

func (stub *resourceSourceStub) CreateTask(_ context.Context, request provisionresourcecap.TaskRequest) (*provisionresourcecap.Task, error) {
	stub.created = append(stub.created, request)
	return stub.task, stub.err
}

func (stub *resourceSourceStub) ReadTask(_ context.Context, request provisionresourcecap.TaskReadRequest) (*provisionresourcecap.Task, error) {
	stub.read = append(stub.read, request)
	return stub.task, stub.err
}

func (*resourceSourceStub) ReportOutcome(context.Context, provisionresourcecap.OutcomeReport) error {
	return nil
}

func TestResourceOwnerQueryFirstDoesNotCreateTask(t *testing.T) {
	t.Parallel()
	stub := &resourceSourceStub{}
	worker := &fulfillmentWorker{resourceSource: stub}
	claimed, request := resourceWorkerFixture(productfulfillmentcap.ActionQuery)
	invocation := worker.invokeResourceOwner(context.Background(), claimed, request)
	if invocation.failed || invocation.providerBoundaryCrossed || invocation.result == nil ||
		invocation.result.Status != productfulfillmentcap.StatusNotFound || len(stub.created) != 0 || len(stub.read) != 0 {
		t.Fatalf("query-first result = %+v created=%d read=%d", invocation, len(stub.created), len(stub.read))
	}
}

func TestResourceOwnerSubmitForwardsFrozenSnapshotsWithoutServerParsing(t *testing.T) {
	t.Parallel()
	stub := &resourceSourceStub{task: validResourceTask(provisionresourcecap.TaskStatusPending)}
	worker := &fulfillmentWorker{resourceSource: stub}
	claimed, request := resourceWorkerFixture(productfulfillmentcap.ActionSubmit)
	invocation := worker.invokeResourceOwner(context.Background(), claimed, request)
	if invocation.failed || !invocation.providerBoundaryCrossed || invocation.result == nil ||
		invocation.result.Status != productfulfillmentcap.StatusAccepted || len(stub.created) != 1 {
		t.Fatalf("submit result = %+v created=%d", invocation, len(stub.created))
	}
	forwarded := stub.created[0]
	if forwarded.InterfaceInstanceNo != "pfi_11111111111111111111111111111111" || forwarded.CustomerNo != "customer_1" ||
		forwarded.InterfaceConfigurationRevision != "rci_11111111111111111111111111111111" ||
		string(forwarded.ProductConfiguration) != string(request.ProductConfiguration) ||
		string(forwarded.CustomerConfiguration) != string(request.CustomerConfiguration) ||
		forwarded.Fulfillment.OrderItemNo != request.OrderItemNo ||
		provisionresourcecap.ValidateTaskRequest(forwarded) != nil {
		t.Fatalf("forwarded resource request = %+v", forwarded)
	}
	replayed := worker.invokeResourceOwner(context.Background(), claimed, request)
	if replayed.failed || len(stub.created) != 2 ||
		stub.created[1].OperationNo != forwarded.OperationNo ||
		stub.created[1].RequestDigest != forwarded.RequestDigest {
		t.Fatalf("replayed resource request changed identity: %+v", stub.created)
	}
}

func TestResourceOwnerTaskPreparationFailureRemainsSafelyRetryable(t *testing.T) {
	t.Parallel()
	stub := &resourceSourceStub{err: errors.New("upstream principal is still converging")}
	worker := &fulfillmentWorker{resourceSource: stub}
	claimed, request := resourceWorkerFixture(productfulfillmentcap.ActionSubmit)
	invocation := worker.invokeResourceOwner(context.Background(), claimed, request)
	if !invocation.failed || invocation.providerBoundaryCrossed || invocation.result != nil ||
		len(stub.created) != 1 {
		t.Fatalf("pre-task resource failure was treated as an uncertain server write: %+v", invocation)
	}
}

func TestResourceOwnerExistingTaskUsesReadAndMapsManualReview(t *testing.T) {
	t.Parallel()
	stub := &resourceSourceStub{task: validResourceTask(provisionresourcecap.TaskStatusManualReview)}
	worker := &fulfillmentWorker{resourceSource: stub}
	claimed, request := resourceWorkerFixture(productfulfillmentcap.ActionQuery)
	claimed.PreviousExternalReference = "resource_task_1"
	request.PreviousExternalReference = claimed.PreviousExternalReference
	invocation := worker.invokeResourceOwner(context.Background(), claimed, request)
	if invocation.failed || !invocation.manualReview || invocation.result == nil ||
		invocation.result.Status != productfulfillmentcap.StatusResultUnknown || len(stub.read) != 1 ||
		stub.read[0].ProviderTaskNo != claimed.PreviousExternalReference || len(stub.created) != 0 {
		t.Fatalf("manual read result = %+v created=%d read=%+v", invocation, len(stub.created), stub.read)
	}
}

func TestResourceOwnerReadRejectsAnotherTaskReference(t *testing.T) {
	t.Parallel()
	stub := &resourceSourceStub{task: validResourceTask(provisionresourcecap.TaskStatusRunning)}
	worker := &fulfillmentWorker{resourceSource: stub}
	claimed, request := resourceWorkerFixture(productfulfillmentcap.ActionQuery)
	claimed.PreviousExternalReference = "another_resource_task"
	request.PreviousExternalReference = claimed.PreviousExternalReference
	invocation := worker.invokeResourceOwner(context.Background(), claimed, request)
	if !invocation.failed || !invocation.providerBoundaryCrossed || invocation.result != nil {
		t.Fatalf("mismatched resource task was accepted: %+v", invocation)
	}
}

func resourceWorkerFixture(action productfulfillmentcap.Action) (*claimedFulfillment, *productfulfillmentcap.Request) {
	productConfiguration := json.RawMessage(`{"merchant":"unchanged"}`)
	request := &productfulfillmentcap.Request{
		Version: productfulfillmentcap.RequestVersion, IntentNo: "PFI_1", IdempotencyKey: "PFI_1",
		OrderNo: "ORD_1", OrderItemNo: "ORI_1", UnitSequence: 1, Quantity: 1,
		ProductNo: "PRD_1", ProductVersionNo: "PV_1", ConfigurationRevision: "revision_1",
		ProductConfiguration: productConfiguration, ProductConfigurationDigest: strings.Repeat("a", 64),
		CustomerConfiguration: json.RawMessage(`{"customer":"unchanged"}`), CustomerConfigurationID: "config_1",
	}
	return &claimedFulfillment{
		Action: action, CustomerNo: "customer_1", EntitlementOwnerID: provisionresourcecap.OwnerID,
		InterfaceInstanceNo:                    "pfi_11111111111111111111111111111111",
		ResourceInterfaceConfigurationRevision: "rci_11111111111111111111111111111111",
	}, request
}

func validResourceTask(status provisionresourcecap.TaskStatus) *provisionresourcecap.Task {
	now := time.Now().UTC()
	task := &provisionresourcecap.Task{
		ProviderTaskNo: "resource_task_1", ChannelNo: "CHN_1", Status: status,
		CapabilityEvidenceNo: "snapshot_1", CapabilityEvidenceDigest: strings.Repeat("b", 64),
		EvidenceDigest: strings.Repeat("c", 64), ObservedAt: now,
	}
	if status == provisionresourcecap.TaskStatusSucceeded || status == provisionresourcecap.TaskStatusFailed ||
		status == provisionresourcecap.TaskStatusManualReview {
		task.CompletedAt = now
	}
	if status == provisionresourcecap.TaskStatusFailed || status == provisionresourcecap.TaskStatusManualReview ||
		status == provisionresourcecap.TaskStatusResultUnknown {
		task.FailureCode = "resource.needs_attention"
	}
	return task
}
