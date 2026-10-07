// This file implements bounded page staging and atomic terminal publication.

package providercapacity

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

var observationNumberPattern = regexp.MustCompile(`^capacity_observation_[0-9a-f]{32}$`)

// Begin serializes per-channel observation creation and persists the exact first request shape.
func (s *serviceImpl) Begin(ctx context.Context, input BeginInput) (Observation, error) {
	return s.begin(ctx, input, beginOptions{TriggerCode: TriggerSystem})
}

func (s *serviceImpl) begin(
	ctx context.Context,
	input BeginInput,
	options beginOptions,
) (Observation, error) {
	if s == nil || s.database == nil || s.random == nil || ctx == nil || ctx.Err() != nil {
		return Observation{}, gerror.New("resource providerCapacity observation service is unavailable")
	}
	if err := validateBeginInput(input); err != nil {
		return Observation{}, err
	}
	if (options.TriggerCode != TriggerSystem && options.TriggerCode != TriggerManual) ||
		(options.TriggerCode == TriggerSystem && options.RequestedBy != 0) ||
		(options.TriggerCode == TriggerManual && options.RequestedBy <= 0) {
		return Observation{}, gerror.New("resource providerCapacity observation trigger is invalid")
	}
	observationNo, err := newObservationNumber(s.random)
	if err != nil {
		return Observation{}, err
	}
	created := Observation{}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if err := lockEnabledChannel(ctx, tx, input.ChannelNo); err != nil {
			return err
		}
		columns := dao.ProviderCapacityObservation.Columns()
		var existing entity.ProviderCapacityObservation
		if err := tx.Model(dao.ProviderCapacityObservation.Table()).Safe().Ctx(ctx).
			Fields(columns.Id).
			Where(do.ProviderCapacityObservation{
				TenantId: idccap.DefaultTenantID, ChannelNo: input.ChannelNo,
				CapacityKind: input.Kind, StatusCode: StatusCollecting,
			}).Limit(1).Scan(&existing); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return gerror.Wrap(err, "load collecting resource providerCapacity observation")
		}
		if existing.Id != 0 {
			return ErrObservationAlreadyCollecting
		}
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		_, err = tx.Model(dao.ProviderCapacityObservation.Table()).Safe().Ctx(ctx).Data(do.ProviderCapacityObservation{
			TenantId: idccap.DefaultTenantID, ObservationNo: observationNo,
			ChannelNo: input.ChannelNo, CapacityKind: input.Kind,
			StatusCode: StatusCollecting, PageSize: input.PageSize,
			NextCursor: "", PageCount: 0, ItemCount: 0,
			ObservationFingerprint: "", ObservedAt: &now,
			TriggerCode: options.TriggerCode, RequestedBy: options.RequestedBy,
			AttemptCount: 0, NextAttemptAt: &now,
		}).Insert()
		if err != nil {
			return gerror.Wrap(err, "insert resource providerCapacity observation")
		}
		created = Observation{
			ObservationNo: observationNo, Status: StatusCollecting,
			PageSize: input.PageSize,
		}
		return nil
	})
	if err != nil {
		return Observation{}, gerror.Wrap(err, "begin resource providerCapacity observation")
	}
	return created, nil
}

