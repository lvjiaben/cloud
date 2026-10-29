// This file defines customer-owned native server management and one-time delivery.
package v1

import (
	"encoding/json"
	"github.com/gogf/gf/v2/frame/g"
)

type NativeServerDetailReq struct {
	g.Meta   `path:"/account/native-servers/{serverNo}" method:"get" tags:"IDC Account Servers" summary:"Get my native server management detail" dc:"Check current customer ownership before reading the exact upstream resource. Unknown observations never enable power operations."`
	ServerNo string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
}

type NativeServerResolveReq struct {
	g.Meta        `path:"/account/native-servers/resolve/{entitlementNo}" method:"get" tags:"IDC Account Servers" summary:"Resolve my delivered server" dc:"Resolve an entitlement already returned in the current customer's order history into its resource-owned public server number. Foreign and missing services are indistinguishable."`
	EntitlementNo string `json:"entitlementNo" in:"path" v:"required|regex:^ent_[0-9a-f]{32}$" dc:"Owned entitlement from order history" eg:"ent_0123456789abcdef0123456789abcdef"`
}

type NativeServerResolveRes struct {
	ServerNo string `json:"serverNo" dc:"Public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
}

type NativeServerOperation struct {
	FailureCode      string               `json:"failureCode" dc:"Safe machine-readable failure reason; empty when no failure is recorded" eg:"provider_rejected"`
	AvailableOutputs []NativeServerOutput `json:"availableOutputs" dc:"Unconsumed outputs from a succeeded operation; never secret handles" eg:"[]"`
	WaitingUntil     *int64               `json:"waitingUntil,omitempty" dc:"Active upstream channel protection deadline in Unix timestamp in milliseconds; only queued unsent operations expose it, otherwise absent" eg:"1792382700000"`
	OperationNo      string               `json:"operationNo" dc:"Public operation reference" eg:"nso_0123456789abcdef0123456789abcdef"`
	Action           string               `json:"action" dc:"Submitted native management action" eg:"start"`
	Status           string               `json:"status" dc:"Resource owner task status; unknown and manual takeover never mean success" eg:"pending"`
	CreatedAt        int64                `json:"createdAt" dc:"Creation time in Unix milliseconds" eg:"1792382400000"`
	UpdatedAt        int64                `json:"updatedAt" dc:"Latest task time in Unix milliseconds" eg:"1792382400000"`
}

type NativeServerDetailRes struct {
	CommercialCapabilities []NativeServerCommercialCapability `json:"commercialCapabilities" dc:"Commercial quote entry eligibility, separate from free executable actions; merchant price, stock and policy are checked when requesting a quote" eg:"[]"`
	Information            NativeServerInformation            `json:"information" dc:"Confirmed guest runtime and connection facts; absent numeric values mean unknown" eg:"{}"`
	Capabilities           []NativeServerCapability           `json:"capabilities" dc:"Advertised native actions and current unavailability reasons" eg:"[]"`
	EntitlementNo          string                             `json:"entitlementNo" dc:"Owned core service entitlement used for billing renewal" eg:"ent_0123456789abcdef0123456789abcdef"`
	Addresses              []string                           `json:"addresses" dc:"Validated allocated IP addresses; empty does not prove absence unless addressesConfirmed is true" eg:"[]"`
	AddressesConfirmed     bool                               `json:"addressesConfirmed" dc:"Current supplier response explicitly confirmed the address list" eg:"true"`
	ServerNo               string                             `json:"serverNo" dc:"Public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	ProductNameZhCN        string                             `json:"productNameZhCn" dc:"Frozen Chinese product name" eg:"Standard cloud server"`
	ProductNameEnUS        string                             `json:"productNameEnUs" dc:"Frozen English product name" eg:"Standard cloud server"`
	EntitlementStatus      string                             `json:"entitlementStatus" dc:"Core-owned entitlement status" eg:"active"`
	RuntimeState           string                             `json:"runtimeState" dc:"Current normalized upstream runtime state" eg:"running"`
	OrderNo                string                             `json:"orderNo" dc:"Owned source order number" eg:"ORD_0123456789abcdef0123456789abcdef"`
	PeriodUnit             string                             `json:"periodUnit" dc:"Frozen billing period unit" eg:"month"`
	PeriodCount            int                                `json:"periodCount" dc:"Frozen billing period count" eg:"1"`
	StartedAt              int64                              `json:"startedAt" dc:"Service start time in Unix milliseconds" eg:"1792382400000"`
	ExpiresAt              *int64                             `json:"expiresAt,omitempty" dc:"Service expiry time in Unix milliseconds, absent for no expiry" eg:"1795060800000"`
	ObservedAt             *int64                             `json:"observedAt,omitempty" dc:"Current successful upstream observation time in Unix milliseconds" eg:"1792382400000"`
	AvailableActions       []string                           `json:"availableActions" dc:"Currently executable native management actions" eg:"[\"stop\",\"reboot\"]"`
	Operations             []NativeServerOperation            `json:"operations" dc:"At most ten latest customer management operations" eg:"[]"`
	CredentialAvailable    bool                               `json:"credentialAvailable" dc:"Initial password has confirmed successful delivery evidence and has not been consumed" eg:"true"`
}

