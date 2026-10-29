// This file validates the explicit generic native import contract separately
// from server hardware declarations. A plugin name or missing hardware fields
// never grant native-sale admission; exact declared protocol and mappings do.
package resourcecap

import (
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

// ValidateNativeProductEvidence checks the product half of one exact connector
// release before a resource-owned interface projection admits native purchases.
// Generic server evidence continues to use its independent server validator.
func ValidateNativeProductEvidence(evidence *capmodel.BusinessExtensionProviderEvidence) error {
	if evidence == nil || evidence.Point != capmodel.BusinessExtensionProductFulfillment || evidence.ContractVersion != productfulfillmentcap.ContractVersion {
		return gerror.New("native product evidence is invalid")
	}
	declaration, err := productfulfillmentcap.ParseProviderDeclaration(evidence.ProviderMetadata, evidence.AuthorizedMethods)
	if err != nil || !declaration.SaleImportProtocol.NativePurchase() || declaration.EntitlementOwnerID != OwnerPluginID {
		return gerror.New("native purchase protocol is unavailable")
	}
	schema := evidence.ProductConfiguration
	if schema == nil || schema.Version != "v1" || len(schema.Fields) != 2 || len(schema.Conditions) != 0 || evidence.CustomerConfiguration != nil && len(evidence.CustomerConfiguration.Fields) != 0 {
		return gerror.New("native product identity schema is invalid")
	}
	fields := map[string]*capmodel.ConfigurationFieldDeclaration{}
	for _, field := range schema.Fields {
		if field == nil || fields[field.Key] != nil || !field.Required {
			return gerror.New("native product identity field is invalid")
		}
		fields[field.Key] = field
	}
	product, revision := fields["upstream_product_id"], fields["upstream_configuration_revision"]
	if product == nil || revision == nil || product.Type != capmodel.ConfigurationFieldSingleSelect || product.DynamicCandidates == nil || product.DynamicCandidates.Version != "v1" || product.DynamicCandidates.Method != DynamicProviderMethodConfigurationCandidates || len(product.StaticCandidates) != 0 || revision.Type != capmodel.ConfigurationFieldSingleLine || revision.MinimumLength != 64 || revision.MaximumLength != 64 {
		return gerror.New("native product identity mapping is incomplete")
	}
	declared := false
	for _, method := range evidence.AuthorizedMethods {
		if method == product.DynamicCandidates.Method {
			declared = true
		}
	}
	if !declared {
		return gerror.New("native product candidate method is unauthorized")
	}
	return nil
}

// ValidateNativeImportSnapshot verifies source-derived field mappings before
// the product owner persists a native finance snapshot. The schema carries no
// executable methods, credentials, supplier identity or inferred hardware roles.
func ValidateNativeImportSnapshot(snapshot *channelcommercecap.ImportSnapshot) error {
	if snapshot == nil || snapshot.Validate() != nil || snapshot.Schema == nil || len(snapshot.ConfigurationRevision) != 64 || !nativePositiveID(snapshot.Product.PublicSlug) || snapshot.Fulfillment.Granularity != productfulfillmentcap.GranularityPerQuantityUnit || !snapshot.Fulfillment.OngoingService || snapshot.Fulfillment.RenewalMode != productfulfillmentcap.RenewalModeUnavailable || snapshot.Fulfillment.MaximumPurchaseCycles != 1 {
		return gerror.New("native import snapshot is invalid")
	}
	fields := map[string]*capmodel.ConfigurationFieldDeclaration{}
	for _, field := range snapshot.Schema.Fields {
		if field == nil || field.DynamicCandidates != nil || field.UploadPolicy != "" {
			return gerror.New("native import schema contains unsupported methods")
		}
		fields[field.Key] = field
	}
	for _, mapping := range snapshot.Mappings {
		kind, id, found := strings.Cut(mapping.FieldToken, ":")
		field := fields[mapping.FieldKey]
		if !found || !nativePositiveID(id) || field == nil || !nativeFieldMapping(kind, id, field, mapping) {
			return gerror.New("native import field mapping is invalid")
		}
	}
	for _, schedule := range snapshot.Prices {
		if schedule.MinimumPurchaseCycles != 1 {
			return gerror.New("native import purchase cycle count is unsupported")
		}
		for _, price := range schedule.Options {
			field := fields[price.FieldKey]
			if field == nil || field.Type != capmodel.ConfigurationFieldInteger && field.Type != capmodel.ConfigurationFieldSingleSelect && field.Type != capmodel.ConfigurationFieldTreeSelect {
				return gerror.New("native import price field is unavailable")
			}
			if field.Type == capmodel.ConfigurationFieldSingleSelect || field.Type == capmodel.ConfigurationFieldTreeSelect {
				found := false
				for _, candidate := range field.StaticCandidates {
					if candidate.Value == price.Value {
						found = true
					}
				}
				if !found {
					return gerror.New("native import price choice is unavailable")
				}
			}
		}
	}
	return nil
}

func nativeFieldMapping(kind, id string, field *capmodel.ConfigurationFieldDeclaration, mapping channelcommercecap.ImportFieldMapping) bool {
	switch kind {
	case "config":
		if mapping.FieldKey != "cfg_"+id || field.Type != capmodel.ConfigurationFieldSingleSelect && field.Type != capmodel.ConfigurationFieldInteger && field.Type != capmodel.ConfigurationFieldTreeSelect {
			return false
		}
		if field.Type == capmodel.ConfigurationFieldTreeSelect {
			return nativeTreeFieldMapping(id, field, mapping)
		}
		for _, option := range mapping.Options {
			if option.Value != option.OptionToken || option.Value != "0" && !nativePositiveID(option.Value) {
				return false
			}
		}
	case "custom":
		if mapping.FieldKey != "custom_"+id || field.Type != capmodel.ConfigurationFieldSingleLine && field.Type != capmodel.ConfigurationFieldMultiLine && field.Type != capmodel.ConfigurationFieldSingleSelect {
			return false
		}
		for _, option := range mapping.Options {
			if option.Value != option.OptionToken {
				return false
			}
		}
	default:
		return false
	}
	return true
}

func nativePositiveID(value string) bool {
	number, err := strconv.ParseInt(value, 10, 64)
	return err == nil && number > 0 && strconv.FormatInt(number, 10) == value
}

func nativeTreeFieldMapping(root string, field *capmodel.ConfigurationFieldDeclaration, mapping channelcommercecap.ImportFieldMapping) bool {
	candidates := map[string]bool{}
	for _, candidate := range field.StaticCandidates {
		if candidate == nil || candidates[candidate.Value] {
			return false
		}
		candidates[candidate.Value] = true
	}
	for _, option := range mapping.Options {
		if option.Value != option.OptionToken || !nativeTreeSourcePath(option.Value, root) {
			return false
		}
	}
	for _, candidate := range field.StaticCandidates {
		if candidate == nil || !nativeTreeSourcePath(candidate.Value, root) {
			return false
		}
		parent := ""
		if index := strings.LastIndex(candidate.Value, "/"); index >= 0 {
			parent = candidate.Value[:index]
		}
		if parent != candidate.ParentValue || parent != "" && !candidates[parent] {
			return false
		}
	}
	return true
}
func nativeTreeSourcePath(value, root string) bool {
	parts := strings.Split(value, "/")
	if len(value) > 256 || len(parts) < 1 || len(parts) > 8 {
		return false
	}
	seen := map[string]bool{}
	for index, part := range parts {
		field, choice, ok := strings.Cut(part, ":")
		if !ok || !nativePositiveID(field) || !nativePositiveID(choice) || seen[field] || index == 0 && field != root {
			return false
		}
		seen[field] = true
	}
	return true
}
