package serviceaction

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

type Worker interface {
	Run(context.Context, int) (int, error)
}

type worker struct {
	database   gdb.DB
	extensions pluginhost.BusinessExtensionRegistrar
	contexts   productautomation.ExistingBusinessRuntimeContextIssuer
	random     io.Reader
}

func NewWorker(database gdb.DB, extensions pluginhost.BusinessExtensionRegistrar, contexts productautomation.ExistingBusinessRuntimeContextIssuer) (Worker, error) {
	if database == nil || extensions == nil || contexts == nil {
		return nil, errors.New("managed service action worker dependencies are unavailable")
	}
	return &worker{database: database, extensions: extensions, contexts: contexts, random: rand.Reader}, nil
}

type claim struct {
	TaskNo, EntitlementNo, ActionKey, PreviousStatus, PreviousPhase, LeaseToken string
	PluginID, ReleaseVersion, ContractVersion, ReleaseDigest, DeclarationDigest string
	InterfaceInstanceNo                                                         string
	InterfaceConfigurationRevision                                              int64
	SourceIntentNo, OrderNo, OrderItemNo, ConfigurationSnapshotNo               string
	ExternalReference, ActionMethod, QueryMethod, QueryActionKey                string
	ConfirmedEntitlement                                                        string
	InputSnapshot, ProductConfiguration                                         json.RawMessage
	AttemptNo, QueryAttemptCount, SubmitAttemptCount                            int
}

func (w *worker) Run(ctx context.Context, limit int) (int, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > 64 {
		return 0, errors.New("managed service action worker input is invalid")
	}
	processed := 0
	for processed < limit {
		claimed, err := w.claim(ctx)
		if err != nil {
			return processed, err
		}
		if claimed == nil {
			return processed, nil
		}
		processed++
		w.execute(ctx, claimed) // one failed task must not stop the bounded batch
	}
	return processed, nil
}

func (w *worker) claim(ctx context.Context) (*claim, error) {
	var selected *claim
	err := w.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		// An expired submit may have crossed the supplier boundary. It is never
		// submitted again; only an explicit takeover can resolve it.
		if _, err := w.database.Exec(txCtx, `UPDATE plugin_idc_core_managed_service_action_task SET status_code=CASE WHEN status_code='submitting' THEN 'result_unknown' ELSE 'retry_wait' END,phase_code=CASE WHEN status_code='submitting' THEN 'final' ELSE phase_code END,lease_token='',lease_expires_at=NULL,next_attempt_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND status_code IN('querying','submitting') AND lease_expires_at<=CURRENT_TIMESTAMP`); err != nil {
			return err
		}
		var row *claim
		const claimQuery = `SELECT task.task_no,task.entitlement_no,task.action_key,task.status_code AS previous_status,
task.phase_code AS previous_phase,task.plugin_id,task.plugin_release_version AS release_version,
task.contract_version,task.release_digest,task.declaration_digest,task.interface_instance_no,
task.interface_configuration_revision,task.source_intent_no,task.order_no,task.order_item_no,
task.configuration_snapshot_no,task.external_reference,task.action_method,task.query_method,
task.query_action_key,task.confirmed_entitlement,task.input_snapshot,
configuration.merchant_value_snapshot AS product_configuration,task.attempt_count+1 AS attempt_no,
task.query_attempt_count,task.submit_attempt_count
FROM plugin_idc_core_managed_service_action_task task
JOIN plugin_idc_core_product_fulfillment_intent intent
  ON intent.tenant_id=task.tenant_id AND intent.intent_no=task.source_intent_no
JOIN plugin_idc_core_catalog_product_version_configuration configuration
  ON configuration.tenant_id=intent.tenant_id AND configuration.version_no=intent.version_no
 AND configuration.interface_instance_no=task.interface_instance_no
 AND configuration.interface_configuration_revision=task.interface_configuration_revision
WHERE task.tenant_id=0 AND task.lease_token='' AND task.next_attempt_at<=CURRENT_TIMESTAMP
  AND task.status_code IN('pending_query','pending_submit','retry_wait')
  AND ((task.phase_code='submit_allowed' AND task.submit_attempt_count<1)
    OR (task.phase_code<>'submit_allowed' AND task.query_attempt_count<10))
ORDER BY task.next_attempt_at,task.id LIMIT 1 FOR UPDATE OF task SKIP LOCKED`
		err := w.database.GetScan(txCtx, &row, claimQuery)
		if err != nil || row == nil {
			return err
		}
		row.LeaseToken, err = randomReference(w.random, "lease_")
		if err != nil {
			return err
		}
		status, action := "querying", "query"
		if row.PreviousPhase == "submit_allowed" {
			status, action = "submitting", "submit"
		}
		request, err := row.request(action)
		if err != nil {
			return err
		}
		raw, _ := json.Marshal(request)
		result, err := w.database.Exec(txCtx, `UPDATE plugin_idc_core_managed_service_action_task SET status_code=?,lease_token=?,lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '60 seconds',attempt_count=attempt_count+1,query_attempt_count=query_attempt_count+CASE WHEN ?='query' THEN 1 ELSE 0 END,submit_attempt_count=submit_attempt_count+CASE WHEN ?='submit' THEN 1 ELSE 0 END,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE task_no=? AND lease_token='' AND status_code=?`, status, row.LeaseToken, action, action, row.TaskNo, row.PreviousStatus)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return errors.New("managed service action claim conflict")
		}
		_, err = w.database.Exec(txCtx, `INSERT INTO plugin_idc_core_managed_service_action_attempt(tenant_id,task_no,attempt_no,action_code,request_digest) VALUES(0,?,?,?,?)`, row.TaskNo, row.AttemptNo, action, digest(raw))
		if err != nil {
			return err
		}
		selected = row
		return nil
	})
	return selected, err
}

