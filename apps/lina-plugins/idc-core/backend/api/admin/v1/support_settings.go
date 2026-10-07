package v1

import "github.com/gogf/gf/v2/frame/g"

type SupportDepartmentSetting struct {
	DepartmentNo       string `json:"departmentNo" dc:"Stable support department number"`
	NameZhCN           string `json:"nameZhCN" dc:"Chinese display name"`
	NameEnUS           string `json:"nameEnUS" dc:"English display name"`
	ParentDepartmentNo string `json:"parentDepartmentNo,omitempty" dc:"Optional parent support department number"`
	AssignmentMode     string `json:"assignmentMode" dc:"manual_claim, round_robin, or least_unresolved"`
	Status             string `json:"status" dc:"active or disabled"`
	Version            int64  `json:"version" dc:"Optimistic concurrency version"`
}

type SupportDepartmentSettingListReq struct {
	g.Meta `path:"/admin/support/settings/departments" method:"get" tags:"IDC Support Settings" summary:"List support department settings" permission:"idc:admin:support:configuration:manage"`
}
type SupportDepartmentSettingListRes struct {
	Items []SupportDepartmentSetting `json:"items" dc:"Support department settings"`
}
type SupportDepartmentSettingCreateReq struct {
	g.Meta             `path:"/admin/support/settings/departments" method:"post" tags:"IDC Support Settings" summary:"Create a support department" permission:"idc:admin:support:configuration:manage"`
	NameZhCN           string `json:"nameZhCN" dc:"Chinese display name"`
	NameEnUS           string `json:"nameEnUS" dc:"English display name"`
	ParentDepartmentNo string `json:"parentDepartmentNo" dc:"Optional parent support department number"`
	AssignmentMode     string `json:"assignmentMode" d:"manual_claim" dc:"manual_claim, round_robin, or least_unresolved"`
	Status             string `json:"status" d:"active" dc:"active or disabled"`
}
type SupportDepartmentSettingCreateRes struct {
	Item *SupportDepartmentSetting `json:"item" dc:"Created support department"`
}
type SupportDepartmentSettingUpdateReq struct {
	g.Meta             `path:"/admin/support/settings/departments/{departmentNo}" method:"put" tags:"IDC Support Settings" summary:"Update a support department" permission:"idc:admin:support:configuration:manage"`
	DepartmentNo       string `json:"departmentNo" dc:"Support department number"`
	NameZhCN           string `json:"nameZhCN" dc:"Chinese display name"`
	NameEnUS           string `json:"nameEnUS" dc:"English display name"`
	ParentDepartmentNo string `json:"parentDepartmentNo" dc:"Optional parent support department number"`
	AssignmentMode     string `json:"assignmentMode" dc:"manual_claim, round_robin, or least_unresolved"`
	Status             string `json:"status" dc:"active or disabled"`
	ExpectedVersion    int64  `json:"expectedVersion" dc:"Version returned by the latest read"`
}
type SupportDepartmentSettingUpdateRes = SupportDepartmentSettingCreateRes
type SupportDepartmentSettingDeleteReq struct {
	g.Meta          `path:"/admin/support/settings/departments/{departmentNo}" method:"delete" tags:"IDC Support Settings" summary:"Disable a support department" permission:"idc:admin:support:configuration:manage"`
	DepartmentNo    string `json:"departmentNo" dc:"Support department number"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Version returned by the latest read"`
}
type SupportDepartmentSettingDeleteRes = SupportDepartmentSettingCreateRes

