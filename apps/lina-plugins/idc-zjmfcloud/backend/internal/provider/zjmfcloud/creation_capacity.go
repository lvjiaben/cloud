// This file merges bounded cloud creation sources into stable plan candidates.

package zjmfcloud

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const maximumCreationCandidateCount = 20_000

type creationCandidate struct {
	reference string
	data      json.RawMessage
}

func (p *Provider) listProvisionCandidates(
	ctx context.Context,
	request resourcecap.ProviderCapacityRequest,
) (*resourcecap.ProviderCapacityPage, error) {
	pageNumber, expectedFingerprint, err := decodeSnapshotCursor(
		request.Cursor,
		request.Kind,
		request.Limit,
	)
	if err != nil {
		return nil, err
	}
	candidates, err := p.loadCreationCandidates(ctx)
	if err != nil {
		clearCreationCandidates(candidates)
		return nil, err
	}
	defer clearCreationCandidates(candidates)
	sort.Slice(candidates, func(left int, right int) bool {
		return candidates[left].reference < candidates[right].reference
	})
	fingerprint, err := creationCandidateFingerprint(candidates)
	if err != nil {
		return nil, err
	}
	if expectedFingerprint != "" && expectedFingerprint != fingerprint {
		return nil, errInvalidResponse
	}

	start := (pageNumber - 1) * request.Limit
	if start > len(candidates) || (start == len(candidates) && pageNumber > 1) {
		return nil, errInvalidResponse
	}
	end := start + request.Limit
	if end > len(candidates) {
		end = len(candidates)
	}
	observedAt := p.now()
	items := make([]resourcecap.ProviderCapacityItem, 0, end-start)
	for _, candidate := range candidates[start:end] {
		items = append(items, resourcecap.ProviderCapacityItem{
			Kind:        resourcecap.ProviderCapacityKindPlan,
			ExternalRef: candidate.reference,
			Data:        append(json.RawMessage(nil), candidate.data...),
			ObservedAt:  observedAt,
		})
	}
	complete := end == len(candidates)
	nextCursor := ""
	if !complete {
		if pageNumber >= maximumCursorPage {
			return nil, errInvalidResponse
		}
		nextCursor = encodeSnapshotCursor(
			request.Kind,
			request.Limit,
			pageNumber+1,
			fingerprint,
		)
	}
	page := &resourcecap.ProviderCapacityPage{
		Items:      items,
		NextCursor: nextCursor,
		Complete:   complete,
		Evidence: []resourcecap.Evidence{{
			Kind:       "upstream_creation_capacity",
			Reference:  string(request.Kind) + ":" + strconv.Itoa(pageNumber),
			Summary:    "field-whitelisted storage candidates projected from bounded creation sources",
			ObservedAt: observedAt,
		}},
	}
	if err := page.Validate(request); err != nil {
		return nil, errInvalidResponse
	}
	return page, nil
}

// loadCreationCandidates reads each frozen creation-capacity source exactly
// once and merges their field-whitelisted projections under one global bound.
func (p *Provider) loadCreationCandidates(ctx context.Context) ([]creationCandidate, error) {
	graphResponse, err := p.authenticatedRequest(
		ctx,
		http.MethodGet,
		"/clouds/create_page",
		nil,
		nil,
	)
	if err != nil {
		return nil, normalizeReadCallError(err)
	}
	defer clear(graphResponse.body)
	if graphResponse.statusCode < 200 || graphResponse.statusCode >= 300 {
		return nil, readStatusError(graphResponse.statusCode)
	}
	graphRoot, err := decodeObject(graphResponse.body)
	if err != nil {
		return nil, err
	}
	defer clearRawObject(graphRoot)
	candidates, err := creationCandidates(graphRoot)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("enable", "1")
	query.Set("list_type", "all")
	storesResponse, err := p.authenticatedRequest(
		ctx,
		http.MethodGet,
		"/stores",
		query,
		nil,
	)
	if err != nil {
		clearCreationCandidates(candidates)
		return nil, normalizeReadCallError(err)
	}
	defer clear(storesResponse.body)
	if storesResponse.statusCode < 200 || storesResponse.statusCode >= 300 {
		clearCreationCandidates(candidates)
		return nil, readStatusError(storesResponse.statusCode)
	}
	storesRoot, err := decodeObject(storesResponse.body)
	if err != nil {
		clearCreationCandidates(candidates)
		return nil, err
	}
	defer clearRawObject(storesRoot)
	if err = appendGlobalStorageCandidates(&candidates, storesRoot); err != nil {
		clearCreationCandidates(candidates)
		return nil, err
	}
	return candidates, nil
}