func (row *claim) request(action string) (*productfulfillmentcap.ManagedServiceActionRequest, error) {
	key, method, idempotency := row.ActionKey, row.ActionMethod, row.TaskNo
	if action == "query" {
		key, method, idempotency = row.QueryActionKey, row.QueryMethod, ""
	}
	_ = method
	request := &productfulfillmentcap.ManagedServiceActionRequest{Version: productfulfillmentcap.ManagedServiceActionVersion, ActionKey: key, EntitlementNo: row.EntitlementNo, IdempotencyKey: idempotency, ExternalReference: row.ExternalReference, ProductConfiguration: row.ProductConfiguration}
	if action == "submit" {
		request.Input = row.InputSnapshot
	} else {
		request.RequestedActionKey = row.ActionKey
		request.Input = row.InputSnapshot
	}
	return request, productfulfillmentcap.ValidateManagedServiceActionRequest(request, key, action == "submit")
}

func (w *worker) execute(ctx context.Context, claimed *claim) {
	action := "query"
	method := claimed.QueryMethod
	if claimed.PreviousPhase == "submit_allowed" {
		action, method = "submit", claimed.ActionMethod
	}
	request, err := claimed.request(action)
	var raw []byte
	if err == nil {
		raw, err = json.Marshal(request)
	}
	var result *productfulfillmentcap.ManagedServiceActionResult
	boundary := false
	if err == nil {
		runtime, issueErr := w.contexts.IssueExistingBusinessRuntimeContext(ctx, productautomation.ExistingBusinessRuntimeContextInput{PluginID: claimed.PluginID, ReleaseVersion: claimed.ReleaseVersion, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: claimed.ContractVersion, ReleaseDigest: claimed.ReleaseDigest, DeclarationDigest: claimed.DeclarationDigest, Method: method, InterfaceInstanceNo: claimed.InterfaceInstanceNo, InterfaceConfigurationRevision: claimed.InterfaceConfigurationRevision, IntentNo: claimed.SourceIntentNo, OrderNo: claimed.OrderNo, OrderItemNo: claimed.OrderItemNo, ConfigurationSnapshotNo: claimed.ConfigurationSnapshotNo})
		if issueErr != nil || runtime == nil || runtime.Release == nil {
			err = errors.New("managed service runtime is unavailable")
		} else {
			defer runtime.Release()
			boundary = true
			host, invokeErr := w.extensions.InvokeProviderMethod(ctx, pluginhost.BusinessExtensionMethodInvokeInput{PluginID: claimed.PluginID, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: claimed.ContractVersion, Method: method, ReleaseVersion: claimed.ReleaseVersion, ReleaseDigest: claimed.ReleaseDigest, DeclarationDigest: claimed.DeclarationDigest, Purpose: capmodel.BusinessExtensionInvocationPurposeExistingBusiness, RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: runtime.ContextID}, Payload: raw})
			if invokeErr != nil || host == nil || host.Version != capmodel.BusinessExtensionInvocationProtocolV1 || json.Unmarshal(host.Payload, &result) != nil || productfulfillmentcap.ValidateManagedServiceActionResult(result, request.ActionKey, action == "query") != nil {
				err = errors.New("managed service provider result is unavailable")
			} else if action == "query" && result.Decision == "" {
				err = errors.New("managed service provider query decision is unavailable")
			}
		}
	}
	w.finalize(ctx, claimed, action, result, err, boundary)
}

