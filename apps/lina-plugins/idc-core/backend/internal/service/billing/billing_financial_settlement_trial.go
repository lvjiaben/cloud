// This file validates and consumes one held trial right per frozen generic
// quantity as the sole authority for a zero-value trial financial settlement.
package billing

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/trialqualification"
)

const (
	trialSettlementStatusHeld     = "held"
	trialSettlementStatusConsumed = "consumed"
	trialSettlementMovementHold   = "hold"
	trialSettlementMovementUse    = "consume"
)

type trialSettlementEntitlementRow struct {
	EntitlementNo          string     `orm:"entitlement_no"`
	ScopeNo                string     `orm:"scope_no"`
	CustomerNo             string     `orm:"customer_no"`
	ProductNo              string     `orm:"product_no"`
	VersionNo              string     `orm:"version_no"`
	BillingRuleNo          string     `orm:"billing_rule_no"`
	OrderNo                string     `orm:"order_no"`
	InvoiceNo              string     `orm:"invoice_no"`
	OrderItemNo            string     `orm:"order_item_no"`
	UnitSequence           int        `orm:"unit_sequence"`
	CurrencyCode           string     `orm:"currency_code"`
	TrialLimitQuantity     int        `orm:"trial_limit_quantity"`
	StatusCode             string     `orm:"status_code"`
	HoldExpiresAt          *time.Time `orm:"hold_expires_at"`
	IdentityEvidenceDigest *string    `orm:"identity_evidence_digest"`
	ContentDigest          string     `orm:"content_digest"`
	Version                int64      `orm:"version"`
	OrderChargeType        string     `orm:"order_charge_type"`
	OrderTrialLimit        int        `orm:"order_trial_limit"`
}

type trialSettlementGroup struct {
	scope        entity.TrialQualificationScope
	entitlements []trialSettlementEntitlementRow
	digests      []string
	holdMovement entity.TrialQualificationMovement
}

type trialSettlementScopeUpdate struct {
	scopeNo                       string
	heldBefore, heldAfter         int
	consumedBefore, consumedAfter int
	versionBefore, versionAfter   int64
}

func prepareTrialFinancialSettlement(
	ctx context.Context,
	database gdb.DB,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	items settlementOrderItemSummary,
	identityDigest string,
	now time.Time,
) (preparedFinancialSettlement, error) {
	if !trialFinancialSettlementItemsEligible(items) {
		return preparedFinancialSettlement{}, bizerr.NewCode(CodeSettlementNotEligible)
	}
	basis, err := prepareSyntheticFinancialSettlement(ctx, database, order, invoice, now)
	if err != nil || basis != SettlementBasisZeroAmount {
		return preparedFinancialSettlement{}, bizerr.NewCode(CodeSettlementNotEligible)
	}
	groups, err := loadAndValidateTrialSettlementGroups(
		ctx, database, order, invoice, int(items.TotalQuantity), now,
	)
	if err != nil {
		return preparedFinancialSettlement{}, err
	}
	consumeDigests, entitlementDigests, holdDigests, err := consumeTrialSettlementGroups(
		ctx, database, order, groups, identityDigest, now,
	)
	if err != nil {
		return preparedFinancialSettlement{}, err
	}
	evidenceDigest, err := trialqualification.SettlementEvidenceDigest(
		trialqualification.SettlementEvidenceFact{
			OrderNo: order.OrderNo, InvoiceNo: invoice.InvoiceNo,
			CustomerNo: order.CustomerNo, IdentityEvidenceDigest: identityDigest,
			Entitlements: entitlementDigests, HoldMovements: holdDigests,
			ConsumeMovements: consumeDigests,
		},
	)
	if err != nil {
		return preparedFinancialSettlement{}, err
	}
	prepared := preparedFinancialSettlement{
		settlementNo: deterministicFinancialSettlementNo(order.OrderNo),
		orderNo:      order.OrderNo, invoiceNo: invoice.InvoiceNo,
		customerNo: order.CustomerNo, currencyCode: order.CurrencyCode,
		basis: SettlementBasisTrial, identityEvidenceDigest: identityDigest,
		settledAt: now,
		sources: []preparedFinancialSettlementSource{{
			sourceType:      FinancialSettlementSourceTrial,
			sourceReference: order.OrderNo, evidenceDigest: evidenceDigest,
		}},
	}
	if err = finalizePreparedFinancialSettlement(&prepared); err != nil {
		return preparedFinancialSettlement{}, err
	}
	return prepared, nil
}

