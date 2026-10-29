// Package customermanagement provides bounded, staff-only customer read models.
package customermanagement

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/customer360summary"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// Config bounds customer management pagination and serialized responses.
type Config struct {
	MaximumPageSize      int
	MaximumResponseBytes int
}

type cursorBoundary struct {
	CreatedAt  time.Time
	InternalID int64
	CustomerNo string
}

// SearchInput applies indexed, bounded filters to the management customer list.
type SearchInput struct {
	CustomerCode string
	NamePrefix   string
	Phone        string
	Type         customer.Type
	Status       customer.Status
	GroupNo      string
	TagNo        string
	Cursor       string
	Page         int
	Limit        int
}

// Summary is the non-legal, non-credential customer list projection.
type Summary struct {
	Balances        []BalanceSummary
	CustomerNo      string
	CustomerCode    string
	PrimaryEmail    string
	PrimaryPhone    string
	Type            customer.Type
	Status          customer.Status
	Verification    customer.VerificationStatus
	DisplayName     string
	CountryCode     string
	PreferredLocale string

	Level     *LevelSummary
	TagCount  int
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LevelSummary is the display-safe current customer-level projection.
type LevelSummary struct {
	LevelNo  string
	Code     string
	NameZhCn string
	NameEnUs string
	Priority int
}

// Page is one bounded keyset page. A nil Next cursor means no later rows exist.
type Page struct {
	Customers  []Summary
	NextCursor string
	Total      int
}

// SegmentSummary is one display-safe merchant-defined group or tag.
type SegmentSummary struct {
	Number     string
	Code       string
	NameZhCn   string
	NameEnUs   string
	ColorToken string
	Status     string
}

// SegmentPreview is a bounded customer-assignment slice. Full assignment lists
// require a separate customer-scoped pagination path from the catalog search.
type SegmentPreview struct {
	Items   []SegmentSummary
	HasMore bool
}

// SegmentKind selects one merchant-defined customer segmentation catalog.
type SegmentKind string

const (
	SegmentKindGroup SegmentKind = "group"
	SegmentKindTag   SegmentKind = "tag"
)

// SegmentLocale selects the indexed localized name used for prefix filtering and ordering.
type SegmentLocale string

const (
	SegmentLocaleZhCN SegmentLocale = "zh-CN"
	SegmentLocaleEnUS SegmentLocale = "en-US"
)

// SegmentSearchInput applies bounded, indexed filters to active segment definitions.
type SegmentSearchInput struct {
	Kind       SegmentKind
	Locale     SegmentLocale
	NamePrefix string
	Cursor     string
	Limit      int
}

// SegmentPage is one active group or tag keyset page.
type SegmentPage struct {
	Items      []SegmentSummary
	NextCursor string
}

// AssignmentSearchInput pages the complete group or tag assignments of one
// customer. It is separate from the merchant-wide segment catalog search.
type AssignmentSearchInput struct {
	CustomerNo string
	Kind       SegmentKind
	Locale     SegmentLocale
	Cursor     string
	Limit      int
}

type AssignmentPage struct {
	Items      []SegmentSummary
	NextCursor string
}

type ActivityKind string

const (
	ActivityKindNote      ActivityKind = "note"
	ActivityKindFollowUp  ActivityKind = "follow_up"
	ActivityKindBlacklist ActivityKind = "blacklist"
)

type ActivitySearchInput struct {
	CustomerNo string
	Kind       ActivityKind
	Cursor     string
	Limit      int
}

type ActivityItem struct {
	Number    string
	Kind      ActivityKind
	Category  string
	Status    string
	Title     string
	Detail    string
	RiskLevel string
	PlannedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ActivityPage struct {
	Items      []ActivityItem
	NextCursor string
}

// OverviewCounts contains local customer-domain counts only. Cross-domain
// commerce and resource summaries are loaded by their owning capabilities.
type OverviewCounts struct {
	ActiveContacts       int64
	ActiveAddresses      int64
	ActiveCustomFields   int64
	ActiveNotes          int64
	PlannedFollowUps     int64
	AvailableAttachments int64
	ActiveBlacklist      int64
	OpenRisks            int64
	ActiveGroups         int64
	ActiveTags           int64
}

// BusinessSummary contains display-safe counts supplied by each owning domain.
type BusinessSummary struct {
	Orders       int64
	Bills        int64
	Transactions int64
	Servers      int64

	TaxInvoices   int64
	Tickets       int64
	Notifications int64
	Audits        int64
}

type BalanceSummary struct {
	AccountTypeCode       string
	CurrencyCode          string
	StatusCode            string
	BalanceMinor          int64
	HeldBalanceMinor      int64
	AvailableBalanceMinor int64
	UpdatedAt             time.Time
}

// Overview is the bounded local foundation of the management customer 360.
type Overview struct {
	Statistics OperationsStatistics
	Customer   Summary
	Groups     SegmentPreview
	Tags       SegmentPreview
	Counts     OverviewCounts
	Business   BusinessSummary
	Balances   []BalanceSummary
}

// Service defines management customer reads independently from customer self-service.
type Service interface {
	// SearchProducts returns all order-backed product types for one visible customer.
	SearchProducts(context.Context, string, int, int) (*ProductPage, error)
	// Search requires an existing direct staff audience, exact host-governed
	// list authority, and a data scope representable by the customer query.
	// Unsupported or ambiguous scopes fail before persistence access.
	Search(ctx context.Context, input SearchInput) (*Page, error)
	// GetOverview returns only local customer-domain summaries and requires a
	// direct staff audience plus exact query authority before validating the target.
	GetOverview(ctx context.Context, customerNo string) (*Overview, error)
	// SearchSegments returns active group or tag candidates for management filters.
	SearchSegments(ctx context.Context, input SegmentSearchInput) (*SegmentPage, error)
	// SearchAssignments returns the complete active assignments of one customer.
	SearchAssignments(ctx context.Context, input AssignmentSearchInput) (*AssignmentPage, error)
	SearchActivities(ctx context.Context, input ActivitySearchInput) (*ActivityPage, error)
}

type persistence interface {
	search(ctx context.Context, input normalizedSearch) (*Page, error)
	getOverview(ctx context.Context, customerNo string) (*Overview, error)
	searchSegments(ctx context.Context, input normalizedSegmentSearch) (*SegmentPage, error)
}

type serviceImpl struct {
	bizCtxSvc     bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
	persistence   persistence
	business      customer360summary.Reader
	config        Config
}

var _ Service = (*serviceImpl)(nil)

// New creates the staff-only customer reader from host context, persisted
// audience, and authorization services. All runtime dependencies are explicit.
func New(
	bizCtxSvc bizctxcap.Service,
	principalSvc principal.Reader,
	authorization authz.Service,
	config Config,
) (Service, error) {
	return newService(bizCtxSvc, principalSvc, authorization, newDAOPersistence(), config)
}

// NewWithBusinessSummary creates the production reader with its frozen
// cross-domain summary graph.
func NewWithBusinessSummary(
	bizCtxSvc bizctxcap.Service,
	principalSvc principal.Reader,
	authorization authz.Service,
	business customer360summary.Reader,
	config Config,
) (Service, error) {
	if nilInterface(business) {
		return nil, gerror.New("customer management reader requires business summary service")
	}
	service, err := newService(bizCtxSvc, principalSvc, authorization, newDAOPersistence(), config)
	if err != nil {
		return nil, err
	}
	implementation, ok := service.(*serviceImpl)
	if !ok {
		return nil, gerror.New("customer management reader composition is invalid")
	}
	implementation.business = business
	return implementation, nil
}
