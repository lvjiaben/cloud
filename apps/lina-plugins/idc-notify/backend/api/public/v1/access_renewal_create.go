package v1

import "github.com/gogf/gf/v2/frame/g"

type AccessRenewalCreateReq struct {
	g.Meta          `path:"/public/support/presales/access-renewals" method:"post" tags:"IDC Public Presales" summary:"Renew anonymous presales access" dc:"Consumes one freshly verified code and replaces all prior access links for this anonymous ticket."`
	BrowserNonce    string `json:"browserNonce" v:"required|min-length:16|max-length:128" dc:"Random nonce retained by this browser" eg:"7bdb8be1-cbe4-4c32-8b83-e5122ea20a42"`
	Destination     string `json:"destination" v:"required|max-length:320" dc:"Previously verified email address or E.164 phone number" eg:"help@example.com"`
	DestinationKind string `json:"destinationKind" v:"required|in:email,sms" dc:"Destination kind" eg:"email"`
	Locale          string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Message language" eg:"zh-CN"`
	OperationNo     string `json:"operationNo" v:"required|min-length:8|max-length:64" dc:"Browser-generated renewal idempotency operation number" eg:"presales-renew-20260815-001"`
	VerificationNo  string `json:"verificationNo" v:"required|max-length:48" dc:"Verification number returned by code delivery" eg:"nver_01JZ..."`
	Code            string `json:"code" v:"required|length:6" dc:"Six-digit verification code" eg:"123456"`
	TicketNo        string `json:"ticketNo" v:"required|max-length:48" dc:"Anonymous presales ticket number" eg:"TKT_01JZ..."`
}

type AccessRenewalCreateRes PresalesAccessProjection
