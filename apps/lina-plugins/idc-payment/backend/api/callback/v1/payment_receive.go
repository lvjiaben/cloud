package v1

import "github.com/gogf/gf/v2/frame/g"

type PaymentReceiveReq struct {
	g.Meta    `path:"/callbacks/payments/{channelNo}" method:"get,post" tags:"IDC Payment Callbacks" summary:"Receive a payment-provider callback" dc:"Verify the exact merchant channel callback and append one immutable receipt plus asynchronous work item. GET is exclusively the EPay signed server-notification protocol; POST retains existing providers. This endpoint never posts money or settles an order inline."`
	ChannelNo string `json:"channelNo" in:"path" v:"required|max-length:64" dc:"Configured payment channel reference used to select the exact verification credentials" eg:"channel-alipay-main"`
}
type PaymentReceiveRes struct{}