type SupportIssueCategorySetting struct {
	CategoryNo   string `json:"categoryNo" dc:"Stable issue category number"`
	NameZhCN     string `json:"nameZhCN" dc:"Chinese display name"`
	NameEnUS     string `json:"nameEnUS" dc:"English display name"`
	DepartmentNo string `json:"departmentNo,omitempty" dc:"Mapped support department number"`
	Priority     string `json:"priority" dc:"low, normal, high, or urgent"`
	SortOrder    int    `json:"sortOrder" dc:"Ascending display order"`
	Status       string `json:"status" dc:"active or disabled"`
	Version      int64  `json:"version" dc:"Optimistic concurrency version"`
}
type SupportIssueCategorySettingListReq struct {
	g.Meta `path:"/admin/support/settings/issue-categories" method:"get" tags:"IDC Support Settings" summary:"List issue category settings" permission:"idc:admin:support:configuration:manage"`
}
type SupportIssueCategorySettingListRes struct {
	Items []SupportIssueCategorySetting `json:"items" dc:"Issue category settings"`
}
type SupportIssueCategorySettingCreateReq struct {
	g.Meta       `path:"/admin/support/settings/issue-categories" method:"post" tags:"IDC Support Settings" summary:"Create an issue category" permission:"idc:admin:support:configuration:manage"`
	NameZhCN     string `json:"nameZhCN" dc:"Chinese display name"`
	NameEnUS     string `json:"nameEnUS" dc:"English display name"`
	DepartmentNo string `json:"departmentNo" dc:"Optional mapped support department number"`
	Priority     string `json:"priority" dc:"low, normal, high, or urgent"`
	SortOrder    int    `json:"sortOrder" dc:"Ascending display order"`
	Status       string `json:"status" d:"active" dc:"active or disabled"`
}
type SupportIssueCategorySettingCreateRes struct {
	Item *SupportIssueCategorySetting `json:"item" dc:"Created issue category"`
}
type SupportIssueCategorySettingUpdateReq struct {
	g.Meta          `path:"/admin/support/settings/issue-categories/{categoryNo}" method:"put" tags:"IDC Support Settings" summary:"Update an issue category" permission:"idc:admin:support:configuration:manage"`
	CategoryNo      string `json:"categoryNo" dc:"Issue category number"`
	NameZhCN        string `json:"nameZhCN" dc:"Chinese display name"`
	NameEnUS        string `json:"nameEnUS" dc:"English display name"`
	DepartmentNo    string `json:"departmentNo" dc:"Optional mapped support department number"`
	Priority        string `json:"priority" dc:"low, normal, high, or urgent"`
	SortOrder       int    `json:"sortOrder" dc:"Ascending display order"`
	Status          string `json:"status" dc:"active or disabled"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Version returned by the latest read"`
}
type SupportIssueCategorySettingUpdateRes = SupportIssueCategorySettingCreateRes
type SupportIssueCategorySettingDeleteReq struct {
	g.Meta          `path:"/admin/support/settings/issue-categories/{categoryNo}" method:"delete" tags:"IDC Support Settings" summary:"Disable an issue category" permission:"idc:admin:support:configuration:manage"`
	CategoryNo      string `json:"categoryNo" dc:"Issue category number"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Version returned by the latest read"`
}
type SupportIssueCategorySettingDeleteRes = SupportIssueCategorySettingCreateRes

