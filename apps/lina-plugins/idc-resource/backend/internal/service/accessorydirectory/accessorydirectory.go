// Package accessorydirectory owns immutable, safe per-server accessory projections.
package accessorydirectory

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var (
	ErrDirectoryUnavailable = errors.New("resource accessory directory is unavailable")
	ErrObjectUnavailable    = errors.New("resource accessory object is unavailable")
)

type PublishInput struct {
	MappingNo           string
	LocalResourceNo     string
	ChannelNo           string
	ExternalResourceRef string
	Snapshot            resourcecap.AccessorySnapshot
}

type Item struct {
	ObjectNo   string
	Kind       resourcecap.AccessoryKind
	Name       string
	State      resourcecap.AccessoryState
	Projection json.RawMessage
}

type Directory struct {
	SnapshotNo string
	ObservedAt time.Time
	ExpiresAt  time.Time
	Items      []Item
}

type ResolvedObject struct {
	SnapshotNo  string
	ExternalRef string
	ObservedAt  time.Time
	ExpiresAt   time.Time
}

type Service interface {
	Publish(context.Context, PublishInput) (*Directory, error)
	Read(context.Context, string) (*Directory, error)
	Resolve(context.Context, string, string, resourcecap.AccessoryKind) (*ResolvedObject, error)
}

type serviceImpl struct {
	database gdb.DB
	random   io.Reader
}

func New(database gdb.DB) (Service, error) {
	if database == nil {
		return nil, gerror.New("resource accessory directory database is required")
	}
	return &serviceImpl{database: database, random: rand.Reader}, nil
}

func (s *serviceImpl) Publish(ctx context.Context, input PublishInput) (*Directory, error) {
	return s.publish(ctx, input, "")
}

