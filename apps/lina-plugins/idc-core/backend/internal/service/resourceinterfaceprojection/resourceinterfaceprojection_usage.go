// This file projects current local server and effective product usage in two
// bounded aggregate queries. Staff scope is checked before any database read;
// no inventory, provider request, draft version, or historical binding is used.

package resourceinterfaceprojection

import (
	"context"
	"regexp"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

// usageChannelPattern mirrors the resource owner's ValidateChannelRef contract.
// This read-only projection accepts opaque channel names, not just newly created
// CHN_ numbers. It does not admit an automation binding or supplier operation.
var usageChannelPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{0,255}$`)

// usageInterfaceOrigin names the existing resource-owned interface discriminator.
type usageInterfaceOrigin string

const resourceUsageOrigin usageInterfaceOrigin = "resource"

// BatchGet authorizes first, then aggregates only the requested channel set.
func (reader *usageReader) BatchGet(ctx context.Context, channelNos []string) ([]resourceinterfaceprojectioncap.Usage, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(codeUsageNotAccessible)
	}
	actor, err := reader.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: "idc:admin:resource:configuration:list", TargetNo: "provider-configurations",
	})
	if err != nil || actor <= 0 {
		return nil, bizerr.NewCode(codeUsageNotAccessible)
	}
	if len(channelNos) > 100 {
		return nil, bizerr.NewCode(codeUsageInvalid)
	}
	var (
		result = make([]resourceinterfaceprojectioncap.Usage, len(channelNos))
		seen   = make(map[string]int, len(channelNos))
	)
	for index, channelNo := range channelNos {
		if !usageChannelPattern.MatchString(channelNo) {
			return nil, bizerr.NewCode(codeUsageInvalid)
		}
		if _, exists := seen[channelNo]; exists {
			return nil, bizerr.NewCode(codeUsageInvalid)
		}
		seen[channelNo] = index
		result[index].ChannelNo = channelNo
	}
	if len(result) == 0 {
		return result, nil
	}
	var servers []struct {
		ChannelNo       string
		ActiveInstances int
		TotalInstances  int
	}
	err = reader.database.Model(dao.ServiceInstance.Table()).Safe().Ctx(ctx).
		Where(do.ServiceInstance{TenantId: idccap.DefaultTenantID, ResourceOwnerCode: resourceinterfaceprojectioncap.OwnerID}).
		WhereIn("channel_no", channelNos).
		Fields("channel_no, COUNT(*) AS total_instances, SUM(CASE WHEN entitlement_status_code='active' THEN 1 ELSE 0 END) AS active_instances").
		Group("channel_no").Scan(&servers)
	if err != nil {
		return nil, bizerr.WrapCode(err, codeUsageUnavailable)
	}
	var products []struct {
		ChannelNo          string
		ConfiguredProducts int
	}
	err = reader.database.Model(dao.CatalogProduct.Table()+" product").Safe().Ctx(ctx).
		InnerJoin("plugin_idc_core_catalog_product_version version",
			"version.tenant_id=product.tenant_id AND version.product_no=product.product_no AND version.version_no=product.current_version_no").
		InnerJoin("plugin_idc_core_catalog_product_version_fulfillment_binding binding",
			"binding.tenant_id=version.tenant_id AND binding.version_no=version.version_no").
		InnerJoin("plugin_idc_core_product_automation_interface_instance interface",
			"interface.tenant_id=binding.tenant_id AND interface.interface_instance_no=binding.interface_instance_no").
		WherePrefix("product", do.CatalogProduct{TenantId: idccap.DefaultTenantID}).
		WherePrefix("interface", do.ProductAutomationInterfaceInstance{ResourceOwnerId: resourceinterfaceprojectioncap.OwnerID, InterfaceOriginCode: resourceUsageOrigin}).
		WhereIn("interface.resource_channel_ref", channelNos).
		Fields("interface.resource_channel_ref AS channel_no, COUNT(DISTINCT product.product_no) AS configured_products").
		Group("interface.resource_channel_ref").Scan(&products)
	if err != nil {
		return nil, bizerr.WrapCode(err, codeUsageUnavailable)
	}
	for _, row := range servers {
		index := seen[row.ChannelNo]
		result[index].ActiveInstances = row.ActiveInstances
		result[index].TotalInstances = row.TotalInstances
	}
	for _, row := range products {
		result[seen[row.ChannelNo]].ConfiguredProducts = row.ConfiguredProducts
	}
	return result, nil
}
