// This file defines private staff upload endpoints for governed configuration files.
package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogConfigurationFileReserveBody struct {
	FieldKey       string `json:"fieldKey" v:"required|max-length:64" dc:"Declared file field key"`
	FileName       string `json:"fileName" v:"required|max-length:255" dc:"Original file name"`
	SizeBytes      int64  `json:"sizeBytes" v:"required|min:1" dc:"Declared byte size"`
	TargetRevision int64  `json:"targetRevision" v:"min:0" dc:"Current interface revision or product configuration version"`
}
type CatalogConfigurationFileTargetQuery struct {
	TargetRevision int64 `json:"targetRevision" in:"query" v:"min:0" dc:"Current interface revision or product configuration version"`
}
type CatalogConfigurationFileRes struct {
	FileToken string `json:"fileToken" dc:"Opaque governed file token"`
	FileName  string `json:"fileName" dc:"Safe file name"`
	MediaType string `json:"mediaType" dc:"Server inspected MIME type"`
	SizeBytes int64  `json:"sizeBytes" dc:"Server confirmed size"`
	Status    string `json:"status" dc:"Safe upload status"`
	ExpiresAt int64  `json:"expiresAt" dc:"Expiry in Unix milliseconds"`
}

type CatalogAutomationInterfaceConfigurationFileReserveReq struct {
	g.Meta              `path:"/admin/catalog/automation/interfaces/{interfaceInstanceNo}/configuration/file-uploads" method:"post" tags:"IDC Catalog Management" summary:"Reserve interface configuration file" permission:"idc:admin:catalog:automation-interface:manage"`
	InterfaceInstanceNo string `json:"interfaceInstanceNo" in:"path" v:"required|regex:^pfi_[0-9a-f]{32}$"`
	CatalogConfigurationFileReserveBody
}
type CatalogAutomationInterfaceConfigurationFileReserveRes = CatalogConfigurationFileRes
type CatalogAutomationInterfaceConfigurationFileUploadReq struct {
	g.Meta              `path:"/admin/catalog/automation/interfaces/{interfaceInstanceNo}/configuration/file-uploads/{uploadToken}/content" method:"put" mime:"application/octet-stream" tags:"IDC Catalog Management" summary:"Upload interface configuration file" permission:"idc:admin:catalog:automation-interface:manage"`
	InterfaceInstanceNo string `json:"interfaceInstanceNo" in:"path" v:"required|regex:^pfi_[0-9a-f]{32}$"`
	UploadToken         string `json:"uploadToken" in:"path" v:"required|length:36,36"`
	CatalogConfigurationFileTargetQuery
}
type CatalogAutomationInterfaceConfigurationFileUploadRes = CatalogConfigurationFileRes
type CatalogAutomationInterfaceConfigurationFileDetailReq struct {
	g.Meta              `path:"/admin/catalog/automation/interfaces/{interfaceInstanceNo}/configuration/file-uploads/{uploadToken}" method:"get" tags:"IDC Catalog Management" summary:"Read interface configuration file" permission:"idc:admin:catalog:automation-interface:manage"`
	InterfaceInstanceNo string `json:"interfaceInstanceNo" in:"path" v:"required|regex:^pfi_[0-9a-f]{32}$"`
	UploadToken         string `json:"uploadToken" in:"path" v:"required|length:36,36"`
	CatalogConfigurationFileTargetQuery
}
type CatalogAutomationInterfaceConfigurationFileDetailRes = CatalogConfigurationFileRes
type CatalogAutomationInterfaceConfigurationFileRevokeReq struct {
	g.Meta              `path:"/admin/catalog/automation/interfaces/{interfaceInstanceNo}/configuration/file-uploads/{uploadToken}" method:"delete" tags:"IDC Catalog Management" summary:"Revoke interface configuration file" permission:"idc:admin:catalog:automation-interface:manage"`
	InterfaceInstanceNo string `json:"interfaceInstanceNo" in:"path" v:"required|regex:^pfi_[0-9a-f]{32}$"`
	UploadToken         string `json:"uploadToken" in:"path" v:"required|length:36,36"`
	CatalogConfigurationFileTargetQuery
}
type CatalogAutomationInterfaceConfigurationFileRevokeRes = CatalogConfigurationFileRes

type CatalogProductVersionConfigurationFileReserveReq struct {
	g.Meta    `path:"/admin/catalog/product-versions/{versionNo}/configuration/file-uploads" method:"post" tags:"IDC Catalog Management" summary:"Reserve product configuration file" permission:"idc:admin:catalog:configuration:write"`
	VersionNo string `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$"`
	CatalogConfigurationFileReserveBody
}
type CatalogProductVersionConfigurationFileReserveRes = CatalogConfigurationFileRes
type CatalogProductVersionConfigurationFileUploadReq struct {
	g.Meta      `path:"/admin/catalog/product-versions/{versionNo}/configuration/file-uploads/{uploadToken}/content" method:"put" mime:"application/octet-stream" tags:"IDC Catalog Management" summary:"Upload product configuration file" permission:"idc:admin:catalog:configuration:write"`
	VersionNo   string `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$"`
	UploadToken string `json:"uploadToken" in:"path" v:"required|length:36,36"`
	CatalogConfigurationFileTargetQuery
}
type CatalogProductVersionConfigurationFileUploadRes = CatalogConfigurationFileRes
type CatalogProductVersionConfigurationFileDetailReq struct {
	g.Meta      `path:"/admin/catalog/product-versions/{versionNo}/configuration/file-uploads/{uploadToken}" method:"get" tags:"IDC Catalog Management" summary:"Read product configuration file" permission:"idc:admin:catalog:configuration:write"`
	VersionNo   string `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$"`
	UploadToken string `json:"uploadToken" in:"path" v:"required|length:36,36"`
	CatalogConfigurationFileTargetQuery
}
type CatalogProductVersionConfigurationFileDetailRes = CatalogConfigurationFileRes
type CatalogProductVersionConfigurationFileRevokeReq struct {
	g.Meta      `path:"/admin/catalog/product-versions/{versionNo}/configuration/file-uploads/{uploadToken}" method:"delete" tags:"IDC Catalog Management" summary:"Revoke product configuration file" permission:"idc:admin:catalog:configuration:write"`
	VersionNo   string `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$"`
	UploadToken string `json:"uploadToken" in:"path" v:"required|length:36,36"`
	CatalogConfigurationFileTargetQuery
}
type CatalogProductVersionConfigurationFileRevokeRes = CatalogConfigurationFileRes