type SupportCustomStatusSetting struct {
	StatusNo  string `json:"statusNo" dc:"Stable custom status number"`
	NameZhCN  string `json:"nameZhCN" dc:"Chinese display name"`
	NameEnUS  string `json:"nameEnUS" dc:"English display name"`
	BaseStage string `json:"baseStage" dc:"Mapped base ticket stage"`
	SortOrder int    `json:"sortOrder" dc:"Ascending display order"`
	Status    string `json:"status" dc:"active or disabled"`
	Version   int64  `json:"version" dc:"Optimistic concurrency version"`
}
type SupportCustomStatusSettingListReq struct {
	g.Meta `path:"/admin/support/settings/statuses" method:"get" tags:"IDC Support Settings" summary:"List custom ticket status settings" permission:"idc:admin:support:configuration:manage"`
}
type SupportCustomStatusSettingListRes struct {
	Items []SupportCustomStatusSetting `json:"items" dc:"Custom ticket status settings"`
}
type SupportCustomStatusSettingCreateReq struct {
	g.Meta    `path:"/admin/support/settings/statuses" method:"post" tags:"IDC Support Settings" summary:"Create a custom ticket status" permission:"idc:admin:support:configuration:manage"`
	NameZhCN  string `json:"nameZhCN" dc:"Chinese display name"`
	NameEnUS  string `json:"nameEnUS" dc:"English display name"`
	BaseStage string `json:"baseStage" dc:"Mapped base ticket stage"`
	SortOrder int    `json:"sortOrder" dc:"Ascending display order"`
	Status    string `json:"status" d:"active" dc:"active or disabled"`
}
type SupportCustomStatusSettingCreateRes struct {
	Item *SupportCustomStatusSetting `json:"item" dc:"Created custom ticket status"`
}
type SupportCustomStatusSettingUpdateReq struct {
	g.Meta          `path:"/admin/support/settings/statuses/{statusNo}" method:"put" tags:"IDC Support Settings" summary:"Update a custom ticket status" permission:"idc:admin:support:configuration:manage"`
	StatusNo        string `json:"statusNo" dc:"Custom ticket status number"`
	NameZhCN        string `json:"nameZhCN" dc:"Chinese display name"`
	NameEnUS        string `json:"nameEnUS" dc:"English display name"`
	BaseStage       string `json:"baseStage" dc:"Mapped base ticket stage"`
	SortOrder       int    `json:"sortOrder" dc:"Ascending display order"`
	Status          string `json:"status" dc:"active or disabled"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Version returned by the latest read"`
}
type SupportCustomStatusSettingUpdateRes = SupportCustomStatusSettingCreateRes
type SupportCustomStatusSettingDeleteReq struct {
	g.Meta          `path:"/admin/support/settings/statuses/{statusNo}" method:"delete" tags:"IDC Support Settings" summary:"Disable a custom ticket status" permission:"idc:admin:support:configuration:manage"`
	StatusNo        string `json:"statusNo" dc:"Custom ticket status number"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Version returned by the latest read"`
}
type SupportCustomStatusSettingDeleteRes = SupportCustomStatusSettingCreateRes

type SupportReplyPresetSetting struct {
	PresetNo     string `json:"presetNo" dc:"Stable reply preset number"`
	DepartmentNo string `json:"departmentNo" dc:"Support department number"`
	Locale       string `json:"locale" dc:"zh-CN or en-US"`
	Title        string `json:"title" dc:"Preset title"`
	Body         string `json:"body" dc:"Text inserted into the reply draft"`
	SortOrder    int    `json:"sortOrder" dc:"Ascending display order"`
	Status       string `json:"status" dc:"active or disabled"`
	Version      int64  `json:"version" dc:"Optimistic concurrency version"`
}
type SupportReplyPresetSettingListReq struct {
	g.Meta `path:"/admin/support/settings/reply-presets" method:"get" tags:"IDC Support Settings" summary:"List reply preset settings" permission:"idc:admin:support:configuration:manage"`
}
type SupportReplyPresetSettingListRes struct {
	Items []SupportReplyPresetSetting `json:"items" dc:"Reply preset settings"`
}
type SupportReplyPresetSettingCreateReq struct {
	g.Meta       `path:"/admin/support/settings/reply-presets" method:"post" tags:"IDC Support Settings" summary:"Create a reply preset" permission:"idc:admin:support:configuration:manage"`
	DepartmentNo string `json:"departmentNo" dc:"Support department number"`
	Locale       string `json:"locale" dc:"zh-CN or en-US"`
	Title        string `json:"title" dc:"Preset title"`
	Body         string `json:"body" dc:"Text inserted into the reply draft"`
	SortOrder    int    `json:"sortOrder" dc:"Ascending display order"`
	Status       string `json:"status" d:"active" dc:"active or disabled"`
}
type SupportReplyPresetSettingCreateRes struct {
	Item *SupportReplyPresetSetting `json:"item" dc:"Created reply preset"`
}
type SupportReplyPresetSettingUpdateReq struct {
	g.Meta          `path:"/admin/support/settings/reply-presets/{presetNo}" method:"put" tags:"IDC Support Settings" summary:"Update a reply preset" permission:"idc:admin:support:configuration:manage"`
	PresetNo        string `json:"presetNo" dc:"Reply preset number"`
	DepartmentNo    string `json:"departmentNo" dc:"Support department number"`
	Locale          string `json:"locale" dc:"zh-CN or en-US"`
	Title           string `json:"title" dc:"Preset title"`
	Body            string `json:"body" dc:"Text inserted into the reply draft"`
	SortOrder       int    `json:"sortOrder" dc:"Ascending display order"`
	Status          string `json:"status" dc:"active or disabled"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Version returned by the latest read"`
}
type SupportReplyPresetSettingUpdateRes = SupportReplyPresetSettingCreateRes
type SupportReplyPresetSettingDeleteReq struct {
	g.Meta          `path:"/admin/support/settings/reply-presets/{presetNo}" method:"delete" tags:"IDC Support Settings" summary:"Disable a reply preset" permission:"idc:admin:support:configuration:manage"`
	PresetNo        string `json:"presetNo" dc:"Reply preset number"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Version returned by the latest read"`
}
type SupportReplyPresetSettingDeleteRes = SupportReplyPresetSettingCreateRes