func creationCandidates(root rawObject) ([]creationCandidate, error) {
	rawAreas, err := decodeRawObjectArray(root["data"])
	if err != nil {
		return nil, err
	}
	defer clearRawObjects(rawAreas)
	nodeType, _ := firstScalar(root, "node_type")
	candidates := make([]creationCandidate, 0)
	seen := make(map[string]struct{})
	for _, area := range rawAreas {
		areaReference, ok := firstReference(area, "id")
		if !ok {
			clearCreationCandidates(candidates)
			return nil, errInvalidResponse
		}
		areaName, _ := firstScalar(area, "name")
		areaStores, err := optionalRawObjectArray(area["store"])
		if err != nil {
			clearCreationCandidates(candidates)
			return nil, err
		}
		for _, store := range areaStores {
			candidate, candidateErr := creationCandidateFromStore(
				nodeType,
				areaReference,
				areaName,
				"",
				nil,
				nil,
				store,
			)
			if candidateErr != nil || !appendUniqueCreationCandidate(&candidates, seen, candidate) {
				clearRawObjects(areaStores)
				clearCreationCandidates(candidates)
				return nil, errInvalidResponse
			}
		}
		clearRawObjects(areaStores)

		rawNodes, err := optionalRawObjectArray(area["node"])
		if err != nil {
			clearCreationCandidates(candidates)
			return nil, err
		}
		for _, node := range rawNodes {
			nodeReference, ok := firstReference(node, "id")
			nodeStatus, hasStatus := firstScalar(node, "status")
			if !ok || !hasStatus {
				clearRawObjects(rawNodes)
				clearCreationCandidates(candidates)
				return nil, errInvalidResponse
			}
			nodeName, _ := firstScalar(node, "name")
			nodeStores, storesErr := optionalRawObjectArray(node["store"])
			if storesErr != nil {
				clearRawObjects(rawNodes)
				clearCreationCandidates(candidates)
				return nil, storesErr
			}
			for _, store := range nodeStores {
				candidate, candidateErr := creationCandidateFromStore(
					nodeType,
					areaReference,
					areaName,
					nodeReference,
					nodeName,
					nodeStatus,
					store,
				)
				if candidateErr != nil || !appendUniqueCreationCandidate(&candidates, seen, candidate) {
					clearRawObjects(nodeStores)
					clearRawObjects(rawNodes)
					clearCreationCandidates(candidates)
					return nil, errInvalidResponse
				}
			}
			clearRawObjects(nodeStores)
		}
		clearRawObjects(rawNodes)
		if len(candidates) > maximumCreationCandidateCount {
			clearCreationCandidates(candidates)
			return nil, errInvalidResponse
		}
	}
	return candidates, nil
}

func creationCandidateFromStore(
	nodeType any,
	areaReference string,
	areaName any,
	nodeReference string,
	nodeName any,
	nodeStatus any,
	store rawObject,
) (creationCandidate, error) {
	storeReference, ok := firstReference(store, "id")
	if !ok {
		return creationCandidate{}, errInvalidResponse
	}
	storeStatus, hasStatus := firstScalar(store, "status")
	storeFree, hasFree := firstScalar(store, "free")
	if !hasStatus || !hasFree {
		return creationCandidate{}, errInvalidResponse
	}
	projection := map[string]any{
		"area_id":        areaReference,
		"candidate_type": "storage",
		"source_scope":   "area",
		"store_free":     storeFree,
		"store_id":       storeReference,
		"store_status":   storeStatus,
	}
	if areaName != nil {
		projection["area_name"] = areaName
	}
	if nodeType != nil {
		projection["node_type"] = nodeType
	}
	if nodeReference != "" {
		projection["node_id"] = nodeReference
		projection["node_status"] = nodeStatus
		projection["source_scope"] = "node"
		if nodeName != nil {
			projection["node_name"] = nodeName
		}
	}
	if storeName, found := firstScalar(store, "show_name", "name"); found {
		projection["store_name"] = storeName
	}
	if storeType, found := firstScalar(store, "type"); found {
		projection["store_type"] = storeType
	}
	data, err := json.Marshal(projection)
	if err != nil {
		return creationCandidate{}, errInvalidResponse
	}
	reference := "area:" + areaReference + ":"
	if nodeReference != "" {
		reference += "node:" + nodeReference + ":"
	}
	reference += "store:" + storeReference
	return creationCandidate{reference: reference, data: data}, nil
}

