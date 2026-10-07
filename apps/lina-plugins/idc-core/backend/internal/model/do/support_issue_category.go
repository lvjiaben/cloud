// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportIssueCategory is the golang structure of table plugin_idc_core_support_issue_category for DAO operations like Where/Data.
type SupportIssueCategory struct {
	g.Meta       `orm:"table:plugin_idc_core_support_issue_category, do:true"`
	Id           any        //
	TenantId     any        //
	CategoryNo   any        //
	NameZhCn     any        //
	NameEnUs     any        //
	DepartmentNo any        //
	PriorityCode any        //
	SortOrder    any        //
	StatusCode   any        //
	Version      any        //
	CreatedBy    any        //
	UpdatedBy    any        //
	CreatedAt    *time.Time //
	UpdatedAt    *time.Time //
}
