// This file defines the isolated upstream ticket HTTP contract.
package v1

import "github.com/gogf/gf/v2/frame/g"

type SupportUpstreamChannel struct {
	BaseURL              string `json:"baseUrl" dc:"Configured HTTPS origin" eg:"https://support.example.com"`
	AuthMode             string `json:"authMode" dc:"Authentication mode: zjmf_api_login or resource_login" eg:"zjmf_api_login"`
	Status               string `json:"status" dc:"Channel status: enabled or disabled" eg:"enabled"`
	CredentialMaskedTail string `json:"credentialMaskedTail" dc:"Safe credential tail only" eg:"***admin"`
	Configured           bool   `json:"configured" dc:"Whether the channel has saved credentials" eg:"true"`
	Version              int64  `json:"version" dc:"Optimistic concurrency version" eg:"2"`
	UpdatedAt            int64  `json:"updatedAt" dc:"Unix timestamp in milliseconds" eg:"1787558400000"`
}

type SupportUpstreamChannelGetReq struct {
	g.Meta `path:"/admin/support/settings/upstream-channel" method:"get" tags:"IDC Support" summary:"Get upstream ticket channel" dc:"Returns a safe projection without any credential plaintext." permission:"idc:admin:support:configuration:manage"`
}
type SupportUpstreamChannelGetRes struct {
	Item *SupportUpstreamChannel `json:"item" dc:"Safe channel projection" eg:"legacy-finance-ticket"`
}

type SupportUpstreamChannelPutReq struct {
	g.Meta          `path:"/admin/support/settings/upstream-channel" method:"put" tags:"IDC Support" summary:"Configure upstream ticket channel" dc:"Replaces the isolated channel credential after current-session verification." permission:"idc:admin:support:configuration:manage"`
	OperationNo     string `json:"operationNo" dc:"Unique idempotent operation number" eg:"OP_upstream_001"`
	BaseURL         string `json:"baseUrl" dc:"One public HTTPS origin" eg:"https://support.example.com"`
	AuthMode        string `json:"authMode" dc:"Authentication mode: zjmf_api_login or resource_login" eg:"zjmf_api_login"`
	Username        string `json:"username" dc:"Upstream account name; never returned" eg:"support-agent"`
	Password        string `json:"password" dc:"Upstream password; never returned" eg:"change-me"`
	Status          string `json:"status" dc:"Channel status: enabled or disabled" eg:"enabled"`
	Reason          string `json:"reason" dc:"Required configuration change reason" eg:"Rotate the upstream support account"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Current version, or zero for first configuration" eg:"1"`
}
type SupportUpstreamChannelPutRes = SupportUpstreamChannelGetRes

type SupportUpstreamDepartmentMapPutReq struct {
	g.Meta                 `path:"/admin/support/settings/upstream-departments/{departmentNo}" method:"put" tags:"IDC Support" summary:"Map support department upstream" dc:"Creates or replaces one local-to-upstream department mapping." permission:"idc:admin:support:configuration:manage"`
	DepartmentNo           string `json:"departmentNo" dc:"Local support department number" eg:"DEP_support"`
	UpstreamDepartmentID   int64  `json:"upstreamDepartmentId" dc:"Upstream ticket department identifier" eg:"12"`
	UpstreamDepartmentName string `json:"upstreamDepartmentName" dc:"Operator-facing upstream department label" eg:"Technical support"`
	Status                 string `json:"status" dc:"Mapping status: active or disabled" eg:"active"`
	ExpectedVersion        int64  `json:"expectedVersion" dc:"Current version, or zero for first mapping" eg:"1"`
}
type SupportUpstreamDepartmentMap struct {
	DepartmentNo           string `json:"departmentNo" dc:"Local support department number" eg:"DEP_support"`
	UpstreamDepartmentID   int64  `json:"upstreamDepartmentId" dc:"Upstream ticket department identifier" eg:"12"`
	UpstreamDepartmentName string `json:"upstreamDepartmentName" dc:"Operator-facing upstream department label" eg:"Technical support"`
	Status                 string `json:"status" dc:"Mapping status: active or disabled" eg:"active"`
	Version                int64  `json:"version" dc:"Optimistic concurrency version" eg:"2"`
}
type SupportUpstreamDepartmentMapGetReq struct {
	g.Meta       `path:"/admin/support/settings/upstream-departments/{departmentNo}" method:"get" tags:"IDC Support" summary:"Get upstream department mapping" dc:"Returns the current mapping and version, or an empty item before first configuration." permission:"idc:admin:support:configuration:manage"`
	DepartmentNo string `json:"departmentNo" dc:"Local support department number" eg:"DEP_support"`
}
type SupportUpstreamDepartmentMapGetRes struct {
	Item *SupportUpstreamDepartmentMap `json:"item" dc:"Current mapping, or null when it has not been configured" eg:"DEP_support"`
}
type SupportUpstreamDepartmentMapPutRes = SupportUpstreamDepartmentMapGetRes

