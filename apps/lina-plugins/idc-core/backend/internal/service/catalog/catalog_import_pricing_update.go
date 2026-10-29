// An existing imported draft may change its merchant pricing policy without
// replacing source-owned fields, bindings, mappings, periods or credentials.
package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (s *managementServiceImpl) updateNativeImportPricing(ctx context.Context, input ImportInput, source *resourceinterfaceprojectioncap.CatalogDetail, prices []importPrice, actor int, digest string) (*ImportResult, bool, error) {
	var contract *entity.CatalogImportContract
	if err := dao.CatalogImportContract.Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: input.TargetVersionNo}).LockUpdate().Scan(&contract); err != nil {
		return nil, false, err
	}
	if contract == nil {
		return nil, false, nil
	}
	var frozen channelcommercecap.ImportSnapshot
	var read resourceinterfaceprojectioncap.CatalogReadEvidence
	if json.Unmarshal([]byte(contract.Snapshot), &frozen) != nil || frozen.Validate() != nil || json.Unmarshal([]byte(contract.ReadEvidence), &read) != nil || read.Validate() != nil || source == nil || source.ImportSnapshot == nil || source.ReadEvidence == nil || contract.InterfaceInstanceNo != input.InterfaceInstanceNo || input.Reference != frozen.Product.PublicSlug || !reflect.DeepEqual(read, *source.ReadEvidence) || !sameImportedPricingSource(frozen, *source.ImportSnapshot) {
		return nil, true, bizerr.NewCode(CodeUpstreamConfigurationLocked)
	}
	raw, hashErr := json.Marshal(frozen)
	if hashErr != nil {
		return nil, true, hashErr
	}
	sourceDigest := sha256.Sum256(raw)
	if hex.EncodeToString(sourceDigest[:]) != contract.SnapshotDigest {
		return nil, true, bizerr.NewCode(CodeManagementConflict)
	}
	current, err := s.authorizer.AuthorizePriceWrite(ctx, input.TargetVersionNo)
	if err != nil || current != actor {
		return nil, true, bizerr.NewCode(CodeManagementNotAccessible)
	}
	if replay, replayErr := loadImportedPriceCommand(ctx, actor, input, digest); replayErr != nil || replay != nil {
		return replay, true, replayErr
	}
	target, product, err := lockCatalogVersionTarget(ctx, input.TargetVersionNo, false)
	if err != nil {
		return nil, true, err
	}
	if target.StatusCode != catalogStatusDraft || product.CategoryNo != input.CategoryNo {
		return nil, true, bizerr.NewCode(CodeManagementConflict)
	}
	var terms []entity.CatalogImportTerm
	if err = dao.CatalogImportTerm.Ctx(ctx).Where(do.CatalogImportTerm{TenantId: 0, VersionNo: target.VersionNo}).Limit(maximumPublicationPrices + 1).LockUpdate().Scan(&terms); err != nil {
		return nil, true, err
	}
	if len(terms) != len(prices) {
		return nil, true, bizerr.NewCode(CodeManagementConflict)
	}
	var dimensions []configurationPriceDimensionIdentity
	if err = dao.CatalogProduct.DB().GetScan(ctx, &dimensions, `SELECT dimension_no,field_key,candidate_value,pricing_mode_code FROM plugin_idc_core_catalog_configuration_price_dimension WHERE tenant_id=0 AND version_no=? LIMIT ?`, target.VersionNo, maximumConfigurationPriceDimensions+1); err != nil {
		return nil, true, err
	}
	if len(dimensions) > maximumConfigurationPriceDimensions {
		return nil, true, bizerr.NewCode(CodeManagementConflict)
	}
	rows, updates, err := s.importedPricePolicyRows(target.VersionNo, input, source.ImportSnapshot, terms, dimensions, prices)
	if err != nil {
		return nil, true, err
	}
	if _, err = dao.CatalogPrice.Ctx(ctx).Where(do.CatalogPrice{TenantId: 0, VersionNo: target.VersionNo}).Delete(); err != nil {
		return nil, true, err
	}
	if len(rows.prices) > 0 {
		result, writeErr := dao.CatalogPrice.Ctx(ctx).Data(rows.prices).Batch(256).Insert()
		if writeErr = requireCatalogBatchInsert(result, writeErr, len(rows.prices), "update imported merchant prices"); writeErr != nil {
			return nil, true, writeErr
		}
	}
	if err = writeImportedPricePolicyTerms(ctx, target.VersionNo, updates); err != nil {
		return nil, true, err
	}
	if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "catalog.import-price.save", IdempotencyKey: input.RequestKey, OperationCode: "catalog.import-price.save", TargetType: "catalog_product_version", TargetNo: target.VersionNo, ResultCode: "succeeded", ReasonCode: "manual_save", RiskCode: "high", RequiredPermission: permissionCatalogPriceWrite, BeforeSnapshot: map[string]any{"pricing": terms}, AfterSnapshot: map[string]any{"pricing": updates}}); err != nil {
		return nil, true, err
	}
	result := &ImportResult{PricingOnly: true, Item: ProductDraft{ProductNo: product.ProductNo, VersionNo: target.VersionNo, CategoryNo: product.CategoryNo, VersionNumber: target.VersionNumber, Status: product.StatusCode, VersionStatus: target.StatusCode, Source: target.SourceCode}, MissingConfiguration: []ImportMissingConfiguration{}}
	if err = insertImportedPriceCommand(ctx, actor, input, digest); err != nil {
		return nil, true, err
	}
	return result, true, nil
}
func sameImportedPricingSource(frozen, current channelcommercecap.ImportSnapshot) bool {
	if current.Validate() != nil || frozen.Product.PublicSlug != current.Product.PublicSlug || frozen.ConfigurationRevision != current.ConfigurationRevision || !reflect.DeepEqual(frozen.Schema, current.Schema) || !reflect.DeepEqual(frozen.Mappings, current.Mappings) || !reflect.DeepEqual(frozen.Currencies, current.Currencies) || !reflect.DeepEqual(frozen.Fulfillment, current.Fulfillment) || len(frozen.Prices) != len(current.Prices) {
		return false
	}
	normalize := func(prices []channelcommercecap.ImportPriceSchedule) map[string]channelcommercecap.ImportPriceSchedule {
		result := map[string]channelcommercecap.ImportPriceSchedule{}
		for _, price := range prices {
			price.Base = nil
			price.Options = nil
			price.WholeItemRule = nil
			result[price.CurrencyCode+"\x00"+price.TermReference] = price
		}
		return result
	}
	return reflect.DeepEqual(normalize(frozen.Prices), normalize(current.Prices))
}

