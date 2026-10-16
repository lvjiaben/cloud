package serveroperation

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

func (s *batchManagementImpl) Execute(ctx context.Context,
	input BatchExecutionInput) (*ServiceBatch, error) {
	actor, grants, err := s.management.authorize(ctx, []string{
		PermissionServerBatchSafe, PermissionServerBatchDangerous,
	})
	if err != nil || actor <= 0 || !validReference(input.BatchNo, 40) || len(input.PreviewDigest) != 64 {
		return nil, bizerr.NewCode(CodeServerBatchNotAccessible)
	}
	batch, err := s.loadBatch(ctx, input.BatchNo, actor)
	if err != nil || batch.PreviewDigest != input.PreviewDigest {
		return nil, bizerr.NewCode(CodeServerBatchConflict)
	}
	permission := batchPermission(batch.Action)
	if permission == "" || !grants[permission] {
		return nil, bizerr.NewCode(CodeServerBatchNotAccessible)
	}
	if batch.Status == "completed" {
		return batch, nil
	}
	if batch.Status == "previewed" {
		if batch.Dangerous {
			if strings.TrimSpace(input.ProofToken) == "" {
				return nil, bizerr.NewCode(CodeServerBatchInvalid)
			}
			if _, err = s.recentAuth.Consume(ctx, input.ProofToken, batchReauthBinding(batch)); err != nil {
				return nil, err
			}
		} else if input.ProofToken != "" {
			return nil, bizerr.NewCode(CodeServerBatchInvalid)
		}
		result, updateErr := s.management.directory.database.Exec(ctx, `UPDATE plugin_idc_core_server_service_batch
SET status_code='executing',confirmed_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND batch_no=? AND requested_by=? AND status_code='previewed'
 AND preview_digest=?`, batch.BatchNo, actor, batch.PreviewDigest)
		if updateErr != nil {
			return nil, wrapBatchUnavailable(updateErr)
		}
		affected, countErr := result.RowsAffected()
		if countErr != nil || affected != 1 {
			return nil, bizerr.NewCode(CodeServerBatchConflict)
		}
	}
	for _, item := range batch.Items {
		if item.ExecutionStatus != "pending" {
			continue
		}
		if item.PreviewStatus != "eligible" {
			if finishErr := s.finishBatchItem(ctx, batch.BatchNo, item.LineNumber,
				"skipped", item.FailureCode, "", ""); finishErr != nil {
				return nil, wrapBatchUnavailable(finishErr)
			}
			continue
		}
		operationNo, eventNo, executionErr := s.executeBatchItem(ctx, batch, item, actor)
		status, failure := "succeeded", ""
		if executionErr != nil {
			status, failure = batchFailureStatus(batch.Dangerous), batchExecutionError(executionErr)
		}
		if finishErr := s.finishBatchItem(ctx, batch.BatchNo, item.LineNumber, status,
			failure, operationNo, eventNo); finishErr != nil {
			return nil, wrapBatchUnavailable(finishErr)
		}
	}
	if _, err = s.management.directory.database.Exec(ctx, `UPDATE plugin_idc_core_server_service_batch
SET status_code='completed',completed_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND batch_no=? AND requested_by=? AND status_code='executing'
 AND NOT EXISTS (SELECT 1 FROM plugin_idc_core_server_service_batch_item item
  WHERE item.batch_no=plugin_idc_core_server_service_batch.batch_no
   AND item.execution_status_code='pending')`, batch.BatchNo, actor); err != nil {
		return nil, wrapBatchUnavailable(err)
	}
	return s.loadBatch(ctx, batch.BatchNo, actor)
}

func batchFailureStatus(dangerous bool) string {
	if dangerous {
		return "manual_review"
	}
	return "failed"
}

func (s *batchManagementImpl) executeBatchItem(ctx context.Context, batch *ServiceBatch,
	item BatchImpactItem, actor int) (string, string, error) {
	switch batch.Action {
	case BatchActionTagAdd, BatchActionTagRemove:
		return s.executeBatchTag(ctx, batch, item, actor)
	case BatchActionClassificationSet:
		return s.executeBatchClassification(ctx, batch, item, actor)
	case BatchActionNoteAppend:
		return s.executeBatchNote(ctx, batch, item, actor)
	case BatchActionRenewalReminder:
		return s.executeBatchReminder(ctx, batch, item, actor)
	case BatchActionReboot, BatchActionReinstall:
		action := serveroperationresourcecap.ActionReboot
		if batch.Action == BatchActionReinstall {
			action = serveroperationresourcecap.ActionReinstall
		}
		operation, err := s.management.operations.Create(ctx, Command{
			CustomerNo: item.CustomerNo, ServiceNo: item.ServiceNo, Action: action,
			Parameters: item.Parameters, IdempotencyKey: batch.BatchNo + ":" + item.ServiceNo,
			ActorType: "staff", ActorReference: strconv.Itoa(actor),
			RequiredPermission: PermissionServerBatchDangerous,
		})
		if err != nil || operation == nil {
			return "", "", err
		}
		return operation.OperationNo, "", nil
	case BatchActionSuspend, BatchActionTerminate:
		taskNo, err := s.createManualLifecycleTask(ctx, batch, item, actor)
		return taskNo, "", err
	case BatchActionIPChange:
		checkout, err := s.checkoutIPChangeForStaff(ctx, item.CustomerNo, item.Parameters,
			batch.BatchNo+":"+item.ServiceNo, actor)
		if err != nil || checkout == nil {
			return "", "", err
		}
		return checkout.OperationNo, "", nil
	default:
		return "", "", bizerr.NewCode(CodeServerBatchInvalid)
	}
}

