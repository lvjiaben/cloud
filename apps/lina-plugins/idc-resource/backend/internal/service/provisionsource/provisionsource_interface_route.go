// This file resolves a generic product interface into one resource-owned
// supplier channel and a compatible internal pool without exposing pools to
// the product core.

package provisionsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const maximumInterfacePoolCandidates = 64

var configurationRevisionPattern = regexp.MustCompile(`^rci_[0-9a-f]{32}$`)

type interfacePoolCandidate struct {
	PoolNo    string `orm:"pool_no"`
	ChannelNo string `orm:"channel_no"`
}

// resolveInterfacePools maps one product-facing interface number to its
// resource-owned channel and returns only that channel's bounded, enabled
// pools. Generic product code never receives or chooses these pool numbers.
func (s *Service) resolveInterfacePools(
	ctx context.Context,
	interfaceInstanceNo string,
	configurationRevision string,
) ([]interfacePoolCandidate, error) {
	var rows []interfacePoolCandidate
	if err := s.database.GetScan(ctx, &rows, `
SELECT pool.pool_no, binding.channel_no
FROM plugin_idc_resource_interface_channel_binding binding
 INNER JOIN plugin_idc_resource_channel channel
  ON channel.tenant_id = binding.tenant_id AND channel.channel_no = binding.channel_no
 AND channel.status_code = 'enabled' AND channel.deleted_at IS NULL
 AND channel.provider_runtime_type = 'dynamic'
INNER JOIN plugin_idc_resource_interface_configuration_revision revision
  ON revision.tenant_id=binding.tenant_id
 AND revision.interface_instance_no=binding.interface_instance_no
 AND revision.revision_no=?
INNER JOIN plugin_idc_resource_pool pool
  ON pool.tenant_id = binding.tenant_id AND pool.channel_no = binding.channel_no
 AND pool.status_code = 'enabled' AND pool.deleted_at IS NULL
WHERE binding.tenant_id = ? AND binding.interface_instance_no = ?
ORDER BY pool.routing_priority ASC, pool.routing_weight DESC, pool.pool_no ASC
LIMIT ?`, configurationRevision, idccap.DefaultTenantID, interfaceInstanceNo, maximumInterfacePoolCandidates+1); err != nil {
		return nil, gerror.Wrap(err, "resolve server interface resource pools")
	}
	if len(rows) == 0 || len(rows) > maximumInterfacePoolCandidates {
		return nil, provisionresourcecap.ErrCapabilityUnavailable
	}
	return rows, nil
}

func (s *Service) prepareProvisionParameters(
	ctx context.Context,
	request provisionresourcecap.TaskRequest,
) (*routeEvidence, json.RawMessage, string, error) {
	policy, err := parseServerGenerationPolicy(request.ProductConfiguration)
	if err != nil {
		return nil, nil, "", err
	}
	components, err := parseFrozenServerConfiguration(
		request.ProductConfiguration, request.CustomerConfiguration,
	)
	if err != nil {
		return nil, nil, "", err
	}
	pools, err := s.resolveInterfacePools(ctx, request.InterfaceInstanceNo, request.InterfaceConfigurationRevision)
	if err != nil {
		return nil, nil, "", err
	}
	ready, err := s.principal(ctx, request.CustomerNo, pools[0].PoolNo)
	if err != nil {
		return nil, nil, "", err
	}
	if !ready {
		return nil, nil, "", provisionresourcecap.ErrSourceUnavailable
	}
	localResourceNo, _ := frozenServerIdentity(request)
	hostname, err := generatedServerHostname(request, policy)
	if err != nil {
		return nil, nil, "", err
	}
	for _, pool := range pools {
		route, routeErr := s.resolveRoute(ctx, pool.PoolNo, request.CustomerNo)
		if routeErr != nil || route.ChannelNo != pool.ChannelNo {
			continue
		}
		plan := resourcecap.ServerProvisionPlan{ExternalPrincipalRef: route.ExternalPrincipalRef,
			Hostname: hostname, Selector: json.RawMessage(route.Selector),
			ProductConfiguration:  append(json.RawMessage(nil), request.ProductConfiguration...),
			CustomerConfiguration: append(json.RawMessage(nil), request.CustomerConfiguration...),
			Components:            components}
		if route.ProviderRuntimeType != "dynamic" || s.managedParameters == nil {
			continue
		}
		parameters, parameterErr := s.managedParameters.Build(ctx, route.ChannelNo, request.InterfaceConfigurationRevision, plan)
		if parameterErr != nil || validateCapabilityParameters(route.Constraints, parameters) != nil {
			continue
		}
		return route, parameters, localResourceNo, nil
	}
	return nil, nil, "", provisionresourcecap.ErrParametersUnavailable
}

func frozenServerIdentity(request provisionresourcecap.TaskRequest) (string, string) {
	digest := sha256.Sum256([]byte(
		"idc-resource/server/v1\x00" + request.Fulfillment.IntentNo + "\x00" +
			request.Fulfillment.OrderItemNo + "\x00" +
			strconv.Itoa(request.Fulfillment.UnitSequence),
	))
	encoded := hex.EncodeToString(digest[:])
	return "SRV_" + encoded[:32], "srv-" + encoded[:16] + ".internal"
}