type SupportUpstreamEntitlementMapPutReq struct {
	g.Meta            `path:"/admin/support/settings/upstream-entitlements/{entitlementNo}" method:"put" tags:"IDC Support" summary:"Map customer entitlement upstream" dc:"Creates or replaces the local long-term entitlement to opaque upstream reference mapping." permission:"idc:admin:support:configuration:manage"`
	EntitlementNo     string `json:"entitlementNo" dc:"Local long-term entitlement number" eg:"ent_0123456789abcdef0123456789abcdef"`
	UpstreamReference string `json:"upstreamReference" dc:"Opaque upstream entitlement reference" eg:"upstream-ref-8086"`
	Status            string `json:"status" dc:"Mapping status: active or disabled" eg:"active"`
	ExpectedVersion   int64  `json:"expectedVersion" dc:"Current version, or zero for first mapping" eg:"1"`
}
type SupportUpstreamEntitlementMap struct {
	EntitlementNo     string `json:"entitlementNo" dc:"Local long-term entitlement number" eg:"ent_0123456789abcdef0123456789abcdef"`
	UpstreamReference string `json:"upstreamReference" dc:"Opaque upstream entitlement reference" eg:"upstream-ref-8086"`
	Status            string `json:"status" dc:"Mapping status: active or disabled" eg:"active"`
	Version           int64  `json:"version" dc:"Optimistic concurrency version" eg:"2"`
}
type SupportUpstreamEntitlementMapGetReq struct {
	g.Meta        `path:"/admin/support/settings/upstream-entitlements/{entitlementNo}" method:"get" tags:"IDC Support" summary:"Get upstream entitlement mapping" dc:"Returns the current opaque upstream reference mapping and version, or an empty item before first configuration." permission:"idc:admin:support:configuration:manage"`
	EntitlementNo string `json:"entitlementNo" dc:"Local long-term entitlement number" eg:"ent_0123456789abcdef0123456789abcdef"`
}
type SupportUpstreamEntitlementMapGetRes struct {
	Item *SupportUpstreamEntitlementMap `json:"item" dc:"Current mapping, or null when it has not been configured" eg:"ent_0123456789abcdef0123456789abcdef"`
}
type SupportUpstreamEntitlementMapPutRes = SupportUpstreamEntitlementMapGetRes

