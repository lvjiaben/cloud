// This file defines shared department response DTOs for the org-core API.
package v1

// DeptItem exposes department fields visible through org-core APIs.
type DeptItem struct {
	Id        int    `json:"id" dc:"Department ID" eg:"100"`
	ParentId  int    `json:"parentId" dc:"Parent department ID, 0 indicates top-level department" eg:"0"`
	Ancestors string `json:"ancestors" dc:"Ancestor path, comma separated department ID links" eg:"0,100"`
	Name      string `json:"name" dc:"Department name" eg:"Technology Department"`
	Code      string `json:"code" dc:"Department code" eg:"TECH"`
	OrderNum  int    `json:"orderNum" dc:"Sort order" eg:"1"`
	Leader    int    `json:"leader" dc:"Owner user ID" eg:"1"`
	Phone     string `json:"phone" dc:"Contact number" eg:"021-88888888"`
	Email     string `json:"email" dc:"Contact email" eg:"tech@company.com"`
	Status    int    `json:"status" dc:"Department status: 1=normal 0=disabled" eg:"1"`
	Remark    string `json:"remark" dc:"Remark" eg:"Responsible for the company's technology research and development work"`
	CreatedAt *int64 `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1776756000000"`
	UpdatedAt *int64 `json:"updatedAt" dc:"Update time as Unix timestamp in milliseconds" eg:"1776757800000"`
}
