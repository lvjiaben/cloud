package productfulfillment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
)

const maximumEntitlementCreatedBatch = 64

type EntitlementCreatedBatch struct{ Selected, Delivered, Retried, ManualAttention int }

type EntitlementCreatedWorker interface {
	Run(context.Context, int) (*EntitlementCreatedBatch, error)
}

type entitlementCreatedWorker struct {
	database gdb.DB
	router   entitlementcreatedcap.Router
}

type entitlementCreatedClaim struct {
	ID           int64 `orm:"id"`
	Event        entitlementcreatedcap.Event
	LeaseToken   string `orm:"lease_token"`
	AttemptCount int    `orm:"attempt_count"`
}

func NewEntitlementCreatedWorker(database gdb.DB, router entitlementcreatedcap.Router) (EntitlementCreatedWorker, error) {
	if database == nil || router == nil {
		return nil, errors.New("entitlement-created worker dependencies are unavailable")
	}
	return &entitlementCreatedWorker{database: database, router: router}, nil
}

func (worker *entitlementCreatedWorker) Run(ctx context.Context, limit int) (*EntitlementCreatedBatch, error) {
	if worker == nil || worker.database == nil || worker.router == nil || ctx == nil || ctx.Err() != nil || limit < 1 || limit > maximumEntitlementCreatedBatch {
		return nil, errors.New("entitlement-created worker input is invalid")
	}
	report := &EntitlementCreatedBatch{}
	for report.Selected < limit {
		claim, err := worker.claim(ctx)
		if err != nil {
			return report, err
		}
		if claim == nil {
			break
		}
		report.Selected++
		sink, available := worker.router.Resolve(claim.Event.OwnerID)
		if !available {
			if err = worker.manualAttention(ctx, claim, "entitlement_owner_unavailable"); err != nil {
				return report, err
			}
			report.ManualAttention++
			continue
		}
		receipt, deliveryErr := sink.Deliver(ctx, claim.Event)
		if deliveryErr == nil && receipt != nil && entitlementcreatedcap.ValidateReceipt(claim.Event, *receipt) {
			if err = worker.finish(ctx, claim, receipt); err != nil {
				return report, err
			}
			report.Delivered++
			continue
		}
		manual, retryErr := worker.retry(ctx, claim)
		if retryErr != nil {
			return report, retryErr
		}
		if manual {
			report.ManualAttention++
		} else {
			report.Retried++
		}
	}
	return report, nil
}