func trialFinancialSettlementItemsEligible(items settlementOrderItemSummary) bool {
	return items.TrialCount == items.ItemCount && items.TotalQuantity >= 1 &&
		items.TotalQuantity <= maximumFinancialSettlementSources
}

func loadAndValidateTrialSettlementGroups(
	ctx context.Context,
	database gdb.DB,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	expected int,
	now time.Time,
) ([]trialSettlementGroup, error) {
	scopeNos, err := loadTrialSettlementScopeNos(ctx, database, order.OrderNo, expected)
	if err != nil {
		return nil, err
	}
	scopes, err := lockTrialSettlementScopes(ctx, database, order.CustomerNo, scopeNos)
	if err != nil {
		return nil, err
	}
	entitlements, err := lockTrialSettlementEntitlements(ctx, database, order.OrderNo, expected)
	if err != nil {
		return nil, err
	}
	groups := make(map[string]trialSettlementGroup, len(scopes))
	for _, scope := range scopes {
		groups[scope.ScopeNo] = trialSettlementGroup{scope: scope}
	}
	for _, entitlement := range entitlements {
		group, found := groups[entitlement.ScopeNo]
		if !found || !validHeldTrialSettlementEntitlement(entitlement, order, invoice, now) {
			return nil, bizerr.NewCode(CodeSettlementIntegrity)
		}
		digest, digestErr := trialqualification.EntitlementDigest(
			trialqualification.EntitlementFact{
				EntitlementNo: entitlement.EntitlementNo, ScopeNo: entitlement.ScopeNo,
				CustomerNo: entitlement.CustomerNo, ProductNo: entitlement.ProductNo,
				VersionNo: entitlement.VersionNo, BillingRuleNo: entitlement.BillingRuleNo,
				OrderNo: entitlement.OrderNo, InvoiceNo: entitlement.InvoiceNo,
				OrderItemNo: entitlement.OrderItemNo, UnitSequence: entitlement.UnitSequence,
				CurrencyCode:       entitlement.CurrencyCode,
				TrialLimitQuantity: entitlement.TrialLimitQuantity,
				HoldExpiresAt:      *entitlement.HoldExpiresAt,
			},
		)
		if digestErr != nil || digest != entitlement.ContentDigest {
			return nil, bizerr.NewCode(CodeSettlementIntegrity)
		}
		group.entitlements = append(group.entitlements, entitlement)
		group.digests = append(group.digests, digest)
		groups[entitlement.ScopeNo] = group
	}
	ordered := make([]trialSettlementGroup, 0, len(scopeNos))
	for _, scopeNo := range scopeNos {
		group := groups[scopeNo]
		if len(group.entitlements) < 1 || group.scope.HeldQuantity < len(group.entitlements) {
			return nil, bizerr.NewCode(CodeSettlementIntegrity)
		}
		ordered = append(ordered, group)
	}
	if err = loadAndValidateTrialHoldMovements(ctx, database, order.OrderNo, ordered); err != nil {
		return nil, err
	}
	return ordered, nil
}

func loadTrialSettlementScopeNos(
	ctx context.Context,
	database gdb.DB,
	orderNo string,
	expected int,
) ([]string, error) {
	rows := make([]struct {
		ScopeNo string `orm:"scope_no"`
	}, 0)
	if err := database.GetScan(ctx, &rows, `
SELECT DISTINCT scope_no
FROM plugin_idc_core_trial_entitlement
WHERE tenant_id = 0 AND order_no = ?
ORDER BY scope_no ASC
LIMIT ?`, orderNo, expected+1); err != nil {
		return nil, gerror.Wrap(err, "load trial settlement scope identities")
	}
	if len(rows) == 0 || len(rows) > expected {
		return nil, bizerr.NewCode(CodeSettlementIntegrity)
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.ScopeNo)
	}
	return result, nil
}

