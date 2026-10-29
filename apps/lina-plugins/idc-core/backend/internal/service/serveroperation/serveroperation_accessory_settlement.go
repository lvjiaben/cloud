// This file owns resuming a paid accessory's original server operation.

package serveroperation

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/fulfillment"
)

// accessoryOperationStatus names only the local transitions used by this consumer.
type accessoryOperationStatus string

// accessoryOrderStatus keeps the order state distinct from invoice status.
type accessoryOrderStatus string

// accessorySettlementReasonCode is the append-only operation-event reason.
type accessorySettlementReasonCode string

// Accessory settlement reuses the existing operation state machine and audit reason.
const (
	accessoryAwaitingSettlement accessoryOperationStatus              = "awaiting_financial_settlement"
	accessoryExecutionReady     accessoryOperationStatus              = "ready"
	accessoryOrderPaid          accessoryOrderStatus                  = "paid"
	accessoryProviderTaskNone   serveroperationresourcecap.TaskStatus = "none"
	accessorySettlementReason   accessorySettlementReasonCode         = "server_accessory.financially_settled"
)

// accessorySettlementConsumer owns only local materialization, never provider writes.
type accessorySettlementConsumer struct {
	database gdb.DB
	evidence billing.SettlementEvidenceReader
}

// NewAccessorySettlementConsumer injects read-only billing authority. No
// provider source or payment command is reachable from this consumer.
func NewAccessorySettlementConsumer(database gdb.DB, evidence billing.SettlementEvidenceReader) (fulfillment.AddonSettlementConsumer, error) {
	if nilServerOperationDependency(database) || nilServerOperationDependency(evidence) {
		return nil, gerror.New("accessory settlement requires database and billing evidence")
	}
	return &accessorySettlementConsumer{database: database, evidence: evidence}, nil
}

// ConsumeAddonSettlement recognizes durable addon intent before attempting
// settlement recovery. A malformed/missing addon association never becomes a
// generic purchase, and the receipt/execution/event commit atomically.
func (s *accessorySettlementConsumer) ConsumeAddonSettlement(ctx context.Context, input fulfillment.SettlementEvent) (*fulfillment.Delivery, bool, error) {
	if ctx == nil || ctx.Err() != nil || !validReference(input.OrderNo, 40) {
		return nil, false, ErrInvalidInput
	}
	var (
		result  *fulfillment.Delivery
		handled bool
	)
	err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		// Billing uses order -> invoice -> customer lock order. Locking the
		// original order first also serializes different event deliveries.
		var order *entity.CommerceOrder
		if err := s.database.Model(dao.CommerceOrder.Table()).Ctx(txCtx).
			Where(do.CommerceOrder{TenantId: 0, OrderNo: input.OrderNo}).LockUpdate().Scan(&order); err != nil {
			return err
		}
		if order == nil {
			return ErrOperationConflict
		}
		owned, err := s.database.GetValue(txCtx, accessorySettlementClassificationSQL, order.QuoteNo, order.OrderNo)
		if err != nil || !owned.Bool() {
			return err
		}
		handled = true
		proof, err := s.evidence.GetSettledOrderEvidence(txCtx, input.OrderNo)
		if err != nil {
			return err
		}
		if !accessorySettlementEventMatches(input, proof) || accessoryOrderStatus(order.StatusCode) != accessoryOrderPaid {
			return ErrOperationConflict
		}
		row, err := s.lockAccessorySettlement(txCtx, input)
		if err != nil {
			return err
		}
		if row == nil || !row.FrozenValid || !accessorySettlementAction(row.ActionCode, row.AccessoryKind) {
			return ErrOperationConflict
		}
		content, err := accessorySettlementIntentDigest(row)
		if err != nil || content != row.ContentDigest {
			return ErrOperationConflict
		}
		evidence, err := digestValue(struct {
			Event                                                               fulfillment.SettlementEvent
			OperationDigest, QuoteDigest, FinancialSourceDigest, IdentityDigest string
		}{input, row.ContentDigest, row.QuoteContentDigest, proof.SourceSnapshotDigest, proof.IdentityEvidenceDigest})
		if err != nil {
			return err
		}
		var receipt *entity.FinancialSettlementDelivery
		if err = s.database.Model(dao.FinancialSettlementDelivery.Table()).Ctx(txCtx).
			Where(do.FinancialSettlementDelivery{TenantId: 0, SettlementNo: input.SettlementNo}).Scan(&receipt); err != nil {
			return err
		}
		if receipt != nil {
			if receipt.EventNo != input.EventNo || receipt.OrderNo != input.OrderNo || receipt.InvoiceNo != input.InvoiceNo ||
				receipt.CustomerNo != input.CustomerNo || receipt.CurrencyCode != input.CurrencyCode || receipt.EvidenceDigest != evidence ||
				row.SettlementNo != input.SettlementNo || row.FinanciallySettledAt == nil || !row.FinanciallySettledAt.Equal(proof.SettledAt) {
				return ErrOperationConflict
			}
			if err = s.validateAccessorySettlementReplay(txCtx, row, receipt); err != nil {
				return err
			}
			// Replays do not check the current service version or reset a task:
			// an already completed operation may legitimately have advanced it.
			result = accessorySettlementDelivery(input, receipt.DeliveryNo, true)
			return nil
		}
		if err = s.validateAccessorySettlementReady(txCtx, row); err != nil {
			return err
		}
		now, err := databaseNow(txCtx, s.database)
		if err != nil {
			return err
		}
		result, err = s.persistAccessorySettlement(txCtx, input, row, evidence, proof.SettledAt, now)
		return err
	})
	return result, handled, err
}

