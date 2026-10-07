package v1

import "github.com/gogf/gf/v2/frame/g"

type AIConfigGetReq struct {
	g.Meta `path:"/admin/ai/config" method:"get" tags:"IDC AI" summary:"Get AI configuration" permission:"idc:admin:ai:config:view"`
}
type AIConfigGetRes struct {
	Enabled            bool   `json:"enabled"`
	BaseURL            string `json:"baseUrl"`
	Model              string `json:"model"`
	KeyConfigured      bool   `json:"keyConfigured"`
	BudgetMinor        int64  `json:"budgetMinor"`
	RateLimitPerMinute int    `json:"rateLimitPerMinute"`
	RetentionDays      int    `json:"retentionDays"`
	Revision           int64  `json:"revision"`
}
type AIConfigUpdateReq struct {
	g.Meta             `path:"/admin/ai/config" method:"put" tags:"IDC AI" summary:"Update AI configuration" permission:"idc:admin:ai:config:manage"`
	BaseURL            string `json:"baseUrl" v:"required|url|max-length:512"`
	APIKey             string `json:"apiKey,omitempty" v:"max-length:4096"`
	Model              string `json:"model" v:"required|max-length:128"`
	Enabled            bool   `json:"enabled"`
	BudgetMinor        int64  `json:"budgetMinor" v:"min:0"`
	RateLimitPerMinute int    `json:"rateLimitPerMinute" v:"min:1|max:1000"`
	RetentionDays      int    `json:"retentionDays" v:"min:1|max:3650"`
}
type AIConfigUpdateRes struct{ AIConfigGetRes }
type AIConfigTestReq struct {
	g.Meta  `path:"/admin/ai/config/test" method:"post" tags:"IDC AI" summary:"Test AI configuration" permission:"idc:admin:ai:config:manage"`
	BaseURL string `json:"baseUrl" v:"required|url|max-length:512"`
	APIKey  string `json:"apiKey,omitempty" v:"max-length:4096"`
	Model   string `json:"model" v:"required|max-length:128"`
}
type AIConfigTestRes struct {
	Reachable bool   `json:"reachable"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}
type AIChatReq struct {
	g.Meta         `path:"/admin/ai/chat" method:"post" tags:"IDC AI" summary:"Ask the read-only AI copilot" permission:"idc:admin:ai:use"`
	Prompt         string `json:"prompt" v:"required|max-length:4000"`
	ConversationID int64  `json:"conversationId,omitempty"`
}
type AIChatRes struct {
	ConversationID int64    `json:"conversationId"`
	Answer         string   `json:"answer"`
	Evidence       []string `json:"evidence"`
	DataAsOf       string   `json:"dataAsOf"`
	Uncertainty    string   `json:"uncertainty"`
	Safety         string   `json:"safety"`
	Model          string   `json:"model,omitempty"`
}
type AIFeedbackReq struct {
	g.Meta         `path:"/admin/ai/feedback" method:"post" tags:"IDC AI" summary:"Send AI feedback" permission:"idc:admin:ai:use"`
	ConversationID int64  `json:"conversationId"`
	Rating         int    `json:"rating" v:"min:1|max:5"`
	Comment        string `json:"comment,omitempty" v:"max-length:1000"`
}
type AIFeedbackRes struct {
	Accepted bool `json:"accepted"`
}
type AIAuditListReq struct {
	g.Meta `path:"/admin/ai/audit" method:"get" tags:"IDC AI" summary:"List AI safety audit" permission:"idc:admin:ai:audit"`
	Limit  int `json:"limit" v:"max:100"`
}
type AIAuditListRes struct {
	Items []AIAuditItem `json:"items"`
}
type AIAuditItem struct {
	Tool      string `json:"tool"`
	Decision  string `json:"decision"`
	Reason    string `json:"reason,omitempty"`
	CreatedAt string `json:"createdAt"`
}
