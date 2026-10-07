// This file owns serialized customer-and-product trial holds and their
// append-only qualification projection. It performs local database work only.

package commerce

import (
	"context"
	"encoding/json"
	"sort"
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
	trialEntitlementStatusHeld     = "held"
	trialEntitlementStatusConsumed = "consumed"
	trialEntitlementStatusReleased = "released"
	trialEntitlementStatusExpired  = "expired"

	trialMovementHold    = "hold"
	trialMovementConsume = "consume"
	trialMovementRelease = "release"
	trialMovementExpire  = "expire"
)

type checkoutTrialUnitRow struct {
	ProductNo          string `orm:"product_no"`
	VersionNo          string `orm:"version_no"`
	BillingRuleNo      string `orm:"billing_rule_no"`
	OrderItemNo        string `orm:"order_item_no"`
	UnitSequence       int    `orm:"unit_sequence"`
	TrialLimitQuantity int    `orm:"trial_limit_quantity"`
	PriorTrialUses     int    `orm:"prior_trial_uses"`
}

type checkoutTrialGroup struct {
	limitQuantity int
	priorUses     int
	units         []checkoutTrialUnitRow
}

type trialEntitlementInsert struct {
	EntitlementNo      string    `json:"entitlement_no"`
	ScopeNo            string    `json:"scope_no"`
	CustomerNo         string    `json:"customer_no"`
	ProductNo          string    `json:"product_no"`
	VersionNo          string    `json:"version_no"`
	BillingRuleNo      string    `json:"billing_rule_no"`
	OrderNo            string    `json:"order_no"`
	InvoiceNo          string    `json:"invoice_no"`
	OrderItemNo        string    `json:"order_item_no"`
	UnitSequence       int       `json:"unit_sequence"`
	CurrencyCode       string    `json:"currency_code"`
	TrialLimitQuantity int       `json:"trial_limit_quantity"`
	HoldExpiresAt      time.Time `json:"hold_expires_at"`
	ContentDigest      string    `json:"content_digest"`
	CreatedAt          time.Time `json:"created_at"`
}

type trialScopeUpdate struct {
	scopeNo                       string
	heldBefore, heldAfter         int
	consumedBefore, consumedAfter int
	versionBefore, versionAfter   int64
}

func (s *serviceImpl) reserveCheckoutTrialQualifications(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
	currencyCode string,
	rows checkoutRows,
	quoteItems []entity.CommerceQuoteItemSnapshot,
	quantities map[string]int,
	now time.Time,
) error {
	trialItems, totalUnits, err := checkoutTrialItemSummary(quoteItems, quantities)
	if err != nil || trialItems == 0 {
		return err
	}
	units, err := loadCheckoutTrialUnits(ctx, tx, rows.orderNo, totalUnits)
	if err != nil {
		return err
	}
	groups, productNos, err := groupCheckoutTrialUnits(units)
	if err != nil {
		return err
	}
	candidateScopes := make(map[string]string, len(productNos))
	for _, productNo := range productNos {
		scopeNo, numberErr := newOpaqueNumber(s.random, "TRS_")
		if numberErr != nil {
			return numberErr
		}
		candidateScopes[productNo] = scopeNo
	}
	if err = insertMissingTrialScopes(ctx, tx, customerNo, productNos, candidateScopes, now); err != nil {
		return err
	}
	locked, err := lockCheckoutTrialScopes(ctx, tx, customerNo, productNos)
	if err != nil {
		return err
	}
	return s.persistCheckoutTrialHold(
		ctx, tx, customerNo, currencyCode, rows, groups, locked, now,
	)
}

func checkoutTrialItemSummary(items []entity.CommerceQuoteItemSnapshot, quantities map[string]int) (int, int, error) {
	trialItems, totalUnits := 0, 0
	for _, item := range items {
		if item.ChargeTypeCode != "trial" {
			continue
		}
		trialItems++
		quantity := quantities[item.QuoteItemNo]
		if quantity < 1 || totalUnits > maximumCheckoutFulfillmentUnits-quantity {
			return 0, 0, bizerr.NewCode(CodeQuoteNotAvailable)
		}
		totalUnits += quantity
	}
	if trialItems != 0 && trialItems != len(items) {
		return 0, 0, bizerr.NewCode(CodeQuoteNotAvailable)
	}
	return trialItems, totalUnits, nil
}

