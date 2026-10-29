// This file defines the bounded secret-free Provider invocation log contract.
package v1

import "github.com/gogf/gf/v2/frame/g"

// ProviderAPILogListReq filters immutable local invocation history without upstream calls.
type ProviderAPILogListReq struct {
	g.Meta      `path:"/admin/resource/api-logs" method:"get" tags:"IDC Resource Tasks" summary:"List Provider API logs" dc:"Return a bounded merchant-local page of body-free invocation history without calling a Provider" permission:"idc:admin:resource:api-log:list"`
	Page        int    `json:"page" in:"query" d:"1" v:"between:1,1000000" dc:"One-based page number" eg:"1"`
	Limit       int    `json:"limit" in:"query" d:"25" v:"between:1,100" dc:"Page size from 1 through 100" eg:"25"`
	ChannelNo   string `json:"channelNo" in:"query" v:"max-length:64" dc:"Optional exact channel number; omitted means all authorized channels" eg:"channel_primary"`
	ChannelName string `json:"channelName" in:"query" v:"max-length:128" dc:"Optional exact interface display name; omitted means all names" eg:"Primary cloud"`
	MethodCode  string `json:"methodCode" in:"query" v:"max-length:96" dc:"Optional exact Provider method; omitted means all methods" eg:"connection.verify"`
	Result      string `json:"result" in:"query" v:"in:,succeeded,failed" dc:"Optional invocation result: succeeded for completed calls, failed for runtime errors or failed, unknown or invalid operation results; omitted means both" eg:"failed"`
}

// ProviderAPILogListRes carries the exact filtered total and current page only.
type ProviderAPILogListRes struct {
	Items []*ProviderAPILogItem `json:"items" dc:"Current page of body-free invocation records" eg:"[]"`
	Total int                   `json:"total" dc:"Exact count under the same merchant scope and filters" eg:"10"`
}

// ProviderAPILogItem describes call completion, never final server provisioning success.
type ProviderAPILogItem struct {
	ID               int64  `json:"id" dc:"Immutable local history record ID" eg:"1"`
	ChannelNo        string `json:"channelNo" dc:"Local channel reference" eg:"channel_primary"`
	ChannelName      string `json:"channelName" dc:"Local channel display name" eg:"Primary cloud"`
	ProviderPluginID string `json:"providerPluginId" dc:"Provider connector ID" eg:"idc-zjmfcloud"`
	MethodCode       string `json:"methodCode" dc:"Invoked Provider method" eg:"connection.verify"`
	Succeeded        bool   `json:"succeeded" dc:"Whether invocation completed without a runtime error or a failed or unknown operation result; does not imply final resource success" eg:"true"`
	ErrorCode        string `json:"errorCode" dc:"Safe owner-generated failure class; empty on success" eg:"provider_call_failed"`
	DurationMS       int64  `json:"durationMs" dc:"Invocation duration in milliseconds" eg:"120"`
	StartedAt        int64  `json:"startedAt" dc:"Invocation start Unix timestamp in milliseconds" eg:"1786464000000"`
}
