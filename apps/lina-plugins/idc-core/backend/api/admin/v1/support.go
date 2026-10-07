package v1

import "github.com/gogf/gf/v2/frame/g"

type TicketItem struct {
	TicketNo             string                    `json:"ticketNo"`
	Type                 string                    `json:"type"`
	Visibility           string                    `json:"visibility"`
	CustomerNo           string                    `json:"customerNo,omitempty"`
	CustomerDisplayLabel string                    `json:"customerDisplayLabel,omitempty"`
	CategoryNo           string                    `json:"categoryNo,omitempty"`
	CategoryNameZhCN     string                    `json:"categoryNameZhCN,omitempty"`
	CategoryNameEnUS     string                    `json:"categoryNameEnUS,omitempty"`
	DepartmentNo         string                    `json:"departmentNo,omitempty"`
	Queue                string                    `json:"queue"`
	Subject              string                    `json:"subject"`
	BaseStage            string                    `json:"baseStage"`
	CustomerStage        string                    `json:"customerStage"`
	CustomStatusNo       string                    `json:"customStatusNo,omitempty"`
	Priority             string                    `json:"priority"`
	ClosedReason         string                    `json:"closedReason,omitempty"`
	AssigneeUserID       int64                     `json:"assigneeUserId,omitempty"`
	CurrentRound         int                       `json:"currentRound"`
	FollowUp             bool                      `json:"followUp"`
	SLARisk              string                    `json:"slaRisk"`
	SLAConfigured        bool                      `json:"slaConfigured"`
	ResolvedAt           int64                     `json:"resolvedAt,omitempty"`
	ClosedAt             int64                     `json:"closedAt,omitempty"`
	LastActivityAt       int64                     `json:"lastActivityAt"`
	CreatedAt            int64                     `json:"createdAt"`
	UpdatedAt            int64                     `json:"updatedAt"`
	Version              int64                     `json:"version"`
	RelatedEntitlement   *TicketRelatedEntitlement `json:"relatedEntitlement,omitempty" dc:"Immutable entitlement link snapshot and current safe projection" eg:"ent_0123456789abcdef0123456789abcdef"`
}
type AttachmentItem struct {
	AttachmentNo string `json:"attachmentNo"`
	MessageNo    string `json:"messageNo,omitempty"`
	Visibility   string `json:"visibility"`
	FileName     string `json:"fileName"`
	MediaType    string `json:"mediaType"`
	SizeBytes    int64  `json:"sizeBytes"`
	CreatedAt    int64  `json:"createdAt"`
	Hidden       bool   `json:"hidden"`
}
type MessageItem struct {
	MessageNo    string           `json:"messageNo"`
	Type         string           `json:"type"`
	Visibility   string           `json:"visibility"`
	AuthorType   string           `json:"authorType"`
	Body         string           `json:"body"`
	AuthorUserID int64            `json:"authorUserId,omitempty"`
	CreatedAt    int64            `json:"createdAt"`
	Automated    bool             `json:"automated"`
	Hidden       bool             `json:"hidden"`
	Attachments  []AttachmentItem `json:"attachments"`
}
type TicketDetail struct {
	Ticket                     *TicketItem                    `json:"ticket"`
	PublicMessages             []MessageItem                  `json:"publicMessages"`
	InternalNotes              []MessageItem                  `json:"internalNotes"`
	Attachments                []AttachmentItem               `json:"attachments"`
	AssignmentHistory          []AssignmentItem               `json:"assignmentHistory"`
	Relations                  []RelationItem                 `json:"relations"`
	Escalations                []EscalationItem               `json:"escalations"`
	SLARounds                  []SLARoundItem                 `json:"slaRounds"`
	StatusHistory              []StatusHistoryItem            `json:"statusHistory"`
	AvailableActions           []string                       `json:"availableActions"`
	AvailableStatusTransitions []TicketStatusTransitionOption `json:"availableStatusTransitions"`
	CustomFields               []TicketCustomFieldValue       `json:"customFields" dc:"Immutable ticket custom field snapshots visible to the authorized employee" eg:"SCF_service_area"`
}

