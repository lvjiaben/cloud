// This file defines bounded merchant directory configuration endpoints.
package v1

import "github.com/gogf/gf/v2/frame/g"

// Directory does not grant access to a customer's purchased services.
type Directory struct {
	NavigationType string `json:"navigationType" dc:"Member destination: product, system, custom or plugin; default product" eg:"product"`
	TargetURL      string `json:"targetUrl" dc:"Internal page path for system or plugin; HTTP or HTTPS URL for custom; empty for product" eg:"/account/overview"`
	Icon           string `json:"icon" dc:"Iconify icon identifier used in member navigation" eg:"lucide:folder"`
	DirectoryNo    string `json:"directoryNo" dc:"Opaque merchant directory reference" eg:"pdr_example"`
	NameZhCn       string `json:"nameZhCn" dc:"Chinese directory name" eg:"My cloud services"`
	NameEnUs       string `json:"nameEnUs" dc:"English directory name" eg:"My cloud services"`
	SortOrder      int    `json:"sortOrder" dc:"Navigation order from 0 to 1000000" eg:"10"`
	StatusCode     string `json:"statusCode" dc:"Navigation availability: enabled or disabled" eg:"enabled"`
	Version        int64  `json:"version" dc:"Current optimistic configuration version" eg:"1"`
	ProductCount   int    `json:"productCount" dc:"Number of bound products" eg:"1"`
}
type ListReq struct {
	g.Meta   `path:"/admin/product-directories" method:"get" tags:"IDC Product Directories" summary:"List merchant product directories" dc:"Read bounded shared merchant navigation configuration" permission:"idc:admin:product-directory:list"`
	Page     int `json:"page" d:"1" v:"min:1|max:1000000" dc:"Page number; defaults to 1" eg:"1"`
	PageSize int `json:"pageSize" d:"20" v:"min:1|max:100" dc:"Page size from 1 to 100; defaults to 20" eg:"20"`
}
type ListRes struct {
	Items    []Directory `json:"items" dc:"Merchant directories on the current page" eg:"[]"`
	Total    int         `json:"total" dc:"Total matching directories" eg:"1"`
	Page     int         `json:"page" dc:"Current page number" eg:"1"`
	PageSize int         `json:"pageSize" dc:"Current page size" eg:"20"`
}
type CreateReq struct {
	NavigationType string `json:"navigationType" dc:"Member destination: product, system, custom or plugin; default product" eg:"product"`
	TargetURL      string `json:"targetUrl" dc:"Internal page path for system or plugin; HTTP or HTTPS URL for custom; empty for product" eg:"/account/overview"`
	Icon           string `json:"icon" dc:"Iconify icon identifier used in member navigation" eg:"lucide:folder"`
	g.Meta         `path:"/admin/product-directories" method:"post" tags:"IDC Product Directories" summary:"Create a merchant product directory" dc:"Create enabled navigation without assigning products; assignment is managed by product editing" permission:"idc:admin:product-directory:create"`
	NameZhCn       string `json:"nameZhCn" v:"required|max-length:128" dc:"Chinese directory name" eg:"My cloud services"`
	NameEnUs       string `json:"nameEnUs" v:"required|max-length:128" dc:"English directory name" eg:"My cloud services"`
	SortOrder      int    `json:"sortOrder" v:"min:0|max:1000000" dc:"Navigation order from 0 to 1000000" eg:"10"`
}
type CreateRes struct {
	Directory *Directory `json:"directory" dc:"Created merchant directory" eg:"{}"`
}
type UpdateReq struct {
	NavigationType  string `json:"navigationType" dc:"Member destination: product, system, custom or plugin; default product" eg:"product"`
	TargetURL       string `json:"targetUrl" dc:"Internal page path for system or plugin; HTTP or HTTPS URL for custom; empty for product" eg:"/account/overview"`
	Icon            string `json:"icon" dc:"Iconify icon identifier used in member navigation" eg:"lucide:folder"`
	g.Meta          `path:"/admin/product-directories/{directoryNo}" method:"put" tags:"IDC Product Directories" summary:"Update a merchant product directory" dc:"Update navigation metadata at the expected version, preserving all product assignments" permission:"idc:admin:product-directory:update"`
	DirectoryNo     string `json:"directoryNo" in:"path" v:"required|max-length:40" dc:"Opaque merchant directory reference" eg:"pdr_example"`
	NameZhCn        string `json:"nameZhCn" v:"required|max-length:128" dc:"Chinese directory name" eg:"My cloud services"`
	NameEnUs        string `json:"nameEnUs" v:"required|max-length:128" dc:"English directory name" eg:"My cloud services"`
	SortOrder       int    `json:"sortOrder" v:"min:0|max:1000000" dc:"Navigation order from 0 to 1000000" eg:"10"`
	StatusCode      string `json:"statusCode" v:"required|in:enabled,disabled" dc:"Navigation availability: enabled or disabled" eg:"disabled"`
	ExpectedVersion int64  `json:"expectedVersion" v:"required|min:1" dc:"Expected optimistic configuration version" eg:"1"`
}
type UpdateRes struct {
	Directory *Directory `json:"directory" dc:"Updated merchant directory" eg:"{}"`
}