func lockTrialSettlementScopes(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	scopeNos []string,
) ([]entity.TrialQualificationScope, error) {
	rows := make([]entity.TrialQualificationScope, 0, len(scopeNos))
	if err := database.Model(dao.TrialQualificationScope.Table()).Ctx(ctx).
		Where(do.TrialQualificationScope{
			TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		}).WhereIn(dao.TrialQualificationScope.Columns().ScopeNo, scopeNos).
		OrderAsc(dao.TrialQualificationScope.Columns().ScopeNo).LockUpdate().Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "lock trial settlement scopes")
	}
	if len(rows) != len(scopeNos) {
		return nil, bizerr.NewCode(CodeSettlementIntegrity)
	}
	for index, row := range rows {
		if row.ScopeNo != scopeNos[index] || row.Version < 1 || row.HeldQuantity < 0 ||
			row.ConsumedQuantity < 0 || row.HeldQuantity+row.ConsumedQuantity > 100 {
			return nil, bizerr.NewCode(CodeSettlementIntegrity)
		}
	}
	return rows, nil
}

func lockTrialSettlementEntitlements(
	ctx context.Context,
	database gdb.DB,
	orderNo string,
	expected int,
) ([]trialSettlementEntitlementRow, error) {
	rows := make([]trialSettlementEntitlementRow, 0, expected)
	if err := database.GetScan(ctx, &rows, `
SELECT e.entitlement_no, e.scope_no, e.customer_no, e.product_no,
       e.version_no, e.billing_rule_no, e.order_no, e.invoice_no,
       e.order_item_no, e.unit_sequence, e.currency_code,
       e.trial_limit_quantity, e.status_code, e.hold_expires_at,
       e.identity_evidence_digest, e.content_digest, e.version,
       i.charge_type_code AS order_charge_type,
       i.trial_limit_quantity AS order_trial_limit
FROM plugin_idc_core_trial_entitlement e
JOIN plugin_idc_core_commerce_order_item i
  ON i.tenant_id = e.tenant_id AND i.order_no = e.order_no
 AND i.order_item_no = e.order_item_no
WHERE e.tenant_id = 0 AND e.order_no = ?
ORDER BY e.scope_no ASC, e.order_item_no ASC, e.unit_sequence ASC
LIMIT ?
FOR UPDATE OF e`, orderNo, expected+1); err != nil {
		return nil, gerror.Wrap(err, "lock trial settlement entitlements")
	}
	if len(rows) != expected {
		return nil, bizerr.NewCode(CodeSettlementIntegrity)
	}
	return rows, nil
}

func validHeldTrialSettlementEntitlement(
	row trialSettlementEntitlementRow,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	now time.Time,
) bool {
	return row.CustomerNo == order.CustomerNo && row.OrderNo == order.OrderNo &&
		row.InvoiceNo == invoice.InvoiceNo && row.CurrencyCode == order.CurrencyCode &&
		row.ProductNo != "" && row.VersionNo != "" && row.BillingRuleNo != "" &&
		row.OrderItemNo != "" && row.UnitSequence >= 1 && row.UnitSequence <= 1000 &&
		row.TrialLimitQuantity >= 1 && row.TrialLimitQuantity <= 100 &&
		row.OrderChargeType == "trial" && row.OrderTrialLimit == row.TrialLimitQuantity &&
		row.StatusCode == trialSettlementStatusHeld &&
		row.HoldExpiresAt != nil && row.HoldExpiresAt.After(now) &&
		row.IdentityEvidenceDigest == nil && row.Version == 1 &&
		canonicalDigestPattern.MatchString(row.ContentDigest)
}

