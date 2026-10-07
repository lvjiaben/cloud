// This file defines the optional field-whitelisted Provider accessory directory.

package resourcecap

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const MaximumAccessoryItemCount = 10_000

type AccessoryKind string

const (
	AccessoryKindIP            AccessoryKind = "ip"
	AccessoryKindDisk          AccessoryKind = "disk"
	AccessoryKindSnapshot      AccessoryKind = "snapshot"
	AccessoryKindBackup        AccessoryKind = "backup"
	AccessoryKindSecurityGroup AccessoryKind = "security_group"
	AccessoryKindNAT           AccessoryKind = "nat"
	AccessoryKindVirtualMedia  AccessoryKind = "virtual_media"
	AccessoryKindTraffic       AccessoryKind = "traffic"
)

type AccessoryState string

const (
	AccessoryStateAvailable AccessoryState = "available"
	AccessoryStateAttached  AccessoryState = "attached"
	AccessoryStateMounted   AccessoryState = "mounted"
	AccessoryStateActive    AccessoryState = "active"
	AccessoryStateInactive  AccessoryState = "inactive"
	AccessoryStateUnknown   AccessoryState = "unknown"
)

type AccessoryQuery struct {
	ResourceRef string
}

type AccessoryItem struct {
	Kind        AccessoryKind
	ExternalRef string
	DisplayName string
	State       AccessoryState
	Data        json.RawMessage
}

type AccessorySnapshot struct {
	Items      []AccessoryItem
	ObservedAt time.Time
	ExpiresAt  time.Time
}

// AccessoryProvider is optional because not every upstream adapter can expose
// a coherent per-server accessory directory. It is read-only; mutations still
// use Provider.Execute and the normal resource task state machine.
type AccessoryProvider interface {
	QueryAccessories(ctx context.Context, request AccessoryQuery) (*AccessorySnapshot, error)
}

func (q AccessoryQuery) Validate() error {
	return validateRequiredReference("accessory resource reference", q.ResourceRef)
}

func (s AccessorySnapshot) ValidateAt(now time.Time) error {
	if now.IsZero() || s.ObservedAt.IsZero() || s.ExpiresAt.IsZero() ||
		s.ObservedAt.After(now.Add(time.Minute)) || !s.ExpiresAt.After(s.ObservedAt) ||
		!s.ExpiresAt.After(now) || len(s.Items) > MaximumAccessoryItemCount {
		return gerror.New("resource Provider accessory snapshot time or size is invalid")
	}
	seen := make(map[string]struct{}, len(s.Items))
	for index := range s.Items {
		item := s.Items[index]
		if !validAccessoryKind(item.Kind) || !validAccessoryState(item.State) ||
			validateRequiredReference("accessory external reference", item.ExternalRef) != nil ||
			validateOptionalText("accessory display name", item.DisplayName, maximumNameBytes) != nil ||
			validateJSONObject("accessory projection", item.Data, true) != nil {
			return gerror.New("resource Provider accessory item is invalid")
		}
		key := string(item.Kind) + "\x00" + item.ExternalRef
		if _, exists := seen[key]; exists {
			return gerror.New("resource Provider accessory item is duplicated")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validAccessoryKind(value AccessoryKind) bool {
	switch value {
	case AccessoryKindIP, AccessoryKindDisk, AccessoryKindSnapshot, AccessoryKindBackup,
		AccessoryKindSecurityGroup, AccessoryKindNAT, AccessoryKindVirtualMedia, AccessoryKindTraffic:
		return true
	default:
		return false
	}
}

func validAccessoryState(value AccessoryState) bool {
	switch value {
	case AccessoryStateAvailable, AccessoryStateAttached, AccessoryStateMounted,
		AccessoryStateActive, AccessoryStateInactive, AccessoryStateUnknown:
		return true
	default:
		return false
	}
}
