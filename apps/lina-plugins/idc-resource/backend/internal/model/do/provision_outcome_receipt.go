// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProvisionOutcomeReceipt is the golang structure of table plugin_idc_resource_provision_outcome_receipt for DAO operations like Where/Data.
type ProvisionOutcomeReceipt struct {
	g.Meta                  `orm:"table:plugin_idc_resource_provision_outcome_receipt, do:true"`
	Id                      any        //
	TenantId                any        //
	ReceiptNo               any        //
	TaskNo                  any        //
	ServiceNo               any        //
	StatusCode              any        //
	RequestDigest           any        //
	TaskEvidenceDigest      any        //
	CoreEvidenceDigest      any        // Secret-free evidence produced after core commits its service and fulfillment state
	CreatedAt               *time.Time //
	OperationNo             any        // Frozen opaque fulfillment operation copied from the validated provision request
	CustomerNo              any        // Frozen opaque customer owner copied from the validated provision request
	ConfigurationSnapshotNo any        // Frozen configuration snapshot copied from the validated provision request
}
