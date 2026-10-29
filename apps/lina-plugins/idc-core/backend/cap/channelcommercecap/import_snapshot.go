// This file defines the complete, authenticated same-system sale import
// projection. Public tokens remain opaque and no provider credential or private
// method name is permitted in the schema or its reversible purchase mapping.

package channelcommercecap

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
)

const ImportProtocolVersion = "1"

// ImportRemoteEvaluationProtocolVersion fences dynamically priced source terms.
const ImportRemoteEvaluationProtocolVersion = "3"
const ImportPricingRemoteEvaluation = "remote_evaluation"

// ImportPurchaseCycleLimitProtocolVersion fences mandatory source policy:
// purchase ceilings and customer visibility/defaults cannot be stripped by v1.
const ImportPurchaseCycleLimitProtocolVersion = "2"

// ImportSnapshot freezes a complete configuration and pricing description for
// a draft import. Local publication still requires explicit merchant pricing;
// upstream procurement must obtain a fresh authoritative term and quote.
type ImportSnapshot struct {
	// ProductNo is owner-local adapter evidence and never crosses the wire.
	ProductNo             string                                   `json:"-"`
	ProtocolVersion       string                                   `json:"protocolVersion"`
	PricingMode           string                                   `json:"pricingMode,omitempty"`
	Product               CatalogProduct                           `json:"product"`
	ConfigurationRevision string                                   `json:"configurationRevision"`
	Schema                *capmodel.ConfigurationSchemaDeclaration `json:"schema"`
	Mappings              []ImportFieldMapping                     `json:"mappings"`
	Currencies            []CatalogCurrency                        `json:"currencies"`
	Prices                []ImportPriceSchedule                    `json:"prices"`
	PricingComplete       bool                                     `json:"pricingComplete"`
	Fulfillment           ImportFulfillment                        `json:"fulfillment"`
}

// ImportFieldMapping connects a stable, schema-safe opaque key to the upstream
// public purchase token. Dynamic options are resolved through governed methods.
type ImportFieldMapping struct {
	CustomerVisible *bool                 `json:"customerVisible,omitempty"`
	DefaultValue    json.RawMessage       `json:"defaultValue,omitempty"`
	FieldKey        string                `json:"fieldKey"`
	FieldToken      string                `json:"fieldToken"`
	Options         []ImportOptionMapping `json:"options"`
}

// ImportOptionMapping never exposes the original provider candidate value.
type ImportOptionMapping struct {
	Value       string `json:"value"`
	OptionToken string `json:"optionToken"`
}

// ImportFulfillment preserves declared delivery and lifecycle semantics without
// exposing the upstream provider's implementation or private method names.
type ImportFulfillment struct {
	// MaximumPurchaseCycles is a source limit carried through every resale.
	// Zero preserves legacy snapshots without an additional cycle ceiling.
	MaximumPurchaseCycles int    `json:"maximumPurchaseCycles,omitempty"`
	Granularity           string `json:"granularity"`
	OngoingService        bool   `json:"ongoingService"`
	RenewalMode           string `json:"renewalMode"`
}

// CatalogImportRuntime exports only products visible to the authenticated API
// customer and applies the same customer pricing authority used by purchase.
type CatalogImportRuntime interface {
	ReadCatalogImportSnapshot(context.Context, string, string) (*ImportSnapshot, error)
}

