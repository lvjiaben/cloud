// This file routes idc-resource-owned fulfillment through its typed source
// while keeping server configuration interpretation outside the generic core.

package productfulfillment

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

// Local progress reads never contact the supplier. Their shorter interval
// preserves the previous bounded observation window by scaling its read budget.
const resourceOwnerProgressIntervalSeconds = 5
const resourceOwnerMaximumQueryAttempts = maximumQueryAttempts * retryIntervalSeconds / resourceOwnerProgressIntervalSeconds

// resourceWorkerQueryAttempts expresses local reads in the original query-budget
// units. Raw attempts remain in the audit; only post-submit local reads scale.
func resourceWorkerQueryAttempts(claimed *claimedFulfillment) int {
	if claimed.EntitlementOwnerID != provisionresourcecap.OwnerID || claimed.PreviousPhase != "post_submit_query" {
		return claimed.QueryAttempts
	}
	readsPerAttempt := retryIntervalSeconds / resourceOwnerProgressIntervalSeconds
	return claimed.QueryAttempts / readsPerAttempt
}

// resourceWorkerScheduleSeconds keeps local progress responsive without
// changing the supplier owner's query schedule or permitting another write.
func resourceWorkerScheduleSeconds(claimed *claimedFulfillment, decision workerDecision, result *productfulfillmentcap.Result) int {
	if claimed.EntitlementOwnerID != provisionresourcecap.OwnerID || !decision.ScheduleNext {
		return retryIntervalSeconds
	}
	if claimed.Action == productfulfillmentcap.ActionQuery && result != nil && result.Status == productfulfillmentcap.StatusNotFound &&
		(claimed.PreviousExternalReference == "" || (claimed.RecoveryCount > 0 && claimed.PreviousPhase == "pre_submit_query")) {
		return 0
	}
	return resourceOwnerProgressIntervalSeconds
}

type resourceInvocation struct {
	result                  *productfulfillmentcap.Result
	rawResult               []byte
	failed                  bool
	providerBoundaryCrossed bool
	manualReview            bool
	recoverableRejection    bool
	failureCode             string
	responseExcerpt         string
	outcomeRequest          *provisionresourcecap.TaskRequest
	providerTaskNo          string
	taskEvidenceDigest      string
}

