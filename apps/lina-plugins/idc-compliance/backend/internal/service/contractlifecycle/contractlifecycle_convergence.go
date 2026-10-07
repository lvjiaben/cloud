// This file applies supplier-authoritative contract observations and event times transactionally.

package contractlifecycle

import (
	"context"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// Apply appends exactly one timeline event and advances local state only when
// the observation is newer and agrees with the current terminal history.
func (s *serviceImpl) Apply(
	ctx context.Context,
	observation Observation,
	audit idccap.BusinessAuditWriter,
) (result ApplyResult, err error) {
	if s == nil || ctx == nil || ctx.Err() != nil || nilDependency(audit) || !validObservation(observation) {
		return ApplyResult{}, gerror.New("contract lifecycle observation is invalid")
	}
	err = s.flows.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var replayed bool
		result, replayed, err = s.replayedResult(ctx, observation)
		if err != nil || replayed {
			return err
		}

		var rows []lockedFlow
		if scanErr := s.flows.Ctx(ctx).As("f").
			InnerJoin(s.contracts.Table()+" c", "c.tenant_id = f.tenant_id AND c.contract_no = f.contract_no").
			Fields(`c.contract_no, c.order_no, c.customer_no, c.status_code AS contract_status,
f.flow_no, f.purpose_code, f.status_code AS flow_status,
COALESCE(f.provider_status, '') AS provider_status,
f.gateway_route_version, CURRENT_TIMESTAMP AS database_now`).
			Where("f.tenant_id", idccap.DefaultTenantID).
			Where("f.provider_code", providerTencentESSBasic).
			Where("f.gateway_route_no", observation.RouteNo).
			LockUpdate().Scan(&rows); scanErr != nil {
			return gerror.Wrap(scanErr, "lock contract flow for exact status")
		}
		if len(rows) != 1 || rows[0].DatabaseNow.IsZero() {
			return gerror.New("contract flow for exact status is unavailable")
		}
		locked := rows[0]
		projected, projectionErr := project(observation.ProviderStatus, locked.PurposeCode)
		if projectionErr != nil {
			return projectionErr
		}
		decision := authoritativeDecision(locked, observation, projected)
		flowStatus := locked.FlowStatus
		contractStatus := locked.ContractStatus
		if decision == "applied" {
			flowStatus = projected.FlowStatus
			if projected.ContractStatus != "" {
				contractStatus = projected.ContractStatus
			}
		}
		now := locked.DatabaseNow.UTC()
		if observation.ObservedAt.After(now.Add(5*time.Minute)) ||
			observation.SupplierObservedAt.After(now.Add(5*time.Minute)) {
			return gerror.New("contract status observation time is invalid")
		}

		if observation.RouteVersion > locked.GatewayRouteVersion {
			providerStatus := locked.ProviderStatus
			if decision == "applied" {
				providerStatus = observation.ProviderStatus
			}
			flowData := do.ContractFlow{
				StatusCode: flowStatus, ProviderStatus: &providerStatus,
				GatewayRouteVersion: observation.RouteVersion,
				Version:             gdb.Raw("version + 1"),
			}
			if flowStatus == "completed" {
				completedAt := observation.SupplierObservedAt.UTC()
				flowData.CompletedAt = &completedAt
			}
			if terminalFailureStatus(flowStatus) {
				flowData.FailureCode = projected.ReasonCode
			}
			flowUpdate, updateErr := s.flows.Ctx(ctx).
				Where(do.ContractFlow{
					TenantId: idccap.DefaultTenantID, FlowNo: locked.FlowNo,
					GatewayRouteVersion: locked.GatewayRouteVersion,
				}).Data(flowData).Update()
			if updateErr != nil {
				return gerror.Wrap(updateErr, "update exact contract flow status")
			}
			if affected, affectedErr := flowUpdate.RowsAffected(); affectedErr != nil || affected != 1 {
				return gerror.New("exact contract flow status changed concurrently")
			}
		}
		if decision == "applied" && contractStatus != locked.ContractStatus {
			contractData := do.Contract{
				StatusCode: contractStatus,
				Version:    gdb.Raw("version + 1"),
			}
			if contractStatus == "completed" {
				completedAt := observation.SupplierObservedAt.UTC()
				contractData.CompletedAt = &completedAt
			}
			if contractStatus == "terminated" {
				terminatedAt := observation.SupplierObservedAt.UTC()
				contractData.TerminatedAt = &terminatedAt
			}
			contractUpdate, updateErr := s.contracts.Ctx(ctx).
				Where(do.Contract{
					TenantId: idccap.DefaultTenantID, ContractNo: locked.ContractNo,
					StatusCode: locked.ContractStatus,
				}).Data(contractData).Update()
			if updateErr != nil {
				return gerror.Wrap(updateErr, "update exact contract status")
			}
			if affected, affectedErr := contractUpdate.RowsAffected(); affectedErr != nil || affected != 1 {
				return gerror.New("exact contract status changed concurrently")
			}
		}

		deliveryNo := nullableDelivery(observation.DeliveryNo)
		supplierObservedAt := observation.SupplierObservedAt.UTC()
		observedAt := observation.ObservedAt.UTC()
		_, insertErr := s.events.Ctx(ctx).Data(do.ContractStatusEvent{
			TenantId: idccap.DefaultTenantID, ContractNo: locked.ContractNo, FlowNo: locked.FlowNo,
			GatewayDeliveryNo: deliveryNo, GatewayRouteNo: observation.RouteNo,
			GatewayRouteVersion: observation.RouteVersion, SourceKind: observation.SourceKind,
			ProviderStatus: observation.ProviderStatus, ProjectedStatus: projectedTimelineStatus(projected),
			DecisionCode: decision, ReasonCode: projected.ReasonCode,
			SupplierObservedAt: &supplierObservedAt, ObservedAt: &observedAt,
		}).Insert()
		if insertErr != nil {
			return gerror.Wrap(insertErr, "append exact contract status timeline")
		}
		if failureErr := s.syncFailureCase(
			ctx, locked, observation, decision, contractStatus, now,
		); failureErr != nil {
			return failureErr
		}
		if auditErr := audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract.status-observation",
			IdempotencyKey:   observation.RouteNo + ":" + strconv.FormatInt(observation.RouteVersion, 10),
			OperationCode:    "compliance.contract.status-observation",
			TargetType:       "contract", TargetNo: locked.ContractNo, CustomerNo: locked.CustomerNo,
			ResultCode: decision, ReasonCode: projected.ReasonCode, RiskCode: "high",
			CorrelationNo:  locked.FlowNo,
			BeforeSnapshot: map[string]any{"contract_status": locked.ContractStatus, "flow_status": locked.FlowStatus},
			AfterSnapshot: map[string]any{
				"contract_status": contractStatus, "flow_status": flowStatus,
				"provider_status": observation.ProviderStatus,
			},
		}); auditErr != nil {
			return gerror.Wrap(auditErr, "append exact contract status audit")
		}
		result = ApplyResult{ContractNo: locked.ContractNo, FlowNo: locked.FlowNo,
			ContractStatus: contractStatus, FlowStatus: flowStatus, DecisionCode: decision}
		return nil
	})
	return result, err
}