type SupportUpstreamPrepareReq struct {
	g.Meta        `path:"/admin/support/tickets/{ticketNo}/upstream-previews" method:"post" tags:"IDC Support" summary:"Prepare upstream ticket preview" dc:"Freezes only selected customer-visible messages and attachments for final confirmation." permission:"idc:admin:support:ticket:upstream-forward"`
	TicketNo      string   `json:"ticketNo" dc:"Local ticket number" eg:"TKT_001"`
	TicketVersion int64    `json:"ticketVersion" dc:"Current local ticket version" eg:"3"`
	OperationNo   string   `json:"operationNo" dc:"Unique preview operation number" eg:"OP_preview_001"`
	MessageNos    []string `json:"messageNos" dc:"Selected customer-visible message numbers, maximum 100 combined items" eg:"MSG_001"`
	AttachmentNos []string `json:"attachmentNos" dc:"Selected customer-visible attachment numbers, maximum 100 combined items" eg:"ATT_001"`
}
type SupportUpstreamPreview struct {
	ForwardNo              string   `json:"forwardNo" dc:"Frozen forwarding record number" eg:"UPF_001"`
	TicketNo               string   `json:"ticketNo" dc:"Local ticket number" eg:"TKT_001"`
	Title                  string   `json:"title" dc:"Final upstream title" eg:"TKT_001 Server unavailable"`
	Content                string   `json:"content" dc:"Final upstream content preview" eg:"Customer-visible reply"`
	SelectionDigest        string   `json:"selectionDigest" dc:"Digest of selected immutable content" eg:"aabbcc"`
	PreviewDigest          string   `json:"previewDigest" dc:"Digest required for confirmation" eg:"ddeeff"`
	MessageNos             []string `json:"messageNos" dc:"Frozen selected message numbers" eg:"MSG_001"`
	AttachmentNos          []string `json:"attachmentNos" dc:"Frozen selected attachment numbers" eg:"ATT_001"`
	DepartmentID           int64    `json:"departmentId" dc:"Mapped upstream department identifier" eg:"12"`
	EntitlementUpstreamRef string   `json:"entitlementUpstreamRef" dc:"Mapped opaque upstream reference when the customer ticket has a related entitlement" eg:"upstream-ref-8086"`
	Version                int64    `json:"version" dc:"Forwarding optimistic version" eg:"1"`
}
type SupportUpstreamPrepareRes struct {
	Item *SupportUpstreamPreview `json:"item" dc:"Frozen final preview" eg:"UPF_001"`
}

type SupportUpstreamConfirmReq struct {
	g.Meta          `path:"/admin/support/upstream-forwards/{forwardNo}/confirmations" method:"post" tags:"IDC Support" summary:"Confirm upstream ticket send" dc:"Performs one non-replayed upstream write after matching the frozen preview digest." permission:"idc:admin:support:ticket:upstream-forward"`
	ForwardNo       string `json:"forwardNo" dc:"Frozen forwarding record number" eg:"UPF_001"`
	PreviewDigest   string `json:"previewDigest" dc:"Digest displayed in the final preview" eg:"ddeeff"`
	OperationNo     string `json:"operationNo" dc:"Unique confirmation operation number" eg:"OP_send_001"`
	Reason          string `json:"reason" dc:"Required send reason" eg:"Escalate confirmed customer evidence"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Current forwarding version" eg:"1"`
}
type SupportUpstreamForward struct {
	ForwardNo               string `json:"forwardNo" dc:"Forwarding record number" eg:"UPF_001"`
	TicketNo                string `json:"ticketNo" dc:"Local ticket number" eg:"TKT_001"`
	ProviderTicketReference string `json:"providerTicketReference" dc:"Upstream ticket number when accepted" eg:"9527"`
	Status                  string `json:"status" dc:"Current forwarding state" eg:"accepted"`
	ResultClassification    string `json:"resultClassification" dc:"accepted, definitive_not_received, or result_unknown" eg:"accepted"`
	EvidenceCode            string `json:"evidenceCode" dc:"Safe result evidence classification" eg:"ticket_created"`
	PreviewDigest           string `json:"previewDigest" dc:"Frozen preview digest required for an explicitly eligible manual retry" eg:"ddeeff"`
	RetryEligible           bool   `json:"retryEligible" dc:"Whether manual retry is currently safe" eg:"false"`
	AttemptCount            int    `json:"attemptCount" dc:"Number of network write attempts" eg:"1"`
	Version                 int64  `json:"version" dc:"Optimistic concurrency version" eg:"2"`
	UpdatedAt               int64  `json:"updatedAt" dc:"Unix timestamp in milliseconds" eg:"1787558400000"`
}
type SupportUpstreamConfirmRes struct {
	Item *SupportUpstreamForward `json:"item" dc:"Authoritative forwarding result" eg:"UPF_001"`
}

