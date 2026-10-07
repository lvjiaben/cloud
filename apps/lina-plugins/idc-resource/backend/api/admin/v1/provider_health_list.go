// This file defines the bounded staff Provider health projection.
package v1

import "github.com/gogf/gf/v2/frame/g"

type ProviderHealthListReq struct {
	Page      int `json:"page" in:"query" v:"between:1,1000000" dc:"One-based page number; cannot be combined with a cursor" eg:"1"`
	g.Meta    `path:"/admin/resource/channel-health" method:"get" tags:"IDC Resource Health" summary:"List Provider channel health" dc:"Return one bounded keyset page from the local current health projection. This endpoint never calls a Provider and never exposes credentials, endpoints, raw upstream errors, or immutable snapshot history." permission:"idc:admin:resource:health:list"`
	ChannelNo string `json:"channelNo" in:"query" v:"max-length:64" dc:"Optional exact opaque resource channel number; omitted to include every visible channel" eg:"channel_primary"`
	Cursor    string `json:"cursor" in:"query" v:"max-length:64" dc:"Opaque channel continuation cursor returned by the previous page; omitted for the first page" eg:"channel_primary"`
	Limit     int    `json:"limit" in:"query" v:"between:1,100" d:"25" dc:"Maximum returned channel health projections; defaults to 25 when omitted" eg:"25"`
}

type ProviderHealthListRes struct {
	Total      int                          `json:"total" dc:"Exact total within the current staff scope and filters" eg:"120"`
	Items      []*ProviderChannelHealthItem `json:"items" dc:"At most 100 local current channel health projections" eg:"[]"`
	NextCursor string                       `json:"nextCursor" dc:"Opaque continuation cursor; empty on the last page" eg:"channel_secondary"`
}

type ProviderHealthObservation struct {
	HealthCode  string `json:"healthCode" dc:"Normalized state: healthy means confirmed success, degraded means a failure was observed, unavailable means writes are protected, and unknown means no usable observation exists" eg:"healthy"`
	SummaryCode string `json:"summaryCode" dc:"Stable non-secret machine summary code" eg:"provider.write.succeeded"`
	LatencyMs   int    `json:"latencyMs" dc:"Bounded latest operation latency in milliseconds" eg:"126"`
	ObservedAt  *int64 `json:"observedAt" dc:"Latest observation Unix timestamp in milliseconds; null before observation" eg:"1786464000000"`
}

type ProviderHealthPolicy struct {
	WindowSampleLimit         int    `json:"windowSampleLimit" dc:"Maximum retained write samples from 5 through 100" eg:"20"`
	MinimumSamplesToProtect   int    `json:"minimumSamplesToProtect" dc:"Minimum samples required before write protection may open" eg:"5"`
	FailureLimit              int    `json:"failureLimit" dc:"Write failure count that opens protection" eg:"3"`
	UnknownResultLimit        int    `json:"unknownResultLimit" dc:"Unknown-result count that opens protection" eg:"2"`
	ConsecutiveFailureLimit   int    `json:"consecutiveFailureLimit" dc:"Consecutive write failure count that opens protection" eg:"3"`
	ProtectionDurationSeconds int    `json:"protectionDurationSeconds" dc:"Automatic write protection duration from 60 through 86400 seconds" eg:"300"`
	Version                   int64  `json:"version" dc:"Policy optimistic concurrency version" eg:"1"`
	UpdatedAt                 *int64 `json:"updatedAt" dc:"Latest audited policy update Unix timestamp in milliseconds; null before the first update" eg:"1786464000000"`
}

type ProviderChannelHealthItem struct {
	ChannelNo           string                     `json:"channelNo" dc:"Opaque local resource channel number" eg:"channel_primary"`
	ChannelName         string                     `json:"channelName" dc:"Resource channel display name" eg:"Primary Cloud"`
	ProviderID          string                     `json:"providerId" dc:"Compiled Provider descriptor identifier" eg:"zjmfcloud"`
	ChannelStatus       string                     `json:"channelStatus" dc:"Local channel configuration state, such as enabled or disabled" eg:"enabled"`
	HealthCode          string                     `json:"healthCode" dc:"Derived current state: healthy, degraded, unavailable, or unknown" eg:"healthy"`
	CircuitOpen         bool                       `json:"circuitOpen" dc:"Whether new automatic Provider writes are protected" eg:"false"`
	WriteProtectedUntil *int64                     `json:"writeProtectedUntil" dc:"Database-clock write-protection deadline as a Unix timestamp in milliseconds; null when inactive" eg:"1786464300000"`
	SampleCount         int                        `json:"sampleCount" dc:"Current bounded write-window sample count" eg:"8"`
	FailureCount        int                        `json:"failureCount" dc:"Current bounded write-window failure count" eg:"1"`
	UnknownCount        int                        `json:"unknownCount" dc:"Current bounded write-window unknown-result count" eg:"0"`
	ConsecutiveFailures int                        `json:"consecutiveFailures" dc:"Current consecutive write failure count" eg:"0"`
	LastSummaryCode     string                     `json:"lastSummaryCode" dc:"Latest stable non-secret write summary code" eg:"provider.write.succeeded"`
	LastObservedAt      *int64                     `json:"lastObservedAt" dc:"Latest write observation Unix timestamp in milliseconds; null before observation" eg:"1786464000000"`
	Policy              *ProviderHealthPolicy      `json:"policy" dc:"Merchant-managed bounded write-protection policy" eg:"{}"`
	Authentication      *ProviderHealthObservation `json:"authentication" dc:"Latest authentication health observation" eg:"{}"`
	Query               *ProviderHealthObservation `json:"query" dc:"Latest read-query health observation" eg:"{}"`
	Write               *ProviderHealthObservation `json:"write" dc:"Latest write health observation" eg:"{}"`
	ProviderCapacity    *ProviderHealthObservation `json:"providerCapacity" dc:"Latest Provider capacity health observation" eg:"{}"`
}
