// This file defines the anonymous, bounded public product catalog projection.

package catalog

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

const (
	defaultPublicCatalogPageSize      = 12
	maximumPublicCatalogPageSize      = 24
	maximumPublicCatalogResponseBytes = 1024 * 1024
	publicCatalogEnvelopeReserve      = 4096
	maximumPublicCatalogCategoryCount = 500
)

var publicCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// PublicCatalogSearchInput contains only stable, anonymous catalog filters.
type PublicCatalogSearchInput struct {
	PublicSlugs []string
	NamePrefix  string
	CategoryNo  string
	Currency    string
	Cursor      string
	Limit       int
}

// PublicCatalogProduct is safe for anonymous visitors.
type PublicCatalogProduct struct {
	PublicSlug       string
	CategoryNo       string
	CategoryNameZhCN string
	CategoryNameEnUS string
	NameZhCN         string
	NameEnUS         string
	SummaryZhCN      string
	SummaryEnUS      string
	Currencies       []string
	Availability     string
}

// PublicCatalogCategory is an enabled anonymous filter choice.
type PublicCatalogCategory struct {
	CategoryNo string
	NameZhCN   string
	NameEnUS   string
}

// PublicCatalogPage is one bounded list response and its fixed filter choices.
type PublicCatalogPage struct {
	Products            []PublicCatalogProduct
	Categories          []PublicCatalogCategory
	AvailableCurrencies []string
	NextCursor          string
}

// PublicCatalogReader exposes only published, local PostgreSQL projections.
type PublicCatalogReader interface {
	Search(ctx context.Context, input PublicCatalogSearchInput) (*PublicCatalogPage, error)
	GetBySlug(ctx context.Context, publicSlug string) (*PublicCatalogProduct, error)
}

// LegacyServerCatalogReader is the private bridge for server configuration
// flows that have not yet moved into the server automation plugin contract.
// It must never be exposed as the anonymous generic catalog HTTP contract.
type LegacyServerCatalogReader interface {
	GetConfiguration(ctx context.Context, publicSlug string) (*PublicConfigurationGraph, error)
	ValidateConfiguration(
		ctx context.Context,
		input PublicConfigurationValidationInput,
	) (*PublicConfigurationValidation, error)
	QuoteConfiguration(
		ctx context.Context,
		input PublicConfigurationQuoteInput,
	) (*PublicConfigurationQuote, error)
}

// CatalogReader is the composition-time implementation contract. Public HTTP
// controllers deliberately receive only PublicCatalogReader.
type CatalogReader interface {
	PublicCatalogReader
	// SearchNumbered returns a bounded supplier selection page with an exact visible total.
	SearchNumbered(context.Context, PublicCatalogNumberedInput) (*PublicCatalogNumberedPage, error)
	PublicStructuredConfigurationReader
	LegacyServerCatalogReader
	ConfigurationPriceSelectionReader
	CatalogImportReader
	// TrafficPackageProducts returns bounded server-product choices for an already authorized installation-level accessory configuration.
	TrafficPackageProducts(context.Context, string, []string) ([]TrafficPackageProduct, error)
}

// PublicConfigurationCacheMetrics is an adapter-ready observation snapshot.
type PublicConfigurationCacheMetrics struct {
	Hits        uint64
	Misses      uint64
	Writes      uint64
	Evictions   uint64
	Corruptions uint64
}

// PublicConfigurationCacheObserver exposes diagnostics without expanding the
// anonymous catalog API or making cache state authoritative.
type PublicConfigurationCacheObserver interface {
	PublicConfigurationCacheMetrics() PublicConfigurationCacheMetrics
}

type publicCatalogReaderImpl struct {
	currencies               CurrencyReader
	database                 gdb.DB
	cursor                   *publicCatalogCursorCodec
	revision                 *publicConfigurationRevisionCodec
	config                   *publicConfigurationCache
	newBusinessAvailability  PublicPurchaseNewBusinessAvailability
	dynamicCandidateResolver PublicPurchaseDynamicCandidateResolver
	purchaseFiles            purchaseconfigurationfile.Service
	billingLabels            PublicBillingLabelResources
}

var _ PublicCatalogReader = (*publicCatalogReaderImpl)(nil)
var _ LegacyServerCatalogReader = (*publicCatalogReaderImpl)(nil)
var _ PublicConfigurationCacheObserver = (*publicCatalogReaderImpl)(nil)

