package serveroperation

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type pendingAccessoryBinding struct {
	EntitlementNo, ServiceNo, CustomerNo, AccessoryKind, RequestedName string
	ChannelNo, PoolNo, BillingModeCode, PeriodUnitCode                 string
	BaselineObjectNos, BaselineSnapshotNo                              string
	PeriodCount                                                        int
	BaselineObservedAt, CompletedAt                                    time.Time
}

func (w *workerImpl) reconcileAccessoryBindings(ctx context.Context, limit int) (int, int, error) {
	if w.directory == nil {
		return 0, 0, nil
	}
	rows := make([]pendingAccessoryBinding, 0, limit)
	if err := w.database.GetScan(ctx, &rows, `SELECT entitlement.entitlement_no,entitlement.service_no,
entitlement.customer_no,entitlement.accessory_kind,entitlement.requested_name,
entitlement.billing_mode_code,entitlement.period_unit_code,entitlement.period_count,
entitlement.baseline_object_nos::TEXT AS baseline_object_nos,
entitlement.baseline_snapshot_no,entitlement.baseline_observed_at,
operation.completed_at,service.channel_no,service.pool_no
FROM plugin_idc_core_server_accessory_entitlement entitlement
JOIN plugin_idc_core_business_operation operation
 ON operation.tenant_id=entitlement.tenant_id AND operation.operation_no=entitlement.create_operation_no
JOIN plugin_idc_core_service_instance service
 ON service.tenant_id=entitlement.tenant_id AND service.service_no=entitlement.service_no
WHERE entitlement.tenant_id=0 AND entitlement.status_code='pending_binding'
 AND operation.status_code='succeeded' AND operation.completed_at IS NOT NULL
ORDER BY operation.completed_at,entitlement.id LIMIT ?`, limit); err != nil {
		return 0, 0, gerror.Wrap(err, "list pending accessory bindings")
	}
	bound, manual := 0, 0
	for index := range rows {
		if ctx.Err() != nil {
			return bound, manual, ctx.Err()
		}
		outcome, err := w.reconcileAccessoryBinding(ctx, &rows[index])
		if err != nil {
			return bound, manual, err
		}
		switch outcome {
		case "bound":
			bound++
		case "manual_review":
			manual++
		}
	}
	return bound, manual, nil
}

func (w *workerImpl) reconcileAccessoryBinding(ctx context.Context, row *pendingAccessoryBinding) (string, error) {
	var baseline []string
	if row == nil || json.Unmarshal([]byte(row.BaselineObjectNos), &baseline) != nil {
		return "", ErrInvalidInput
	}
	directory, err := w.directory.GetAccessories(ctx, serveroperationresourcecap.CapabilityRoute{
		ServiceNo: row.ServiceNo, ChannelNo: row.ChannelNo, PoolNo: row.PoolNo,
	})
	if err != nil {
		return "", err
	}
	now, err := databaseNow(ctx, w.database)
	if err != nil {
		return "", err
	}
	if directory == nil || directory.SnapshotNo == row.BaselineSnapshotNo ||
		!directory.ObservedAt.After(row.BaselineObservedAt) || !directory.ExpiresAt.After(now) {
		return "pending", nil
	}
	seen := make(map[string]struct{}, len(baseline))
	for _, objectNo := range baseline {
		seen[objectNo] = struct{}{}
	}
	candidates := make([]serveroperationresourcecap.AccessoryItem, 0, 2)
	for _, item := range directory.Items {
		if string(item.Kind) != row.AccessoryKind || !validReference(item.ObjectNo, 128) {
			continue
		}
		if _, existed := seen[item.ObjectNo]; !existed {
			candidates = append(candidates, item)
		}
	}
	if len(candidates) > 1 && row.RequestedName != "" {
		named := candidates[:0]
		for _, candidate := range candidates {
			if candidate.Name == row.RequestedName {
				named = append(named, candidate)
			}
		}
		candidates = named
	}
	if len(candidates) == 0 {
		return "pending", nil
	}
	if len(candidates) != 1 {
		return "manual_review", w.moveAccessoryBindingToManualReview(ctx, row, directory)
	}
	return "bound", w.bindAccessoryEntitlement(ctx, row, candidates[0], directory)
}

