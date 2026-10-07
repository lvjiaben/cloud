// This file owns points award, refund reversal, and debt repayment semantics.

package marketing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const maximumPointsPerOperation int64 = 1_000_000_000_000_000

// PointsAwardInput appends one idempotent source award.
type PointsAwardInput struct {
	CustomerNo      string
	SourceType      string
	SourceReference string
	Points          int64
	CampaignNo      string
	ExpiresAt       *time.Time
}

// PointsReverseInput reverses one award due to an explicit refund or manual correction.
type PointsReverseInput struct {
	CustomerNo              string
	AwardSourceType         string
	AwardSourceReference    string
	ReversalSourceType      string
	ReversalSourceReference string
	Points                  int64
}

// PointsMutation is a stable projection of an awarded or reversed quantity.
type PointsMutation struct {
	LotNo              string
	CustomerNo         string
	Points             int64
	AvailableDelta     int64
	FrozenDelta        int64
	DebtDelta          int64
	AvailablePoints    int64
	FrozenPoints       int64
	DebtPoints         int64
	LotAvailablePoints int64
	LotFrozenPoints    int64
	LotConsumedPoints  int64
	LotReversedPoints  int64
	ReversalShortfall  int64
	Reused             bool
}

// PointsService owns append-only award and reversal flows. It never treats points as money.
type PointsService interface {
	Award(ctx context.Context, input PointsAwardInput) (*PointsMutation, error)
	ReverseAward(ctx context.Context, input PointsReverseInput) (*PointsMutation, error)
}

type pointsServiceImpl struct {
	database gdb.DB
	random   io.Reader
}

var _ PointsService = (*pointsServiceImpl)(nil)

// NewPointsService constructs a local points service.
func NewPointsService(database gdb.DB) (PointsService, error) {
	return newPointsService(database, rand.Reader)
}

func newPointsService(database gdb.DB, random io.Reader) (*pointsServiceImpl, error) {
	if database == nil {
		return nil, gerror.New("points service requires a database")
	}
	if random == nil {
		return nil, gerror.New("points service requires a random source")
	}
	return &pointsServiceImpl{database: database, random: random}, nil
}

type pointsAccountRow struct {
	AvailablePoints int64 `orm:"available_points"`
	FrozenPoints    int64 `orm:"frozen_points"`
	DebtPoints      int64 `orm:"debt_points"`
}

type pointsLotRow struct {
	LotNo                   string     `orm:"lot_no"`
	CustomerNo              string     `orm:"customer_no"`
	CampaignNo              *string    `orm:"campaign_no"`
	SourceType              string     `orm:"source_type_code"`
	SourceReference         string     `orm:"source_reference"`
	OriginalPoints          int64      `orm:"original_points"`
	AvailablePoints         int64      `orm:"available_points"`
	FrozenPoints            int64      `orm:"frozen_points"`
	ConsumedPoints          int64      `orm:"consumed_points"`
	ExpiredPoints           int64      `orm:"expired_points"`
	ReversedPoints          int64      `orm:"reversed_points"`
	ReversalShortfallPoints int64      `orm:"reversal_shortfall_points"`
	ExpiresAt               *time.Time `orm:"expires_at"`
	StatusCode              string     `orm:"status_code"`
}

type pointsEntryRow struct {
	EntryNo           string  `orm:"entry_no"`
	LotNo             *string `orm:"lot_no"`
	AvailableDelta    int64   `orm:"available_delta"`
	FrozenDelta       int64   `orm:"frozen_delta"`
	DebtDelta         int64   `orm:"debt_delta"`
	ReversalOfEntryNo *string `orm:"reversal_of_entry_no"`
}