// PublicConfigurationCacheMetrics returns a lock-free point-in-time snapshot.
func (s *publicCatalogReaderImpl) PublicConfigurationCacheMetrics() PublicConfigurationCacheMetrics {
	if s == nil {
		return PublicConfigurationCacheMetrics{}
	}
	return s.config.snapshotMetrics()
}

// NewPublicCatalogReader creates the anonymous read model with a distinct cursor purpose.
func NewPublicCatalogReader(database gdb.DB, cursorKey []byte) (CatalogReader, error) {
	return newPublicCatalogReader(database, cursorKey, nil, nil, nil)
}

// NewPublicCatalogReaderConfigured creates the public reader with the
// authoritative automation admission check required by generic purchasing.
func NewPublicCatalogReaderConfigured(
	database gdb.DB,
	cursorKey []byte,
	runtime PublicPurchaseConfigurationRuntime,
	purchaseFiles purchaseconfigurationfile.Service,
) (CatalogReader, error) {
	if runtime == nil || purchaseFiles == nil {
		return nil, gerror.New("public catalog reader requires purchase configuration runtime and governed file verification")
	}
	return newPublicCatalogReader(database, cursorKey, runtime, runtime, purchaseFiles)
}

// NewPublicPurchaseFileTargetResolver creates the non-HTTP resolver used
// before the final public reader is assembled. It shares the same opaque-token
// codec and published-version checks but cannot verify or store file content.
func NewPublicPurchaseFileTargetResolver(
	database gdb.DB,
	cursorKey []byte,
	runtime PublicPurchaseConfigurationRuntime,
) (purchaseconfigurationfile.TargetResolver, error) {
	if runtime == nil {
		return nil, gerror.New("purchase file target resolver requires purchase configuration runtime")
	}
	reader, err := newPublicCatalogReader(database, cursorKey, runtime, runtime, nil)
	if err != nil {
		return nil, err
	}
	resolver, ok := reader.(purchaseconfigurationfile.TargetResolver)
	if !ok {
		return nil, gerror.New("public catalog reader does not resolve purchase file targets")
	}
	return resolver, nil
}

func newPublicCatalogReader(
	database gdb.DB,
	cursorKey []byte,
	availability PublicPurchaseNewBusinessAvailability,
	candidates PublicPurchaseDynamicCandidateResolver,
	purchaseFiles purchaseconfigurationfile.Service,
) (CatalogReader, error) {
	if database == nil {
		return nil, gerror.New("public catalog reader requires a database")
	}
	cursor, err := newPublicCatalogCursorCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	revision, err := newPublicConfigurationRevisionCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	return &publicCatalogReaderImpl{
		database: database, cursor: cursor, revision: revision,
		config: newPublicConfigurationCache(), newBusinessAvailability: availability,
		dynamicCandidateResolver: candidates, purchaseFiles: purchaseFiles,
	}, nil
}

func (s *publicCatalogReaderImpl) Search(
	ctx context.Context,
	input PublicCatalogSearchInput,
) (*PublicCatalogPage, error) {
	normalized, err := normalizePublicCatalogSearch(input, s.cursor)
	if err != nil {
		return nil, err
	}
	page, err := searchPublicCatalogPage(ctx, s.database, normalized)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	if err = validatePublicCatalogPage(page); err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	return page, nil
}

func (s *publicCatalogReaderImpl) GetBySlug(
	ctx context.Context,
	publicSlug string,
) (*PublicCatalogProduct, error) {
	slug := strings.ToLower(strings.TrimSpace(publicSlug))
	if !publicSlugPattern.MatchString(slug) || utf8.RuneCountInString(slug) > 128 {
		return nil, bizerr.NewCode(CodePublicCatalogInvalid)
	}
	product, err := loadPublicCatalogProduct(ctx, s.database, slug)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	if product == nil {
		return nil, bizerr.NewCode(CodePublicCatalogNotFound)
	}
	if err = validatePublicCatalogProduct(*product); err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCatalogUnavailable)
	}
	return product, nil
}

type normalizedPublicCatalogSearch struct {
	publicSlugs []string
	namePrefix  string
	likePrefix  string
	categoryNo  string
	currency    string
	limit       int
	cursor      *publicCatalogCursorBoundary
	cursorCodec *publicCatalogCursorCodec
}