type SupportUpstreamRetryReq struct {
	g.Meta          `path:"/admin/support/upstream-forwards/{forwardNo}/retries" method:"post" tags:"IDC Support" summary:"Retry upstream ticket send" dc:"Allows one manual retry only after authoritative evidence proves the previous write was not received." permission:"idc:admin:support:ticket:upstream-forward"`
	ForwardNo       string `json:"forwardNo" dc:"Forwarding record number" eg:"UPF_001"`
	PreviewDigest   string `json:"previewDigest" dc:"Original frozen preview digest" eg:"ddeeff"`
	OperationNo     string `json:"operationNo" dc:"Unique retry operation number" eg:"OP_retry_001"`
	Reason          string `json:"reason" dc:"Required manual retry reason" eg:"Upstream confirmed no ticket was created"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Current forwarding version" eg:"2"`
}
type SupportUpstreamRetryRes = SupportUpstreamConfirmRes

type SupportUpstreamForwardGetReq struct {
	g.Meta    `path:"/admin/support/upstream-forwards/{forwardNo}" method:"get" tags:"IDC Support" summary:"Get upstream forwarding state" dc:"Returns the authoritative local forwarding and retry state." permission:"idc:admin:support:ticket:upstream-forward"`
	ForwardNo string `json:"forwardNo" dc:"Forwarding record number" eg:"UPF_001"`
}
type SupportUpstreamForwardGetRes = SupportUpstreamConfirmRes

type SupportTicketUpstreamForwardListReq struct {
	g.Meta   `path:"/admin/support/tickets/{ticketNo}/upstream-forwards" method:"get" tags:"IDC Support" summary:"List ticket upstream forwards" dc:"Returns the latest twenty forwarding attempts after rechecking ticket visibility." permission:"idc:admin:support:ticket:upstream-forward"`
	TicketNo string `json:"ticketNo" dc:"Local ticket number" eg:"TKT_001"`
}
type SupportTicketUpstreamForwardListRes struct {
	Items []*SupportUpstreamForward `json:"items" dc:"Latest forwarding attempts, newest first" eg:"UPF_001"`
}

type SupportUpstreamSyncReq struct {
	g.Meta          `path:"/admin/support/upstream-forwards/{forwardNo}/reply-syncs" method:"post" tags:"IDC Support" summary:"Sync upstream replies" dc:"Uses a read-only upstream query and stores new staff replies as customer-invisible internal content." permission:"idc:admin:support:ticket:upstream-forward"`
	ForwardNo       string `json:"forwardNo" dc:"Forwarding record number" eg:"UPF_001"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Current forwarding version" eg:"2"`
}
type SupportUpstreamSyncRes struct {
	Added int `json:"added" dc:"Number of newly synchronized internal replies" eg:"2"`
}

type SupportUpstreamPublishReq struct {
	g.Meta         `path:"/admin/support/tickets/{ticketNo}/upstream-replies/{localMessageNo}/publications" method:"post" tags:"IDC Support" summary:"Publish reviewed upstream reply" dc:"Copies one reviewed internal upstream reply into a new immutable customer-visible staff reply." permission:"idc:admin:support:ticket:upstream-forward"`
	TicketNo       string `json:"ticketNo" dc:"Local ticket number" eg:"TKT_001"`
	LocalMessageNo string `json:"localMessageNo" dc:"Customer-invisible synchronized message number" eg:"MSG_upstream_001"`
	Reason         string `json:"reason" dc:"Required publication reason" eg:"Reviewed and safe for customer"`
	TicketVersion  int64  `json:"ticketVersion" dc:"Current local ticket version" eg:"4"`
}
type SupportUpstreamPublishRes struct {
	Item *TicketDetail `json:"item" dc:"Updated authorized ticket detail" eg:"TKT_001"`
}
