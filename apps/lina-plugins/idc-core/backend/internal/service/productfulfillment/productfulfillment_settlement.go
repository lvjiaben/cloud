package productfulfillment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/logger"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

func (s *serviceImpl) ConsumeFinanciallySettled(ctx context.Context, input SettlementInput) (*SettlementResult, error) {
	if ctx == nil || ctx.Err() != nil || strings.TrimSpace(input.SettlementNo) == "" || strings.TrimSpace(input.OrderNo) == "" {
		return nil, errorsNew("product_fulfillment_settlement_invalid")
	}
	var output *SettlementResult
	stage := "settlement"
	err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		authority, err := s.lockSettlement(txCtx, input)
		if err != nil {
			return err
		}
		stage = "frozen_items"
		items, err := s.loadFrozenItems(txCtx, authority, nil)
		if err != nil {
			return err
		}
		stage = "configuration"
		if err = s.validateFrozenConfigurations(txCtx, items); err != nil {
			return err
		}
		stage = "existing_intents"
		existingValue, valueErr := s.database.GetValue(txCtx, `SELECT count(*) FROM plugin_idc_core_product_fulfillment_intent WHERE tenant_id=0 AND order_no=?`, authority.OrderNo)
		if valueErr != nil {
			return valueErr
		}
		existingIntents := existingValue.Int()
		if existingIntents < 0 || existingIntents > 1000 {
			return errorsNew("product_fulfillment_order_evidence_unavailable")
		}
		if existingIntents > 0 {
			output, err = s.materializeAll(txCtx, authority, items, nil)
			return err
		}
		ownerGatesCleared := make(map[string]bool)
		gateItems := make([]string, 0, len(items))
		for _, item := range items {
			if item.StrategyCode == StrategyPaymentConfirmedPlugin && item.InterfaceNewBusinessState == "accepting" && item.InventoryDecisionCode == "available" {
				gateItems = append(gateItems, item.OrderItemNo)
			}
		}
		if len(gateItems) > 0 && s.gates != nil {
			stage = "owner_gates"
			ownerGatesCleared, err = evaluateOwnerGates(txCtx, s.gates, authority.OrderNo, gateItems)
			if err != nil {
				return err
			}
		}
		stage = "materialization"
		output, err = s.materializeAll(txCtx, authority, items, ownerGatesCleared)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Warningf(ctx, "product fulfillment settlement blocked stage=%s", stage)
	}
	return output, err
}

func (s *serviceImpl) lockSettlement(ctx context.Context, input SettlementInput) (*settlementAuthority, error) {
	var row *settlementAuthority
	err := s.database.GetScan(ctx, &row, `
SELECT settlement.settlement_no, settlement.order_no,
       settlement.customer_no AS payer_customer_no,
       COALESCE(attribution.service_owner_customer_no, settlement.customer_no) AS service_owner_customer_no,
       settlement.invoice_no, settlement.currency_code,
       settlement.settled_at, orders.status_code AS order_status,orders.risk_decision_code,
       orders.item_count
FROM plugin_idc_core_financial_settlement settlement
JOIN plugin_idc_core_commerce_order orders
  ON orders.tenant_id=settlement.tenant_id AND orders.order_no=settlement.order_no
LEFT JOIN plugin_idc_core_channel_order_attribution attribution
  ON attribution.tenant_id=settlement.tenant_id AND attribution.order_no=settlement.order_no
WHERE settlement.tenant_id=0 AND settlement.settlement_no=? AND settlement.order_no=?
FOR UPDATE OF settlement, orders`, input.SettlementNo, input.OrderNo)
	if err != nil {
		return nil, err
	}
	if row == nil || row.SettledAt.IsZero() || row.OrderStatus != "paid" || row.ItemCount < 1 || row.ItemCount > 100 {
		return nil, errorsNew("product_fulfillment_settlement_unavailable")
	}
	return row, nil
}

