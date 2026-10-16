// Package supportdesk owns the Phase 11 ticket lifecycle and its access boundary.
package supportdesk

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/orgcap"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/supportupstream"
)

const (
	StageNew             = "new"
	StageInProgress      = "in_progress"
	StageWaitingCustomer = "waiting_customer"
	StageWaitingUpstream = "waiting_upstream"
	StageResolved        = "resolved"
	StageClosed          = "closed"

	PermissionList          = "idc:admin:support:ticket:list"
	PermissionQuery         = "idc:admin:support:ticket:query"
	PermissionCreate        = "idc:admin:support:ticket:create"
	PermissionReply         = "idc:admin:support:ticket:reply"
	PermissionNote          = "idc:admin:support:ticket:note"
	PermissionClaim         = "idc:admin:support:ticket:claim"
	PermissionTransfer      = "idc:admin:support:ticket:transfer"
	PermissionResolve       = "idc:admin:support:ticket:resolve"
	PermissionClose         = "idc:admin:support:ticket:close"
	PermissionReopen        = "idc:admin:support:ticket:reopen"
	PermissionMerge         = "idc:admin:support:ticket:merge"
	PermissionSplit         = "idc:admin:support:ticket:split"
	PermissionEscalate      = "idc:admin:support:ticket:escalate"
	PermissionHide          = "idc:admin:support:ticket:hide"
	PermissionUpstream      = "idc:admin:support:ticket:upstream-forward"
	PermissionLead          = "idc:admin:support:ticket:lead"
	PermissionViewAll       = "idc:admin:support:ticket:view-all"
	PermissionMergeAll      = "idc:admin:support:ticket:merge-all"
	PermissionConfigure     = "idc:admin:support:configuration:manage"
	ActionStatusTransition  = "status_transition"
	maxTicketBodyBytes      = 65536
	maxCustomFieldCount     = 100
	maxCustomOptionCount    = 100
	maxCustomFieldSettings  = 200
	maxAttachmentBytes      = int64(20 * 1024 * 1024)
	maxTicketUploadBytes    = int64(200 * 1024 * 1024)
	anonymousAccessLifetime = 2 * time.Hour
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_SUPPORT_TICKET_INVALID", "The support ticket request is invalid", gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_SUPPORT_TICKET_NOT_ACCESSIBLE", "The support ticket is not accessible", gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_SUPPORT_TICKET_CONFLICT", "The support ticket changed; reload it and try again", gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_SUPPORT_TICKET_UNAVAILABLE", "The support ticket service is temporarily unavailable", gcode.CodeInternalError,
	)
	CodeAttachmentRejected = bizerr.MustDefine(
		"IDC_SUPPORT_ATTACHMENT_REJECTED", "The attachment type or content is not allowed", gcode.CodeInvalidParameter,
	)
	CodeAnonymousLinkUnconfigured = bizerr.MustDefine(
		"IDC_SUPPORT_ANONYMOUS_LINK_UNCONFIGURED", "Anonymous ticket access is not configured", gcode.CodeInvalidOperation,
	)
)

type Ticket struct {
	ID                          int64
	TicketNo                    string
	TicketType                  string
	Visibility                  string
	CustomerNo                  string
	CustomerDisplayLabel        string
	RelatedEntitlementNo        string
	RelatedEntitlementVersion   int64
	RelatedEntitlementNameZhCN  string
	RelatedEntitlementNameEnUS  string
	RelatedEntitlementStatus    string
	CurrentEntitlementNameZhCN  string
	CurrentEntitlementNameEnUS  string
	CurrentEntitlementStatus    string
	CurrentEntitlementVersion   int64
	CurrentEntitlementAvailable bool
	EntitlementAssociationRef   string
	AnonymousDestinationDigest  string
	AnonymousDestinationKind    string
	CategoryNo                  string
	CategoryNameZhCN            string
	CategoryNameEnUS            string
	DepartmentNo                string
	AssigneeUserID              int64
	Subject                     string
	BaseStage                   string
	CustomerStage               string
	CustomStatusNo              string
	Priority                    string
	CurrentRound                int
	FollowUpRequired            bool
	SLARisk                     string
	SLAConfigured               bool
	ResolvedAt                  *time.Time
	ClosedAt                    *time.Time
	ClosedReason                string
	LastActivityAt              time.Time
	Version                     int64
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
	AnonymousAccessExpiresAt    *time.Time
}

type Message struct {
	MessageNo    string
	Type         string
	Visibility   string
	AuthorType   string
	AuthorUserID int64
	Body         string
	Automated    bool
	Hidden       bool
	CreatedAt    time.Time
	Attachments  []Attachment
}

type Attachment struct {
	AttachmentNo string
	MessageNo    string
	Visibility   string
	FileName     string
	MediaType    string
	SizeBytes    int64
	Hidden       bool
	CreatedAt    time.Time
}

