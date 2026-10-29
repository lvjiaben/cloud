// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceConfigurationSnapshot is the golang structure for table commerce_configuration_snapshot.
type CommerceConfigurationSnapshot struct {
	Id                      int64      `json:"id"                      orm:"id"                        description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	ConfigurationSnapshotNo string     `json:"configurationSnapshotNo" orm:"configuration_snapshot_no" description:""`
	ProductNo               string     `json:"productNo"               orm:"product_no"                description:""`
	VersionNo               string     `json:"versionNo"               orm:"version_no"                description:""`
	SourceRevisionDigest    string     `json:"sourceRevisionDigest"    orm:"source_revision_digest"    description:""`
	SelectionDigest         string     `json:"selectionDigest"         orm:"selection_digest"          description:""`
	SelectedOptionCount     int        `json:"selectedOptionCount"     orm:"selected_option_count"     description:""`
	TotalSelectedQuantity   int64      `json:"totalSelectedQuantity"   orm:"total_selected_quantity"   description:""`
	ContentDigest           string     `json:"contentDigest"           orm:"content_digest"            description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
	CustomerValueDigest     string     `json:"customerValueDigest"     orm:"customer_value_digest"     description:""`
	TypedValueSnapshot      string     `json:"typedValueSnapshot"      orm:"typed_value_snapshot"      description:"Compiler-produced customer values for native imported pricing; NULL for historical snapshots"`
	ServerLoginToken        string     `json:"serverLoginToken"        orm:"server_login_token"        description:""`
}