func (w *fulfillmentWorker) invokeResourceOwner(ctx context.Context, claimed *claimedFulfillment, request *productfulfillmentcap.Request) resourceInvocation {
	if w.resourceSource == nil {
		return resourceInvocation{failed: true}
	}
	resourceRequest, err := w.resourceTaskRequest(ctx, claimed, request)
	if err != nil {
		return failedResourcePreparation(provisionresourcecap.PreparationFailureRequest, err)
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
	var correction *provisionresourcecap.RecoveryConfiguration
	if claimed.RecoveryCount > 0 {
		correction, err = loadRecoveryConfiguration(ctx, w.database, claimed.IntentNo)
		if err != nil {
			return failedResourcePreparation(provisionresourcecap.PreparationFailureLookup, err)
		}
	}
	var task *provisionresourcecap.Task
	if claimed.Action == productfulfillmentcap.ActionSubmit {
		if claimed.RecoveryCount > 0 && claimed.PreviousExternalReference != "" {
			task, err = w.resourceSource.RecoverRejected(ctx, provisionresourcecap.TaskReadRequest{ProviderTaskNo: claimed.PreviousExternalReference, Request: resourceRequest, RecoveryConfiguration: correction})
		} else {
			task, err = w.resourceSource.CreateTask(ctx, resourceRequest, correction)
		}
	} else {
		task, err = w.resourceSource.ReadTask(ctx, provisionresourcecap.TaskReadRequest{
			ProviderTaskNo: claimed.PreviousExternalReference,
			Request:        resourceRequest,
		})
	}
	if err != nil || task == nil || provisionresourcecap.ValidateTask(*task) != nil {
		invocation.failed = true
		// A failed local read must retain the already-observed owner task and
		// accepted/running state. It cannot prove absence or authorize resubmit.
		invocation.providerBoundaryCrossed = claimed.Action == productfulfillmentcap.ActionQuery &&
			claimed.PreviousExternalReference != "" && claimed.PreviousPhase == "post_submit_query" &&
			(claimed.PreviousStatus == "accepted" || claimed.PreviousStatus == "running")
		invocation.failureCode, invocation.responseExcerpt = provisionresourcecap.PreparationDiagnostic(err)
		return invocation
	}
	invocation.providerBoundaryCrossed = true
	if claimed.Action == productfulfillmentcap.ActionQuery && task.ProviderTaskNo != claimed.PreviousExternalReference {
		invocation.failed = true
		return invocation
	}
	if claimed.Action == productfulfillmentcap.ActionQuery && claimed.RecoveryCount > 0 &&
		claimed.PreviousPhase == "pre_submit_query" && task.Status == provisionresourcecap.TaskStatusFailed &&
		(task.FailureCode == "upstream_request_rejected" || task.FailureCode == provisionresourcecap.FailureNonAcceptanceVerified || task.FailureCode == provisionresourcecap.FailureRequestNotSent) {
		// The staff command grants a successor, never another write of this
		// closed task. The owner independently fences successor creation.
		return marshalResourceInvocation(&productfulfillmentcap.Result{
			Version: productfulfillmentcap.RequestVersion, Status: productfulfillmentcap.StatusNotFound,
		}, true)
	}
	result, manualReview, err := projectResourceTask(*task)
	if err != nil || productfulfillmentcap.ValidateResult(claimed.Action, result) != nil {
		invocation.failed = true
		return invocation
	}
	invocation = marshalResourceInvocation(result, true)
	invocation.manualReview = manualReview
	invocation.failureCode = safeDiagnosticFailureCode(task.FailureCode)
	invocation.responseExcerpt = task.ResponseExcerpt
	invocation.recoverableRejection = task.Status == provisionresourcecap.TaskStatusFailed && (task.FailureCode == "upstream_request_rejected" || task.FailureCode == provisionresourcecap.FailureNonAcceptanceVerified || task.FailureCode == provisionresourcecap.FailureRequestNotSent)
	if task.Status == provisionresourcecap.TaskStatusSucceeded ||
		task.Status == provisionresourcecap.TaskStatusFailed ||
		task.Status == provisionresourcecap.TaskStatusManualReview {
		invocation.outcomeRequest = &resourceRequest
		invocation.providerTaskNo = task.ProviderTaskNo
		invocation.taskEvidenceDigest = task.EvidenceDigest
	}
	return invocation
}

// failedResourcePreparation preserves only reviewed local diagnostics. It does
// not change scheduling, mark a provider write or authorize unknown-task recovery.
func failedResourcePreparation(code provisionresourcecap.PreparationFailureCode, cause error) resourceInvocation {
	failureCode, responseExcerpt := provisionresourcecap.PreparationDiagnostic(provisionresourcecap.WrapPreparationFailure(code, cause))
	return resourceInvocation{failed: true, failureCode: failureCode, responseExcerpt: responseExcerpt}
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

// resourceTaskRequest reconstructs an imported purchase only from the exact
// provider declaration and frozen read contract. The marker only avoids a
// needless contract read for ordinary server requests; it grants no authority.
func (w *fulfillmentWorker) resourceTaskRequest(ctx context.Context, claimed *claimedFulfillment, request *productfulfillmentcap.Request) (provisionresourcecap.TaskRequest, error) {
	result, err := frozenResourceTaskRequest(claimed, request)
	if err != nil {
		return result, err
	}
	if w.database != nil {
		var choice struct{ ServerLoginToken, Hostname, PasswordSecretRef, ChannelNo string }
		err = w.database.GetScan(ctx, &choice, `SELECT snapshot.server_login_token, login.hostname,login.password_secret_ref,login.channel_no FROM plugin_idc_core_commerce_structured_configuration_snapshot snapshot JOIN plugin_idc_core_purchase_server_login login ON login.tenant_id=snapshot.tenant_id AND login.server_login_token=snapshot.server_login_token AND login.customer_no=snapshot.customer_no AND login.product_version_no=snapshot.version_no WHERE snapshot.tenant_id=0 AND snapshot.snapshot_no=? AND snapshot.order_item_no=? AND snapshot.customer_no=?`, request.CustomerConfigurationID, request.OrderItemNo, claimed.CustomerNo)
		if err != nil {
			return result, err
		}
		if choice.ServerLoginToken != "" {
			result.ServerLogin = &provisionresourcecap.ServerLoginChoice{Hostname: choice.Hostname, PasswordSecretRef: choice.PasswordSecretRef, ChannelNo: choice.ChannelNo}
			result.RequestDigest, err = provisionresourcecap.TaskRequestDigest(result)
			if err != nil {
				return result, err
			}
		}
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(request.ProductConfiguration, &fields) != nil {
		return result, gerror.New("frozen product configuration is invalid")
	}
	if _, exists := fields["upstream_configuration_revision"]; !exists {
		return result, nil
	}
	if w.extensions == nil {
		return result, gerror.New("native purchase exact provider is unavailable")
	}
	exact, err := w.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{PluginID: claimed.PluginID, ReleaseVersion: claimed.ReleaseVersion, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: claimed.ContractVersion, ReleaseDigest: claimed.ReleaseDigest, DeclarationDigest: claimed.DeclarationDigest})
	if err != nil || exact == nil {
		return result, gerror.New("native purchase exact provider is unavailable")
	}
	declaration, err := productfulfillmentcap.ParseProviderDeclaration(exact.ProviderMetadata, exact.AuthorizedMethods)
	if err != nil {
		return result, err
	}
	if !declaration.SaleImportProtocol.NativePurchase() {
		return result, nil
	}
	method := claimed.ProvisionMethod
	if claimed.Action == productfulfillmentcap.ActionQuery {
		method = claimed.QueryMethod
	}
	snapshot, read, err := w.loadPeerContract(ctx, claimed, method)
	if err != nil {
		return result, err
	}
	if read.SaleImportProtocol != declaration.SaleImportProtocol {
		return result, gerror.New("native purchase source protocol mismatch")
	}
	purchase, err := w.peerPurchase(ctx, claimed, snapshot, read, method)
	if err != nil {
		return result, err
	}
	purchase.Quantity = 1
	result.NativePurchase = purchase
	result.NativePurchaseProtocol = declaration.SaleImportProtocol
	result.RequestDigest, err = provisionresourcecap.TaskRequestDigest(result)
	if err != nil {
		return result, err
	}
	return result, provisionresourcecap.ValidateTaskRequest(result)
}