type Detail struct {
	Ticket                     *Ticket
	Messages                   []Message
	PublicMessages             []Message
	InternalNotes              []Message
	Attachments                []Attachment
	AssignmentHistory          []AssignmentHistory
	Relations                  []Relation
	Escalations                []Escalation
	SLARounds                  []SLARound
	StatusHistory              []StatusHistory
	AvailableActions           []string
	AvailableStatusTransitions []StatusTransitionOption
	ReopenDeadline             *time.Time
	Rating                     *RatingProjection
	CustomFields               []CustomFieldValue
	AnonymousAccessExpiresAt   *time.Time
}

// CustomFieldOption is one current or frozen bilingual selection option.
type CustomFieldOption struct {
	OptionNo, NameZhCN, NameEnUS string
	SortOrder                    int
	Status                       SettingStatus
	Version                      int64
}

// CustomFieldDefinition is one versioned field configuration projected without submitted values.
type CustomFieldDefinition struct {
	FieldNo, FieldType, NameZhCN, NameEnUS, DescriptionZhCN, DescriptionEnUS string
	AppliesPresales, AppliesCustomer, AppliesInternal                        bool
	Visibility                                                               string
	Required                                                                 bool
	SortOrder                                                                int
	Status                                                                   SettingStatus
	Version                                                                  int64
	Options                                                                  []CustomFieldOption
}

// CustomFieldOptionInput carries one complete option configuration member and its optimistic version.
type CustomFieldOptionInput struct {
	OptionNo, NameZhCN, NameEnUS string
	SortOrder                    int
	Status                       SettingStatus
	ExpectedVersion              int64
}

// CustomFieldSettingInput carries the complete bounded configuration of one custom field.
type CustomFieldSettingInput struct {
	FieldType, NameZhCN, NameEnUS, DescriptionZhCN, DescriptionEnUS string
	AppliesPresales, AppliesCustomer, AppliesInternal               bool
	Visibility                                                      string
	Required                                                        bool
	SortOrder                                                       int
	Status                                                          SettingStatus
	ExpectedVersion                                                 int64
	Options                                                         []CustomFieldOptionInput
}

// CustomFieldSubmission carries exactly one typed value member selected by FieldNo.
type CustomFieldSubmission struct {
	FieldNo      string
	TextValue    *string
	NumberValue  *string
	DateValue    *string
	BooleanValue *bool
	OptionNos    []string
}

// CustomFieldValue is the immutable submitted definition, value, and selected-option snapshot.
type CustomFieldValue struct {
	FieldNo, FieldType, NameZhCN, NameEnUS, DescriptionZhCN, DescriptionEnUS string
	Visibility                                                               string
	Required                                                                 bool
	SortOrder                                                                int
	DefinitionVersion                                                        int64
	NormalizedValue                                                          string
	SelectedOptions                                                          []CustomFieldOption
}

// EntitlementCandidate is a safe current long-term entitlement projection owned by the current customer.
type EntitlementCandidate struct {
	EntitlementNo, AssociationReference, NameZhCN, NameEnUS, Status string
	Version                                                         int64
}

// EntitlementCandidatePage is one bounded cursor page of safe entitlement candidates.
type EntitlementCandidatePage struct {
	Items      []EntitlementCandidate
	NextCursor string
}

type AssignmentHistory struct {
	HistoryNo, Action, FromDepartmentNo, ToDepartmentNo, ReasonCode, Reason string
	FromUserID, ToUserID, ActorUserID                                       int64
	OccurredAt                                                              time.Time
}

type Relation struct {
	RelationNo, Type, SourceTicketNo, TargetTicketNo, Reason string
	ActorUserID                                              int64
	CreatedAt                                                time.Time
}

type Escalation struct {
	EscalationNo         string
	FromUserID, ToUserID int64
	Reason               string
	CreatedAt            time.Time
}

type SLARound struct {
	RoundNo                                                                      int
	RuleNo, CalendarNo, CalendarMode                                             string
	FirstResponseDueAt, ResolutionDueAt, FirstRespondedAt, ResolvedAt, StoppedAt *time.Time
	PausedSeconds                                                                int64
	FirstResponseBreached, ResolutionBreached                                    bool
}

type StatusHistory struct {
	HistoryNo, FromStage, ToStage, CustomStatusNo, Source, ReasonCode, Reason string
	ActorUserID                                                               int64
	RuleVersion                                                               int64
	OccurredAt                                                                time.Time
}

type RatingProjection struct {
	Stars       int
	RoundNo     int
	Feedback    string
	SubmittedAt time.Time
}

type Page struct {
	Tickets    []*Ticket
	NextCursor string
	Actions    []string
	// Total counts the same visible, filtered collection before pagination.
	Total int64
}

type SearchInput struct {
	Stage, Status, Priority, Type, DepartmentNo string
	AssigneeUserID                              int64
	FollowUp                                    *bool
	Query, Cursor                               string
	Limit                                       int
	// Page selects one-based numbered pagination; zero retains cursor queries.
	Page int
}

