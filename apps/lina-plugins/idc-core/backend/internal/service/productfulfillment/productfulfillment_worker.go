package productfulfillment

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

const maximumWorkerBatch = 64

type fulfillmentWorker struct {
	database       gdb.DB
	extensions     pluginhost.BusinessExtensionRegistrar
	contexts       productautomation.ExistingBusinessRuntimeContextIssuer
	evidence       providerEvidenceSealer
	systemReasons  SystemTransitionReasonCatalog
	resourceSource provisionresourcecap.Source
	random         io.Reader
}

type claimedFulfillment struct {
	TaskNo, IntentNo, LeaseToken, PreviousStatus, PreviousPhase string
	PreviousExternalReference                                   string
	PluginID, ReleaseVersion, ContractVersion                   string
	ReleaseDigest, DeclarationDigest                            string
	InterfaceInstanceNo, ProvisionMethod, QueryMethod           string
	OrderNo, OrderItemNo, ProductNo, VersionNo                  string
	ConfigurationRevision, ConfigurationSnapshotNo              string
	CustomerDeclaration                                         string
	CustomerNo, EntitlementOwnerID                              string
	CustomerConfiguration                                       json.RawMessage
	ProductConfiguration                                        json.RawMessage
	ProductConfigurationDigest                                  string
	InterfaceConfigurationRevision                              int64
	ResourceInterfaceConfigurationRevision                      string
	UnitSequence, Quantity, AttemptNo                           int
	QueryAttempts, SubmitAttempts, MaximumAttempts              int
	RetryBaseSeconds                                            int
	Action                                                      productfulfillmentcap.Action
}

func NewWorker(database gdb.DB, extensions pluginhost.BusinessExtensionRegistrar, contexts productautomation.ExistingBusinessRuntimeContextIssuer, evidence providerEvidenceSealer, systemReasons SystemTransitionReasonCatalog, resourceSource provisionresourcecap.Source) (Worker, error) {
	if database == nil || extensions == nil || contexts == nil || evidence == nil || systemReasons == nil {
		return nil, errors.New("product fulfillment worker dependencies are unavailable")
	}
	return &fulfillmentWorker{database: database, extensions: extensions, contexts: contexts, evidence: evidence, systemReasons: systemReasons, resourceSource: resourceSource, random: rand.Reader}, nil
}

func (w *fulfillmentWorker) Run(ctx context.Context, limit int) (*WorkerBatch, error) {
	if w == nil || w.database == nil || w.extensions == nil || w.contexts == nil || ctx == nil || ctx.Err() != nil || limit < 1 || limit > maximumWorkerBatch {
		return nil, errors.New("product fulfillment worker input is invalid")
	}
	report := &WorkerBatch{}
	for report.Selected < limit {
		if err := w.recoverOneExpiredLease(ctx); err != nil {
			return report, err
		}
		claimed, err := w.claimOne(ctx)
		if err != nil {
			return report, err
		}
		if claimed == nil {
			break
		}
		report.Selected++
		decision, err := w.invokeAndFinalize(ctx, claimed)
		if err != nil {
			report.Failed++
			continue
		}
		if decision.TaskStatus == "succeeded" {
			report.Completed++
		}
		if decision.IntentStatus == "manual_pending" {
			report.ManualAttention++
		}
		if decision.ScheduleNext {
			report.Scheduled++
		}
	}
	return report, nil
}

