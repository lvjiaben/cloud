package v1

import "github.com/gogf/gf/v2/frame/g"

type VerificationCodeCreateReq struct {
	g.Meta          `path:"/public/support/presales/verification-codes" method:"post" tags:"IDC Public Presales" summary:"Send a presales verification code" dc:"Consumes one bound human proof, atomically applies all abuse and cost limits, and performs at most one provider call."`
	BrowserNonce    string `json:"browserNonce" v:"required|min-length:16|max-length:128" dc:"Random nonce retained by this browser" eg:"7bdb8be1-cbe4-4c32-8b83-e5122ea20a42"`
	Destination     string `json:"destination" v:"required|max-length:320" dc:"Email address or E.164 phone number" eg:"help@example.com"`
	DestinationKind string `json:"destinationKind" v:"required|in:email,sms" dc:"Destination kind" eg:"email"`
	Locale          string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Message language" eg:"zh-CN"`
	Operation       string `json:"operation" v:"required|in:create,renew" dc:"Anonymous ticket operation" eg:"create"`
	TicketNo        string `json:"ticketNo" v:"max-length:48" dc:"Ticket number, required only for renewal" eg:"TKT_01JZ..."`
	ProofToken      string `json:"proofToken" v:"required|max-length:4096" dc:"Two-minute single-use human-verification proof" eg:"proof_01JZ..."`
	IdempotencyKey  string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Browser-generated idempotency key" eg:"presales-code-20260815-001"`
}

type VerificationCodeCreateRes VerificationCodeProjection
