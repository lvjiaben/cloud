// This file defines one staff-requested Provider capacity observation refresh.
package v1

import "github.com/gogf/gf/v2/frame/g"

type ProviderCapacityRefreshCreateReq struct {
	g.Meta    `path:"/admin/resource/channels/{channelNo}/provider-capacity-observations" method:"post" tags:"IDC Provider Capacity" summary:"Refresh Provider capacity observations" dc:"Create one local capacity collection. Provider access is asynchronous and advances at most one bounded page per worker claim. Observations never create or synchronize product stock records." permission:"idc:admin:resource:provider-capacity:refresh"`
	ChannelNo string `json:"channelNo" in:"path" v:"required|max-length:64" dc:"Enabled resource channel business number"`
	Kind      string `json:"kind" v:"required|in:region,line,plan,image,stock" dc:"Normalized Provider capacity observation kind"`
}

type ProviderCapacityRefreshCreateRes struct {
	ObservationNo string `json:"observationNo" dc:"Generated local capacity observation number"`
	Status        string `json:"status" dc:"Initial collecting state"`
}