type CategoryOption struct {
	ID                 int64
	CategoryNo         string
	NameZhCN, NameEnUS string
	Priority           string
	SortOrder          int
}

type CategoryPage struct {
	Items      []CategoryOption
	NextCursor string
}

type CategorySearchInput struct {
	Keyword, Cursor string
	Limit           int
}

type DepartmentOption struct {
	OrgDeptID                                                int `orm:"org_dept_id"`
	OrgDeptName                                              string
	DepartmentNo, NameZhCN, NameEnUS, AssignmentMode, Status string
	ParentDepartmentNo                                       string
	Version                                                  int64
}

type StatusOption struct {
	StatusNo, NameZhCN, NameEnUS, BaseStage, Status string
	Version                                         int64
}

// StatusTransitionOption is one active custom status the current employee can
// apply to the current ticket without bypassing the mapped base-stage action.
type StatusTransitionOption struct {
	StatusNo, NameZhCN, NameEnUS, BaseStage string
	Version                                 int64
	RequiresReason                          bool
}

type ReplyPresetOption struct {
	PresetNo, DepartmentNo, Locale, Title, Body, Status string
	Version                                             int64
}

type AutomationPolicy struct {
	AutoCloseDays, ReopenDays int
	Version                   int64
}

type AutomationPolicyInput struct {
	AutoCloseDays, ReopenDays int
	ExpectedVersion           int64
}

type SettingStatus string

const (
	SettingStatusActive   SettingStatus = "active"
	SettingStatusDisabled SettingStatus = "disabled"
)

type AssignmentMode string

const (
	AssignmentModeManualClaim     AssignmentMode = "manual_claim"
	AssignmentModeRoundRobin      AssignmentMode = "round_robin"
	AssignmentModeLeastUnresolved AssignmentMode = "least_unresolved"
)

type DepartmentSettingInput struct {
	OrgDeptID                              int
	NameZhCN, NameEnUS, ParentDepartmentNo string
	AssignmentMode                         AssignmentMode
	Status                                 SettingStatus
	ExpectedVersion                        int64
}

type IssueCategorySetting struct {
	CategoryNo, NameZhCN, NameEnUS, DepartmentNo, Priority string
	SortOrder                                              int
	Status                                                 SettingStatus
	Version                                                int64
}

type IssueCategorySettingInput struct {
	NameZhCN, NameEnUS, DepartmentNo, Priority string
	SortOrder                                  int
	Status                                     SettingStatus
	ExpectedVersion                            int64
}

type CustomStatusSetting struct {
	StatusNo, NameZhCN, NameEnUS, BaseStage string
	SortOrder                               int
	Status                                  SettingStatus
	Version                                 int64
}

type CustomStatusSettingInput struct {
	NameZhCN, NameEnUS, BaseStage string
	SortOrder                     int
	Status                        SettingStatus
	ExpectedVersion               int64
}

type ReplyPresetSetting struct {
	PresetNo, DepartmentNo, Locale, Title, Body string
	SortOrder                                   int
	Status                                      SettingStatus
	Version                                     int64
}

type ReplyPresetSettingInput struct {
	DepartmentNo, Locale, Title, Body string
	SortOrder                         int
	Status                            SettingStatus
	ExpectedVersion                   int64
}

type SLACalendarSetting struct {
	CalendarNo, Name, Mode, Timezone string
	Workdays                         []int
	WorkStartMinute, WorkEndMinute   *int
	Status                           SettingStatus
	Version                          int64
}

type SLACalendarSettingInput struct {
	Name, Mode, Timezone           string
	Workdays                       []int
	WorkStartMinute, WorkEndMinute *int
	Status                         SettingStatus
	ExpectedVersion                int64
}

type SLAHolidaySetting struct {
	CalendarNo, Date, Name string
}

type SLAHolidaySettingInput struct {
	Date, Name string
}

type SLARuleSetting struct {
	RuleNo, DepartmentNo, CategoryNo, Priority, CalendarNo string
	FirstResponseMinutes, ResolutionMinutes                *int
	NearDueMinutes                                         []int
	Status                                                 SettingStatus
	Version                                                int64
}

type SLARuleSettingInput struct {
	DepartmentNo, CategoryNo, Priority, CalendarNo string
	FirstResponseMinutes, ResolutionMinutes        *int
	NearDueMinutes                                 []int
	Status                                         SettingStatus
	ExpectedVersion                                int64
}

type CreateInput struct {
	TicketType, CustomerNo, CategoryNo, DepartmentNo, Subject, Body, Priority string
	EntitlementNo, AssociationReference                                       string
	CustomFields                                                              []CustomFieldSubmission
	UploadNos                                                                 []string
}

type MessageInput struct {
	TicketNo, Body string
	UploadNos      []string
	TicketVersion  int64
}

