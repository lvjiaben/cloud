// This file resolves bounded configuration price selections and verifies their
// immutable quantity and algorithm when restoring checkout snapshots.

package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

const (
	maximumConfigurationPriceDimensions         = 4096
	maximumSelectedConfigurationPriceDimensions = 128
	maximumConfigurationPriceSelectionSnapshots = 100
)

var (
	configurationPriceDimensionNoPattern = regexp.MustCompile(`^cpd_[0-9a-f]{32}$`)
	configurationPriceSnapshotNoPattern  = regexp.MustCompile(`^CFG_[0-9a-f]{32}$`)
)

type configurationPriceDimensionIdentity struct {
	DimensionNo, FieldKey, CandidateValue string
	PricingMode                           ConfigurationPriceMode `orm:"pricing_mode_code"`
}

type ConfigurationPriceSelection struct {
	DimensionNo string
	Quantity    int
	PricingMode ConfigurationPriceMode
}

// ConfigurationPriceSelectionReader restores the immutable core-owned price
// choices saved for completed purchases. It never returns plugin field keys or
// customer values.
type ConfigurationPriceSelectionReader interface {
	LoadConfigurationPriceSelections(ctx context.Context, snapshotNos []string) (map[string][]ConfigurationPriceSelection, error)
}

var _ ConfigurationPriceSelectionReader = (*publicCatalogReaderImpl)(nil)

// ConfigurationPriceSelectionReaderFromPublicReader keeps renewal and other
// trusted callers on a narrow, read-only contract.
func ConfigurationPriceSelectionReaderFromPublicReader(reader PublicCatalogReader) (ConfigurationPriceSelectionReader, error) {
	result, ok := reader.(ConfigurationPriceSelectionReader)
	if !ok || result == nil {
		return nil, gerror.New("catalog reader does not restore configuration price selections")
	}
	return result, nil
}

func loadSelectedConfigurationPriceDimensions(ctx context.Context, database gdb.DB, versionNo string, values []configurationcontract.ConfigurationValue) ([]ConfigurationPriceSelection, string, error) {
	var rows []configurationPriceDimensionIdentity
	if err := database.GetScan(ctx, &rows, `SELECT dimension_no,field_key,candidate_value,pricing_mode_code
FROM plugin_idc_core_catalog_configuration_price_dimension
WHERE tenant_id=0 AND version_no=? AND status_code='active'
ORDER BY field_key,candidate_value,id LIMIT ?`, versionNo, maximumConfigurationPriceDimensions+1); err != nil {
		return nil, "", gerror.Wrap(err, "load configuration price dimensions")
	}
	if len(rows) > maximumConfigurationPriceDimensions {
		return nil, "", gerror.New("configuration price dimension bound exceeded")
	}
	result, selectionErr := selectConfigurationPriceDimensions(rows, values)
	if selectionErr != nil {
		return nil, "", selectionErr
	}
	if len(result) > maximumSelectedConfigurationPriceDimensions {
		return nil, "", gerror.New("selected configuration price dimension bound exceeded")
	}
	sort.Slice(result, func(i, j int) bool { return result[i].DimensionNo < result[j].DimensionNo })
	digest, err := ConfigurationPriceSelectionDigest(result)
	if err != nil {
		return nil, "", err
	}
	return result, digest, nil
}

func configurationPriceSelectionMap(input []ConfigurationPriceSelection) map[string]int {
	result := make(map[string]int, len(input))
	for _, item := range input {
		result[item.DimensionNo] = item.Quantity
	}
	return result
}

// ConfigurationPriceSelectionDigest binds a term or snapshot to the complete,
// sorted price selection without exposing business-specific configuration.
func ConfigurationPriceSelectionDigest(input []ConfigurationPriceSelection) (string, error) {
	if len(input) > maximumSelectedConfigurationPriceDimensions {
		return "", gerror.New("selected configuration price dimension bound exceeded")
	}
	items := append([]ConfigurationPriceSelection(nil), input...)
	sort.Slice(items, func(i, j int) bool { return items[i].DimensionNo < items[j].DimensionNo })
	digestInput := make([]byte, 0, len(items)*48)
	previous := ""
	for _, item := range items {
		if !configurationPriceDimensionNoPattern.MatchString(item.DimensionNo) || item.Quantity < 1 || item.Quantity > math.MaxInt32 || item.DimensionNo == previous {
			return "", gerror.New("invalid configuration price selection")
		}
		if !validConfigurationPriceMode(item.PricingMode) {
			return "", gerror.New("invalid configuration pricing mode")
		}
		previous = item.DimensionNo
		digestInput = append(digestInput, item.DimensionNo...)
		digestInput = append(digestInput, 0)
		digestInput = strconv.AppendInt(digestInput, int64(item.Quantity), 10)
		if item.PricingMode != "" && item.PricingMode != ConfigurationPriceFixed {
			digestInput = append(digestInput, 0)
			digestInput = append(digestInput, string(item.PricingMode)...)
		}
		digestInput = append(digestInput, 0)
	}
	digest := sha256.Sum256(digestInput)
	return hex.EncodeToString(digest[:]), nil
}

