// This file defines the complete, authenticated same-system sale import
// projection. Public tokens remain opaque and no provider credential or private
// method name is permitted in the schema or its reversible purchase mapping.

package channelcommercecap

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
)

const ImportProtocolVersion = "1"

// ImportSnapshot freezes a complete configuration and pricing description for
// a draft import. Local publication still requires explicit merchant pricing;
// upstream procurement must obtain a fresh authoritative term and quote.
type ImportSnapshot struct {
	// ProductNo is owner-local adapter evidence and never crosses the wire.
	ProductNo             string                                   `json:"-"`
	ProtocolVersion       string                                   `json:"protocolVersion"`
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
	FieldKey   string                `json:"fieldKey"`
	FieldToken string                `json:"fieldToken"`
	Options    []ImportOptionMapping `json:"options"`
}

// ImportOptionMapping never exposes the original provider candidate value.
type ImportOptionMapping struct {
	Value       string `json:"value"`
	OptionToken string `json:"optionToken"`
}

// ImportFulfillment preserves declared delivery and lifecycle semantics without
// exposing the upstream provider's implementation or private method names.
type ImportFulfillment struct {
	Granularity    string `json:"granularity"`
	OngoingService bool   `json:"ongoingService"`
	RenewalMode    string `json:"renewalMode"`
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
	if snapshot.ProtocolVersion != ImportProtocolVersion || !publicProductSlug.MatchString(snapshot.Product.PublicSlug) ||
		len(snapshot.ConfigurationRevision) == 0 || len(snapshot.ConfigurationRevision) > 4096 ||
		len(snapshot.Mappings) > 128 || len(snapshot.Prices) == 0 || len(snapshot.Prices) > 4096 || len(snapshot.Currencies) == 0 || len(snapshot.Currencies) > 64 || !snapshot.PricingComplete {
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
		if !currencies[price.CurrencyCode] || seen[key] {
			return gerror.New("import price currency or term is invalid")
		}
		seen[key] = true
		for _, option := range price.Options {
			if option.PricingMode == ImportConfigurationPricePerUnit || option.PricingMode == ImportConfigurationPriceBandTotal {
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
		if err := price.validateImportSchedule(); err != nil {
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