type ClaimInput struct {
	TicketNo      string
	TicketVersion int64
}

type TransferInput struct {
	TicketNo, DepartmentNo, Reason string
	AssigneeUserID, TicketVersion  int64
}

type TransitionInput struct {
	TicketNo, Reason string
	TicketVersion    int64
}

type StatusTransitionInput struct {
	TicketNo, CustomStatusNo, Reason string
	TicketVersion                    int64
}

type RatingInput struct {
	TicketNo, Feedback string
	Stars, RoundNo     int
	TicketVersion      int64
}

type VersionedTicket struct {
	TicketNo string
	Version  int64
}

type MergeCandidate struct {
	TicketNo, Subject, Stage string
	Version                  int64
}

type MergeCandidatePage struct {
	Items      []MergeCandidate
	NextCursor string
}

type MergeInput struct {
	Main    VersionedTicket
	Sources []VersionedTicket
	Reason  string
}

type SplitInput struct {
	TicketNo, CategoryNo, Reason string
	TicketVersion                int64
	MessageNos, AttachmentNos    []string
}

type EscalateInput struct {
	TicketNo, Reason        string
	ToUserID, TicketVersion int64
}

type HideInput struct {
	TicketNo, TargetType, TargetNo, Reason string
	TicketVersion                          int64
}

type AttachmentReservation struct {
	UploadNo, TicketNo, FileName, Context string
	SizeBytes, NextOffset                 int64
	ExpiresAt                             time.Time
	Version                               int64
}

type AttachmentUploadInput struct {
	TicketNo, Context, FileName string
	SizeBytes                   int64
}

type AttachmentUploadResult struct {
	UploadNo, FileName, MediaType string
	SizeBytes, Version            int64
	Ready                         bool
}

type AttachmentOpen struct {
	FileName, MediaType string
	SizeBytes           int64
	Body                io.ReadCloser
}

type AnonymousAccess struct {
	LinkNo, Token, TicketNo string
	ExpiresAt               time.Time
}

type actor struct {
	UserID      int
	CustomerNo  string
	Global      bool
	DataScope   int
	Leader      bool
	Departments []string
}

