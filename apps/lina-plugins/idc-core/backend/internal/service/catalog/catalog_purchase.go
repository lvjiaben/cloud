// This file exposes a bounded, internal purchase projection without widening
// the anonymous public catalog contract.

package catalog

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

const maximumPurchaseRevalidationItems = 100

const (
	PurchaseIssueProductUnavailable = "product_unavailable"
	PurchaseIssueVersionReplaced    = "version_replaced"
	PurchaseIssueBillingUnavailable = "billing_unavailable"
)

// PurchaseConfigurationInput contains only the opaque structured-configuration
// and billing tokens issued by the generic public purchase surface.
type PurchaseConfigurationInput struct {
	PublicSlug            string
	ConfigurationRevision string
	BillingTermToken      string
	CurrencyCode          string
	PurchaseCycles        int
	Values                []PublicPurchaseConfigurationValue
}

// PurchaseConfigurationItem is a legacy server-resize specification snapshot.
type PurchaseConfigurationItem struct {
	GroupNo           string
	OptionNo          string
	GroupCode         string
	OptionCode        string
	ComponentTypeCode string
	GroupNameZhCN     string
	GroupNameEnUS     string
	OptionNameZhCN    string
	OptionNameEnUS    string
	NormalizedValue   string
	Quantity          int
	SortOrder         int
}

// PurchaseConfiguration is safe for transaction-domain persistence. Internal
// identifiers are derived by catalog code and are never accepted from a caller.
type PurchaseConfiguration struct {
	ProductNo               string
	VersionNo               string
	BillingRuleNo           string
	CurrencyCode            string
	SourceRevisionDigest    string
	MinimumPurchaseCycles   int
	RequiresCustomerContext bool
	PriceSelections         []ConfigurationPriceSelection
	PriceSelectionDigest    string
	CustomerValueDigest     string
}

// ServerResizeConfigurationInput is deliberately separate from the generic
// purchase contract. It is consumed only by the server-domain resize adapter.
type ServerResizeConfigurationInput struct {
	PublicSlug     string
	Revision       string
	TermNo         string
	CurrencyCode   string
	PurchaseCycles int
	Selections     []PublicConfigurationSelection
}

// ServerResizeConfiguration contains the temporary legacy server option
// snapshot required by the server resize flow. Generic purchases never see it.
type ServerResizeConfiguration struct {
	ProductNo               string
	VersionNo               string
	BillingRuleNo           string
	CurrencyCode            string
	SourceRevisionDigest    string
	MinimumPurchaseCycles   int
	RequiresCustomerContext bool
	Items                   []PurchaseConfigurationItem
}

// PurchaseConfigurationReference is the bounded immutable fact set needed for
// a batch availability check.
type PurchaseConfigurationReference struct {
	ReferenceNo   string
	ProductNo     string
	VersionNo     string
	BillingRuleNo string
}

// PurchaseConfigurationState reports current sellability without changing the cart.
type PurchaseConfigurationState struct {
	ReferenceNo           string
	PublicSlug            string
	ProductNameZhCN       string
	ProductNameEnUS       string
	ConfigurationRequired bool
	IssueCode             string
}

// PurchaseConfigurationResolver is the internal catalog-to-commerce boundary.
type PurchaseConfigurationResolver interface {
	ResolvePurchaseConfiguration(context.Context, PurchaseConfigurationInput) (*PurchaseConfiguration, error)
	RevalidatePurchaseConfigurations(context.Context, []PurchaseConfigurationReference) ([]PurchaseConfigurationState, error)
}

// ServerResizeConfigurationResolver is a temporary server-domain boundary.
// New product purchases MUST use PurchaseConfigurationResolver and plugin-declared
// structured configuration instead of the legacy fixed server option graph.
type ServerResizeConfigurationResolver interface {
	ResolveServerResizeConfiguration(context.Context, ServerResizeConfigurationInput) (*ServerResizeConfiguration, error)
}