func loadCheckoutTrialUnits(
	ctx context.Context,
	tx gdb.TX,
	orderNo string,
	expected int,
) ([]checkoutTrialUnitRow, error) {
	result := make([]checkoutTrialUnitRow, 0, expected)
	if err := tx.Ctx(ctx).GetScan(&result, `
SELECT i.product_no, i.version_no, i.billing_rule_no, i.order_item_no,
       unit.unit_sequence, i.trial_limit_quantity, i.prior_trial_uses
FROM plugin_idc_core_commerce_order_item i
JOIN plugin_idc_core_commerce_order_item_quantity_snapshot quantity
  ON quantity.tenant_id=i.tenant_id AND quantity.order_item_no=i.order_item_no
CROSS JOIN LATERAL generate_series(1, quantity.quantity) AS unit(unit_sequence)
WHERE i.tenant_id = 0 AND i.order_no = ? AND i.charge_type_code = 'trial'
ORDER BY i.product_no ASC, i.line_number ASC, unit.unit_sequence ASC
LIMIT ?`, orderNo, expected+1); err != nil {
		return nil, gerror.Wrap(err, "load checkout trial unit identities")
	}
	if len(result) != expected {
		return nil, gerror.New("checkout trial unit identities are inconsistent")
	}
	return result, nil
}

func groupCheckoutTrialUnits(
	rows []checkoutTrialUnitRow,
) (map[string]checkoutTrialGroup, []string, error) {
	groups := make(map[string]checkoutTrialGroup)
	for _, row := range rows {
		if row.ProductNo == "" || row.VersionNo == "" || row.BillingRuleNo == "" ||
			row.OrderItemNo == "" || row.UnitSequence < 1 || row.UnitSequence > 1000 || row.TrialLimitQuantity < 1 ||
			row.TrialLimitQuantity > 100 || row.PriorTrialUses < 0 ||
			row.PriorTrialUses >= row.TrialLimitQuantity {
			return nil, nil, gerror.New("checkout trial unit fact is invalid")
		}
		group, found := groups[row.ProductNo]
		if found && (group.limitQuantity != row.TrialLimitQuantity || group.priorUses != row.PriorTrialUses) {
			return nil, nil, gerror.New("checkout trial product facts are inconsistent")
		}
		if !found {
			group.limitQuantity = row.TrialLimitQuantity
			group.priorUses = row.PriorTrialUses
		}
		group.units = append(group.units, row)
		groups[row.ProductNo] = group
	}
	productNos := make([]string, 0, len(groups))
	for productNo := range groups {
		productNos = append(productNos, productNo)
	}
	sort.Strings(productNos)
	return groups, productNos, nil
}

