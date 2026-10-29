package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

type ActionCreateReq struct {
	g.Meta         `path:"/account/service-entitlements/{entitlementNo}/actions/{actionKey}" method:"post" tags:"IDC Customer Services" summary:"Run an action on my service" pageBridgeCapability:"service-entitlements.customer.v1" pageBridgeHost:"customer"`
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
type EntitlementListReq struct {
	DirectoryNo string `json:"directoryNo" v:"max-length:40" dc:"Optional member navigation directory; omitted keeps the plugin customer list ungrouped" eg:"pdr_example"`
	PluginID    string `json:"pluginId" v:"max-length:128" dc:"Optional owning fulfillment plugin filter for one plugin-owned purchased-services block" eg:"yunsu-ssl"`
	g.Meta      `path:"/account/service-entitlements" method:"get" tags:"IDC Customer Services" summary:"List my managed services" pageBridgeCapability:"service-entitlements.customer.v1" pageBridgeHost:"customer"`
	Cursor      string `json:"cursor"`
	Limit       int    `json:"limit" d:"20" v:"between:1,100"`
}
type EntitlementListRes struct {
	Items      []ServiceEntitlement `json:"items"`
	NextCursor string               `json:"nextCursor"`
}
type EntitlementGetReq struct {
	g.Meta        `path:"/account/service-entitlements/{entitlementNo}" method:"get" tags:"IDC Customer Services" summary:"Get my managed service" pageBridgeCapability:"service-entitlements.customer.v1" pageBridgeHost:"customer"`
	EntitlementNo string `json:"entitlementNo" in:"path" v:"required|max-length:40"`
}
type EntitlementGetRes struct {
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
	g.Meta `path:"/account/service-action-tasks/{taskNo}" method:"get" tags:"IDC Customer Services" summary:"Get my service action" pageBridgeCapability:"service-entitlements.customer.v1" pageBridgeHost:"customer"`
	TaskNo string `json:"taskNo" in:"path" v:"required|max-length:40"`
}
type ActionTaskGetRes struct {
	Task *ActionTask `json:"task"`
}

type DownloadTicketCreateReq struct {
	g.Meta        `path:"/account/service-entitlements/{entitlementNo}/downloads/{downloadKey}/tickets" method:"post" tags:"IDC Customer Services" summary:"Create a one-use certificate download ticket" pageBridgeCapability:"service-entitlements.customer.v1" pageBridgeHost:"customer"`
	EntitlementNo string `json:"entitlementNo" in:"path" v:"required|max-length:40"`
	DownloadKey   string `json:"downloadKey" in:"path" v:"required|max-length:64"`
}
type DownloadTicketCreateRes struct {
	Ticket    string `json:"ticket"`
	FileName  string `json:"fileName"`
	ExpiresAt int64  `json:"expiresAt"`
}
type DownloadOpenReq struct {
	g.Meta `path:"/account/service-downloads/{ticket}" method:"get" tags:"IDC Customer Services" summary:"Redeem my one-use certificate download" pageBridgeCapability:"service-entitlements.customer.v1" pageBridgeHost:"customer"`
	Ticket string `json:"ticket" in:"path" v:"required|max-length:80"`
}
type DownloadOpenRes struct{}
