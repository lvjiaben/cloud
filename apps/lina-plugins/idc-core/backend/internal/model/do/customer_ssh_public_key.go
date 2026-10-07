// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerSshPublicKey is the golang structure of table plugin_idc_core_customer_ssh_public_key for DAO operations like Where/Data.
type CustomerSshPublicKey struct {
	g.Meta               `orm:"table:plugin_idc_core_customer_ssh_public_key, do:true"`
	Id                   any        //
	TenantId             any        //
	KeyNo                any        //
	CustomerNo           any        //
	Name                 any        //
	AlgorithmCode        any        //
	PublicKey            any        //
	FingerprintSha256    any        //
	RequestFingerprint   any        //
	IdempotencyKeyDigest any        //
	Version              any        //
	CreatedAt            *time.Time //
	RevokedAt            *time.Time //
}
