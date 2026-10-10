// Package productdirectory owns merchant-defined navigation over purchased
// product entitlements. It never changes sales categories, fulfillment or money.
package productdirectory

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// Status is the merchant navigation availability, independent of service state.
type Status string

const (
	StatusEnabled  Status = "enabled"
	StatusDisabled Status = "disabled"
	// AllDirectory is the reserved customer fallback including unclassified services.
	AllDirectory     = "all"
	PermissionList   = "idc:admin:product-directory:list"
	PermissionCreate = "idc:admin:product-directory:create"
	PermissionUpdate = "idc:admin:product-directory:update"
)

// Access resolves trusted staff permissions or the current customer subject.
type Access interface {
	Admin(context.Context, string) (int, error)
	Customer(context.Context) (string, error)
}

// Product is the minimal batched display projection for bound products.
type Product struct{ ProductNo, NameZhCn, NameEnUs string }

// Directory is a merchant configuration projection with bounded product links.
type Directory struct {
	DirectoryNo, NameZhCn, NameEnUs string
	SortOrder                       int
	StatusCode                      Status
	Version                         int64
	ProductNos                      []string
	Products                        []Product
	ProductCount                    int
	ServiceCount                    int
}

// Page is the bounded directory result; Total enables complete navigation paging.
type Page struct {
	Items                 []Directory
	Total, Page, PageSize int
}

// SaveInput creates or replaces one directory's configuration; updates require
// the current version and membership conflicts never silently move a product.
type SaveInput struct {
	DirectoryNo, NameZhCn, NameEnUs string
	SortOrder                       int
	StatusCode                      Status
	ExpectedVersion                 int64
	ProductNos                      []string
}

// Entitlement is a customer-safe purchased service projection; timestamps are milliseconds.
type Entitlement struct {
	EntitlementNo, ProductNo, ProductNameZhCn, ProductNameEnUs, PluginId, StatusCode, OrderNo string
	StartedAt                                                                                 int64
	ExpiresAt                                                                                 *int64
}

// ServicePage projects only the current customer's purchased services.
type ServicePage struct {
	Items                 []Entitlement
	Total, Page, PageSize int
}

// Service keeps all mutations and reads behind trusted actor resolution.
type Service interface {
	// ListAdmin reads globally shared navigation for authorized staff with bounded pagination.
	ListAdmin(context.Context, int, int) (*Page, error)
	// Save checks staff permission, version and every linked product in one transaction.
	Save(context.Context, SaveInput) (*Directory, error)
	// ListAccount reads enabled merchant menus with current-customer scoped service counts.
	ListAccount(context.Context, int, int) (*Page, error)
	// ListServices pages the current customer's entitlements; all includes unclassified or disabled directories.
	ListServices(context.Context, string, int, int) (*ServicePage, error)
}
type serviceImpl struct {
	database gdb.DB
	access   Access
	audit    idccap.BusinessAuditWriter
}

var _ Service = (*serviceImpl)(nil)

// New injects shared database, actor resolution and audit persistence; no private runtime graph is created.
func New(database gdb.DB, access Access, writer idccap.BusinessAuditWriter) (Service, error) {
	if database == nil || access == nil || writer == nil {
		return nil, gerror.New("product directory dependencies are required")
	}
	return &serviceImpl{database: database, access: access, audit: writer}, nil
}
