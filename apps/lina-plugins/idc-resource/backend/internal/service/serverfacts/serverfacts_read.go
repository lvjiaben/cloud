// This file performs one bounded local join and validates projected values again
// before exposing them to a customer. Historical facts never enable an action.
package serverfacts

import (
	"context"
	"encoding/json"
	"net/netip"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const maxFacts = 100
const staleAfter = 5 * time.Minute

var entitlementPattern = regexp.MustCompile(`^ent_[0-9a-f]{32}$`)

// displayFields is the complete persistence whitelist, not a raw API response.
type displayFields struct {
	Hostname           string                    `json:"hostname"`
	OSName             string                    `json:"osName"`
	Addresses          []string                  `json:"addresses"`
	AddressesConfirmed bool                      `json:"addressesConfirmed"`
	State              resourcecap.ResourceState `json:"state"`
}

func (s *serviceImpl) Read(ctx context.Context, customerNo string, numbers []string) ([]Fact, error) {
	if ctx == nil || ctx.Err() != nil || customerNo == "" {
		return nil, gerror.New("native customer facts unavailable")
	}
	if len(numbers) > maxFacts {
		return nil, ErrInvalid
	}
	keys := make([]string, 0, len(numbers))
	seen := make(map[string]bool, len(numbers))
	for _, no := range numbers {
		if !entitlementPattern.MatchString(no) {
			return nil, ErrInvalid
		}
		if !seen[no] {
			keys = append(keys, no)
			seen[no] = true
		}
	}
	result := make([]Fact, 0, len(keys))
	if len(keys) == 0 {
		return result, nil
	}
	var rows []struct {
		EntitlementNo, ServerNo, Facts string
		ObservedAt                     *time.Time
	}
	if err := s.database.GetScan(ctx, &rows, readQuery, customerNo, keys); err != nil {
		return nil, gerror.Wrap(err, "read native server facts")
	}
	now := time.Now()
	for _, row := range rows {
		fact := Fact{EntitlementNo: row.EntitlementNo, ServerNo: row.ServerNo, Addresses: []string{}, LastObservedState: resourcecap.ResourceStateUnknown, Stale: true}
		var fields displayFields
		if row.ObservedAt != nil && !row.ObservedAt.After(now.Add(time.Second)) && json.Unmarshal([]byte(row.Facts), &fields) == nil && validState(fields.State) {
			fact.Hostname = safeLabel(fields.Hostname, 253)
			fact.OSName = safeLabel(fields.OSName, 128)
			fact.LastObservedState = fields.State
			fact.ObservedAt = row.ObservedAt
			fact.Stale = now.Sub(*row.ObservedAt) > staleAfter
			if fields.AddressesConfirmed && validAddresses(fields.Addresses) {
				fact.Addresses = fields.Addresses
				fact.AddressesConfirmed = true
			}
		}
		result = append(result, fact)
	}
	return result, nil
}

func validState(value resourcecap.ResourceState) bool {
	switch value {
	case resourcecap.ResourceStatePending, resourcecap.ResourceStateRunning, resourcecap.ResourceStateActive, resourcecap.ResourceStateStopped, resourcecap.ResourceStateSuspended, resourcecap.ResourceStateFailed, resourcecap.ResourceStateTerminated, resourcecap.ResourceStateUnknown:
		return true
	default:
		return false
	}
}

func validAddresses(values []string) bool {
	if values == nil || len(values) > 1024 {
		return false
	}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		ip, err := netip.ParseAddr(value)
		if err != nil || ip.Zone() != "" || ip.Unmap().String() != value || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func safeLabel(value string, maximum int) string {
	if len(value) > maximum || strings.TrimSpace(value) != value {
		return ""
	}
	for _, r := range value {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return ""
		}
	}
	return value
}

const readQuery = `SELECT ownership.entitlement_no,ownership.server_no,fact.facts,fact.observed_at
FROM plugin_idc_resource_server_entitlement_projection ownership
JOIN plugin_idc_resource_mapping mapping ON mapping.tenant_id=ownership.tenant_id AND mapping.mapping_no=ownership.mapping_no AND mapping.local_resource_no=ownership.local_resource_no
LEFT JOIN plugin_idc_resource_server_runtime_fact fact ON fact.tenant_id=mapping.tenant_id AND fact.mapping_no=mapping.mapping_no AND fact.mapping_version=mapping.version
WHERE ownership.tenant_id=0 AND ownership.customer_no=? AND ownership.entitlement_no IN (?)
AND mapping.status_code='active' AND mapping.external_resource_ref<>'' ORDER BY ownership.entitlement_no`