type importedPricePolicyUpdate struct {
	BillingRuleNo string          `json:"billing_rule_no"`
	CurrencyCode  string          `json:"currency_code"`
	MarkupPercent string          `json:"markup_percent"`
	WholeItemRule json.RawMessage `json:"whole_item_rule"`
}

func (s *managementServiceImpl) importedPricePolicyRows(version string, input ImportInput, snapshot *channelcommercecap.ImportSnapshot, terms []entity.CatalogImportTerm, dimensions []configurationPriceDimensionIdentity, prices []importPrice) (*portableImportRows, []importedPricePolicyUpdate, error) {
	rules := map[string]string{}
	for _, term := range terms {
		rules[term.CurrencyCode+"\x00"+term.UpstreamTermReference] = term.BillingRuleNo
	}
	dimensionByKey := map[string]string{}
	for _, dimension := range dimensions {
		dimensionByKey[dimension.FieldKey+"\x00"+dimension.CandidateValue] = dimension.DimensionNo
	}
	rows := &portableImportRows{}
	updates := make([]importedPricePolicyUpdate, 0, len(prices))
	markup := input.MarkupPercent
	if markup == "" {
		markup = "0"
	}
	for _, price := range prices {
		p := price.portable
		if p == nil {
			return nil, nil, bizerr.NewCode(CodeManagementInvalid)
		}
		rule := rules[p.CurrencyCode+"\x00"+p.TermReference]
		if rule == "" {
			return nil, nil, bizerr.NewCode(CodeManagementConflict)
		}
		definition := portableImportedBillingDefinition(snapshot, *p)
		if len(p.Base) > 0 {
			if err := s.appendPortableBands(rows, version, rule, p.CurrencyCode, "", ConfigurationPriceFixed, definition, p.Base); err != nil {
				return nil, nil, err
			}
		}
		for _, option := range p.Options {
			candidate := option.Value
			if option.PricingMode != "" && option.PricingMode != channelcommercecap.ImportConfigurationPriceFixed {
				candidate = "quantity"
			}
			dimension := dimensionByKey[option.FieldKey+"\x00"+candidate]
			if dimension == "" {
				return nil, nil, bizerr.NewCode(CodeManagementConflict)
			}
			if err := s.appendPortableBands(rows, version, rule, p.CurrencyCode, dimension, ConfigurationPriceMode(option.PricingMode), definition, option.Bands); err != nil {
				return nil, nil, err
			}
		}
		raw, err := json.Marshal(p.WholeItemRule)
		if err != nil {
			return nil, nil, err
		}
		updates = append(updates, importedPricePolicyUpdate{BillingRuleNo: rule, CurrencyCode: p.CurrencyCode, MarkupPercent: markup, WholeItemRule: raw})
	}
	return rows, updates, nil
}
func writeImportedPricePolicyTerms(ctx context.Context, version string, updates []importedPricePolicyUpdate) error {
	raw, err := json.Marshal(updates)
	if err != nil {
		return err
	}
	result, err := dao.CatalogImportTerm.DB().Exec(ctx, `WITH desired AS (SELECT item.billing_rule_no,item.currency_code,item.markup_percent,item.whole_item_rule FROM jsonb_to_recordset(CAST(? AS jsonb)) AS item(billing_rule_no TEXT,currency_code TEXT,markup_percent TEXT,whole_item_rule JSONB)) UPDATE plugin_idc_core_catalog_import_term term SET markup_percent=desired.markup_percent,whole_item_rule=NULLIF(desired.whole_item_rule,'null'::jsonb) FROM desired WHERE term.tenant_id=0 AND term.version_no=? AND term.billing_rule_no=desired.billing_rule_no AND term.currency_code=desired.currency_code`, string(raw), version)
	return requireNativeWholeItemUpdates(result, err, len(updates))
}
