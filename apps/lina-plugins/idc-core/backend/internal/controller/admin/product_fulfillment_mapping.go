// This file maps authorized fulfillment facts to staff-only API projections.
package admin

import (
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

const (
	fulfillmentReasonReviewApprove     = "review_approve"
	fulfillmentReasonReviewReject      = "review_reject"
	fulfillmentReasonTakeover          = "takeover"
	fulfillmentReasonConcludeCompleted = "manual_conclude_completed"
	fulfillmentReasonConcludeUnable    = "manual_conclude_unable"
)

func toAPIProductFulfillmentSummary(input *productfulfillment.IntentDetail) *v1.ProductFulfillmentTaskSummary {
	if input == nil {
		return nil
	}
	return &v1.ProductFulfillmentTaskSummary{
		TargetEntitlementNo: input.TargetEntitlementNo, IntentNo: input.IntentNo, OrderNo: input.OrderNo, OrderItemNo: input.OrderItemNo,
		CustomerNo: input.CustomerNo, ProductNameEnUS: input.ProductNameEnUS,
		CustomerCode: input.CustomerCode, CustomerDisplayName: input.CustomerDisplayName,
		CustomerPrimaryPhone: input.CustomerPrimaryPhone,
		ProductNameZhCN:      input.ProductNameZhCN, FulfillmentModeCode: input.StrategyCode,
		GranularityCode: input.GranularityCode, StatusCode: input.StatusCode,
		Quantity: input.Quantity, UnitSequence: input.UnitSequence,
		UpdatedAt: input.UpdatedAt.UnixMilli(), Version: input.Version,
		CanReviewApprove: input.CanReviewApprove, CanReviewReject: input.CanReviewReject,
		CanTakeover: input.CanTakeover, CanManualConclude: input.CanManualConclude,
		CanRecoverPreparation: input.CanRecoverPreparation,
		CanRequestRefund:      input.CanRequestRefund,
	}
}

func toAPIProductFulfillmentDetail(input *productfulfillment.IntentDetail) *v1.ProductFulfillmentTaskDetail {
	if input == nil {
		return nil
	}
	events := make([]*v1.ProductFulfillmentTaskEvent, 0, len(input.Events))
	for _, event := range input.Events {
		events = append(events, &v1.ProductFulfillmentTaskEvent{
			EventCode: event.TypeCode, OccurredAt: event.OccurredAt.UnixMilli(),
			OperatorDisplayName: event.OperatorDisplayName, ReasonCode: event.ReasonCode,
			ReasonLabelEnUS: event.ReasonLabelEnUS, ReasonLabelZhCN: event.ReasonLabelZhCN,
			SafeNote: event.SafeNote,
		})
	}
	var preparation *v1.ProductFulfillmentManagementPreparation
	if input.ManagementPreparation != nil {
		preparation = &v1.ProductFulfillmentManagementPreparation{StatusCode: input.ManagementPreparation.StatusCode, AttemptCount: input.ManagementPreparation.AttemptCount, FailureCode: input.ManagementPreparation.FailureCode}
	}
	var execution *v1.ProductFulfillmentExecutionDiagnostic
	if input.Execution != nil {
		execution = &v1.ProductFulfillmentExecutionDiagnostic{
			ResponseExcerpt: input.Execution.ResponseExcerpt,
			ResourceTaskNo:  input.Execution.ResourceTaskNo, StatusCode: input.Execution.StatusCode,
			PhaseCode: input.Execution.PhaseCode, LastResultCode: input.Execution.LastResultCode,
			AttemptCount: input.Execution.AttemptCount, FailureCode: input.Execution.FailureCode,
		}
	}
	records := make([]v1.ProductFulfillmentRecoveryConfigurationRecord, 0, len(input.RecoveryConfigurations))
	for _, record := range input.RecoveryConfigurations {
		records = append(records, v1.ProductFulfillmentRecoveryConfigurationRecord{Configuration: record.Configuration, OccurredAt: record.OccurredAt.UnixMilli()})
	}
	verifications := make([]v1.ProductFulfillmentNonAcceptanceRecord, 0, len(input.NonAcceptanceVerifications))
	for _, record := range input.NonAcceptanceVerifications {
		verifications = append(verifications, v1.ProductFulfillmentNonAcceptanceRecord{ProviderTaskNo: record.ProviderTaskNo, ActorUserID: record.ActorUserID, Note: record.Note, VerifiedAt: record.VerifiedAt.UnixMilli()})
	}
	return &v1.ProductFulfillmentTaskDetail{
		CanRepairManagement: input.CanRepairManagement, CanVerifyNonAcceptance: input.CanVerifyNonAcceptance, NonAcceptanceVerifications: verifications,
		ProductFulfillmentTaskSummary: *toAPIProductFulfillmentSummary(input),
		Execution:                     execution, ManagementPreparation: preparation,
		RecoveryConfigurations: records,
		FinanciallySettled:     !input.FinanciallySettledAt.IsZero(),
		Events:                 events,
		ReviewApprovalReasons: mapProductFulfillmentReasonOptions(
			input.ReasonOptions[fulfillmentReasonReviewApprove],
		),
		ReviewRejectionReasons: mapProductFulfillmentReasonOptions(
			input.ReasonOptions[fulfillmentReasonReviewReject],
		),
		TakeoverReasons: mapProductFulfillmentReasonOptions(
			input.ReasonOptions[fulfillmentReasonTakeover],
		),
		ManualCompletedReasons: mapProductFulfillmentReasonOptions(
			input.ReasonOptions[fulfillmentReasonConcludeCompleted],
		),
		ManualUnableReasons: mapProductFulfillmentReasonOptions(
			input.ReasonOptions[fulfillmentReasonConcludeUnable],
		),
		RecoveryReasons: mapProductFulfillmentReasonOptions(input.ReasonOptions["recover_preparation"]),
	}
}

func mapProductFulfillmentReasonOptions(input []productfulfillment.ReasonOption) []*v1.ProductFulfillmentReasonOption {
	result := make([]*v1.ProductFulfillmentReasonOption, 0, len(input))
	for _, option := range input {
		result = append(result, &v1.ProductFulfillmentReasonOption{
			Code: option.Code, LabelEnUS: option.LabelEnUS, LabelZhCN: option.LabelZhCN,
		})
	}
	return result
}
