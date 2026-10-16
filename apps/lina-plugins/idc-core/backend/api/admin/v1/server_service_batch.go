// Package v1 defines the administrator HTTP contract for bounded server operations.
package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

type ServerServiceBatchPreviewItemInput struct {
	ServiceNo  string          `json:"serviceNo" v:"required|max-length:40" dc:"Stable selected server service number" eg:"SVC_SERVER_0001"`
	Parameters json.RawMessage `json:"parameters" v:"required" dc:"Action-specific object; secrets and Provider identifiers are forbidden" eg:"{}"`
}

type ServerServiceListReq struct {
	Page              int `json:"page" in:"query" v:"between:1,1000000" dc:"One-based page number; cannot be combined with a cursor" eg:"1"`
	g.Meta            `path:"/admin/resource/server-services" method:"get" tags:"IDC Server Batches" summary:"List selectable server services" dc:"Return a bounded server directory for the management table and exact batch selection." permission:"idc:admin:resource:server-batch:view"`
	Keyword           string `json:"keyword" in:"query" v:"max-length:80" dc:"Optional service number or customer number search" eg:"SVC_SERVER_0001"`
	EntitlementStatus string `json:"entitlementStatus" in:"query" v:"max-length:32" dc:"Optional exact entitlement status" eg:"active"`
	RuntimeStatus     string `json:"runtimeStatus" in:"query" v:"max-length:32" dc:"Optional exact runtime status" eg:"running"`
	Cursor            string `json:"cursor" in:"query" v:"max-length:40" dc:"Previous page last service number" eg:"SVC_SERVER_0001"`
	Limit             int    `json:"limit" in:"query" v:"min:1|max:100" d:"50" dc:"Bounded page size" eg:"50"`
}

type ServerServiceListItem struct {
	ServiceNo            string `json:"serviceNo" dc:"Stable server service number" eg:"SVC_SERVER_0001"`
	CustomerNo           string `json:"customerNo" dc:"Owning customer number visible to the authorized staff member" eg:"CUS_0001"`
	ProductNameZhCN      string `json:"productNameZhCN" dc:"Chinese product display name" eg:"Standard cloud server"`
	ProductNameEnUS      string `json:"productNameEnUS" dc:"English product display name" eg:"Standard Cloud Server"`
	EntitlementStatus    string `json:"entitlementStatus" dc:"Current entitlement status" eg:"active"`
	RuntimeStatus        string `json:"runtimeStatus" dc:"Current runtime status" eg:"running"`
	ProviderStatus       string `json:"providerStatus" dc:"Current Provider task status without Provider identifiers" eg:"none"`
	EntitlementExpiresAt *int64 `json:"entitlementExpiresAt" dc:"Entitlement expiry in Unix milliseconds, or null" eg:"1787184000000"`
}

type ServerServiceListRes struct {
	Total      int                      `json:"total" dc:"Exact total within the current staff scope and filters" eg:"120"`
	Items      []*ServerServiceListItem `json:"items" dc:"Bounded selectable server services"`
	NextCursor string                   `json:"nextCursor" dc:"Next page cursor, or empty when no more results" eg:"SVC_SERVER_0001"`
}

type ServerServiceBatchPreviewFields struct {
	Action    string                                `json:"action" v:"required|max-length:32" dc:"Safe metadata, reminder, or dangerous lifecycle action" eg:"terminate"`
	Items     []*ServerServiceBatchPreviewItemInput `json:"items" v:"required" dc:"One to twenty unique exact server selections; collection bounds are enforced by the batch service"`
	RequestNo string                                `json:"requestNo" v:"required|min-length:16|max-length:128" dc:"Client idempotency key bound to the exact selection and parameters" eg:"server-batch-550e8400-e29b-41d4-a716-446655440000"`
}

type ServerServiceSafeBatchPreviewReq struct {
	g.Meta `path:"/admin/resource/server-batches/safe/previews" method:"post" tags:"IDC Server Batches" summary:"Preview a bounded safe server batch" dc:"Freeze each exact selected server before a tag, note, or renewal reminder effect." permission:"idc:admin:resource:server-batch:safe"`
	ServerServiceBatchPreviewFields
}

type ServerServiceDangerousBatchPreviewReq struct {
	g.Meta `path:"/admin/resource/server-batches/dangerous/previews" method:"post" tags:"IDC Server Batches" summary:"Preview a bounded dangerous server batch" dc:"Freeze customer, funds, expiry, local resource summary, current states, and irreversible impact before any dangerous effect." permission:"idc:admin:resource:server-batch:dangerous"`
	ServerServiceBatchPreviewFields
}

