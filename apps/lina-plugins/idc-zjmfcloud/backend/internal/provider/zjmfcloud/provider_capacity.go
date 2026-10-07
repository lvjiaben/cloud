// This file implements bounded, paginated, field-whitelisted Provider capacity discovery.

package zjmfcloud

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type providerCapacityProjection struct {
	route  string
	fields []string
	idKeys []string
}

var providerCapacityProjections = map[resourcecap.ProviderCapacityKind]providerCapacityProjection{
	resourcecap.ProviderCapacityKindRegion: {
		route:  "/areas",
		idKeys: []string{"id"},
		fields: []string{
			"id", "name", "country", "status", "total", "used", "free", "locked",
			"ip_total", "ip_used", "ip_free", "ip_lock",
		},
	},
	resourcecap.ProviderCapacityKindLine: {
		route:  "/nodes",
		idKeys: []string{"id"},
		fields: []string{
			"id", "name", "area", "area_id", "node_group", "node_group_id", "type", "status",
			"cpu_total", "cpu_used", "memory_total", "memory_used", "disk_total", "disk_used",
		},
	},
	resourcecap.ProviderCapacityKindImage: {
		route:  "/image",
		idKeys: []string{"id"},
		fields: []string{
			"id", "name", "group", "group_id", "status", "type", "boot_mode", "os_type", "arch",
		},
	},
	resourcecap.ProviderCapacityKindStock: {
		route:  "/ip_groups",
		idKeys: []string{"id"},
		fields: []string{
			"id", "name", "area_id", "node_id", "node_group_id", "status", "total", "used", "free", "locked",
			"ip_total", "ip_used", "ip_free", "ip_lock",
		},
	},
}

// ListProviderCapacity returns one bounded, field-whitelisted upstream page.
func (p *Provider) ListProviderCapacity(
	ctx context.Context,
	request resourcecap.ProviderCapacityRequest,
) (*resourcecap.ProviderCapacityPage, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	filters := bytes.TrimSpace(request.Filters)
	if len(filters) != 0 && !bytes.Equal(filters, []byte(`{}`)) {
		return nil, errUnsupported
	}
	if request.Kind == resourcecap.ProviderCapacityKindPlan {
		return p.listProvisionCandidates(ctx, request)
	}
	projection, supported := providerCapacityProjections[request.Kind]
	if !supported {
		return nil, errUnsupported
	}
	pageNumber, err := decodeCursor(request.Cursor, request.Kind, request.Limit)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("page", strconv.Itoa(pageNumber))
	query.Set("per_page", strconv.Itoa(request.Limit))

	response, err := p.authenticatedRequest(ctx, http.MethodGet, projection.route, query, nil)
	if err != nil {
		return nil, normalizeReadCallError(err)
	}
	defer clear(response.body)
	if response.statusCode < 200 || response.statusCode >= 300 {
		return nil, readStatusError(response.statusCode)
	}
	rawItems, meta, err := decodeObjectPage(response.body)
	if err != nil || len(rawItems) > request.Limit {
		clearRawObjects(rawItems)
		clearRawObject(meta)
		return nil, errInvalidResponse
	}
	defer clearRawObjects(rawItems)
	defer clearRawObject(meta)

	observedAt := p.now()
	items := make([]resourcecap.ProviderCapacityItem, 0, len(rawItems))
	for _, rawItem := range rawItems {
		reference, ok := firstReference(rawItem, projection.idKeys...)
		if !ok {
			return nil, errInvalidResponse
		}
		data, projectionErr := projectScalars(rawItem, projection.fields)
		if projectionErr != nil {
			return nil, projectionErr
		}
		items = append(items, resourcecap.ProviderCapacityItem{
			Kind:        request.Kind,
			ExternalRef: reference,
			Data:        data,
			ObservedAt:  observedAt,
		})
	}

	complete, nextCursor, err := pageBoundary(request.Kind, pageNumber, request.Limit, len(items), meta)
	if err != nil {
		return nil, err
	}
	page := &resourcecap.ProviderCapacityPage{
		Items:      items,
		NextCursor: nextCursor,
		Complete:   complete,
		Evidence: []resourcecap.Evidence{{
			Kind:       "upstream_page",
			Reference:  string(request.Kind) + ":" + strconv.Itoa(pageNumber),
			Summary:    "field-whitelisted upstream capacity page observed",
			ObservedAt: observedAt,
		}},
	}
	if err := page.Validate(request); err != nil {
		return nil, errInvalidResponse
	}
	return page, nil
}

func decodeCursor(cursor string, kind resourcecap.ProviderCapacityKind, limit int) (int, error) {
	if cursor == "" {
		return 1, nil
	}
	parts := strings.Split(cursor, ":")
	if len(parts) != 4 || parts[0] != "v1" || parts[1] != string(kind) {
		return 0, errInvalidConfig
	}
	cursorLimit, err := strconv.Atoi(parts[2])
	if err != nil || cursorLimit != limit {
		return 0, errInvalidConfig
	}
	page, err := strconv.Atoi(parts[3])
	if err != nil || page < 2 || page > maximumCursorPage {
		return 0, errInvalidConfig
	}
	return page, nil
}

func pageBoundary(
	kind resourcecap.ProviderCapacityKind,
	page int,
	limit int,
	itemCount int,
	meta rawObject,
) (bool, string, error) {
	metaPage, hasPage := metaInteger(meta, "page")
	if hasPage && metaPage != page {
		return false, "", errInvalidResponse
	}
	totalPages, hasTotalPages := metaInteger(meta, "total_page")
	if hasTotalPages {
		if totalPages < page && !(totalPages == 0 && page == 1 && itemCount == 0) {
			return false, "", errInvalidResponse
		}
		if page >= totalPages {
			return true, "", nil
		}
		if page >= maximumCursorPage {
			return false, "", errInvalidResponse
		}
		return false, encodeCursor(kind, limit, page+1), nil
	}
	if itemCount < limit {
		return true, "", nil
	}
	if page >= maximumCursorPage {
		return false, "", errInvalidResponse
	}
	return false, encodeCursor(kind, limit, page+1), nil
}

func encodeCursor(kind resourcecap.ProviderCapacityKind, limit int, page int) string {
	return "v1:" + string(kind) + ":" + strconv.Itoa(limit) + ":" + strconv.Itoa(page)
}
