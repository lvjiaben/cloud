// This file validates peer delivery snapshots and materializes their mappings
// in the fulfillment transaction; finalize freezes the snapshot with task state.
// Only explicit managed upstream services create local entitlements; the order
// reference and service identity are immutable and used for exact renewals.

package productfulfillment

import (
	"context"
	"encoding/json"
	"strings"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/renewalownercap"
)

func (w *fulfillmentWorker) storePeerDeliveries(ctx context.Context, claimed *claimedFulfillment, result *productfulfillmentcap.Result) error {
	if claimed.PluginID != renewalownercap.PeerOwnerID || result.Status != productfulfillmentcap.StatusSucceeded {
		return nil
	}
	if !strings.HasPrefix(result.ExternalReference, "peer-order:") || len(result.Deliveries) == 0 || productfulfillmentcap.ValidateResult(claimed.Action, result) != nil {
		return errorsNew(taskConflictCode)
	}
	var policy *struct {
		OngoingManagement bool
		RenewalModeCode   string
	}
	if err := w.database.GetScan(ctx, &policy, `SELECT ongoing_management,renewal_mode_code FROM plugin_idc_core_product_fulfillment_intent WHERE tenant_id=0 AND intent_no=?`, claimed.IntentNo); err != nil || policy == nil {
		return errorsNew(taskConflictCode)
	}
	managed := 0
	seen := make(map[string]bool, len(result.Deliveries))
	for _, delivery := range result.Deliveries {
		if delivery.UpstreamServiceNo == "" {
			continue
		}
		if !policy.OngoingManagement || seen[delivery.UpstreamServiceNo] || (policy.RenewalModeCode != string(productfulfillmentcap.RenewalModeUnavailable) && delivery.ExpiresAt == 0) {
			return errorsNew(taskConflictCode)
		}
		seen[delivery.UpstreamServiceNo] = true
		managed++
	}
	if policy.OngoingManagement && managed == 0 {
		return errorsNew(taskConflictCode)
	}
	raw, err := json.Marshal(result.Deliveries)
	if err != nil {
		return err
	}
	// The task is already locked by finalize. Comparing the snapshot also rejects
	// any changed mapping if a committed result is accidentally replayed.
	saved, err := w.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_task WHERE tenant_id=0 AND task_no=? AND (upstream_deliveries='[]'::jsonb OR upstream_deliveries=?::jsonb))`, claimed.TaskNo, raw)
	if err != nil || !saved.Bool() {
		return errorsNew(taskConflictCode)
	}
	if managed == 0 {
		return nil
	}
	_, err = w.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_entitlement(
 tenant_id,entitlement_no,delivery_no,source_intent_no,customer_no,order_no,order_item_no,product_no,version_no,unit_sequence,
 configuration_snapshot_no,interface_instance_no,plugin_id,plugin_release_version,contract_version,declaration_digest,
 status_code,started_at,expires_at,upstream_order_reference,upstream_delivery_no,upstream_service_no)
 SELECT 0,'ent_'||substr(md5('peer-entitlement:'||i.intent_no||':'||x."upstreamServiceNo"),1,32),d.delivery_no,i.intent_no,i.customer_no,i.order_no,i.order_item_no,i.product_no,i.version_no,i.unit_sequence,
 i.configuration_snapshot_no,i.interface_instance_no,i.plugin_id,i.plugin_release_version,i.contract_version,i.declaration_digest,
 'active',to_timestamp(x."startedAt"::numeric/1000),CASE WHEN x."expiresAt">0 THEN to_timestamp(x."expiresAt"::numeric/1000) END,?,x."upstreamDeliveryNo",x."upstreamServiceNo"
 FROM plugin_idc_core_product_fulfillment_intent i
 JOIN plugin_idc_core_product_delivery_record d ON d.tenant_id=i.tenant_id AND d.order_no=i.order_no AND d.order_item_no=i.order_item_no AND d.customer_no=i.customer_no
 CROSS JOIN jsonb_to_recordset(?::jsonb) AS x("upstreamDeliveryNo" text,"upstreamServiceNo" text,"startedAt" bigint,"expiresAt" bigint)
 WHERE i.tenant_id=0 AND i.intent_no=? AND i.plugin_id='idc-peer' AND i.ongoing_management AND COALESCE(x."upstreamServiceNo",'')<>''
 ON CONFLICT(entitlement_no) DO NOTHING`, result.ExternalReference, raw, claimed.IntentNo)
	if err != nil {
		return err
	}
	count, err := w.database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND source_intent_no=? AND upstream_order_reference=? AND upstream_service_no<>''`, claimed.IntentNo, result.ExternalReference)
	if err != nil || count.Int() != managed {
		return errorsNew(taskConflictCode)
	}
	return nil
}
