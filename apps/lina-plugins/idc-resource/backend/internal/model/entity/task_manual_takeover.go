// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// TaskManualTakeover is the golang structure for table task_manual_takeover.
type TaskManualTakeover struct {
	Id             int64      `json:"id"             orm:"id"               description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"        description:""`
	TakeoverNo     string     `json:"takeoverNo"     orm:"takeover_no"      description:""`
	TaskNo         string     `json:"taskNo"         orm:"task_no"          description:""`
	FromStatusCode string     `json:"fromStatusCode" orm:"from_status_code" description:""`
	TaskVersion    int64      `json:"taskVersion"    orm:"task_version"     description:""`
	ReasonCode     string     `json:"reasonCode"     orm:"reason_code"      description:""`
	RequestDigest  string     `json:"requestDigest"  orm:"request_digest"   description:"Canonical digest of actor, task version, reason, and sorted evidence numbers for exact safe replay"`
	EvidenceDigest string     `json:"evidenceDigest" orm:"evidence_digest"  description:"Canonical digest of immutable task evidence numbers and their redacted contents"`
	EvidenceCount  int        `json:"evidenceCount"  orm:"evidence_count"   description:""`
	AssignedUserId int64      `json:"assignedUserId" orm:"assigned_user_id" description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"       description:""`
}
