// Live resource products use the current connection while paid orders retain
// their immutable connection facts. Other interfaces require proven equivalence.
package productautomation

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// validateConfigurationInterfaceRevision separates live resource routing from historical purchase snapshots.
func (s *serviceImpl) validateConfigurationInterfaceRevision(ctx context.Context, interfaceNo string, row *entity.CatalogProductVersionConfiguration, current *entity.ProductAutomationInterfaceConfigurationRevision) error {
	if row == nil || current == nil || row.InterfaceInstanceNo != interfaceNo {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if row.InterfaceConfigurationRevision == current.Revision {
		return nil
	}
	// A live product owns its specifications and prices, not a historical
	// transport address. New purchases bind the current connection at checkout;
	// existing orders retain their own immutable connection revision.
	resourceOwned, ownerErr := s.resourceInterfaceOwned(ctx, interfaceNo)
	if ownerErr != nil {
		return bizerr.WrapCode(ownerErr, catalog.CodeManagementUnavailable)
	}
	if resourceOwned && current.ResourceOwnerConfigurationRevision != "" && current.SchemaVersion == "resource-owner-v1" {
		return nil
	}
	comparer, ok := s.resourceCandidates.(resourceinterfaceprojectioncap.ConfigurationRevisionComparer)
	if !ok || current.ResourceOwnerConfigurationRevision == "" {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	var saved *entity.ProductAutomationInterfaceConfigurationRevision
	err := s.database.Model(do.ProductAutomationInterfaceConfigurationRevision{}).Ctx(ctx).
		Where(do.ProductAutomationInterfaceConfigurationRevision{TenantId: idccap.DefaultTenantID, InterfaceInstanceNo: interfaceNo, Revision: row.InterfaceConfigurationRevision}).Scan(&saved)
	if err != nil {
		return bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if saved == nil || saved.ResourceOwnerConfigurationRevision == "" || saved.SchemaVersion != current.SchemaVersion {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	equivalent, err := comparer.EquivalentConfigurationRevisions(ctx, interfaceNo, saved.ResourceOwnerConfigurationRevision, current.ResourceOwnerConfigurationRevision)
	if err != nil {
		return bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if !equivalent {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	return nil
}
