// This file declares payment-channel configuration and bounded administration
// queries. Pagination never includes credential material or changes permissions.

package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

type PaymentChannelItem struct {
	ChannelNo         string   `json:"channelNo"`
	ProviderCode      string   `json:"providerCode"`
	DisplayName       string   `json:"displayName"`
	EndpointOrigin    string   `json:"endpointOrigin"`
	MerchantReference string   `json:"merchantReference"`
	StatusCode        string   `json:"statusCode"`
	Modes             []string `json:"modes"`
	Currencies        []string `json:"currencies"`
	Version           int64    `json:"version"`
	VerifiedAt        int64    `json:"verifiedAt"`
}

type PaymentChannelListReq struct {
	g.Meta   `path:"/admin/payment-channels" method:"get" tags:"Payment Channel" summary:"List configured payment channels" dc:"Return one bounded page of configured channels and its exact visible total without credentials." permission:"idc:admin:payment:channel:view"`
	Page     int `json:"page" d:"1" v:"between:1,1000000" dc:"One-based page number" eg:"1"`
	PageSize int `json:"pageSize" d:"20" v:"between:1,100" dc:"Number of rows per page from 1 to 100" eg:"20"`
}
type PaymentChannelListRes struct {
	Channels           []*PaymentChannelItem           `json:"channels"`
	ProviderCandidates []*PaymentProviderCandidateItem `json:"providerCandidates"`
	Total              int                             `json:"total" dc:"Exact number of visible matching records" eg:"123"`
}

type PaymentProviderCandidateItem struct {
	PluginID               string          `json:"pluginId"`
	ProviderCode           string          `json:"providerCode"`
	DisplayName            string          `json:"displayName"`
	ReleaseVersion         string          `json:"releaseVersion"`
	EndpointOrigin         string          `json:"endpointOrigin"`
	Modes                  []string        `json:"modes"`
	Currencies             []string        `json:"currencies"`
	InterfaceConfiguration json.RawMessage `json:"interfaceConfiguration"`
}

type PaymentChannelCreateReq struct {
	g.Meta               `path:"/admin/payment-channels" method:"post" tags:"Payment Channel" summary:"Verify and create a payment channel" permission:"idc:admin:payment:channel:manage"`
	ChannelNo            string   `json:"channelNo" v:"required|length:1,64"`
	ProviderCode         string   `json:"providerCode" v:"required|in:alipay,wechatpay,epay,unionpay"`
	DisplayName          string   `json:"displayName" v:"required|length:1,128"`
	EndpointOrigin       string   `json:"endpointOrigin" v:"required|length:8,2048"`
	MerchantReference    string   `json:"merchantReference" v:"length:0,256"`
	MerchantID           string   `json:"merchantId" v:"required|length:1,256"`
	AppID                string   `json:"appId" v:"length:0,256"`
	Secret               string   `json:"secret" v:"length:0,8192"`
	PrivateKey           string   `json:"privateKey" v:"length:0,65535"`
	PublicKey            string   `json:"publicKey" v:"length:0,65535"`
	CertificateSerialNo  string   `json:"certificateSerialNo" v:"length:0,256"`
	WechatPayPublicKeyID string   `json:"wechatPayPublicKeyId" v:"length:0,256"`
	APIV3Key             string   `json:"apiV3Key" v:"length:0,64"`
	JSAPIAuthorized      bool     `json:"jsapiAuthorized"`
	EnabledModes         []string `json:"enabledModes"`
}
type PaymentChannelCreateRes struct {
	Channel *PaymentChannelItem `json:"channel"`
}

type PaymentChannelStatusReq struct {
	g.Meta          `path:"/admin/payment-channels/{channelNo}/status" method:"put" tags:"Payment Channel" summary:"Enable or disable a verified payment channel" permission:"idc:admin:payment:channel:manage"`
	ChannelNo       string `json:"channelNo" v:"required"`
	StatusCode      string `json:"statusCode" v:"required|in:enabled,disabled"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
}
type PaymentChannelStatusRes struct {
	Channel *PaymentChannelItem `json:"channel"`
}

type PaymentChannelConnectionVerificationReq struct {
	g.Meta          `path:"/admin/payment-channels/{channelNo}/connection-verifications" method:"post" tags:"Payment Channel" summary:"Run a read-only payment channel connection check" permission:"idc:admin:payment:channel:manage"`
	ChannelNo       string `json:"channelNo" v:"required"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1"`
}
type PaymentChannelConnectionVerificationRes struct {
	Channel *PaymentChannelItem `json:"channel"`
}