type SupportSLACalendarSetting struct {
	CalendarNo      string `json:"calendarNo" dc:"Stable SLA calendar number"`
	Name            string `json:"name" dc:"Calendar display name"`
	Mode            string `json:"mode" dc:"continuous or business_hours"`
	Timezone        string `json:"timezone,omitempty" dc:"IANA timezone for business hours"`
	Workdays        []int  `json:"workdays" dc:"ISO weekdays from 1 to 7"`
	WorkStartMinute *int   `json:"workStartMinute,omitempty" dc:"Minute after midnight when work starts"`
	WorkEndMinute   *int   `json:"workEndMinute,omitempty" dc:"Minute after midnight when work ends"`
	Status          string `json:"status" dc:"active or disabled"`
	Version         int64  `json:"version" dc:"Optimistic concurrency version"`
}
type SupportSLACalendarSettingListReq struct {
	g.Meta `path:"/admin/support/settings/sla-calendars" method:"get" tags:"IDC Support Settings" summary:"List SLA calendar settings" permission:"idc:admin:support:configuration:manage"`
}
type SupportSLACalendarSettingListRes struct {
	Items []SupportSLACalendarSetting `json:"items" dc:"SLA calendar settings"`
}
type SupportSLACalendarSettingCreateReq struct {
	g.Meta          `path:"/admin/support/settings/sla-calendars" method:"post" tags:"IDC Support Settings" summary:"Create an SLA calendar" permission:"idc:admin:support:configuration:manage"`
	Name            string `json:"name" dc:"Calendar display name"`
	Mode            string `json:"mode" d:"continuous" dc:"continuous or business_hours"`
	Timezone        string `json:"timezone" dc:"IANA timezone for business hours"`
	Workdays        []int  `json:"workdays" dc:"ISO weekdays from 1 to 7"`
	WorkStartMinute *int   `json:"workStartMinute" dc:"Minute after midnight when work starts"`
	WorkEndMinute   *int   `json:"workEndMinute" dc:"Minute after midnight when work ends"`
	Status          string `json:"status" d:"active" dc:"active or disabled"`
}
type SupportSLACalendarSettingCreateRes struct {
	Item *SupportSLACalendarSetting `json:"item" dc:"Created SLA calendar"`
}
type SupportSLACalendarSettingUpdateReq struct {
	g.Meta          `path:"/admin/support/settings/sla-calendars/{calendarNo}" method:"put" tags:"IDC Support Settings" summary:"Update an SLA calendar" permission:"idc:admin:support:configuration:manage"`
	CalendarNo      string `json:"calendarNo" dc:"SLA calendar number"`
	Name            string `json:"name" dc:"Calendar display name"`
	Mode            string `json:"mode" dc:"continuous or business_hours"`
	Timezone        string `json:"timezone" dc:"IANA timezone for business hours"`
	Workdays        []int  `json:"workdays" dc:"ISO weekdays from 1 to 7"`
	WorkStartMinute *int   `json:"workStartMinute" dc:"Minute after midnight when work starts"`
	WorkEndMinute   *int   `json:"workEndMinute" dc:"Minute after midnight when work ends"`
	Status          string `json:"status" dc:"active or disabled"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Version returned by the latest read"`
}
type SupportSLACalendarSettingUpdateRes = SupportSLACalendarSettingCreateRes
type SupportSLACalendarSettingDeleteReq struct {
	g.Meta          `path:"/admin/support/settings/sla-calendars/{calendarNo}" method:"delete" tags:"IDC Support Settings" summary:"Disable an SLA calendar" permission:"idc:admin:support:configuration:manage"`
	CalendarNo      string `json:"calendarNo" dc:"SLA calendar number"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Version returned by the latest read"`
}
type SupportSLACalendarSettingDeleteRes = SupportSLACalendarSettingCreateRes

