// This file defines same-system purchase-file uploads for the API-bound buyer.
package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

type PurchaseFileReserveReq struct {
	g.Meta                `path:"/channel/catalog/products/{publicSlug}/file-uploads" method:"post" tags:"Open API" summary:"Reserve a purchase configuration file" dc:"Reserve a private file for the API-bound customer and exact configuration field; requires order.create."`
	PublicSlug            string `json:"publicSlug" v:"required|max-length:160" dc:"Published product slug"`
	ConfigurationRevision string `json:"configurationRevision" v:"required|max-length:4096" dc:"Exact upstream configuration revision"`
	FieldToken            string `json:"fieldToken" v:"required|max-length:4096" dc:"Opaque upstream file field token"`
	FileName              string `json:"fileName" v:"required|max-length:255" dc:"Original safe file name"`
	SizeBytes             int64  `json:"sizeBytes" v:"required|min:1|max:5368709120" dc:"Exact file size in bytes"`
}
type PurchaseFileReserveRes struct {
	channelcommercecap.PurchaseFileReservation
}

type PurchaseFileUploadReq struct {
	g.Meta      `path:"/channel/file-uploads/{uploadToken}/content" method:"put" tags:"Open API" summary:"Upload purchase configuration file content" dc:"Stream raw file bytes to an API-customer-owned reservation; requires order.create. Only inspected ready files can be quoted or ordered."`
	UploadToken string `json:"uploadToken" v:"required|max-length:4096" dc:"Opaque upload reservation token"`
}
type PurchaseFileUploadRes struct {
	channelcommercecap.PurchaseFile
}
