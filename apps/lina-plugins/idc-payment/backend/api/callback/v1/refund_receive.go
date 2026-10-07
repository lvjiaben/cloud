// This file defines the public refund-provider callback route contract. The
// route identifies only a configured channel and never accepts refund policy.

package v1

import "github.com/gogf/gf/v2/frame/g"

// RefundReceiveReq identifies the configured channel whose signed callback is being received.
type RefundReceiveReq struct {
	g.Meta    `path:"/callbacks/refunds/{channelNo}" method:"post" tags:"IDC Payment Callbacks" summary:"Receive a refund-provider callback" dc:"Verify and store one refund-provider result for the exact configured channel. The callback never decides refund eligibility, approval, amount, destination, or customer consent."`
	ChannelNo string `json:"channelNo" in:"path" v:"required|max-length:64" dc:"Configured payment channel reference used to select the exact verification credentials" eg:"channel-wechat-main"`
}

// RefundReceiveRes is empty because the controller writes the provider protocol acknowledgement directly.
type RefundReceiveRes struct{}