type SupportSLAHolidaySetting struct {
	CalendarNo string `json:"calendarNo" dc:"SLA calendar number"`
	Date       string `json:"date" dc:"Holiday date in YYYY-MM-DD"`
	Name       string `json:"name" dc:"Holiday display name"`
}
type SupportSLAHolidaySettingListReq struct {
	g.Meta     `path:"/admin/support/settings/sla-calendars/{calendarNo}/holidays" method:"get" tags:"IDC Support Settings" summary:"List SLA calendar holidays" permission:"idc:admin:support:configuration:manage"`
	CalendarNo string `json:"calendarNo" dc:"SLA calendar number"`
}
type SupportSLAHolidaySettingListRes struct {
	Items []SupportSLAHolidaySetting `json:"items" dc:"SLA calendar holidays"`
}
type SupportSLAHolidaySettingPutReq struct {
	g.Meta     `path:"/admin/support/settings/sla-calendars/{calendarNo}/holidays/{date}" method:"put" tags:"IDC Support Settings" summary:"Create or update an SLA calendar holiday" permission:"idc:admin:support:configuration:manage"`
	CalendarNo string `json:"calendarNo" dc:"SLA calendar number"`
	Date       string `json:"date" dc:"Holiday date in YYYY-MM-DD"`
	Name       string `json:"name" dc:"Holiday display name"`
}
type SupportSLAHolidaySettingPutRes struct {
	Item *SupportSLAHolidaySetting `json:"item" dc:"Saved SLA calendar holiday"`
}
type SupportSLAHolidaySettingDeleteReq struct {
	g.Meta     `path:"/admin/support/settings/sla-calendars/{calendarNo}/holidays/{date}" method:"delete" tags:"IDC Support Settings" summary:"Delete an SLA calendar holiday" permission:"idc:admin:support:configuration:manage"`
	CalendarNo string `json:"calendarNo" dc:"SLA calendar number"`
	Date       string `json:"date" dc:"Holiday date in YYYY-MM-DD"`
}
type SupportSLAHolidaySettingDeleteRes struct{}