func (w *fulfillmentWorker) recoverOneExpiredLease(ctx context.Context) error {
	return w.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var candidate *struct{ TaskNo, IntentNo string }
		if err := w.database.GetScan(txCtx, &candidate, `SELECT task.task_no,task.intent_no FROM plugin_idc_core_product_fulfillment_task task JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.tenant_id=task.tenant_id AND intent.intent_no=task.intent_no WHERE task.tenant_id=0 AND task.status_code IN ('querying','submitting') AND task.lease_expires_at<=CURRENT_TIMESTAMP ORDER BY task.lease_expires_at,task.task_no LIMIT 1 FOR UPDATE OF intent SKIP LOCKED`); err != nil || candidate == nil {
			return err
		}
		var expired *struct {
			TaskNo, IntentNo, StatusCode, PhaseCode, ExternalReference string
			AttemptCount, QueryAttemptCount, MaximumAttempts           int
			TakeoverRequested, CancelRequested                         bool
			Version                                                    int64
		}
		if err := w.database.GetScan(txCtx, &expired, `SELECT task_no,intent_no,status_code,phase_code,external_reference,attempt_count,query_attempt_count,maximum_attempts,takeover_requested,cancel_requested,version FROM plugin_idc_core_product_fulfillment_task WHERE tenant_id=0 AND task_no=? AND intent_no=? AND status_code IN ('querying','submitting') AND lease_expires_at<=CURRENT_TIMESTAMP FOR UPDATE`, candidate.TaskNo, candidate.IntentNo); err != nil || expired == nil {
			return err
		}
		safeEvidence := []byte(`{"status":"result_unknown","recovery":"lease_expired"}`)
		closed, err := w.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_attempt SET finished_at=CURRENT_TIMESTAMP,result_code='worker_interrupted',result_evidence_digest=?,result_status_code='result_unknown',safe_result_evidence=?::jsonb,safe_summary='Worker interrupted before a durable provider result' WHERE tenant_id=0 AND task_no=? AND attempt_no=? AND finished_at IS NULL`, sha256Hex(safeEvidence), safeEvidence, expired.TaskNo, expired.AttemptCount)
		if err != nil || affected(closed) != 1 {
			return errorsNew("product_fulfillment_task_conflict")
		}
		nextStatus, nextPhase := "retry_wait", expired.PhaseCode
		if expired.StatusCode == "submitting" {
			// Once a submit lease expires, the write may already have crossed the
			// provider boundary. A frozen provider reference permits query-only
			// recovery. Without one, fail closed as an unknown result and require
			// an explicitly authorized takeover; never submit again.
			if expired.ExternalReference == "" {
				nextStatus, nextPhase = "result_unknown", "frozen_unknown"
			} else {
				nextPhase = "post_submit_query"
			}
		}
		if expired.TakeoverRequested {
			nextStatus, nextPhase = "manual_takeover", "frozen_unknown"
		} else if expired.CancelRequested && (expired.StatusCode == "submitting" || expired.QueryAttemptCount >= expired.MaximumAttempts) {
			nextStatus, nextPhase = "cancellation_attention", "frozen_unknown"
		}
		updated, err := w.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code=?,phase_code=?,lease_token='',lease_expires_at=NULL,next_attempt_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND task_no=? AND version=? AND status_code=?`, nextStatus, nextPhase, expired.TaskNo, expired.Version, expired.StatusCode)
		if err != nil || affected(updated) != 1 {
			return errorsNew("product_fulfillment_task_conflict")
		}
		if expired.CancelRequested && nextStatus == "cancellation_attention" {
			reason, reasonErr := w.systemReasons.ResolveSystemTransitionReason(SystemReasonCancellationRequiresAttention)
			if reasonErr != nil {
				return reasonErr
			}
			if _, err = w.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_fulfillment_system_transition(tenant_id,transition_no,intent_no,event_code,reason_code,reason_label_zh_cn,reason_label_en_us) VALUES(0,'pfs_'||substr(md5('manual-cancel'||?),1,32),?,'moved_to_manual',?,?,?) ON CONFLICT(intent_no,event_code) DO NOTHING`, expired.IntentNo, expired.IntentNo, SystemReasonCancellationRequiresAttention, reason.LabelZhCN, reason.LabelEnUS); err != nil {
				return err
			}
		}
		if expired.TakeoverRequested || nextStatus == "cancellation_attention" {
			intentResult, updateErr := w.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='manual_pending',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND intent_no=? AND status_code='processing'`, expired.IntentNo)
			if updateErr != nil || affected(intentResult) != 1 {
				return errorsNew("product_fulfillment_task_conflict")
			}
		}
		return nil
	})
}