func (s *serviceImpl) loadFrozenItems(ctx context.Context, authority *settlementAuthority, orderItemNos []string) ([]frozenItem, error) {
	if orderItemNos == nil {
		orderItemNos = []string{}
	}
	rawOrderItemNos, err := json.Marshal(orderItemNos)
	if err != nil {
		return nil, err
	}
	var rows []frozenItem
	err = s.database.GetScan(ctx, &rows, `
SELECT item.order_item_no,item.order_no,
       COALESCE((SELECT target.entitlement_no FROM plugin_idc_core_commerce_addon_target target WHERE target.tenant_id=item.tenant_id AND target.order_no=item.order_no),'') AS target_entitlement_no,
       COALESCE(attribution.service_owner_customer_no,orders.customer_no) AS customer_no,
       item.product_no,item.version_no,
       item.product_name_zh_cn,item.product_name_en_us,item.inventory_decision_code,
       quantity.quantity,quantity.content_digest AS quantity_digest,
       policy.strategy_code,policy.granularity_code,policy.ongoing_management,policy.renewal_mode_code,policy.renewal_method,policy.entitlement_owner_id,
       policy.provision_method,policy.query_method,
       policy.maximum_attempts,policy.retry_base_seconds,policy.policy_digest,
       COALESCE(configuration.snapshot_no,'') AS configuration_snapshot_no,
       COALESCE(configuration.configuration_revision,'') AS configuration_revision,
       COALESCE(configuration.interface_instance_no,'') AS interface_instance_no,
       COALESCE(configuration.interface_configuration_revision,0) AS interface_configuration_revision,
       COALESCE(configuration.content_digest,'') AS configuration_content_digest,
       COALESCE(binding.plugin_id,'') AS plugin_id,
       COALESCE(binding.plugin_release_version,'') AS plugin_release_version,
       COALESCE(binding.contract_version,'') AS contract_version,
       COALESCE(binding.release_digest,'') AS release_digest,
       COALESCE(binding.declaration_digest,'') AS declaration_digest,
       COALESCE(binding.authorized_methods,ARRAY[]::TEXT[]) AS authorized_methods,
       COALESCE(interface.new_business_state,'') AS interface_new_business_state,
       COALESCE(configuration.uploader_user_id,0) AS uploader_user_id,
       COALESCE(configuration.customer_declaration_digest,'') AS customer_declaration_digest,
       COALESCE(configuration.candidate_snapshot_digest,'') AS candidate_snapshot_digest,
       COALESCE(configuration.customer_value_digest,'') AS customer_value_digest,
       COALESCE(version_configuration.customer_declaration_snapshot::text,'') AS customer_declaration_snapshot,
       COALESCE(configuration.candidate_snapshot::text,'') AS candidate_snapshot,
       COALESCE(configuration.typed_value_snapshot::text,'') AS typed_value_snapshot,
       ?::timestamptz AS settled_at
FROM plugin_idc_core_commerce_order_item item
JOIN plugin_idc_core_commerce_order orders ON orders.tenant_id=item.tenant_id AND orders.order_no=item.order_no
LEFT JOIN plugin_idc_core_channel_order_attribution attribution
  ON attribution.tenant_id=item.tenant_id AND attribution.order_no=item.order_no
JOIN plugin_idc_core_commerce_order_item_quantity_snapshot quantity
  ON quantity.tenant_id=item.tenant_id AND quantity.order_item_no=item.order_item_no
JOIN plugin_idc_core_catalog_product_version_fulfillment_policy policy
  ON policy.tenant_id=item.tenant_id AND policy.version_no=item.version_no
LEFT JOIN plugin_idc_core_catalog_product_version_fulfillment_binding binding
  ON binding.tenant_id=item.tenant_id AND binding.version_no=item.version_no
LEFT JOIN plugin_idc_core_product_automation_interface_instance interface
  ON interface.tenant_id=binding.tenant_id AND interface.interface_instance_no=binding.interface_instance_no
LEFT JOIN plugin_idc_core_commerce_structured_configuration_snapshot configuration
  ON configuration.tenant_id=item.tenant_id AND configuration.order_item_no=item.order_item_no
LEFT JOIN plugin_idc_core_catalog_product_version_configuration version_configuration
  ON version_configuration.tenant_id=item.tenant_id AND version_configuration.version_no=item.version_no
WHERE item.tenant_id=0 AND item.order_no=?
  AND (?::jsonb='[]'::jsonb OR item.order_item_no IN(SELECT jsonb_array_elements_text(?::jsonb)))
ORDER BY item.line_number,item.id
LIMIT 101
FOR UPDATE OF item,quantity,policy`, authority.SettledAt, authority.OrderNo, rawOrderItemNos, rawOrderItemNos)
	if err != nil {
		return nil, err
	}
	expectedCount := authority.ItemCount
	if len(orderItemNos) > 0 {
		expectedCount = len(orderItemNos)
	}
	if len(rows) == 0 || len(rows) != expectedCount {
		return nil, errorsNew("product_fulfillment_order_evidence_unavailable")
	}
	for i := range rows {
		if err := validateFrozenItem(&rows[i], authority); err != nil {
			return nil, err
		}
		// No installed contract currently declares target-service provisioning.
		// Ordinary provision methods create a new resource and must never run
		// for an add-on; keep its paid evidence in the manual delivery queue.
		if rows[i].TargetEntitlementNo != "" {
			makeAddonManual(&rows[i])
		}
	}
	return rows, nil
}