func authoritativeDecision(
	locked lockedFlow,
	observation Observation,
	projected projection,
) string {
	decision := decide(locked, observation, projected)
	if observation.GatewayDecisionCode == "conflict" ||
		(observation.GatewayDecisionCode == "stale" && observation.DeliveryNo != "") {
		return observation.GatewayDecisionCode
	}
	return decision
}

func (s *serviceImpl) replayedResult(
	ctx context.Context,
	observation Observation,
) (ApplyResult, bool, error) {
	var existing []struct {
		ContractNo   string `orm:"contract_no"`
		FlowNo       string `orm:"flow_no"`
		DecisionCode string `orm:"decision_code"`
	}
	if err := s.events.Ctx(ctx).
		Fields("contract_no, flow_no, decision_code").
		Where(do.ContractStatusEvent{
			TenantId: idccap.DefaultTenantID, GatewayRouteNo: observation.RouteNo,
			GatewayRouteVersion: observation.RouteVersion,
		}).Limit(2).Scan(&existing); err != nil {
		return ApplyResult{}, false, gerror.Wrap(err, "read existing contract status event")
	}
	if len(existing) == 0 {
		return ApplyResult{}, false, nil
	}
	if len(existing) != 1 {
		return ApplyResult{}, false, gerror.New("contract status timeline is ambiguous")
	}
	if observation.DeliveryNo != "" {
		deliveryCount, err := s.events.Ctx(ctx).
			Where(do.ContractStatusEvent{
				TenantId: idccap.DefaultTenantID, GatewayRouteNo: observation.RouteNo,
				GatewayRouteVersion: observation.RouteVersion,
			}).Where("gateway_delivery_no IS NULL OR gateway_delivery_no = ?", observation.DeliveryNo).
			Count()
		if err != nil || deliveryCount != 1 {
			return ApplyResult{}, false, gerror.New("contract status delivery conflicts with existing timeline")
		}
	}
	return ApplyResult{
		ContractNo: existing[0].ContractNo, FlowNo: existing[0].FlowNo,
		DecisionCode: existing[0].DecisionCode, Replayed: true,
	}, true, nil
}
