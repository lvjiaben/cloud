// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// PrincipalMapping is the golang structure of table plugin_idc_resource_principal_mapping for DAO operations like Where/Data.
type PrincipalMapping struct {
	g.Meta               `orm:"table:plugin_idc_resource_principal_mapping, do:true"`
	Id                   any        //
	TenantId             any        //
	MappingNo            any        //
	CustomerNo           any        // Opaque core-owned customer reference with no cross-plugin database foreign key
	ChannelNo            any        //
	ExternalPrincipalRef any        // Channel-local upstream client identifier; never inferred from the local customer number
	StatusCode           any        //
	EvidenceDigest       any        // Secret-free digest of the authoritative mapping evidence
	ObservedAt           *time.Time //
	Version              any        //
	CreatedAt            *time.Time //
	UpdatedAt            *time.Time //
}
