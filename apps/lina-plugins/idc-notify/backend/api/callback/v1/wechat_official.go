package v1

import "github.com/gogf/gf/v2/frame/g"

type WeChatOfficialVerifyReq struct {
	g.Meta    `path:"/callbacks/wechat-official" method:"get" tags:"IDC Notification Callbacks" summary:"Verify WeChat Official Account encrypted callback" dc:"Verifies and decrypts the official encrypted callback challenge for the current channel generation"`
	Signature string `json:"msg_signature" in:"query" v:"required|max-length:128" dc:"Official encrypted-message signature" eg:"2f6f7f0d4fbd0123456789abcdef0123456789ab"`
	Timestamp string `json:"timestamp" in:"query" v:"required|max-length:20" dc:"Official callback timestamp in Unix seconds" eg:"1786752000"`
	Nonce     string `json:"nonce" in:"query" v:"required|max-length:128" dc:"Official callback nonce" eg:"593260414"`
	Encrypted string `json:"echostr" in:"query" v:"required|max-length:4096" dc:"Official encrypted callback challenge" eg:"encrypted-challenge"`
}

type WeChatOfficialVerifyRes struct{}

type WeChatOfficialReceiveReq struct {
	g.Meta    `path:"/callbacks/wechat-official" method:"post" tags:"IDC Notification Callbacks" summary:"Receive WeChat Official Account encrypted delivery result" dc:"Verifies, decrypts, deduplicates, and converges an official encrypted template-message result"`
	Signature string `json:"msg_signature" in:"query" v:"required|max-length:128" dc:"Official encrypted-message signature" eg:"2f6f7f0d4fbd0123456789abcdef0123456789ab"`
	Timestamp string `json:"timestamp" in:"query" v:"required|max-length:20" dc:"Official callback timestamp in Unix seconds" eg:"1786752000"`
	Nonce     string `json:"nonce" in:"query" v:"required|max-length:128" dc:"Official callback nonce" eg:"593260414"`
}

type WeChatOfficialReceiveRes struct{}