func insertMissingTrialScopes(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
	productNos []string,
	candidateScopes map[string]string,
	now time.Time,
) error {
	values := make([]string, 0, len(productNos))
	arguments := make([]any, 0, len(productNos)*5)
	for _, productNo := range productNos {
		values = append(values, "(0, ?, ?, ?, 0, 0, 1, ?, ?)")
		arguments = append(arguments, candidateScopes[productNo], customerNo, productNo, now, now)
	}
	_, err := tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_core_trial_qualification_scope
    (tenant_id, scope_no, customer_no, product_no, held_quantity,
     consumed_quantity, version, created_at, updated_at)
VALUES `+strings.Join(values, ",")+`
ON CONFLICT (tenant_id, customer_no, product_no) DO NOTHING`, arguments...)
	if err != nil {
		return gerror.Wrap(err, "insert missing checkout trial scopes")
	}
	return nil
}

func lockCheckoutTrialScopes(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
	productNos []string,
) (map[string]entity.TrialQualificationScope, error) {
	rows := make([]entity.TrialQualificationScope, 0, len(productNos))
	arguments := make([]any, 0, len(productNos)+1)
	arguments = append(arguments, customerNo)
	for _, productNo := range productNos {
		arguments = append(arguments, productNo)
	}
	if err := tx.Ctx(ctx).GetScan(&rows, `
SELECT id, tenant_id, scope_no, customer_no, product_no,
       held_quantity, consumed_quantity, version, created_at, updated_at
FROM plugin_idc_core_trial_qualification_scope
WHERE tenant_id = 0 AND customer_no = ?
  AND product_no IN (`+commerceSQLPlaceholders(len(productNos))+`)
ORDER BY product_no ASC
FOR UPDATE`, arguments...); err != nil {
		return nil, gerror.Wrap(err, "lock checkout trial qualification scopes")
	}
	if len(rows) != len(productNos) {
		return nil, gerror.New("checkout trial qualification scope set is incomplete")
	}
	result := make(map[string]entity.TrialQualificationScope, len(rows))
	for _, row := range rows {
		if row.CustomerNo != customerNo || row.HeldQuantity < 0 || row.ConsumedQuantity < 0 || row.Version < 1 {
			return nil, gerror.New("checkout trial qualification scope is invalid")
		}
		result[row.ProductNo] = row
	}
	return result, nil
}

func (s *serviceImpl) persistCheckoutTrialHold(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
	currencyCode string,
	checkout checkoutRows,
	groups map[string]checkoutTrialGroup,
	locked map[string]entity.TrialQualificationScope,
	now time.Time,
) error {
	productNos := make([]string, 0, len(groups))
	for productNo := range groups {
		productNos = append(productNos, productNo)
	}
	sort.Strings(productNos)
	entitlements := make([]trialEntitlementInsert, 0)
	movements := make([]do.TrialQualificationMovement, 0, len(groups))
	updates := make([]trialScopeUpdate, 0, len(groups))
	for _, productNo := range productNos {
		group, scope := groups[productNo], locked[productNo]
		quantity := len(group.units)
		if quantity < 1 || scope.HeldQuantity+scope.ConsumedQuantity > group.limitQuantity-quantity {
			return bizerr.NewCode(CodeQuoteNotAvailable)
		}
		digests := make([]string, 0, quantity)
		for _, unit := range group.units {
			entitlementNo, err := newOpaqueNumber(s.random, "TRE_")
			if err != nil {
				return err
			}
			contentDigest, err := trialqualification.EntitlementDigest(
				trialqualification.EntitlementFact{
					EntitlementNo: entitlementNo, ScopeNo: scope.ScopeNo,
					CustomerNo: customerNo, ProductNo: productNo,
					VersionNo: unit.VersionNo, BillingRuleNo: unit.BillingRuleNo,
					OrderNo: checkout.orderNo, InvoiceNo: checkout.invoiceNo,
					OrderItemNo: unit.OrderItemNo, UnitSequence: unit.UnitSequence,
					CurrencyCode: currencyCode, TrialLimitQuantity: group.limitQuantity,
					HoldExpiresAt: checkout.paymentDueAt,
				})
			if err != nil {
				return err
			}
			digests = append(digests, contentDigest)
			entitlements = append(entitlements, trialEntitlementInsert{
				EntitlementNo: entitlementNo, ScopeNo: scope.ScopeNo,
				CustomerNo: customerNo, ProductNo: productNo,
				VersionNo: unit.VersionNo, BillingRuleNo: unit.BillingRuleNo,
				OrderNo: checkout.orderNo, InvoiceNo: checkout.invoiceNo,
				OrderItemNo: unit.OrderItemNo, UnitSequence: unit.UnitSequence,
				CurrencyCode: currencyCode, TrialLimitQuantity: group.limitQuantity,
				HoldExpiresAt: checkout.paymentDueAt, ContentDigest: contentDigest, CreatedAt: now,
			})
		}
		evidenceDigest, err := trialqualification.MovementEvidenceDigest(
			trialqualification.MovementEvidenceFact{
				MovementType: trialMovementHold, OrderNo: checkout.orderNo,
				ScopeNo: scope.ScopeNo, Entitlements: digests,
			},
		)
		if err != nil {
			return err
		}
		movementNo, err := newOpaqueNumber(s.random, "TRM_")
		if err != nil {
			return err
		}
		update := trialScopeUpdate{
			scopeNo: scope.ScopeNo, heldBefore: scope.HeldQuantity,
			heldAfter:      scope.HeldQuantity + quantity,
			consumedBefore: scope.ConsumedQuantity, consumedAfter: scope.ConsumedQuantity,
			versionBefore: scope.Version, versionAfter: scope.Version + 1,
		}
		movementDigest, err := trialMovementContentDigest(
			movementNo, scope, checkout.orderNo, trialMovementHold, quantity,
			update, evidenceDigest, now,
		)
		if err != nil {
			return err
		}
		movements = append(movements, do.TrialQualificationMovement{
			TenantId: idccap.DefaultTenantID, MovementNo: movementNo, ScopeNo: scope.ScopeNo,
			CustomerNo: customerNo, ProductNo: productNo, OrderNo: checkout.orderNo,
			MovementTypeCode: trialMovementHold, Quantity: quantity,
			HeldBefore: update.heldBefore, HeldAfter: update.heldAfter,
			ConsumedBefore: update.consumedBefore, ConsumedAfter: update.consumedAfter,
			ScopeVersionBefore: update.versionBefore, ScopeVersionAfter: update.versionAfter,
			EvidenceDigest: evidenceDigest, ContentDigest: movementDigest, CreatedAt: &now,
		})
		updates = append(updates, update)
	}
	rawEntitlements, err := json.Marshal(entitlements)
	if err != nil {
		return gerror.Wrap(err, "encode checkout trial entitlements")
	}
	if _, err = tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_core_trial_entitlement(
tenant_id,entitlement_no,scope_no,customer_no,product_no,version_no,billing_rule_no,
order_no,invoice_no,order_item_no,unit_sequence,currency_code,trial_limit_quantity,
status_code,hold_expires_at,content_digest,version,created_at,updated_at)
SELECT 0,entitlement_no,scope_no,customer_no,product_no,version_no,billing_rule_no,
order_no,invoice_no,order_item_no,unit_sequence,currency_code,trial_limit_quantity,
'held',hold_expires_at,content_digest,1,created_at,created_at
FROM jsonb_to_recordset(?::jsonb) AS x(entitlement_no text,scope_no text,customer_no text,
product_no text,version_no text,billing_rule_no text,order_no text,invoice_no text,
order_item_no text,unit_sequence int,currency_code text,trial_limit_quantity int,
hold_expires_at timestamptz,content_digest text,created_at timestamptz)`, rawEntitlements); err != nil {
		return gerror.Wrap(err, "insert checkout trial entitlements")
	}
	if err := updateTrialScopes(ctx, tx, updates, now); err != nil {
		return err
	}
	if _, err := tx.Model(dao.TrialQualificationMovement.Table()).Ctx(ctx).Data(movements).Insert(); err != nil {
		return gerror.Wrap(err, "insert checkout trial qualification movements")
	}
	return nil
}

