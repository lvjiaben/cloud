// This file exposes bounded organization choices for configuring support queues.

package v1

import "github.com/gogf/gf/v2/frame/g"

// SupportOrganizationDepartment is a visible organization candidate.
type SupportOrganizationDepartment struct {
	DeptID   int    `json:"deptId" dc:"Organization department identifier" eg:"1"`
	DeptName string `json:"deptName" dc:"Organization department display name" eg:"Customer Support"`
	DeptCode string `json:"deptCode" dc:"Organization department code" eg:"SUPPORT"`
}

// SupportOrganizationDepartmentListReq lists organization candidates in role scope.
type SupportOrganizationDepartmentListReq struct {
	g.Meta   `path:"/admin/support/organization-departments" method:"get" tags:"IDC Support Settings" summary:"List organization departments for support queues" dc:"Returns enabled organization departments within the current employee data scope; disabled organization returns no choices" permission:"idc:admin:support:configuration:manage"`
	Keyword  string `json:"keyword" dc:"Optional department name or code search; empty includes all visible departments" eg:"Support"`
	Page     int    `json:"page" d:"1" v:"min:1" dc:"Page number starting at one" eg:"1"`
	PageSize int    `json:"pageSize" d:"20" v:"between:1,100" dc:"Page size between one and one hundred" eg:"20"`
}

// SupportOrganizationDepartmentListRes contains the bounded choices and total.
type SupportOrganizationDepartmentListRes struct {
	Items []SupportOrganizationDepartment `json:"items" dc:"Visible enabled organization department choices" eg:"[]"`
	Total int                             `json:"total" dc:"Total matching departments within the same scope" eg:"1"`
}

// SupportManagedDepartmentListReq reads queues managed by the current supervisor.
type SupportManagedDepartmentListReq struct {
	g.Meta `path:"/admin/support/managed-departments" method:"get" tags:"IDC Support Settings" summary:"List managed support departments" dc:"Returns support departments within the current supervisor role and organization scope; management does not grant ticket actions" permission:"idc:admin:support:ticket:lead"`
}

// SupportManagedDepartmentListRes returns bounded managed support queues.
type SupportManagedDepartmentListRes struct {
	Items []SupportDepartmentSetting `json:"items" dc:"Support queues within the current supervisor scope" eg:"[]"`
}
