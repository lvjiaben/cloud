// This file validates bounded import submissions and calculates only explicitly
// complete new-purchase prices using the pinned ISO/CLDR currency dependency.

package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/text/currency"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

const maximumImportPrices = 120

var versionNumberPatternForImport = regexp.MustCompile(`^ver_[0-9a-f]{32}$`)

var importRequestKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{15,127}$`)

type importPrice struct {
	portable      *channelcommercecap.ImportPriceSchedule
	definition    BillingRuleDefinition
	currencyCode  string
	amountMinor   int64
	setupFeeMinor int64
}

func validateImportInput(input ImportInput) (string, error) {
	if input.TargetVersionNo != "" && !versionNumberPatternForImport.MatchString(input.TargetVersionNo) {
		return "", bizerr.NewCode(CodeManagementInvalid)
	}
	if !importRequestKeyPattern.MatchString(input.RequestKey) || !categoryNumberPattern.MatchString(input.CategoryNo) ||
		len(input.InterfaceInstanceNo) != 36 || !strings.HasPrefix(input.InterfaceInstanceNo, "pfi_") ||
		len(input.Reference) == 0 || len(input.Reference) > 256 || strings.TrimSpace(input.Reference) != input.Reference || strings.ContainsRune(input.Reference, '\x00') ||
		!validLocalizedText(input.NameZhCN, 256, true) || !validLocalizedText(input.NameEnUS, 256, true) || len(input.Prices) > maximumImportPrices {
		return "", bizerr.NewCode(CodeManagementInvalid)
	}
	switch input.PricingMode {
	case ImportPricingMarkup:
		if len(input.MarkupPercent) == 0 || len(input.MarkupPercent) > 128 || len(input.Prices) != 0 {
			return "", bizerr.NewCode(CodeManagementInvalid)
		}
		if _, err := parseImportPriceDecimal(input.MarkupPercent); err != nil {
			return "", err
		}
	case ImportPricingManual:
		if input.MarkupPercent != "" || len(input.Prices) == 0 {
			return "", bizerr.NewCode(CodeManagementInvalid)
		}
		if _, err := calculateImportPrices(input.Prices, ImportPricingManual, ""); err != nil {
			return "", err
		}
	default:
		return "", bizerr.NewCode(CodeManagementInvalid)
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return "", bizerr.WrapCode(err, CodeManagementInvalid)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// importCurrencyMinorUnit uses Standard.Rounding (not Cash) from x/text v0.40.0.
// The tender iterator excludes ISO placeholders, precious metals and fund units;
// historical tender codes are recognized without inventing a currency fallback.
func importCurrencyMinorUnit(code string) (int, error) {
	if !currencyCodePattern.MatchString(code) {
		return 0, bizerr.NewCode(CodeManagementInvalid)
	}
	unit, err := currency.ParseISO(code)
	if err != nil || unit == currency.XXX {
		return 0, bizerr.NewCode(CodeManagementInvalid)
	}
	known := false
	for query := currency.Query(currency.Historical); query.Next(); {
		if query.Unit() == unit && query.IsTender() {
			known = true
			break
		}
	}
	if !known {
		return 0, bizerr.NewCode(CodeManagementInvalid)
	}
	scale, _ := currency.Standard.Rounding(unit)
	return scale, nil
}

func calculateImportPrices(prices []resourceinterfaceprojectioncap.CatalogPrice, mode ImportPricingMode, percent string) ([]importPrice, error) {
	result := make([]importPrice, 0, len(prices))
	seen := make(map[string]bool, len(prices))
	if len(prices) > maximumImportPrices {
		return nil, bizerr.NewCode(CodeManagementInvalid)
	}
	for _, price := range prices {
		if len(price.Amount) == 0 || len(price.Amount) > 128 || len(price.SetupFee) == 0 || len(price.SetupFee) > 128 {
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
		definition := BillingRuleDefinition{ChargeType: ChargeTypeRecurring, PeriodUnit: PeriodUnit(price.Period), PeriodCount: price.PeriodCount, PricePhase: PricePhaseNewPurchase, QuantityMode: QuantityModeFixed, MinimumPurchaseCycles: 1}
		if price.Period == resourceinterfaceprojectioncap.CatalogPeriodOnce {
			definition.ChargeType = ChargeTypeOneTime
		}
		if ValidateBillingRule(definition) != nil {
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
		key := fmt.Sprintf("%s:%s:%d", price.CurrencyCode, price.Period, price.PeriodCount)
		if seen[key] {
			return nil, bizerr.NewCode(CodeManagementInvalid)
		}
		seen[key] = true
		precision, err := importCurrencyMinorUnit(price.CurrencyCode)
		if err != nil {
			return nil, err
		}
		var amount, setup int64
		if mode == ImportPricingMarkup {
			amount, err = importMarkupPriceMinor(price.Amount, percent, precision)
		} else {
			amount, err = importFinalPriceMinor(price.Amount, precision)
		}
		if err != nil {
			return nil, err
		}
		if mode == ImportPricingMarkup {
			setup, err = importMarkupPriceMinor(price.SetupFee, percent, precision)
		} else {
			setup, err = importFinalPriceMinor(price.SetupFee, precision)
		}
		if err != nil {
			return nil, err
		}
		result = append(result, importPrice{definition: definition, currencyCode: price.CurrencyCode, amountMinor: amount, setupFeeMinor: setup})
	}
	return result, nil
}

func prepareImportSource(input ImportInput, source *resourceinterfaceprojectioncap.CatalogDetail) ([]byte, []importPrice, bool, error) {
	if source == nil || source.Product.Reference != input.Reference || !validLocalizedText(source.Product.Name, 256, false) ||
		!validLocalizedText(source.Product.Description, 1024, true) || len(source.Options) > 128 || len(source.ProductConfiguration) > 128 || len(source.BasePrices) > maximumImportPrices {
		return nil, nil, false, bizerr.NewCode(CodeManagementInvalid)
	}
	// Normalize only absent collections. The source remains a safe, structured
	// projection, never a raw supplier response or arbitrary metadata document.
	copySource := *source
	copySource.BasePrices = append([]resourceinterfaceprojectioncap.CatalogPrice{}, source.BasePrices...)
	copySource.ProductConfiguration = append([]resourceinterfaceprojectioncap.CatalogConfigurationValue{}, source.ProductConfiguration...)
	copySource.Options = append([]resourceinterfaceprojectioncap.CatalogOption{}, source.Options...)
	for i := range copySource.Options {
		copySource.Options[i].Choices = append([]resourceinterfaceprojectioncap.CatalogChoice{}, source.Options[i].Choices...)
		for j := range copySource.Options[i].Choices {
			copySource.Options[i].Choices[j].Prices = append([]resourceinterfaceprojectioncap.CatalogPrice{}, source.Options[i].Choices[j].Prices...)
		}
	}
	encoded, err := json.Marshal(copySource)
	if err != nil || len(encoded) > 1<<20 {
		return nil, nil, false, bizerr.NewCode(CodeManagementInvalid)
	}
	if input.PricingMode == ImportPricingManual {
		if source.ImportSnapshot != nil {
			prices, priceErr := calculatePortableManualPrices(source.ImportSnapshot, input.Prices)
			return encoded, prices, false, priceErr
		}
		prices, priceErr := calculateImportPrices(input.Prices, input.PricingMode, "")
		return encoded, prices, false, priceErr
	}
	if source.ImportSnapshot != nil {
		prices, priceErr := calculatePortableImportPrices(source.ImportSnapshot, input.MarkupPercent)
		return encoded, prices, false, priceErr
	}
	if !source.PriceScheduleComplete || len(source.Options) > 0 || len(source.BasePrices) == 0 {
		return encoded, []importPrice{}, true, nil
	}
	prices, err := calculateImportPrices(source.BasePrices, input.PricingMode, input.MarkupPercent)
	return encoded, prices, false, err
}
