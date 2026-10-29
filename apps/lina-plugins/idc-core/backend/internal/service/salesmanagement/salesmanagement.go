// Package salesmanagement owns administrator operations for agents, sales
// ownership and commissions. It uses governed host identity capabilities only.
package salesmanagement

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
)

// Service is the staff-only management surface. Unsupported data scopes are
// rejected before reading any merchant-wide configuration or customer record.
type Service interface {
	// GetLevelPricing returns independently versioned tier-wide pricing under pricing permission; nil means unset.
	GetLevelPricing(context.Context, string) (LevelPricing, error)
	// SaveLevelPricing appends the tier-wide percentage or clears it; stale versions fail and historical quotes remain unchanged.
	SaveLevelPricing(context.Context, string, *int, int64) (LevelPricing, error)
	// ListRules returns current rule revisions without changing financial records.
	ListRules(context.Context, PageInput) (CommissionRulePage, error)
	// SaveRule appends one rule version with transaction audit.
	SaveRule(context.Context, CommissionRuleInput) (CommissionRule, error)
	// ProductCandidates returns published products and their available currency-period combinations.
	ProductCandidates(context.Context, PageInput) (ProductPage, error)
	// StaffCandidates uses the governed host directory and validates active staff audience.
	StaffCandidates(context.Context, PageInput) (StaffPage, error)
	// ListOwnership returns current staff/referral relationships in a scoped page.
	ListOwnership(context.Context, PageInput) (OwnershipPage, error)
	// SaveOwnership preserves history and rejects stale versions.
	SaveOwnership(context.Context, OwnershipInput) (Ownership, error)
	// ListPrices returns current price revisions with joined display labels.
	ListPrices(context.Context, PageInput) (PricePage, error)
	// SavePrice appends an audited revision against a saved base-price term, including first-draft preparation; stale writes and unpriced terms are rejected.
	SavePrice(context.Context, PriceInput) (PriceRule, error)
	// DeletePrice appends a deletion revision, preserving all historical prices; stale versions fail.
	DeletePrice(context.Context, string, int64) (PriceRule, error)
	// PriceLevelCandidates lists tiers under price-management authorization.
	PriceLevelCandidates(context.Context, PageInput) (LevelPage, error)
	// PriceProductCandidates lists current published or first-draft base-price terms under pricing authorization; it does not enable draft sales.
	PriceProductCandidates(context.Context, PageInput) (ProductPage, error)
	// ListLevels returns a bounded page of named agent tiers; errors do not expose storage details.
	ListLevels(context.Context, PageInput) (LevelPage, error)
	// SaveLevel creates or version-checks one tier without altering historic prices.
	SaveLevel(context.Context, LevelInput) (Level, error)
	// ListAgents joins customer and tier display names in one bounded query.
	ListAgents(context.Context, PageInput) (AgentPage, error)
	// SaveAgent assigns or suspends a customer tier with a versioned history event.
	SaveAgent(context.Context, AgentInput) (Agent, error)
}

// PageInput constrains list size and search; page numbers start at one.
type PageInput struct {
	Page, PageSize                         int
	Search                                 string
	ProductNo, CustomerNo, LevelNo, Target string
}

// Level describes a merchant-owned tier. Version protects concurrent edits.
type Level struct {
	PercentageBasisPoints              *int
	PricingVersion                     int64
	LevelNo, Name, EnglishName, Status string
	Version                            int64
}

// LevelPage is a bounded, numbered page.
type LevelPage struct {
	Items []Level
	Total int
}

// LevelInput creates when LevelNo is empty; otherwise ExpectedVersion is required.
type LevelInput struct {
	LevelNo, Name, EnglishName, Status string
	ExpectedVersion                    int64
}

// Agent describes the current customer tier and its optional expiry.
type Agent struct {
	CustomerNo, CustomerCode, CustomerName, LevelNo, LevelName, Status string
	ExpiresAt                                                          *time.Time
	Version                                                            int64
}

// AgentPage is a bounded, numbered customer qualification page.
type AgentPage struct {
	Items []Agent
	Total int
}

// AgentInput changes a customer's tier. Zero expiry means no expiry.
type AgentInput struct {
	CustomerNo, LevelNo, Status string
	ExpiresAt                   *time.Time
	ExpectedVersion             int64
}

type serviceImpl struct {
	database      gdb.DB
	bizCtx        bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
	users         usercap.Service
	rules         saleschannel.Service
	audit         idccap.BusinessAuditWriter
}

var _ Service = (*serviceImpl)(nil)

// New receives shared owner capabilities; no host storage or credentials are used.
func New(database gdb.DB, bizCtx bizctxcap.Service, reader principal.Reader, authorization authz.Service, users usercap.Service, rules saleschannel.Service, audit idccap.BusinessAuditWriter) (Service, error) {
	if database == nil || bizCtx == nil || reader == nil || authorization == nil || users == nil || rules == nil || audit == nil {
		return nil, gerror.New("sales management requires database and identity capabilities")
	}
	return &serviceImpl{database: database, bizCtx: bizCtx, principal: reader, authorization: authorization, users: users, rules: rules, audit: audit}, nil
}
