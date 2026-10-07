// This file defines the staff evidence-backed red-letter completion API.

package v1

import "github.com/gogf/gf/v2/frame/g"

type RedLetterManualCompletionCreateReq struct {
	g.Meta            `path:"/admin/compliance/red-letter-tasks/{taskNo}/manual-completions" method:"post" tags:"IDC Compliance Red Letter" summary:"Complete a red-letter task with evidence" dc:"Record real tax correction evidence using the independent handling permission. Evidence and audit are append-only; an unknown result without evidence cannot be completed." permission:"idc:admin:compliance:red-letter:handle"`
	TaskNo            string `json:"taskNo" v:"required|max-length:64" dc:"Stable red-letter task number" eg:"rlt_0123456789abcdef0123456789abcdef01234567"`
	ExpectedVersion   int64  `json:"expectedVersion" v:"required|min:1" dc:"Last observed task version" eg:"2"`
	IdempotencyKey    string `json:"idempotencyKey" v:"required|min-length:16|max-length:128" dc:"Caller-generated exact retry key" eg:"red-letter-completion-0001"`
	EvidenceReference string `json:"evidenceReference" v:"required|max-length:128" dc:"Stable real-world red-letter evidence reference" eg:"red-confirmation-20260817-001"`
	EvidenceSHA256    string `json:"evidenceSha256" v:"required|regex:^[0-9a-f]{64}$" dc:"SHA-256 digest of the submitted evidence" eg:"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`
	UploadedObjectRef string `json:"uploadedObjectRef" v:"max-length:512" dc:"Optional opaque private-storage object reference returned by the upload flow" eg:"private://red-letter/0123456789abcdef"`
	MediaType         string `json:"mediaType" v:"max-length:128" dc:"Required media type when an uploaded object reference is provided" eg:"application/pdf"`
	ByteSize          int64  `json:"byteSize" v:"min:0|max:20971520" dc:"Required evidence file size when an uploaded object reference is provided" eg:"102400"`
	OccurredAt        int64  `json:"occurredAt" v:"required|min:1" dc:"Actual evidence occurrence time as Unix milliseconds" eg:"1767225600000"`
}

type RedLetterManualCompletionCreateRes struct {
	Item *RedLetterTaskItem `json:"item" dc:"Completed red-letter task" eg:"{}"`
}
