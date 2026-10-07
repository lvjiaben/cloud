// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingSegmentCondition is the golang structure for table marketing_segment_condition.
type MarketingSegmentCondition struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	ConditionNo       string     `json:"conditionNo"       orm:"condition_no"        description:""`
	SegmentNo         string     `json:"segmentNo"         orm:"segment_no"          description:""`
	OrGroupNo         int        `json:"orGroupNo"         orm:"or_group_no"         description:""`
	SortOrder         int        `json:"sortOrder"         orm:"sort_order"          description:""`
	FactCode          string     `json:"factCode"          orm:"fact_code"           description:""`
	OperatorCode      string     `json:"operatorCode"      orm:"operator_code"       description:""`
	ValueKindCode     string     `json:"valueKindCode"     orm:"value_kind_code"     description:""`
	ValueText         *string    `json:"valueText"         orm:"value_text"          description:""`
	ValueIntegerLower *int64     `json:"valueIntegerLower" orm:"value_integer_lower" description:""`
	ValueIntegerUpper *int64     `json:"valueIntegerUpper" orm:"value_integer_upper" description:""`
	ValueBoolean      *bool      `json:"valueBoolean"      orm:"value_boolean"       description:""`
	CurrencyCode      *string    `json:"currencyCode"      orm:"currency_code"       description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
}