func (w *workerImpl) bindAccessoryEntitlement(ctx context.Context, row *pendingAccessoryBinding,
	item serveroperationresourcecap.AccessoryItem, directory *serveroperationresourcecap.AccessoryProjection) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := databaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		var startAt, endAt any
		if row.BillingModeCode == "paid" {
			start := databaseTime(row.CompletedAt)
			end, ok := addAccessoryPeriod(start, row.PeriodUnitCode, row.PeriodCount)
			if !ok {
				return ErrInvalidInput
			}
			startAt, endAt = start, end
		}
		evidence, err := digestValue(struct {
			EntitlementNo, ObjectNo, SnapshotNo string
			ObservedAt                          time.Time
		}{row.EntitlementNo, item.ObjectNo, directory.SnapshotNo, directory.ObservedAt.UTC()})
		if err != nil {
			return err
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_accessory_entitlement
SET object_no=?,status_code='active',current_period_start_at=?,current_period_end_at=?,
binding_attempt_count=binding_attempt_count+1,last_binding_attempt_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND entitlement_no=? AND status_code='pending_binding'`, item.ObjectNo,
			startAt, endAt, now, now, row.EntitlementNo); err != nil {
			return gerror.Wrap(err, "bind accessory entitlement")
		}
		eventNo, _ := deterministicReference("aee_", row.EntitlementNo+":bound:"+item.ObjectNo)
		_, err = w.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement_event
(tenant_id,event_no,entitlement_no,event_type_code,from_status_code,to_status_code,evidence_digest,occurred_at)
VALUES (0,?,?,'object_bound','pending_binding','active',?,?) ON CONFLICT (event_no) DO NOTHING`,
			eventNo, row.EntitlementNo, evidence, now)
		return gerror.Wrap(err, "record accessory binding")
	})
}

func (w *workerImpl) moveAccessoryBindingToManualReview(ctx context.Context, row *pendingAccessoryBinding,
	directory *serveroperationresourcecap.AccessoryProjection) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := databaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		evidence, err := digestValue(struct{ EntitlementNo, SnapshotNo string }{row.EntitlementNo, directory.SnapshotNo})
		if err != nil {
			return err
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_accessory_entitlement
SET status_code='manual_review',binding_attempt_count=binding_attempt_count+1,
last_binding_attempt_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND entitlement_no=? AND status_code='pending_binding'`, now, now, row.EntitlementNo); err != nil {
			return err
		}
		eventNo, _ := deterministicReference("aee_", row.EntitlementNo+":ambiguous:"+directory.SnapshotNo)
		_, err = w.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement_event
(tenant_id,event_no,entitlement_no,event_type_code,from_status_code,to_status_code,evidence_digest,occurred_at)
VALUES (0,?,?,'binding_ambiguous','pending_binding','manual_review',?,?) ON CONFLICT (event_no) DO NOTHING`,
			eventNo, row.EntitlementNo, evidence, now)
		return err
	})
}

func addAccessoryPeriod(start time.Time, unit string, count int) (time.Time, bool) {
	switch unit {
	case "day":
		return start.AddDate(0, 0, count), count > 0
	case "month":
		return start.AddDate(0, count, 0), count > 0
	case "year":
		return start.AddDate(count, 0, 0), count > 0
	default:
		return time.Time{}, false
	}
}