func loadAndValidateTrialHoldMovements(
	ctx context.Context,
	database gdb.DB,
	orderNo string,
	groups []trialSettlementGroup,
) error {
	scopeNos := make([]string, 0, len(groups))
	for _, group := range groups {
		scopeNos = append(scopeNos, group.scope.ScopeNo)
	}
	rows := make([]entity.TrialQualificationMovement, 0, len(groups))
	if err := database.Model(dao.TrialQualificationMovement.Table()).Ctx(ctx).
		Where(do.TrialQualificationMovement{
			TenantId: idccap.DefaultTenantID, OrderNo: orderNo,
			MovementTypeCode: trialSettlementMovementHold,
		}).WhereIn(dao.TrialQualificationMovement.Columns().ScopeNo, scopeNos).
		OrderAsc(dao.TrialQualificationMovement.Columns().ScopeNo).
		Limit(len(groups) + 1).Scan(&rows); err != nil {
		return gerror.Wrap(err, "load trial hold movements")
	}
	if len(rows) != len(groups) {
		return bizerr.NewCode(CodeSettlementIntegrity)
	}
	for index := range groups {
		row, group := rows[index], &groups[index]
		if row.ScopeNo != group.scope.ScopeNo || row.CustomerNo != group.scope.CustomerNo ||
			row.ProductNo != group.scope.ProductNo || row.Quantity != len(group.entitlements) ||
			row.MovementTypeCode != trialSettlementMovementHold || row.CreatedAt == nil {
			return bizerr.NewCode(CodeSettlementIntegrity)
		}
		evidence, err := trialqualification.MovementEvidenceDigest(
			trialqualification.MovementEvidenceFact{
				MovementType: trialSettlementMovementHold, OrderNo: orderNo,
				ScopeNo: row.ScopeNo, Entitlements: group.digests,
			},
		)
		if err != nil || evidence != row.EvidenceDigest {
			return bizerr.NewCode(CodeSettlementIntegrity)
		}
		digest, err := trialqualification.MovementDigest(trialqualification.MovementFact{
			MovementNo: row.MovementNo, ScopeNo: row.ScopeNo, CustomerNo: row.CustomerNo,
			ProductNo: row.ProductNo, OrderNo: row.OrderNo, MovementType: row.MovementTypeCode,
			Quantity: row.Quantity, HeldBefore: row.HeldBefore, HeldAfter: row.HeldAfter,
			ConsumedBefore: row.ConsumedBefore, ConsumedAfter: row.ConsumedAfter,
			VersionBefore: row.ScopeVersionBefore, VersionAfter: row.ScopeVersionAfter,
			EvidenceDigest: row.EvidenceDigest, CreatedAt: *row.CreatedAt,
		})
		if err != nil || digest != row.ContentDigest {
			return bizerr.NewCode(CodeSettlementIntegrity)
		}
		group.holdMovement = row
	}
	return nil
}

func consumeTrialSettlementGroups(
	ctx context.Context,
	database gdb.DB,
	order *entity.CommerceOrder,
	groups []trialSettlementGroup,
	identityDigest string,
	now time.Time,
) ([]string, []string, []string, error) {
	updates := make([]trialSettlementScopeUpdate, 0, len(groups))
	movements := make([]do.TrialQualificationMovement, 0, len(groups))
	consumeDigests := make([]string, 0, len(groups))
	entitlementDigests := make([]string, 0)
	holdDigests := make([]string, 0, len(groups))
	for _, group := range groups {
		quantity := len(group.entitlements)
		update := trialSettlementScopeUpdate{
			scopeNo: group.scope.ScopeNo, heldBefore: group.scope.HeldQuantity,
			heldAfter:      group.scope.HeldQuantity - quantity,
			consumedBefore: group.scope.ConsumedQuantity,
			consumedAfter:  group.scope.ConsumedQuantity + quantity,
			versionBefore:  group.scope.Version, versionAfter: group.scope.Version + 1,
		}
		if update.heldAfter < 0 || update.consumedAfter > 100 {
			return nil, nil, nil, bizerr.NewCode(CodeSettlementIntegrity)
		}
		evidence, err := trialqualification.MovementEvidenceDigest(
			trialqualification.MovementEvidenceFact{
				MovementType: trialSettlementMovementUse, OrderNo: order.OrderNo,
				ScopeNo: group.scope.ScopeNo, Entitlements: group.digests,
				IdentityEvidenceDigest: identityDigest,
			},
		)
		if err != nil {
			return nil, nil, nil, err
		}
		movementNo := deterministicTrialConsumeMovementNo(order.OrderNo, group.scope.ScopeNo)
		movementDigest, err := trialqualification.MovementDigest(trialqualification.MovementFact{
			MovementNo: movementNo, ScopeNo: group.scope.ScopeNo,
			CustomerNo: group.scope.CustomerNo, ProductNo: group.scope.ProductNo,
			OrderNo: order.OrderNo, MovementType: trialSettlementMovementUse,
			Quantity: quantity, HeldBefore: update.heldBefore, HeldAfter: update.heldAfter,
			ConsumedBefore: update.consumedBefore, ConsumedAfter: update.consumedAfter,
			VersionBefore: update.versionBefore, VersionAfter: update.versionAfter,
			EvidenceDigest: evidence, CreatedAt: now,
		})
		if err != nil {
			return nil, nil, nil, err
		}
		movements = append(movements, do.TrialQualificationMovement{
			TenantId: idccap.DefaultTenantID, MovementNo: movementNo,
			ScopeNo: group.scope.ScopeNo, CustomerNo: group.scope.CustomerNo,
			ProductNo: group.scope.ProductNo, OrderNo: order.OrderNo,
			MovementTypeCode: trialSettlementMovementUse, Quantity: quantity,
			HeldBefore: update.heldBefore, HeldAfter: update.heldAfter,
			ConsumedBefore: update.consumedBefore, ConsumedAfter: update.consumedAfter,
			ScopeVersionBefore: update.versionBefore, ScopeVersionAfter: update.versionAfter,
			EvidenceDigest: evidence, ContentDigest: movementDigest, CreatedAt: &now,
		})
		updates = append(updates, update)
		consumeDigests = append(consumeDigests, movementDigest)
		entitlementDigests = append(entitlementDigests, group.digests...)
		holdDigests = append(holdDigests, group.holdMovement.ContentDigest)
	}
	identityDigestCopy := identityDigest
	result, err := database.Model(dao.TrialEntitlement.Table()).Ctx(ctx).
		Where(do.TrialEntitlement{
			TenantId: idccap.DefaultTenantID, OrderNo: order.OrderNo,
			StatusCode: trialSettlementStatusHeld,
		}).Data(do.TrialEntitlement{
		StatusCode: trialSettlementStatusConsumed, ConsumedAt: &now,
		IdentityEvidenceDigest: &identityDigestCopy,
		Version:                gdb.Raw(dao.TrialEntitlement.Columns().Version + " + 1"), UpdatedAt: &now,
	}).Update()
	if err != nil {
		return nil, nil, nil, gerror.Wrap(err, "consume trial settlement entitlements")
	}
	if err = requireTrialSettlementAffected(result, len(entitlementDigests)); err != nil {
		return nil, nil, nil, err
	}
	if err = updateTrialSettlementScopes(ctx, database, updates, now); err != nil {
		return nil, nil, nil, err
	}
	if _, err = database.Model(dao.TrialQualificationMovement.Table()).Ctx(ctx).
		Data(movements).Insert(); err != nil {
		return nil, nil, nil, gerror.Wrap(err, "insert trial consumption movements")
	}
	return consumeDigests, entitlementDigests, holdDigests, nil
}

