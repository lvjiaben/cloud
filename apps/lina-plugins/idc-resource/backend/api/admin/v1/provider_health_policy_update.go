// This file defines one audited safe Provider health policy update.
package v1

import "github.com/gogf/gf/v2/frame/g"

type ProviderHealthPolicyUpdateReq struct {
	g.Meta                    `path:"/admin/resource/channels/{channelNo}/health-policy" method:"put" tags:"IDC Resource Health" summary:"Update Provider health policy" dc:"Update bounded automatic write-protection thresholds with optimistic concurrency. Active protection cannot be manually closed or shortened, and this endpoint performs no Provider operation." permission:"idc:admin:resource:health-policy:write"`
	ChannelNo                 string `json:"channelNo" in:"path" v:"required|max-length:64" dc:"Exact opaque resource channel number" eg:"channel_primary"`
	WindowSampleLimit         int    `json:"windowSampleLimit" v:"required|between:5,100" dc:"Maximum retained write samples" eg:"20"`
	MinimumSamplesToProtect   int    `json:"minimumSamplesToProtect" v:"required|between:3,100" dc:"Minimum samples before protection may open" eg:"5"`
	FailureLimit              int    `json:"failureLimit" v:"required|between:1,100" dc:"Failure count that opens protection" eg:"3"`
	UnknownResultLimit        int    `json:"unknownResultLimit" v:"required|between:1,100" dc:"Unknown-result count that opens protection" eg:"2"`
	ConsecutiveFailureLimit   int    `json:"consecutiveFailureLimit" v:"required|between:1,100" dc:"Consecutive failure count that opens protection" eg:"3"`
	ProtectionDurationSeconds int    `json:"protectionDurationSeconds" v:"required|between:60,86400" dc:"Automatic write protection duration in seconds" eg:"300"`
	ExpectedVersion           int64  `json:"expectedVersion" v:"required|min:1" dc:"Policy version last read by the staff user" eg:"1"`
}

type ProviderHealthPolicyUpdateRes struct {
	Item *ProviderChannelHealthItem `json:"item" dc:"Committed current channel health and policy projection" eg:"{}"`
}
