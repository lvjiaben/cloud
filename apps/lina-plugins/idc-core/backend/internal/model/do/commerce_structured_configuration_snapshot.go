// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommerceStructuredConfigurationSnapshot is the golang structure of table plugin_idc_core_commerce_structured_configuration_snapshot for DAO operations like Where/Data.
type CommerceStructuredConfigurationSnapshot struct {
	g.Meta                         `orm:"table:plugin_idc_core_commerce_structured_configuration_snapshot, do:true"`
	Id                             any        //
	TenantId                       any        //
	SnapshotNo                     any        //
	CustomerNo                     any        //
	UploaderUserId                 any        //
	QuoteNo                        any        //
	QuoteItemNo                    any        //
	ProductNo                      any        //
	VersionNo                      any        //
	ConfigurationRevision          any        //
	InterfaceInstanceNo            any        //
	InterfaceConfigurationRevision any        //
	CustomerDeclarationDigest      any        //
	CandidateSnapshotDigest        any        //
	CustomerValueDigest            any        //
	CandidateSnapshot              any        //
	TypedValueSnapshot             any        //
	ContentDigest                  any        //
	OrderNo                        any        //
	OrderItemNo                    any        //
	BoundAt                        *time.Time //
	CreatedAt                      *time.Time //
	ServerLoginToken               any        //
}