func updateTrialSettlementScopes(
	ctx context.Context,
	database gdb.DB,
	updates []trialSettlementScopeUpdate,
	now time.Time,
) error {
	values := make([]string, 0, len(updates))
	arguments := make([]any, 0, 1+len(updates)*7)
	arguments = append(arguments, now)
	for _, update := range updates {
		values = append(values, "(?::varchar, ?::int, ?::int, ?::bigint, ?::int, ?::int, ?::bigint)")
		arguments = append(arguments, update.scopeNo, update.heldAfter, update.consumedAfter,
			update.versionAfter, update.heldBefore, update.consumedBefore, update.versionBefore)
	}
	result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_trial_qualification_scope AS s
SET held_quantity = v.held_after, consumed_quantity = v.consumed_after,
    version = v.version_after, updated_at = ?
FROM (VALUES `+strings.Join(values, ",")+`) AS v(
    scope_no, held_after, consumed_after, version_after,
    held_before, consumed_before, version_before
)
WHERE s.tenant_id = 0 AND s.scope_no = v.scope_no
  AND s.held_quantity = v.held_before
  AND s.consumed_quantity = v.consumed_before
  AND s.version = v.version_before`, arguments...)
	if err != nil {
		return gerror.Wrap(err, "update trial settlement scopes")
	}
	return requireTrialSettlementAffected(result, len(updates))
}

func requireTrialSettlementAffected(
	result interface{ RowsAffected() (int64, error) },
	expected int,
) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read trial settlement mutation result")
	}
	if affected != int64(expected) {
		return bizerr.NewCode(CodeSettlementIntegrity)
	}
	return nil
}

func deterministicTrialConsumeMovementNo(orderNo, scopeNo string) string {
	digest := sha256Digest("trial-consume-movement\x00" + orderNo + "\x00" + scopeNo)
	return "trm_" + hex.EncodeToString(digest[:16])
}
