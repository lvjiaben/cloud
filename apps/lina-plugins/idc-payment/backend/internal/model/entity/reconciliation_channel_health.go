// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ReconciliationChannelHealth is the golang structure for table reconciliation_channel_health.
type ReconciliationChannelHealth struct {
	Id                      int64      `json:"id"                      orm:"id"                        description:""`
	ChannelNo               string     `json:"channelNo"               orm:"channel_no"                description:""`
	CapabilityCode          string     `json:"capabilityCode"          orm:"capability_code"           description:""`
	HealthCode              string     `json:"healthCode"              orm:"health_code"               description:""`
	LatestBatchNo           string     `json:"latestBatchNo"           orm:"latest_batch_no"           description:""`
	LatestBusinessDate      time.Time  `json:"latestBusinessDate"      orm:"latest_business_date"      description:""`
	UnresolvedCount         int        `json:"unresolvedCount"         orm:"unresolved_count"          description:""`
	ConsecutiveFailureCount int        `json:"consecutiveFailureCount" orm:"consecutive_failure_count" description:""`
	ResultUnknown           bool       `json:"resultUnknown"           orm:"result_unknown"            description:""`
	FailureCode             string     `json:"failureCode"             orm:"failure_code"              description:""`
	Version                 int64      `json:"version"                 orm:"version"                   description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                description:""`
}
