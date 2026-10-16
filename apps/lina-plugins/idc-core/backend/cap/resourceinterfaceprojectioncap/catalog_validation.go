// This file validates supplier sales projections at the plugin trust boundary.
// All collections, text and decimal representations are bounded; no raw JSON or
// inferred billing interval can enter the catalog owner through this contract.
package resourceinterfaceprojectioncap

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
)

// Decimal input has a transport bound, not a business pricing ceiling.
var catalogDecimalPattern = regexp.MustCompile(`^[0-9]{1,24}(\.[0-9]{1,12})?$`)
var catalogCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// Validate rejects incomplete owner-generated read evidence before admission.
func (e CatalogReadEvidence) Validate() error {
	if !interfaceNoPattern.MatchString(e.InterfaceInstanceNo) || e.OwnerID != OwnerID ||
		!channelPattern.MatchString(e.ResourceChannelRef) || !configurationRevisionPattern.MatchString(e.InterfaceConfigurationRevision) ||
		!pluginPattern.MatchString(e.PluginID) || len(e.PluginID) > 64 ||
		!catalogText(e.PluginReleaseVersion, 32, false) || !catalogText(e.ContractVersion, 32, false) ||
		!digestPattern.MatchString(e.ReleaseDigest) || !digestPattern.MatchString(e.DeclarationDigest) ||
		len(e.AuthorizedMethods) == 0 || len(e.AuthorizedMethods) > 16 {
		return gerror.New("upstream catalog read evidence is invalid")
	}
	return validateProjectionMethods(e.AuthorizedMethods)
}

func (q CatalogQuery) Validate() error {
	if q.Page < 1 || q.Page > 1_000_000 || q.PageSize < 1 || q.PageSize > 100 ||
		!catalogText(q.Keyword, 128, true) {
		return gerror.New("upstream catalog query is invalid")
	}
	return nil
}

func (q CatalogProductQuery) Validate() error {
	if !catalogText(q.Reference, 256, false) {
		return gerror.New("upstream catalog product reference is invalid")
	}
	return nil
}

func (p CatalogPage) Validate(q CatalogQuery) error {
	if q.Validate() != nil || p.Total < 0 || p.Total > 1_000_000 || len(p.Items) > q.PageSize {
		return gerror.New("upstream catalog page is invalid")
	}
	remaining := p.Total - (q.Page-1)*q.PageSize
	if remaining < 0 {
		remaining = 0
	}
	if remaining > q.PageSize {
		remaining = q.PageSize
	}
	if len(p.Items) != remaining {
		return gerror.New("upstream catalog page count is inconsistent")
	}
	seen := make(map[string]struct{}, len(p.Items))
	for _, item := range p.Items {
		if !validCatalogProduct(item) {
			return gerror.New("upstream catalog product is invalid")
		}
		if _, exists := seen[item.Reference]; exists {
			return gerror.New("upstream catalog product is duplicated")
		}
		seen[item.Reference] = struct{}{}
	}
	return nil
}

func (d CatalogDetail) Validate(q CatalogProductQuery) error {
	if d.ImportSnapshot != nil && (d.ImportSnapshot.Validate() != nil || d.ImportSnapshot.Product.PublicSlug != q.Reference) {
		return gerror.New("upstream import snapshot is invalid")
	}
	if d.PeerPurchase != nil && (d.PeerPurchase.Validate() != nil || d.PeerPurchase.Product.PublicSlug != q.Reference) {
		return gerror.New("upstream peer product snapshot is invalid")
	}
	if q.Validate() != nil || d.Product.Reference != q.Reference || !validCatalogProduct(d.Product) ||
		!validCatalogPrices(d.BasePrices) || len(d.Options) > 128 || (d.ImportSnapshot == nil && d.PriceScheduleComplete && len(d.BasePrices) == 0) {
		return gerror.New("upstream catalog detail is invalid")
	}
	if err := validCatalogConfiguration(d.ProductConfiguration); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(d.Options))
	choices := 0
	for _, option := range d.Options {
		if !catalogText(option.Reference, 256, false) || !catalogText(option.Name, 256, false) || len(option.Choices) > 512 {
			return gerror.New("upstream catalog option is invalid")
		}
		if _, exists := seen[option.Reference]; exists {
			return gerror.New("upstream catalog option is duplicated")
		}
		seen[option.Reference] = struct{}{}
		seenChoices := make(map[string]struct{}, len(option.Choices))
		for _, choice := range option.Choices {
			if !catalogText(choice.Reference, 256, false) || !catalogText(choice.Name, 256, false) || !validCatalogPrices(choice.Prices) {
				return gerror.New("upstream catalog choice is invalid")
			}
			if _, exists := seenChoices[choice.Reference]; exists {
				return gerror.New("upstream catalog choice is duplicated")
			}
			seenChoices[choice.Reference] = struct{}{}
			choices++
			if choices > 2048 {
				return gerror.New("upstream catalog choices exceed the limit")
			}
		}
	}
	return nil
}

// validCatalogConfiguration enforces the transport allowlist. The catalog owner
// additionally checks every field and value against the exact bound schema.
func validCatalogConfiguration(values []CatalogConfigurationValue) error {
	if len(values) > 128 {
		return gerror.New("upstream catalog configuration exceeds the limit")
	}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if !catalogText(value.FieldKey, 64, false) || seen[value.FieldKey] {
			return gerror.New("upstream catalog configuration field is invalid")
		}
		seen[value.FieldKey] = true
		arms := 0
		if value.TextValue != nil {
			arms++
			if !catalogText(*value.TextValue, 8192, true) {
				return gerror.New("upstream catalog configuration text is invalid")
			}
		}
		if value.IntegerValue != nil {
			arms++
		}
		if value.DecimalValue != nil {
			arms++
			if !catalogDecimalPattern.MatchString(*value.DecimalValue) {
				return gerror.New("upstream catalog configuration decimal is invalid")
			}
		}
		if value.BooleanValue != nil {
			arms++
		}
		if value.OptionValue != nil {
			arms++
			if !catalogText(*value.OptionValue, 256, false) {
				return gerror.New("upstream catalog configuration option is invalid")
			}
		}
		if arms != 1 {
			return gerror.New("upstream catalog configuration requires one scalar value")
		}
	}
	return nil
}

func validCatalogProduct(p CatalogProduct) bool {
	return catalogText(p.Reference, 256, false) && catalogText(p.Name, 256, false) && catalogText(p.Description, 1024, true)
}

func validCatalogPrices(prices []CatalogPrice) bool {
	if len(prices) > 120 {
		return false
	}
	type key struct {
		currency string
		period   CatalogPeriod
		count    int
	}
	seen := make(map[key]struct{}, len(prices))
	for _, price := range prices {
		if !catalogCurrencyPattern.MatchString(price.CurrencyCode) || !catalogDecimalPattern.MatchString(price.Amount) ||
			!catalogDecimalPattern.MatchString(price.SetupFee) || price.PeriodCount < 1 || price.PeriodCount > 120 {
			return false
		}
		switch price.Period {
		case CatalogPeriodOnce:
			if price.PeriodCount != 1 {
				return false
			}
		case CatalogPeriodHour, CatalogPeriodDay, CatalogPeriodMonth, CatalogPeriodYear:
		default:
			return false
		}
		dimension := key{price.CurrencyCode, price.Period, price.PeriodCount}
		if _, exists := seen[dimension]; exists {
			return false
		}
		seen[dimension] = struct{}{}
	}
	return true
}

func catalogText(value string, limit int, optional bool) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) != value || (!optional && value == "") || utf8.RuneCountInString(value) > limit {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}