type Service interface {
	SearchPublicCategories(context.Context, CategorySearchInput) (*CategoryPage, error)
	// PublicPresalesCustomFields returns the bounded active customer-visible definition set for public presales creation.
	PublicPresalesCustomFields(context.Context) ([]CustomFieldDefinition, error)
	// AccountCustomFields authenticates the current customer and returns active customer-visible customer-ticket definitions.
	AccountCustomFields(context.Context) ([]CustomFieldDefinition, error)
	// StaffInternalCustomFields requires internal-ticket creation permission and returns active internal-only definitions.
	StaffInternalCustomFields(context.Context) ([]CustomFieldDefinition, error)
	// SearchAccountEntitlementCandidates returns a bounded cursor page owned by the current customer and rejects non-customer contexts.
	SearchAccountEntitlementCandidates(context.Context, string, string, string, int) (*EntitlementCandidatePage, error)
	Departments(context.Context) ([]DepartmentOption, error)
	// ManagedDepartments lists queues for a supervisor within the same live role scope.
	ManagedDepartments(context.Context) ([]DepartmentOption, error)
	// OrganizationDepartments returns enabled, bounded department choices constrained by current role scope.
	OrganizationDepartments(context.Context, string, int, int) (*OrganizationDepartmentPage, error)
	IssueCategories(context.Context) ([]CategoryOption, error)
	Statuses(context.Context) ([]StatusOption, error)
	ReplyPresets(context.Context, string) ([]ReplyPresetOption, error)
	AutomationPolicy(context.Context) (*AutomationPolicy, error)
	UpdateAutomationPolicy(context.Context, AutomationPolicyInput) (*AutomationPolicy, error)
	DepartmentSettings(context.Context) ([]DepartmentOption, error)
	CreateDepartmentSetting(context.Context, DepartmentSettingInput) (*DepartmentOption, error)
	UpdateDepartmentSetting(context.Context, string, DepartmentSettingInput) (*DepartmentOption, error)
	DisableDepartmentSetting(context.Context, string, int64) (*DepartmentOption, error)
	IssueCategorySettings(context.Context) ([]IssueCategorySetting, error)
	CreateIssueCategorySetting(context.Context, IssueCategorySettingInput) (*IssueCategorySetting, error)
	UpdateIssueCategorySetting(context.Context, string, IssueCategorySettingInput) (*IssueCategorySetting, error)
	DisableIssueCategorySetting(context.Context, string, int64) (*IssueCategorySetting, error)
	CustomStatusSettings(context.Context) ([]CustomStatusSetting, error)
	CreateCustomStatusSetting(context.Context, CustomStatusSettingInput) (*CustomStatusSetting, error)
	UpdateCustomStatusSetting(context.Context, string, CustomStatusSettingInput) (*CustomStatusSetting, error)
	DisableCustomStatusSetting(context.Context, string, int64) (*CustomStatusSetting, error)
	ReplyPresetSettings(context.Context) ([]ReplyPresetSetting, error)
	CreateReplyPresetSetting(context.Context, ReplyPresetSettingInput) (*ReplyPresetSetting, error)
	UpdateReplyPresetSetting(context.Context, string, ReplyPresetSettingInput) (*ReplyPresetSetting, error)
	DisableReplyPresetSetting(context.Context, string, int64) (*ReplyPresetSetting, error)
	SLACalendarSettings(context.Context) ([]SLACalendarSetting, error)
	CreateSLACalendarSetting(context.Context, SLACalendarSettingInput) (*SLACalendarSetting, error)
	UpdateSLACalendarSetting(context.Context, string, SLACalendarSettingInput) (*SLACalendarSetting, error)
	DisableSLACalendarSetting(context.Context, string, int64) (*SLACalendarSetting, error)
	SLAHolidaySettings(context.Context, string) ([]SLAHolidaySetting, error)
	PutSLAHolidaySetting(context.Context, string, SLAHolidaySettingInput) (*SLAHolidaySetting, error)
	DeleteSLAHolidaySetting(context.Context, string, string) error
	SLARuleSettings(context.Context) ([]SLARuleSetting, error)
	CreateSLARuleSetting(context.Context, SLARuleSettingInput) (*SLARuleSetting, error)
	UpdateSLARuleSetting(context.Context, string, SLARuleSettingInput) (*SLARuleSetting, error)
	DisableSLARuleSetting(context.Context, string, int64) (*SLARuleSetting, error)
	// CustomFieldSettings requires configuration permission and returns a bounded directory including disabled definitions.
	CustomFieldSettings(context.Context) ([]CustomFieldDefinition, error)
	// CreateCustomFieldSetting validates and atomically creates one bilingual definition and its bounded options.
	CreateCustomFieldSetting(context.Context, CustomFieldSettingInput) (*CustomFieldDefinition, error)
	// UpdateCustomFieldSetting uses optimistic concurrency and never rewrites immutable submitted snapshots.
	UpdateCustomFieldSetting(context.Context, string, CustomFieldSettingInput) (*CustomFieldDefinition, error)
	// DisableCustomFieldSetting uses optimistic concurrency to disable a definition without physically deleting references.
	DisableCustomFieldSetting(context.Context, string, int64) (*CustomFieldDefinition, error)
	IssueVerifiedAnonymousLink(context.Context, string, string, string) (*AnonymousAccess, error)
	DetailAnonymous(context.Context, string) (*Detail, error)
	ReplyAnonymous(context.Context, string, MessageInput) (*Detail, error)
	CloseAnonymous(context.Context, string, int64) (*Detail, error)
	ReopenAnonymous(context.Context, string, int64) (*Detail, error)
	RateAnonymous(context.Context, string, RatingInput) (*Detail, error)
	// SearchStaff returns bounded rows and a same-filter total within the staff data scope; invalid pages and denied access fail closed.
	SearchStaff(context.Context, SearchInput) (*Page, error)
	// SearchAllStaff requires the separate global-list permission and returns scoped numbered or cursor rows with an accurate total.
	SearchAllStaff(context.Context, SearchInput) (*Page, error)
	DetailStaff(context.Context, string) (*Detail, error)
	CreateStaff(context.Context, CreateInput) (*Detail, error)
	ReplyStaff(context.Context, MessageInput) (*Detail, error)
	NoteStaff(context.Context, MessageInput) (*Detail, error)
	Claim(context.Context, ClaimInput) (*Detail, error)
	Transfer(context.Context, TransferInput) error
	Resolve(context.Context, TransitionInput) (*Detail, error)
	CloseStaff(context.Context, TransitionInput) (*Detail, error)
	ReopenStaff(context.Context, TransitionInput) (*Detail, error)
	TransitionStatus(context.Context, StatusTransitionInput) (*Detail, error)
	Merge(context.Context, MergeInput, bool) (*Detail, error)
	MergeCandidates(context.Context, string, string, int) (*MergeCandidatePage, error)
	MergeAllCandidates(context.Context, string, string, int) (*MergeCandidatePage, error)
	Split(context.Context, SplitInput) (*Detail, error)
	Escalate(context.Context, EscalateInput) (*Detail, error)
	Hide(context.Context, HideInput) (*Detail, error)
	// SearchAccount derives the active customer and returns only their visible tickets; Page zero uses Cursor, positive Page selects a bounded numbered page.
	SearchAccount(context.Context, SearchInput) (*Page, error)
	DetailAccount(context.Context, string) (*Detail, error)
	CreateAccount(context.Context, CreateInput) (*Detail, error)
	ReplyAccount(context.Context, MessageInput) (*Detail, error)
	CloseAccount(context.Context, TransitionInput) (*Detail, error)
	ReopenAccount(context.Context, TransitionInput) (*Detail, error)
	RateAccount(context.Context, RatingInput) (*Detail, error)
	ReserveStaffAttachment(context.Context, AttachmentUploadInput) (*AttachmentReservation, error)
	ReserveStaffInitialAttachment(context.Context, AttachmentUploadInput) (*AttachmentReservation, error)
	ReserveAccountAttachment(context.Context, AttachmentUploadInput) (*AttachmentReservation, error)
	ReserveAccountInitialAttachment(context.Context, AttachmentUploadInput) (*AttachmentReservation, error)
	ReserveAnonymousAttachment(context.Context, string, AttachmentUploadInput) (*AttachmentReservation, error)
	UploadStaffAttachment(context.Context, string, string, io.Reader) (*AttachmentUploadResult, error)
	UploadAccountAttachment(context.Context, string, string, io.Reader) (*AttachmentUploadResult, error)
	UploadAnonymousAttachment(context.Context, string, string, string, io.Reader) (*AttachmentUploadResult, error)
	OpenStaffAttachment(context.Context, string) (*AttachmentOpen, error)
	OpenAccountAttachment(context.Context, string) (*AttachmentOpen, error)
	OpenAnonymousAttachment(context.Context, string, string) (*AttachmentOpen, error)
	UpstreamChannel(context.Context) (*UpstreamChannel, error)
	ConfigureUpstreamChannel(context.Context, UpstreamChannelInput) (*UpstreamChannel, error)
	UpstreamDepartmentMap(context.Context, string) (*UpstreamDepartmentMap, error)
	SaveUpstreamDepartmentMap(context.Context, UpstreamDepartmentMapInput) (*UpstreamDepartmentMap, error)
	UpstreamEntitlementMap(context.Context, string) (*UpstreamEntitlementMap, error)
	SaveUpstreamEntitlementMap(context.Context, UpstreamEntitlementMapInput) (*UpstreamEntitlementMap, error)
	PrepareUpstream(context.Context, UpstreamPrepareInput) (*UpstreamPreview, error)
	ConfirmUpstream(context.Context, UpstreamConfirmInput) (*UpstreamForward, error)
	RetryUpstream(context.Context, UpstreamConfirmInput) (*UpstreamForward, error)
	UpstreamForward(context.Context, string) (*UpstreamForward, error)
	UpstreamForwards(context.Context, string) ([]UpstreamForward, error)
	SyncUpstreamReplies(context.Context, string, int64) (int, error)
	PublishUpstreamReply(context.Context, UpstreamPublishInput) (*Detail, error)
}