func (worker *entitlementCreatedWorker) claim(ctx context.Context) (*entitlementCreatedClaim, error) {
	var claimed *entitlementCreatedClaim
	err := worker.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		// A worker may stop after its final delivery attempt but before recording the
		// result. Close that expired lease for manual handling before claiming more
		// work; otherwise incrementing attempt_count would violate the bounded-retry
		// database constraint.
		if _, err := worker.database.Exec(txCtx, `UPDATE plugin_idc_core_entitlement_created_outbox SET status_code='manual_attention',lease_token='',lease_expires_at=NULL,last_error_code='owner_delivery_result_unknown',updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND status_code='delivering' AND lease_expires_at<=CURRENT_TIMESTAMP AND attempt_count>=10`); err != nil {
			return err
		}
		var row *struct {
			ID                                                                           int64 `orm:"id"`
			EventNo, EntitlementNo, IntentNo, CustomerNo, ProductNo, VersionNo           string
			ProductNameZhCN, ProductNameEnUS                                             string
			ConfigurationSnapshotNo, InterfaceInstanceNo, PluginID, PluginReleaseVersion string
			ContractVersion, DeclarationDigest, OwnerID                                  string
			AttemptCount                                                                 int
		}
		if err := worker.database.GetScan(txCtx, &row, `SELECT id,event_no,entitlement_no,intent_no,customer_no,product_no,version_no,product_name_zh_cn,product_name_en_us,configuration_snapshot_no,interface_instance_no,plugin_id,plugin_release_version,contract_version,declaration_digest,owner_id,attempt_count FROM plugin_idc_core_entitlement_created_outbox WHERE tenant_id=0 AND attempt_count<10 AND ((status_code IN ('pending','retry_wait') AND next_attempt_at<=CURRENT_TIMESTAMP) OR (status_code='delivering' AND lease_expires_at<=CURRENT_TIMESTAMP)) ORDER BY next_attempt_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`); err != nil || row == nil {
			return err
		}
		lease, err := entitlementCreatedLease()
		if err != nil {
			return err
		}
		result, err := worker.database.Exec(txCtx, `UPDATE plugin_idc_core_entitlement_created_outbox SET status_code='delivering',attempt_count=attempt_count+1,lease_token=?,lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '60 seconds',updated_at=CURRENT_TIMESTAMP WHERE id=? AND attempt_count=?`, lease, row.ID, row.AttemptCount)
		if err != nil || affected(result) != 1 {
			return errors.New("entitlement-created outbox claim conflict")
		}
		claimed = &entitlementCreatedClaim{ID: row.ID, LeaseToken: lease, AttemptCount: row.AttemptCount + 1, Event: entitlementcreatedcap.Event{Version: entitlementcreatedcap.ContractVersion, EventNo: row.EventNo, EntitlementNo: row.EntitlementNo, IntentNo: row.IntentNo, CustomerNo: row.CustomerNo, ProductNo: row.ProductNo, ProductVersionNo: row.VersionNo, ProductNameZhCN: row.ProductNameZhCN, ProductNameEnUS: row.ProductNameEnUS, ConfigurationSnapshotNo: row.ConfigurationSnapshotNo, InterfaceInstanceNo: row.InterfaceInstanceNo, PluginID: row.PluginID, PluginReleaseVersion: row.PluginReleaseVersion, ContractVersion: row.ContractVersion, DeclarationDigest: row.DeclarationDigest, OwnerID: row.OwnerID}}
		if !entitlementcreatedcap.ValidateEvent(claimed.Event) {
			return errors.New("entitlement-created outbox evidence is invalid")
		}
		return nil
	})
	return claimed, err
}

func (worker *entitlementCreatedWorker) manualAttention(ctx context.Context, claim *entitlementCreatedClaim, code string) error {
	result, err := worker.database.Exec(ctx, `UPDATE plugin_idc_core_entitlement_created_outbox SET status_code='manual_attention',lease_token='',lease_expires_at=NULL,last_error_code=?,updated_at=CURRENT_TIMESTAMP WHERE id=? AND status_code='delivering' AND lease_token=?`, code, claim.ID, claim.LeaseToken)
	if err != nil || affected(result) != 1 {
		return errors.New("entitlement-created outbox manual-attention conflict")
	}
	return nil
}

func (worker *entitlementCreatedWorker) finish(ctx context.Context, claim *entitlementCreatedClaim, receipt *entitlementcreatedcap.Receipt) error {
	result, err := worker.database.Exec(ctx, `UPDATE plugin_idc_core_entitlement_created_outbox SET status_code='delivered',owner_reference=?,receipt_evidence_digest=?,lease_token='',lease_expires_at=NULL,last_error_code='',updated_at=CURRENT_TIMESTAMP WHERE id=? AND status_code='delivering' AND lease_token=?`, receipt.OwnerReference, receipt.EvidenceDigest, claim.ID, claim.LeaseToken)
	if err != nil || affected(result) != 1 {
		return errors.New("entitlement-created outbox completion conflict")
	}
	return nil
}

func (worker *entitlementCreatedWorker) retry(ctx context.Context, claim *entitlementCreatedClaim) (bool, error) {
	manual := claim.AttemptCount >= 10
	status := "retry_wait"
	if manual {
		status = "manual_attention"
	}
	result, err := worker.database.Exec(ctx, `UPDATE plugin_idc_core_entitlement_created_outbox SET status_code=?,next_attempt_at=CURRENT_TIMESTAMP+INTERVAL '30 seconds',lease_token='',lease_expires_at=NULL,last_error_code='owner_delivery_unavailable',updated_at=CURRENT_TIMESTAMP WHERE id=? AND status_code='delivering' AND lease_token=?`, status, claim.ID, claim.LeaseToken)
	if err != nil || affected(result) != 1 {
		return false, errors.New("entitlement-created outbox retry conflict")
	}
	return manual, nil
}

func entitlementCreatedLease() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