func updateTrialScopes(ctx context.Context, tx gdb.TX, updates []trialScopeUpdate, now time.Time) error {
	values := make([]string, 0, len(updates))
	arguments := make([]any, 0, 1+len(updates)*7)
	arguments = append(arguments, now)
	for _, update := range updates {
		values = append(values, "(?::varchar, ?::int, ?::int, ?::bigint, ?::int, ?::int, ?::bigint)")
		arguments = append(arguments, update.scopeNo, update.heldAfter, update.consumedAfter,
			update.versionAfter, update.heldBefore, update.consumedBefore, update.versionBefore)
	}
	result, err := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_core_trial_qualification_scope AS s
SET held_quantity = v.held_after,
    consumed_quantity = v.consumed_after,
    version = v.version_after,
    updated_at = ?
FROM (VALUES `+strings.Join(values, ",")+`) AS v(
    scope_no, held_after, consumed_after, version_after,
    held_before, consumed_before, version_before
)
WHERE s.tenant_id = 0 AND s.scope_no = v.scope_no
  AND s.held_quantity = v.held_before
  AND s.consumed_quantity = v.consumed_before
  AND s.version = v.version_before`, arguments...)
	if err != nil {
		return gerror.Wrap(err, "update checkout trial qualification scopes")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read checkout trial qualification scope update")
	}
	if affected != int64(len(updates)) {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func trialMovementContentDigest(
	movementNo string,
	scope entity.TrialQualificationScope,
	orderNo string,
	movementType string,
	quantity int,
	update trialScopeUpdate,
	evidenceDigest string,
	createdAt time.Time,
) (string, error) {
	return trialqualification.MovementDigest(trialqualification.MovementFact{
		MovementNo: movementNo, ScopeNo: scope.ScopeNo, CustomerNo: scope.CustomerNo,
		ProductNo: scope.ProductNo, OrderNo: orderNo, MovementType: movementType,
		Quantity: quantity, HeldBefore: update.heldBefore, HeldAfter: update.heldAfter,
		ConsumedBefore: update.consumedBefore, ConsumedAfter: update.consumedAfter,
		VersionBefore: update.versionBefore, VersionAfter: update.versionAfter,
		EvidenceDigest: evidenceDigest, CreatedAt: createdAt,
	})
}

type releaseTrialEntitlementRow struct {
	EntitlementNo      string     `orm:"entitlement_no"`
	ScopeNo            string     `orm:"scope_no"`
	CustomerNo         string     `orm:"customer_no"`
	ProductNo          string     `orm:"product_no"`
	VersionNo          string     `orm:"version_no"`
	BillingRuleNo      string     `orm:"billing_rule_no"`
	OrderNo            string     `orm:"order_no"`
	InvoiceNo          string     `orm:"invoice_no"`
	OrderItemNo        string     `orm:"order_item_no"`
	UnitSequence       int        `orm:"unit_sequence"`
	CurrencyCode       string     `orm:"currency_code"`
	TrialLimitQuantity int        `orm:"trial_limit_quantity"`
	HoldExpiresAt      *time.Time `orm:"hold_expires_at"`
	ContentDigest      string     `orm:"content_digest"`
}

func releaseOrderTrialQualifications(
	ctx context.Context,
	tx gdb.TX,
	order *entity.CommerceOrder,
	reason OrderCancellationReason,
	now time.Time,
) error {
	productRows := make([]struct {
		ProductNo string `orm:"product_no"`
	}, 0)
	if err := tx.Ctx(ctx).GetScan(&productRows, `
SELECT DISTINCT product_no
FROM plugin_idc_core_trial_entitlement
WHERE tenant_id = 0 AND order_no = ? AND status_code = 'held'
ORDER BY product_no ASC
LIMIT ?`, order.OrderNo, maximumCartItems+1); err != nil {
		return gerror.Wrap(err, "load cancelled order trial products")
	}
	if len(productRows) == 0 {
		return nil
	}
	if len(productRows) > maximumCartItems {
		return bizerr.NewCode(CodeOrderNotCancelable)
	}
	productNos := make([]string, 0, len(productRows))
	for _, row := range productRows {
		productNos = append(productNos, row.ProductNo)
	}
	locked, err := lockCheckoutTrialScopes(ctx, tx, order.CustomerNo, productNos)
	if err != nil {
		return gerror.Wrap(err, "lock cancelled order trial scopes")
	}
	entitlements := make([]releaseTrialEntitlementRow, 0)
	if err = tx.Ctx(ctx).GetScan(&entitlements, `
SELECT entitlement_no, scope_no, customer_no, product_no, version_no,
       billing_rule_no, order_no, invoice_no, order_item_no, unit_sequence,
       currency_code, trial_limit_quantity, hold_expires_at, content_digest
FROM plugin_idc_core_trial_entitlement
WHERE tenant_id = 0 AND order_no = ? AND status_code = 'held'
ORDER BY product_no ASC, order_item_no ASC, unit_sequence ASC
LIMIT ?
FOR UPDATE`, order.OrderNo, maximumCheckoutFulfillmentUnits+1); err != nil {
		return gerror.Wrap(err, "lock cancelled order trial entitlements")
	}
	if len(entitlements) == 0 || len(entitlements) > maximumCheckoutFulfillmentUnits {
		return bizerr.NewCode(CodeOrderNotCancelable)
	}
	type releaseGroup struct {
		scope   entity.TrialQualificationScope
		digests []string
	}
	groups := make(map[string]releaseGroup, len(locked))
	for productNo, scope := range locked {
		groups[productNo] = releaseGroup{scope: scope}
	}
	for _, entitlement := range entitlements {
		group, found := groups[entitlement.ProductNo]
		if !found || entitlement.ScopeNo != group.scope.ScopeNo ||
			entitlement.CustomerNo != order.CustomerNo || entitlement.OrderNo != order.OrderNo ||
			entitlement.HoldExpiresAt == nil || entitlement.TrialLimitQuantity < 1 {
			return bizerr.NewCode(CodeOrderNotCancelable)
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
			return bizerr.NewCode(CodeOrderNotCancelable)
		}
		group.digests = append(group.digests, digest)
		groups[entitlement.ProductNo] = group
	}
	movementType, entitlementStatus := trialMovementRelease, trialEntitlementStatusReleased
	if reason == OrderCancellationReasonPaymentExpired {
		movementType, entitlementStatus = trialMovementExpire, trialEntitlementStatusExpired
	}
	updates := make([]trialScopeUpdate, 0, len(groups))
	movements := make([]do.TrialQualificationMovement, 0, len(groups))
	for _, productNo := range productNos {
		group := groups[productNo]
		quantity := len(group.digests)
		if quantity < 1 || group.scope.HeldQuantity < quantity {
			return bizerr.NewCode(CodeOrderNotCancelable)
		}
		update := trialScopeUpdate{
			scopeNo: group.scope.ScopeNo, heldBefore: group.scope.HeldQuantity,
			heldAfter:      group.scope.HeldQuantity - quantity,
			consumedBefore: group.scope.ConsumedQuantity,
			consumedAfter:  group.scope.ConsumedQuantity,
			versionBefore:  group.scope.Version, versionAfter: group.scope.Version + 1,
		}
		evidenceDigest, digestErr := trialqualification.MovementEvidenceDigest(
			trialqualification.MovementEvidenceFact{
				MovementType: movementType, OrderNo: order.OrderNo,
				ScopeNo: group.scope.ScopeNo, Entitlements: group.digests,
			},
		)
		if digestErr != nil {
			return digestErr
		}
		movementKey, digestErr := quoteContentDigest(struct {
			OrderNo, ScopeNo, MovementType string
		}{order.OrderNo, group.scope.ScopeNo, movementType})
		if digestErr != nil {
			return digestErr
		}
		movementNo := "trm_" + movementKey[:32]
		movementDigest, digestErr := trialMovementContentDigest(
			movementNo, group.scope, order.OrderNo, movementType, quantity,
			update, evidenceDigest, now,
		)
		if digestErr != nil {
			return digestErr
		}
		movements = append(movements, do.TrialQualificationMovement{
			TenantId: idccap.DefaultTenantID, MovementNo: movementNo,
			ScopeNo: group.scope.ScopeNo, CustomerNo: order.CustomerNo,
			ProductNo: productNo, OrderNo: order.OrderNo,
			MovementTypeCode: movementType, Quantity: quantity,
			HeldBefore: update.heldBefore, HeldAfter: update.heldAfter,
			ConsumedBefore: update.consumedBefore, ConsumedAfter: update.consumedAfter,
			ScopeVersionBefore: update.versionBefore, ScopeVersionAfter: update.versionAfter,
			EvidenceDigest: evidenceDigest, ContentDigest: movementDigest, CreatedAt: &now,
		})
		updates = append(updates, update)
	}
	result, err := tx.Model(dao.TrialEntitlement.Table()).Ctx(ctx).
		Where(do.TrialEntitlement{
			TenantId: idccap.DefaultTenantID, OrderNo: order.OrderNo,
			StatusCode: trialEntitlementStatusHeld,
		}).Data(do.TrialEntitlement{
		StatusCode: entitlementStatus, ReleasedAt: &now,
		Version: gdb.Raw(dao.TrialEntitlement.Columns().Version + " + 1"), UpdatedAt: &now,
	}).Update()
	if err != nil {
		return gerror.Wrap(err, "release cancelled order trial entitlements")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != int64(len(entitlements)) {
		return bizerr.NewCode(CodeOrderNotCancelable)
	}
	if err = updateTrialScopes(ctx, tx, updates, now); err != nil {
		return err
	}
	if _, err = tx.Model(dao.TrialQualificationMovement.Table()).Ctx(ctx).
		Data(movements).Insert(); err != nil {
		return gerror.Wrap(err, "insert cancelled order trial movements")
	}
	return nil
}

func commerceSQLPlaceholders(count int) string {
	if count < 1 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}