type AssignmentItem struct {
	HistoryNo        string `json:"historyNo"`
	Action           string `json:"action"`
	FromDepartmentNo string `json:"fromDepartmentNo,omitempty"`
	ToDepartmentNo   string `json:"toDepartmentNo,omitempty"`
	ReasonCode       string `json:"reasonCode,omitempty"`
	Reason           string `json:"reason,omitempty"`
	FromUserID       int64  `json:"fromUserId,omitempty"`
	ToUserID         int64  `json:"toUserId,omitempty"`
	ActorUserID      int64  `json:"actorUserId"`
	OccurredAt       int64  `json:"occurredAt"`
}
type RelationItem struct {
	RelationNo     string `json:"relationNo"`
	Type           string `json:"type"`
	SourceTicketNo string `json:"sourceTicketNo"`
	TargetTicketNo string `json:"targetTicketNo"`
	Reason         string `json:"reason"`
	ActorUserID    int64  `json:"actorUserId"`
	CreatedAt      int64  `json:"createdAt"`
}
type EscalationItem struct {
	EscalationNo string `json:"escalationNo"`
	Reason       string `json:"reason"`
	FromUserID   int64  `json:"fromUserId"`
	ToUserID     int64  `json:"toUserId"`
	CreatedAt    int64  `json:"createdAt"`
}
type SLARoundItem struct {
	RoundNo               int    `json:"round"`
	RuleNo                string `json:"ruleNo,omitempty"`
	CalendarNo            string `json:"calendarNo,omitempty"`
	CalendarMode          string `json:"calendarMode,omitempty"`
	FirstResponseDueAt    int64  `json:"firstResponseDueAt,omitempty"`
	ResolutionDueAt       int64  `json:"resolutionDueAt,omitempty"`
	FirstRespondedAt      int64  `json:"firstRespondedAt,omitempty"`
	ResolvedAt            int64  `json:"resolvedAt,omitempty"`
	StoppedAt             int64  `json:"stoppedAt,omitempty"`
	PausedSeconds         int64  `json:"pausedSeconds"`
	FirstResponseBreached bool   `json:"firstResponseBreached"`
	ResolutionBreached    bool   `json:"resolutionBreached"`
}
type StatusHistoryItem struct {
	HistoryNo      string `json:"historyNo"`
	FromStage      string `json:"fromStage,omitempty"`
	ToStage        string `json:"toStage"`
	CustomStatusNo string `json:"customStatusNo,omitempty"`
	Source         string `json:"source"`
	ReasonCode     string `json:"reasonCode,omitempty"`
	Reason         string `json:"reason,omitempty"`
	ActorUserID    int64  `json:"actorUserId"`
	RuleVersion    int64  `json:"ruleVersion,omitempty"`
	OccurredAt     int64  `json:"occurredAt"`
}

