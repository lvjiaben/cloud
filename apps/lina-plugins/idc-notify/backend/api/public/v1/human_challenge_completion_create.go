package v1

import "github.com/gogf/gf/v2/frame/g"

type HumanChallengeCompletionCreateReq struct {
	g.Meta          `path:"/public/support/presales/human-challenge-completions" method:"post" tags:"IDC Public Presales" summary:"Complete a presales human challenge" dc:"Completes the bound challenge and returns a two-minute single-use proof."`
	BrowserNonce    string `json:"browserNonce" v:"required|min-length:16|max-length:128" dc:"Random nonce retained by this browser" eg:"7bdb8be1-cbe4-4c32-8b83-e5122ea20a42"`
	Destination     string `json:"destination" v:"required|max-length:320" dc:"Email address or E.164 phone number" eg:"help@example.com"`
	DestinationKind string `json:"destinationKind" v:"required|in:email,sms" dc:"Destination kind" eg:"email"`
	Locale          string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Message language" eg:"zh-CN"`
	Operation       string `json:"operation" v:"required|in:create,renew" dc:"Anonymous ticket operation" eg:"create"`
	TicketNo        string `json:"ticketNo" v:"max-length:48" dc:"Ticket number, required only for renewal" eg:"TKT_01JZ..."`
	ChallengeNo     string `json:"challengeNo" v:"required|max-length:96" dc:"Challenge number returned by creation" eg:"hc_01JZ..."`
	Answer          string `json:"answer" v:"max-length:256" dc:"Answer for the built-in challenge" eg:"7"`
	ProviderToken   string `json:"providerToken" v:"max-length:4096" dc:"Token for an installed external challenge provider" eg:"0.ABC..."`
}

type HumanChallengeCompletionCreateRes HumanChallengeProof