func (s *pointsServiceImpl) Award(ctx context.Context, input PointsAwardInput) (*PointsMutation, error) {
	normalized, err := normalizePointsAward(input)
	if err != nil {
		return nil, err
	}
	var result *PointsMutation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		account, lockErr := lockPointsAccount(ctx, s.database, normalized.CustomerNo, true)
		if lockErr != nil {
			return lockErr
		}
		existing, loadErr := loadPointsLotBySource(
			ctx, s.database, normalized.CustomerNo, normalized.SourceType,
			normalized.SourceReference, true,
		)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if !sameAward(existing, normalized) {
				return bizerr.NewCode(CodePointsConflict)
			}
			result = projectPointsMutation(existing, account, normalized.Points, 0, 0, 0, true)
			return nil
		}
		if normalized.ExpiresAt != nil {
			now, nowErr := s.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
			if nowErr != nil {
				return gerror.Wrap(nowErr, "read points award database time")
			}
			if !normalized.ExpiresAt.After(now.Time()) {
				return bizerr.NewCode(CodePointsInvalid)
			}
		}
		if normalized.CampaignNo != "" {
			campaign, campaignErr := s.database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_marketing_campaign
WHERE tenant_id = 0 AND campaign_no = ? AND campaign_type_code = 'points'`, normalized.CampaignNo)
			if campaignErr != nil {
				return gerror.Wrap(campaignErr, "validate points award campaign")
			}
			if campaign.Int() != 1 {
				return bizerr.NewCode(CodePointsConflict)
			}
		}
		debtRepaid := minInt64(account.DebtPoints, normalized.Points)
		availableAward := normalized.Points - debtRepaid
		lotNo, numberErr := newMarketingNumber(s.random, "plot_")
		if numberErr != nil {
			return numberErr
		}
		status := "available"
		if availableAward == 0 {
			status = "reversed"
		}
		var campaignNo any
		var campaignType any
		if normalized.CampaignNo != "" {
			campaignNo = normalized.CampaignNo
			campaignType = "points"
		}
		_, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_points_lot (
    tenant_id, lot_no, customer_no, campaign_no, campaign_type_code,
    source_type_code, source_reference, original_points, available_points,
    reversed_points, expires_at, status_code
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			lotNo, normalized.CustomerNo, campaignNo, campaignType,
			normalized.SourceType, normalized.SourceReference, normalized.Points,
			availableAward, debtRepaid, normalized.ExpiresAt, status,
		)
		if insertErr != nil {
			return gerror.Wrap(insertErr, "insert points award lot")
		}
		accountResult, accountErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_points_account
SET available_points = available_points + ?, debt_points = debt_points - ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND customer_no = ? AND debt_points >= ?`,
			availableAward, debtRepaid, normalized.CustomerNo, debtRepaid,
		)
		if accountErr != nil {
			return gerror.Wrap(accountErr, "apply points award account projection")
		}
		if affectedErr := requireOneAffected(accountResult, "apply points award account projection"); affectedErr != nil {
			return affectedErr
		}
		lineNo := 1
		if availableAward > 0 {
			if _, entryErr := s.insertPointsEntry(ctx, pointsEntryInsert{
				CustomerNo: normalized.CustomerNo, LotNo: lotNo, EntryType: "earn",
				AvailableDelta: availableAward, SourceType: normalized.SourceType,
				SourceReference: normalized.SourceReference, LineNo: lineNo,
			}); entryErr != nil {
				return entryErr
			}
			lineNo++
		}
		if debtRepaid > 0 {
			if _, entryErr := s.insertPointsEntry(ctx, pointsEntryInsert{
				CustomerNo: normalized.CustomerNo, LotNo: lotNo, EntryType: "debt_repay",
				DebtDelta: -debtRepaid, SourceType: normalized.SourceType,
				SourceReference: normalized.SourceReference, LineNo: lineNo,
			}); entryErr != nil {
				return entryErr
			}
		}
		account.AvailablePoints += availableAward
		account.DebtPoints -= debtRepaid
		lot := &pointsLotRow{
			LotNo: lotNo, CustomerNo: normalized.CustomerNo, OriginalPoints: normalized.Points,
			AvailablePoints: availableAward, ReversedPoints: debtRepaid,
			ExpiresAt: normalized.ExpiresAt, StatusCode: status,
		}
		result = projectPointsMutation(lot, account, normalized.Points, availableAward, 0, -debtRepaid, false)
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodePointsUnavailable)
	}
	return result, nil
}

