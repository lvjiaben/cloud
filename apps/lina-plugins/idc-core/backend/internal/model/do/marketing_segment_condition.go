// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingSegmentCondition is the golang structure of table plugin_idc_core_marketing_segment_condition for DAO operations like Where/Data.
type MarketingSegmentCondition struct {
	g.Meta            `orm:"table:plugin_idc_core_marketing_segment_condition, do:true"`
	Id                any        //
	TenantId          any        //
	ConditionNo       any        //
	SegmentNo         any        //
	OrGroupNo         any        //
	SortOrder         any        //
	FactCode          any        //
	OperatorCode      any        //
	ValueKindCode     any        //
	ValueText         *string    //
	ValueIntegerLower *int64     //
	ValueIntegerUpper *int64     //
	ValueBoolean      *bool      //
	CurrencyCode      *string    //
	CreatedAt         *time.Time //
}
