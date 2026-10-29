// This file defines bounded staff fulfillment projections and action inputs.
package v1

import "lina-plugin-idc-core/backend/cap/provisionresourcecap"

type ProductFulfillmentReasonOption struct {
	Code      string `json:"code" dc:"Stable reason code accepted for this exact action"`
	LabelEnUS string `json:"labelEnUS" dc:"Safe English reason label"`
	LabelZhCN string `json:"labelZhCN" dc:"Safe Simplified Chinese reason label"`
}

type ProductFulfillmentTaskSummary struct {
	TargetEntitlementNo   string `json:"targetEntitlementNo,omitempty" dc:"Owned target service number for an add-on; empty for an ordinary purchase" eg:"ent_service"`
	IntentNo              string `json:"intentNo" dc:"Stable merchant-side fulfillment intent number"`
	OrderNo               string `json:"orderNo" dc:"Stable order number"`
	OrderItemNo           string `json:"orderItemNo" dc:"Frozen order item number"`
	CustomerNo            string `json:"customerNo" dc:"Owning customer number visible under the current data scope"`
	CustomerCode          string `json:"customerCode" dc:"Short numeric customer number"`
	CustomerDisplayName   string `json:"customerDisplayName" dc:"Display-safe customer name"`
	CustomerPrimaryPhone  string `json:"customerPrimaryPhone" dc:"Primary customer phone"`
	ProductNameEnUS       string `json:"productNameEnUS" dc:"Frozen English product name"`
	ProductNameZhCN       string `json:"productNameZhCN" dc:"Frozen Simplified Chinese product name"`
	FulfillmentModeCode   string `json:"fulfillmentModeCode" dc:"Frozen generic fulfillment mode"`
	GranularityCode       string `json:"granularityCode" dc:"Frozen generic fulfillment granularity"`
	StatusCode            string `json:"statusCode" dc:"Safe fulfillment workflow projection"`
	Quantity              int    `json:"quantity" dc:"Frozen generic order item quantity"`
	UnitSequence          int    `json:"unitSequence" dc:"One-based unit sequence within the frozen granularity"`
	UpdatedAt             int64  `json:"updatedAt" dc:"Last authoritative update time as Unix milliseconds"`
	Version               int64  `json:"version" dc:"Optimistic workflow version"`
	CanReviewApprove      bool   `json:"canReviewApprove" dc:"Whether current facts allow an authorized approval-and-start action"`
	CanReviewReject       bool   `json:"canReviewReject" dc:"Whether current facts allow an authorized rejection-to-manual action"`
	CanTakeover           bool   `json:"canTakeover" dc:"Whether current facts allow an authorized automation takeover"`
	CanManualConclude     bool   `json:"canManualConclude" dc:"Whether current facts allow an authorized manual outcome"`
	CanRequestRefund      bool   `json:"canRequestRefund" dc:"Whether a failed resource delivery can enter staff refund checks" eg:"false"`
	CanRecoverPreparation bool   `json:"canRecoverPreparation" dc:"Whether current facts allow query-first recovery of failed resource preparation" eg:"false"`
}

type ProductFulfillmentTaskEvent struct {
	EventCode           string `json:"eventCode" dc:"Bounded safe event classification"`
	OccurredAt          int64  `json:"occurredAt" dc:"Event time as Unix milliseconds"`
	OperatorDisplayName string `json:"operatorDisplayName,omitempty" dc:"Safe staff display name when applicable"`
	ReasonCode          string `json:"reasonCode,omitempty" dc:"Stable reason code when applicable"`
	ReasonLabelEnUS     string `json:"reasonLabelEnUS,omitempty" dc:"Frozen safe English reason label from the committed event"`
	ReasonLabelZhCN     string `json:"reasonLabelZhCN,omitempty" dc:"Frozen safe Simplified Chinese reason label from the committed event"`
	SafeNote            string `json:"safeNote,omitempty" dc:"Bounded staff-facing note with secrets and supplier payloads excluded"`
}

// ProductFulfillmentManagementPreparation contains local progress only.
type ProductFulfillmentManagementPreparation struct {
	StatusCode   string `json:"statusCode" dc:"Local management readiness: preparing, ready, or needs_attention; independent from delivery, lifecycle and billing" eg:"preparing"`
	AttemptCount int    `json:"attemptCount" dc:"Recorded local management notification attempts; never supplier creation attempts" eg:"2"`
	FailureCode  string `json:"failureCode,omitempty" dc:"Reviewed local category: entitlement_owner_unavailable, owner_delivery_unavailable, or owner_delivery_result_unknown; absent when no reviewed category exists; never raw errors or supplier messages" eg:"owner_delivery_unavailable"`
}

