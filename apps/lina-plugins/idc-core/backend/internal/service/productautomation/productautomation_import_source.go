// This file joins an already authorized version to its immutable upstream
// import evidence. A single indexed lookup survives internal edit snapshots;
// no supplier read, source secrets or row-dependent queries are involved.

package productautomation

import (
	"context"
	"encoding/json"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (s *serviceImpl) importedProductReference(ctx context.Context, version, interfaceNo string) (string, string, error) {
	var row *struct {
		Reference      string `orm:"upstream_reference"`
		SourceSnapshot string `orm:"source_snapshot"`
	}
	err := s.database.Model(do.CatalogProductImport{}).Ctx(ctx).As("i").
		InnerJoin("plugin_idc_core_catalog_product_version", "v", "v.product_no=i.product_no AND v.tenant_id=i.tenant_id").
		Fields("i.upstream_reference,i.source_snapshot").Where("i.tenant_id", 0).Where("v.version_no", version).
		Where("i.interface_instance_no", interfaceNo).OrderDesc("i.id").Limit(1).Scan(&row)
	if err != nil {
		return "", "", bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if row == nil {
		return "", "", nil
	}
	var source struct {
		Product struct {
			Name string `json:"name"`
		} `json:"product"`
	}
	if err := json.Unmarshal([]byte(row.SourceSnapshot), &source); err != nil {
		return "", "", bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	return row.Reference, source.Product.Name, nil
}

// readImportedConfiguration shares the authoritative source lock across reads
// and writes. A previous source remains protected after internal version copies.
func (s *serviceImpl) readImportedConfiguration(ctx context.Context, version, interfaceNo string, exact *capmodel.BusinessExtensionProviderEvidence) (bool, string, string, error) {
	var imported bool
	if supportsPortableImport(exact) {
		var err error
		imported, err = s.database.Model(do.CatalogImportContract{}).Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: version, InterfaceInstanceNo: interfaceNo}).Exist()
		if err != nil {
			return false, "", "", bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
		}
	}
	reference, name, err := s.importedProductReference(ctx, version, interfaceNo)
	return imported || reference != "", reference, name, err
}

// interfaceCatalogAvailability reads one bounded owner projection for the current
// authorized page. Product-fulfillment methods cannot prove resource catalog
// methods; both declarations belong to distinct extension points.
func (s *serviceImpl) interfaceCatalogAvailability(ctx context.Context, rows []*entity.ProductAutomationInterfaceInstance) (map[string]bool, error) {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.InterfaceOriginCode == "resource" {
			ids = append(ids, row.InterfaceInstanceNo)
		}
	}
	if len(ids) == 0 || s.resourceCandidates == nil {
		return map[string]bool{}, nil
	}
	result, err := s.resourceCandidates.CatalogAvailability(ctx, ids)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	return result, nil
}