func (w *worker) finalize(ctx context.Context, claimed *claim, action string, result *productfulfillmentcap.ManagedServiceActionResult, invokeErr error, boundary bool) {
	status, phase, resultCode := "retry_wait", claimed.PreviousPhase, "provider_unavailable"
	projection := json.RawMessage(`{}`)
	if result != nil && len(result.Projection) > 0 {
		projection = result.Projection
	}
	if action == "query" {
		if invokeErr == nil {
			switch result.Decision {
			case productfulfillmentcap.ManagedServiceQueryDecisionSubmitAllowed:
				status, phase, resultCode = "pending_submit", "submit_allowed", "query_submit_allowed"
			case productfulfillmentcap.ManagedServiceQueryDecisionAlreadySucceeded:
				status, phase, resultCode = "succeeded", "final", "query_already_succeeded"
			case productfulfillmentcap.ManagedServiceQueryDecisionForbidden:
				status, phase, resultCode = "failed", "final", "query_business_forbidden"
			default:
				status, phase, resultCode = "manual_attention", "final", "query_decision_missing"
			}
		} else if claimed.QueryAttemptCount+1 >= 10 {
			status, phase = "manual_attention", "final"
		}
	} else if invokeErr == nil && result.Status == productfulfillmentcap.ManagedServiceActionStatusSucceeded {
		status, phase, resultCode = "succeeded", "final", "provider_confirmed"
	} else if boundary {
		status, phase, resultCode = "result_unknown", "final", "provider_result_unknown"
	} else {
		status, phase = "manual_attention", "final"
	}
	safe := map[string]any{"status": status, "result": resultCode}
	if len(projection) > 0 && string(projection) != "{}" {
		safe["projection"] = json.RawMessage(projection)
	}
	evidence, marshalErr := json.Marshal(safe)
	if marshalErr != nil {
		return
	}
	_ = w.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		_, err := w.database.Exec(txCtx, `UPDATE plugin_idc_core_managed_service_action_attempt SET finished_at=CURRENT_TIMESTAMP,result_digest=?,result_code=?,safe_evidence=?::jsonb WHERE task_no=? AND attempt_no=? AND finished_at IS NULL`, digest(evidence), resultCode, evidence, claimed.TaskNo, claimed.AttemptNo)
		if err != nil {
			return err
		}
		_, err = w.database.Exec(txCtx, `UPDATE plugin_idc_core_managed_service_action_task SET status_code=?,phase_code=?,safe_projection=?::jsonb,last_result_code=?,lease_token='',lease_expires_at=NULL,next_attempt_at=CURRENT_TIMESTAMP+INTERVAL '30 seconds',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE task_no=? AND lease_token=? AND status_code IN('querying','submitting')`, status, phase, projection, resultCode, claimed.TaskNo, claimed.LeaseToken)
		if err != nil {
			return err
		}
		if status == "succeeded" && claimed.ConfirmedEntitlement != "" {
			result, updateErr := w.database.Exec(txCtx, `UPDATE plugin_idc_core_service_entitlement SET status_code=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND entitlement_no=? AND status_code='pending_activation'`, claimed.ConfirmedEntitlement, claimed.EntitlementNo)
			if updateErr != nil {
				return updateErr
			}
			if affected, _ := result.RowsAffected(); affected != 1 {
				return errors.New("managed service entitlement transition conflict")
			}
		}
		return nil
	})
}