type SupportSLARuleSetting struct {
	RuleNo               string `json:"ruleNo" dc:"Stable SLA rule number"`
	DepartmentNo         string `json:"departmentNo" dc:"Support department number"`
	CategoryNo           string `json:"categoryNo,omitempty" dc:"Optional issue category override"`
	Priority             string `json:"priority" dc:"low, normal, high, or urgent"`
	CalendarNo           string `json:"calendarNo" dc:"SLA calendar number"`
	FirstResponseMinutes *int   `json:"firstResponseMinutes,omitempty" dc:"Configured first response target in minutes"`
	ResolutionMinutes    *int   `json:"resolutionMinutes,omitempty" dc:"Configured resolution target in minutes"`
	NearDueMinutes       []int  `json:"nearDueMinutes" dc:"Reminder nodes before a deadline in minutes"`
	Status               string `json:"status" dc:"active or disabled"`
	Version              int64  `json:"version" dc:"Optimistic concurrency version"`
}
type SupportSLARuleSettingListReq struct {
	g.Meta `path:"/admin/support/settings/sla-rules" method:"get" tags:"IDC Support Settings" summary:"List SLA rule settings" permission:"idc:admin:support:configuration:manage"`
}
type SupportSLARuleSettingListRes struct {
	Items []SupportSLARuleSetting `json:"items" dc:"SLA rule settings"`
}
type SupportSLARuleSettingCreateReq struct {
	g.Meta               `path:"/admin/support/settings/sla-rules" method:"post" tags:"IDC Support Settings" summary:"Create an SLA rule" permission:"idc:admin:support:configuration:manage"`
	DepartmentNo         string `json:"departmentNo" dc:"Support department number"`
	CategoryNo           string `json:"categoryNo" dc:"Optional issue category override"`
	Priority             string `json:"priority" dc:"low, normal, high, or urgent"`
	CalendarNo           string `json:"calendarNo" dc:"SLA calendar number"`
	FirstResponseMinutes *int   `json:"firstResponseMinutes" dc:"Configured first response target in minutes"`
	ResolutionMinutes    *int   `json:"resolutionMinutes" dc:"Configured resolution target in minutes"`
	NearDueMinutes       []int  `json:"nearDueMinutes" dc:"Reminder nodes before a deadline in minutes"`
	Status               string `json:"status" d:"active" dc:"active or disabled"`
}
type SupportSLARuleSettingCreateRes struct {
	Item *SupportSLARuleSetting `json:"item" dc:"Created SLA rule"`
}
type SupportSLARuleSettingUpdateReq struct {
	g.Meta               `path:"/admin/support/settings/sla-rules/{ruleNo}" method:"put" tags:"IDC Support Settings" summary:"Update an SLA rule" permission:"idc:admin:support:configuration:manage"`
	RuleNo               string `json:"ruleNo" dc:"SLA rule number"`
	DepartmentNo         string `json:"departmentNo" dc:"Support department number"`
	CategoryNo           string `json:"categoryNo" dc:"Optional issue category override"`
	Priority             string `json:"priority" dc:"low, normal, high, or urgent"`
	CalendarNo           string `json:"calendarNo" dc:"SLA calendar number"`
	FirstResponseMinutes *int   `json:"firstResponseMinutes" dc:"Configured first response target in minutes"`
	ResolutionMinutes    *int   `json:"resolutionMinutes" dc:"Configured resolution target in minutes"`
	NearDueMinutes       []int  `json:"nearDueMinutes" dc:"Reminder nodes before a deadline in minutes"`
	Status               string `json:"status" dc:"active or disabled"`
	ExpectedVersion      int64  `json:"expectedVersion" dc:"Version returned by the latest read"`
}
type SupportSLARuleSettingUpdateRes = SupportSLARuleSettingCreateRes
type SupportSLARuleSettingDeleteReq struct {
	g.Meta          `path:"/admin/support/settings/sla-rules/{ruleNo}" method:"delete" tags:"IDC Support Settings" summary:"Disable an SLA rule" permission:"idc:admin:support:configuration:manage"`
	RuleNo          string `json:"ruleNo" dc:"SLA rule number"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Version returned by the latest read"`
}
type SupportSLARuleSettingDeleteRes = SupportSLARuleSettingCreateRes
