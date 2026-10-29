// Remote pricing readiness checks source-owned periods and merchant rules,
// without manufacturing static configuration amounts for publication.
package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

func loadRemotePublicationPricing(ctx context.Context, version string, graph *catalogPublicationGraph) error {
	var contract *struct{ Snapshot, SnapshotDigest, ReadEvidence string }
	if err := dao.CatalogImportContract.Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: version}).Fields("snapshot,snapshot_digest,read_evidence").Scan(&contract); err != nil {
		return err
	}
	if contract == nil {
		return nil
	}
	var snapshot channelcommercecap.ImportSnapshot
	if json.Unmarshal([]byte(contract.Snapshot), &snapshot) != nil {
		return bizerr.NewCode(CodePublicationIncomplete)
	}
	if snapshot.PricingMode != channelcommercecap.ImportPricingRemoteEvaluation {
		return nil
	}
	var read resourceinterfaceprojectioncap.CatalogReadEvidence
	if snapshot.Validate() != nil || json.Unmarshal([]byte(contract.ReadEvidence), &read) != nil || read.Validate() != nil || !read.SaleImportProtocol.NativePurchase() {
		return bizerr.NewCode(CodePublicationIncomplete)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != contract.SnapshotDigest {
		return bizerr.NewCode(CodePublicationIncomplete)
	}
	if err = dao.CatalogImportTerm.Ctx(ctx).Where(do.CatalogImportTerm{TenantId: 0, VersionNo: version}).Limit(maximumPublicationPrices + 1).Scan(&graph.importTerms); err != nil {
		return err
	}
	if len(graph.importTerms) > maximumPublicationPrices {
		return bizerr.NewCode(CodePublicationIncomplete)
	}
	graph.remoteSnapshot = &snapshot
	return nil
}
func completeRemotePublicationPricing(graph *catalogPublicationGraph) bool {
	if graph.remoteSnapshot == nil || len(graph.importTerms) != len(graph.remoteSnapshot.Prices) {
		return false
	}
	rules := map[string]BillingRuleDefinition{}
	for _, rule := range graph.billingRules {
		rules[rule.BillingRuleNo] = BillingRuleDefinitionFromEntity(rule)
	}
	source := map[string]channelcommercecap.ImportPriceSchedule{}
	for _, schedule := range graph.remoteSnapshot.Prices {
		source[schedule.CurrencyCode+"\x00"+schedule.TermReference] = schedule
	}
	activeBases := map[string]bool{}
	for _, price := range graph.prices {
		if price.OptionNo == nil || *price.OptionNo == "" {
			activeBases[price.BillingRuleNo+"\x00"+price.CurrencyCode] = true
		}
	}
	seen := map[string]bool{}
	for _, term := range graph.importTerms {
		key := term.CurrencyCode + "\x00" + term.UpstreamTermReference
		schedule, found := source[key]
		if !found || seen[key] {
			return false
		}
		seen[key] = true
		definition, found := rules[term.BillingRuleNo]
		if !found || ValidateBillingRule(definition) != nil || definition.ChargeType != ChargeType(schedule.ChargeType) || definition.PeriodUnit != PeriodUnit(schedule.PeriodUnit) || definition.PeriodCount != schedule.PeriodCount || definition.MinimumPurchaseCycles != 1 {
			return false
		}
		if _, err := parseImportPriceDecimal(term.MarkupPercent); err != nil {
			return false
		}
		if term.WholeItemRule != "" && term.WholeItemRule != "null" {
			var rule channelcommercecap.ImportWholeItemRule
			if json.Unmarshal([]byte(term.WholeItemRule), &rule) != nil || rule.Validate() != nil || (rule.Kind == "fixed" && !activeBases[term.BillingRuleNo+"\x00"+term.CurrencyCode]) {
				return false
			}
		}
	}
	return true
}