func (s *batchManagementImpl) executeBatchClassification(ctx context.Context, batch *ServiceBatch,
	item BatchImpactItem, actor int) (string, string, error) {
	classificationNo, ok := batchClassificationReference(item.Parameters)
	if !ok {
		return "", "", bizerr.NewCode(CodeServerBatchInvalid)
	}
	operationNo, _ := deterministicReference("sbc_", struct{ BatchNo, ServiceNo string }{batch.BatchNo, item.ServiceNo})
	err := s.management.directory.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var before string
		if err := s.management.directory.database.GetScan(ctx, &before, `SELECT classification_no FROM plugin_idc_core_server_service_classification WHERE tenant_id=0 AND service_no=? AND customer_no=? FOR UPDATE`, item.ServiceNo, item.CustomerNo); err != nil {
			return err
		}
		result, err := s.management.directory.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_service_classification
(tenant_id,service_no,customer_no,classification_no,assigned_by,created_at,updated_at)
SELECT 0,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP
FROM plugin_idc_core_server_classification classification
WHERE classification.tenant_id=0 AND classification.classification_no=? AND classification.status_code='active'
ON CONFLICT (service_no) DO UPDATE SET classification_no=EXCLUDED.classification_no,
customer_no=EXCLUDED.customer_no,assigned_by=EXCLUDED.assigned_by,updated_at=CURRENT_TIMESTAMP`, item.ServiceNo, item.CustomerNo,
			classificationNo, actor, classificationNo)
		if err != nil {
			return err
		}
		affected, countErr := result.RowsAffected()
		if countErr != nil || affected != 1 {
			return bizerr.NewCode(CodeServerBatchConflict)
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"classification_no": before},
			IdempotencyScope: "server.batch.item", IdempotencyKey: batch.BatchNo + ":" + item.ServiceNo,
			OperationCode: "server.batch.classification_set", TargetType: "server_service",
			TargetNo: item.ServiceNo, CustomerNo: item.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "staff_batch_classification", RiskCode: "low",
			RequiredPermission: PermissionServerBatchSafe, CorrelationNo: operationNo,
			AfterSnapshot: map[string]any{"classification_no": classificationNo},
		})
	})
	return operationNo, "", err
}

func (s *batchManagementImpl) executeBatchTag(ctx context.Context, batch *ServiceBatch,
	item BatchImpactItem, actor int) (string, string, error) {
	var parameters struct {
		Tag string `json:"tag"`
	}
	if json.Unmarshal(item.Parameters, &parameters) != nil || !validTag(parameters.Tag) {
		return "", "", bizerr.NewCode(CodeServerBatchInvalid)
	}
	operationNo, _ := deterministicReference("sbo_", struct{ BatchNo, ServiceNo string }{batch.BatchNo, item.ServiceNo})
	err := s.management.directory.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var tagCount int
		if err := s.management.directory.database.GetScan(ctx, &tagCount, `SELECT COUNT(1) FROM plugin_idc_core_server_service_tag WHERE tenant_id=0 AND service_no=? AND customer_no=? AND tag_code=?`, item.ServiceNo, item.CustomerNo, parameters.Tag); err != nil {
			return err
		}
		if batch.Action == BatchActionTagAdd {
			_, err := s.management.directory.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_service_tag
(tenant_id,service_no,customer_no,tag_code,created_by)
VALUES (0,?,?,?,?) ON CONFLICT (service_no,tag_code) DO NOTHING`, item.ServiceNo,
				item.CustomerNo, parameters.Tag, actor)
			if err != nil {
				return err
			}
		} else {
			if _, err := s.management.directory.database.Exec(ctx, `DELETE FROM plugin_idc_core_server_service_tag
WHERE tenant_id=0 AND service_no=? AND customer_no=? AND tag_code=?`, item.ServiceNo,
				item.CustomerNo, parameters.Tag); err != nil {
				return err
			}
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"tag": parameters.Tag, "present": tagCount > 0},
			IdempotencyScope: "server.batch.item", IdempotencyKey: batch.BatchNo + ":" + item.ServiceNo,
			OperationCode: "server.batch." + string(batch.Action), TargetType: "server_service",
			TargetNo: item.ServiceNo, CustomerNo: item.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "staff_batch_metadata", RiskCode: "low",
			RequiredPermission: PermissionServerBatchSafe, CorrelationNo: operationNo,
			AfterSnapshot: map[string]any{"tag": parameters.Tag, "action": batch.Action},
		})
	})
	return operationNo, "", err
}

