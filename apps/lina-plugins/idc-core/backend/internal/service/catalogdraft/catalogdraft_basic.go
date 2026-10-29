// This file commits the basic-information form in one owner transaction.
// Existing section services retain their exact permissions, audits and revisions.
package catalogdraft

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

// BasicSettingsInput contains only changed optional sections. Nil preserves them.
type BasicSettingsInput struct {
	Description     catalog.ProductDescriptionUpdateInput
	VersionNo       string
	Inventory       *catalog.ProductVersionInventoryUpdateInput
	AddonProductNos *[]string
	LoginRules      *BasicLoginRulesInput
}

// BasicLoginRulesInput replaces only login rules under configuration concurrency.
type BasicLoginRulesInput struct {
	ExpectedVersion int64
	Values          []productautomation.ConfigurationValue
}

// SaveBasicSettings atomically saves the form, preserving section-specific
// authority. Failed writes, stale inputs and apply errors roll back all sections.
func SaveBasicSettings(ctx context.Context, input BasicSettingsInput, writer catalog.ManagementService, publication catalog.PublicationService, automation productautomation.Service) (int64, error) {
	var version int64
	save := func(ctx context.Context) error {
		var err error
		version, err = writer.UpdateProductDescription(ctx, input.Description)
		if err != nil {
			return err
		}
		if input.AddonProductNos != nil {
			version, err = writer.SetProductAddons(ctx, catalog.ProductAddonsInput{ProductNo: input.Description.ProductNo, Version: version, ProductNos: *input.AddonProductNos})
			if err != nil {
				return err
			}
		}
		if input.Inventory != nil || input.LoginRules != nil {
			// The route product and editable version must identify the same owner.
			count, loadErr := dao.CatalogProductVersion.Ctx(ctx).Where(do.CatalogProductVersion{TenantId: 0, ProductNo: input.Description.ProductNo, VersionNo: input.VersionNo}).Count()
			if loadErr != nil {
				return bizerr.WrapCode(loadErr, catalog.CodeManagementUnavailable)
			}
			if input.VersionNo == "" || count != 1 {
				return bizerr.NewCode(catalog.CodeManagementInvalid)
			}
		}
		if input.Inventory != nil {
			stock := *input.Inventory
			stock.VersionNo = input.VersionNo
			if _, err = writer.UpdateProductVersionInventory(ctx, stock); err != nil {
				return err
			}
		}
		if input.LoginRules != nil {
			if err = saveBasicLoginRules(ctx, input.VersionNo, input.LoginRules, automation); err != nil {
				return err
			}
		}
		return nil
	}
	if input.Inventory != nil || input.LoginRules != nil {
		if err := publication.WithSavedSettings(ctx, input.VersionNo, save); err != nil {
			return 0, err
		}
		// Applying the saved version advances the product revision once more.
		return version + 1, nil
	}
	// Metadata and companion products need no editable-version permission.
	err := dao.CatalogProduct.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return save(ctx) })
	return version, err
}

func saveBasicLoginRules(ctx context.Context, versionNo string, input *BasicLoginRulesInput, automation productautomation.Service) error {
	if len(input.Values) > 8 {
		return bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	configuration, err := automation.GetMerchantConfiguration(ctx, versionNo)
	if err != nil {
		return err
	}
	if configuration == nil || configuration.Version != input.ExpectedVersion {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	allowed := map[string]bool{"hostname_customer_input": true, "password_customer_input": true, "hostname_prefix": true, "hostname_length": true, "hostname_characters": true, "password_prefix": true, "password_length": true, "password_characters": true}
	values := make([]productautomation.ConfigurationValue, 0, len(configuration.MerchantValues)+len(input.Values))
	for _, value := range configuration.MerchantValues {
		if !allowed[value.FieldKey] {
			values = append(values, value)
		}
	}
	seen := make(map[string]bool, len(input.Values))
	for _, value := range input.Values {
		if !allowed[value.FieldKey] || seen[value.FieldKey] {
			return bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		seen[value.FieldKey] = true
		values = append(values, value)
	}
	return automation.SaveVersionConfiguration(ctx, productautomation.SaveVersionConfigurationInput{VersionNo: versionNo, ExpectedVersion: input.ExpectedVersion, MerchantValues: values, CustomerFieldSelections: configuration.CustomerFieldSelections})
}