type ServerServiceBatchImpactItem struct {
	LineNumber           int             `json:"lineNumber" dc:"Stable preview line" eg:"1"`
	ServiceNo            string          `json:"serviceNo" dc:"Stable server service number" eg:"SVC_SERVER_0001"`
	CustomerNo           string          `json:"customerNo" dc:"Owning customer number visible to the authorized staff member" eg:"CUS_0001"`
	ServiceVersion       int64           `json:"serviceVersion" dc:"Frozen service version" eg:"3"`
	CurrencyCode         string          `json:"currencyCode" dc:"Frozen service currency" eg:"CNY"`
	EntitlementStatus    string          `json:"entitlementStatus" dc:"Frozen entitlement status" eg:"active"`
	RuntimeStatus        string          `json:"runtimeStatus" dc:"Frozen runtime status" eg:"running"`
	ProviderStatus       string          `json:"providerStatus" dc:"Frozen Provider task status" eg:"none"`
	EntitlementExpiresAt *int64          `json:"entitlementExpiresAt" dc:"Frozen expiry in Unix milliseconds, or null" eg:"1787184000000"`
	UnpaidAmountMinor    int64           `json:"unpaidAmountMinor" dc:"Current service-related unpaid amount in the service currency" eg:"0"`
	ActionAmountMinor    int64           `json:"actionAmountMinor" dc:"Frozen action charge in the service currency" eg:"0"`
	ResourceSummary      string          `json:"resourceSummary" dc:"Safe local resource type and pool summary without Provider identifiers" eg:"cloud_server / primary"`
	Irreversible         bool            `json:"irreversible" dc:"Whether this action can erase data or permanently remove the service" eg:"true"`
	Parameters           json.RawMessage `json:"parameters" dc:"Normalized exact per-service parameters" eg:"{}"`
	PreviewStatus        string          `json:"previewStatus" dc:"Per-service preview eligibility" eg:"eligible"`
	FailureCode          string          `json:"failureCode" dc:"Stable ineligibility or execution reason" eg:""`
	ExecutionStatus      string          `json:"executionStatus" dc:"Independent per-service execution result" eg:"pending"`
	OperationNo          string          `json:"operationNo" dc:"Independent operation or task number" eg:"sop_0123456789abcdef"`
	NotificationEventNo  string          `json:"notificationEventNo" dc:"Independent reminder event number" eg:"NEV_0123456789abcdef"`
	ExecutedAt           *int64          `json:"executedAt" dc:"Execution decision time in Unix milliseconds, or null" eg:"1787184000000"`
}

type ServerServiceBatch struct {
	BatchNo       string                          `json:"batchNo" dc:"Stable batch number" eg:"sbt_0123456789abcdef"`
	Action        string                          `json:"action" dc:"Frozen action" eg:"terminate"`
	Dangerous     bool                            `json:"dangerous" dc:"Whether current active session is mandatory" eg:"true"`
	Status        string                          `json:"status" dc:"Batch coordination status" eg:"previewed"`
	PreviewDigest string                          `json:"previewDigest" dc:"Exact immutable impact digest" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	Items         []*ServerServiceBatchImpactItem `json:"items" dc:"Bounded per-service impact and independent result list"`
	CreatedAt     int64                           `json:"createdAt" dc:"Preview creation time in Unix milliseconds" eg:"1787184000000"`
}

type ServerServiceBatchPreviewRes struct {
	Batch *ServerServiceBatch `json:"batch"`
}
type ServerServiceSafeBatchPreviewRes = ServerServiceBatchPreviewRes
type ServerServiceDangerousBatchPreviewRes = ServerServiceBatchPreviewRes

type ServerServiceBatchDetailReq struct {
	g.Meta  `path:"/admin/resource/server-batches/{batchNo}" method:"get" tags:"IDC Server Batches" summary:"Get one server batch" dc:"Return an operator-owned exact batch and its independent per-service results." permission:"idc:admin:resource:server-batch:view"`
	BatchNo string `json:"batchNo" in:"path" v:"required|max-length:40" dc:"Stable batch number" eg:"sbt_0123456789abcdef"`
}
type ServerServiceBatchDetailRes struct {
	Batch *ServerServiceBatch `json:"batch"`
}

type ServerServiceBatchConfirmationReq struct {
	g.Meta        `path:"/admin/resource/server-batches/{batchNo}/confirmation" method:"post" tags:"IDC Server Batches" summary:"Confirm a current session for a dangerous batch" dc:"Issue one short-lived, single-use proof bound to the exact dangerous batch preview after checking the current host session." permission:"idc:admin:resource:server-batch:dangerous"`
	BatchNo       string `json:"batchNo" in:"path" v:"required|max-length:40" dc:"Stable dangerous batch number" eg:"sbt_0123456789abcdef"`
	PreviewDigest string `json:"previewDigest" v:"required|length:64" dc:"Exact immutable impact digest shown to the operator" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
}
type ServerServiceBatchConfirmationRes struct {
	ProofToken string `json:"proofToken" dc:"Opaque single-use current-session proof"`
	ExpiresAt  int64  `json:"expiresAt" dc:"Proof expiry in Unix milliseconds" eg:"1787184300000"`
}

