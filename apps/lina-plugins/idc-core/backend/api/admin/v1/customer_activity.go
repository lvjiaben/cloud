package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerNoteCreateReq struct {
	g.Meta         `path:"/admin/customers/{customerNo}/notes" method:"post" tags:"IDC Customer Management" summary:"Add customer note" dc:"Append an internal customer note; a correction creates a new note and withdraws the superseded note without erasing history." permission:"idc:admin:customer:update"`
	CustomerNo     string `json:"customerNo" v:"required" dc:"Stable external customer number"`
	Category       string `json:"category" v:"required|in:general,billing,service,sales,risk,compliance" dc:"Note category" eg:"general"`
	Content        string `json:"content" v:"required|max-length:8192" dc:"Internal note content"`
	SupersedesNo   string `json:"supersedesNo" dc:"Active note number corrected by this note, or empty"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:12|max-length:128"`
}

type CustomerNoteCreateRes struct {
	NoteNo       string `json:"noteNo"`
	CustomerNo   string `json:"customerNo"`
	Category     string `json:"category"`
	Status       string `json:"status"`
	SupersedesNo string `json:"supersedesNo"`
	CreatedAt    int64  `json:"createdAt" dc:"Database creation time in Unix milliseconds"`
}

type CustomerFollowUpPlanReq struct {
	g.Meta         `path:"/admin/customers/{customerNo}/follow-ups" method:"post" tags:"IDC Customer Management" summary:"Plan customer follow-up" permission:"idc:admin:customer:update"`
	CustomerNo     string `json:"customerNo" v:"required"`
	Channel        string `json:"channel" v:"required|in:phone,email,sms,wechat,meeting,ticket,other"`
	Subject        string `json:"subject" v:"required|max-length:512"`
	Detail         string `json:"detail" v:"max-length:8192"`
	AssignedUserID int    `json:"assignedUserId" v:"min:0" dc:"Responsible staff user ID; zero means unassigned"`
	PlannedAt      int64  `json:"plannedAt" v:"required|min:1" dc:"Planned time in Unix milliseconds"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:12|max-length:128"`
}

type CustomerFollowUpPlanRes struct {
	FollowUpNo string `json:"followUpNo"`
	CustomerNo string `json:"customerNo"`
	Status     string `json:"status"`
	PlannedAt  int64  `json:"plannedAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}

type CustomerFollowUpTransitionReq struct {
	g.Meta         `path:"/admin/customers/{customerNo}/follow-ups/{followUpNo}/transition" method:"post" tags:"IDC Customer Management" summary:"Complete or cancel customer follow-up" permission:"idc:admin:customer:update"`
	CustomerNo     string `json:"customerNo" v:"required"`
	FollowUpNo     string `json:"followUpNo" v:"required"`
	Action         string `json:"action" v:"required|in:complete,cancel"`
	Outcome        string `json:"outcome" v:"max-length:32" dc:"Stable optional outcome code"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:12|max-length:128"`
}

type CustomerFollowUpTransitionRes = CustomerFollowUpPlanRes

type CustomerBlacklistCreateReq struct {
	g.Meta            `path:"/admin/customers/{customerNo}/blacklist" method:"post" tags:"IDC Customer Management" summary:"Propose customer blacklist restriction" dc:"Create a pending restriction that has no effect until a separate approval action." permission:"idc:admin:customer:update"`
	CustomerNo        string `json:"customerNo" v:"required"`
	Scope             string `json:"scope" v:"required|in:registration,purchase,refund,resource_operation,all_business"`
	ReasonCode        string `json:"reasonCode" v:"required|max-length:64"`
	ReasonDetail      string `json:"reasonDetail" v:"max-length:2048"`
	EvidenceReference string `json:"evidenceReference" v:"max-length:256"`
	RiskLevel         string `json:"riskLevel" v:"required|in:low,medium,high,critical"`
	StartsAt          int64  `json:"startsAt" v:"min:0" dc:"Start time in Unix milliseconds; zero uses database time"`
	ExpiresAt         int64  `json:"expiresAt" v:"min:0" dc:"Optional expiry time in Unix milliseconds"`
	IdempotencyKey    string `json:"idempotencyKey" v:"required|min-length:12|max-length:128"`
}

type CustomerBlacklistCreateRes struct {
	EntryNo    string `json:"entryNo"`
	CustomerNo string `json:"customerNo"`
	Status     string `json:"status"`
	UpdatedAt  int64  `json:"updatedAt"`
}

type CustomerBlacklistDecisionReq struct {
	g.Meta         `path:"/admin/customers/{customerNo}/blacklist/{entryNo}/decision" method:"post" tags:"IDC Customer Management" summary:"Approve, reject, or revoke customer blacklist restriction" permission:"idc:admin:customer:update"`
	CustomerNo     string `json:"customerNo" v:"required"`
	EntryNo        string `json:"entryNo" v:"required"`
	Action         string `json:"action" v:"required|in:approve,reject,revoke"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:12|max-length:128"`
}

type CustomerBlacklistDecisionRes = CustomerBlacklistCreateRes
