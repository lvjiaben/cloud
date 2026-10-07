// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerConsent is the golang structure of table plugin_idc_core_customer_consent for DAO operations like Where/Data.
type CustomerConsent struct {
	g.Meta          `orm:"table:plugin_idc_core_customer_consent, do:true"`
	Id              any        //
	TenantId        any        //
	ConsentNo       any        //
	CustomerNo      any        //
	ConsentTypeCode any        //
	PolicyReference any        //
	PolicyVersion   any        //
	ActionCode      any        //
	SourceCode      any        //
	Locale          any        //
	EvidenceDigest  any        // Digest of canonical consent evidence; raw request metadata remains outside ordinary fields
	OccurredAt      *time.Time //
	RecordedBy      any        //
	CreatedAt       *time.Time //
}
