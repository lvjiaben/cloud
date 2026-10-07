// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractFile is the golang structure of table plugin_idc_compliance_contract_file for DAO operations like Where/Data.
type ContractFile struct {
	g.Meta             `orm:"table:plugin_idc_compliance_contract_file, do:true"`
	Id                 any        //
	TenantId           any        //
	FileNo             any        //
	ContractNo         any        //
	FlowNo             *string    //
	FileTypeCode       any        //
	PrivateObjectRef   any        //
	ContentSha256      any        //
	MediaType          any        //
	ByteSize           any        //
	StatusCode         any        //
	Immutable          any        //
	ArchivedAt         *time.Time //
	RetentionUntil     *time.Time //
	LegalHold          any        //
	CreatedAt          *time.Time //
	UpdatedAt          *time.Time //
	SupplierObservedAt *time.Time //
}
