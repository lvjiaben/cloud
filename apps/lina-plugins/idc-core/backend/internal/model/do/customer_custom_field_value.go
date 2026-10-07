// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerCustomFieldValue is the golang structure of table plugin_idc_core_customer_custom_field_value for DAO operations like Where/Data.
type CustomerCustomFieldValue struct {
	g.Meta           `orm:"table:plugin_idc_core_customer_custom_field_value, do:true"`
	Id               any        //
	TenantId         any        //
	ValueNo          any        //
	CustomerNo       any        //
	FieldNo          any        //
	TextValue        *string    //
	TextLookupDigest any        // Keyed normalized digest for exact filtering without indexing long plaintext
	SecretValueRef   any        // Opaque core secret-envelope reference used instead of plaintext for sensitive custom values
	IntegerValue     *int64     //
	DecimalValue     *string    //
	BooleanValue     *bool      //
	DateValue        *time.Time //
	DatetimeValue    *time.Time //
	StatusCode       any        //
	Version          any        //
	CreatedBy        any        //
	UpdatedBy        any        //
	CreatedAt        *time.Time //
	UpdatedAt        *time.Time //
}
