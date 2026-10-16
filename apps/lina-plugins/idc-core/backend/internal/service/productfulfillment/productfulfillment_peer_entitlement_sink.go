// This file consumes idc-peer entitlement-created events locally. Peer
// fulfillment has already persisted the upstream service mapping atomically;
// the sink acknowledges that fact and never pretends an external provisioning
// receipt exists.

package productfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/entitlementcreatedcap"
	"lina-plugin-idc-core/backend/cap/renewalownercap"
)

type peerEntitlementSink struct{ database gdb.DB }

// NewPeerEntitlementCreatedSink uses the shared core database to confirm only
// previously persisted service associations, without an external side effect.
func NewPeerEntitlementCreatedSink(database gdb.DB) (entitlementcreatedcap.Sink, error) {
	if database == nil {
		return nil, gerror.New("peer entitlement sink database is unavailable")
	}
	return &peerEntitlementSink{database: database}, nil
}

func (sink *peerEntitlementSink) Deliver(ctx context.Context, event entitlementcreatedcap.Event) (*entitlementcreatedcap.Receipt, error) {
	if sink == nil || sink.database == nil || ctx == nil || ctx.Err() != nil || !entitlementcreatedcap.ValidateEvent(event) || event.OwnerID != renewalownercap.PeerOwnerID || event.PluginID != renewalownercap.PeerOwnerID {
		return nil, gerror.New("peer entitlement event is unavailable")
	}
	var row *struct{ UpstreamOrderReference, UpstreamServiceNo string }
	if err := sink.database.GetScan(ctx, &row, `SELECT upstream_order_reference,upstream_service_no FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND entitlement_no=? AND source_intent_no=? AND customer_no=? AND product_no=? AND version_no=? AND plugin_id='idc-peer' AND configuration_snapshot_no=? AND interface_instance_no=? AND plugin_release_version=? AND contract_version=? AND declaration_digest=?`, event.EntitlementNo, event.IntentNo, event.CustomerNo, event.ProductNo, event.ProductVersionNo, event.ConfigurationSnapshotNo, event.InterfaceInstanceNo, event.PluginReleaseVersion, event.ContractVersion, event.DeclarationDigest); err != nil || row == nil || row.UpstreamOrderReference == "" || row.UpstreamServiceNo == "" {
		return nil, gerror.New("peer entitlement mapping is unavailable")
	}
	digest := sha256.Sum256([]byte("idc-peer/entitlement-created-receipt/v1\x00" + event.EventNo + "\x00" + event.EntitlementNo + "\x00" + row.UpstreamOrderReference + "\x00" + row.UpstreamServiceNo))
	return &entitlementcreatedcap.Receipt{Version: entitlementcreatedcap.ContractVersion, EventNo: event.EventNo, OwnerReference: row.UpstreamServiceNo, EvidenceDigest: hex.EncodeToString(digest[:])}, nil
}