type NativeServerOperationCreateReq struct {
	g.Meta         `path:"/account/native-servers/{serverNo}/operations" method:"post" tags:"IDC Account Servers" summary:"Submit a native server management action" dc:"Recheck current ownership, entitlement, refund fence, state, capability and task exclusion. Stop and reboot require confirmation; writes run asynchronously through the resource owner's durable task."`
	ServerNo       string          `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	Action         string          `json:"action" v:"required|in:start,stop,reboot,reset_password,console,reinstall,enter_rescue,exit_rescue,security_group_attach,security_group_detach,nat_create,nat_delete,virtual_media_mount,virtual_media_unmount,snapshot_restore,snapshot_delete,backup_restore,backup_delete" dc:"Native free management action; paid changes require their separate commerce flow" eg:"reboot"`
	Parameters     json.RawMessage `json:"parameters" dc:"Whitelisted action inputs; omitted means an empty object and power mode defaults to graceful" eg:"{}"`
	Password       string          `json:"password" v:"max-length:1024" dc:"New password for reset_password or temporary password for enter_rescue, never returned or stored in ordinary task data" eg:""`
	ProofToken     string          `json:"proofToken" v:"max-length:4096" dc:"Single-use current-session confirmation for a high-risk action" eg:""`
	IdempotencyKey string          `json:"idempotencyKey" v:"required|length:1,128|regex:^[A-Za-z0-9][A-Za-z0-9_.:-]*$" dc:"Stable request key retained for retries of this same operation" eg:"native-power-request-1"`
	Confirmed      bool            `json:"confirmed" dc:"Explicit impact confirmation for a modifying operation" eg:"true"`
}

type NativeServerOperationCreateRes struct {
	Operation NativeServerOperation `json:"operation" dc:"Durable operation state, never immediate supplier success" eg:"{}"`
}

type NativeServerCredentialRevealReq struct {
	g.Meta    `path:"/account/native-servers/{serverNo}/initial-credential/reveal" method:"post" tags:"IDC Account Servers" summary:"Reveal my confirmed initial server password once" dc:"Current-customer-only no-store response. Requires explicit confirmation, an operable entitlement, succeeded resource task and matching core outcome receipt. The delivery is consumed before plaintext response and must never be automatically retried."`
	ServerNo  string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	Confirmed bool   `json:"confirmed" v:"required" dc:"Explicit one-time password delivery confirmation" eg:"true"`
}

// NativeServerCredentialRevealRes is written directly to the no-store response;
// plaintext never enters an ordinary controller result or logging projection.
type NativeServerCredentialRevealRes struct{}

// NativeServerListReq selects only resource-owned native entitlements.
type NativeServerListReq struct {
	g.Meta `path:"/account/native-servers" method:"get" tags:"IDC Account Servers" summary:"List my native servers" dc:"Return one bounded customer-owned native server page without querying the supplier per row. Runtime state is queried only on detail."`
	Page   int `json:"page" in:"query" v:"between:1,1000000" d:"1" dc:"One-based page number" eg:"1"`
	Limit  int `json:"limit" in:"query" v:"between:1,50" d:"20" dc:"Page size from one to fifty" eg:"20"`
}
type NativeServerListItem struct {
	ServerNo        string `json:"serverNo" dc:"Public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	ProductNameZhCN string `json:"productNameZhCn" dc:"Frozen Chinese product name" eg:"Standard cloud server"`
	ProductNameEnUS string `json:"productNameEnUs" dc:"Frozen English product name" eg:"Standard cloud server"`
	State           string `json:"state" dc:"Local resource mapping state, distinct from current runtime" eg:"active"`
}
type NativeServerListRes struct {
	Items []NativeServerListItem `json:"items" dc:"Bounded owned native server page" eg:"[]"`
	Total int                    `json:"total" dc:"Exact owned native server count under the same page filter" eg:"1"`
}

