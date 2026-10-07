// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ExternalEvidence is the golang structure of table plugin_idc_compliance_external_evidence for DAO operations like Where/Data.
type ExternalEvidence struct {
	g.Meta            `orm:"table:plugin_idc_compliance_external_evidence, do:true"`
	Id                any        //
	TenantId          any        //
	EvidenceNo        any        //
	AggregateTypeCode any        //
	AggregateNo       any        //
	SourceSystemCode  any        //
	SourceEventRef    any        //
	EvidenceTypeCode  any        //
	PayloadSha256     any        //
	PrivateObjectRef  *string    //
	MediaType         *string    //
	ByteSize          *int64     //
	DispositionCode   any        //
	OccurredAt        *time.Time //
	ReceivedAt        *time.Time //
	CreatedAt         *time.Time //
}
