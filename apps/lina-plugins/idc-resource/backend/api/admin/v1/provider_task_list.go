// This file defines the bounded Provider task management queue.
package v1

import "github.com/gogf/gf/v2/frame/g"

type ProviderTaskListReq struct {
	Page      int `json:"page" in:"query" v:"between:1,1000000" dc:"One-based page number; cannot be combined with a cursor" eg:"1"`
	g.Meta    `path:"/admin/resource/tasks" method:"get" tags:"IDC Resource Tasks" summary:"List Provider tasks" dc:"Return one bounded keyset page from the local Provider task projection. This endpoint never calls a Provider and never returns task parameters, credentials, secret bindings, or raw upstream payloads." permission:"idc:admin:resource:task:list"`
	ChannelNo string `json:"channelNo" in:"query" v:"max-length:64" dc:"Optional exact opaque resource channel number" eg:"channel_primary"`
	Status    string `json:"status" in:"query" v:"max-length:24" dc:"Optional exact durable Provider task status" eg:"result_unknown"`
	Cursor    string `json:"cursor" in:"query" v:"max-length:64" dc:"Task number continuation cursor returned by the previous page" eg:"task_0123456789abcdef"`
	Limit     int    `json:"limit" in:"query" v:"between:1,100" d:"25" dc:"Maximum returned task projections; defaults to 25 when omitted" eg:"25"`
}

type ProviderTaskListRes struct {
	Total      int                 `json:"total" dc:"Exact total within the current staff scope and filters" eg:"120"`
	Items      []*ProviderTaskItem `json:"items" dc:"At most 100 local non-secret task projections" eg:"[]"`
	NextCursor string              `json:"nextCursor" dc:"Continuation task number; empty on the last page" eg:"task_0123456789abcdef"`
}

type ProviderTaskManualTakeover struct {
	TakeoverNo     string `json:"takeoverNo" dc:"Opaque immutable manual takeover number" eg:"takeover_0123456789abcdef0123456789abcdef"`
	FromStatus     string `json:"fromStatus" dc:"Eligible task status from which automation was closed" eg:"result_unknown"`
	TaskVersion    int64  `json:"taskVersion" dc:"Task version selected by the operator" eg:"3"`
	ReasonCode     string `json:"reasonCode" dc:"Stable constrained takeover reason code" eg:"provider_result_unresolved"`
	EvidenceCount  int    `json:"evidenceCount" dc:"Number of immutable evidence records selected from 1 through 16" eg:"2"`
	AssignedUserID int64  `json:"assignedUserId" dc:"Host staff identity that accepted responsibility" eg:"1001"`
	CreatedAt      int64  `json:"createdAt" dc:"Database-clock takeover Unix timestamp in milliseconds" eg:"1786464000000"`
}

type ProviderTaskItem struct {
	TaskNo                  string                      `json:"taskNo" dc:"Opaque local Provider task number" eg:"task_0123456789abcdef"`
	CapabilityCode          string                      `json:"capabilityCode" dc:"Normalized Provider capability code" eg:"server.start"`
	LocalResourceNo         string                      `json:"localResourceNo" dc:"Opaque local resource number; empty before provision mapping" eg:"resource_0123456789abcdef"`
	ChannelNo               string                      `json:"channelNo" dc:"Opaque local resource channel number" eg:"channel_primary"`
	ChannelName             string                      `json:"channelName" dc:"Resource channel display name" eg:"Primary Cloud"`
	Status                  string                      `json:"status" dc:"Durable Provider task state" eg:"result_unknown"`
	AttemptCount            int                         `json:"attemptCount" dc:"Number of Provider write attempts already claimed" eg:"1"`
	MaxAttempts             int                         `json:"maxAttempts" dc:"Maximum Provider write attempts" eg:"8"`
	ObservationAttemptCount int                         `json:"observationAttemptCount" dc:"Read-only reconciliation attempts; never authorizes another write" eg:"4"`
	ResultUnknown           bool                        `json:"resultUnknown" dc:"Whether the task is protected by an unresolved-write hard fence" eg:"true"`
	Version                 int64                       `json:"version" dc:"Optimistic task version" eg:"3"`
	NextObserveAt           *int64                      `json:"nextObserveAt" dc:"Next local read-only reconciliation Unix timestamp in milliseconds; null when none" eg:"1786464300000"`
	CreatedAt               int64                       `json:"createdAt" dc:"Task creation Unix timestamp in milliseconds" eg:"1786463000000"`
	UpdatedAt               int64                       `json:"updatedAt" dc:"Latest task update Unix timestamp in milliseconds" eg:"1786464000000"`
	Takeover                *ProviderTaskManualTakeover `json:"takeover" dc:"Immutable assignment after automation is closed; null before takeover" eg:"{}"`
}