// NativeServerOutput advertises one explicitly consumable confidential result.
type NativeServerOutput struct {
	Name      string `json:"name" dc:"Output name: generatedCredential or consoleTicket" eg:"consoleTicket"`
	Policy    string `json:"policy" dc:"Delivery policy: one_time_credential or short_lived_ticket" eg:"short_lived_ticket"`
	ExpiresAt *int64 `json:"expiresAt,omitempty" dc:"Output expiry as Unix timestamp in milliseconds, absent for a nonexpiring credential" eg:"1792382700000"`
}

// NativeServerInformation excludes upstream accounts, secrets and commercial specifications.
type NativeServerInformation struct {
	Hostname      string `json:"hostname" dc:"Observed guest hostname; empty means unknown" eg:"cloud-1"`
	OSName        string `json:"osName" dc:"Observed operating system name; empty means unknown" eg:"Debian 12"`
	LoginUsername string `json:"loginUsername" dc:"Observed guest login account; never guessed from OS type" eg:"root"`
	Port          *int   `json:"port,omitempty" dc:"Observed guest connection port, absent when unknown" eg:"22"`
	CPUCores      *int   `json:"cpuCores,omitempty" dc:"Observed virtual CPU cores" eg:"2"`
	MemoryMiB     *int   `json:"memoryMiB,omitempty" dc:"Observed guest memory in MiB" eg:"2048"`
	NetworkType   string `json:"networkType" dc:"Observed network type; empty means unknown" eg:"public"`
	BootOrder     string `json:"bootOrder" dc:"Observed boot device order; empty means unknown" eg:"cd"`
	Rescue        *bool  `json:"rescue,omitempty" dc:"Observed rescue mode; absent when unknown" eg:"false"`
}

// NativeServerCapability distinguishes unsupported actions from temporary conflicts.
type NativeServerCapability struct {
	Action     string `json:"action" dc:"Native management action" eg:"reinstall"`
	Supported  bool   `json:"supported" dc:"Last complete capability snapshot advertises this action" eg:"true"`
	Available  bool   `json:"available" dc:"Current ownership, entitlement, task and runtime permit attempting this action; inputs are revalidated on submit" eg:"false"`
	ReasonCode string `json:"reasonCode" dc:"Unavailable reason: unsupported, capability_unavailable, entitlement_unavailable, operation_in_progress, runtime_unconfirmed, runtime_state_conflict or choices_unavailable; empty when available" eg:"operation_in_progress"`
}

// NativeServerCommercialCapability never authorizes direct resource execution.
type NativeServerCommercialCapability struct {
	Action     string `json:"action" dc:"Commercial action: snapshot_create, backup_create, traffic_add, ip_change or resize" eg:"resize"`
	Supported  bool   `json:"supported" dc:"The current provider capability snapshot advertises this action" eg:"true"`
	CanQuote   bool   `json:"canQuote" dc:"Source facts permit requesting an authoritative commercial quote; this does not imply merchant policy, price or stock approval" eg:"true"`
	ReasonCode string `json:"reasonCode" dc:"Unavailable reason: capability_unavailable, unsupported, entitlement_unavailable, operation_in_progress, runtime_unconfirmed, runtime_state_conflict or choices_unavailable; empty when quote request is allowed" eg:""`
}