type TicketListReq struct {
	g.Meta         `path:"/admin/support/tickets" method:"get" tags:"IDC Support" summary:"List support tickets" permission:"idc:admin:support:ticket:list"`
	Stage          string `json:"stage"`
	Status         string `json:"status"`
	Priority       string `json:"priority"`
	Type           string `json:"type"`
	DepartmentNo   string `json:"departmentNo"`
	Query          string `json:"query"`
	Cursor         string `json:"cursor"`
	AssigneeUserID int64  `json:"assigneeUserId"`
	FollowUp       *bool  `json:"followUp"`
	Limit          int    `json:"limit" d:"25" v:"min:1|max:100"`
}
type TicketListRes struct {
	Tickets     []*TicketItem `json:"tickets"`
	NextCursor  string        `json:"nextCursor"`
	PageActions []string      `json:"pageActions"`
}
type TicketGlobalListReq struct {
	g.Meta         `path:"/admin/support/all-tickets" method:"get" tags:"IDC Support" summary:"List all support tickets in the evaluated data scope" permission:"idc:admin:support:ticket:view-all"`
	Stage          string `json:"stage"`
	Status         string `json:"status"`
	Priority       string `json:"priority"`
	Type           string `json:"type"`
	DepartmentNo   string `json:"departmentNo"`
	Query          string `json:"query"`
	Cursor         string `json:"cursor"`
	AssigneeUserID int64  `json:"assigneeUserId"`
	FollowUp       *bool  `json:"followUp"`
	Limit          int    `json:"limit" d:"25" v:"min:1|max:100"`
}
type TicketGlobalListRes = TicketListRes
type TicketDetailReq struct {
	g.Meta   `path:"/admin/support/tickets/{ticketNo}" method:"get" tags:"IDC Support" summary:"Get support ticket" permission:"idc:admin:support:ticket:query"`
	TicketNo string `json:"ticketNo"`
}
type TicketDetailRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketCreateReq struct {
	g.Meta       `path:"/admin/support/tickets" method:"post" tags:"IDC Support" summary:"Create internal support ticket" permission:"idc:admin:support:ticket:create"`
	DepartmentNo string                        `json:"departmentNo"`
	CategoryNo   string                        `json:"categoryNo"`
	Subject      string                        `json:"subject"`
	Body         string                        `json:"body"`
	Priority     string                        `json:"priority"`
	UploadNos    []string                      `json:"uploadNos"`
	CustomFields []TicketCustomFieldSubmission `json:"customFields" dc:"Validated internal-only custom field submissions" eg:"SCF_internal_reason"`
}
type TicketCreateRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketMessageReq struct {
	TicketNo      string   `json:"ticketNo"`
	Body          string   `json:"body"`
	UploadNos     []string `json:"uploadNos"`
	TicketVersion int64    `json:"ticketVersion"`
}
type TicketPublicReplyReq struct {
	g.Meta `path:"/admin/support/tickets/{ticketNo}/public-replies" method:"post" tags:"IDC Support" summary:"Reply to support ticket" permission:"idc:admin:support:ticket:reply"`
	TicketMessageReq
}
type TicketPublicReplyRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketInternalNoteReq struct {
	g.Meta `path:"/admin/support/tickets/{ticketNo}/internal-notes" method:"post" tags:"IDC Support" summary:"Add immutable internal note" permission:"idc:admin:support:ticket:note"`
	TicketMessageReq
}
type TicketInternalNoteRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketClaimReq struct {
	g.Meta        `path:"/admin/support/tickets/{ticketNo}/claims" method:"post" tags:"IDC Support" summary:"Claim support ticket" permission:"idc:admin:support:ticket:claim"`
	TicketNo      string `json:"ticketNo"`
	TicketVersion int64  `json:"ticketVersion"`
}
type TicketClaimRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketTransferReq struct {
	g.Meta         `path:"/admin/support/tickets/{ticketNo}/transfers" method:"post" tags:"IDC Support" summary:"Transfer support ticket" permission:"idc:admin:support:ticket:transfer"`
	TicketNo       string `json:"ticketNo"`
	DepartmentNo   string `json:"departmentNo"`
	Reason         string `json:"reason"`
	AssigneeUserID int64  `json:"assigneeUserId"`
	TicketVersion  int64  `json:"ticketVersion"`
}
type TicketTransferRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketTransitionReq struct {
	TicketNo      string `json:"ticketNo"`
	Reason        string `json:"reason"`
	TicketVersion int64  `json:"ticketVersion"`
}
type TicketResolveReq struct {
	g.Meta `path:"/admin/support/tickets/{ticketNo}/resolutions" method:"post" tags:"IDC Support" summary:"Resolve support ticket" permission:"idc:admin:support:ticket:resolve"`
	TicketTransitionReq
}
type TicketResolveRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketCloseReq struct {
	g.Meta `path:"/admin/support/tickets/{ticketNo}/closures" method:"post" tags:"IDC Support" summary:"Force close support ticket" permission:"idc:admin:support:ticket:close"`
	TicketTransitionReq
}
type TicketCloseRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketReopenReq struct {
	g.Meta `path:"/admin/support/tickets/{ticketNo}/reopenings" method:"post" tags:"IDC Support" summary:"Reopen support ticket" permission:"idc:admin:support:ticket:reopen"`
	TicketTransitionReq
}
type TicketReopenRes struct {
	Item *TicketDetail `json:"item"`
}
type VersionedTicketReq struct {
	TicketNo string `json:"ticketNo"`
	Version  int64  `json:"version"`
}
type TicketMergeReq struct {
	g.Meta      `path:"/admin/support/tickets/{ticketNo}/merges" method:"post" tags:"IDC Support" summary:"Merge support tickets" permission:"idc:admin:support:ticket:merge"`
	TicketNo    string               `json:"ticketNo"`
	MainVersion int64                `json:"mainVersion"`
	Sources     []VersionedTicketReq `json:"sources"`
	Reason      string               `json:"reason"`
}
type TicketMergeRes struct {
	Item *TicketDetail `json:"item"`
}
type MergeCandidateItem struct {
	TicketNo string `json:"ticketNo"`
	Subject  string `json:"subject"`
	Stage    string `json:"stage"`
	Version  int64  `json:"version"`
}
type TicketMergeCandidateListReq struct {
	g.Meta   `path:"/admin/support/tickets/{ticketNo}/merge-candidates" method:"get" tags:"IDC Support" summary:"List visible support ticket merge candidates" permission:"idc:admin:support:ticket:merge"`
	TicketNo string `json:"ticketNo"`
	Cursor   string `json:"cursor"`
	Limit    int    `json:"limit" d:"25" v:"min:1|max:100"`
}
type TicketMergeCandidateListRes struct {
	Items      []MergeCandidateItem `json:"items"`
	NextCursor string               `json:"nextCursor"`
}
type TicketMergeAllCandidateListReq struct {
	g.Meta   `path:"/admin/support/tickets/{ticketNo}/global-merge-candidates" method:"get" tags:"IDC Support" summary:"List merge candidates across the authorized global scope" permission:"idc:admin:support:ticket:merge-all"`
	TicketNo string `json:"ticketNo"`
	Cursor   string `json:"cursor"`
	Limit    int    `json:"limit" d:"25" v:"min:1|max:100"`
}
type TicketMergeAllCandidateListRes = TicketMergeCandidateListRes
type TicketMergeAllReq struct {
	g.Meta      `path:"/admin/support/tickets/{ticketNo}/global-merges" method:"post" tags:"IDC Support" summary:"Merge support tickets across authorized queues" permission:"idc:admin:support:ticket:merge-all"`
	TicketNo    string               `json:"ticketNo"`
	MainVersion int64                `json:"mainVersion"`
	Sources     []VersionedTicketReq `json:"sources"`
	Reason      string               `json:"reason"`
}
type TicketMergeAllRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketSplitReq struct {
	g.Meta        `path:"/admin/support/tickets/{ticketNo}/splits" method:"post" tags:"IDC Support" summary:"Split selected public content into a new ticket" permission:"idc:admin:support:ticket:split"`
	TicketNo      string   `json:"ticketNo"`
	CategoryNo    string   `json:"categoryNo"`
	Reason        string   `json:"reason"`
	TicketVersion int64    `json:"ticketVersion"`
	MessageNos    []string `json:"messageNos"`
	AttachmentNos []string `json:"attachmentNos"`
}
type TicketSplitRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketEscalateReq struct {
	g.Meta        `path:"/admin/support/tickets/{ticketNo}/escalations" method:"post" tags:"IDC Support" summary:"Escalate a support ticket" permission:"idc:admin:support:ticket:escalate"`
	TicketNo      string `json:"ticketNo"`
	Reason        string `json:"reason"`
	ToUserID      int64  `json:"toUserId"`
	TicketVersion int64  `json:"ticketVersion"`
}
type TicketEscalateRes struct {
	Item *TicketDetail `json:"item"`
}
type TicketHideReq struct {
	g.Meta        `path:"/admin/support/tickets/{ticketNo}/hidden-content" method:"post" tags:"IDC Support" summary:"Hide immutable ticket content" permission:"idc:admin:support:ticket:hide"`
	TicketNo      string `json:"ticketNo"`
	TargetType    string `json:"targetType"`
	TargetNo      string `json:"targetNo"`
	Reason        string `json:"reason"`
	TicketVersion int64  `json:"ticketVersion"`
}
type TicketHideRes struct {
	Item *TicketDetail `json:"item"`
}
type AttachmentUploadCreateReq struct {
	g.Meta    `path:"/admin/support/tickets/{ticketNo}/attachment-uploads" method:"post" tags:"IDC Support" summary:"Reserve a ticket attachment" permission:"idc:admin:support:ticket:reply"`
	TicketNo  string `json:"ticketNo"`
	Context   string `json:"context"`
	FileName  string `json:"fileName"`
	SizeBytes int64  `json:"sizeBytes"`
}
type InternalAttachmentUploadCreateReq struct {
	g.Meta    `path:"/admin/support/tickets/{ticketNo}/internal-note-attachment-uploads" method:"post" tags:"IDC Support" summary:"Reserve an internal-note ticket attachment" permission:"idc:admin:support:ticket:note"`
	TicketNo  string `json:"ticketNo"`
	FileName  string `json:"fileName"`
	SizeBytes int64  `json:"sizeBytes"`
}
type InternalAttachmentUploadCreateRes = AttachmentUploadCreateRes
type InternalTicketAttachmentUploadCreateReq struct {
	g.Meta    `path:"/admin/support/internal-ticket-attachment-uploads" method:"post" tags:"IDC Support" summary:"Reserve an attachment for a new internal ticket" permission:"idc:admin:support:ticket:create"`
	FileName  string `json:"fileName"`
	SizeBytes int64  `json:"sizeBytes"`
}
type InternalTicketAttachmentUploadCreateRes = AttachmentUploadCreateRes
type AttachmentUploadCreateRes struct {
	UploadNo   string `json:"uploadNo"`
	NextOffset int64  `json:"nextOffset"`
	ExpiresAt  int64  `json:"expiresAt"`
	Version    int64  `json:"version"`
}
type AttachmentUploadContentUpdateReq struct {
	g.Meta   `path:"/admin/support/ticket-attachment-uploads/{uploadNo}/content" method:"put" mime:"application/octet-stream" tags:"IDC Support" summary:"Upload reserved ticket attachment" permission:"idc:admin:support:ticket:reply"`
	UploadNo string `json:"uploadNo"`
}
type InternalAttachmentUploadContentUpdateReq struct {
	g.Meta   `path:"/admin/support/internal-note-attachment-uploads/{uploadNo}/content" method:"put" mime:"application/octet-stream" tags:"IDC Support" summary:"Upload reserved internal-note ticket attachment" permission:"idc:admin:support:ticket:note"`
	UploadNo string `json:"uploadNo"`
}
type InternalAttachmentUploadContentUpdateRes = AttachmentUploadContentUpdateRes
type InternalTicketAttachmentUploadContentUpdateReq struct {
	g.Meta   `path:"/admin/support/internal-ticket-attachment-uploads/{uploadNo}/content" method:"put" mime:"application/octet-stream" tags:"IDC Support" summary:"Upload an attachment reserved for a new internal ticket" permission:"idc:admin:support:ticket:create"`
	UploadNo string `json:"uploadNo"`
}
type InternalTicketAttachmentUploadContentUpdateRes = AttachmentUploadContentUpdateRes
type AttachmentUploadContentUpdateRes struct {
	UploadNo  string `json:"uploadNo"`
	FileName  string `json:"fileName"`
	MediaType string `json:"mediaType"`
	SizeBytes int64  `json:"sizeBytes"`
	Version   int64  `json:"version"`
	Ready     bool   `json:"ready"`
}
type AttachmentOpenReq struct {
	g.Meta       `path:"/admin/support/attachments/{attachmentNo}/content" method:"get" tags:"IDC Support" summary:"Download a currently visible ticket attachment" permission:"idc:admin:support:ticket:query"`
	AttachmentNo string `json:"attachmentNo"`
}
type AttachmentOpenRes struct{}