func (s *serviceImpl) validateFrozenConfigurations(ctx context.Context, items []frozenItem) error {
	inputs := make([]productautomation.FrozenCustomerConfigurationInput, 0, len(items))
	for _, item := range items {
		if item.StrategyCode == StrategyManual && item.InterfaceInstanceNo == "" {
			continue
		}
		if s.frozen == nil {
			return errorsNew("product_fulfillment_exact_evidence_unavailable")
		}
		schema, err := configurationcontract.DecodeDeclaration(item.CustomerDeclarationSnapshot)
		if err != nil {
			return errorsNew("product_fulfillment_configuration_invalid")
		}
		values, err := configurationcontract.DecodeCompiledValues(schema, []byte(item.TypedValueSnapshot))
		if err != nil {
			return errorsNew("product_fulfillment_configuration_invalid")
		}
		candidates := configurationcontract.ResolvedCandidates{}
		if err = json.Unmarshal([]byte(item.CandidateSnapshot), &candidates); err != nil {
			return errorsNew("product_fulfillment_configuration_invalid")
		}
		inputs = append(inputs, productautomation.FrozenCustomerConfigurationInput{OrderNo: item.OrderNo, CustomerNo: item.CustomerNo, UploaderUserID: item.UploaderUserID, VersionNo: item.VersionNo, InterfaceInstanceNo: item.InterfaceInstanceNo, InterfaceConfigurationRevision: item.InterfaceConfigurationRevision, CustomerDeclarationDigest: item.CustomerDeclarationDigest, CandidateSnapshotDigest: item.CandidateSnapshotDigest, CustomerValueDigest: item.CustomerValueDigest, ConfigurationRevision: item.ConfigurationRevision, Values: values, Candidates: candidates})
	}
	if len(inputs) == 0 {
		return nil
	}
	evidence, err := s.frozen.ValidateBatch(ctx, inputs)
	if err != nil || len(evidence) != len(inputs) {
		logger.Warningf(ctx, "frozen fulfillment configuration validation failed reason=%s", frozenConfigurationFailureReason(err))
		return errorsNew("product_fulfillment_exact_evidence_unavailable")
	}
	return nil
}

func validateFrozenItem(item *frozenItem, authority *settlementAuthority) error {
	if item == nil || item.OrderNo != authority.OrderNo || item.CustomerNo != authority.ServiceOwnerCustomerNo ||
		item.Quantity < 1 || item.Quantity > 1000 || len(item.QuantityDigest) != 64 || len(item.PolicyDigest) != 64 ||
		item.MaximumAttempts != maximumQueryAttempts || item.RetryBaseSeconds != retryIntervalSeconds {
		return errorsNew("product_fulfillment_order_evidence_invalid")
	}
	if item.InventoryDecisionCode != "available" {
		return errorsNew("product_fulfillment_order_evidence_invalid")
	}
	switch item.GranularityCode {
	case GranularityPerOrderItem, GranularityPerQuantityUnit:
	default:
		return errorsNew("product_fulfillment_policy_invalid")
	}
	switch item.StrategyCode {
	case StrategyManual:
		if item.ProvisionMethod != "" || item.QueryMethod != "" || item.OngoingManagement ||
			item.RenewalMode != productfulfillmentcap.RenewalModeUnavailable || item.RenewalMethod != "" {
			return errorsNew("product_fulfillment_policy_invalid")
		}
		if item.InterfaceInstanceNo != "" && !completeBinding(item) {
			return errorsNew("product_fulfillment_binding_invalid")
		}
	case StrategyApprovalThenPlugin, StrategyPaymentConfirmedPlugin:
		if !completeBinding(item) || item.ConfigurationContentDigest == "" ||
			item.ProvisionMethod == "" || item.QueryMethod == "" {
			return errorsNew("product_fulfillment_binding_invalid")
		}
		if !containsMethod(item.AuthorizedMethods, item.ProvisionMethod) || !containsMethod(item.AuthorizedMethods, item.QueryMethod) {
			return errorsNew("product_fulfillment_method_unauthorized")
		}
		if err := validateFrozenRenewal(item); err != nil {
			return err
		}
	default:
		return errorsNew("product_fulfillment_policy_invalid")
	}
	return nil
}

func validateFrozenRenewal(item *frozenItem) error {
	if (item.OngoingManagement && item.EntitlementOwnerID == "") ||
		(!item.OngoingManagement && item.EntitlementOwnerID != "") {
		return errorsNew("product_fulfillment_entitlement_owner_invalid")
	}
	switch item.RenewalMode {
	case productfulfillmentcap.RenewalModeProviderConfirmed:
		if !item.OngoingManagement || item.RenewalMethod == "" ||
			!containsMethod(item.AuthorizedMethods, item.RenewalMethod) {
			return errorsNew("product_fulfillment_renewal_contract_invalid")
		}
	case productfulfillmentcap.RenewalModeLocalEntitlement:
		if !item.OngoingManagement || item.RenewalMethod != "" {
			return errorsNew("product_fulfillment_renewal_contract_invalid")
		}
	case productfulfillmentcap.RenewalModeUnavailable:
		if item.RenewalMethod != "" {
			return errorsNew("product_fulfillment_renewal_contract_invalid")
		}
	default:
		return errorsNew("product_fulfillment_renewal_contract_invalid")
	}
	return nil
}

