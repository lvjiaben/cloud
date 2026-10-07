// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Secret is the golang structure of table plugin_idc_core_secret for DAO operations like Where/Data.
type Secret struct {
	g.Meta         `orm:"table:plugin_idc_core_secret, do:true"`
	Id             any        //
	TenantId       any        //
	SecretNo       any        // Opaque secret business number reserved by the core business-number ledger
	OwnerPluginId  any        // Plugin that owns and authorizes the secret binding
	CallerScope    any        // Exact least-privilege caller scope bound into encryption AAD
	StatusCode     any        // Secret lifecycle state: configured or revoked
	CurrentVersion any        // Current immutable encrypted payload version
	MaskedTail     any        // Non-secret fixed-prefix display mask with at most four printable trailing bytes
	RevokedAt      *time.Time // Time after which callback access is denied
	CreatedAt      *time.Time //
	UpdatedAt      *time.Time //
}
