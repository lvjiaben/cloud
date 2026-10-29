// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// NoticeRead is the golang structure for table notice_read.
type NoticeRead struct {
	Id        int64      `json:"id"        orm:"id"         description:""`
	NoticeId  int64      `json:"noticeId"  orm:"notice_id"  description:"Announcement ID"`
	UserId    int64      `json:"userId"    orm:"user_id"    description:"Reader user ID"`
	CreatedAt *time.Time `json:"createdAt" orm:"created_at" description:"Read time"`
}