var _ PurchaseConfigurationResolver = (*publicCatalogReaderImpl)(nil)

// PurchaseResolverFromPublicReader keeps the anonymous interface narrow while
// allowing startup composition to opt into the catalog-owned purchase boundary.
func PurchaseResolverFromPublicReader(reader PublicCatalogReader) (PurchaseConfigurationResolver, error) {
	resolver, ok := reader.(PurchaseConfigurationResolver)
	if !ok || resolver == nil {
		return nil, gerror.New("public catalog reader does not provide purchase resolution")
	}
	return resolver, nil
}

func ServerResizeResolverFromPublicReader(reader PublicCatalogReader) (ServerResizeConfigurationResolver, error) {
	resolver, ok := reader.(ServerResizeConfigurationResolver)
	if !ok || resolver == nil {
		return nil, gerror.New("public catalog reader does not provide server resize resolution")
	}
	return resolver, nil
}

// ResolvePurchaseConfiguration authenticates generic catalog and billing facts.
// Product-specific customer values are frozen separately from the plugin-owned
// structured configuration contract after the immutable quote is created.
func (s *publicCatalogReaderImpl) ResolvePurchaseConfiguration(
	ctx context.Context,
	input PurchaseConfigurationInput,
) (*PurchaseConfiguration, error) {
	slug := strings.ToLower(strings.TrimSpace(input.PublicSlug))
	currency := strings.TrimSpace(input.CurrencyCode)
	if !validPublicConfigurationSlug(slug) ||
		currency != input.CurrencyCode || !currencyCodePattern.MatchString(currency) ||
		input.PurchaseCycles < 1 || input.PurchaseCycles > 120 || s.revision == nil {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	revision := strings.TrimSpace(input.ConfigurationRevision)
	termToken := strings.TrimSpace(input.BillingTermToken)
	if revision != input.ConfigurationRevision || termToken != input.BillingTermToken ||
		!strings.HasPrefix(termToken, "term_") {
		return nil, bizerr.NewCode(CodePublicQuoteStale)
	}
	snapshot, err := s.loadStructuredConfiguration(ctx, slug, revision)
	if err != nil {
		return nil, err
	}
	values, err := s.decodePublicStructuredValues(snapshot, input.Values)
	if err != nil {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	if err = s.verifyStructuredFiles(ctx, snapshot, values); err != nil {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	evaluation, resolved, err := s.evaluateStructuredConfiguration(ctx, snapshot, values)
	if err != nil || !evaluation.Valid {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	compiled, err := configurationcontract.CompileResolved(
		snapshot.Schema, values, configurationcontract.SurfaceCustomer,
		authorizedDynamicMethods(snapshot.Schema), resolved,
	)
	if err != nil {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	priceSelections, selectionDigest, err := loadSelectedConfigurationPriceDimensions(ctx, s.database, snapshot.Target.VersionNo, values)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	source, err := loadPublicQuoteSource(ctx, s.database, snapshot.Target.VersionNo, currency)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	quote, err := buildPublicConfigurationQuote(revision, currency, source, configurationPriceSelectionMap(priceSelections))
	if err != nil {
		return nil, err
	}
	billingRuleNo := ""
	minimumCycles := 0
	requiresCustomerContext := false
	for _, term := range quote.Terms {
		encoded, encodeErr := s.revision.encodeStructuredQuoteTerm(slug, snapshot.Target.VersionNo, term.TermNo, selectionDigest)
		if encodeErr != nil {
			return nil, bizerr.WrapCode(encodeErr, CodePublicQuoteUnavailable)
		}
		if len(encoded) == len(termToken) && subtle.ConstantTimeCompare([]byte(encoded), []byte(termToken)) == 1 {
			if (term.Status != PublicQuoteFinal && term.Status != PublicQuoteRequiresCustomerContext) ||
				input.PurchaseCycles < term.MinimumPurchaseCycles {
				return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
			}
			billingRuleNo = term.TermNo
			minimumCycles = term.MinimumPurchaseCycles
			requiresCustomerContext = term.Status == PublicQuoteRequiresCustomerContext
			break
		}
	}
	if billingRuleNo == "" {
		return nil, bizerr.NewCode(CodePublicQuoteStale)
	}
	if err = s.recheckStructuredConfiguration(ctx, snapshot); err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(revision))
	return &PurchaseConfiguration{
		ProductNo: snapshot.Target.ProductNo, VersionNo: snapshot.Target.VersionNo,
		BillingRuleNo: billingRuleNo, CurrencyCode: currency,
		SourceRevisionDigest:    hex.EncodeToString(digest[:]),
		MinimumPurchaseCycles:   minimumCycles,
		RequiresCustomerContext: requiresCustomerContext,
		PriceSelections:         append([]ConfigurationPriceSelection(nil), priceSelections...),
		PriceSelectionDigest:    selectionDigest,
		CustomerValueDigest:     frozenConfigurationDigest(compiled.ValuesJSON),
	}, nil
}

// ResolveServerResizeConfiguration keeps the existing server-only resize flow
// isolated until that flow is fully owned by the server plugin contract.
func (s *publicCatalogReaderImpl) ResolveServerResizeConfiguration(
	ctx context.Context,
	input ServerResizeConfigurationInput,
) (*ServerResizeConfiguration, error) {
	slug := strings.ToLower(strings.TrimSpace(input.PublicSlug))
	currency := strings.TrimSpace(input.CurrencyCode)
	selections, err := normalizePublicConfigurationSelections(input.Selections)
	if err != nil || !validPublicConfigurationSlug(slug) ||
		currency != input.CurrencyCode || !currencyCodePattern.MatchString(currency) ||
		input.PurchaseCycles < 1 || input.PurchaseCycles > 120 || s.revision == nil {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	target, err := loadPublicConfigurationTarget(ctx, s.database, slug)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	if target == nil {
		return nil, bizerr.NewCode(CodePublicCatalogNotFound)
	}
	revision := strings.TrimSpace(input.Revision)
	termNo := strings.TrimSpace(input.TermNo)
	if revision != input.Revision || termNo != input.TermNo ||
		!strings.HasPrefix(termNo, "term_") || !s.revision.verify(revision, slug, target.VersionNo) {
		return nil, bizerr.NewCode(CodePublicQuoteStale)
	}
	graph, err := s.publicQuoteGraph(ctx, slug, revision, *target)
	if err != nil {
		return nil, err
	}
	validation, err := evaluatePublicConfiguration(graph, selections)
	if err != nil || !validation.Valid {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	source, err := loadPublicQuoteSource(ctx, s.database, target.VersionNo, currency)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	quote, err := buildPublicConfigurationQuote(revision, currency, source, selections)
	if err != nil {
		return nil, err
	}
	billingRuleNo := ""
	minimumCycles := 0
	requiresCustomerContext := false
	for _, term := range quote.Terms {
		encoded, encodeErr := s.revision.encodeQuoteTerm(slug, target.VersionNo, term.TermNo)
		if encodeErr != nil {
			return nil, bizerr.WrapCode(encodeErr, CodePublicQuoteUnavailable)
		}
		if len(encoded) == len(termNo) && subtle.ConstantTimeCompare([]byte(encoded), []byte(termNo)) == 1 {
			if (term.Status != PublicQuoteFinal && term.Status != PublicQuoteRequiresCustomerContext) ||
				input.PurchaseCycles < term.MinimumPurchaseCycles {
				return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
			}
			billingRuleNo = term.TermNo
			minimumCycles = term.MinimumPurchaseCycles
			requiresCustomerContext = term.Status == PublicQuoteRequiresCustomerContext
			break
		}
	}
	if billingRuleNo == "" {
		return nil, bizerr.NewCode(CodePublicQuoteStale)
	}
	items, err := loadPurchaseConfigurationItems(ctx, s.database, target.VersionNo, selections)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	current, err := s.database.GetValue(ctx, publicConfigurationStillCurrentSQL, slug, target.VersionNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	if !current.Bool() {
		return nil, bizerr.NewCode(CodePublicQuoteStale)
	}
	digest := sha256.Sum256([]byte(revision))
	return &ServerResizeConfiguration{
		ProductNo: target.ProductNo, VersionNo: target.VersionNo,
		BillingRuleNo: billingRuleNo, CurrencyCode: currency,
		SourceRevisionDigest:    hex.EncodeToString(digest[:]),
		MinimumPurchaseCycles:   minimumCycles,
		RequiresCustomerContext: requiresCustomerContext, Items: items,
	}, nil
}

type purchaseConfigurationItemRow struct {
	GroupNo           string `orm:"group_no"`
	OptionNo          string `orm:"option_no"`
	GroupCode         string `orm:"group_code"`
	OptionCode        string `orm:"option_code"`
	ComponentTypeCode string `orm:"component_type_code"`
	GroupNameZhCN     string `orm:"group_name_zh_cn"`
	GroupNameEnUS     string `orm:"group_name_en_us"`
	OptionNameZhCN    string `orm:"option_name_zh_cn"`
	OptionNameEnUS    string `orm:"option_name_en_us"`
	NormalizedValue   string `orm:"normalized_value"`
	SortOrder         int    `orm:"sort_order"`
}

func loadPurchaseConfigurationItems(
	ctx context.Context,
	database gdb.DB,
	versionNo string,
	selections map[string]int,
) ([]PurchaseConfigurationItem, error) {
	rows := make([]purchaseConfigurationItemRow, 0)
	if err := database.GetScan(ctx, &rows, purchaseConfigurationItemsSQL,
		versionNo, maximumPublicConfigurationOptions+1,
	); err != nil {
		return nil, gerror.Wrap(err, "load purchase configuration items")
	}
	if len(rows) > maximumPublicConfigurationOptions {
		return nil, gerror.New("purchase configuration item bound exceeded")
	}
	items := make([]PurchaseConfigurationItem, 0, len(selections))
	for _, row := range rows {
		quantity, selected := selections[row.OptionNo]
		if !selected {
			continue
		}
		items = append(items, PurchaseConfigurationItem{
			GroupNo: row.GroupNo, OptionNo: row.OptionNo,
			GroupCode: row.GroupCode, OptionCode: row.OptionCode,
			ComponentTypeCode: row.ComponentTypeCode,
			GroupNameZhCN:     row.GroupNameZhCN, GroupNameEnUS: row.GroupNameEnUS,
			OptionNameZhCN: row.OptionNameZhCN, OptionNameEnUS: row.OptionNameEnUS,
			NormalizedValue: row.NormalizedValue, Quantity: quantity, SortOrder: row.SortOrder,
		})
	}
	if len(items) != len(selections) {
		return nil, bizerr.NewCode(CodePublicQuoteStale)
	}
	return items, nil
}

const purchaseConfigurationItemsSQL = `
SELECT config_group.group_no, option.option_no, config_group.group_code,
       option.option_code, option.component_type_code,
       config_group.name_zh_cn AS group_name_zh_cn,
       config_group.name_en_us AS group_name_en_us,
       option.name_zh_cn AS option_name_zh_cn,
       option.name_en_us AS option_name_en_us,
       option.normalized_value, option.sort_order
FROM plugin_idc_core_catalog_configuration_option option
INNER JOIN plugin_idc_core_catalog_configuration_group config_group
    ON config_group.tenant_id = option.tenant_id
   AND config_group.version_no = option.version_no
   AND config_group.group_no = option.group_no
WHERE option.tenant_id = 0 AND option.version_no = ? AND option.status_code = 'active'
ORDER BY config_group.sort_order ASC, config_group.id ASC, option.sort_order ASC, option.id ASC
LIMIT ?`

// RevalidatePurchaseConfigurations evaluates all references with five bounded,
// set-based reads regardless of the number of cart lines.
func (s *publicCatalogReaderImpl) RevalidatePurchaseConfigurations(
	ctx context.Context,
	input []PurchaseConfigurationReference,
) ([]PurchaseConfigurationState, error) {
	if len(input) > maximumPurchaseRevalidationItems {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	if len(input) == 0 {
		return []PurchaseConfigurationState{}, nil
	}
	productNos := make([]string, 0, len(input))
	versionNos := make([]string, 0, len(input))
	billingRuleNos := make([]string, 0, len(input))
	seenReferences := make(map[string]struct{}, len(input))
	for _, item := range input {
		if !catalogBusinessNumberPattern.MatchString(item.ReferenceNo) ||
			!catalogBusinessNumberPattern.MatchString(item.ProductNo) ||
			!catalogBusinessNumberPattern.MatchString(item.VersionNo) ||
			!catalogBusinessNumberPattern.MatchString(item.BillingRuleNo) {
			return nil, bizerr.NewCode(CodePublicQuoteInvalid)
		}
		if _, duplicate := seenReferences[item.ReferenceNo]; duplicate {
			return nil, bizerr.NewCode(CodePublicQuoteInvalid)
		}
		seenReferences[item.ReferenceNo] = struct{}{}
		productNos = append(productNos, item.ProductNo)
		versionNos = append(versionNos, item.VersionNo)
		billingRuleNos = append(billingRuleNos, item.BillingRuleNo)
	}
	products, categories, versions, rules, err := loadPurchaseRevalidationFacts(
		ctx, s.database, productNos, versionNos, billingRuleNos,
	)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	configuredVersions := make(map[string]bool, len(versionNos))
	var configuredRows []struct {
		VersionNo string `orm:"version_no"`
	}
	if err = s.database.GetScan(ctx, &configuredRows, `SELECT version_no FROM plugin_idc_core_catalog_product_version_configuration WHERE tenant_id=0 AND version_no IN(?)`, versionNos); err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	for _, configured := range configuredRows {
		configuredVersions[configured.VersionNo] = true
	}
	result := make([]PurchaseConfigurationState, 0, len(input))
	for _, item := range input {
		issue := ""
		product, productExists := products[item.ProductNo]
		if !productExists || product.StatusCode != "active" || product.DeletedAt != nil ||
			!categories[product.CategoryNo] {
			issue = PurchaseIssueProductUnavailable
		} else if product.CurrentVersionNo == nil || *product.CurrentVersionNo != item.VersionNo ||
			versions[item.VersionNo] != "published" {
			issue = PurchaseIssueVersionReplaced
		} else if !rules[item.VersionNo+"\x00"+item.BillingRuleNo] {
			issue = PurchaseIssueBillingUnavailable
		}
		state := PurchaseConfigurationState{ReferenceNo: item.ReferenceNo, IssueCode: issue}
		if productExists {
			state.PublicSlug = product.PublicSlug
			state.ProductNameZhCN = product.NameZhCn
			state.ProductNameEnUS = product.NameEnUs
		}
		state.ConfigurationRequired = configuredVersions[item.VersionNo]
		result = append(result, state)
	}
	return result, nil
}

func loadPurchaseRevalidationFacts(
	ctx context.Context,
	database gdb.DB,
	productNos []string,
	versionNos []string,
	billingRuleNos []string,
) (map[string]entity.CatalogProduct, map[string]bool, map[string]string, map[string]bool, error) {
	var productRows []entity.CatalogProduct
	if err := database.Model(dao.CatalogProduct.Table()).Ctx(ctx).Fields(
		dao.CatalogProduct.Columns().ProductNo, dao.CatalogProduct.Columns().CategoryNo,
		dao.CatalogProduct.Columns().NameZhCn, dao.CatalogProduct.Columns().NameEnUs,
		dao.CatalogProduct.Columns().StatusCode, dao.CatalogProduct.Columns().CurrentVersionNo,
		dao.CatalogProduct.Columns().DeletedAt,
	).Where(do.CatalogProduct{TenantId: idccap.DefaultTenantID}).
		WhereIn(dao.CatalogProduct.Columns().ProductNo, uniqueStrings(productNos)).Scan(&productRows); err != nil {
		return nil, nil, nil, nil, gerror.Wrap(err, "load purchase products")
	}
	categoryNos := make([]string, 0, len(productRows))
	products := make(map[string]entity.CatalogProduct, len(productRows))
	for _, row := range productRows {
		products[row.ProductNo] = row
		categoryNos = append(categoryNos, row.CategoryNo)
	}
	var categoryRows []entity.CatalogCategory
	if len(categoryNos) > 0 {
		if err := database.Model(dao.CatalogCategory.Table()).Ctx(ctx).Fields(
			dao.CatalogCategory.Columns().CategoryNo, dao.CatalogCategory.Columns().StatusCode,
			dao.CatalogCategory.Columns().DeletedAt,
		).Where(do.CatalogCategory{TenantId: idccap.DefaultTenantID}).
			WhereIn(dao.CatalogCategory.Columns().CategoryNo, uniqueStrings(categoryNos)).Scan(&categoryRows); err != nil {
			return nil, nil, nil, nil, gerror.Wrap(err, "load purchase categories")
		}
	}
	categories := make(map[string]bool, len(categoryRows))
	for _, row := range categoryRows {
		categories[row.CategoryNo] = row.StatusCode == "active" && row.DeletedAt == nil
	}
	var versionRows []entity.CatalogProductVersion
	if err := database.Model(dao.CatalogProductVersion.Table()).Ctx(ctx).Fields(
		dao.CatalogProductVersion.Columns().VersionNo, dao.CatalogProductVersion.Columns().StatusCode,
	).Where(do.CatalogProductVersion{TenantId: idccap.DefaultTenantID}).
		WhereIn(dao.CatalogProductVersion.Columns().VersionNo, uniqueStrings(versionNos)).Scan(&versionRows); err != nil {
		return nil, nil, nil, nil, gerror.Wrap(err, "load purchase versions")
	}
	versions := make(map[string]string, len(versionRows))
	for _, row := range versionRows {
		versions[row.VersionNo] = row.StatusCode
	}
	var billingRows []entity.CatalogBillingRule
	if err := database.Model(dao.CatalogBillingRule.Table()).Ctx(ctx).Fields(
		dao.CatalogBillingRule.Columns().VersionNo, dao.CatalogBillingRule.Columns().BillingRuleNo,
		dao.CatalogBillingRule.Columns().StatusCode,
	).Where(do.CatalogBillingRule{TenantId: idccap.DefaultTenantID}).
		WhereIn(dao.CatalogBillingRule.Columns().VersionNo, uniqueStrings(versionNos)).
		WhereIn(dao.CatalogBillingRule.Columns().BillingRuleNo, uniqueStrings(billingRuleNos)).
		Scan(&billingRows); err != nil {
		return nil, nil, nil, nil, gerror.Wrap(err, "load purchase billing rules")
	}
	rules := make(map[string]bool, len(billingRows))
	for _, row := range billingRows {
		rules[row.VersionNo+"\x00"+row.BillingRuleNo] = row.StatusCode == "active"
	}
	return products, categories, versions, rules, nil
}

func uniqueStrings(input []string) []string {
	seen := make(map[string]struct{}, len(input))
	result := make([]string, 0, len(input))
	for _, value := range input {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