func markDeletedAccessoryEntitlement(ctx context.Context, database gdb.DB, row *workRow,
	request serveroperationresourcecap.TaskRequest, task *serveroperationresourcecap.Task, now time.Time) error {
	kind, supported := accessoryEntitlementKind(request.Action)
	if !supported || (request.Action != serveroperationresourcecap.ActionSnapshotDelete &&
		request.Action != serveroperationresourcecap.ActionBackupDelete) {
		return nil
	}
	var parameters struct {
		ObjectNo string `json:"objectNo"`
	}
	if json.Unmarshal(request.Parameters, &parameters) != nil || !validReference(parameters.ObjectNo, 128) {
		return ErrInvalidInput
	}
	var entitlement *struct {
		EntitlementNo string `orm:"entitlement_no"`
		StatusCode    string `orm:"status_code"`
	}
	if err := database.GetScan(ctx, &entitlement, `SELECT entitlement_no,status_code
FROM plugin_idc_core_server_accessory_entitlement
WHERE tenant_id=0 AND service_no=? AND accessory_kind=? AND object_no=? FOR UPDATE`,
		row.ServiceNo, kind, parameters.ObjectNo); err != nil {
		return err
	}
	if entitlement == nil || entitlement.StatusCode == "deleted" {
		return nil
	}
	if err := execOne(ctx, database, `UPDATE plugin_idc_core_server_accessory_entitlement
SET status_code='deleted',auto_renew_enabled=FALSE,pending_invoice_no=NULL,pending_policy_no=NULL,
pending_policy_version=NULL,pending_period_unit_code=NULL,pending_period_count=NULL,
pending_total_amount_minor=NULL,last_renewal_attempt_at=NULL,deleted_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND entitlement_no=? AND status_code=?`, now, now,
		entitlement.EntitlementNo, entitlement.StatusCode); err != nil {
		return err
	}
	evidence, err := digestValue(struct{ EntitlementNo, OperationNo, TaskEvidence string }{
		entitlement.EntitlementNo, row.OperationNo, task.EvidenceDigest})
	if err != nil {
		return err
	}
	eventNo, _ := deterministicReference("aee_", entitlement.EntitlementNo+":deleted:"+row.OperationNo)
	_, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement_event
(tenant_id,event_no,entitlement_no,event_type_code,from_status_code,to_status_code,operation_no,evidence_digest,occurred_at)
VALUES (0,?,?,'deleted',?,'deleted',?,?,?) ON CONFLICT (event_no) DO NOTHING`, eventNo,
		entitlement.EntitlementNo, entitlement.StatusCode, row.OperationNo, evidence, now)
	return err
}

func requireRestorableAccessoryEntitlement(ctx context.Context, database gdb.DB, serviceNo string,
	action serveroperationresourcecap.Action, parameters json.RawMessage) error {
	kind, supported := accessoryEntitlementKind(action)
	if !supported || (action != serveroperationresourcecap.ActionSnapshotRestore &&
		action != serveroperationresourcecap.ActionBackupRestore) {
		return nil
	}
	var input struct {
		ObjectNo string `json:"objectNo"`
	}
	if json.Unmarshal(parameters, &input) != nil || strings.TrimSpace(input.ObjectNo) == "" {
		return ErrInvalidInput
	}
	status, err := database.GetValue(ctx, `SELECT status_code
FROM plugin_idc_core_server_accessory_entitlement
WHERE tenant_id=0 AND service_no=? AND accessory_kind=? AND object_no=?`, serviceNo, kind, input.ObjectNo)
	if err != nil {
		return err
	}
	if status.String() == "payment_pending" {
		return ErrActionUnavailable
	}
	return nil
}

func accessoryEntitlementKind(action serveroperationresourcecap.Action) (string, bool) {
	switch action {
	case serveroperationresourcecap.ActionSnapshotDelete, serveroperationresourcecap.ActionSnapshotRestore:
		return string(serveroperationresourcecap.AccessoryKindSnapshot), true
	case serveroperationresourcecap.ActionBackupDelete, serveroperationresourcecap.ActionBackupRestore:
		return string(serveroperationresourcecap.AccessoryKindBackup), true
	default:
		return "", false
	}
}
