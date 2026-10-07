package v1

type ProductFulfillmentReasonOption struct {
	Code      string `json:"code" dc:"Stable reason code accepted for this exact action"`
	LabelEnUS string `json:"labelEnUS" dc:"Safe English reason label"`
	LabelZhCN string `json:"labelZhCN" dc:"Safe Simplified Chinese reason label"`
}

type ProductFulfillmentTaskSummary struct {
	IntentNo             string `json:"intentNo" dc:"Stable merchant-side fulfillment intent number"`
	OrderNo              string `json:"orderNo" dc:"Stable order number"`
	OrderItemNo          string `json:"orderItemNo" dc:"Frozen order item number"`
	CustomerNo           string `json:"customerNo" dc:"Owning customer number visible under the current data scope"`
	CustomerCode         string `json:"customerCode" dc:"Short numeric customer number"`
	CustomerDisplayName  string `json:"customerDisplayName" dc:"Display-safe customer name"`
	CustomerPrimaryPhone string `json:"customerPrimaryPhone" dc:"Primary customer phone"`
	ProductNameEnUS      string `json:"productNameEnUS" dc:"Frozen English product name"`
	ProductNameZhCN      string `json:"productNameZhCN" dc:"Frozen Simplified Chinese product name"`
	FulfillmentModeCode  string `json:"fulfillmentModeCode" dc:"Frozen generic fulfillment mode"`
	GranularityCode      string `json:"granularityCode" dc:"Frozen generic fulfillment granularity"`
	StatusCode           string `json:"statusCode" dc:"Safe fulfillment workflow projection"`
	Quantity             int    `json:"quantity" dc:"Frozen generic order item quantity"`
	UnitSequence         int    `json:"unitSequence" dc:"One-based unit sequence within the frozen granularity"`
	UpdatedAt            int64  `json:"updatedAt" dc:"Last authoritative update time as Unix milliseconds"`
	Version              int64  `json:"version" dc:"Optimistic workflow version"`
	CanReviewApprove     bool   `json:"canReviewApprove" dc:"Whether current facts allow an authorized approval-and-start action"`
	CanReviewReject      bool   `json:"canReviewReject" dc:"Whether current facts allow an authorized rejection-to-manual action"`
	CanTakeover          bool   `json:"canTakeover" dc:"Whether current facts allow an authorized automation takeover"`
	CanManualConclude    bool   `json:"canManualConclude" dc:"Whether current facts allow an authorized manual outcome"`
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

type ProductFulfillmentTaskDetail struct {
	ProductFulfillmentTaskSummary
	FinanciallySettled     bool                              `json:"financiallySettled" dc:"Whether the funding owner has committed authoritative settlement"`
	Events                 []*ProductFulfillmentTaskEvent    `json:"events" dc:"Bounded oldest-to-newest safe workflow history"`
	ReviewApprovalReasons  []*ProductFulfillmentReasonOption `json:"reviewApprovalReasons" dc:"Current stable reasons accepted by approval and start"`
	ReviewRejectionReasons []*ProductFulfillmentReasonOption `json:"reviewRejectionReasons" dc:"Current stable reasons accepted by rejection to manual handling"`
	TakeoverReasons        []*ProductFulfillmentReasonOption `json:"takeoverReasons" dc:"Current stable reasons accepted by takeover"`
	ManualCompletedReasons []*ProductFulfillmentReasonOption `json:"manualCompletedReasons" dc:"Current stable reasons accepted when manual fulfillment completed"`
	ManualUnableReasons    []*ProductFulfillmentReasonOption `json:"manualUnableReasons" dc:"Current stable reasons accepted when manual fulfillment could not be completed"`
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
	Note            string `json:"note" v:"required|length:10,512" dc:"Required bounded manual outcome note"`
}