func normalizePublicCatalogSearch(
	input PublicCatalogSearchInput,
	cursorCodec *publicCatalogCursorCodec,
) (normalizedPublicCatalogSearch, error) {
	if len(input.PublicSlugs) > maximumPublicCatalogPageSize {
		return normalizedPublicCatalogSearch{}, bizerr.NewCode(CodePublicCatalogInvalid)
	}
	slugs := make([]string, 0, len(input.PublicSlugs))
	for _, slug := range input.PublicSlugs {
		if !publicSlugPattern.MatchString(slug) || utf8.RuneCountInString(slug) > 128 {
			return normalizedPublicCatalogSearch{}, bizerr.NewCode(CodePublicCatalogInvalid)
		}
		slugs = append(slugs, slug)
	}
	// Canonicalize a set so input order and duplicates do not change cursor identity.
	sort.Strings(slugs)
	unique := slugs[:0]
	for _, slug := range slugs {
		if len(unique) == 0 || unique[len(unique)-1] != slug {
			unique = append(unique, slug)
		}
	}
	namePrefix := strings.ToLower(strings.Join(strings.Fields(input.NamePrefix), " "))
	categoryNo := strings.TrimSpace(input.CategoryNo)
	currency := strings.TrimSpace(input.Currency)
	if !utf8.ValidString(namePrefix) || strings.ContainsRune(namePrefix, '\x00') ||
		utf8.RuneCountInString(namePrefix) > 128 ||
		(categoryNo != "" && !categoryNumberPattern.MatchString(categoryNo)) ||
		(currency != "" && !publicCurrencyPattern.MatchString(currency)) {
		return normalizedPublicCatalogSearch{}, bizerr.NewCode(CodePublicCatalogInvalid)
	}
	if input.Limit == 0 {
		input.Limit = defaultPublicCatalogPageSize
	}
	if input.Limit < 1 || input.Limit > maximumPublicCatalogPageSize || cursorCodec == nil {
		return normalizedPublicCatalogSearch{}, bizerr.NewCode(CodePublicCatalogInvalid)
	}
	normalized := normalizedPublicCatalogSearch{
		publicSlugs: unique,
		namePrefix:  namePrefix, likePrefix: escapeCatalogLikePrefix(namePrefix),
		categoryNo: categoryNo, currency: currency,
		limit: input.Limit, cursorCodec: cursorCodec,
	}
	if input.Cursor != "" {
		cursor, err := cursorCodec.decode(input.Cursor, normalized)
		if err != nil {
			return normalizedPublicCatalogSearch{}, bizerr.NewCode(CodePublicCatalogInvalid)
		}
		normalized.cursor = cursor
	}
	return normalized, nil
}

func validatePublicCatalogPage(page *PublicCatalogPage) error {
	if page == nil || page.Products == nil || page.Categories == nil || page.AvailableCurrencies == nil ||
		len(page.Products) > maximumPublicCatalogPageSize ||
		len(page.Categories) > maximumPublicCatalogCategoryCount ||
		len(page.AvailableCurrencies) > 64 ||
		(page.NextCursor != "" && len(page.NextCursor) > maximumPublicCatalogCursorLength) {
		return gerror.New("public catalog page is invalid")
	}
	for _, product := range page.Products {
		if err := validatePublicCatalogProduct(product); err != nil {
			return err
		}
	}
	for index, currency := range page.AvailableCurrencies {
		if !publicCurrencyPattern.MatchString(currency) ||
			(index > 0 && page.AvailableCurrencies[index-1] >= currency) {
			return gerror.New("public catalog currency choices are inconsistent")
		}
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		return gerror.Wrap(err, "measure public catalog page")
	}
	if len(encoded) > maximumPublicCatalogResponseBytes-publicCatalogEnvelopeReserve {
		return gerror.New("public catalog page exceeds response bound")
	}
	return nil
}

func validatePublicCatalogProduct(product PublicCatalogProduct) error {
	if !publicSlugPattern.MatchString(product.PublicSlug) ||
		!categoryNumberPattern.MatchString(product.CategoryNo) ||
		len(product.Currencies) == 0 || len(product.Currencies) > 64 {
		return gerror.New("public catalog product is inconsistent")
	}
	for index, currency := range product.Currencies {
		if !publicCurrencyPattern.MatchString(currency) ||
			(index > 0 && product.Currencies[index-1] >= currency) {
			return gerror.New("public catalog currencies are inconsistent")
		}
	}
	switch product.Availability {
	case "available", "unavailable":
		return nil
	default:
		return gerror.New("public catalog availability is inconsistent")
	}
}