type configurationPriceSelectionSnapshotRow struct {
	SnapshotNo          string                 `orm:"configuration_snapshot_no"`
	SelectionDigest     string                 `orm:"selection_digest"`
	SelectedOptionCount int                    `orm:"selected_option_count"`
	DimensionNo         *string                `orm:"dimension_no"`
	Quantity            *int                   `orm:"quantity"`
	PricingMode         ConfigurationPriceMode `orm:"pricing_mode_code"`
}

// LoadConfigurationPriceSelections restores up to 100 snapshots in one query
// and verifies their recorded count and digest before returning any selection.
func (s *publicCatalogReaderImpl) LoadConfigurationPriceSelections(ctx context.Context, snapshotNos []string) (map[string][]ConfigurationPriceSelection, error) {
	if s == nil || s.database == nil || len(snapshotNos) == 0 || len(snapshotNos) > maximumConfigurationPriceSelectionSnapshots {
		return nil, gerror.New("invalid configuration price snapshot request")
	}
	unique := make(map[string]struct{}, len(snapshotNos))
	for _, snapshotNo := range snapshotNos {
		if !configurationPriceSnapshotNoPattern.MatchString(snapshotNo) {
			return nil, gerror.New("invalid configuration price snapshot reference")
		}
		unique[snapshotNo] = struct{}{}
	}
	if len(unique) != len(snapshotNos) {
		return nil, gerror.New("duplicate configuration price snapshot reference")
	}
	var rows []configurationPriceSelectionSnapshotRow
	if err := s.database.GetScan(ctx, &rows, `SELECT snapshot.configuration_snapshot_no,snapshot.selection_digest,snapshot.selected_option_count,
selection.dimension_no,selection.quantity,dimension.pricing_mode_code
FROM plugin_idc_core_commerce_configuration_snapshot snapshot
LEFT JOIN plugin_idc_core_commerce_configuration_price_selection selection
  ON selection.tenant_id=snapshot.tenant_id
 AND selection.configuration_snapshot_no=snapshot.configuration_snapshot_no
LEFT JOIN plugin_idc_core_catalog_configuration_price_dimension dimension
 ON dimension.tenant_id=selection.tenant_id AND dimension.version_no=selection.version_no AND dimension.dimension_no=selection.dimension_no
WHERE snapshot.tenant_id=0
  AND snapshot.configuration_snapshot_no = ANY(string_to_array(?, ','))
ORDER BY snapshot.configuration_snapshot_no,selection.dimension_no`, strings.Join(snapshotNos, ",")); err != nil {
		return nil, gerror.Wrap(err, "load configuration price selections")
	}
	result := make(map[string][]ConfigurationPriceSelection, len(snapshotNos))
	digests := make(map[string]string, len(snapshotNos))
	counts := make(map[string]int, len(snapshotNos))
	for _, row := range rows {
		if _, exists := result[row.SnapshotNo]; !exists {
			result[row.SnapshotNo] = []ConfigurationPriceSelection{}
		}
		digests[row.SnapshotNo] = row.SelectionDigest
		counts[row.SnapshotNo] = row.SelectedOptionCount
		if row.DimensionNo != nil && row.Quantity != nil {
			result[row.SnapshotNo] = append(result[row.SnapshotNo], ConfigurationPriceSelection{DimensionNo: *row.DimensionNo, Quantity: *row.Quantity, PricingMode: row.PricingMode})
		}
	}
	for _, snapshotNo := range snapshotNos {
		selections, found := result[snapshotNo]
		if !found || len(selections) != counts[snapshotNo] {
			return nil, gerror.New("configuration price snapshot is incomplete")
		}
		digest, err := ConfigurationPriceSelectionDigest(selections)
		if err != nil || digest != digests[snapshotNo] {
			return nil, gerror.New("configuration price snapshot integrity check failed")
		}
	}
	return result, nil
}

