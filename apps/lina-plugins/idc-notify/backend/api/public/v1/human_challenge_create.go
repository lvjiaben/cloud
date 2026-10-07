package v1

import "github.com/gogf/gf/v2/frame/g"

type HumanChallengeCreateReq struct {
	g.Meta           `path:"/public/support/presales/human-challenges" method:"post" tags:"IDC Public Presales" summary:"Create a presales human challenge" dc:"Creates one safe human-verification presentation bound to the destination, browser, operation, and optional ticket."`
	BrowserNonce     string `json:"browserNonce" v:"required|min-length:16|max-length:128" dc:"Random nonce generated and retained only by this browser" eg:"7bdb8be1-cbe4-4c32-8b83-e5122ea20a42"`
	Destination      string `json:"destination" v:"required|max-length:320" dc:"Email address or E.164 phone number used only for this operation" eg:"help@example.com"`
	DestinationKind  string `json:"destinationKind" v:"required|in:email,sms" dc:"Destination kind" eg:"email"`
	Locale           string `json:"locale" v:"required|in:zh-CN,en-US" dc:"Message language" eg:"zh-CN"`
	Operation        string `json:"operation" v:"required|in:create,renew" dc:"Anonymous ticket operation" eg:"create"`
	TicketNo         string `json:"ticketNo" v:"max-length:48" dc:"Ticket number, required only for access renewal" eg:"TKT_01JZ..."`
	PresentationKind string `json:"presentationKind" v:"required|in:accessible,image" dc:"Preferred accessible or image challenge presentation" eg:"accessible"`
}

type HumanChallengeCreateRes HumanChallengePresentation