// appendGlobalStorageCandidates projects the one bounded top-level storage
// array without falling back to per-node fan-out requests.
func appendGlobalStorageCandidates(
	candidates *[]creationCandidate,
	root rawObject,
) error {
	stores, err := decodeRawObjectArray(root["data"])
	if err != nil {
		return err
	}
	defer clearRawObjects(stores)
	seen := make(map[string]struct{}, len(*candidates)+len(stores))
	for _, candidate := range *candidates {
		seen[candidate.reference] = struct{}{}
	}
	for _, store := range stores {
		candidate, candidateErr := creationCandidateFromGlobalStore(store)
		if candidateErr != nil || !appendUniqueCreationCandidate(candidates, seen, candidate) {
			return errInvalidResponse
		}
	}
	return nil
}

func creationCandidateFromGlobalStore(store rawObject) (creationCandidate, error) {
	storeReference, ok := firstReference(store, "id")
	storeStatus, hasStatus := firstScalar(store, "status")
	if !ok || !hasStatus {
		return creationCandidate{}, errInvalidResponse
	}
	projection := map[string]any{
		"candidate_type": "storage",
		"source_scope":   "global",
		"store_id":       storeReference,
		"store_status":   storeStatus,
	}
	if storeName, found := firstScalar(store, "name", "show_name"); found {
		projection["store_name"] = storeName
	}
	if storeType, found := firstScalar(store, "type"); found {
		projection["store_type"] = storeType
	}
	if storeFree, found := firstScalar(store, "free"); found {
		projection["store_free"] = storeFree
	}
	data, err := json.Marshal(projection)
	if err != nil {
		return creationCandidate{}, errInvalidResponse
	}
	return creationCandidate{
		reference: "global:store:" + storeReference,
		data:      data,
	}, nil
}

func firstScalar(object rawObject, keys ...string) (any, bool) {
	for _, key := range keys {
		if raw, exists := object[key]; exists {
			if value, ok := decodeScalar(raw); ok {
				return value, true
			}
		}
	}
	return nil, false
}

func optionalRawObjectArray(raw json.RawMessage) ([]rawObject, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	return decodeRawObjectArray(raw)
}

func decodeRawObjectArray(raw json.RawMessage) ([]rawObject, error) {
	if len(raw) == 0 || raw[0] != '[' {
		return nil, errInvalidResponse
	}
	var objects []rawObject
	if json.Unmarshal(raw, &objects) != nil || len(objects) > maximumCreationCandidateCount {
		clearRawObjects(objects)
		return nil, errInvalidResponse
	}
	return objects, nil
}

func appendUniqueCreationCandidate(
	candidates *[]creationCandidate,
	seen map[string]struct{},
	candidate creationCandidate,
) bool {
	if len(*candidates) >= maximumCreationCandidateCount {
		clear(candidate.data)
		return false
	}
	if _, exists := seen[candidate.reference]; exists {
		clear(candidate.data)
		return false
	}
	seen[candidate.reference] = struct{}{}
	*candidates = append(*candidates, candidate)
	return true
}

func clearCreationCandidates(candidates []creationCandidate) {
	for index := range candidates {
		clear(candidates[index].data)
		candidates[index].reference = ""
	}
}

func creationCandidateFingerprint(candidates []creationCandidate) (string, error) {
	hash := sha256.New()
	for _, candidate := range candidates {
		for _, part := range [][]byte{
			[]byte(candidate.reference),
			{0},
			candidate.data,
			{0},
		} {
			if _, err := hash.Write(part); err != nil {
				return "", errInvalidResponse
			}
		}
	}
	return hex.EncodeToString(hash.Sum(nil)[:16]), nil
}

func decodeSnapshotCursor(
	cursor string,
	kind resourcecap.ProviderCapacityKind,
	limit int,
) (int, string, error) {
	if cursor == "" {
		return 1, "", nil
	}
	parts := strings.Split(cursor, ":")
	if len(parts) != 5 || parts[0] != "v1" || parts[1] != string(kind) {
		return 0, "", errInvalidConfig
	}
	cursorLimit, err := strconv.Atoi(parts[2])
	if err != nil || cursorLimit != limit {
		return 0, "", errInvalidConfig
	}
	page, err := strconv.Atoi(parts[3])
	if err != nil || page < 2 || page > maximumCursorPage {
		return 0, "", errInvalidConfig
	}
	if len(parts[4]) != 32 {
		return 0, "", errInvalidConfig
	}
	if _, err := hex.DecodeString(parts[4]); err != nil {
		return 0, "", errInvalidConfig
	}
	return page, parts[4], nil
}

func encodeSnapshotCursor(
	kind resourcecap.ProviderCapacityKind,
	limit int,
	page int,
	fingerprint string,
) string {
	return "v1:" + string(kind) + ":" + strconv.Itoa(limit) + ":" +
		strconv.Itoa(page) + ":" + fingerprint
}