func (w *fulfillmentWorker) claimOne(ctx context.Context) (*claimedFulfillment, error) {
	var claimed *claimedFulfillment
	err := w.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var candidate *struct{ TaskNo, IntentNo string }
		if err := w.database.GetScan(txCtx, &candidate, `SELECT task.task_no,task.intent_no FROM plugin_idc_core_product_fulfillment_task task JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.tenant_id=task.tenant_id AND intent.intent_no=task.intent_no WHERE task.tenant_id=0 AND task.lease_token='' AND task.next_attempt_at<=CURRENT_TIMESTAMP AND task.status_code IN ('pending_query','pending_submit','retry_wait','accepted','running') AND ((task.phase_code='submit_allowed' AND task.submit_attempt_count<3) OR (task.phase_code<>'submit_allowed' AND task.query_attempt_count<10)) ORDER BY task.next_attempt_at,task.task_no LIMIT 1 FOR UPDATE OF intent SKIP LOCKED`); err != nil || candidate == nil {
			return err
		}
		var row *claimedFulfillment
		err := w.database.GetScan(txCtx, &row, `SELECT task.task_no,task.intent_no,task.status_code AS previous_status,task.phase_code AS previous_phase,
intent.plugin_id,intent.plugin_release_version AS release_version,intent.contract_version,intent.release_digest,intent.declaration_digest,
intent.interface_instance_no,intent.interface_configuration_revision,interface_configuration.resource_owner_configuration_revision AS resource_interface_configuration_revision,intent.provision_method,intent.query_method,
intent.order_no,intent.order_item_no,intent.product_no,intent.version_no,intent.configuration_snapshot_no,
intent.customer_no,intent.entitlement_owner_id,
snapshot.configuration_revision,version_configuration.customer_declaration_snapshot::text AS customer_declaration,
snapshot.typed_value_snapshot AS customer_configuration,
version_configuration.merchant_value_snapshot AS product_configuration,version_configuration.merchant_value_digest AS product_configuration_digest,
intent.unit_sequence,intent.quantity,task.attempt_count+1 AS attempt_no,task.query_attempt_count,task.submit_attempt_count,
		task.maximum_attempts,task.retry_base_seconds,task.external_reference AS previous_external_reference
FROM plugin_idc_core_product_fulfillment_task task
JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.tenant_id=task.tenant_id AND intent.intent_no=task.intent_no
JOIN plugin_idc_core_commerce_structured_configuration_snapshot snapshot ON snapshot.tenant_id=intent.tenant_id AND snapshot.snapshot_no=intent.configuration_snapshot_no AND snapshot.order_no=intent.order_no AND snapshot.order_item_no=intent.order_item_no
JOIN plugin_idc_core_catalog_product_version_configuration version_configuration ON version_configuration.tenant_id=intent.tenant_id AND version_configuration.version_no=intent.version_no AND version_configuration.interface_instance_no=intent.interface_instance_no AND version_configuration.interface_configuration_revision=intent.interface_configuration_revision
JOIN plugin_idc_core_product_automation_interface_configuration_revision interface_configuration ON interface_configuration.tenant_id=intent.tenant_id AND interface_configuration.interface_instance_no=intent.interface_instance_no AND interface_configuration.revision=intent.interface_configuration_revision
WHERE task.tenant_id=0 AND task.task_no=? AND task.intent_no=? AND task.lease_token='' AND task.next_attempt_at<=CURRENT_TIMESTAMP
AND task.status_code IN ('pending_query','pending_submit','retry_wait','accepted','running')
AND ((task.phase_code='submit_allowed' AND task.submit_attempt_count<3) OR (task.phase_code<>'submit_allowed' AND task.query_attempt_count<10))
LIMIT 1 FOR UPDATE OF task`, candidate.TaskNo, candidate.IntentNo)
		if err != nil || row == nil {
			return err
		}
		row.Action = productfulfillmentcap.ActionQuery
		claimStatus := "querying"
		if row.PreviousPhase == "submit_allowed" {
			row.Action, claimStatus = productfulfillmentcap.ActionSubmit, "submitting"
		}
		row.LeaseToken, err = workerLeaseToken(w.random)
		if err != nil {
			return err
		}
		request, err := row.request()
		if err != nil {
			return err
		}
		requestRaw, err := json.Marshal(request)
		if err != nil {
			return err
		}
		evidenceDigest := sha256Hex(requestRaw)
		result, err := w.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code=?,lease_token=?,lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '60 seconds',attempt_count=attempt_count+1,query_attempt_count=query_attempt_count+CASE WHEN ?='query' THEN 1 ELSE 0 END,submit_attempt_count=submit_attempt_count+CASE WHEN ?='submit' THEN 1 ELSE 0 END,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND task_no=? AND lease_token='' AND status_code=?`, claimStatus, row.LeaseToken, row.Action, row.Action, row.TaskNo, row.PreviousStatus)
		if err != nil || affected(result) != 1 {
			return errorsNew("product_fulfillment_task_conflict")
		}
		if _, err = w.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_fulfillment_attempt(tenant_id,task_no,attempt_no,action_code,started_at,evidence_digest) VALUES(0,?,?,?,CURRENT_TIMESTAMP,?)`, row.TaskNo, row.AttemptNo, row.Action, evidenceDigest); err != nil {
			return err
		}
		if _, err = w.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='processing',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND intent_no=? AND status_code='queued'`, row.IntentNo); err != nil {
			return err
		}
		row.QueryAttempts++
		if row.Action == productfulfillmentcap.ActionSubmit {
			row.QueryAttempts--
			row.SubmitAttempts++
		}
		claimed = row
		return nil
	})
	return claimed, err
}

func (row *claimedFulfillment) request() (*productfulfillmentcap.Request, error) {
	customerConfiguration, err := safeProviderCustomerConfiguration(row.CustomerDeclaration, row.CustomerConfiguration)
	if err != nil {
		return nil, err
	}
	request := &productfulfillmentcap.Request{Version: productfulfillmentcap.RequestVersion, IntentNo: row.IntentNo,
		IdempotencyKey: row.IntentNo, OrderNo: row.OrderNo, OrderItemNo: row.OrderItemNo, UnitSequence: row.UnitSequence,
		Quantity: row.Quantity, ProductNo: row.ProductNo, ProductVersionNo: row.VersionNo,
		ConfigurationRevision: row.ConfigurationRevision, CustomerConfiguration: customerConfiguration,
		ProductConfiguration: row.ProductConfiguration, ProductConfigurationDigest: row.ProductConfigurationDigest,
		CustomerConfigurationID: row.ConfigurationSnapshotNo}
	request.PreviousExternalReference = row.PreviousExternalReference
	if row.PreviousPhase == "post_submit_query" && row.PreviousExternalReference == "" {
		return nil, errors.New("post-submit fulfillment query requires an external reference")
	}
	return request, productfulfillmentcap.ValidateRequest(request)
}

func safeProviderCustomerConfiguration(declarationRaw string, valuesRaw json.RawMessage) (json.RawMessage, error) {
	declaration, err := configurationcontract.DecodeDeclaration(declarationRaw)
	if err != nil {
		return nil, errors.New("customer configuration declaration is unavailable")
	}
	values := map[string]json.RawMessage{}
	if err = json.Unmarshal(valuesRaw, &values); err != nil || values == nil {
		return nil, errors.New("customer configuration is unavailable")
	}
	for _, field := range declaration.Fields {
		if field != nil && field.Type == "file" {
			delete(values, field.Key)
		}
	}
	result, err := json.Marshal(values)
	if err != nil {
		return nil, errors.New("customer configuration is unavailable")
	}
	return result, nil
}

func (w *fulfillmentWorker) invokeAndFinalize(ctx context.Context, claimed *claimedFulfillment) (workerDecision, error) {
	request, err := claimed.request()
	if err != nil {
		return workerDecision{}, err
	}
	var (
		result                  *productfulfillmentcap.Result
		rawResult               []byte
		invokeErr               error
		providerBoundaryCrossed bool
		resourceManualReview    bool
		resourceOutcome         *resourceInvocation
	)
	if claimed.EntitlementOwnerID == provisionresourcecap.OwnerID {
		invocation := w.invokeResourceOwner(ctx, claimed, request)
		result, rawResult = invocation.result, invocation.rawResult
		providerBoundaryCrossed, resourceManualReview = invocation.providerBoundaryCrossed, invocation.manualReview
		resourceOutcome = &invocation
		if invocation.failed {
			invokeErr = errors.New("resource owner fulfillment invocation failed")
		}
	} else {
		result, rawResult, providerBoundaryCrossed, invokeErr = w.invokeDynamicProvider(ctx, claimed, request)
	}
	defer clear(rawResult)
	var protectedEvidence *sealedProviderEvidence
	if result != nil && len(result.Evidence) > 0 {
		protectedEvidence, err = w.evidence.Seal(claimed.TaskNo, claimed.AttemptNo, result.Evidence)
		clear(result.Evidence)
		result.Evidence = nil
		if err != nil {
			invokeErr, result = err, nil
		}
	}
	return w.finalize(ctx, claimed, result, rawResult, invokeErr != nil, providerBoundaryCrossed, resourceManualReview, resourceOutcome, protectedEvidence)
}

func (w *fulfillmentWorker) invokeDynamicProvider(ctx context.Context, claimed *claimedFulfillment, request *productfulfillmentcap.Request) (*productfulfillmentcap.Result, []byte, bool, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, nil, false, err
	}
	method := claimed.QueryMethod
	if claimed.Action == productfulfillmentcap.ActionSubmit {
		method = claimed.ProvisionMethod
	}
	runtimeContext, invokeErr := w.contexts.IssueExistingBusinessRuntimeContext(ctx, productautomation.ExistingBusinessRuntimeContextInput{
		PluginID: claimed.PluginID, ReleaseVersion: claimed.ReleaseVersion, Point: capmodel.BusinessExtensionProductFulfillment,
		ContractVersion: claimed.ContractVersion, ReleaseDigest: claimed.ReleaseDigest, DeclarationDigest: claimed.DeclarationDigest,
		Method: method, InterfaceInstanceNo: claimed.InterfaceInstanceNo, InterfaceConfigurationRevision: claimed.InterfaceConfigurationRevision,
		IntentNo: claimed.IntentNo, OrderNo: claimed.OrderNo, OrderItemNo: claimed.OrderItemNo, ConfigurationSnapshotNo: claimed.ConfigurationSnapshotNo,
	})
	if invokeErr == nil && (runtimeContext == nil || runtimeContext.ContextID == "" || runtimeContext.Release == nil) {
		invokeErr = errors.New("provider runtime context is unavailable")
	}
	if invokeErr != nil {
		return nil, nil, false, invokeErr
	}
	defer runtimeContext.Release()
	hostResult, invokeErr := w.extensions.InvokeProviderMethod(ctx, pluginhost.BusinessExtensionMethodInvokeInput{
		PluginID: claimed.PluginID, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: claimed.ContractVersion,
		Method: method, ReleaseVersion: claimed.ReleaseVersion, ReleaseDigest: claimed.ReleaseDigest,
		DeclarationDigest: claimed.DeclarationDigest, Purpose: capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
		RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: runtimeContext.ContextID}, Payload: payload,
	})
	if invokeErr != nil {
		return nil, nil, true, invokeErr
	}
	if hostResult == nil || hostResult.Version != capmodel.BusinessExtensionInvocationProtocolV1 {
		return nil, nil, true, errors.New("provider returned an invalid fulfillment envelope")
	}
	rawResult := append([]byte(nil), hostResult.Payload...)
	var result *productfulfillmentcap.Result
	if err = json.Unmarshal(hostResult.Payload, &result); err != nil || productfulfillmentcap.ValidateResult(claimed.Action, result) != nil ||
		(claimed.Action == productfulfillmentcap.ActionQuery && claimed.PreviousExternalReference != "" && result.ExternalReference != "" && result.ExternalReference != claimed.PreviousExternalReference) {
		return nil, rawResult, true, errors.New("provider returned an invalid fulfillment result")
	}
	return result, rawResult, true, nil
}

func (w *fulfillmentWorker) finalize(ctx context.Context, claimed *claimedFulfillment, result *productfulfillmentcap.Result, rawResult []byte, invokeFailed, providerBoundaryCrossed, resourceManualReview bool, resourceOutcome *resourceInvocation, protectedEvidence *sealedProviderEvidence) (workerDecision, error) {
	decision := workerDecision{}
	err := w.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var intent *struct {
			StatusCode string
			Version    int64
		}
		if err := w.database.GetScan(txCtx, &intent, `SELECT status_code,version FROM plugin_idc_core_product_fulfillment_intent WHERE tenant_id=0 AND intent_no=? LIMIT 1 FOR UPDATE`, claimed.IntentNo); err != nil || intent == nil {
			return errorsNew("product_fulfillment_task_conflict")
		}
		var current *struct {
			StatusCode, PhaseCode, LeaseToken string
			TakeoverRequested                 bool
			CancelRequested                   bool
			Version                           int64
		}
		if err := w.database.GetScan(txCtx, &current, `SELECT status_code,phase_code,lease_token,takeover_requested,cancel_requested,version FROM plugin_idc_core_product_fulfillment_task WHERE tenant_id=0 AND task_no=? LIMIT 1 FOR UPDATE`, claimed.TaskNo); err != nil || current == nil || current.LeaseToken != claimed.LeaseToken {
			return errorsNew("product_fulfillment_task_conflict")
		}
		decision = decideWorkerResult(claimed.PreviousStatus, claimed.PreviousPhase, claimed.Action, result, invokeFailed, providerBoundaryCrossed, current.TakeoverRequested, current.CancelRequested, claimed.QueryAttempts, claimed.SubmitAttempts)
		if resourceManualReview {
			decision = workerDecision{TaskStatus: "manual_takeover", IntentStatus: "manual_pending", PhaseCode: "frozen_unknown"}
		}
		stored := result
		resultCode := "provider_result"
		if invokeFailed || stored == nil {
			resultCode = "provider_result_unavailable"
			stored = &productfulfillmentcap.Result{Version: productfulfillmentcap.RequestVersion, Status: productfulfillmentcap.StatusResultUnknown}
		}
		safeEvidence, marshalErr := json.Marshal(map[string]any{
			"status": stored.Status, "externalReference": stored.ExternalReference,
		})
		if marshalErr != nil {
			return marshalErr
		}
		resultDigest := sha256Hex(rawResult)
		if len(rawResult) == 0 {
			resultDigest = sha256Hex(safeEvidence)
		}
		attemptResult, err := w.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_attempt SET finished_at=CURRENT_TIMESTAMP,result_code=?,result_evidence_digest=?,result_status_code=?,external_reference=?,safe_result_evidence=?::jsonb,safe_summary=? WHERE tenant_id=0 AND task_no=? AND attempt_no=? AND finished_at IS NULL`, resultCode, resultDigest, stored.Status, stored.ExternalReference, safeEvidence, resultCode, claimed.TaskNo, claimed.AttemptNo)
		if err != nil || affected(attemptResult) != 1 {
			if err != nil {
				return err
			}
			return errorsNew("product_fulfillment_task_conflict")
		}
		if err = w.storeProtectedProviderEvidence(txCtx, claimed, protectedEvidence); err != nil {
			return err
		}
		if err = w.enqueueResourceOutcome(txCtx, claimed, stored, resourceManualReview, resourceOutcome, resultDigest); err != nil {
			return err
		}
		nextExternalReference := claimed.PreviousExternalReference
		if stored.ExternalReference != "" {
			nextExternalReference = stored.ExternalReference
		}
		taskResult, err := w.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code=?,phase_code=?,next_attempt_at=CURRENT_TIMESTAMP+INTERVAL '30 seconds',lease_token='',lease_expires_at=NULL,external_reference=?,last_result_code=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND task_no=? AND version=? AND lease_token=? AND status_code IN ('querying','submitting')`, decision.TaskStatus, decision.PhaseCode, nextExternalReference, resultCode, claimed.TaskNo, current.Version, claimed.LeaseToken)
		if err != nil || affected(taskResult) != 1 {
			return errorsNew("product_fulfillment_task_conflict")
		}
		if err = w.recordCancellationTransition(txCtx, claimed.IntentNo, current.CancelRequested, decision); err != nil {
			return err
		}
		return w.updateIntentStatus(txCtx, claimed.IntentNo, intent.StatusCode, intent.Version, decision.IntentStatus)
	})
	return decision, err
}

func (w *fulfillmentWorker) storeProtectedProviderEvidence(ctx context.Context, claimed *claimedFulfillment, evidence *sealedProviderEvidence) error {
	if evidence == nil {
		return nil
	}
	_, err := w.database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_attempt_provider_evidence(tenant_id,task_no,attempt_no,key_id,nonce,ciphertext,evidence_digest) VALUES(0,?,?,?,?,?,?)`, claimed.TaskNo, claimed.AttemptNo, evidence.KeyID, evidence.Nonce, evidence.Ciphertext, evidence.Digest)
	return err
}

