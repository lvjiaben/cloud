// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractTemplateContent is the golang structure of table plugin_idc_compliance_contract_template_content for DAO operations like Where/Data.
type ContractTemplateContent struct {
	g.Meta                   `orm:"table:plugin_idc_compliance_contract_template_content, do:true"`
	Id                       any        //
	TenantId                 any        //
	ContentRevisionNo        any        //
	TemplateNo               any        //
	VersionNo                any        //
	RevisionNo               any        //
	CommandIdempotencyDigest any        //
	CommandFingerprint       any        //
	SourceModeCode           any        //
	EncryptionKeyNo          any        //
	SourceObjectRef          any        //
	SourceSha256             any        //
	SourceMediaType          any        //
	SourceByteSize           any        //
	PdfObjectRef             *string    //
	PdfSha256                *string    //
	PdfByteSize              *int64     //
	PdfPageCount             any        //
	ConversionStatusCode     any        //
	ProviderFileRef          *string    //
	CreatedByUserId          any        //
	CreatedAt                *time.Time //
}