// validateAccessorySettlementReplay checks the immutable receipt and original
// execution identity without writing or requiring the current service state.
func (s *accessorySettlementConsumer) validateAccessorySettlementReplay(ctx context.Context, row *accessorySettlementRow, receipt *entity.FinancialSettlementDelivery) error {
	expectedContent, err := digestValue(struct{ DeliveryNo, Evidence string }{receipt.DeliveryNo, receipt.EvidenceDigest})
	if err != nil || receipt.ContentDigest != expectedContent || receipt.FulfillmentCount != 0 ||
		receipt.ReadyCount != 0 || receipt.WaitingGateCount != 0 || receipt.ManualReviewCount != 0 {
		return ErrOperationConflict
	}
	if row.SubjectTypeCode == nativeAccessorySubject {
		return s.validateNativeAccessorySettlementReplay(ctx, row, receipt.EvidenceDigest)
	}
	var execution *entity.ServerOperationExecution
	if err = s.database.GetScan(ctx, &execution, `SELECT execution_no,operation_no,service_no,action_code,
resource_owner_code,channel_no,pool_no,expected_service_version,parameters::TEXT AS parameters,
secret_bindings::TEXT AS secret_bindings,request_digest FROM plugin_idc_core_server_operation_execution
WHERE tenant_id=0 AND operation_no=?`, row.OperationNo); err != nil {
		return err
	}
	if execution == nil || execution.ServiceNo != row.ServiceNo || execution.ActionCode != row.ActionCode ||
		execution.ExpectedServiceVersion != row.ExpectedServiceVersion || execution.Parameters != row.Parameters ||
		execution.SecretBindings != "[]" || execution.ResourceOwnerCode != provisionresourcecap.OwnerID {
		return ErrOperationConflict
	}
	expectedNo, err := deterministicReference("aox_", row.OperationNo)
	if err != nil || execution.ExecutionNo != expectedNo {
		return ErrOperationConflict
	}
	digest, err := serveroperationresourcecap.TaskRequestDigest(serveroperationresourcecap.TaskRequest{
		OperationNo: row.OperationNo, ServiceNo: row.ServiceNo, Action: serveroperationresourcecap.Action(row.ActionCode),
		ChannelNo: execution.ChannelNo, PoolNo: execution.PoolNo, ExpectedServiceVersion: row.ExpectedServiceVersion,
		Parameters: json.RawMessage(row.Parameters),
	})
	if err != nil || digest != execution.RequestDigest {
		return ErrOperationConflict
	}
	return nil
}

