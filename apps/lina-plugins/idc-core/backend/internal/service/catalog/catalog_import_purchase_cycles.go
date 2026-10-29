// This file enforces source-proven native billing boundaries before checkout.
// The frozen owner read protocol is authoritative; connector names, public
// product identifiers and configurable display fields never grant an exception.
package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/model/do"
)

// validateImportedPurchaseCycles avoids additional reads for the universal
// one-cycle case and batch-checks every larger request against its frozen source.
func (s *publicCatalogReaderImpl) validateImportedPurchaseCycles(ctx context.Context, lines []PurchaseQuoteLine) error {
	versions := make([]string, 0, len(lines))
	seen := make(map[string]struct{})
	for _, line := range lines {
		if line.PurchaseCycles != 1 {
			if _, found := seen[line.VersionNo]; !found {
				versions = append(versions, line.VersionNo)
				seen[line.VersionNo] = struct{}{}
			}
		}
	}
	if len(versions) == 0 {
		return nil
	}
	limits, err := loadImportedPurchaseCycleLimits(ctx, s.database, versions)
	if err != nil {
		return err
	}
	return validateImportedPurchaseCycleLimits(lines, limits)
}

// loadImportedPurchaseCycleLimits reads one bounded immutable source batch.
// Limits survive re-export even when the next supplier is a same-system peer.
func loadImportedPurchaseCycleLimits(ctx context.Context, database gdb.DB, versions []string) (map[string]int, error) {
	var contracts []struct{ VersionNo, Snapshot, SnapshotDigest string }
	if err := database.Model(do.CatalogImportContract{}).Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0}).WhereIn("version_no", versions).Fields("version_no,snapshot,snapshot_digest").Limit(len(versions) + 1).Scan(&contracts); err != nil {
		return nil, unavailablePurchaseQuote(err)
	}
	if len(contracts) > len(versions) {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	limits := make(map[string]int, len(contracts))
	for _, contract := range contracts {
		var snapshot channelcommercecap.ImportSnapshot
		if json.Unmarshal([]byte(contract.Snapshot), &snapshot) != nil || snapshot.Validate() != nil {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		encoded, err := json.Marshal(snapshot)
		if err != nil {
			return nil, unavailablePurchaseQuote(err)
		}
		digest := sha256.Sum256(encoded)
		if hex.EncodeToString(digest[:]) != contract.SnapshotDigest {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		limits[contract.VersionNo] = snapshot.Fulfillment.MaximumPurchaseCycles
	}
	return limits, nil
}

// validateImportedPurchaseCycleLimits applies source ceilings before any cart
// write or immutable checkout quote, while zero keeps existing local behavior.
func validateImportedPurchaseCycleLimits(lines []PurchaseQuoteLine, limits map[string]int) error {
	for _, line := range lines {
		if limits[line.VersionNo] > 0 && (line.PurchaseCycles < 1 || line.PurchaseCycles > limits[line.VersionNo]) {
			return bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
	}
	return nil
}