type serviceImpl struct {
	database             gdb.DB
	bizCtx               bizctxcap.Service
	principal            principal.Reader
	authorization        authz.Service
	storage              storagecap.Service
	notifications        notificationcap.EventSink
	audit                idccap.BusinessAuditWriter
	organization         orgcap.Service
	upstreamSecrets      secretcap.Manager
	upstreamSecretAccess secretcap.Accessor
	sessions             sessioncap.Service
	newUpstreamClient    func(string, string) (supportupstream.Client, error)
	now                  func() time.Time
	random               io.Reader
}

var _ Service = (*serviceImpl)(nil)

func New(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader, authorization authz.Service, storage storagecap.Service) (Service, error) {
	return newService(database, bizCtx, principalReader, authorization, storage, nil)
}

// NewReader creates a read-only support service with the shared organization capability.
func NewReader(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader, authorization authz.Service, storage storagecap.Service, organization orgcap.Service) (Service, error) {
	if organization == nil {
		return nil, gerror.New("support reader requires organization capability")
	}
	service, err := newService(database, bizCtx, principalReader, authorization, storage, nil)
	if err != nil {
		return nil, err
	}
	service.organization = organization
	return service, nil
}

// NewWithEvents creates the operational support service with a transaction-local
// business notification sink. Read-only consumers use New and cannot emit events.
func NewWithEvents(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader, authorization authz.Service, storage storagecap.Service, notifications notificationcap.EventSink) (Service, error) {
	if notifications == nil {
		return nil, gerror.New("support desk requires a notification event sink")
	}
	return newService(database, bizCtx, principalReader, authorization, storage, notifications)
}

// NewWithEventsAndAudit creates the route service with transaction-local
// notification and business-audit writers. Status mutations fail closed unless
// this production constructor supplies both dependencies.
func NewWithEventsAndAudit(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader, authorization authz.Service, storage storagecap.Service, notifications notificationcap.EventSink, auditWriter idccap.BusinessAuditWriter) (Service, error) {
	if auditWriter == nil {
		return nil, gerror.New("support desk requires a business audit writer")
	}
	if notifications == nil {
		return nil, gerror.New("support desk requires a notification event sink")
	}
	service, err := newService(database, bizCtx, principalReader, authorization, storage, notifications)
	if err != nil {
		return nil, err
	}
	service.audit = auditWriter
	return service, nil
}

