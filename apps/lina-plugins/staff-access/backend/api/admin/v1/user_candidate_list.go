package v1

import "github.com/gogf/gf/v2/frame/g"

type UserCandidateListReq struct {
	g.Meta   `path:"/admin/staff-access/user-candidates" method:"get" tags:"Staff Permissions" summary:"List visible enabled employee candidates" dc:"Returns a bounded host-governed employee candidate page without exposing host user storage." permission:"system:staff-access:list"`
	Keyword  string `json:"keyword"`
	Page     int    `json:"page" d:"1" v:"min:1"`
	PageSize int    `json:"pageSize" d:"20" v:"min:1|max:100"`
}
type UserCandidateItem struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}
type UserCandidateListRes struct {
	Items []UserCandidateItem `json:"items"`
	Total int                 `json:"total"`
}
