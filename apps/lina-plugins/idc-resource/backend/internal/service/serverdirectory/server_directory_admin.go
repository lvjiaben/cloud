// This file exposes a bounded, non-secret ZJMFCloud directory after exact staff authorization.
package serverdirectory

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"strings"
)

const cloudAdminMaximumPageSize = 100

const cloudConnectorPluginID = "idc-zjmfcloud"
const cloudAdminViewPermission = "idc:admin:resource:server-batch:view"

// AdminService is bound once to the owner's directory and trusted staff authorizer.
type AdminService struct {
	directory  *Service
	authorizer managementcap.Authorizer
}

func NewAdmin(directory *Service, authorizer managementcap.Authorizer) (*AdminService, error) {
	if directory == nil || directory.database == nil || authorizer == nil {
		return nil, bizerr.NewCode(CodeDirectoryAdminUnavailable)
	}
	return &AdminService{directory: directory, authorizer: authorizer}, nil
}
func (s *AdminService) authorize(ctx context.Context, target string) error {
	if s == nil || s.authorizer == nil {
		return bizerr.NewCode(CodeDirectoryAdminUnavailable)
	}
	actor, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{Permission: cloudAdminViewPermission, TargetNo: target})
	if err != nil || actor < 1 {
		return bizerr.NewCode(CodeDirectoryAdminNotAccessible)
	}
	return nil
}

// List never accepts a provider identity from the isolated page.
func (s *AdminService) List(ctx context.Context, page int, limit int, entitlements ...string) (*Page, error) {
	if err := s.authorize(ctx, cloudConnectorPluginID); err != nil {
		return nil, err
	}
	if page < 1 || page > 1000000 || limit < 1 || limit > cloudAdminMaximumPageSize || len(entitlements) > 1 {
		return nil, bizerr.NewCode(CodeDirectoryAdminInvalid)
	}
	var (
		countQuery = cloudAdminDirectoryCountQuery
		listQuery  = cloudAdminDirectoryQuery
		countArgs  = []any{cloudConnectorPluginID}
		listArgs   = []any{cloudConnectorPluginID}
	)
	if len(entitlements) == 1 && entitlements[0] != "" {
		entitlement := entitlements[0]
		if len(entitlement) > 40 || !identityPattern.MatchString(entitlement) {
			return nil, bizerr.NewCode(CodeDirectoryAdminInvalid)
		}
		countQuery += " AND ownership.entitlement_no=?"
		listQuery = strings.Replace(listQuery, "AND ownership.server_no>?", "AND ownership.entitlement_no=? AND ownership.server_no>?", 1)
		countArgs = append(countArgs, entitlement)
		listArgs = append(listArgs, entitlement)
	}
	listArgs = append(listArgs, "", limit+1, (page-1)*limit)
	var total struct {
		Total int `orm:"total"`
	}
	if err := s.directory.database.GetScan(ctx, &total, countQuery, countArgs...); err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryAdminUnavailable)
	}
	var rows []directoryRow
	if err := s.directory.database.GetScan(ctx, &rows, listQuery, listArgs...); err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryAdminUnavailable)
	}
	result := &Page{Total: total.Total, Items: make([]Item, 0, min(len(rows), limit))}
	for i := 0; i < len(rows) && i < limit; i++ {
		item, e := projectRow(rows[i])
		if e != nil {
			return nil, bizerr.WrapCode(e, CodeDirectoryAdminUnavailable)
		}
		result.Items = append(result.Items, item)
	}
	if len(rows) > limit && len(result.Items) > 0 {
		result.NextCursor = encodeCursor(result.Items[len(result.Items)-1].ServerNo)
	}
	return result, nil
}
func (s *AdminService) Detail(ctx context.Context, serverNo string) (*Item, error) {
	if err := s.authorize(ctx, serverNo); err != nil {
		return nil, err
	}
	if !serverNoPattern.MatchString(serverNo) {
		return nil, bizerr.NewCode(CodeDirectoryAdminInvalid)
	}
	var rows []directoryRow
	if err := s.directory.database.GetScan(ctx, &rows, cloudAdminDirectoryDetailQuery, cloudConnectorPluginID, serverNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryAdminUnavailable)
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(CodeDirectoryAdminNotAccessible)
	}
	item, err := projectRow(rows[0])
	if err != nil {
		return nil, bizerr.NewCode(CodeDirectoryAdminNotAccessible)
	}
	return &item, nil
}