// NewWithEventsAuditAndOrganization creates the production support service with
// live organization assignments and role authorization.
func NewWithEventsAuditAndOrganization(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader, authorization authz.Service, storage storagecap.Service, notifications notificationcap.EventSink, auditWriter idccap.BusinessAuditWriter, organization orgcap.Service) (Service, error) {
	if organization == nil {
		return nil, gerror.New("support desk requires organization capability")
	}
	service, err := NewWithEventsAndAudit(database, bizCtx, principalReader, authorization, storage, notifications, auditWriter)
	if err != nil {
		return nil, err
	}
	implementation, ok := service.(*serviceImpl)
	if !ok {
		return nil, gerror.New("support desk implementation is unavailable")
	}
	implementation.organization = organization
	return implementation, nil
}

// NewProduction creates the complete Phase 11 support route service.
func NewProduction(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader, authorization authz.Service, storage storagecap.Service, notifications notificationcap.EventSink, auditWriter idccap.BusinessAuditWriter, organization orgcap.Service, upstreamSecrets secretcap.Manager, upstreamSecretAccess secretcap.Accessor, sessions sessioncap.Service) (Service, error) {
	if upstreamSecrets == nil || upstreamSecretAccess == nil || sessions == nil {
		return nil, gerror.New("support desk requires upstream ticket secrets and sessions")
	}
	service, err := NewWithEventsAuditAndOrganization(database, bizCtx, principalReader, authorization, storage, notifications, auditWriter, organization)
	if err != nil {
		return nil, err
	}
	implementation, ok := service.(*serviceImpl)
	if !ok {
		return nil, gerror.New("support desk implementation is unavailable")
	}
	implementation.upstreamSecrets = upstreamSecrets
	implementation.upstreamSecretAccess = upstreamSecretAccess
	implementation.sessions = sessions
	implementation.newUpstreamClient = func(baseURL, authMode string) (supportupstream.Client, error) {
		return supportupstream.New(baseURL, authMode)
	}
	return implementation, nil
}

func newService(database gdb.DB, bizCtx bizctxcap.Service, principalReader principal.Reader, authorization authz.Service, storage storagecap.Service, notifications notificationcap.EventSink) (*serviceImpl, error) {
	if database == nil || bizCtx == nil || principalReader == nil || authorization == nil || storage == nil {
		return nil, gerror.New("support desk requires database, identity, authorization, and storage services")
	}
	return &serviceImpl{database: database, bizCtx: bizCtx, principal: principalReader, authorization: authorization, storage: storage, notifications: notifications, now: time.Now, random: rand.Reader}, nil
}

var businessNoPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)

func (s *serviceImpl) newNo(prefix string) (string, error) {
	buf := make([]byte, 16)
	if _, err := io.ReadFull(s.random, buf); err != nil {
		return "", gerror.Wrap(err, "generate support business number")
	}
	return prefix + hex.EncodeToString(buf), nil
}

func normalizeText(value string, maxBytes int) (string, bool) {
	value = strings.TrimSpace(value)
	return value, value != "" && len([]byte(value)) <= maxBytes
}

func customerStage(stage string) string {
	if stage == "waiting_upstream" {
		return "in_progress"
	}
	if stage == "waiting_customer" {
		return "waiting_customer"
	}
	return stage
}

func encodeCursor(id int64) string {
	if id <= 0 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10)))
}

func decodeCursor(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	id, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	return id, nil
}

func wrapUnavailable(err error, operation string) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(gerror.Wrap(err, operation), CodeUnavailable)
}

func sqlPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}

func (t *Ticket) finishProjection() {
	if t == nil {
		return
	}
	t.CustomerStage = customerStage(t.BaseStage)
}

