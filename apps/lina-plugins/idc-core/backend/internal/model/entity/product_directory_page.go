// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

// ProductDirectoryPage is the golang structure for table product_directory_page.
type ProductDirectoryPage struct {
	TenantId    int64  `json:"tenantId"    orm:"tenant_id"    description:""`
	DirectoryNo string `json:"directoryNo" orm:"directory_no" description:""`
	PluginId    string `json:"pluginId"    orm:"plugin_id"    description:""`
	SortOrder   int    `json:"sortOrder"   orm:"sort_order"   description:""`
}