func completeBinding(item *frozenItem) bool {
	return item != nil && item.InterfaceInstanceNo != "" && item.PluginID != "" &&
		item.PluginReleaseVersion != "" && item.ContractVersion != "" && item.ConfigurationSnapshotNo != "" &&
		item.InterfaceConfigurationRevision > 0 && len(item.ReleaseDigest) == 64 && len(item.DeclarationDigest) == 64
}
func containsMethod(methods []string, target string) bool {
	for _, method := range methods {
		if method == target {
			return true
		}
	}
	return false
}

type intentInsert struct {
	IntentNo                       string    `json:"intent_no"`
	SettlementNo                   string    `json:"settlement_no"`
	OrderNo                        string    `json:"order_no"`
	OrderItemNo                    string    `json:"order_item_no"`
	CustomerNo                     string    `json:"customer_no"`
	PayerCustomerNo                string    `json:"payer_customer_no"`
	InvoiceNo                      string    `json:"invoice_no"`
	CurrencyCode                   string    `json:"currency_code"`
	ProductNo                      string    `json:"product_no"`
	VersionNo                      string    `json:"version_no"`
	ProductNameZhCN                string    `json:"product_name_zh_cn"`
	ProductNameEnUS                string    `json:"product_name_en_us"`
	Quantity                       int       `json:"quantity"`
	UnitSequence                   int       `json:"unit_sequence"`
	StrategyCode                   string    `json:"strategy_code"`
	GranularityCode                string    `json:"granularity_code"`
	OngoingManagement              bool      `json:"ongoing_management"`
	RenewalMode                    string    `json:"renewal_mode_code"`
	RenewalMethod                  string    `json:"renewal_method"`
	EntitlementOwnerID             string    `json:"entitlement_owner_id"`
	InitialStatusCode              string    `json:"initial_status_code"`
	StatusCode                     string    `json:"status_code"`
	ConfigurationSnapshotNo        string    `json:"configuration_snapshot_no"`
	InterfaceInstanceNo            string    `json:"interface_instance_no"`
	InterfaceConfigurationRevision int64     `json:"interface_configuration_revision"`
	PluginID                       string    `json:"plugin_id"`
	PluginReleaseVersion           string    `json:"plugin_release_version"`
	ContractVersion                string    `json:"contract_version"`
	ReleaseDigest                  string    `json:"release_digest"`
	DeclarationDigest              string    `json:"declaration_digest"`
	ProvisionMethod                string    `json:"provision_method"`
	QueryMethod                    string    `json:"query_method"`
	PolicyDigest                   string    `json:"policy_digest"`
	EvidenceDigest                 string    `json:"evidence_digest"`
	SettledAt                      time.Time `json:"financially_settled_at"`
	MaximumAttempts                int       `json:"maximum_attempts"`
	RetryBaseSeconds               int       `json:"retry_base_seconds"`
}

type deliveryInsert struct {
	DeliveryNo              string    `json:"delivery_no"`
	OrderNo                 string    `json:"order_no"`
	OrderItemNo             string    `json:"order_item_no"`
	CustomerNo              string    `json:"customer_no"`
	ProductNo               string    `json:"product_no"`
	VersionNo               string    `json:"version_no"`
	Quantity                int       `json:"quantity"`
	StrategyCode            string    `json:"strategy_code"`
	PolicyDigest            string    `json:"policy_digest"`
	ConfigurationSnapshotNo string    `json:"configuration_snapshot_no"`
	InterfaceInstanceNo     string    `json:"interface_instance_no"`
	PluginID                string    `json:"plugin_id"`
	PluginReleaseVersion    string    `json:"plugin_release_version"`
	ContractVersion         string    `json:"contract_version"`
	DeclarationDigest       string    `json:"declaration_digest"`
	OngoingManagement       bool      `json:"ongoing_management"`
	RenewalMode             string    `json:"renewal_mode_code"`
	RenewalMethod           string    `json:"renewal_method"`
	EntitlementOwnerID      string    `json:"entitlement_owner_id"`
	StatusCode              string    `json:"status_code"`
	StartedAt               time.Time `json:"started_at"`
	ContentDigest           string    `json:"content_digest"`
}