func (s *serviceImpl) staffActor(ctx context.Context, permission string) (*actor, error) {
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 ||
		current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported ||
		(current.DataScope < 1 || current.DataScope > 4) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	// Shared support settings include cross-department routing and SLA policy.
	if permission == PermissionConfigure && current.DataScope >= 3 {
		return nil, bizNotAccessible()
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	keys := []authz.PermissionKey{authz.PermissionKey(permission), authz.PermissionKey(PermissionViewAll), authz.PermissionKey(PermissionLead), authz.PermissionKey(idccap.HostPermissionWildcard)}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if !grants[authz.PermissionKey(permission)] && !grants[authz.PermissionKey(idccap.HostPermissionWildcard)] {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	departments, err := s.actorDepartments(ctx, current.UserID)
	if err != nil {
		return nil, err
	}
	leader := grants[authz.PermissionKey(PermissionLead)] || grants[authz.PermissionKey(idccap.HostPermissionWildcard)]
	global := current.DataScope <= 2 && (grants[authz.PermissionKey(idccap.HostPermissionWildcard)] || grants[authz.PermissionKey(PermissionViewAll)])
	return &actor{UserID: current.UserID, Global: global, DataScope: current.DataScope, Leader: leader, Departments: departments}, nil
}

func (s *serviceImpl) customerActor(ctx context.Context) (*actor, error) {
	current := s.bizCtx.Current(ctx)
	directCustomer := current.ActingUserID == 0 && !current.IsImpersonation && !current.ActingAsTenant
	delegatedCustomer := current.IsImpersonation && current.ActingUserID > 0 && current.ActingUserID != current.UserID && !current.ActingAsTenant
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID || (!directCustomer && !delegatedCustomer) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || !businessNoPattern.MatchString(audience.CustomerNo) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return &actor{UserID: current.UserID, CustomerNo: audience.CustomerNo}, nil
}

func appendStaffAccess(where *strings.Builder, args *[]any, a *actor, alias string) {
	if a.Global {
		return
	}
	where.WriteString(" AND (")
	where.WriteString("(" + alias + ".assignee_user_id = ? OR EXISTS (SELECT 1 FROM plugin_idc_core_support_ticket_follower f WHERE f.tenant_id = 0 AND f.ticket_no = " + alias + ".ticket_no AND f.user_id = ?))")
	if len(a.Departments) == 0 {
		where.WriteString(" AND 1=0")
	} else {
		where.WriteString(" AND " + alias + ".department_no IN (" + sqlPlaceholders(len(a.Departments)) + ")")
	}
	*args = append(*args, a.UserID, a.UserID)
	for _, department := range a.Departments {
		*args = append(*args, department)
	}
	// Department scope follows the live organization assignment and explicit queue mapping.
	if a.DataScope <= 3 && len(a.Departments) > 0 {
		where.WriteString(" OR " + alias + ".department_no IN (" + sqlPlaceholders(len(a.Departments)) + ")")
		for _, department := range a.Departments {
			*args = append(*args, department)
		}
	}
	if a.DataScope <= 2 && a.Leader {
		where.WriteString(" OR " + alias + ".department_no IS NULL")
	}
	where.WriteString(")")
}

func scanTickets(ctx context.Context, database gdb.DB, query string, args ...any) ([]*Ticket, error) {
	var rows []*Ticket
	if err := database.GetScan(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	for _, row := range rows {
		row.finishProjection()
	}
	return rows, nil
}

func ticketProjection(alias, categoryAlias, customerAlias string) string {
	const projectionFormat = `%s.id,%s.ticket_no,%s.ticket_type,%s.visibility_code AS visibility,%s.customer_no,` +
		`COALESCE(NULLIF(%s.display_name,''),'') AS customer_display_label,` +
		`COALESCE(%s.related_entitlement_no,'') AS related_entitlement_no,` +
		`COALESCE(%s.related_entitlement_version,0) AS related_entitlement_version,` +
		`COALESCE(%s.related_entitlement_name_zh_cn,'') AS related_entitlement_name_zh_cn,` +
		`COALESCE(%s.related_entitlement_name_en_us,'') AS related_entitlement_name_en_us,` +
		`COALESCE(%s.related_entitlement_status,'') AS related_entitlement_status,` +
		`COALESCE(%s.anonymous_destination_digest,'') AS anonymous_destination_digest,` +
		`COALESCE(%s.anonymous_destination_kind,'') AS anonymous_destination_kind,%s.category_no,` +
		`COALESCE(%s.name_zh_cn,'') AS category_name_zh_cn,COALESCE(%s.name_en_us,'') AS category_name_en_us,` +
		`%s.department_no,COALESCE(%s.assignee_user_id,0) AS assignee_user_id,%s.subject,%s.base_stage,` +
		`%s.custom_status_no,%s.priority_code AS priority,%s.current_round,%s.follow_up_required,` +
		`%s.sla_risk_code AS sla_risk,EXISTS (SELECT 1 FROM plugin_idc_core_support_ticket_sla_round ` +
		`configured_sla WHERE configured_sla.tenant_id=0 AND configured_sla.ticket_no=%s.ticket_no AND ` +
		`configured_sla.round_no=%s.current_round AND configured_sla.rule_no IS NOT NULL) AS sla_configured,` +
		`%s.resolved_at,%s.closed_at,COALESCE(%s.closed_reason_code,'') AS closed_reason,%s.last_activity_at,` +
		`%s.version,%s.created_at,%s.updated_at`
	return fmt.Sprintf(
		projectionFormat,
		alias, alias, alias, alias, alias, customerAlias,
		alias, alias, alias, alias, alias,
		alias, alias, alias, categoryAlias,
		categoryAlias, alias, alias, alias, alias, alias, alias, alias, alias, alias,
		alias, alias, alias, alias, alias, alias, alias, alias, alias,
	)
}
