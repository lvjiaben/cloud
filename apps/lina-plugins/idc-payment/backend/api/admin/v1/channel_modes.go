// This file updates enabled payment entry points without replacing credentials.
package v1

import "github.com/gogf/gf/v2/frame/g"

type PaymentChannelModesSaveReq struct {
	g.Meta          `path:"/admin/payment-channels/{channelNo}/payment-modes" method:"put" tags:"Payment Channel" summary:"Update enabled payment modes" dc:"Update only installed provider-supported modes under direct staff authorization and optimistic concurrency. Credentials, verification and financial history remain unchanged." permission:"idc:admin:payment:channel:manage"`
	ChannelNo       string   `json:"channelNo" in:"path" v:"required|length:1,64" dc:"Exact payment channel number" eg:"EPAY_MAIN"`
	EnabledModes    []string `json:"enabledModes" v:"required" dc:"Nonempty distinct payment modes supported by the currently installed provider" eg:"[\"epay_alipay\",\"epay_wechat\"]"`
	ExpectedVersion int64    `json:"expectedVersion" v:"required|min:1" dc:"Channel version read before editing" eg:"1"`
}
type PaymentChannelModesSaveRes struct {
	Channel *PaymentChannelItem `json:"channel" dc:"Updated channel without credential material" eg:"{}"`
}