func (s *serviceImpl) materializeAll(ctx context.Context, authority *settlementAuthority, items []frozenItem, ownerGatesCleared map[string]bool) (*SettlementResult, error) {
	var frozenInitialRows []struct{ IntentNo, InitialStatusCode string }
	if err := s.database.GetScan(ctx, &frozenInitialRows, `SELECT intent_no,initial_status_code FROM plugin_idc_core_product_fulfillment_intent WHERE tenant_id=0 AND order_no=? ORDER BY order_item_no,unit_sequence LIMIT 1001`, authority.OrderNo); err != nil {
		return nil, err
	}
	frozenInitial := make(map[string]string, len(frozenInitialRows))
	for _, row := range frozenInitialRows {
		frozenInitial[row.IntentNo] = row.InitialStatusCode
	}
	plans, err := buildIntentPlans(authority, items, ownerGatesCleared, frozenInitial)
	if err != nil {
		return nil, err
	}
	if err = s.materializeDeliveries(ctx, authority, items, plans); err != nil {
		return nil, err
	}
	result := &SettlementResult{OrderNo: authority.OrderNo}
	raw, err := json.Marshal(plans)
	if err != nil {
		return nil, err
	}
	createdRows := make([]struct{ IntentNo string }, 0, len(plans))
	err = s.database.GetScan(ctx, &createdRows, `WITH input AS (
SELECT x.intent_no,x.settlement_no,x.order_no,x.order_item_no,x.customer_no,x.payer_customer_no,x.invoice_no,x.currency_code,x.product_no,x.version_no,x.product_name_zh_cn,x.product_name_en_us,x.quantity,x.unit_sequence,x.strategy_code,x.granularity_code,x.ongoing_management,x.renewal_mode_code,x.renewal_method,x.entitlement_owner_id,x.initial_status_code,x.status_code,x.configuration_snapshot_no,x.interface_instance_no,x.interface_configuration_revision,x.plugin_id,x.plugin_release_version,x.contract_version,x.release_digest,x.declaration_digest,x.provision_method,x.query_method,x.policy_digest,x.evidence_digest,x.financially_settled_at,x.maximum_attempts,x.retry_base_seconds
FROM jsonb_to_recordset(?::jsonb) AS x(intent_no text,settlement_no text,order_no text,order_item_no text,customer_no text,payer_customer_no text,invoice_no text,currency_code text,product_no text,version_no text,product_name_zh_cn text,product_name_en_us text,quantity int,unit_sequence int,strategy_code text,granularity_code text,ongoing_management boolean,renewal_mode_code text,renewal_method text,entitlement_owner_id text,initial_status_code text,status_code text,configuration_snapshot_no text,interface_instance_no text,interface_configuration_revision bigint,plugin_id text,plugin_release_version text,contract_version text,release_digest text,declaration_digest text,provision_method text,query_method text,policy_digest text,evidence_digest text,financially_settled_at timestamptz,maximum_attempts int,retry_base_seconds int)), inserted AS (
INSERT INTO plugin_idc_core_product_fulfillment_intent(tenant_id,intent_no,settlement_no,order_no,order_item_no,customer_no,payer_customer_no,invoice_no,currency_code,product_no,version_no,product_name_zh_cn,product_name_en_us,quantity,unit_sequence,strategy_code,granularity_code,ongoing_management,renewal_mode_code,renewal_method,entitlement_owner_id,initial_status_code,status_code,configuration_snapshot_no,interface_instance_no,interface_configuration_revision,plugin_id,plugin_release_version,contract_version,release_digest,declaration_digest,provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest,evidence_digest,financially_settled_at)
SELECT 0,intent_no,settlement_no,order_no,order_item_no,customer_no,payer_customer_no,invoice_no,currency_code,product_no,version_no,product_name_zh_cn,product_name_en_us,quantity,unit_sequence,strategy_code,granularity_code,ongoing_management,renewal_mode_code,renewal_method,entitlement_owner_id,initial_status_code,status_code,configuration_snapshot_no,interface_instance_no,interface_configuration_revision,plugin_id,plugin_release_version,contract_version,release_digest,declaration_digest,provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest,evidence_digest,financially_settled_at FROM input ON CONFLICT(order_item_no,unit_sequence) DO NOTHING RETURNING intent_no) SELECT intent_no FROM inserted`, raw)
	if err != nil {
		return nil, err
	}
	return s.verifyMaterializedIntents(ctx, authority, plans, createdRows, result)
}