func (w *fulfillmentWorker) enqueueResourceOutcome(ctx context.Context, claimed *claimedFulfillment, result *productfulfillmentcap.Result, manualReview bool, invocation *resourceInvocation, evidenceDigest string) error {
	if invocation == nil || invocation.outcomeRequest == nil {
		return nil
	}
	status := provisionresourcecap.OutcomeStatusFailed
	if result.Status == productfulfillmentcap.StatusSucceeded {
		status = provisionresourcecap.OutcomeStatusSucceeded
	} else if manualReview {
		status = provisionresourcecap.OutcomeStatusManualReview
	}
	report := provisionresourcecap.OutcomeReport{
		ProviderTaskNo: invocation.providerTaskNo, Request: *invocation.outcomeRequest,
		Status: status, TaskEvidenceDigest: invocation.taskEvidenceDigest, CoreEvidenceDigest: evidenceDigest,
	}
	if provisionresourcecap.ValidateOutcomeReport(report) != nil {
		return errorsNew("product_fulfillment_resource_outcome_invalid")
	}
	payload, err := json.Marshal(report)
	if err != nil {
		return err
	}
	insert, err := w.database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_resource_outcome_outbox(tenant_id,task_no,provider_task_no,report_payload,report_digest,status_code,next_attempt_at) VALUES(0,?,?,?,?,'pending',CURRENT_TIMESTAMP) ON CONFLICT(task_no) DO NOTHING`, claimed.TaskNo, report.ProviderTaskNo, payload, sha256Hex(payload))
	if err != nil || affected(insert) != 1 {
		return errorsNew("product_fulfillment_resource_outcome_conflict")
	}
	return nil
}

func (w *fulfillmentWorker) recordCancellationTransition(ctx context.Context, intentNo string, requested bool, decision workerDecision) error {
	if !requested {
		return nil
	}
	var reasonCode, eventCode, digestPrefix string
	if decision.IntentStatus == "canceled" {
		reasonCode, eventCode, digestPrefix = SystemReasonOrderCancelled, "cancelled", "cancelled"
	} else if decision.TaskStatus == "cancellation_attention" {
		reasonCode, eventCode, digestPrefix = SystemReasonCancellationRequiresAttention, "moved_to_manual", "manual-cancel"
	} else {
		return nil
	}
	reason, err := w.systemReasons.ResolveSystemTransitionReason(reasonCode)
	if err != nil {
		return err
	}
	_, err = w.database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_system_transition(tenant_id,transition_no,intent_no,event_code,reason_code,reason_label_zh_cn,reason_label_en_us) VALUES(0,'pfs_'||substr(md5(?||?),1,32),?,?,?,?,?) ON CONFLICT(intent_no,event_code) DO NOTHING`, digestPrefix, intentNo, intentNo, eventCode, reasonCode, reason.LabelZhCN, reason.LabelEnUS)
	return err
}

func (w *fulfillmentWorker) updateIntentStatus(ctx context.Context, intentNo, currentStatus string, version int64, nextStatus string) error {
	if nextStatus == "" || nextStatus == currentStatus {
		return nil
	}
	result, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND intent_no=? AND version=?`, nextStatus, intentNo, version)
	if err != nil || affected(result) != 1 {
		return errorsNew("product_fulfillment_task_conflict")
	}
	return nil
}

func workerLeaseToken(random io.Reader) (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(random, value); err != nil {
		return "", err
	}
	return "pfl_" + hex.EncodeToString(value), nil
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}