// accessorySettlementEventMatches treats delivery coordinates as untrusted input.
func accessorySettlementEventMatches(input fulfillment.SettlementEvent, proof *billing.FinancialSettlement) bool {
	return proof != nil && validReference(input.EventNo, 40) && proof.SettlementNo == input.SettlementNo &&
		proof.OrderNo == input.OrderNo && proof.InvoiceNo == input.InvoiceNo && proof.CustomerNo == input.CustomerNo &&
		proof.CurrencyCode == input.CurrencyCode && string(proof.Basis) == input.BasisCode &&
		proof.TotalAmountMinor > 0 && proof.TotalAmountMinor == input.TotalAmountMinor &&
		proof.SettledAt.UnixMilli() == input.SettledAtUnixMS && len(proof.Sources) == input.SourceCount
}

// accessorySettlementAction admits only the three already-priced creation actions.
func accessorySettlementAction(action, kind string) bool {
	switch serveroperationresourcecap.Action(action) {
	case serveroperationresourcecap.ActionTrafficAdd:
		return kind == string(serveroperationresourcecap.AccessoryKindTraffic)
	case serveroperationresourcecap.ActionSnapshotCreate:
		return kind == string(serveroperationresourcecap.AccessoryKindSnapshot)
	case serveroperationresourcecap.ActionBackupCreate:
		return kind == string(serveroperationresourcecap.AccessoryKindBackup)
	default:
		return false
	}
}

// accessorySettlementIntentDigest reproduces the immutable checkout intent exactly.
func accessorySettlementIntentDigest(row *accessorySettlementRow) (string, error) {
	if row.SubjectTypeCode == nativeAccessorySubject {
		return nativeAccessorySettlementIntentDigest(row)
	}
	return digestValue(struct {
		OperationNo, QuoteNo, OrderNo, InvoiceNo, ServiceNo, Action, Parameters string
		Financial                                                               bool
		ExpectedVersion                                                         int64
	}{row.OperationNo, row.QuoteNo, row.OrderNo, row.InvoiceNo, row.ServiceNo, row.ActionCode, row.Parameters, true, row.ExpectedServiceVersion})
}

// validateAccessorySettlementReady fences current ownership, state and task exclusivity.
func (s *accessorySettlementConsumer) validateAccessorySettlementReady(ctx context.Context, row *accessorySettlementRow) error {
	if row.SubjectTypeCode == nativeAccessorySubject {
		return s.validateNativeAccessorySettlementReady(ctx, row)
	}
	if accessoryOperationStatus(row.StatusCode) != accessoryAwaitingSettlement || row.SettlementNo != "" || row.FinanciallySettledAt != nil ||
		row.CurrentServiceVersion != row.ExpectedServiceVersion || row.QuoteServiceVersion+1 != row.ExpectedServiceVersion ||
		row.CustomerSubjectVersion != row.CurrentCustomerVersion || customer.Status(row.CustomerStatus) != customer.StatusActive ||
		row.LastOperationNo != row.OperationNo || row.ChannelNo == "" || row.PoolNo != row.ServicePoolNo {
		return ErrOperationConflict
	}
	service := &serviceRow{EntitlementStatusCode: row.EntitlementStatusCode,
		RuntimeStatusCode: row.RuntimeStatusCode, TrafficSuspended: row.TrafficSuspended}
	taskStatus := serveroperationresourcecap.TaskStatus(row.ProviderTaskStatus)
	if !allowedAction(service, serveroperationresourcecap.Action(row.ActionCode)) ||
		(taskStatus != accessoryProviderTaskNone && taskStatus != serveroperationresourcecap.TaskStatusSucceeded && taskStatus != serveroperationresourcecap.TaskStatusFailed) {
		return ErrOperationConflict
	}
	busy, err := s.database.GetValue(ctx, `SELECT EXISTS (
SELECT 1 FROM plugin_idc_core_server_operation_execution WHERE tenant_id=0
 AND (operation_no=? OR (service_no=? AND status_code IN ('ready','pending','running','result_unknown'))))`, row.OperationNo, row.ServiceNo)
	if err != nil {
		return err
	}
	if busy.Bool() {
		return ErrOperationConflict
	}
	return nil
}