type ProductFulfillmentTaskDetail struct {
	CanRepairManagement        bool                                            `json:"canRepairManagement" dc:"Whether current local facts permit a separately authorized resource management link repair; never permits supplier provisioning or refund" eg:"false"`
	ManagementPreparation      *ProductFulfillmentManagementPreparation        `json:"managementPreparation,omitempty" dc:"Local service management handoff progress; absent without a matching notification; never triggers supplier calls or authorizes creation retries" eg:"{}"`
	CanVerifyNonAcceptance     bool                                            `json:"canVerifyNonAcceptance" dc:"Whether an authorized administrator can record verified non-acceptance for the current unknown task" eg:"false"`
	NonAcceptanceVerifications []ProductFulfillmentNonAcceptanceRecord         `json:"nonAcceptanceVerifications,omitempty" dc:"Latest 16 immutable staff verification facts; original unknown attempts remain unchanged" eg:"[]"`
	RecoveryConfigurations     []ProductFulfillmentRecoveryConfigurationRecord `json:"recoveryConfigurations,omitempty" dc:"Latest 16 immutable operational correction records; paid specifications remain unchanged" eg:"[]"`
	ProductFulfillmentTaskSummary
	Execution              *ProductFulfillmentExecutionDiagnostic `json:"execution,omitempty" dc:"Local resource-owned automation facts; absent for non-resource deliveries or before task creation; never triggers a supplier call" eg:"{}"`
	FinanciallySettled     bool                                   `json:"financiallySettled" dc:"Whether the funding owner has committed authoritative settlement"`
	Events                 []*ProductFulfillmentTaskEvent         `json:"events" dc:"Creation event and latest 64 safe workflow events ordered oldest to newest"`
	ReviewApprovalReasons  []*ProductFulfillmentReasonOption      `json:"reviewApprovalReasons" dc:"Current stable reasons accepted by approval and start"`
	ReviewRejectionReasons []*ProductFulfillmentReasonOption      `json:"reviewRejectionReasons" dc:"Current stable reasons accepted by rejection to manual handling"`
	TakeoverReasons        []*ProductFulfillmentReasonOption      `json:"takeoverReasons" dc:"Current stable reasons accepted by takeover"`
	ManualCompletedReasons []*ProductFulfillmentReasonOption      `json:"manualCompletedReasons" dc:"Current stable reasons accepted when manual fulfillment completed"`
	ManualUnableReasons    []*ProductFulfillmentReasonOption      `json:"manualUnableReasons" dc:"Current stable reasons accepted when manual fulfillment could not be completed"`
	RecoveryReasons        []*ProductFulfillmentReasonOption      `json:"recoveryReasons" dc:"Current reasons accepted for safe resource preparation recovery" eg:"[]"`
}

// ProductFulfillmentExecutionDiagnostic is a staff-only, secret-free local projection.
type ProductFulfillmentExecutionDiagnostic struct {
	ResponseExcerpt string `json:"responseExcerpt,omitempty" dc:"At most 4096 bytes of resource-owner sanitized call-stage or HTTP response diagnostics; credentials are removed; absent for historical attempts without captured diagnostics" eg:"{}"`
	FailureCode     string `json:"failureCode,omitempty" dc:"Reviewed failure category from the latest closed resource attempt; empty when no safe category was recorded; never a supplier message" eg:"idc_resource_provider_timeout"`
	ResourceTaskNo  string `json:"resourceTaskNo,omitempty" dc:"Opaque related local resource task number; empty before resource task creation; its detail requires resource task query permission" eg:"task_0123456789abcdef0123456789abcdef"`
	StatusCode      string `json:"statusCode" dc:"Local automation state: pending_query, querying, pending_submit, submitting, accepted, running, retry_wait, succeeded, failed, result_unknown, manual_takeover, cancellation_attention, or canceled; projected without reinterpretation" eg:"running"`
	PhaseCode       string `json:"phaseCode" dc:"Local automation phase: pre_submit_query, submit_allowed, post_submit_query, or frozen_unknown; projected without reinterpretation" eg:"pre_submit_query"`
	LastResultCode  string `json:"lastResultCode,omitempty" dc:"Last normalized local result code; empty before a result; never a supplier message or payload" eg:"provider_result_unavailable"`
	AttemptCount    int    `json:"attemptCount" dc:"Number of recorded local automation attempts" eg:"1"`
}

type ProductFulfillmentActionReq struct {
	IntentNo        string `json:"intentNo" in:"path" v:"required|max-length:40" dc:"Stable fulfillment intent number"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Expected optimistic workflow version"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Action key reused unchanged across uncertain retries"`
	ReasonCode      string `json:"reasonCode" v:"required|regex:^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$" dc:"Stable reason selected from the current detail response"`
	Note            string `json:"note" v:"max-length:512" dc:"Optional bounded merchant processing note"`
}

type ProductFulfillmentManualActionReq struct {
	IntentNo        string `json:"intentNo" in:"path" v:"required|max-length:40" dc:"Stable fulfillment intent number"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Expected optimistic workflow version"`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Action key reused unchanged across uncertain retries"`
	ReasonCode      string `json:"reasonCode" v:"required|regex:^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$" dc:"Stable reason selected from the current detail response"`
	Note            string `json:"note" v:"required|length:10,512" dc:"Required processing note of 10 to 512 characters"`
}

// ProductFulfillmentRecoveryConfigurationRecord is an immutable staff selection.
type ProductFulfillmentRecoveryConfigurationRecord struct {
	Configuration provisionresourcecap.RecoveryConfiguration `json:"configuration" dc:"Validated operational configuration for this recovery" eg:"{}"`
	OccurredAt    int64                                      `json:"occurredAt" dc:"Recovery selection creation Unix timestamp in milliseconds" eg:"1792484071316"`
}
