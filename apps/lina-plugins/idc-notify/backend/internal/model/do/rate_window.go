// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// RateWindow is the golang structure of table plugin_idc_notify_rate_window for DAO operations like Where/Data.
type RateWindow struct {
	g.Meta          `orm:"table:plugin_idc_notify_rate_window, do:true"`
	TenantId        any        //
	DimensionCode   any        //
	SubjectDigest   any        //
	WindowSeconds   any        //
	WindowStartedAt *time.Time //
	Counter         any        //
	ExpiresAt       *time.Time //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