func (s *pointsServiceImpl) ReverseAward(
	ctx context.Context,
	input PointsReverseInput,
) (*PointsMutation, error) {
	normalized, err := normalizePointsReverse(input)
	if err != nil {
		return nil, err
	}
	var result *PointsMutation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		account, lockErr := lockPointsAccount(ctx, s.database, normalized.CustomerNo, false)
		if lockErr != nil {
			return lockErr
		}
		if account == nil {
			return bizerr.NewCode(CodePointsConflict)
		}
		replayed, replayErr := loadPointsEntryBySource(
			ctx, s.database, normalized.CustomerNo, normalized.ReversalSourceType,
			normalized.ReversalSourceReference, "reverse",
		)
		if replayErr != nil {
			return replayErr
		}
		lot, loadErr := loadPointsLotBySource(
			ctx, s.database, normalized.CustomerNo, normalized.AwardSourceType,
			normalized.AwardSourceReference, true,
		)
		if loadErr != nil {
			return loadErr
		}
		if lot == nil {
			return bizerr.NewCode(CodePointsConflict)
		}
		originalEntry, entryErr := loadOriginalEarnEntry(ctx, s.database, lot.LotNo, lot.CustomerNo)
		if entryErr != nil {
			return entryErr
		}
		if replayed != nil {
			if originalEntry == nil || replayed.ReversalOfEntryNo == nil ||
				*replayed.ReversalOfEntryNo != originalEntry.EntryNo ||
				-replayed.AvailableDelta-replayed.FrozenDelta+replayed.DebtDelta != normalized.Points {
				return bizerr.NewCode(CodePointsConflict)
			}
			result = projectPointsMutation(
				lot, account, normalized.Points, replayed.AvailableDelta,
				replayed.FrozenDelta, replayed.DebtDelta, true,
			)
			return nil
		}
		if originalEntry == nil {
			return bizerr.NewCode(CodePointsConflict)
		}
		remainingConsumed := lot.ConsumedPoints - lot.ReversalShortfallPoints
		capacity := lot.AvailablePoints + lot.FrozenPoints + remainingConsumed
		if normalized.Points > capacity {
			return bizerr.NewCode(CodePointsConflict)
		}
		fromAvailable := minInt64(lot.AvailablePoints, normalized.Points)
		remaining := normalized.Points - fromAvailable
		fromFrozen := minInt64(lot.FrozenPoints, remaining)
		shortfall := remaining - fromFrozen
		status := pointsLotStatus(
			lot.AvailablePoints-fromAvailable,
			lot.FrozenPoints-fromFrozen,
			lot.ConsumedPoints,
			lot.ExpiredPoints,
		)
		lotResult, lotErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_points_lot
SET available_points = available_points - ?, frozen_points = frozen_points - ?,
    reversed_points = reversed_points + ?,
    reversal_shortfall_points = reversal_shortfall_points + ?,
    status_code = ?, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND lot_no = ? AND customer_no = ?
  AND available_points >= ? AND frozen_points >= ?
  AND consumed_points - reversal_shortfall_points >= ?`,
			fromAvailable, fromFrozen, fromAvailable+fromFrozen, shortfall, status,
			lot.LotNo, lot.CustomerNo, fromAvailable, fromFrozen, shortfall,
		)
		if lotErr != nil {
			return gerror.Wrap(lotErr, "apply points award reversal")
		}
		if affectedErr := requireOneAffected(lotResult, "apply points award reversal"); affectedErr != nil {
			return affectedErr
		}
		accountResult, accountErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_points_account
SET available_points = available_points - ?, frozen_points = frozen_points - ?,
    debt_points = debt_points + ?, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND customer_no = ?
  AND available_points >= ? AND frozen_points >= ?`,
			fromAvailable, fromFrozen, shortfall, normalized.CustomerNo, fromAvailable, fromFrozen,
		)
		if accountErr != nil {
			return gerror.Wrap(accountErr, "apply points reversal account projection")
		}
		if affectedErr := requireOneAffected(accountResult, "apply points reversal account projection"); affectedErr != nil {
			return affectedErr
		}
		_, insertErr := s.insertPointsEntry(ctx, pointsEntryInsert{
			CustomerNo: normalized.CustomerNo, LotNo: lot.LotNo, EntryType: "reverse",
			AvailableDelta: -fromAvailable, FrozenDelta: -fromFrozen, DebtDelta: shortfall,
			SourceType:      normalized.ReversalSourceType,
			SourceReference: normalized.ReversalSourceReference, LineNo: 1,
			ReversalOfEntryNo: originalEntry.EntryNo,
		})
		if insertErr != nil {
			return insertErr
		}
		lot.AvailablePoints -= fromAvailable
		lot.FrozenPoints -= fromFrozen
		lot.ReversedPoints += fromAvailable + fromFrozen
		lot.ReversalShortfallPoints += shortfall
		lot.StatusCode = status
		account.AvailablePoints -= fromAvailable
		account.FrozenPoints -= fromFrozen
		account.DebtPoints += shortfall
		result = projectPointsMutation(
			lot, account, normalized.Points, -fromAvailable, -fromFrozen, shortfall, false,
		)
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodePointsUnavailable)
	}
	return result, nil
}

