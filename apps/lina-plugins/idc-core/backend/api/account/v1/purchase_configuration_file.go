// This file defines authenticated private upload endpoints for governed
// customer purchase-configuration file fields.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PurchaseConfigurationFileUploadCreateReq struct {
	g.Meta                `path:"/account/catalog/products/{publicSlug}/purchase-configuration/file-uploads" method:"post" tags:"IDC Customer Catalog" summary:"Reserve a governed purchase file upload" dc:"Reserve one private upload for an exact published customer configuration field. The caller must be an active customer and no storage address is returned."`
	PublicSlug            string `json:"publicSlug" in:"path" v:"required|length:1,128" dc:"Public product slug" eg:"ssl-certificate"`
	ConfigurationRevision string `json:"configurationRevision" v:"required|max-length:128" dc:"Opaque published configuration revision" eg:"Q1JfYWJj"`
	FieldToken            string `json:"fieldToken" v:"required|max-length:256" dc:"Opaque file field token from the same revision" eg:"RkZfYWJj"`
	FileName              string `json:"fileName" v:"required|max-length:255" dc:"Original file name including an allowed extension" eg:"identity.pdf"`
	SizeBytes             int64  `json:"sizeBytes" v:"required|min:1" dc:"Declared file size in bytes; the frozen field policy supplies the upper bound" eg:"2048"`
}

type PurchaseConfigurationFileUploadCreateRes struct {
	UploadToken string `json:"uploadToken" dc:"Opaque private upload token bound to the exact customer login, product revision, and field" eg:"CFU_0123456789ABCDEF0123456789ABCDEF"`
	ExpiresAt   int64  `json:"expiresAt" dc:"Reservation expiry as Unix timestamp in milliseconds" eg:"1720000000000"`
}

type PurchaseConfigurationFileUploadContentUpdateReq struct {
	g.Meta      `path:"/account/catalog/purchase-configuration/file-uploads/{uploadToken}/content" method:"put" mime:"application/octet-stream" tags:"IDC Customer Catalog" summary:"Upload governed purchase file content" dc:"Upload the exact reserved bytes through the host. Ownership, actual type, size, extension, and the frozen scan policy are rechecked before the token becomes usable."`
	UploadToken string `json:"uploadToken" in:"path" v:"required|length:36,36" dc:"Opaque upload token returned by reservation" eg:"CFU_0123456789ABCDEF0123456789ABCDEF"`
}

type PurchaseConfigurationFileUploadContentUpdateRes struct {
	FileToken string `json:"fileToken" dc:"Opaque usable file token; it is valid only for the exact uploading login and frozen field" eg:"CFU_0123456789ABCDEF0123456789ABCDEF"`
	FileName  string `json:"fileName" dc:"Inspected safe file name" eg:"identity.pdf"`
	MediaType string `json:"mediaType" dc:"Server-inspected MIME type" eg:"application/pdf"`
	SizeBytes int64  `json:"sizeBytes" dc:"Server-confirmed file size in bytes" eg:"2048"`
	Status    string `json:"status" dc:"Safe upload state: ready" eg:"ready"`
	ExpiresAt int64  `json:"expiresAt" dc:"Usable token expiry as Unix timestamp in milliseconds" eg:"1720000000000"`
}

type PurchaseConfigurationFileUploadDetailReq struct {
	g.Meta      `path:"/account/catalog/purchase-configuration/file-uploads/{uploadToken}" method:"get" tags:"IDC Customer Catalog" summary:"Read my governed purchase file upload" dc:"Return safe metadata only when the exact current customer login owns the upload token."`
	UploadToken string `json:"uploadToken" in:"path" v:"required|length:36,36" dc:"Opaque upload token" eg:"CFU_0123456789ABCDEF0123456789ABCDEF"`
}

type PurchaseConfigurationFileUploadDetailRes = PurchaseConfigurationFileUploadContentUpdateRes

type PurchaseConfigurationFileUploadRevokeReq struct {
	g.Meta      `path:"/account/catalog/purchase-configuration/file-uploads/{uploadToken}" method:"delete" tags:"IDC Customer Catalog" summary:"Revoke my governed purchase file upload" dc:"Immediately prevent a reserved or ready upload token owned by the exact current login from being used by a new order. Permanent evidence already bound to an order cannot be revoked."`
	UploadToken string `json:"uploadToken" in:"path" v:"required|length:36,36" dc:"Opaque upload token owned by the exact current login" eg:"CFU_0123456789ABCDEF0123456789ABCDEF"`
}

type PurchaseConfigurationFileUploadRevokeRes = PurchaseConfigurationFileUploadContentUpdateRes