// persistAccessorySettlement atomically appends the execution, operation event and receipt.
func (s *accessorySettlementConsumer) persistAccessorySettlement(ctx context.Context, input fulfillment.SettlementEvent,
	row *accessorySettlementRow, evidence string, settledAt, now time.Time) (*fulfillment.Delivery, error) {
	if row.SubjectTypeCode != nativeAccessorySubject {
		request := serveroperationresourcecap.TaskRequest{OperationNo: row.OperationNo, ServiceNo: row.ServiceNo,
			ChannelNo: row.ChannelNo, PoolNo: row.PoolNo, Action: serveroperationresourcecap.Action(row.ActionCode),
			ExpectedServiceVersion: row.ExpectedServiceVersion, Parameters: json.RawMessage(row.Parameters)}
		requestDigest, err := serveroperationresourcecap.TaskRequestDigest(request)
		if err != nil {
			return nil, err
		}
		executionNo, err := deterministicReference("aox_", row.OperationNo)
		if err != nil {
			return nil, err
		}
		if _, err = s.database.Model(dao.ServerOperationExecution.Table()).Ctx(ctx).Data(do.ServerOperationExecution{
			TenantId: 0, ExecutionNo: executionNo, OperationNo: row.OperationNo, ServiceNo: row.ServiceNo,
			ActionCode: row.ActionCode, ResourceOwnerCode: provisionresourcecap.OwnerID, ChannelNo: row.ChannelNo, PoolNo: row.PoolNo,
			ExpectedServiceVersion: row.ExpectedServiceVersion, OriginalRuntimeStatusCode: row.RuntimeStatusCode,
			Parameters: row.Parameters, SecretBindings: "[]", StatusCode: string(accessoryExecutionReady),
			RequestDigest: requestDigest, Version: 1,
		}).Insert(); err != nil {
			return nil, err
		}
	}
	updated, err := s.database.Model(dao.BusinessOperation.Table()).Ctx(ctx).
		Where(do.BusinessOperation{TenantId: 0, OperationNo: row.OperationNo, Version: row.Version, StatusCode: accessoryAwaitingSettlement}).
		WhereNull(dao.BusinessOperation.Columns().SettlementNo).
		Data(do.BusinessOperation{StatusCode: accessoryExecutionReady, SettlementNo: &input.SettlementNo,
			FinanciallySettledAt: &settledAt, Version: row.Version + 1}).Update()
	if err != nil {
		return nil, err
	}
	affected, err := updated.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, ErrOperationConflict
	}
	from := string(accessoryAwaitingSettlement)
	if row.SubjectTypeCode == nativeAccessorySubject {
		err = appendNativeAccessoryEvent(ctx, s.database, row.OperationNo, row.SubjectNo, row.Version+1,
			from, string(accessoryExecutionReady), string(accessorySettlementReason), input.SettlementNo, evidence)
	} else {
		err = insertEvent(ctx, s.database, row.OperationNo, row.ServiceNo, int(row.Version+1), &from,
			string(accessoryExecutionReady), string(accessorySettlementReason), string(audit.ActorSystem), input.SettlementNo, evidence, now)
	}
	if err != nil {
		return nil, err
	}
	deliveryNo, err := deterministicReference("afd_", input.SettlementNo)
	if err != nil {
		return nil, err
	}
	content, err := digestValue(struct{ DeliveryNo, Evidence string }{deliveryNo, evidence})
	if err != nil {
		return nil, err
	}
	if _, err = s.database.Model(dao.FinancialSettlementDelivery.Table()).Ctx(ctx).Data(do.FinancialSettlementDelivery{
		TenantId: 0, DeliveryNo: deliveryNo, EventNo: input.EventNo, EventType: fulfillment.FinanciallySettledEventType,
		SettlementNo: input.SettlementNo, OrderNo: input.OrderNo, InvoiceNo: input.InvoiceNo, CustomerNo: input.CustomerNo,
		CurrencyCode: input.CurrencyCode, FulfillmentCount: 0, ReadyCount: 0, WaitingGateCount: 0, ManualReviewCount: 0,
		EvidenceDigest: evidence, ContentDigest: content,
	}).Insert(); err != nil {
		return nil, err
	}
	return accessorySettlementDelivery(input, deliveryNo, false), nil
}

// accessorySettlementDelivery keeps this receipt distinct from generic fulfillment counts.
func accessorySettlementDelivery(input fulfillment.SettlementEvent, deliveryNo string, reused bool) *fulfillment.Delivery {
	return &fulfillment.Delivery{DeliveryNo: deliveryNo, EventNo: input.EventNo,
		SettlementNo: input.SettlementNo, OrderNo: input.OrderNo, Reused: reused}
}
