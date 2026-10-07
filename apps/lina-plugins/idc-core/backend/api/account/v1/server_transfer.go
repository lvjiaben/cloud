package v1

import "github.com/gogf/gf/v2/frame/g"

type ServerTransferRequestReq struct {
	g.Meta         `path:"/account/servers/{serviceNo}/transfers" method:"post" tags:"IDC Customer Servers" summary:"Request my server ownership transfer" dc:"Accept one recipient registered email without revealing whether that account exists. An eligible recipient must confirm in the customer center within 24 hours."`
	ServiceNo      string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Owned server service number"`
	RecipientEmail string `json:"recipientEmail" v:"required|email|max-length:320" dc:"Recipient registered email"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:256" dc:"Customer-generated retry key"`
}

type ServerTransferRequestRes struct {
	Accepted bool `json:"accepted" dc:"Whether the request was accepted for safe processing"`
	Reused   bool `json:"reused" dc:"Whether the same request was already accepted"`
}

type IncomingServerTransfer struct {
	TransferNo string `json:"transferNo" dc:"Stable transfer request number"`
	ServiceNo  string `json:"serviceNo" dc:"Server service number to receive"`
	ExpiresAt  int64  `json:"expiresAt" dc:"Confirmation deadline as Unix milliseconds"`
}

type ServerTransferIncomingListReq struct {
	g.Meta `path:"/account/server-transfers/incoming" method:"get" tags:"IDC Customer Servers" summary:"List my incoming server transfers" dc:"Return at most 50 unexpired transfers addressed to the current verified customer."`
}

type ServerTransferIncomingListRes struct {
	Items []*IncomingServerTransfer `json:"items"`
}

type ServerTransferConfirmationCreateReq struct {
	g.Meta         `path:"/account/server-transfers/{transferNo}/confirmations" method:"post" tags:"IDC Customer Servers" summary:"Confirm my incoming server transfer" dc:"Recheck the policy, ownership, billing, refund, operation and confirmation deadline before atomically changing future server ownership."`
	TransferNo     string `json:"transferNo" in:"path" v:"required|max-length:40"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:256"`
}

type ServerTransferConfirmationCreateRes struct {
	TransferNo string `json:"transferNo"`
	ServiceNo  string `json:"serviceNo"`
	Status     string `json:"status"`
	Reused     bool   `json:"reused"`
}
