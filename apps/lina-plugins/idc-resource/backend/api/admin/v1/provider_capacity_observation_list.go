// This file defines the bounded staff Provider-capacity observation projection.
package v1

import "github.com/gogf/gf/v2/frame/g"

type ProviderCapacityObservationListReq struct {
	Page      int `json:"page" in:"query" v:"between:1,1000000" dc:"One-based page number; cannot be combined with a cursor" eg:"1"`
	g.Meta    `path:"/admin/resource/provider-capacity-observations" method:"get" tags:"IDC Provider Capacity" summary:"List Provider capacity observations" dc:"Return a bounded keyset page of local capacity collection status. Provider cursors, fingerprints, endpoints, credentials, and raw errors are never returned. These observations do not create or update product stock records." permission:"idc:admin:resource:provider-capacity:list"`
	ChannelNo string `json:"channelNo" in:"query" v:"max-length:64" dc:"Optional opaque resource channel number"`
	Kind      string `json:"kind" in:"query" v:"in:region,line,plan,image,stock" dc:"Optional normalized Provider capacity observation kind"`
	Status    string `json:"status" in:"query" v:"in:collecting,complete,failed,superseded" dc:"Optional local observation status"`
	Cursor    string `json:"cursor" in:"query" v:"max-length:64" dc:"Opaque keyset cursor returned by the previous page"`
	Limit     int    `json:"limit" in:"query" v:"between:1,100" d:"25" dc:"Maximum returned observations"`
}

type ProviderCapacityObservationListRes struct {
	Total      int                                `json:"total" dc:"Exact total within the current staff scope and filters" eg:"120"`
	Items      []*ProviderCapacityObservationItem `json:"items" dc:"At most 100 local capacity observation projections"`
	NextCursor string                             `json:"nextCursor" dc:"Opaque continuation cursor; empty on the last page"`
	Channels   []*ProviderCapacityChannelOption   `json:"channels" dc:"At most 500 enabled channel choices for capacity refresh"`
}

type ProviderCapacityChannelOption struct {
	ChannelNo   string `json:"channelNo" dc:"Opaque enabled resource channel number"`
	DisplayName string `json:"displayName" dc:"Resource channel display name"`
}

type ProviderCapacityObservationItem struct {
	ObservationNo  string `json:"observationNo" dc:"Opaque local capacity observation number"`
	ChannelNo      string `json:"channelNo" dc:"Opaque resource channel number"`
	ChannelName    string `json:"channelName" dc:"Resource channel display name"`
	Kind           string `json:"kind" dc:"Normalized Provider capacity observation kind"`
	Status         string `json:"status" dc:"Local collecting, complete, failed, or superseded state"`
	Trigger        string `json:"trigger" dc:"Trusted system or staff trigger classification"`
	ItemCount      int    `json:"itemCount" dc:"Validated capacity observation item count"`
	PageCount      int    `json:"pageCount" dc:"Durably staged Provider page count"`
	FailureCode    string `json:"failureCode" dc:"Stable non-secret terminal failure code"`
	RequestedAt    int64  `json:"requestedAt" dc:"Request time in Unix milliseconds"`
	LastObservedAt int64  `json:"lastObservedAt" dc:"Last local observation time in Unix milliseconds"`
	CompletedAt    *int64 `json:"completedAt" dc:"Terminal time in Unix milliseconds; null while collecting"`
}