// selectConfigurationPriceDimensions operates only on values already validated
// against the frozen provider schema. Zero quantity intentionally has no charge
// selection, so no configuration setup fee can leak into checkout.
func selectConfigurationPriceDimensions(rows []configurationPriceDimensionIdentity, values []configurationcontract.ConfigurationValue) ([]ConfigurationPriceSelection, error) {
	selected := make(map[string]int)
	quantities := make(map[string]int64)
	for _, value := range values {
		for _, candidate := range append(append(append([]string{}, value.OptionTokens...), value.TreeOptionTokens...), value.AddressOptionTokens...) {
			selected[value.FieldKey+"\x00"+candidate] = 1
		}
		if value.IntegerValue != nil && value.IntegerTextValue != nil {
			return nil, billingInvalid()
		}
		if value.IntegerValue != nil {
			quantities[value.FieldKey] = *value.IntegerValue
		}
		if value.IntegerTextValue != nil {
			parsed, err := strconv.ParseInt(*value.IntegerTextValue, 10, 64)
			if err != nil || strconv.FormatInt(parsed, 10) != *value.IntegerTextValue {
				return nil, billingInvalid()
			}
			quantities[value.FieldKey] = parsed
		}
	}
	result := make([]ConfigurationPriceSelection, 0, len(rows))
	for _, row := range rows {
		quantity, exists := selected[row.FieldKey+"\x00"+row.CandidateValue]
		if row.PricingMode == ConfigurationPricePerUnit || row.PricingMode == ConfigurationPriceBandTotal {
			number, supplied := quantities[row.FieldKey]
			if !supplied {
				continue
			}
			if number < 0 || number > math.MaxInt32 {
				return nil, billingInvalid()
			}
			quantity, exists = int(number), number > 0
		}
		if !validConfigurationPriceMode(row.PricingMode) {
			return nil, billingInvalid()
		}
		if exists {
			result = append(result, ConfigurationPriceSelection{DimensionNo: row.DimensionNo, Quantity: quantity, PricingMode: row.PricingMode})
		}
	}
	return result, nil
}

func validConfigurationPriceMode(mode ConfigurationPriceMode) bool {
	return mode == "" || mode == ConfigurationPriceFixed || mode == ConfigurationPricePerUnit || mode == ConfigurationPriceBandTotal
}

// verifyConfigurationPriceModes loads all requested dimensions in one bounded
// query and rejects a snapshot whose algorithm differs from its frozen owner.
func verifyConfigurationPriceModes(ctx context.Context, database gdb.DB, lines []PurchaseQuoteLineInput) error {
	versions := make([]string, 0, len(lines))
	dimensions := make([]string, 0)
	for _, line := range lines {
		versions = append(versions, line.VersionNo)
		for _, selection := range line.PriceSelections {
			dimensions = append(dimensions, selection.DimensionNo)
		}
	}
	if len(dimensions) == 0 {
		return nil
	}
	var rows []struct {
		VersionNo, DimensionNo string
		PricingMode            ConfigurationPriceMode `orm:"pricing_mode_code"`
	}
	if err := database.GetScan(ctx, &rows, `SELECT version_no,dimension_no,pricing_mode_code FROM plugin_idc_core_catalog_configuration_price_dimension
WHERE tenant_id=0 AND version_no IN(?) AND dimension_no IN(?) AND status_code='active' LIMIT ?`, versions, dimensions, len(dimensions)+1); err != nil {
		return err
	}
	if len(rows) > len(dimensions) {
		return billingInvalid()
	}
	modes := make(map[string]ConfigurationPriceMode, len(rows))
	for _, row := range rows {
		modes[row.VersionNo+"\x00"+row.DimensionNo] = row.PricingMode
	}
	for _, line := range lines {
		for _, selection := range line.PriceSelections {
			mode, exists := modes[line.VersionNo+"\x00"+selection.DimensionNo]
			requested := selection.PricingMode
			if requested == "" {
				requested = ConfigurationPriceFixed
			}
			if !exists || mode != requested {
				return billingInvalid()
			}
		}
	}
	return nil
}