func (s *batchManagementImpl) executeBatchNote(ctx context.Context, batch *ServiceBatch,
	item BatchImpactItem, actor int) (string, string, error) {
	var parameters struct {
		Note string `json:"note"`
	}
	if json.Unmarshal(item.Parameters, &parameters) != nil || strings.TrimSpace(parameters.Note) != parameters.Note {
		return "", "", bizerr.NewCode(CodeServerBatchInvalid)
	}
	idempotency, _ := digestValue(struct{ BatchNo, ServiceNo string }{batch.BatchNo, item.ServiceNo})
	requestDigest, _ := digestValue(struct{ ServiceNo, Note string }{item.ServiceNo, parameters.Note})
	noteNo, _ := deterministicReference("sbn_", struct{ BatchNo, ServiceNo string }{batch.BatchNo, item.ServiceNo})
	err := s.management.directory.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, err := s.management.directory.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_service_note
(tenant_id,note_no,service_no,customer_no,content,created_by,idempotency_digest,request_digest)
VALUES (0,?,?,?,?,?,?,?) ON CONFLICT (created_by,service_no,idempotency_digest) DO NOTHING`,
			noteNo, item.ServiceNo, item.CustomerNo, parameters.Note, actor, idempotency, requestDigest)
		if err != nil {
			return err
		}
		if affected, countErr := result.RowsAffected(); countErr != nil {
			return countErr
		} else if affected == 0 {
			value, loadErr := s.management.directory.database.GetValue(ctx, `SELECT request_digest
FROM plugin_idc_core_server_service_note
WHERE tenant_id=0 AND created_by=? AND service_no=? AND idempotency_digest=?`, actor,
				item.ServiceNo, idempotency)
			if loadErr != nil || value.String() != requestDigest {
				return bizerr.NewCode(CodeServerBatchConflict)
			}
			return nil
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"note_no": ""},
			IdempotencyScope: "server.batch.item", IdempotencyKey: batch.BatchNo + ":" + item.ServiceNo,
			OperationCode: "server.batch.note_append", TargetType: "server_service",
			TargetNo: item.ServiceNo, CustomerNo: item.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "staff_batch_note", RiskCode: "low",
			RequiredPermission: PermissionServerBatchSafe, CorrelationNo: noteNo,
			AfterSnapshot: map[string]any{"note_no": noteNo},
		})
	})
	return noteNo, "", err
}

func (s *batchManagementImpl) executeBatchReminder(ctx context.Context, batch *ServiceBatch,
	item BatchImpactItem, actor int) (string, string, error) {
	var contact *struct{ ContactNo string }
	err := s.management.directory.database.GetScan(ctx, &contact, `SELECT role.contact_no
FROM plugin_idc_core_customer_contact_role role
JOIN plugin_idc_core_customer_contact contact
 ON contact.tenant_id=role.tenant_id AND contact.customer_no=role.customer_no
 AND contact.contact_no=role.contact_no AND contact.status_code='active'
WHERE role.tenant_id=0 AND role.customer_no=? AND role.role_code='billing' AND role.status_code='active'
ORDER BY role.id LIMIT 1`, item.CustomerNo)
	if err != nil || contact == nil || contact.ContactNo == "" {
		return "", "", bizerr.NewCode(CodeServerBatchUnavailable)
	}
	correlationNo, _ := deterministicReference("sbr_", struct{ BatchNo, ServiceNo string }{batch.BatchNo, item.ServiceNo})
	var receipt notificationcap.EventReceipt
	err = s.management.directory.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		receipt, err = s.notifications.Enqueue(ctx, notificationcap.EventRequest{
			EventCode: "service.renewal.manual_reminder",
			Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact,
				CustomerNo: item.CustomerNo, ContactNo: contact.ContactNo},
			BusinessObjectType: notificationcap.BusinessObjectService,
			BusinessObjectNo:   item.ServiceNo, BusinessObjectVersion: item.ServiceVersion,
			Variables: map[string]string{"service_no": item.ServiceNo,
				"expires_at": formatOptionalTime(item.EntitlementExpiresAt)},
			CorrelationNo: correlationNo, IdempotencyKey: correlationNo,
		})
		if err != nil {
			return err
		}
		if receipt.EventNo == "" {
			return bizerr.NewCode(CodeServerBatchUnavailable)
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"notification_event_no": ""},
			IdempotencyScope: "server.batch.item", IdempotencyKey: batch.BatchNo + ":" + item.ServiceNo,
			OperationCode: "server.batch.renewal_reminder", TargetType: "server_service",
			TargetNo: item.ServiceNo, CustomerNo: item.CustomerNo, ResultCode: idccap.BusinessAuditResultSucceeded,
			ReasonCode: "staff_batch_renewal_reminder", RiskCode: "low",
			RequiredPermission: PermissionServerBatchSafe, CorrelationNo: correlationNo,
			AfterSnapshot: map[string]any{"notification_event_no": receipt.EventNo},
		})
	})
	return correlationNo, receipt.EventNo, err
}

func (s *batchManagementImpl) createManualLifecycleTask(ctx context.Context, batch *ServiceBatch,
	item BatchImpactItem, actor int) (string, error) {
	action := "suspend"
	if batch.Action == BatchActionTerminate {
		action = "terminate"
	}
	taskNo, _ := deterministicReference("lct_", struct{ BatchNo, ServiceNo string }{batch.BatchNo, item.ServiceNo})
	idempotency, _ := digestValue(struct{ BatchNo, ServiceNo, Action string }{batch.BatchNo, item.ServiceNo, action})
	content, _ := digestValue(struct {
		TaskNo, CustomerNo string
		Version            int64
	}{taskNo, item.CustomerNo, item.ServiceVersion})
	err := s.management.directory.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, err := s.management.directory.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_lifecycle_task
(tenant_id,task_no,service_no,customer_no,currency_code,policy_no,policy_version,action_code,
 expected_service_version,expected_entitlement_expires_at,due_at,status_code,idempotency_digest,content_digest)
SELECT 0,?,?,?,?,service.current_lifecycle_policy_no,service.current_lifecycle_policy_version,?,
 service.version,service.entitlement_expires_at,CURRENT_TIMESTAMP,'pending',?,?
FROM plugin_idc_core_service_instance service
WHERE service.tenant_id=0 AND service.service_no=? AND service.customer_no=?
 AND service.version=? AND service.current_lifecycle_policy_no IS NOT NULL
 AND service.current_lifecycle_policy_version IS NOT NULL AND service.entitlement_expires_at IS NOT NULL
ON CONFLICT (service_no,action_code,idempotency_digest) DO NOTHING`, taskNo, item.ServiceNo,
			item.CustomerNo, item.CurrencyCode, action, idempotency, content, item.ServiceNo,
			item.CustomerNo, item.ServiceVersion)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			value, loadErr := s.management.directory.database.GetValue(ctx, `SELECT task_no
FROM plugin_idc_core_service_lifecycle_task
WHERE tenant_id=0 AND service_no=? AND action_code=? AND idempotency_digest=?`, item.ServiceNo,
				action, idempotency)
			if loadErr != nil || value.String() != taskNo {
				return bizerr.NewCode(CodeServerBatchConflict)
			}
			return nil
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "server.batch.item", IdempotencyKey: batch.BatchNo + ":" + item.ServiceNo,
			OperationCode: "server.batch." + action, TargetType: "server_service",
			TargetNo: item.ServiceNo, CustomerNo: item.CustomerNo, ResultCode: idccap.BusinessAuditResultSucceeded,
			ReasonCode: "staff_batch_lifecycle", RiskCode: "high",
			RequiredPermission: PermissionServerBatchDangerous, CorrelationNo: taskNo,
			BeforeSnapshot: map[string]any{"runtime_status": item.RuntimeStatus, "service_version": item.ServiceVersion},
			AfterSnapshot:  map[string]any{"lifecycle_task_no": taskNo, "action": action},
		})
	})
	return taskNo, err
}

func (s *batchManagementImpl) finishBatchItem(ctx context.Context, batchNo string, line int,
	status, failure, operationNo, eventNo string) error {
	var operation, event any
	if operationNo != "" {
		operation = operationNo
	}
	if eventNo != "" {
		event = eventNo
	}
	result, err := s.management.directory.database.Exec(ctx, `UPDATE plugin_idc_core_server_service_batch_item
SET execution_status_code=?,failure_code=?,operation_no=?,notification_event_no=?,
 executed_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND batch_no=? AND line_number=? AND execution_status_code='pending'`,
		status, failure, operation, event, batchNo, line)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return bizerr.NewCode(CodeServerBatchConflict)
	}
	return nil
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}