type DepartmentItem struct {
	DepartmentNo       string `json:"departmentNo"`
	NameZhCN           string `json:"nameZhCN"`
	NameEnUS           string `json:"nameEnUS"`
	ParentDepartmentNo string `json:"parentDepartmentNo,omitempty"`
	AssignmentMode     string `json:"assignmentMode"`
	Status             string `json:"status"`
	Version            int64  `json:"version"`
}
type DepartmentListReq struct {
	g.Meta `path:"/admin/support/departments" method:"get" tags:"IDC Support" summary:"List visible support departments" permission:"idc:admin:support:ticket:list"`
}
type DepartmentListRes struct {
	Departments []DepartmentItem `json:"departments"`
}
type IssueCategoryItem struct {
	CategoryNo string `json:"categoryNo"`
	NameZhCN   string `json:"nameZhCN"`
	NameEnUS   string `json:"nameEnUS"`
	Priority   string `json:"priority"`
	SortOrder  int    `json:"sortOrder"`
}
type IssueCategoryListReq struct {
	g.Meta `path:"/admin/support/issue-categories" method:"get" tags:"IDC Support" summary:"List support issue categories" permission:"idc:admin:support:ticket:list"`
}
type IssueCategoryListRes struct {
	IssueCategories []IssueCategoryItem `json:"issueCategories"`
}
type StatusItem struct {
	StatusNo  string `json:"statusNo"`
	NameZhCN  string `json:"nameZhCN"`
	NameEnUS  string `json:"nameEnUS"`
	BaseStage string `json:"baseStage"`
	Status    string `json:"status"`
	Version   int64  `json:"version"`
}
type StatusListReq struct {
	g.Meta `path:"/admin/support/statuses" method:"get" tags:"IDC Support" summary:"List support custom statuses" permission:"idc:admin:support:ticket:list"`
}
type StatusListRes struct {
	Statuses []StatusItem `json:"statuses"`
}
type ReplyPresetItem struct {
	PresetNo     string `json:"presetNo"`
	DepartmentNo string `json:"departmentNo"`
	Locale       string `json:"locale"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	Status       string `json:"status"`
	Version      int64  `json:"version"`
}
type ReplyPresetListReq struct {
	g.Meta `path:"/admin/support/reply-presets" method:"get" tags:"IDC Support" summary:"List support reply presets" permission:"idc:admin:support:ticket:reply"`
	Locale string `json:"locale" d:"zh-CN"`
}
type ReplyPresetListRes struct {
	ReplyPresets []ReplyPresetItem `json:"replyPresets"`
}
type AutomationPolicyItem struct {
	AutoCloseDays int   `json:"autoCloseDays"`
	ReopenDays    int   `json:"reopenDays"`
	Version       int64 `json:"version"`
}
type AutomationPolicyGetReq struct {
	g.Meta `path:"/admin/support/settings/automation-policy" method:"get" tags:"IDC Support" summary:"Get support automation policy" permission:"idc:admin:support:configuration:manage"`
}
type AutomationPolicyGetRes struct {
	Policy *AutomationPolicyItem `json:"policy"`
}
type AutomationPolicyUpdateReq struct {
	g.Meta          `path:"/admin/support/settings/automation-policy" method:"put" tags:"IDC Support" summary:"Update support automation policy" permission:"idc:admin:support:configuration:manage"`
	AutoCloseDays   int   `json:"autoCloseDays"`
	ReopenDays      int   `json:"reopenDays"`
	ExpectedVersion int64 `json:"expectedVersion"`
}
type AutomationPolicyUpdateRes struct {
	Policy *AutomationPolicyItem `json:"policy"`
}