func (s *serviceImpl) materializeDeliveries(
	ctx context.Context,
	authority *settlementAuthority,
	items []frozenItem,
	plans []intentInsert,
) error {
	statusByItem := make(map[string]string, len(items))
	for _, plan := range plans {
		if _, exists := statusByItem[plan.OrderItemNo]; exists {
			continue
		}
		status := plan.InitialStatusCode
		if status == "gate_pending" {
			status = "queued"
		}
		statusByItem[plan.OrderItemNo] = status
	}
	rows := make([]deliveryInsert, 0, len(items))
	for _, item := range items {
		row := deliveryInsert{
			DeliveryNo: deterministicNo("pdl_", item.OrderItemNo), OrderNo: authority.OrderNo,
			OrderItemNo: item.OrderItemNo, CustomerNo: authority.ServiceOwnerCustomerNo,
			ProductNo: item.ProductNo, VersionNo: item.VersionNo, Quantity: item.Quantity,
			StrategyCode: item.StrategyCode, PolicyDigest: item.PolicyDigest,
			ConfigurationSnapshotNo: item.ConfigurationSnapshotNo,
			InterfaceInstanceNo:     item.InterfaceInstanceNo, PluginID: item.PluginID,
			PluginReleaseVersion: item.PluginReleaseVersion, ContractVersion: item.ContractVersion,
			DeclarationDigest: item.DeclarationDigest, OngoingManagement: item.OngoingManagement,
			RenewalMode: item.RenewalMode, RenewalMethod: item.RenewalMethod,
			EntitlementOwnerID: item.EntitlementOwnerID,
			StatusCode:         statusByItem[item.OrderItemNo], StartedAt: authority.SettledAt,
		}
		if row.StrategyCode == StrategyManual {
			row.InterfaceInstanceNo = ""
			row.PluginID = ""
			row.PluginReleaseVersion = ""
			row.ContractVersion = ""
			row.DeclarationDigest = ""
			row.OngoingManagement = false
			row.RenewalMode = productfulfillmentcap.RenewalModeUnavailable
			row.RenewalMethod = ""
			row.EntitlementOwnerID = ""
		}
		digest, err := digestJSON(row)
		if err != nil {
			return err
		}
		row.ContentDigest = digest
		rows = append(rows, row)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	inserted, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_product_delivery_record(
		tenant_id,delivery_no,order_no,order_item_no,customer_no,product_no,version_no,quantity,
		strategy_code,policy_digest,configuration_snapshot_no,interface_instance_no,plugin_id,
		plugin_release_version,contract_version,declaration_digest,ongoing_management,status_code,
		renewal_mode_code,renewal_method,entitlement_owner_id,
		started_at,version,content_digest)
	SELECT 0,delivery_no,order_no,order_item_no,customer_no,product_no,version_no,quantity,
		strategy_code,policy_digest,configuration_snapshot_no,interface_instance_no,plugin_id,
		plugin_release_version,contract_version,declaration_digest,ongoing_management,status_code,
		renewal_mode_code,renewal_method,entitlement_owner_id,
		started_at,1,content_digest
	FROM jsonb_to_recordset(?::jsonb) AS x(delivery_no text,order_no text,order_item_no text,
		customer_no text,product_no text,version_no text,quantity int,strategy_code text,policy_digest text,
		configuration_snapshot_no text,interface_instance_no text,plugin_id text,plugin_release_version text,
		contract_version text,declaration_digest text,ongoing_management boolean,status_code text,
		renewal_mode_code text,renewal_method text,entitlement_owner_id text,
		started_at timestamptz,content_digest text)
	ON CONFLICT(order_item_no) DO NOTHING`, raw)
	if err != nil {
		return err
	}
	if _, err = inserted.RowsAffected(); err != nil {
		return err
	}
	value, err := s.database.GetValue(ctx, `WITH expected AS (
		SELECT * FROM jsonb_to_recordset(?::jsonb) AS x(order_item_no text,content_digest text)
	) SELECT count(*) FROM expected JOIN plugin_idc_core_product_delivery_record delivery
	  ON delivery.tenant_id=0 AND delivery.order_item_no=expected.order_item_no
	 AND delivery.content_digest=expected.content_digest`, raw)
	if err != nil || value.Int() != len(rows) {
		return errorsNew("product_delivery_idempotency_conflict")
	}
	return nil
}

func buildIntentPlans(authority *settlementAuthority, items []frozenItem, ownerGatesCleared map[string]bool, frozenInitial map[string]string) ([]intentInsert, error) {
	plans := make([]intentInsert, 0, authority.ItemCount)
	for _, item := range items {
		units := 1
		if item.GranularityCode == GranularityPerQuantityUnit {
			units = item.Quantity
		}
		if len(plans) > 1000-units {
			return nil, errorsNew("product_fulfillment_intent_capacity_exceeded")
		}
		for sequence := 1; sequence <= units; sequence++ {
			intentNo := deterministicNo("pft_", item.OrderItemNo, fmt.Sprint(sequence))
			status := "queued"
			if item.StrategyCode == StrategyManual || (item.StrategyCode != StrategyManual && item.InterfaceNewBusinessState != "accepting") {
				status = "manual_pending"
			}
			if item.StrategyCode == StrategyApprovalThenPlugin && item.InterfaceNewBusinessState == "accepting" {
				status = "review_pending"
			}
			if item.StrategyCode == StrategyPaymentConfirmedPlugin && item.InterfaceNewBusinessState == "accepting" && !ownerGatesCleared[item.OrderItemNo] {
				status = "gate_pending"
			}
			if frozenStatus := frozenInitial[intentNo]; frozenStatus != "" {
				status = frozenStatus
			}
			frozenEvidence := item
			frozenEvidence.InterfaceNewBusinessState = ""
			evidence, err := digestJSON(struct {
				SettlementNo  string
				Item          frozenItem
				Unit          int
				InitialStatus string
			}{authority.SettlementNo, frozenEvidence, sequence, status})
			if err != nil {
				return nil, err
			}
			plans = append(plans, intentInsert{
				IntentNo: intentNo, SettlementNo: authority.SettlementNo, OrderNo: authority.OrderNo,
				OrderItemNo: item.OrderItemNo, CustomerNo: authority.ServiceOwnerCustomerNo,
				PayerCustomerNo: authority.PayerCustomerNo, InvoiceNo: authority.InvoiceNo,
				CurrencyCode: authority.CurrencyCode, ProductNo: item.ProductNo, VersionNo: item.VersionNo,
				ProductNameZhCN: item.ProductNameZhCN, ProductNameEnUS: item.ProductNameEnUS,
				Quantity: item.Quantity, UnitSequence: sequence, StrategyCode: item.StrategyCode,
				GranularityCode: item.GranularityCode, OngoingManagement: item.OngoingManagement,
				RenewalMode: item.RenewalMode, RenewalMethod: item.RenewalMethod,
				EntitlementOwnerID: item.EntitlementOwnerID,
				InitialStatusCode:  status, StatusCode: status,
				ConfigurationSnapshotNo:        item.ConfigurationSnapshotNo,
				InterfaceInstanceNo:            item.InterfaceInstanceNo,
				InterfaceConfigurationRevision: item.InterfaceConfigurationRevision,
				PluginID:                       item.PluginID, PluginReleaseVersion: item.PluginReleaseVersion,
				ContractVersion: item.ContractVersion, ReleaseDigest: item.ReleaseDigest,
				DeclarationDigest: item.DeclarationDigest, ProvisionMethod: item.ProvisionMethod,
				QueryMethod: item.QueryMethod, PolicyDigest: item.PolicyDigest, EvidenceDigest: evidence,
				SettledAt: authority.SettledAt, MaximumAttempts: item.MaximumAttempts,
				RetryBaseSeconds: item.RetryBaseSeconds,
			})
		}
	}
	return plans, nil
}

func (s *serviceImpl) verifyMaterializedIntents(ctx context.Context, authority *settlementAuthority, plans []intentInsert, createdRows []struct{ IntentNo string }, result *SettlementResult) (*SettlementResult, error) {
	var err error
	created := make([]string, 0, len(createdRows))
	for _, row := range createdRows {
		created = append(created, row.IntentNo)
	}
	type stored struct {
		IntentNo          string `orm:"intent_no"`
		EvidenceDigest    string `orm:"evidence_digest"`
		InitialStatusCode string `orm:"initial_status_code"`
	}
	storedRows := make([]stored, 0, len(plans))
	if err = s.database.GetScan(ctx, &storedRows, `SELECT intent_no,evidence_digest,initial_status_code FROM plugin_idc_core_product_fulfillment_intent WHERE tenant_id=0 AND order_no=? ORDER BY order_item_no,unit_sequence LIMIT 1001`, authority.OrderNo); err != nil {
		return nil, err
	}
	if len(storedRows) != len(plans) {
		return nil, errorsNew("product_fulfillment_idempotency_conflict")
	}
	expected := make(map[string]intentInsert, len(plans))
	for _, p := range plans {
		expected[p.IntentNo] = p
	}
	for _, row := range storedRows {
		p, ok := expected[row.IntentNo]
		if !ok || row.EvidenceDigest != p.EvidenceDigest || row.InitialStatusCode != p.InitialStatusCode {
			return nil, errorsNew("product_fulfillment_idempotency_conflict")
		}
	}
	createdSet := make(map[string]struct{}, len(created))
	for _, intentNo := range created {
		createdSet[intentNo] = struct{}{}
	}
	tasks := make([]map[string]any, 0, len(created))
	for _, p := range plans {
		if _, isCreated := createdSet[p.IntentNo]; !isCreated {
			continue
		}
		switch p.StatusCode {
		case "manual_pending":
			result.ManualCount++
		case "review_pending":
			result.ReviewCount++
		case "queued":
			result.TaskCount++
		}
		if p.StatusCode == "queued" {
			tasks = append(tasks, map[string]any{"task_no": deterministicNo("pfk_", p.IntentNo), "intent_no": p.IntentNo, "maximum_attempts": p.MaximumAttempts, "retry_base_seconds": p.RetryBaseSeconds})
		}
	}
	if len(tasks) > 0 {
		taskRaw, marshalErr := json.Marshal(tasks)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_product_fulfillment_task(tenant_id,task_no,intent_no,status_code,maximum_attempts,retry_base_seconds) SELECT 0,task_no,intent_no,'pending_query',maximum_attempts,retry_base_seconds FROM jsonb_to_recordset(?::jsonb) AS x(task_no text,intent_no text,maximum_attempts int,retry_base_seconds int) ON CONFLICT(intent_no) DO NOTHING`, taskRaw); err != nil {
			return nil, err
		}
		verifiedValue, verifyErr := s.database.GetValue(ctx, `WITH expected AS (SELECT * FROM jsonb_to_recordset(?::jsonb) AS x(task_no text,intent_no text,maximum_attempts int,retry_base_seconds int)) SELECT count(*) FROM expected JOIN plugin_idc_core_product_fulfillment_task task ON task.tenant_id=0 AND task.task_no=expected.task_no AND task.intent_no=expected.intent_no AND task.status_code='pending_query' AND task.attempt_count=0 AND task.maximum_attempts=expected.maximum_attempts AND task.retry_base_seconds=expected.retry_base_seconds`, taskRaw)
		if verifyErr != nil || verifiedValue.Int() != len(tasks) {
			return nil, errorsNew(taskConflictCode)
		}
	}
	var taskEvidence []struct {
		TaskNo                            string
		IntentNo                          string
		MaximumAttempts, RetryBaseSeconds int
	}
	if err = s.database.GetScan(ctx, &taskEvidence, `SELECT task.task_no,task.intent_no,task.maximum_attempts,task.retry_base_seconds FROM plugin_idc_core_product_fulfillment_task task JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.tenant_id=task.tenant_id AND intent.intent_no=task.intent_no WHERE intent.tenant_id=0 AND intent.order_no=? ORDER BY task.intent_no LIMIT 1001`, authority.OrderNo); err != nil {
		return nil, err
	}
	expectedTaskCount := 0
	for _, p := range plans {
		if p.InitialStatusCode == "queued" {
			expectedTaskCount++
		}
	}
	if len(taskEvidence) != expectedTaskCount {
		return nil, errorsNew(taskConflictCode)
	}
	for _, task := range taskEvidence {
		plan, ok := expected[task.IntentNo]
		if !ok || plan.InitialStatusCode != "queued" || task.TaskNo != deterministicNo("pfk_", task.IntentNo) || task.MaximumAttempts != plan.MaximumAttempts || task.RetryBaseSeconds != plan.RetryBaseSeconds {
			return nil, errorsNew(taskConflictCode)
		}
	}
	result.IntentCount = len(created)
	result.Reused = len(created) == 0
	return result, nil
}

