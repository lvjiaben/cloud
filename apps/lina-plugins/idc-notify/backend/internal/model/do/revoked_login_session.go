// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
)

// RevokedLoginSession is the golang structure of table plugin_idc_notify_revoked_login_session for DAO operations like Where/Data.
type RevokedLoginSession struct {
	g.Meta        `orm:"table:plugin_idc_notify_revoked_login_session, do:true"`
	TenantId      any //
	UserId        any //
	SessionDigest any //
	SessionId     any //
	RecoveryNo    any //
	CleanupDone   any //
	CreatedAt     any //
	UpdatedAt     any //
}
