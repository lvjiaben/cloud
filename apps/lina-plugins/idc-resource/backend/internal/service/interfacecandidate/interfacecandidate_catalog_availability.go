// This file projects the resource extension's local catalog methods in one
// bounded query. Product fulfillment declarations are a separate contract.
package interfacecandidate

import (
	"context"
	"regexp"
	"slices"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/model/do"
)

var catalogInterfaceNumber = regexp.MustCompile(`^pfi_[0-9a-f]{32}$`)

func (source *LazySource) CatalogAvailability(ctx context.Context, interfaceNos []string) (map[string]bool, error) {
	if source == nil || ctx == nil || ctx.Err() != nil || len(interfaceNos) > 100 {
		return nil, gerror.New("resource catalog availability request is invalid")
	}
	result := make(map[string]bool, len(interfaceNos))
	for _, id := range interfaceNos {
		if !catalogInterfaceNumber.MatchString(id) {
			return nil, gerror.New("resource catalog interface number is invalid")
		}
		result[id] = false
	}
	if len(interfaceNos) == 0 {
		return result, nil
	}
	source.mu.RLock()
	deps := source.deps
	source.mu.RUnlock()
	if deps == nil {
		return nil, gerror.New("resource catalog availability is unavailable")
	}
	var rows []struct {
		InterfaceNo string   `orm:"interface_instance_no"`
		Methods     []string `orm:"resource_authorized_methods"`
	}
	err := deps.database.Model(do.Channel{}).Ctx(ctx).As("c").
		InnerJoin("plugin_idc_resource_interface_channel_binding b", "c.tenant_id=b.tenant_id AND c.channel_no=b.channel_no").
		Fields("b.interface_instance_no,b.resource_authorized_methods").
		Where("b.tenant_id", idccap.DefaultTenantID).WhereIn("b.interface_instance_no", interfaceNos).
		Where("c.status_code", "enabled").Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "read resource catalog availability")
	}
	for _, row := range rows {
		result[row.InterfaceNo] = slices.Contains(row.Methods, resourcecap.DynamicProviderMethodListCatalog) &&
			slices.Contains(row.Methods, resourcecap.DynamicProviderMethodReadCatalogProduct)
	}
	return result, nil
}
