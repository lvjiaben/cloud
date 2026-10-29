// This file enforces permission-first replay/read ordering and delegates all
// mutation to a single product owner's admission transaction.

package catalogdraft

import (
	"context"
	"errors"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

var errImportReplay = errors.New("catalog import already committed")

const importProductCreatePermission = "idc:admin:catalog:product:create"
const importResourceInterfacePermission = "idc:admin:resource:configuration:list"

func (s *serviceImpl) Import(ctx context.Context, input catalog.ImportInput) (*catalog.ImportResult, error) {
	if input.TargetVersionNo != "" {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	return s.importProduct(ctx, input, nil, nil, "")
}

func (s *serviceImpl) ImportExisting(ctx context.Context, input catalog.ImportInput, publication catalog.PublicationService, automation productautomation.Service, strategy string) (*catalog.ImportResult, error) {
	if input.TargetVersionNo == "" || publication == nil || automation == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	input.FulfillmentStrategyCode = strategy
	return s.importProduct(ctx, input, publication, automation, strategy)
}

func (s *serviceImpl) importProduct(ctx context.Context, input catalog.ImportInput, publication catalog.PublicationService, automation productautomation.Service, strategy string) (*catalog.ImportResult, error) {
	actor, err := s.authorizeImport(ctx, input.InterfaceInstanceNo)
	if err != nil {
		return nil, err
	}
	previous, err := s.catalog.LookupImport(ctx, input)
	if err != nil || previous != nil {
		return previous, err
	}
	if s.source == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	source, err := s.source.ReadCatalogProduct(ctx, input.InterfaceInstanceNo, resourceinterfaceprojectioncap.CatalogProductQuery{Reference: input.Reference})
	if err != nil {
		return nil, err
	}
	if source == nil || source.Validate(resourceinterfaceprojectioncap.CatalogProductQuery{Reference: input.Reference}) != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	prepared, err := s.automation.PrepareImportBinding(ctx, input.InterfaceInstanceNo, source.ReadEvidence, source.ProductConfiguration, source.ImportSnapshot)
	if err != nil {
		return nil, err
	}
	missing := make([]catalog.ImportMissingConfiguration, 0, len(prepared.MissingConfiguration)+len(source.Options))
	for _, issue := range prepared.MissingConfiguration {
		missing = append(missing, catalog.ImportMissingConfiguration{FieldKey: issue.FieldKey, NameZhCN: issue.NameZhCN, NameEnUS: issue.NameEnUS})
	}
	for _, option := range source.Options {
		missing = append(missing, catalog.ImportMissingConfiguration{NameZhCN: option.Name, NameEnUS: option.Name})
	}
	if input.TargetVersionNo != "" && (len(missing) != 0 || (!source.PriceScheduleComplete && (source.ImportSnapshot == nil || source.ImportSnapshot.Validate() != nil))) {
		return nil, bizerr.NewCode(catalog.CodeUpstreamDefinitionIncomplete)
	}
	var result *catalog.ImportResult
	replayed := false
	write := func(ctx context.Context) error {
		return s.automation.WithImportBinding(ctx, prepared, func(txCtx context.Context) (string, error) {
			current, authorizationErr := s.authorizeImport(txCtx, input.InterfaceInstanceNo)
			if authorizationErr != nil || current != actor {
				return "", bizerr.NewCode(catalog.CodeManagementNotAccessible)
			}
			var created bool
			result, created, err = s.catalog.CommitImport(txCtx, input, source, missing)
			if err != nil {
				return "", err
			}
			if !created {
				replayed = true
				return "", nil
			}
			if result.PricingOnly {
				return "", nil
			}
			return result.Item.VersionNo, nil
		})
	}
	if publication != nil {
		err = publication.WithSavedSettings(ctx, input.TargetVersionNo, func(txCtx context.Context) error {
			if writeErr := write(txCtx); writeErr != nil {
				return writeErr
			}
			if replayed {
				return errImportReplay
			}
			authorization, authErr := automation.AuthorizeFulfillmentPolicyWrite(txCtx, input.TargetVersionNo)
			if authErr != nil {
				return authErr
			}
			_, saveErr := catalog.SaveFulfillmentPolicy(txCtx, catalog.FulfillmentPolicyUpdateInput{VersionNo: input.TargetVersionNo, StrategyCode: strategy}, authorization)
			return saveErr
		})
	} else {
		err = write(ctx)
	}
	if errors.Is(err, errImportReplay) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

// authorizeImport is also repeated after remote preparation, including when a
// concurrent request has already committed and the write becomes a replay.
func (s *serviceImpl) authorizeImport(ctx context.Context, interfaceNo string) (int, error) {
	actor := 0
	for _, permission := range []string{importProductCreatePermission, productautomation.PermissionBindingWrite, productautomation.PermissionInterfaceView, importResourceInterfacePermission} {
		current, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{Permission: permission, TargetNo: interfaceNo})
		if err != nil || current <= 0 || (actor != 0 && current != actor) {
			return 0, bizerr.NewCode(catalog.CodeManagementNotAccessible)
		}
		actor = current
	}
	return actor, nil
}
