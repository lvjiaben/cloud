// This file defines the permissioned staff red-letter task detail API.

package v1

import "github.com/gogf/gf/v2/frame/g"

type RedLetterTaskGetReq struct {
	g.Meta `path:"/admin/compliance/red-letter-tasks/{taskNo}" method:"get" tags:"IDC Compliance Red Letter" summary:"Get a red-letter task" dc:"Return one bounded task after the independent red-letter query permission. Private evidence locations are never returned." permission:"idc:admin:compliance:red-letter:query"`
	TaskNo string `json:"taskNo" v:"required|max-length:64" dc:"Stable red-letter task number" eg:"rlt_0123456789abcdef0123456789abcdef01234567"`
}

type RedLetterTaskGetRes struct {
	Item             *RedLetterTaskItem        `json:"item" dc:"Safe red-letter task projection" eg:"{}"`
	Candidates       []RedLetterCandidateItem  `json:"candidates" dc:"Bounded eligible original-invoice choices" eg:"[]"`
	Allocations      []RedLetterAllocationItem `json:"allocations" dc:"Bounded immutable resolved allocations" eg:"[]"`
	Evidence         *RedLetterEvidenceItem    `json:"evidence" dc:"Immutable completion evidence metadata without private location" eg:"{}"`
	HistoryTruncated bool                      `json:"historyTruncated" dc:"Whether a bounded collection has more rows" eg:"false"`
}