// Stage validates and inserts one exact continuation page. Only a terminal page
// supersedes the previous complete observation, within the same transaction.
func (s *serviceImpl) Stage(ctx context.Context, input StageInput) (Observation, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil {
		return Observation{}, gerror.New("resource providerCapacity observation service is unavailable")
	}
	if !observationNumberPattern.MatchString(input.ObservationNo) {
		return Observation{}, gerror.New("resource providerCapacity observation number is invalid")
	}
	if !emptyProviderCapacityFilters(input.Request.Filters) {
		return Observation{}, gerror.New("resource providerCapacity observation filters are not supported")
	}
	if err := input.Page.Validate(input.Request); err != nil {
		return Observation{}, gerror.Wrap(err, "validate resource providerCapacity page")
	}
	canonicalItems, err := canonicalizeItems(input.Page.Items)
	if err != nil {
		return Observation{}, err
	}
	staged := Observation{}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, err := lockCollectingObservation(ctx, tx, input.ObservationNo)
		if err != nil {
			return err
		}
		persistedLeaseDigest := strings.TrimSpace(row.LeaseTokenDigest)
		if input.leaseDigest != "" {
			if persistedLeaseDigest != input.leaseDigest {
				return ErrSynchronizationLeaseChanged
			}
		} else if persistedLeaseDigest != "" {
			return gerror.New("resource providerCapacity observation is leased")
		}
		if row.CapacityKind != string(input.Request.Kind) || row.PageSize != input.Request.Limit ||
			row.NextCursor != input.Request.Cursor {
			return gerror.New("resource providerCapacity continuation does not match persisted observation state")
		}
		if row.PageCount >= MaximumObservationPageCount {
			return gerror.New("resource providerCapacity observation page limit is exceeded")
		}
		if row.ItemCount > MaximumObservationItemCount-len(canonicalItems) {
			return gerror.New("resource providerCapacity observation item limit is exceeded")
		}
		fingerprint := pageFingerprint(
			row.ObservationFingerprint, row.ChannelNo, input.Request.Kind,
			row.PageSize, row.PageCount+1, input.Request.Cursor,
			input.Page.NextCursor, input.Page.Complete, canonicalItems,
		)
		if err := insertItems(ctx, tx, row, canonicalItems); err != nil {
			return err
		}
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		status := StatusCollecting
		completedAt := any(nil)
		observedAt := any(row.ObservedAt)
		nextAttemptAt := any(now)
		if input.Page.Complete {
			status = StatusComplete
			completedAt = now
			observedAt = now
			nextAttemptAt = nil
			columns := dao.ProviderCapacityObservation.Columns()
			if _, err := tx.Model(dao.ProviderCapacityObservation.Table()).Safe().Ctx(ctx).
				Where(do.ProviderCapacityObservation{
					TenantId: idccap.DefaultTenantID, ChannelNo: row.ChannelNo,
					CapacityKind: row.CapacityKind, StatusCode: StatusComplete,
				}).WhereNot(columns.ObservationNo, row.ObservationNo).
				Data(do.ProviderCapacityObservation{StatusCode: StatusSuperseded}).Update(); err != nil {
				return gerror.Wrap(err, "supersede previous resource providerCapacity observation")
			}
		}
		result, err := tx.Model(dao.ProviderCapacityObservation.Table()).Safe().Ctx(ctx).
			Where(do.ProviderCapacityObservation{
				TenantId: idccap.DefaultTenantID, ObservationNo: row.ObservationNo,
				StatusCode: StatusCollecting,
			}).Data(gdb.Map{
			"status_code": status, "observation_fingerprint": fingerprint,
			"item_count": row.ItemCount + len(canonicalItems),
			"page_count": row.PageCount + 1, "next_cursor": input.Page.NextCursor,
			"observed_at": observedAt, "completed_at": completedAt,
			"next_attempt_at": nextAttemptAt, "lease_token_digest": "",
			"lease_expires_at": nil,
		}).Update()
		if err != nil {
			return gerror.Wrap(err, "advance resource providerCapacity observation")
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return gerror.New("resource providerCapacity observation state changed concurrently")
		}
		staged = Observation{
			ObservationNo: row.ObservationNo, Status: status,
			ItemCount: row.ItemCount + len(canonicalItems), PageCount: row.PageCount + 1,
			PageSize: row.PageSize, NextCursor: input.Page.NextCursor,
			Fingerprint: fingerprint,
		}
		return nil
	})
	if err != nil {
		return Observation{}, gerror.Wrap(err, "stage resource providerCapacity observation page")
	}
	return staged, nil
}