type pointsEntryInsert struct {
	CustomerNo        string
	LotNo             string
	EntryType         string
	AvailableDelta    int64
	FrozenDelta       int64
	DebtDelta         int64
	SourceType        string
	SourceReference   string
	LineNo            int
	ReversalOfEntryNo string
}

func (s *pointsServiceImpl) insertPointsEntry(
	ctx context.Context,
	input pointsEntryInsert,
) (string, error) {
	entryNo, err := newMarketingNumber(s.random, "pent_")
	if err != nil {
		return "", err
	}
	var reversal any
	if input.ReversalOfEntryNo != "" {
		reversal = input.ReversalOfEntryNo
	}
	_, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_points_entry (
    tenant_id, entry_no, customer_no, lot_no, entry_type_code,
    available_delta, frozen_delta, debt_delta, source_type_code,
    source_reference, entry_line_no, reversal_of_entry_no
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entryNo, input.CustomerNo, input.LotNo, input.EntryType,
		input.AvailableDelta, input.FrozenDelta, input.DebtDelta,
		input.SourceType, input.SourceReference, input.LineNo, reversal,
	)
	if err != nil {
		return "", gerror.Wrap(err, "insert points entry")
	}
	return entryNo, nil
}

func lockPointsAccount(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	create bool,
) (*pointsAccountRow, error) {
	if create {
		if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_points_account (tenant_id, customer_no)
VALUES (0, ?) ON CONFLICT (customer_no) DO NOTHING`, customerNo); err != nil {
			return nil, gerror.Wrap(err, "initialize points account")
		}
	}
	var account *pointsAccountRow
	if err := database.GetScan(ctx, &account, `
SELECT available_points, frozen_points, debt_points
FROM plugin_idc_core_marketing_points_account
WHERE tenant_id = 0 AND customer_no = ? FOR UPDATE`, customerNo); err != nil {
		return nil, gerror.Wrap(err, "lock points account")
	}
	return account, nil
}

func loadPointsLotBySource(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	sourceType string,
	sourceReference string,
	lock bool,
) (*pointsLotRow, error) {
	query := `
SELECT lot_no, customer_no, campaign_no, source_type_code, source_reference,
       original_points, available_points, frozen_points, consumed_points,
       expired_points, reversed_points, reversal_shortfall_points, expires_at, status_code
FROM plugin_idc_core_marketing_points_lot
WHERE tenant_id = 0 AND customer_no = ? AND source_type_code = ? AND source_reference = ?`
	if lock {
		query += ` FOR UPDATE`
	}
	var lot *pointsLotRow
	if err := database.GetScan(ctx, &lot, query, customerNo, sourceType, sourceReference); err != nil {
		return nil, gerror.Wrap(err, "load points award lot")
	}
	return lot, nil
}

func loadPointsEntryBySource(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	sourceType string,
	sourceReference string,
	entryType string,
) (*pointsEntryRow, error) {
	var entry *pointsEntryRow
	if err := database.GetScan(ctx, &entry, `
SELECT entry_no, lot_no, available_delta, frozen_delta, debt_delta, reversal_of_entry_no
FROM plugin_idc_core_marketing_points_entry
WHERE tenant_id = 0 AND customer_no = ? AND source_type_code = ?
  AND source_reference = ? AND entry_type_code = ? AND entry_line_no = 1
FOR UPDATE`, customerNo, sourceType, sourceReference, entryType); err != nil {
		return nil, gerror.Wrap(err, "load points entry by source")
	}
	return entry, nil
}

func loadOriginalEarnEntry(
	ctx context.Context,
	database gdb.DB,
	lotNo string,
	customerNo string,
) (*pointsEntryRow, error) {
	var entry *pointsEntryRow
	if err := database.GetScan(ctx, &entry, `
SELECT entry_no, lot_no, available_delta, frozen_delta, debt_delta, reversal_of_entry_no
FROM plugin_idc_core_marketing_points_entry
WHERE tenant_id = 0 AND lot_no = ? AND customer_no = ? AND entry_type_code = 'earn'
ORDER BY entry_line_no ASC LIMIT 1`, lotNo, customerNo); err != nil {
		return nil, gerror.Wrap(err, "load original points earning entry")
	}
	return entry, nil
}

func normalizePointsAward(input PointsAwardInput) (PointsAwardInput, error) {
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	input.CampaignNo = strings.TrimSpace(input.CampaignNo)
	if !identifierPattern.MatchString(input.CustomerNo) ||
		!validPointsAwardSource(input.SourceType) || !validBusinessReference(input.SourceReference) ||
		input.Points <= 0 || input.Points > maximumPointsPerOperation ||
		(input.CampaignNo != "" && !identifierPattern.MatchString(input.CampaignNo)) {
		return PointsAwardInput{}, bizerr.NewCode(CodePointsInvalid)
	}
	if input.ExpiresAt != nil {
		normalized := input.ExpiresAt.UTC().Truncate(time.Microsecond)
		input.ExpiresAt = &normalized
	}
	return input, nil
}

func normalizePointsReverse(input PointsReverseInput) (PointsReverseInput, error) {
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.AwardSourceType = strings.TrimSpace(input.AwardSourceType)
	input.AwardSourceReference = strings.TrimSpace(input.AwardSourceReference)
	input.ReversalSourceType = strings.TrimSpace(input.ReversalSourceType)
	input.ReversalSourceReference = strings.TrimSpace(input.ReversalSourceReference)
	if !identifierPattern.MatchString(input.CustomerNo) || !validPointsAwardSource(input.AwardSourceType) ||
		!validBusinessReference(input.AwardSourceReference) ||
		(input.ReversalSourceType != "refund" && input.ReversalSourceType != "manual") ||
		!validBusinessReference(input.ReversalSourceReference) ||
		input.Points <= 0 || input.Points > maximumPointsPerOperation {
		return PointsReverseInput{}, bizerr.NewCode(CodePointsInvalid)
	}
	return input, nil
}

func validPointsAwardSource(value string) bool {
	return value == "order" || value == "activity" || value == "manual" || value == "migration"
}

func validBusinessReference(value string) bool {
	if value == "" || len(value) > 128 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func sameAward(lot *pointsLotRow, input PointsAwardInput) bool {
	if lot.OriginalPoints != input.Points || lot.SourceType != input.SourceType ||
		lot.SourceReference != input.SourceReference {
		return false
	}
	storedCampaign := ""
	if lot.CampaignNo != nil {
		storedCampaign = *lot.CampaignNo
	}
	if storedCampaign != input.CampaignNo {
		return false
	}
	if lot.ExpiresAt == nil || input.ExpiresAt == nil {
		return lot.ExpiresAt == nil && input.ExpiresAt == nil
	}
	return lot.ExpiresAt.UTC().Equal(input.ExpiresAt.UTC())
}

func pointsLotStatus(available int64, frozen int64, consumed int64, expired int64) string {
	if available+frozen > 0 {
		return "available"
	}
	if consumed > 0 {
		return "exhausted"
	}
	if expired > 0 {
		return "expired"
	}
	return "reversed"
}

func projectPointsMutation(
	lot *pointsLotRow,
	account *pointsAccountRow,
	points int64,
	availableDelta int64,
	frozenDelta int64,
	debtDelta int64,
	reused bool,
) *PointsMutation {
	return &PointsMutation{
		LotNo: lot.LotNo, CustomerNo: lot.CustomerNo, Points: points,
		AvailableDelta: availableDelta, FrozenDelta: frozenDelta, DebtDelta: debtDelta,
		AvailablePoints: account.AvailablePoints, FrozenPoints: account.FrozenPoints,
		DebtPoints: account.DebtPoints, LotAvailablePoints: lot.AvailablePoints,
		LotFrozenPoints: lot.FrozenPoints, LotConsumedPoints: lot.ConsumedPoints,
		LotReversedPoints: lot.ReversedPoints,
		ReversalShortfall: lot.ReversalShortfallPoints, Reused: reused,
	}
}

func newMarketingNumber(random io.Reader, prefix string) (string, error) {
	raw := make([]byte, 16)
	if _, err := io.ReadFull(random, raw); err != nil {
		return "", gerror.Wrap(err, "generate marketing business number")
	}
	return prefix + hex.EncodeToString(raw), nil
}

func minInt64(left int64, right int64) int64 {
	if left < right {
		return left
	}
	return right
}
