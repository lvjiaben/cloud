// This file bounds public catalog snapshots before a peer connector can import
// them. Tokens remain opaque and evaluated views never imply static rules.
package channelcommercecap

import (
	"encoding/json"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"regexp"
)

var publicProductSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,159}$`)
var publicCurrencyCode = regexp.MustCompile(`^[A-Z]{3}$`)
var minorAmount = regexp.MustCompile(`^[0-9]{1,24}$`)

func (input ConfigurationInput) Validate() error {
	if !publicProductSlug.MatchString(input.PublicSlug) || len(input.ConfigurationRevision) == 0 || len(input.ConfigurationRevision) > 4096 || len(input.Values) > 128 {
		return gerror.New("product configuration input is invalid")
	}
	if input.CurrencyCode != "" && !publicCurrencyCode.MatchString(input.CurrencyCode) {
		return gerror.New("product currency is invalid")
	}
	raw, err := json.Marshal(input)
	if err != nil || len(raw) > 1<<20 {
		return gerror.New("product configuration input exceeds limit")
	}
	return nil
}

func (view Configuration) Validate() error {
	if len(view.ConfigurationRevision) == 0 || len(view.ConfigurationRevision) > 4096 || len(view.Fields) > 128 || len(view.Values) > 128 {
		return gerror.New("product configuration is invalid")
	}
	fields := make(map[string]bool, len(view.Fields))
	options := 0
	for _, field := range view.Fields {
		if field.FieldToken == "" || len(field.FieldToken) > 4096 || fields[field.FieldToken] {
			return gerror.New("product configuration field is invalid")
		}
		fields[field.FieldToken] = true
		if !validPublicFieldType(field.Type) {
			return gerror.New("product configuration field type is invalid")
		}
		if err := validatePublicOptions(field.Options, 0, &options); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(view)
	if err != nil || len(raw) > 1<<20 {
		return gerror.New("product configuration exceeds limit")
	}
	return nil
}

func validatePublicOptions(values []ConfigurationOption, depth int, count *int) error {
	if depth > 8 || len(values) > 512 {
		return gerror.New("product options exceed limit")
	}
	for _, value := range values {
		*count++
		if *count > 2048 || value.OptionToken == "" || len(value.OptionToken) > 4096 {
			return gerror.New("product option is invalid")
		}
		if err := validatePublicOptions(value.Children, depth+1, count); err != nil {
			return err
		}
	}
	return nil
}

func (value BillingTerms) Validate() error {
	if !publicCurrencyCode.MatchString(value.CurrencyCode) || len(value.ConfigurationRevision) == 0 || len(value.ConfigurationRevision) > 4096 || len(value.Terms) == 0 || len(value.Terms) > 120 {
		return gerror.New("product billing terms are invalid")
	}
	for _, term := range value.Terms {
		schedule := productfulfillmentcap.BillingSchedule{CurrencyCode: value.CurrencyCode, ChargeType: productfulfillmentcap.BillingChargeType(term.ChargeType), PeriodUnit: productfulfillmentcap.BillingPeriodUnit(term.PeriodUnit), PeriodCount: term.PeriodCount, PurchaseCycles: term.MinimumPurchaseCycles}
		if err := schedule.Validate(); err != nil {
			return err
		}
		if err := term.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (snapshot ProductSnapshot) Validate() error {
	if !publicProductSlug.MatchString(snapshot.Product.PublicSlug) || snapshot.ConfigurationMode != "remote_evaluation" || len(snapshot.Currencies) == 0 || len(snapshot.Currencies) > 64 || len(snapshot.BillingTerms) > 64 {
		return gerror.New("product snapshot is invalid")
	}
	if err := snapshot.Configuration.Validate(); err != nil {
		return err
	}
	if err := validateSnapshotCurrencies(snapshot.Currencies); err != nil {
		return err
	}
	for _, item := range snapshot.BillingTerms {
		if item.Precision < 0 || item.Precision > 6 {
			return gerror.New("product billing precision is invalid")
		}
		if err := (BillingTerms{ConfigurationRevision: snapshot.Configuration.ConfigurationRevision, CurrencyCode: item.CurrencyCode, Terms: item.Terms}).Validate(); err != nil {
			return err
		}
	}
	if snapshot.PricingReady && len(snapshot.BillingTerms) == 0 {
		return gerror.New("product price snapshot is incomplete")
	}
	return nil
}

func validPublicFieldType(value string) bool {
	switch value {
	case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine, capmodel.ConfigurationFieldInteger, capmodel.ConfigurationFieldDecimal, capmodel.ConfigurationFieldBoolean, capmodel.ConfigurationFieldSingleSelect, capmodel.ConfigurationFieldMultiSelect, capmodel.ConfigurationFieldDate, capmodel.ConfigurationFieldDateTime, capmodel.ConfigurationFieldTreeSelect, capmodel.ConfigurationFieldAddress, capmodel.ConfigurationFieldFile:
		return true
	default:
		return false
	}
}

// Validate verifies one explicit billing token and its minor-unit amounts.
func (term BillingTerm) Validate() error {
	if term.BillingTermToken == "" || len(term.BillingTermToken) > 4096 || term.MinimumPurchaseCycles < 1 {
		return gerror.New("product billing term is invalid")
	}
	for _, value := range []string{term.SetupAmount, term.SubtotalAmount, term.TaxAmount, term.TotalAmount} {
		if !minorAmount.MatchString(value) {
			return gerror.New("product billing amount is invalid")
		}
	}
	return nil
}

func validateSnapshotCurrencies(values []CatalogCurrency) error {
	seen := make(map[string]bool, len(values))
	for _, item := range values {
		if !publicCurrencyCode.MatchString(item.CurrencyCode) || item.Precision < 0 || item.Precision > 6 || seen[item.CurrencyCode] {
			return gerror.New("product currency precision is invalid")
		}
		seen[item.CurrencyCode] = true
	}
	return nil
}
