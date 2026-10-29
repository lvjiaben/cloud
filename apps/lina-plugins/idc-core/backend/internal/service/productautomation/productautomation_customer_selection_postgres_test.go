// This file verifies persisted merchant configuration and presentation remain
// readable without weakening provider technical bounds or frozen snapshots.
package productautomation

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func TestMerchantConfigurationSurvivesProviderDescriptionUpdatePostgreSQL(t *testing.T) {
	ctx := context.Background()
	database := portableBindingDatabase(t, ctx)
	exact := automationEvidence([]string{"provision", "query.status"})
	local := customerSelectionEvidence(t)
	exact.CustomerConfiguration, exact.ProviderMetadata = local.CustomerConfiguration, local.ProviderMetadata
	exact.CustomerConfiguration.Fields[1].Description = &capmodel.LocalizedText{ZhCN: "留空使用套餐默认配置", EnUS: "Leave blank for package size"}
	extensions := &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{exact}}
	service, err := newWithRandom(database, extensions, &automationTestAuthorizer{}, &automationTestAudit{}, &automationTestCodec{evidence: exact}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateInterface(ctx, CreateInterfaceInput{DisplayName: "Description update fixture", SelectionToken: "opaque"})
	if err != nil {
		t.Fatal(err)
	}
	versionNo := "ver_" + strings.Repeat("a", 32)
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no,status_code) VALUES(0,?,'draft')`, versionNo); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Bind(ctx, BindInput{VersionNo: versionNo, InterfaceInstanceNo: created.InterfaceInstanceNo}); err != nil {
		t.Fatal(err)
	}
	if err = service.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{VersionNo: versionNo, CustomerFieldSelections: []CustomerFieldSelection{{FieldKey: "cpu", Minimum: "2", Maximum: "4", DescriptionZhCN: "选择核心数", DescriptionEnUS: "Choose cores"}}}); err != nil {
		t.Fatal(err)
	}
	before, err := service.loadVersionConfigurationRow(ctx, versionNo)
	if err != nil || before == nil {
		t.Fatal("missing saved configuration", err)
	}
	for _, description := range []*capmodel.LocalizedText{nil, {ZhCN: "新的插件说明", EnUS: "Updated provider help"}} {
		exact.CustomerConfiguration.Fields[1].Description = description
		view, readErr := service.GetVersionConfiguration(ctx, versionNo)
		if readErr != nil || view == nil || view.Version != 1 || len(view.CustomerFieldSelections) != 1 {
			t.Fatal("existing product stopped opening after help text update", readErr)
		}
		selection := view.CustomerFieldSelections[0]
		if selection.Minimum != "2" || selection.Maximum != "4" || selection.DescriptionZhCN != "选择核心数" || selection.DescriptionEnUS != "Choose cores" {
			t.Fatalf("saved limits or merchant description changed: %+v", selection)
		}
	}
	exact.CustomerConfiguration.Fields[1].Minimum = "5"
	if _, err = service.GetVersionConfiguration(ctx, versionNo); !bizerr.Is(err, CodeEvidenceUnavailable) {
		t.Fatal("a real provider bound change was accepted", err)
	}
	after, err := service.loadVersionConfigurationRow(ctx, versionNo)
	if err != nil || after.CustomerDeclarationSnapshot != before.CustomerDeclarationSnapshot || after.CustomerPresentationSnapshot != before.CustomerPresentationSnapshot || after.Version != before.Version {
		t.Fatal("read rewrote saved configuration", err)
	}
	// A merchant can still save a real edit after the provider corrects help text.
	exact.CustomerConfiguration.Fields[1].Minimum = "1"
	if err = service.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{VersionNo: versionNo, ExpectedVersion: 1, CustomerFieldSelections: []CustomerFieldSelection{{FieldKey: "cpu", Minimum: "2", Maximum: "3", DescriptionZhCN: "修改后的说明", DescriptionEnUS: "Updated merchant help"}}}); err != nil {
		t.Fatal("editing a restored product failed", err)
	}
	edited, err := service.GetVersionConfiguration(ctx, versionNo)
	if err != nil || edited.Version != 2 || edited.CustomerFieldSelections[0].Maximum != "3" || edited.CustomerFieldSelections[0].DescriptionZhCN != "修改后的说明" {
		t.Fatal("saved edit was not restored", err)
	}
}

func TestLocalControlsSaveRestoreCopyAndFreezePostgreSQL(t *testing.T) {
	ctx := context.Background()
	database := portableBindingDatabase(t, ctx)
	exact := automationEvidence([]string{"provision", "query.status"})
	local := customerSelectionEvidence(t)
	exact.CustomerConfiguration = local.CustomerConfiguration
	exact.ProviderMetadata = local.ProviderMetadata
	extensions := &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{exact}}
	service, err := newWithRandom(database, extensions, &automationTestAuthorizer{}, &automationTestAudit{}, &automationTestCodec{evidence: exact}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateInterface(ctx, CreateInterfaceInput{DisplayName: "Local widget fixture", SelectionToken: "opaque"})
	if err != nil {
		t.Fatal(err)
	}
	sourceNo := "ver_" + strings.Repeat("a", 32)
	targetNo := "ver_" + strings.Repeat("b", 32)
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no,status_code) VALUES(0,?,'draft')`, sourceNo); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Bind(ctx, BindInput{VersionNo: sourceNo, InterfaceInstanceNo: created.InterfaceInstanceNo}); err != nil {
		t.Fatal(err)
	}
	disabled := false
	selections := []CustomerFieldSelection{{CustomerVisible: &disabled, DefaultValue: json.RawMessage(`4`), UpgradeEnabled: &disabled, UpgradeMinimum: "4", UpgradeMaximum: "16", FieldKey: "cpu", Minimum: "2", Maximum: "16", DisplayControl: configurationcontract.ControlRadio, QuantityStep: "2", NumericChoices: []string{"2", "4", "8"}, NumericPricing: configurationcontract.NumericPricingChoice, NumericLabels: map[string]*capmodel.LocalizedText{"2": {ZhCN: "2核", EnUS: "2 cores"}}},
		{FieldKey: "local_support", ProductFieldKey: "vendor_support", ProductChoiceBindings: map[string]string{"basic": "vendor-basic"}, UpgradeCandidateValues: []string{"basic"}, GroupZhCN: "服务", GroupEnUS: "Services", Advanced: true, LocalField: &capmodel.ConfigurationFieldDeclaration{Key: "local_support", Type: capmodel.ConfigurationFieldSingleSelect, Required: true, Label: &capmodel.LocalizedText{ZhCN: "支持等级", EnUS: "Support level"}, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "basic", Label: &capmodel.LocalizedText{ZhCN: "基础", EnUS: "Basic"}}}}}}
	input := SaveVersionConfigurationInput{VersionNo: sourceNo, ExpectedVersion: 0, CustomerFieldSelections: selections}
	if err = service.SaveVersionConfiguration(ctx, input); err != nil {
		t.Fatal(err)
	}
	saved, err := service.GetVersionConfiguration(ctx, sourceNo)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.CustomerFieldSelections) != 2 || saved.CustomerFieldSelections[0].DisplayControl != configurationcontract.ControlRadio || saved.CustomerFieldSelections[0].QuantityStep != "2" || len(saved.CustomerFieldSelections[0].NumericChoices) != 3 {
		t.Fatalf("local controls lost on restore: %+v", saved.CustomerFieldSelections)
	}
	if saved.CustomerFieldSelections[1].LocalField == nil || !saved.CustomerFieldSelections[1].Advanced || saved.CustomerFieldSelections[1].GroupZhCN != "服务" {
		t.Fatal("local definition or presentation lost")
	}
	if saved.CustomerFieldSelections[0].CustomerVisible == nil || *saved.CustomerFieldSelections[0].CustomerVisible || string(saved.CustomerFieldSelections[0].DefaultValue) != "4" {
		t.Fatal("hidden default lost on save/restore")
	}
	if saved.CustomerFieldSelections[0].UpgradeEnabled == nil || *saved.CustomerFieldSelections[0].UpgradeEnabled || saved.CustomerFieldSelections[0].UpgradeMinimum != "4" || saved.CustomerFieldSelections[1].ProductFieldKey != "vendor_support" || saved.CustomerFieldSelections[1].ProductChoiceBindings["basic"] != "vendor-basic" || len(saved.CustomerFieldSelections[1].UpgradeCandidateValues) != 1 {
		t.Fatalf("custom bindings or upgrade policy lost: %+v", saved.CustomerFieldSelections)
	}
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_configuration_price_dimension(id BIGSERIAL PRIMARY KEY,tenant_id INT,dimension_no VARCHAR(40),version_no VARCHAR(40),configuration_version BIGINT,field_key VARCHAR(128),candidate_value VARCHAR(256),field_label_zh_cn VARCHAR(128),field_label_en_us VARCHAR(128),candidate_label_zh_cn VARCHAR(128),candidate_label_en_us VARCHAR(128),pricing_mode_code VARCHAR(16),status_code VARCHAR(16))`); err != nil {
		t.Fatal(err)
	}
	candidates, candidateErr := service.ListConfigurationPriceCandidates(ctx, sourceNo)
	if candidateErr != nil {
		t.Fatal(candidateErr)
	}
	count := 0
	for _, candidate := range candidates {
		if candidate.FieldToken == configurationPriceCandidateToken(sourceNo, saved.Version, "cpu", "") {
			if candidate.Quantity {
				t.Fatal("fixed specification became per-unit")
			}
			count++
		}
	}
	if count != 3 {
		t.Fatalf("want three discrete prices, got %d", count)
	}
	if _, err = database.Exec(ctx, `ALTER TABLE plugin_idc_core_catalog_product_version ADD COLUMN source_code VARCHAR(16) DEFAULT 'manual'`); err != nil {
		t.Fatal(err)
	}
	tokens := []string{}
	for _, candidate := range candidates {
		if candidate.FieldToken == configurationPriceCandidateToken(sourceNo, saved.Version, "cpu", "") {
			tokens = append(tokens, candidate.CandidateToken)
		}
	}
	dimensions, dimensionErr := service.ResolvePriceTargets(ctx, sourceNo, tokens)
	if dimensionErr != nil || len(dimensions) != 3 {
		t.Fatalf("fixed targets: %+v %v", dimensions, dimensionErr)
	}
	again, dimensionErr := service.ResolvePriceTargets(ctx, sourceNo, tokens)
	if dimensionErr != nil || len(again) != 3 {
		t.Fatal("repeat fixed target resolution failed", dimensionErr)
	}
	if err = service.WithProductCopy(ctx, sourceNo, false, func(txCtx context.Context) (string, error) {
		_, e := database.Exec(txCtx, `INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no,status_code) VALUES(0,?,'draft')`, targetNo)
		return targetNo, e
	}); err != nil {
		t.Fatal(err)
	}
	var source, target *entity.CatalogProductVersionConfiguration
	if err = database.Model(do.CatalogProductVersionConfiguration{}).Ctx(ctx).Where(do.CatalogProductVersionConfiguration{VersionNo: sourceNo}).Scan(&source); err != nil {
		t.Fatal(err)
	}
	if err = database.Model(do.CatalogProductVersionConfiguration{}).Ctx(ctx).Where(do.CatalogProductVersionConfiguration{VersionNo: targetNo}).Scan(&target); err != nil {
		t.Fatal(err)
	}
	if source == nil || target == nil || source.CustomerPresentationSnapshot != target.CustomerPresentationSnapshot {
		t.Fatal("copy dropped local controls")
	}
	// Existing published immutability covers the plugin-owned presentation column.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version SET status_code='published' WHERE version_no=?`, sourceNo); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_configuration SET customer_presentation_snapshot='{}',version=version+1 WHERE version_no=?`, sourceNo); err == nil {
		t.Fatal("published controls were mutable")
	}
	input.VersionNo = targetNo
	input.ExpectedVersion = 1
	input.CustomerFieldSelections[0].NumericChoices = []string{"3"}
	if err = service.SaveVersionConfiguration(ctx, input); err == nil {
		t.Fatal("misaligned numeric choices were saved")
	}
}