type ServerServiceBatchExecuteFields struct {
	BatchNo       string `json:"batchNo" in:"path" v:"required|max-length:40" dc:"Stable batch number" eg:"sbt_0123456789abcdef"`
	PreviewDigest string `json:"previewDigest" v:"required|length:64" dc:"Exact immutable impact digest" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	ProofToken    string `json:"proofToken" dc:"Required only for dangerous actions and never persisted"`
}

type ServerServiceSafeBatchExecuteReq struct {
	g.Meta `path:"/admin/resource/server-batches/{batchNo}/safe-executions" method:"post" tags:"IDC Server Batches" summary:"Execute a safe server batch" dc:"Recheck the exact safe preview and advance each selected server through its own idempotent result." permission:"idc:admin:resource:server-batch:safe"`
	ServerServiceBatchExecuteFields
}

type ServerServiceDangerousBatchExecuteReq struct {
	g.Meta `path:"/admin/resource/server-batches/{batchNo}/dangerous-executions" method:"post" tags:"IDC Server Batches" summary:"Execute a dangerous server batch" dc:"Consume the exact current-session proof and advance every server through its own idempotent operation." permission:"idc:admin:resource:server-batch:dangerous"`
	ServerServiceBatchExecuteFields
}
type ServerServiceBatchExecuteRes struct {
	Batch *ServerServiceBatch `json:"batch"`
}
type ServerServiceSafeBatchExecuteRes = ServerServiceBatchExecuteRes
type ServerServiceDangerousBatchExecuteRes = ServerServiceBatchExecuteRes

type ServerClassification struct {
	ClassificationNo string `json:"classificationNo" dc:"Stable server classification number" eg:"scl_0123456789abcdef"`
	NameZhCN         string `json:"nameZhCN" dc:"Chinese display name" eg:"Core business"`
	NameEnUS         string `json:"nameEnUS" dc:"English display name" eg:"Business critical"`
	Status           string `json:"status" dc:"active or disabled" eg:"active"`
	Version          int64  `json:"version" dc:"Optimistic lock version" eg:"1"`
}

type ServerClassificationListReq struct {
	g.Meta `path:"/admin/resource/server-classifications" method:"get" tags:"IDC Server Batches" summary:"List fixed server classifications" permission:"idc:admin:resource:server-batch:view"`
}
type ServerClassificationListRes struct {
	Items []*ServerClassification `json:"items"`
}

type ServerClassificationSaveFields struct {
	NameZhCN  string `json:"nameZhCN" v:"required|max-length:80" dc:"Chinese display name"`
	NameEnUS  string `json:"nameEnUS" v:"required|max-length:80" dc:"English display name"`
	RequestNo string `json:"requestNo" v:"max-length:128" dc:"Required idempotency key when creating"`
}

type ServerClassificationCreateReq struct {
	g.Meta `path:"/admin/resource/server-classifications" method:"post" tags:"IDC Server Batches" summary:"Create a fixed server classification" permission:"idc:admin:resource:server-batch:safe"`
	ServerClassificationSaveFields
}
type ServerClassificationSaveRes struct {
	Item *ServerClassification `json:"item"`
}
type ServerClassificationCreateRes = ServerClassificationSaveRes
type ServerClassificationUpdateRes = ServerClassificationSaveRes
type ServerClassificationDisableRes = ServerClassificationSaveRes

type ServerClassificationUpdateReq struct {
	g.Meta           `path:"/admin/resource/server-classifications/{classificationNo}" method:"put" tags:"IDC Server Batches" summary:"Update a fixed server classification" permission:"idc:admin:resource:server-batch:safe"`
	ClassificationNo string `json:"classificationNo" in:"path" v:"required|max-length:40"`
	ExpectedVersion  int64  `json:"expectedVersion" v:"required|min:1"`
	ServerClassificationSaveFields
}

type ServerClassificationDisableReq struct {
	g.Meta           `path:"/admin/resource/server-classifications/{classificationNo}" method:"delete" tags:"IDC Server Batches" summary:"Disable a fixed server classification" permission:"idc:admin:resource:server-batch:safe"`
	ClassificationNo string `json:"classificationNo" in:"path" v:"required|max-length:40"`
	ExpectedVersion  int64  `json:"expectedVersion" v:"required|min:1"`
}
