// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractTemplateContent is the golang structure for table contract_template_content.
type ContractTemplateContent struct {
	Id                       int64      `json:"id"                       orm:"id"                         description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                  description:""`
	ContentRevisionNo        string     `json:"contentRevisionNo"        orm:"content_revision_no"        description:""`
	TemplateNo               string     `json:"templateNo"               orm:"template_no"                description:""`
	VersionNo                int64      `json:"versionNo"                orm:"version_no"                 description:""`
	RevisionNo               int64      `json:"revisionNo"               orm:"revision_no"                description:""`
	CommandIdempotencyDigest string     `json:"commandIdempotencyDigest" orm:"command_idempotency_digest" description:""`
	CommandFingerprint       string     `json:"commandFingerprint"       orm:"command_fingerprint"        description:""`
	SourceModeCode           string     `json:"sourceModeCode"           orm:"source_mode_code"           description:""`
	EncryptionKeyNo          string     `json:"encryptionKeyNo"          orm:"encryption_key_no"          description:""`
	SourceObjectRef          string     `json:"sourceObjectRef"          orm:"source_object_ref"          description:""`
	SourceSha256             string     `json:"sourceSha256"             orm:"source_sha256"              description:""`
	SourceMediaType          string     `json:"sourceMediaType"          orm:"source_media_type"          description:""`
	SourceByteSize           int64      `json:"sourceByteSize"           orm:"source_byte_size"           description:""`
	PdfObjectRef             *string    `json:"pdfObjectRef"             orm:"pdf_object_ref"             description:""`
	PdfSha256                *string    `json:"pdfSha256"                orm:"pdf_sha256"                 description:""`
	PdfByteSize              *int64     `json:"pdfByteSize"              orm:"pdf_byte_size"              description:""`
	PdfPageCount             int        `json:"pdfPageCount"             orm:"pdf_page_count"             description:""`
	ConversionStatusCode     string     `json:"conversionStatusCode"     orm:"conversion_status_code"     description:""`
	ProviderFileRef          *string    `json:"providerFileRef"          orm:"provider_file_ref"          description:""`
	CreatedByUserId          int        `json:"createdByUserId"          orm:"created_by_user_id"         description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                 description:""`
}
