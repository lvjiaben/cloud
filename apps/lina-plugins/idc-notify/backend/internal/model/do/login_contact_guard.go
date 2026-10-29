// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
)

// LoginContactGuard is the golang structure of table plugin_idc_notify_login_contact_guard for DAO operations like Where/Data.
type LoginContactGuard struct {
	g.Meta             `orm:"table:plugin_idc_notify_login_contact_guard, do:true"`
	TenantId           any //
	UserId             any //
	Revision           any //
	CreatedAt          any //
	UpdatedAt          any //
	RecoveryNo         any //
	CleanupPending     any //
	RecoveryAt         any //
	RecoveryRevision   any //
	CleanupTokenDigest any //
}