// Validate bounds an import snapshot before persistence. It checks the
// portable contract shape and rejects incomplete price declarations; provider
// keys remain valid only through the opaque mapping list.
func (snapshot ImportSnapshot) Validate() error {
	if !snapshot.validPricingProtocol() || !publicProductSlug.MatchString(snapshot.Product.PublicSlug) ||
		len(snapshot.ConfigurationRevision) == 0 || len(snapshot.ConfigurationRevision) > 4096 ||
		len(snapshot.Mappings) > 128 || len(snapshot.Prices) == 0 || len(snapshot.Prices) > 4096 || len(snapshot.Currencies) == 0 || len(snapshot.Currencies) > 64 {
		return gerror.New("import snapshot is incomplete")
	}
	if snapshot.Schema != nil && len(snapshot.Schema.Fields) > 128 {
		return gerror.New("import schema is invalid")
	}
	if err := validateSnapshotCurrencies(snapshot.Currencies); err != nil {
		return err
	}
	if err := snapshot.validateImportMappings(); err != nil {
		return err
	}
	currencies := make(map[string]bool, len(snapshot.Currencies))
	for _, item := range snapshot.Currencies {
		currencies[item.CurrencyCode] = true
	}
	seen := make(map[string]bool, len(snapshot.Prices))
	for _, price := range snapshot.Prices {
		key := price.CurrencyCode + "\x00" + price.TermReference
		if snapshot.Fulfillment.MaximumPurchaseCycles > 0 && price.MinimumPurchaseCycles > snapshot.Fulfillment.MaximumPurchaseCycles {
			return gerror.New("import minimum purchase cycles exceed source maximum")
		}
		if !currencies[price.CurrencyCode] || seen[key] {
			return gerror.New("import price currency or term is invalid")
		}
		seen[key] = true
		for _, option := range price.Options {
			if option.PricingMode == ImportConfigurationPricePerUnit || option.PricingMode == ImportConfigurationPriceBandTotal || option.PricingMode == ImportConfigurationPriceGraduated {
				var field *capmodel.ConfigurationFieldDeclaration
				if snapshot.Schema != nil {
					for _, candidate := range snapshot.Schema.Fields {
						if candidate != nil && candidate.Key == option.FieldKey {
							field = candidate
							break
						}
					}
				}
				if field == nil || field.Type != capmodel.ConfigurationFieldInteger {
					return gerror.New("import quantity pricing field is unavailable")
				}
			}
		}
		if err := snapshot.validatePriceSchedule(price); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(snapshot)
	if err != nil || len(raw) > 1<<20 {
		return gerror.New("import snapshot exceeds response bound")
	}
	return snapshot.Fulfillment.validate()
}

func (snapshot ImportSnapshot) validateImportMappings() error {
	fields := make(map[string]*capmodel.ConfigurationFieldDeclaration)
	if snapshot.Schema != nil {
		for _, field := range snapshot.Schema.Fields {
			if field == nil || field.Key == "" || fields[field.Key] != nil || !validPublicFieldType(field.Type) {
				return gerror.New("import schema field is invalid")
			}
			fields[field.Key] = field
		}
	}
	if len(fields) != len(snapshot.Mappings) {
		return gerror.New("import field mapping is incomplete")
	}
	seen := make(map[string]bool)
	for _, mapping := range snapshot.Mappings {
		field := fields[mapping.FieldKey]
		if field == nil || seen[mapping.FieldKey] || len(mapping.FieldToken) == 0 || len(mapping.FieldToken) > 4096 || len(mapping.Options) > 512 {
			return gerror.New("import field mapping is invalid")
		}
		seen[mapping.FieldKey] = true
		if err := validateImportDefault(field, mapping); err != nil {
			return err
		}
		if err := validateImportOptionMapping(field, mapping.Options); err != nil {
			return err
		}
	}
	return nil
}

func validateImportOptionMapping(field *capmodel.ConfigurationFieldDeclaration, mappings []ImportOptionMapping) error {
	options := make(map[string]bool)
	for _, option := range mappings {
		if option.Value == "" || options[option.Value] || len(option.OptionToken) == 0 || len(option.OptionToken) > 2048 {
			return gerror.New("import option mapping is invalid")
		}
		options[option.Value] = true
	}
	if len(options) != len(field.StaticCandidates) {
		return gerror.New("import candidate mapping is incomplete")
	}
	for _, candidate := range field.StaticCandidates {
		if candidate == nil || !options[candidate.Value] {
			return gerror.New("import candidate mapping is invalid")
		}
	}
	return nil
}

// validImportSnapshotProtocol rejects stripped v2 policy and rejects new
// metadata on v1. Unrestricted legacy snapshots keep their original bytes.
func validImportSnapshotProtocol(version string, maximum int, mappings []ImportFieldMapping) bool {
	metadata := false
	for _, mapping := range mappings {
		metadata = metadata || mapping.CustomerVisible != nil || len(mapping.DefaultValue) > 0
	}
	switch version {
	case ImportProtocolVersion:
		return maximum == 0 && !metadata
	case ImportPurchaseCycleLimitProtocolVersion:
		return maximum > 0 || metadata
	default:
		return false
	}
}

// validPricingProtocol rejects a stripped mode or a static-price fallback for
// v3. v1/v2 keep byte-compatible complete component schedules.
func (snapshot ImportSnapshot) validPricingProtocol() bool {
	if snapshot.ProtocolVersion == ImportRemoteEvaluationProtocolVersion {
		return snapshot.PricingMode == ImportPricingRemoteEvaluation && !snapshot.PricingComplete
	}
	return snapshot.PricingMode == "" && snapshot.PricingComplete && validImportSnapshotProtocol(snapshot.ProtocolVersion, snapshot.Fulfillment.MaximumPurchaseCycles, snapshot.Mappings)
}
func (snapshot ImportSnapshot) validatePriceSchedule(price ImportPriceSchedule) error {
	if snapshot.PricingMode != ImportPricingRemoteEvaluation {
		return price.validateImportSchedule()
	}
	if len(price.Base) != 0 || len(price.Options) != 0 || price.Usage != nil || price.TrialQuantity != 0 || price.QuantityMode != "fixed" {
		return gerror.New("remote import cannot declare static component prices")
	}
	// Reuse the common schedule validator with a local validation sentinel only;
	// no amount is persisted in or exported from the remote snapshot.
	copyPrice := price
	copyPrice.Base = []ImportPriceBand{{}}
	return copyPrice.validateImportSchedule()
}

// validateImportDefault bounds portable defaults without accepting secrets or
// arbitrary JSON objects. Owner admission validates exact schema bounds as well.
func validateImportDefault(field *capmodel.ConfigurationFieldDeclaration, mapping ImportFieldMapping) error {
	hidden := mapping.CustomerVisible != nil && !*mapping.CustomerVisible
	if len(mapping.DefaultValue) == 0 {
		if hidden {
			return gerror.New("hidden import field requires a default")
		}
		return nil
	}
	if field.Type == capmodel.ConfigurationFieldSecret || field.Type == capmodel.ConfigurationFieldFile || len(mapping.DefaultValue) > 8192 || !json.Valid(mapping.DefaultValue) {
		return gerror.New("import default is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(mapping.DefaultValue))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return gerror.New("import default is invalid")
	}
	valid := false
	switch field.Type {
	case capmodel.ConfigurationFieldBoolean:
		_, valid = value.(bool)
	case capmodel.ConfigurationFieldInteger:
		_, valid = value.(json.Number)
	case capmodel.ConfigurationFieldMultiSelect, capmodel.ConfigurationFieldAddress:
		switch values := value.(type) {
		case string:
			valid = false
		case []any:
			valid = len(values) <= 512
			for _, item := range values {
				if _, ok := item.(string); !ok {
					valid = false
				}
			}
		}
	default:
		_, valid = value.(string)
	}
	if !valid {
		return gerror.New("import default type is invalid")
	}
	return nil
}
