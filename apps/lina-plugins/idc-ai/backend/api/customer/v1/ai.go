package v1

import "github.com/gogf/gf/v2/frame/g"

type AssistantStatusReq struct {
	g.Meta `path:"/customer/ai/status" method:"get" tags:"IDC AI" summary:"Get customer assistant status"`
}
type AssistantStatusRes struct {
	Enabled bool `json:"enabled"`
}
type AssistantChatReq struct {
	g.Meta `path:"/customer/ai/chat" method:"post" tags:"IDC AI" summary:"Ask the customer read-only assistant"`
	Prompt string `json:"prompt" v:"required|max-length:2000"`
}
type AssistantChatRes struct {
	Answer      string `json:"answer"`
	Navigation  string `json:"navigation"`
	DataAsOf    string `json:"dataAsOf"`
	Uncertainty string `json:"uncertainty"`
}