// Fail closes one collecting observation without changing the last complete publication.
func (s *serviceImpl) Fail(ctx context.Context, observationNo string) error {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil {
		return gerror.New("resource providerCapacity observation service is unavailable")
	}
	if !observationNumberPattern.MatchString(observationNo) {
		return gerror.New("resource providerCapacity observation number is invalid")
	}
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, err := lockCollectingObservation(ctx, tx, observationNo)
		if err != nil {
			return err
		}
		now, err := databaseTime(tx)
		if err != nil {
			return err
		}
		result, err := tx.Model(dao.ProviderCapacityObservation.Table()).Safe().Ctx(ctx).
			Where(do.ProviderCapacityObservation{
				TenantId: idccap.DefaultTenantID, ObservationNo: row.ObservationNo,
				StatusCode: StatusCollecting,
			}).Data(gdb.Map{
			"status_code": StatusFailed, "completed_at": now,
			"next_attempt_at": nil, "lease_token_digest": "", "lease_expires_at": nil,
			"failure_code": "synchronization_failed",
		}).Update()
		if err != nil {
			return gerror.Wrap(err, "fail resource providerCapacity observation")
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return gerror.New("resource providerCapacity observation state changed concurrently")
		}
		return nil
	})
}

func validateBeginInput(input BeginInput) error {
	if err := resourcecap.ValidateChannelRef(input.ChannelNo); err != nil {
		return err
	}
	switch input.Kind {
	case resourcecap.ProviderCapacityKindRegion, resourcecap.ProviderCapacityKindLine,
		resourcecap.ProviderCapacityKindPlan, resourcecap.ProviderCapacityKindImage,
		resourcecap.ProviderCapacityKindStock:
	default:
		return gerror.New("resource providerCapacity observation kind is invalid")
	}
	return (resourcecap.ProviderCapacityRequest{Kind: input.Kind, Limit: input.PageSize}).Validate()
}

func emptyProviderCapacityFilters(filters json.RawMessage) bool {
	trimmed := bytes.TrimSpace(filters)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("{}"))
}

func newObservationNumber(reader io.Reader) (string, error) {
	buffer := make([]byte, 16)
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return "", gerror.Wrap(err, "generate resource providerCapacity observation number")
	}
	return "capacity_observation_" + hex.EncodeToString(buffer), nil
}

func lockEnabledChannel(ctx context.Context, tx gdb.TX, channelNo string) error {
	columns := dao.Channel.Columns()
	var channel entity.Channel
	if err := tx.Model(dao.Channel.Table()).Safe().Ctx(ctx).
		Fields(columns.Id).
		Where(do.Channel{
			TenantId: idccap.DefaultTenantID, ChannelNo: channelNo, StatusCode: "enabled",
		}).Lock(gdb.LockForUpdate).Scan(&channel); err != nil {
		return gerror.Wrap(err, "lock enabled resource providerCapacity channel")
	}
	if channel.Id == 0 {
		return gerror.New("enabled resource providerCapacity channel is unavailable")
	}
	return nil
}

func lockCollectingObservation(
	ctx context.Context,
	tx gdb.TX,
	observationNo string,
) (*entity.ProviderCapacityObservation, error) {
	columns := dao.ProviderCapacityObservation.Columns()
	var row entity.ProviderCapacityObservation
	if err := tx.Model(dao.ProviderCapacityObservation.Table()).Safe().Ctx(ctx).
		Fields(
			columns.ObservationNo, columns.ChannelNo, columns.CapacityKind,
			columns.StatusCode, columns.ObservationFingerprint, columns.ItemCount,
			columns.ObservedAt, columns.PageSize, columns.NextCursor, columns.PageCount,
			columns.LeaseTokenDigest,
		).
		Where(do.ProviderCapacityObservation{
			TenantId: idccap.DefaultTenantID, ObservationNo: observationNo,
			StatusCode: StatusCollecting,
		}).Lock(gdb.LockForUpdate).Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "lock collecting resource providerCapacity observation")
	}
	if row.ObservationNo == "" || row.ObservedAt == nil {
		return nil, gerror.New("collecting resource providerCapacity observation is unavailable")
	}
	return &row, nil
}

