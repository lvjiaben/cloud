// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceStructuredConfigurationSnapshot is the golang structure for table commerce_structured_configuration_snapshot.
type CommerceStructuredConfigurationSnapshot struct {
	Id                             int64      `json:"id"                             orm:"id"                               description:""`
	TenantId                       int        `json:"tenantId"                       orm:"tenant_id"                        description:""`
	SnapshotNo                     string     `json:"snapshotNo"                     orm:"snapshot_no"                      description:""`
	CustomerNo                     string     `json:"customerNo"                     orm:"customer_no"                      description:""`
	UploaderUserId                 int64      `json:"uploaderUserId"                 orm:"uploader_user_id"                 description:""`
	QuoteNo                        string     `json:"quoteNo"                        orm:"quote_no"                         description:""`
	QuoteItemNo                    string     `json:"quoteItemNo"                    orm:"quote_item_no"                    description:""`
	ProductNo                      string     `json:"productNo"                      orm:"product_no"                       description:""`
	VersionNo                      string     `json:"versionNo"                      orm:"version_no"                       description:""`
	ConfigurationRevision          string     `json:"configurationRevision"          orm:"configuration_revision"           description:""`
	InterfaceInstanceNo            string     `json:"interfaceInstanceNo"            orm:"interface_instance_no"            description:""`
	InterfaceConfigurationRevision int64      `json:"interfaceConfigurationRevision" orm:"interface_configuration_revision" description:""`
	CustomerDeclarationDigest      string     `json:"customerDeclarationDigest"      orm:"customer_declaration_digest"      description:""`
	CandidateSnapshotDigest        string     `json:"candidateSnapshotDigest"        orm:"candidate_snapshot_digest"        description:""`
	CustomerValueDigest            string     `json:"customerValueDigest"            orm:"customer_value_digest"            description:""`
	CandidateSnapshot              string     `json:"candidateSnapshot"              orm:"candidate_snapshot"               description:""`
	TypedValueSnapshot             string     `json:"typedValueSnapshot"             orm:"typed_value_snapshot"             description:""`
	ContentDigest                  string     `json:"contentDigest"                  orm:"content_digest"                   description:""`
	OrderNo                        string     `json:"orderNo"                        orm:"order_no"                         description:""`
	OrderItemNo                    string     `json:"orderItemNo"                    orm:"order_item_no"                    description:""`
	BoundAt                        *time.Time `json:"boundAt"                        orm:"bound_at"                         description:""`
	CreatedAt                      *time.Time `json:"createdAt"                      orm:"created_at"                       description:""`
	ServerLoginToken               string     `json:"serverLoginToken"               orm:"server_login_token"               description:""`
}
