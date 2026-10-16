package backend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/renewal"
)

type channelServiceRow struct {
	ServiceNo              string     `orm:"service_no"`
	ServiceOwnerCustomerNo string     `orm:"service_owner_customer_no"`
	PayerCustomerNo        string     `orm:"payer_customer_no"`
	CurrencyCode           string     `orm:"currency_code"`
	StatusCode             string     `orm:"status_code"`
	ExpiresAt              *time.Time `orm:"expires_at"`
	Version                int64      `orm:"version"`
}

type channelOperationRow struct {
	OperationNo              string     `orm:"operation_no"`
	ChannelID                string     `orm:"channel_id"`
	PayerCustomerNo          string     `orm:"payer_customer_no"`
	ServiceOwnerCustomerNo   string     `orm:"service_owner_customer_no"`
	ServiceNo                string     `orm:"service_no"`
	ActionCode               string     `orm:"action_code"`
	Parameters               string     `orm:"parameters"`
	ResourceChannelNo        string     `orm:"resource_channel_no"`
	ResourcePoolNo           string     `orm:"resource_pool_no"`
	ExpectedServiceVersion   int64      `orm:"expected_service_version"`
	ExpectedExpiresAt        *time.Time `orm:"expected_expires_at"`
	ProviderTaskNo           string     `orm:"provider_task_no"`
	StatusCode               string     `orm:"status_code"`
	FailureCode              string     `orm:"failure_code"`
	RequestDigest            string     `orm:"request_digest"`
	TaskEvidenceDigest       string     `orm:"task_evidence_digest"`
	CapabilityEvidenceNo     string     `orm:"capability_evidence_no"`
	CapabilityEvidenceDigest string     `orm:"capability_evidence_digest"`
	OutputBindings           string     `orm:"output_bindings"`
	ObservedAt               time.Time  `orm:"observed_at"`
	Version                  int64      `orm:"version"`
}

