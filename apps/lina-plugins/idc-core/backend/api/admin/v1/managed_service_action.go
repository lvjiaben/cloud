package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

type ActionCreateReq struct {
	g.Meta         `path:"/admin/service-entitlements/{entitlementNo}/actions/{actionKey}" method:"post" tags:"IDC Managed Services" summary:"Run a governed service action" permission:"idc:admin:service-entitlement:action" pageBridgeCapability:"service-entitlements.admin.v1" pageBridgeHost:"admin"`
	EntitlementNo  string          `json:"entitlementNo" in:"path" v:"required|max-length:40"`
	ActionKey      string          `json:"actionKey" in:"path" v:"required|max-length:64"`
	IdempotencyKey string          `json:"idempotencyKey" v:"required|max-length:256"`
	Input          json.RawMessage `json:"input"`
}
type LocalizedTitle struct {
	EnUS string `json:"enUS"`
	ZhCN string `json:"zhCN"`
}
type ServiceActionDeclaration struct {
	Key          string         `json:"key"`
	Title        LocalizedTitle `json:"title"`
	Execution    string         `json:"execution"`
	Availability string         `json:"availability"`
}
type ServiceDownloadDeclaration struct {
	Key   string         `json:"key"`
	Title LocalizedTitle `json:"title"`
}
type ServiceEntitlement struct {
	EntitlementNo    string                       `json:"entitlementNo"`
	ProductNameZhCN  string                       `json:"productNameZhCn"`
	ProductNameEnUS  string                       `json:"productNameEnUs"`
	Status           string                       `json:"status"`
	StatusCode       string                       `json:"statusCode"`
	CustomerName     string                       `json:"customerName,omitempty"`
	PrimaryDomain    string                       `json:"primaryDomain"`
	ValidationMethod string                       `json:"validationMethod"`
	Validation       json.RawMessage              `json:"validation"`
	AvailableActions []string                     `json:"availableActions"`
	IssuedAt         int64                        `json:"issuedAt"`
	UpdatedAt        int64                        `json:"updatedAt"`
	StartedAt        int64                        `json:"startedAt"`
	ExpiresAt        int64                        `json:"expiresAt,omitempty"`
	Actions          []ServiceActionDeclaration   `json:"actions"`
	Downloads        []ServiceDownloadDeclaration `json:"downloads"`
}
type ManagedServiceEntitlementListReq struct {
	g.Meta `path:"/admin/service-entitlements" method:"get" tags:"IDC Managed Services" summary:"List managed services" permission:"idc:admin:service-entitlement:list" pageBridgeCapability:"service-entitlements.admin.v1" pageBridgeHost:"admin"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit" d:"20" v:"between:1,100"`
}
type ManagedServiceEntitlementListRes struct {
	Items      []ServiceEntitlement `json:"items"`
	NextCursor string               `json:"nextCursor"`
}
type ManagedServiceEntitlementGetReq struct {
	g.Meta        `path:"/admin/service-entitlements/{entitlementNo}" method:"get" tags:"IDC Managed Services" summary:"Get a managed service" permission:"idc:admin:service-entitlement:list" pageBridgeCapability:"service-entitlements.admin.v1" pageBridgeHost:"admin"`
	EntitlementNo string `json:"entitlementNo" in:"path" v:"required|max-length:40"`
}
type ManagedServiceEntitlementGetRes struct {
	Item *ServiceEntitlement `json:"item"`
}
type ActionTask struct {
	TaskNo         string          `json:"taskNo"`
	EntitlementNo  string          `json:"entitlementNo"`
	ActionKey      string          `json:"actionKey"`
	Status         string          `json:"status"`
	SafeProjection json.RawMessage `json:"safeProjection"`
	Version        int64           `json:"version"`
}
type ActionCreateRes struct {
	Task *ActionTask `json:"task"`
}
type ActionTaskGetReq struct {
	g.Meta `path:"/admin/service-action-tasks/{taskNo}" method:"get" tags:"IDC Managed Services" summary:"Get a governed service action" permission:"idc:admin:service-entitlement:list" pageBridgeCapability:"service-entitlements.admin.v1" pageBridgeHost:"admin"`
	TaskNo string `json:"taskNo" in:"path" v:"required|max-length:40"`
}
type ActionTaskGetRes struct {
	Task *ActionTask `json:"task"`
}
type ActionTakeoverReq struct {
	g.Meta          `path:"/admin/service-action-tasks/{taskNo}/takeover" method:"post" tags:"IDC Managed Services" summary:"Take over an uncertain service action" permission:"idc:admin:service-entitlement:takeover" pageBridgeCapability:"service-entitlements.admin.v1" pageBridgeHost:"admin"`
	TaskNo          string `json:"taskNo" in:"path" v:"required|max-length:40"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
}
type ActionTakeoverRes struct {
	Task *ActionTask `json:"task"`
}
type DownloadTicketCreateReq struct {
	g.Meta        `path:"/admin/service-entitlements/{entitlementNo}/downloads/{downloadKey}/tickets" method:"post" tags:"IDC Managed Services" summary:"Create a one-use certificate download ticket" permission:"idc:admin:service-entitlement:download" pageBridgeCapability:"service-entitlements.admin.v1" pageBridgeHost:"admin"`
	EntitlementNo string `json:"entitlementNo" in:"path" v:"required|max-length:40"`
	DownloadKey   string `json:"downloadKey" in:"path" v:"required|max-length:64"`
}
type DownloadTicketCreateRes struct {
	Ticket    string `json:"ticket"`
	FileName  string `json:"fileName"`
	ExpiresAt int64  `json:"expiresAt"`
}
type DownloadOpenReq struct {
	g.Meta `path:"/admin/service-downloads/{ticket}" method:"get" tags:"IDC Managed Services" summary:"Redeem a one-use certificate download" permission:"idc:admin:service-entitlement:download" pageBridgeCapability:"service-entitlements.admin.v1" pageBridgeHost:"admin"`
	Ticket string `json:"ticket" in:"path" v:"required|max-length:80"`
}
type DownloadOpenRes struct{}
