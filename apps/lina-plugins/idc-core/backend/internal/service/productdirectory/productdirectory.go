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

// NavigationType identifies the customer destination; only product menus bind products.
type NavigationType string

const (
	NavigationProduct NavigationType = "product"
	NavigationSystem  NavigationType = "system"
	NavigationCustom  NavigationType = "custom"
	NavigationPlugin  NavigationType = "plugin"
)

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

// Directory is a merchant configuration projection with aggregate product counts.
type Directory struct {
	PagePluginIDs                      []string
	NavigationType                     NavigationType
	TargetURL, Icon, ParentDirectoryNo string
	DirectoryNo, NameZhCn, NameEnUs    string
	SortOrder                          int
	StatusCode                         Status
	Version                            int64
	ProductCount                       int
	ServiceCount                       int
}

// Page is the bounded directory result; Total enables complete navigation paging.
type Page struct {
	Items                 []Directory
	Parents               []Directory
	Total, Page, PageSize int
}

// SaveInput writes navigation metadata only; product assignment belongs to catalog editing.
type SaveInput struct {
	// Nil preserves existing block configuration; an explicit empty list clears it.
	PagePluginIDs                      *[]string
	NavigationType                     NavigationType
	TargetURL, Icon, ParentDirectoryNo string
	DirectoryNo, NameZhCn, NameEnUs    string
	SortOrder                          int
	StatusCode                         Status
	ExpectedVersion                    int64
}

// Entitlement is a customer-safe purchased service projection; timestamps are milliseconds.
type Entitlement struct {
	EntitlementNo, ProductNo, ProductNameZhCn, ProductNameEnUs, PluginId, StatusCode, OrderNo string
	EntitlementOwnerID                                                                        string
	ManagementPreparation                                                                     string
	RequiresManualProcessing                                                                  bool
	StartedAt                                                                                 int64
	ExpiresAt                                                                                 *int64
	CustomerRemark                                                                            string
	CustomerRemarkRevision                                                                    int64
	CurrentPriceAmountMinor                                                                   *string
	CurrencyCode, PeriodUnitCode                                                              string
	PeriodCount, ChargeCycles                                                                 int
}

// CustomerNote is customer-owned metadata, independent of paid terms and lifecycle versions.
type CustomerNote struct {
	SupportAssociationReference string
	EntitlementNo               string
	Remark                      string
	Revision                    int64
}

// ServiceProvider identifies one plugin-owned purchased-services content block.
type ServiceProvider struct {
	PluginID string
	Total    int
}

// ServicePage projects only the current customer's purchased services.
type ServicePage struct {
	Providers             []ServiceProvider
	Items                 []Entitlement
	Total, Page, PageSize int
}

// Service keeps all mutations and reads behind trusted actor resolution.
type Service interface {
	// ListAdmin reads globally shared navigation for authorized staff with bounded pagination.
	ListAdmin(context.Context, int, int) (*Page, error)
	// ListAdminTree pages menu rows in tree order and includes at most one ancestor
	// per row as context; Total counts matching rows, not repeated ancestors.
	ListAdminTree(context.Context, int, int) (*Page, error)
	// Save checks staff permission and version, preserving product assignments.
	Save(context.Context, SaveInput) (*Directory, error)
	// ListAccount reads enabled merchant menus with customer-scoped counts. An empty
	// type includes all types; a recognized type filters rows and totals before paging.
	ListAccount(context.Context, int, int, NavigationType) (*Page, error)
	// ListServices pages the current customer's entitlements; all includes unclassified or disabled directories.
	ListServices(context.Context, string, int, int, ...string) (*ServicePage, error)
	// ListProviderServices applies the same paid customer boundary and narrows one plugin-owned content block before paging. Empty provider includes all blocks.
	ListProviderServices(context.Context, string, int, int, string, string) (*ServicePage, error)
	// GetNote reads only the authenticated customer's exact entitlement; missing and foreign targets are indistinguishable.
	GetNote(context.Context, string) (*CustomerNote, error)
	// SaveNote updates a non-sensitive note after exact ownership and independent revision checks, with transactional audit.
	SaveNote(context.Context, CustomerNote) (*CustomerNote, error)
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
