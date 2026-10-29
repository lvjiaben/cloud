// This file separates server payment notifications from browser return routing.
package v1

import "github.com/gogf/gf/v2/frame/g"

type PaymentPublicAddress struct {
	NotificationOrigin   string `json:"notificationOrigin" dc:"Independent HTTPS origin receiving payment and refund notifications" eg:"https://notify.example.com"`
	CustomerReturnOrigin string `json:"customerReturnOrigin" dc:"Customer browser return origin; HTTPS or a loopback development origin" eg:"http://127.0.0.1:6032"`
	Version              int64  `json:"version" dc:"Current settings version; zero before the first save" eg:"1"`
}

type PaymentPublicAddressGetReq struct {
	g.Meta `path:"/admin/payment-public-addresses" method:"get" tags:"Payment Channel" summary:"Read payment notification and customer return addresses" dc:"Read current merchant-wide routing without exposing credentials or creating payment attempts." permission:"idc:admin:payment:channel:view"`
}

type PaymentPublicAddressGetRes struct {
	Addresses PaymentPublicAddress `json:"addresses" dc:"Current independent payment routing settings" eg:"{}"`
}

type PaymentPublicAddressSaveReq struct {
	g.Meta               `path:"/admin/payment-public-addresses" method:"put" tags:"Payment Channel" summary:"Save independent payment notification and browser return addresses" dc:"Replace current merchant-wide origins immediately after direct staff and version checks. Empty origins disable new payments; existing financial facts and callbacks remain unchanged." permission:"idc:admin:payment:channel:manage"`
	NotificationOrigin   string `json:"notificationOrigin" v:"length:0,2048" dc:"HTTPS notification gateway origin, independent of the system origin" eg:"https://notify.example.com"`
	CustomerReturnOrigin string `json:"customerReturnOrigin" v:"length:0,2048" dc:"Customer browser return origin" eg:"http://127.0.0.1:6032"`
	ExpectedVersion      int64  `json:"expectedVersion" v:"min:0" dc:"Version read before editing" eg:"0"`
}

type PaymentPublicAddressSaveRes struct {
	Addresses PaymentPublicAddress `json:"addresses" dc:"Current independent payment routing settings" eg:"{}"`
}