type persistedChannelOutput struct {
	Name      string    `json:"name"`
	Policy    string    `json:"policy"`
	SecretRef string    `json:"secretRef"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
}

type channelRenewalParameters struct {
	RenewalCycles    int    `json:"renewalCycles"`
	SettlementMethod string `json:"settlementMethod"`
}

const channelBusinessOperationInsertSQL = `
INSERT INTO plugin_idc_core_business_operation(
 tenant_id,operation_no,operation_type_code,customer_no,payer_customer_no,
 subject_type_code,subject_no,currency_code,status_code,
 financial_settlement_required,execution_mode_code,resource_owner_code,
 channel_no,pool_no,provider_task_no,capability_evidence_no,
 capability_evidence_digest,idempotency_digest,request_digest,content_digest,
 expected_subject_version,last_error_code,execution_started_at,completed_at,
 version,created_at,updated_at)
VALUES(0,?,?,?,?,'entitlement',?,?,?,FALSE,'provider_confirmed','idc-resource',
 ?,?,?,?,?,?,?,?,?,?,?,?,1,?,?)`

const channelServiceOperationInsertSQL = `
INSERT INTO plugin_idc_core_channel_service_operation(
 tenant_id,operation_no,channel_id,payer_customer_no,service_owner_customer_no,
 service_no,action_code,parameters,resource_channel_no,resource_pool_no,
 expected_service_version,expected_expires_at,provider_task_no,status_code,failure_code,
 request_digest,task_evidence_digest,output_bindings,observed_at,completed_at,
 version,created_at,updated_at)
VALUES(0,?,?,?,?,?,?,?::JSONB,?,?,?,?,?,?,?,?,?,?::JSONB,?,?,1,?,?)`

func (runtime *channelCommerceRuntime) Lifecycle(
	ctx context.Context,
	input channelcommercecap.LifecycleInput,
) (*channelcommercecap.LifecycleResult, error) {
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.ServiceNo = strings.TrimSpace(input.ServiceNo)
	input.Operation = strings.ToLower(strings.TrimSpace(input.Operation))
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.ChannelID == "" || input.ServiceNo == "" ||
		len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 120 {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	service, err := runtime.loadChannelService(ctx, input.ChannelID, input.ServiceNo)
	if err != nil {
		return nil, err
	}
	if _, err = runtime.requirePayer(ctx, service.PayerCustomerNo); err != nil {
		return nil, err
	}
	owner, err := runtime.loadCustomer(ctx, service.ServiceOwnerCustomerNo)
	if err != nil || owner.StatusCode != "active" || owner.IdentityStatus != "verified" {
		return nil, bizerr.NewCode(commerce.CodeNotAccessible)
	}
	if input.Operation == "renew" {
		return runtime.renewChannelService(ctx, input, service)
	}
	if input.Operation == "reconfigure" || input.Operation == "change_ip" {
		return runtime.channelPricedLifecycle(ctx, input, service)
	}
	if !channelOperationStatusAllowed(input.Operation, service.StatusCode) {
		return nil, bizerr.NewCode(commerce.CodeConflict)
	}
	if channelOperationNeedsConfirmation(input.Operation) && !input.Confirmed {
		return nil, bizerr.NewCode(commerce.CodeConflict)
	}
	parameters, err := normalizeChannelOperationParameters(input.Parameters)
	if err != nil {
		return nil, err
	}
	return runtime.submitChannelResourceOperation(ctx, input, service, parameters)
}

func (runtime *channelCommerceRuntime) renewChannelService(
	ctx context.Context,
	input channelcommercecap.LifecycleInput,
	service *channelServiceRow,
) (*channelcommercecap.LifecycleResult, error) {
	if runtime.renewals == nil || (service.StatusCode != "active" && service.StatusCode != "expired") {
		return nil, bizerr.NewCode(renewal.CodeCapabilityUnavailable)
	}
	var parameters channelRenewalParameters
	decoder := json.NewDecoder(bytes.NewReader(input.Parameters))
	decoder.DisallowUnknownFields()
	if len(input.Parameters) == 0 || decoder.Decode(&parameters) != nil ||
		parameters.RenewalCycles < 1 || parameters.RenewalCycles > 120 ||
		(parameters.SettlementMethod != "" && parameters.SettlementMethod != "balance") {
		return nil, bizerr.NewCode(renewal.CodeInvalid)
	}
	// The outer transaction binds quote, renewal operation, invoice and balance
	// posting. No unpaid renewal survives an insufficient-balance response.
	var result *channelcommercecap.LifecycleResult
	err := runtime.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		quote, err := runtime.renewals.QuoteForChannel(ctx, service.ServiceOwnerCustomerNo, renewal.QuoteInput{
			EntitlementNo: service.ServiceNo, RenewalCycles: parameters.RenewalCycles,
			IdempotencyKey: input.IdempotencyKey + ":quote",
		})
		if err != nil {
			return err
		}
		operation, err := runtime.renewals.ConfirmForChannel(
			ctx, service.ServiceOwnerCustomerNo, service.PayerCustomerNo,
			renewal.ConfirmInput{QuoteNo: quote.QuoteNo, IdempotencyKey: input.IdempotencyKey + ":confirm"},
			channelActorReference(input.ChannelID),
		)
		if err != nil {
			return err
		}
		reused := quote.Reused || operation.Reused
		if operation.TotalAmountMinor > 0 && operation.Status == "awaiting_financial_settlement" {
			settled, settleErr := runtime.renewals.PayForChannel(
				ctx, service.ServiceOwnerCustomerNo, service.PayerCustomerNo,
				renewal.PayInput{OperationNo: operation.OperationNo}, "balance",
				channelActorReference(input.ChannelID),
			)
			if settleErr != nil {
				return settleErr
			}
			if settled == nil || settled.Operation == nil {
				return bizerr.NewCode(renewal.CodeConflict)
			}
			operation = settled.Operation
			reused = reused || settled.Reused
		}
		result = &channelcommercecap.LifecycleResult{
			OperationNo: operation.OperationNo, QuoteNo: operation.QuoteNo, Status: operation.Status,
			CurrencyCode: operation.CurrencyCode, AmountMinor: operation.TotalAmountMinor,
			PaymentRequired: operation.Status == "awaiting_financial_settlement", Reused: reused,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (runtime *channelCommerceRuntime) submitChannelResourceOperation(
	ctx context.Context,
	input channelcommercecap.LifecycleInput,
	service *channelServiceRow,
	parameters json.RawMessage,
) (*channelcommercecap.LifecycleResult, error) {
	if runtime.serverSource == nil {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	directory, ok := runtime.serverSource.(serveroperationresourcecap.EntitlementRouteDirectory)
	if !ok {
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	route, err := directory.ResolveEntitlementRoute(ctx, serveroperationresourcecap.EntitlementRouteRequest{
		ServiceNo: service.ServiceNo, CustomerNo: service.ServiceOwnerCustomerNo,
	})
	if err != nil || route == nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	action, terminate, ok := channelResourceAction(input.Operation)
	if !ok {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	requestDigest, err := channelDigest(struct {
		ChannelID, PayerCustomerNo, ServiceOwnerCustomerNo, ServiceNo, Operation string
		Parameters                                                               json.RawMessage
	}{input.ChannelID, service.PayerCustomerNo, service.ServiceOwnerCustomerNo,
		service.ServiceNo, input.Operation, parameters})
	if err != nil {
		return nil, err
	}
	operationNo := channelOperationNo(service.ServiceOwnerCustomerNo, input.ChannelID,
		input.Operation, input.IdempotencyKey)
	existing, err := runtime.loadChannelOperation(ctx, operationNo)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.RequestDigest != requestDigest || existing.ChannelID != input.ChannelID ||
			existing.ServiceNo != service.ServiceNo || existing.PayerCustomerNo != service.PayerCustomerNo {
			return nil, bizerr.NewCode(commerce.CodeConflict)
		}
		return runtime.reconcileChannelOperation(ctx, existing, terminate, true)
	}
	if terminate {
		return runtime.createChannelTermination(ctx, input, service, route, operationNo, requestDigest, parameters)
	}
	validator, ok := runtime.serverSource.(serveroperationresourcecap.ParameterValidator)
	if !ok || validator.ValidateParameters(ctx, serveroperationresourcecap.CapabilityRoute{
		ServiceNo: service.ServiceNo, ChannelNo: route.ChannelNo, PoolNo: route.PoolNo,
	}, action, parameters) != nil {
		return nil, serveroperationresourcecap.ErrCapabilityUnavailable
	}
	request := serveroperationresourcecap.TaskRequest{
		OperationNo: operationNo, ServiceNo: service.ServiceNo, ChannelNo: route.ChannelNo,
		PoolNo: route.PoolNo, Action: action, ExpectedServiceVersion: service.Version,
		Parameters: parameters,
	}
	request.RequestDigest, err = serveroperationresourcecap.TaskRequestDigest(request)
	if err != nil {
		return nil, err
	}
	task, err := runtime.serverSource.CreateTask(ctx, request)
	if err != nil || task == nil || serveroperationresourcecap.ValidateTaskForRequest(*task, request) != nil {
		if err != nil {
			return nil, err
		}
		return nil, serveroperationresourcecap.ErrSourceUnavailable
	}
	if err = runtime.persistChannelOperation(ctx, input.ChannelID, service, input.Operation,
		operationNo, requestDigest, parameters, route, task); err != nil {
		return nil, err
	}
	return projectChannelTask(operationNo, task.Status, len(task.SecretOutputs) > 0,
		task.Reused), nil
}

func (runtime *channelCommerceRuntime) createChannelTermination(
	ctx context.Context,
	input channelcommercecap.LifecycleInput,
	service *channelServiceRow,
	route *serveroperationresourcecap.EntitlementRoute,
	operationNo string,
	requestDigest string,
	parameters json.RawMessage,
) (*channelcommercecap.LifecycleResult, error) {
	if runtime.lifecycleSource == nil || service.ExpiresAt == nil {
		return nil, lifecycleresourcecap.ErrSourceUnavailable
	}
	request := lifecycleresourcecap.TaskRequest{
		LifecycleTaskNo: operationNo, ServiceNo: service.ServiceNo,
		ChannelNo: route.ChannelNo, PoolNo: route.PoolNo,
		Action:                 lifecycleresourcecap.ActionTerminate,
		ExpectedServiceVersion: service.Version, ExpectedExpiresAt: service.ExpiresAt.UTC(),
		IdempotencyDigest: channelLifecycleIdempotencyDigest(input.ChannelID, operationNo),
	}
	task, err := runtime.lifecycleSource.CreateTask(ctx, request)
	if err != nil || task == nil || lifecycleresourcecap.ValidateTask(*task) != nil {
		if err != nil {
			return nil, err
		}
		return nil, lifecycleresourcecap.ErrSourceUnavailable
	}
	serverTask := lifecycleTaskProjection(task)
	if err = runtime.persistChannelOperation(ctx, input.ChannelID, service, input.Operation,
		operationNo, requestDigest, parameters, route, serverTask); err != nil {
		return nil, err
	}
	return projectChannelTask(operationNo, serverTask.Status, false, task.Reused), nil
}

func (runtime *channelCommerceRuntime) loadChannelService(
	ctx context.Context,
	channelID string,
	serviceNo string,
) (*channelServiceRow, error) {
	var rows []channelServiceRow
	if err := runtime.database.GetScan(ctx, &rows, `
SELECT entitlement.entitlement_no AS service_no,
       entitlement.customer_no AS service_owner_customer_no,
       attribution.payer_customer_no,entitlement.currency_code,
       entitlement.status_code,entitlement.expires_at,entitlement.version
FROM plugin_idc_core_service_entitlement entitlement
JOIN plugin_idc_core_channel_order_attribution attribution
  ON attribution.tenant_id=entitlement.tenant_id AND attribution.order_no=entitlement.order_no
WHERE entitlement.tenant_id=0 AND attribution.channel_id=?
  AND entitlement.entitlement_no=?
LIMIT 2`, channelID, serviceNo); err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(commerce.CodeNotAccessible)
	}
	return &rows[0], nil
}

func normalizeChannelOperationParameters(value json.RawMessage) (json.RawMessage, error) {
	if len(value) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if len(value) > 16*1024 {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	var object map[string]any
	if json.Unmarshal(value, &object) != nil || object == nil {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	normalized, err := json.Marshal(object)
	if err != nil {
		return nil, bizerr.NewCode(commerce.CodeInvalid)
	}
	return normalized, nil
}

func channelOperationStatusAllowed(operation, status string) bool {
	if operation == "terminate" {
		return status == "active" || status == "grace" || status == "suspended" || status == "expired"
	}
	return status == "active" || status == "grace"
}

func channelOperationNeedsConfirmation(operation string) bool {
	return operation == "reconfigure" || operation == "reinstall" ||
		operation == "change_ip" || operation == "terminate"
}

func channelResourceAction(operation string) (serveroperationresourcecap.Action, bool, bool) {
	switch operation {
	case "reconfigure":
		return serveroperationresourcecap.ActionResize, false, true
	case "reinstall":
		return serveroperationresourcecap.ActionReinstall, false, true
	case "console":
		return serveroperationresourcecap.ActionConsole, false, true
	case "change_ip":
		return serveroperationresourcecap.ActionChangeIP, false, true
	case "terminate":
		return "", true, true
	default:
		return "", false, false
	}
}

func channelOperationNo(owner, channelID, operation, idempotencyKey string) string {
	digest := sha256.Sum256([]byte(owner + "\x00" + channelID + "\x00" + operation + "\x00" + idempotencyKey))
	return "COP_" + hex.EncodeToString(digest[:16])
}

func channelActorReference(channelID string) string {
	digest := sha256.Sum256([]byte(channelID))
	return "channel:" + hex.EncodeToString(digest[:16])
}

func lifecycleTaskProjection(task *lifecycleresourcecap.Task) *serveroperationresourcecap.Task {
	return &serveroperationresourcecap.Task{
		ProviderTaskNo: task.ProviderTaskNo,
		Status:         serveroperationresourcecap.TaskStatus(task.Status),
		FailureCode:    task.FailureCode, CapabilityEvidenceNo: task.CapabilityEvidenceNo,
		CapabilityEvidenceDigest: task.CapabilityEvidenceDigest,
		EvidenceDigest:           task.EvidenceDigest, ObservedAt: task.ObservedAt, Reused: task.Reused,
	}
}

func projectChannelTask(operationNo string, status serveroperationresourcecap.TaskStatus,
	outputAvailable, reused bool) *channelcommercecap.LifecycleResult {
	return &channelcommercecap.LifecycleResult{OperationNo: operationNo, Status: string(status),
		OutputAvailable: outputAvailable, Reused: reused}
}

func businessStatusForChannelTask(status serveroperationresourcecap.TaskStatus) (string, bool) {
	switch status {
	case serveroperationresourcecap.TaskStatusPending, serveroperationresourcecap.TaskStatusRunning:
		return "executing", false
	case serveroperationresourcecap.TaskStatusSucceeded:
		return "succeeded", true
	case serveroperationresourcecap.TaskStatusFailed:
		return "failed", true
	case serveroperationresourcecap.TaskStatusResultUnknown, serveroperationresourcecap.TaskStatusManualReview:
		return "manual_review", false
	default:
		return "", false
	}
}

func businessOperationType(operation string) string {
	switch operation {
	case "reconfigure":
		return "resize"
	case "change_ip":
		return "ip_change"
	default:
		return operation
	}
}

func encodeChannelOutputs(outputs []serveroperationresourcecap.SecretOutput) ([]byte, error) {
	result := make([]persistedChannelOutput, len(outputs))
	for index, output := range outputs {
		result[index] = persistedChannelOutput{Name: output.Name, Policy: string(output.Policy),
			SecretRef: output.Handle.OpaqueRef(), ExpiresAt: output.ExpiresAt}
	}
	return json.Marshal(result)
}

func (runtime *channelCommerceRuntime) persistChannelOperation(
	ctx context.Context,
	channelID string,
	service *channelServiceRow,
	operation string,
	operationNo string,
	requestDigest string,
	parameters json.RawMessage,
	route *serveroperationresourcecap.EntitlementRoute,
	task *serveroperationresourcecap.Task,
) error {
	businessStatus, terminal := businessStatusForChannelTask(task.Status)
	if businessStatus == "" {
		return serveroperationresourcecap.ErrSourceUnavailable
	}
	outputs, err := encodeChannelOutputs(task.SecretOutputs)
	if err != nil {
		return err
	}
	idempotencyDigest := sha256.Sum256([]byte(service.ServiceOwnerCustomerNo + "\x00" +
		channelID + "\x00" + operationNo))
	contentDigest, err := channelDigest(struct {
		OperationNo, ChannelID, Payer, Owner, ServiceNo, Operation, RequestDigest string
		ExpectedVersion                                                           int64
	}{operationNo, channelID, service.PayerCustomerNo, service.ServiceOwnerCustomerNo,
		service.ServiceNo, operation, requestDigest, service.Version})
	if err != nil {
		return err
	}
	now := task.ObservedAt.UTC()
	var completed any
	if terminal {
		completed = task.CompletedAt
		if task.CompletedAt.IsZero() {
			completed = now
		}
	}
	var expectedExpires any
	if operation == "terminate" && service.ExpiresAt != nil {
		expectedExpires = service.ExpiresAt.UTC()
	}
	return runtime.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if runtime.audit == nil {
			return serveroperationresourcecap.ErrSourceUnavailable
		}
		if _, err = runtime.database.Exec(ctx, channelBusinessOperationInsertSQL,
			operationNo, businessOperationType(operation),
			service.ServiceOwnerCustomerNo, service.PayerCustomerNo, service.ServiceNo,
			service.CurrencyCode, businessStatus, route.ChannelNo, route.PoolNo,
			task.ProviderTaskNo, task.CapabilityEvidenceNo, task.CapabilityEvidenceDigest,
			hex.EncodeToString(idempotencyDigest[:]), requestDigest, contentDigest,
			service.Version, task.FailureCode, now, completed, now, now); err != nil {
			return gerror.Wrap(err, "insert channel business operation")
		}
		eventContent, digestErr := channelDigest(struct {
			OperationNo, Status, Evidence, Actor string
			At                                   time.Time
		}{operationNo, businessStatus, task.EvidenceDigest, channelActorReference(channelID), now})
		if digestErr != nil {
			return digestErr
		}
		eventNo := "COE_" + strings.TrimPrefix(operationNo, "COP_")
		if _, err = runtime.database.Exec(ctx, `
INSERT INTO plugin_idc_core_business_operation_event(
 tenant_id,event_no,operation_no,subject_type_code,subject_no,event_sequence,
 from_status_code,to_status_code,reason_code,actor_type_code,actor_reference,
 evidence_digest,content_digest,created_at)
VALUES(0,?,?,'entitlement',?,1,NULL,?,'channel.operation.accepted','system',?,?,?,?)`,
			eventNo, operationNo, service.ServiceNo, businessStatus,
			channelActorReference(channelID), task.EvidenceDigest, eventContent, now); err != nil {
			return gerror.Wrap(err, "insert channel business operation event")
		}
		if _, err = runtime.database.Exec(ctx, channelServiceOperationInsertSQL, operationNo,
			channelID, service.PayerCustomerNo, service.ServiceOwnerCustomerNo,
			service.ServiceNo, businessOperationType(operation), string(parameters),
			route.ChannelNo, route.PoolNo, service.Version, expectedExpires, task.ProviderTaskNo,
			task.Status, task.FailureCode, requestDigest, task.EvidenceDigest,
			string(outputs), now, completed, now, now); err != nil {
			return gerror.Wrap(err, "insert channel service operation")
		}
		return runtime.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "channel.lifecycle.create", IdempotencyKey: operationNo,
			OperationCode: "channel.lifecycle." + operation,
			TargetType:    "service_entitlement", TargetNo: service.ServiceNo,
			CustomerNo: service.ServiceOwnerCustomerNo, ResultCode: idccap.BusinessAuditResultSucceeded,
			ReasonCode: "approved_channel_request", RiskCode: "high",
			CorrelationNo:  operationNo,
			BeforeSnapshot: map[string]any{"serviceVersion": service.Version},
			AfterSnapshot: map[string]any{"status": businessStatus,
				"payerCustomerNo": service.PayerCustomerNo, "channelReference": channelActorReference(channelID)},
		})
	})
}

func (runtime *channelCommerceRuntime) loadChannelOperation(
	ctx context.Context,
	operationNo string,
) (*channelOperationRow, error) {
	var row *channelOperationRow
	if err := runtime.database.GetScan(ctx, &row, `
SELECT operation.operation_no,operation.channel_id,operation.payer_customer_no,
 operation.service_owner_customer_no,operation.service_no,operation.action_code,
 operation.parameters::TEXT AS parameters,operation.resource_channel_no,
 operation.resource_pool_no,operation.expected_service_version,
 operation.expected_expires_at,
 operation.provider_task_no,operation.status_code,operation.failure_code,
 operation.request_digest,operation.task_evidence_digest,
 business.capability_evidence_no,business.capability_evidence_digest,
 operation.output_bindings::TEXT AS output_bindings,operation.observed_at,operation.version
FROM plugin_idc_core_channel_service_operation operation
JOIN plugin_idc_core_business_operation business
 ON business.tenant_id=operation.tenant_id AND business.operation_no=operation.operation_no
WHERE operation.tenant_id=0 AND operation.operation_no=?`, operationNo); err != nil {
		return nil, err
	}
	return row, nil
}

func (runtime *channelCommerceRuntime) reconcileChannelOperation(
	ctx context.Context,
	row *channelOperationRow,
	terminate bool,
	reused bool,
) (*channelcommercecap.LifecycleResult, error) {
	parameters := json.RawMessage(row.Parameters)
	var task *serveroperationresourcecap.Task
	var err error
	if terminate {
		if runtime.lifecycleSource == nil {
			return nil, lifecycleresourcecap.ErrSourceUnavailable
		}
		if row.ExpectedExpiresAt == nil {
			return nil, lifecycleresourcecap.ErrTaskUnavailable
		}
		request := lifecycleresourcecap.TaskRequest{
			LifecycleTaskNo: row.OperationNo, ServiceNo: row.ServiceNo,
			ChannelNo: row.ResourceChannelNo, PoolNo: row.ResourcePoolNo,
			Action:                 lifecycleresourcecap.ActionTerminate,
			ExpectedServiceVersion: row.ExpectedServiceVersion,
			ExpectedExpiresAt:      row.ExpectedExpiresAt.UTC(),
			IdempotencyDigest:      channelLifecycleIdempotencyDigest(row.ChannelID, row.OperationNo),
		}
		projected, readErr := runtime.lifecycleSource.ReadTask(ctx, lifecycleresourcecap.TaskReadRequest{
			ProviderTaskNo: row.ProviderTaskNo, Request: request,
		})
		if readErr != nil || projected == nil || lifecycleresourcecap.ValidateTask(*projected) != nil {
			return nil, errors.Join(lifecycleresourcecap.ErrTaskUnavailable, readErr)
		}
		task = lifecycleTaskProjection(projected)
	} else {
		action, _, _ := channelResourceAction(operationNameFromAction(row.ActionCode))
		request := serveroperationresourcecap.TaskRequest{
			OperationNo: row.OperationNo, ServiceNo: row.ServiceNo,
			ChannelNo: row.ResourceChannelNo, PoolNo: row.ResourcePoolNo,
			Action: action, ExpectedServiceVersion: row.ExpectedServiceVersion,
			Parameters: parameters,
		}
		request.RequestDigest, err = serveroperationresourcecap.TaskRequestDigest(request)
		if err != nil {
			return nil, err
		}
		task, err = runtime.serverSource.ReadTask(ctx, serveroperationresourcecap.TaskReadRequest{
			ProviderTaskNo: row.ProviderTaskNo, Request: request,
		})
		if err != nil || task == nil || serveroperationresourcecap.ValidateTaskForRequest(*task, request) != nil {
			return nil, errors.Join(serveroperationresourcecap.ErrTaskUnavailable, err)
		}
	}
	if task.CapabilityEvidenceNo != row.CapabilityEvidenceNo ||
		task.CapabilityEvidenceDigest != row.CapabilityEvidenceDigest {
		return nil, serveroperationresourcecap.ErrTaskConflict
	}
	if err = runtime.updateChannelOperation(ctx, row, task); err != nil {
		return nil, err
	}
	return projectChannelTask(row.OperationNo, task.Status, len(task.SecretOutputs) > 0,
		reused || task.Reused), nil
}

func channelLifecycleIdempotencyDigest(channelID, operationNo string) string {
	digest := sha256.Sum256([]byte(channelID + "\x00" + strings.TrimPrefix(operationNo, "COP_")))
	return hex.EncodeToString(digest[:])
}

func operationNameFromAction(action string) string {
	switch action {
	case "resize":
		return "reconfigure"
	case "ip_change":
		return "change_ip"
	default:
		return action
	}
}

func (runtime *channelCommerceRuntime) updateChannelOperation(
	ctx context.Context,
	row *channelOperationRow,
	task *serveroperationresourcecap.Task,
) error {
	if string(task.Status) == row.StatusCode && task.EvidenceDigest == row.TaskEvidenceDigest {
		return nil
	}
	businessStatus, terminal := businessStatusForChannelTask(task.Status)
	if businessStatus == "" {
		return serveroperationresourcecap.ErrTaskConflict
	}
	outputs, err := encodeChannelOutputs(task.SecretOutputs)
	if err != nil {
		return err
	}
	var completed any
	if terminal {
		completed = task.CompletedAt
		if task.CompletedAt.IsZero() {
			completed = task.ObservedAt
		}
	}
	return runtime.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if runtime.audit == nil {
			return serveroperationresourcecap.ErrSourceUnavailable
		}
		var current struct {
			OperationNo     string `orm:"operation_no"`
			StatusCode      string `orm:"status_code"`
			Version         int64  `orm:"version"`
			MaximumSequence int    `orm:"maximum_sequence"`
		}
		if err = runtime.database.GetScan(ctx, &current, `
SELECT business.operation_no,business.status_code,business.version,
 COALESCE((SELECT MAX(event_sequence) FROM plugin_idc_core_business_operation_event event
  WHERE event.operation_no=business.operation_no),0)::INT AS maximum_sequence
FROM plugin_idc_core_business_operation business
WHERE business.tenant_id=0 AND business.operation_no=? FOR UPDATE`, row.OperationNo); err != nil {
			return err
		}
		if current.OperationNo == "" {
			return serveroperationresourcecap.ErrTaskConflict
		}
		result, updateErr := runtime.database.Exec(ctx, `
UPDATE plugin_idc_core_channel_service_operation
SET status_code=?,failure_code=?,task_evidence_digest=?,output_bindings=?::JSONB,
 observed_at=?,completed_at=?,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND operation_no=? AND version=?`, task.Status, task.FailureCode,
			task.EvidenceDigest, string(outputs), task.ObservedAt, completed,
			row.OperationNo, row.Version)
		if updateErr != nil {
			return updateErr
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return serveroperationresourcecap.ErrTaskConflict
		}
		if current.StatusCode == businessStatus {
			return nil
		}
		result, updateErr = runtime.database.Exec(ctx, `
UPDATE plugin_idc_core_business_operation
SET status_code=?,last_error_code=?,completed_at=?,version=version+1,
 updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND operation_no=? AND version=?`, businessStatus,
			task.FailureCode, completed, row.OperationNo, current.Version)
		if updateErr != nil {
			return updateErr
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return serveroperationresourcecap.ErrTaskConflict
		}
		eventDigest := sha256.Sum256([]byte(row.OperationNo + "\x00" +
			strconv.Itoa(current.MaximumSequence+1)))
		eventNo := "COE_" + hex.EncodeToString(eventDigest[:16])
		evidence, digestErr := channelDigest(struct {
			OperationNo, From, To, TaskEvidence string
			Sequence                            int
		}{row.OperationNo, current.StatusCode, businessStatus, task.EvidenceDigest,
			current.MaximumSequence + 1})
		if digestErr != nil {
			return digestErr
		}
		_, err = runtime.database.Exec(ctx, `
INSERT INTO plugin_idc_core_business_operation_event(
 tenant_id,event_no,operation_no,subject_type_code,subject_no,event_sequence,
 from_status_code,to_status_code,reason_code,actor_type_code,actor_reference,
 evidence_digest,content_digest,created_at)
VALUES(0,?,?,'entitlement',?,?,?,?, 'channel.operation.reconciled','system',?,?,?,?)`,
			eventNo, row.OperationNo, row.ServiceNo, current.MaximumSequence+1,
			current.StatusCode, businessStatus, channelActorReference(row.ChannelID),
			task.EvidenceDigest, evidence, task.ObservedAt)
		if err != nil {
			return err
		}
		return runtime.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "channel.lifecycle.reconcile",
			IdempotencyKey:   row.OperationNo + ":" + strconv.Itoa(current.MaximumSequence+1),
			OperationCode:    "channel.lifecycle.reconcile", TargetType: "service_entitlement",
			TargetNo: row.ServiceNo, CustomerNo: row.ServiceOwnerCustomerNo,
			ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: "provider_task_reconciled",
			RiskCode: "high", CorrelationNo: row.OperationNo,
			BeforeSnapshot: map[string]any{"status": current.StatusCode},
			AfterSnapshot:  map[string]any{"status": businessStatus, "taskEvidenceDigest": task.EvidenceDigest},
		})
	})
}

func (runtime *channelCommerceRuntime) RedeemLifecycleOutput(
	ctx context.Context,
	channelID string,
	serviceNo string,
	operationNo string,
	consume channelcommercecap.ConfidentialOutputConsumer,
) error {
	if consume == nil || runtime.serverSource == nil {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	redeemer, ok := runtime.serverSource.(serveroperationresourcecap.ConfidentialOutputRedeemer)
	if !ok {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	row, err := runtime.loadChannelOperation(ctx, strings.TrimSpace(operationNo))
	if err != nil || row == nil || row.ChannelID != strings.TrimSpace(channelID) ||
		row.ServiceNo != strings.TrimSpace(serviceNo) || row.StatusCode != "succeeded" {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	var outputs []persistedChannelOutput
	if json.Unmarshal([]byte(row.OutputBindings), &outputs) != nil || len(outputs) != 1 {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	output := outputs[0]
	handle, err := secretcap.NewHandle(output.SecretRef)
	if err != nil {
		return serveroperationresourcecap.ErrOutputUnavailable
	}
	return redeemer.RedeemOutput(ctx, serveroperationresourcecap.ConfidentialOutputRequest{
		ProviderTaskNo: row.ProviderTaskNo, ChannelNo: row.ResourceChannelNo,
		Name: output.Name, Policy: serveroperationresourcecap.SecretOutputPolicy(output.Policy),
		Handle: handle, ExpiresAt: output.ExpiresAt,
	}, serveroperationresourcecap.ConfidentialOutputConsumer(consume))
}
