package customeroperations

import (
	"context"
	"slices"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type OperationPage struct {
	Items []Operation
	Total int
}

// Operations uses the same owned-server scope for count and page. Output
// availability is fetched only when the customer opens one operation.
func (s *Service) Operations(ctx context.Context, serverNo string, page, limit int, action string) (*OperationPage, error) {
	if page < 1 || page > 1000000 || limit < 1 || limit > 50 || (action != "" && !slices.Contains(nativeActions, serveroperationresourcecap.Action(action))) {
		return nil, ErrInvalid
	}
	owned, err := s.ownedRoute(ctx, serverNo, false)
	if err != nil {
		return nil, err
	}
	if _, err = s.entitlements.ReadCurrentEntitlement(ctx, owned.EntitlementNo); err != nil {
		return nil, ErrUnavailable
	}
	filter := `task.tenant_id=0 AND task.local_resource_no=? AND task.channel_no=? AND task.owner_metadata->>'operationNo' LIKE 'nso\_%' ESCAPE '\'`
	args := []any{owned.LocalResourceNo, owned.ChannelNo}
	if action != "" {
		filter += ` AND task.owner_metadata->>'action'=?`
		args = append(args, action)
	}
	count, err := s.database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_resource_task task WHERE `+filter, args...)
	if err != nil {
		return nil, ErrUnavailable
	}
	result := &OperationPage{Items: []Operation{}, Total: count.Int()}
	args = append(args, limit, (page-1)*limit)
	err = s.database.GetScan(ctx, &result.Items, `SELECT task.owner_metadata->>'operationNo' AS operation_no,task.owner_metadata->>'action' AS action,task.status_code AS status,task.failure_code,task.created_at,task.updated_at,
CASE WHEN task.status_code IN ('pending','retry_wait') AND health.circuit_open AND health.write_protected_until>CURRENT_TIMESTAMP THEN health.write_protected_until END AS waiting_until
FROM plugin_idc_resource_task task LEFT JOIN plugin_idc_resource_channel_health_state health ON health.tenant_id=task.tenant_id AND health.channel_no=task.channel_no
WHERE `+filter+` ORDER BY task.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, ErrUnavailable
	}
	for i := range result.Items {
		result.Items[i].AvailableOutputs = []Output{}
	}
	return result, nil
}
