// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// RecentAuthConsumption is the golang structure of table plugin_idc_core_recent_auth_consumption for DAO operations like Where/Data.
type RecentAuthConsumption struct {
	g.Meta        `orm:"table:plugin_idc_core_recent_auth_consumption, do:true"`
	Id            any        //
	TenantId      any        //
	NonceDigest   any        // Domain-separated SHA-256 digest of a random proof nonce; raw nonce and proof are never persisted
	PurposeCode   any        //
	TargetDigest  any        // Digest of the immutable dangerous-operation target or preview
	SessionDigest any        // Digest binding to the host session without persisting its raw identifier
	UserId        any        //
	LoginAt       *time.Time //
	IssuedAt      *time.Time //
	ExpiresAt     *time.Time //
	ConsumedAt    *time.Time //
}