type canonicalItem struct {
	externalRef string
	projection  []byte
	observedAt  time.Time
}

func canonicalizeItems(items []resourcecap.ProviderCapacityItem) ([]canonicalItem, error) {
	canonical := make([]canonicalItem, len(items))
	for index := range items {
		decoder := json.NewDecoder(bytes.NewReader(items[index].Data))
		decoder.UseNumber()
		var projection map[string]any
		if err := decoder.Decode(&projection); err != nil {
			return nil, gerror.New("resource providerCapacity item projection is invalid")
		}
		encoded, err := json.Marshal(projection)
		if err != nil {
			return nil, gerror.New("resource providerCapacity item projection cannot be normalized")
		}
		canonical[index] = canonicalItem{
			externalRef: items[index].ExternalRef, projection: encoded,
			observedAt: items[index].ObservedAt.UTC().Truncate(time.Microsecond),
		}
	}
	sort.Slice(canonical, func(left, right int) bool {
		return canonical[left].externalRef < canonical[right].externalRef
	})
	return canonical, nil
}

func pageFingerprint(
	previous string,
	channelNo string,
	kind resourcecap.ProviderCapacityKind,
	pageSize int,
	pageNumber int,
	requestCursor string,
	nextCursor string,
	complete bool,
	items []canonicalItem,
) string {
	digest := sha256.New()
	writeDigestField(digest, []byte("idc-resource/provider-capacity-page/v1"))
	writeDigestField(digest, []byte(previous))
	writeDigestField(digest, []byte(channelNo))
	writeDigestField(digest, []byte(kind))
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], uint64(pageSize))
	writeDigestField(digest, number[:])
	binary.BigEndian.PutUint64(number[:], uint64(pageNumber))
	writeDigestField(digest, number[:])
	writeDigestField(digest, []byte(requestCursor))
	writeDigestField(digest, []byte(nextCursor))
	if complete {
		writeDigestField(digest, []byte{1})
	} else {
		writeDigestField(digest, []byte{0})
	}
	for index := range items {
		writeDigestField(digest, []byte(items[index].externalRef))
		writeDigestField(digest, items[index].projection)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func writeDigestField(writer io.Writer, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write(value)
}

func insertItems(
	ctx context.Context,
	tx gdb.TX,
	observation *entity.ProviderCapacityObservation,
	items []canonicalItem,
) error {
	if len(items) == 0 {
		return nil
	}
	var statement strings.Builder
	statement.WriteString(`INSERT INTO plugin_idc_resource_provider_capacity_observation_item
    (tenant_id, observation_no, capacity_kind, external_ref, projection, observed_at)
VALUES `)
	arguments := make([]any, 0, len(items)*6)
	for index := range items {
		if index > 0 {
			statement.WriteByte(',')
		}
		statement.WriteString("(?, ?, ?, ?, CAST(? AS JSONB), ?)")
		arguments = append(arguments,
			idccap.DefaultTenantID, observation.ObservationNo, observation.CapacityKind,
			items[index].externalRef, string(items[index].projection), items[index].observedAt,
		)
	}
	if _, err := tx.Exec(statement.String(), arguments...); err != nil {
		return gerror.Wrap(err, "insert resource providerCapacity observation item batch")
	}
	return nil
}

func databaseTime(tx gdb.TX) (time.Time, error) {
	value, err := tx.GetValue("SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "read resource providerCapacity database time")
	}
	now := value.Time()
	if now.IsZero() {
		return time.Time{}, gerror.New("resource providerCapacity database time is unavailable")
	}
	return now.UTC(), nil
}