const cloudAdminDirectoryQuery = `WITH selected AS (
    SELECT ownership.server_no,ownership.action_reference,ownership.product_name_zh_cn,
           ownership.product_name_en_us,ownership.local_resource_no,
           ownership.bound_at,mapping.channel_no,mapping.pool_no,
           mapping.status_code AS mapping_status,
           GREATEST(ownership.updated_at,mapping.updated_at) AS updated_at,
           channel.display_name AS channel_name,pool.display_name AS pool_name,
           pool.resource_kind
    FROM plugin_idc_resource_server_entitlement_projection ownership
    JOIN plugin_idc_resource_mapping mapping
      ON mapping.tenant_id=ownership.tenant_id
     AND mapping.mapping_no=ownership.mapping_no
     AND mapping.local_resource_no=ownership.local_resource_no
    JOIN plugin_idc_resource_channel channel
      ON channel.tenant_id=mapping.tenant_id AND channel.channel_no=mapping.channel_no
    JOIN plugin_idc_resource_pool pool
      ON pool.tenant_id=mapping.tenant_id AND pool.channel_no=mapping.channel_no
     AND pool.pool_no=mapping.pool_no
    WHERE ownership.tenant_id=0 AND channel.provider_plugin_id=?
      AND ownership.server_no>?
    ORDER BY ownership.server_no
    LIMIT ? OFFSET ?
)` + directoryEnrichmentCTE + `
SELECT server_no,action_reference,product_name_zh_cn,product_name_en_us,
       channel_name,pool_name,resource_kind,mapping_status,task_status,
       capabilities,bound_at,updated_at
FROM projected ORDER BY server_no`

const cloudAdminDirectoryDetailQuery = `WITH selected AS (
    SELECT ownership.server_no,ownership.action_reference,ownership.product_name_zh_cn,
           ownership.product_name_en_us,ownership.local_resource_no,
           ownership.bound_at,mapping.channel_no,mapping.pool_no,
           mapping.status_code AS mapping_status,
           GREATEST(ownership.updated_at,mapping.updated_at) AS updated_at,
           channel.display_name AS channel_name,pool.display_name AS pool_name,
           pool.resource_kind
    FROM plugin_idc_resource_server_entitlement_projection ownership
    JOIN plugin_idc_resource_mapping mapping
      ON mapping.tenant_id=ownership.tenant_id
     AND mapping.mapping_no=ownership.mapping_no
     AND mapping.local_resource_no=ownership.local_resource_no
    JOIN plugin_idc_resource_channel channel
      ON channel.tenant_id=mapping.tenant_id AND channel.channel_no=mapping.channel_no
    JOIN plugin_idc_resource_pool pool
      ON pool.tenant_id=mapping.tenant_id AND pool.channel_no=mapping.channel_no
     AND pool.pool_no=mapping.pool_no
    WHERE ownership.tenant_id=0 AND channel.provider_plugin_id=?
      AND ownership.server_no=?
    LIMIT 2
)` + directoryEnrichmentCTE + `
SELECT server_no,action_reference,product_name_zh_cn,product_name_en_us,
       channel_name,pool_name,resource_kind,mapping_status,task_status,
       capabilities,bound_at,updated_at
FROM projected ORDER BY server_no`

const cloudAdminDirectoryCountQuery = `SELECT count(*) AS total FROM plugin_idc_resource_server_entitlement_projection ownership JOIN plugin_idc_resource_mapping mapping ON mapping.tenant_id=ownership.tenant_id AND mapping.mapping_no=ownership.mapping_no AND mapping.local_resource_no=ownership.local_resource_no JOIN plugin_idc_resource_channel channel ON channel.tenant_id=mapping.tenant_id AND channel.channel_no=mapping.channel_no JOIN plugin_idc_resource_pool pool ON pool.tenant_id=mapping.tenant_id AND pool.channel_no=mapping.channel_no AND pool.pool_no=mapping.pool_no WHERE ownership.tenant_id=0 AND channel.provider_plugin_id=?`