type stableError string

func (e stableError) Error() string { return string(e) }
func errorsNew(code string) error {
	switch code {
	case "product_fulfillment_management_unavailable":
		return bizerr.NewCode(CodeManagementUnavailable)
	case "product_fulfillment_action_invalid":
		return bizerr.NewCode(CodeActionInvalid)
	case "product_fulfillment_action_conflict", "product_fulfillment_idempotency_conflict", taskConflictCode:
		return bizerr.NewCode(CodeActionConflict)
	case "product_fulfillment_not_accessible":
		return bizerr.NewCode(CodeNotAccessible)
	case "product_fulfillment_reason_invalid":
		return bizerr.NewCode(CodeReasonInvalid)
	case "product_fulfillment_owner_gate_blocked":
		return bizerr.NewCode(CodeOwnerGateBlocked)
	case "product_fulfillment_new_business_stopped":
		return bizerr.NewCode(CodeNewBusinessStopped)
	default:
		return stableError(code)
	}
}

func deterministicNo(prefix string, parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(part))
	}
	return prefix + hex.EncodeToString(h.Sum(nil)[:16])
}
func digestJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func makeAddonManual(item *frozenItem) {
	item.StrategyCode = StrategyManual
	item.OngoingManagement = false
	item.RenewalMode = productfulfillmentcap.RenewalModeUnavailable
	item.RenewalMethod = ""
	item.EntitlementOwnerID = ""
	item.ProvisionMethod = ""
	item.QueryMethod = ""
	item.InterfaceInstanceNo = ""
	item.InterfaceConfigurationRevision = 0
	item.PluginID = ""
	item.PluginReleaseVersion = ""
	item.ContractVersion = ""
	item.ReleaseDigest = ""
	item.DeclarationDigest = ""
	item.AuthorizedMethods = nil
}
