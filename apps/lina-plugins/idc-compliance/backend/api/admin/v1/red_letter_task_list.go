package v1

import "github.com/gogf/gf/v2/frame/g"

type RedLetterTaskListReq struct {
	Page       int `json:"page" v:"min:0|max:1000000" dc:"Requested page number; omit for cursor pagination" eg:"1"`
	g.Meta     `path:"/admin/compliance/red-letter-tasks" method:"get" tags:"IDC Compliance Red Letter" summary:"List red-letter tasks" dc:"Return one bounded installation-local task page with its authorized total, without private evidence locations." permission:"idc:admin:compliance:red-letter:query"`
	CustomerNo string `json:"customerNo" v:"max-length:64" dc:"Optional exact customer number" eg:"CUS_0123456789ABCDEF"`
	StatusCode string `json:"statusCode" v:"max-length:24" dc:"Optional exact task state" eg:"manual_required"`
	PageSize   int    `json:"pageSize" v:"min:0|max:100" dc:"Page size; zero selects 20" eg:"20"`
	Cursor     string `json:"cursor" v:"max-length:256" dc:"Opaque next-page cursor" eg:"eyJjcmVhdGVkQXQiOiIyMDI2LTA4LTE4VDA4OjAwOjAwWiIsImlkIjoxfQ"`
}
type RedLetterTaskListRes struct {
	Total      int                 `json:"total" dc:"Total matching records in the authorized scope" eg:"25"`
	Items      []RedLetterTaskItem `json:"items" dc:"Safe red-letter task rows" eg:"[]"`
	NextCursor string              `json:"nextCursor" dc:"Opaque next-page cursor; empty on final page" eg:""`
}