func (s *serviceImpl) publish(ctx context.Context, input PublishInput, refreshLeaseDigest string) (*Directory, error) {
	if ctx == nil || ctx.Err() != nil || !validReference(input.MappingNo, 64) ||
		!validReference(input.LocalResourceNo, 64) || !validReference(input.ChannelNo, 64) ||
		!validReference(input.ExternalResourceRef, 256) ||
		(refreshLeaseDigest != "" && !validDigest(refreshLeaseDigest)) {
		return nil, ErrDirectoryUnavailable
	}
	now := time.Now().UTC()
	if err := input.Snapshot.ValidateAt(now); err != nil {
		return nil, ErrDirectoryUnavailable
	}
	items := append([]resourcecap.AccessoryItem(nil), input.Snapshot.Items...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].ExternalRef < items[j].ExternalRef
	})
	snapshotNo, err := randomReference(s.random, "ACD_")
	if err != nil {
		return nil, ErrDirectoryUnavailable
	}
	fingerprint, normalized, err := accessoryFingerprint(items)
	if err != nil {
		return nil, ErrDirectoryUnavailable
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if refreshLeaseDigest != "" {
			var lease *struct {
				MappingNo string `orm:"mapping_no"`
			}
			if leaseErr := tx.GetScan(&lease, `SELECT mapping_no
FROM plugin_idc_resource_accessory_refresh_state
WHERE tenant_id=? AND mapping_no=? AND lease_token_digest=?
 AND lease_expires_at>CURRENT_TIMESTAMP FOR UPDATE`, idccap.DefaultTenantID,
				input.MappingNo, refreshLeaseDigest); leaseErr != nil || lease == nil {
				return ErrDirectoryUnavailable
			}
		}
		var lockedMapping *struct {
			MappingNo string `orm:"mapping_no"`
		}
		if queryErr := tx.GetScan(&lockedMapping, `SELECT mapping_no
FROM plugin_idc_resource_mapping
WHERE tenant_id=? AND mapping_no=? AND local_resource_no=? AND channel_no=?
 AND external_resource_ref=? AND status_code='active'
FOR UPDATE`, idccap.DefaultTenantID, input.MappingNo, input.LocalResourceNo, input.ChannelNo,
			input.ExternalResourceRef); queryErr != nil ||
			lockedMapping == nil || lockedMapping.MappingNo != input.MappingNo {
			return ErrDirectoryUnavailable
		}
		var previous []struct {
			ObjectNo    string `orm:"object_no"`
			Kind        string `orm:"accessory_kind"`
			ExternalRef string `orm:"external_ref"`
		}
		if queryErr := tx.GetScan(&previous, `SELECT item.object_no,item.accessory_kind,item.external_ref
FROM plugin_idc_resource_accessory_snapshot snapshot
JOIN plugin_idc_resource_accessory_snapshot_item item ON item.snapshot_no=snapshot.snapshot_no
WHERE snapshot.tenant_id=? AND snapshot.local_resource_no=? AND snapshot.status_code='complete'`,
			idccap.DefaultTenantID, input.LocalResourceNo); queryErr != nil {
			return gerror.Wrap(queryErr, "read previous resource accessory identities")
		}
		previousObjectNos := make(map[string]string, len(previous))
		for index := range previous {
			previousObjectNos[previous[index].Kind+"\x00"+previous[index].ExternalRef] = previous[index].ObjectNo
		}
		objectNos := make([]string, len(items))
		for index := range items {
			objectNos[index] = previousObjectNos[string(items[index].Kind)+"\x00"+items[index].ExternalRef]
			if objectNos[index] == "" {
				objectNos[index], err = randomReference(s.random, "ACO_")
				if err != nil {
					return ErrDirectoryUnavailable
				}
			}
		}
		if _, updateErr := tx.Exec(`UPDATE plugin_idc_resource_accessory_snapshot
SET status_code='superseded'
WHERE tenant_id=? AND local_resource_no=? AND status_code='complete'`,
			idccap.DefaultTenantID, input.LocalResourceNo); updateErr != nil {
			return gerror.Wrap(updateErr, "supersede resource accessory directory")
		}
		if _, insertErr := tx.Exec(`INSERT INTO plugin_idc_resource_accessory_snapshot
 (snapshot_no,mapping_no,local_resource_no,channel_no,status_code,snapshot_fingerprint,item_count,observed_at,expires_at)
VALUES (?,?,?,?, 'complete',?,?,?,?)`, snapshotNo, input.MappingNo, input.LocalResourceNo,
			input.ChannelNo, fingerprint, len(items), input.Snapshot.ObservedAt.UTC(), input.Snapshot.ExpiresAt.UTC()); insertErr != nil {
			return gerror.Wrap(insertErr, "insert resource accessory directory")
		}
		for index := range items {
			item := items[index]
			if _, insertErr := tx.Exec(`INSERT INTO plugin_idc_resource_accessory_snapshot_item
 (snapshot_no,object_no,accessory_kind,external_ref,display_name,state_code,projection,observed_at)
VALUES (?,?,?,?,?,?,CAST(? AS JSONB),?)`, snapshotNo, objectNos[index], item.Kind, item.ExternalRef,
				item.DisplayName, item.State, normalized[index], input.Snapshot.ObservedAt.UTC()); insertErr != nil {
				return gerror.Wrap(insertErr, "insert resource accessory directory item")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Read(ctx, input.LocalResourceNo)
}

func (s *serviceImpl) Read(ctx context.Context, localResourceNo string) (*Directory, error) {
	if ctx == nil || ctx.Err() != nil || !validReference(localResourceNo, 64) {
		return nil, ErrDirectoryUnavailable
	}
	var header *struct {
		SnapshotNo string    `orm:"snapshot_no"`
		ObservedAt time.Time `orm:"observed_at"`
		ExpiresAt  time.Time `orm:"expires_at"`
	}
	if err := s.database.GetScan(ctx, &header, `SELECT snapshot.snapshot_no,snapshot.observed_at,snapshot.expires_at
FROM plugin_idc_resource_accessory_snapshot snapshot
JOIN plugin_idc_resource_mapping mapping ON mapping.mapping_no=snapshot.mapping_no
WHERE snapshot.tenant_id=? AND snapshot.local_resource_no=? AND snapshot.status_code='complete'
 AND snapshot.expires_at>CURRENT_TIMESTAMP AND mapping.status_code='active'
ORDER BY snapshot.observed_at DESC,snapshot.id DESC LIMIT 1`, idccap.DefaultTenantID, localResourceNo); err != nil {
		return nil, gerror.Wrap(err, "read resource accessory directory")
	}
	if header == nil {
		return nil, ErrDirectoryUnavailable
	}
	var rows []struct {
		ObjectNo    string          `orm:"object_no"`
		Kind        string          `orm:"accessory_kind"`
		Name        string          `orm:"display_name"`
		State       string          `orm:"state_code"`
		Projection  json.RawMessage `orm:"projection"`
		ExternalRef string          `orm:"external_ref"`
	}
	if err := s.database.GetScan(ctx, &rows, `SELECT object_no,accessory_kind,display_name,state_code,projection,external_ref
FROM plugin_idc_resource_accessory_snapshot_item
WHERE tenant_id=? AND snapshot_no=?
ORDER BY accessory_kind,display_name,id`, idccap.DefaultTenantID, header.SnapshotNo); err != nil {
		return nil, gerror.Wrap(err, "read resource accessory directory items")
	}
	providerItems := make([]resourcecap.AccessoryItem, len(rows))
	result := &Directory{SnapshotNo: header.SnapshotNo, ObservedAt: header.ObservedAt.UTC(), ExpiresAt: header.ExpiresAt.UTC(), Items: make([]Item, len(rows))}
	for index := range rows {
		providerItems[index] = resourcecap.AccessoryItem{Kind: resourcecap.AccessoryKind(rows[index].Kind), ExternalRef: rows[index].ExternalRef,
			DisplayName: rows[index].Name, State: resourcecap.AccessoryState(rows[index].State), Data: rows[index].Projection}
		result.Items[index] = Item{ObjectNo: rows[index].ObjectNo, Kind: providerItems[index].Kind,
			Name: rows[index].Name, State: providerItems[index].State, Projection: append(json.RawMessage(nil), rows[index].Projection...)}
	}
	if err := (resourcecap.AccessorySnapshot{Items: providerItems, ObservedAt: result.ObservedAt, ExpiresAt: result.ExpiresAt}).ValidateAt(time.Now().UTC()); err != nil {
		return nil, ErrDirectoryUnavailable
	}
	return result, nil
}

func (s *serviceImpl) Resolve(ctx context.Context, localResourceNo, objectNo string,
	kind resourcecap.AccessoryKind) (*ResolvedObject, error) {
	if ctx == nil || ctx.Err() != nil || !validReference(localResourceNo, 64) || !validReference(objectNo, 64) {
		return nil, ErrObjectUnavailable
	}
	var row *ResolvedObject
	if err := s.database.GetScan(ctx, &row, `SELECT snapshot.snapshot_no,item.external_ref,snapshot.observed_at,snapshot.expires_at
FROM plugin_idc_resource_accessory_snapshot snapshot
JOIN plugin_idc_resource_accessory_snapshot_item item ON item.snapshot_no=snapshot.snapshot_no
JOIN plugin_idc_resource_mapping mapping ON mapping.mapping_no=snapshot.mapping_no
WHERE snapshot.tenant_id=? AND snapshot.local_resource_no=? AND snapshot.status_code='complete'
 AND snapshot.expires_at>CURRENT_TIMESTAMP AND item.object_no=? AND item.accessory_kind=?
 AND mapping.status_code='active'`, idccap.DefaultTenantID, localResourceNo, objectNo, kind); err != nil {
		return nil, gerror.Wrap(err, "resolve resource accessory object")
	}
	if row == nil || !validReference(row.ExternalRef, 256) {
		return nil, ErrObjectUnavailable
	}
	return row, nil
}

func accessoryFingerprint(items []resourcecap.AccessoryItem) (string, []string, error) {
	hash := sha256.New()
	normalized := make([]string, len(items))
	for index := range items {
		var compact bytes.Buffer
		if err := json.Compact(&compact, items[index].Data); err != nil {
			return "", nil, err
		}
		normalized[index] = compact.String()
		for _, value := range []string{string(items[index].Kind), items[index].ExternalRef,
			items[index].DisplayName, string(items[index].State), normalized[index]} {
			hash.Write([]byte{byte(len(value) >> 24), byte(len(value) >> 16), byte(len(value) >> 8), byte(len(value))})
			hash.Write([]byte(value))
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), normalized, nil
}

func randomReference(source io.Reader, prefix string) (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(source, value); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(value), nil
}

func validReference(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && value == strings.TrimSpace(value) && !strings.ContainsAny(value, "\x00\r\n\t")
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for index := range value {
		if (value[index] < '0' || value[index] > '9') && (value[index] < 'a' || value[index] > 'f') {
			return false
		}
	}
	return true
}
