-- Permit an audited recovery of resource-owner preparation failures while
-- retaining every attempt and monotonic cumulative counter. No orders are changed.
ALTER TABLE plugin_idc_core_product_fulfillment_task
 ADD COLUMN IF NOT EXISTS recovery_count INT NOT NULL DEFAULT 0,
 ADD COLUMN IF NOT EXISTS recovery_query_baseline INT NOT NULL DEFAULT 0,
 ADD COLUMN IF NOT EXISTS recovery_submit_baseline INT NOT NULL DEFAULT 0;
ALTER TABLE plugin_idc_core_product_fulfillment_command
 ADD COLUMN IF NOT EXISTS recovery_task_version BIGINT,
 ADD COLUMN IF NOT EXISTS recovery_query_count INT,
 ADD COLUMN IF NOT EXISTS recovery_submit_count INT;
ALTER TABLE plugin_idc_core_product_fulfillment_task DROP CONSTRAINT IF EXISTS ck_idc_product_fulfillment_task_retry;
ALTER TABLE plugin_idc_core_product_fulfillment_task ADD CONSTRAINT ck_idc_product_fulfillment_task_retry CHECK (
 maximum_attempts=10 AND retry_base_seconds=30 AND recovery_count>=0
 AND recovery_query_baseline BETWEEN 0 AND query_attempt_count
 AND recovery_submit_baseline BETWEEN 0 AND submit_attempt_count
 AND query_attempt_count-recovery_query_baseline BETWEEN 0 AND 10
 AND submit_attempt_count-recovery_submit_baseline BETWEEN 0 AND 10
 AND attempt_count=query_attempt_count+submit_attempt_count
);
ALTER TABLE plugin_idc_core_product_fulfillment_command DROP CONSTRAINT IF EXISTS ck_idc_product_fulfillment_command_action;
ALTER TABLE plugin_idc_core_product_fulfillment_command ADD CONSTRAINT ck_idc_product_fulfillment_command_action CHECK (
 action_code IN ('review_approve','review_reject','takeover','manual_conclude_completed','manual_conclude_unable','recover_preparation')
);
ALTER TABLE plugin_idc_core_product_fulfillment_command DROP CONSTRAINT IF EXISTS ck_idc_product_fulfillment_recovery_evidence;
ALTER TABLE plugin_idc_core_product_fulfillment_command ADD CONSTRAINT ck_idc_product_fulfillment_recovery_evidence CHECK (
 (action_code='recover_preparation' AND recovery_task_version>0 AND recovery_query_count>=0 AND recovery_submit_count>=0
  AND recovery_task_version IS NOT NULL AND recovery_query_count IS NOT NULL AND recovery_submit_count IS NOT NULL)
 OR (action_code<>'recover_preparation' AND recovery_task_version IS NULL AND recovery_query_count IS NULL AND recovery_submit_count IS NULL)
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_product_fulfillment_recovery_task_version
 ON plugin_idc_core_product_fulfillment_command(intent_no,recovery_task_version) WHERE action_code='recover_preparation';

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_fulfillment_task()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'product fulfillment task evidence is immutable'; END IF;
	IF TG_OP = 'INSERT' THEN
		IF NEW."status_code" <> 'pending_query' OR NEW."phase_code" <> 'pre_submit_query'
		   OR NEW."attempt_count" <> 0 OR NEW."query_attempt_count" <> 0 OR NEW."submit_attempt_count" <> 0
		   OR NEW."lease_token" <> '' OR NEW."lease_expires_at" IS NOT NULL OR NEW."external_reference" <> ''
		   OR NEW."last_result_code" <> '' OR NEW."takeover_requested" OR NEW."cancel_requested"
		   OR NEW."version" <> 1 OR NEW.recovery_count <> 0 THEN
			RAISE EXCEPTION 'initial product fulfillment task state is invalid';
		END IF;
		RETURN NEW;
	END IF;
    IF NEW."tenant_id" IS DISTINCT FROM OLD."tenant_id" OR NEW."task_no" IS DISTINCT FROM OLD."task_no"
       OR NEW."intent_no" IS DISTINCT FROM OLD."intent_no" OR NEW."maximum_attempts" IS DISTINCT FROM OLD."maximum_attempts"
       OR NEW."retry_base_seconds" IS DISTINCT FROM OLD."retry_base_seconds" OR NEW."created_at" IS DISTINCT FROM OLD."created_at" THEN
        RAISE EXCEPTION 'product fulfillment task frozen evidence is immutable';
    END IF;
    IF NEW."version" <> OLD."version" + 1 OR NEW."updated_at" < OLD."updated_at"
       OR NEW."attempt_count" < OLD."attempt_count" OR NEW."query_attempt_count" < OLD."query_attempt_count"
       OR NEW."submit_attempt_count" < OLD."submit_attempt_count"
       OR NEW."query_attempt_count" - OLD."query_attempt_count" NOT IN (0,1)
       OR NEW."submit_attempt_count" - OLD."submit_attempt_count" NOT IN (0,1)
       OR (NEW."query_attempt_count" - OLD."query_attempt_count") + (NEW."submit_attempt_count" - OLD."submit_attempt_count") NOT IN (0,1) THEN
        RAISE EXCEPTION 'product fulfillment task version or counter transition is invalid';
    END IF;
	IF (NEW."query_attempt_count" = OLD."query_attempt_count" + 1) IS DISTINCT FROM (NEW."status_code"='querying' AND OLD."status_code"<>'querying') OR
	   (NEW."submit_attempt_count" = OLD."submit_attempt_count" + 1) IS DISTINCT FROM (NEW."status_code"='submitting' AND OLD."status_code"<>'submitting') THEN
		RAISE EXCEPTION 'product fulfillment task counters do not match the claimed action';
	END IF;
	IF OLD."takeover_requested" AND NOT NEW."takeover_requested" THEN
		RAISE EXCEPTION 'product fulfillment takeover request is immutable';
	END IF;
	IF OLD."cancel_requested" AND NOT NEW."cancel_requested" THEN
		RAISE EXCEPTION 'product fulfillment cancellation request is immutable';
	END IF;
	IF NEW."cancel_requested" IS DISTINCT FROM OLD."cancel_requested" AND NOT (
		NOT OLD."cancel_requested" AND NEW."cancel_requested" AND NEW."status_code" IN ('querying','submitting','cancellation_attention','canceled')
	) THEN RAISE EXCEPTION 'product fulfillment cancellation transition is invalid'; END IF;
	IF NEW."takeover_requested" IS DISTINCT FROM OLD."takeover_requested" AND NOT (
		NOT OLD."takeover_requested" AND NEW."takeover_requested" AND
		((OLD."status_code" IN ('querying','submitting') AND NEW."status_code"=OLD."status_code") OR NEW."status_code"='manual_takeover')
	) THEN RAISE EXCEPTION 'product fulfillment takeover transition is invalid'; END IF;
    IF NEW.recovery_count = OLD.recovery_count + 1 THEN
        IF OLD.status_code <> 'failed' OR OLD.phase_code <> 'pre_submit_query'
           OR NEW.status_code <> 'pending_query' OR NEW.phase_code <> 'pre_submit_query'
           OR OLD.external_reference <> '' OR NEW.external_reference <> ''
           OR OLD.lease_token <> '' OR NEW.lease_token <> ''
           OR OLD.takeover_requested OR OLD.cancel_requested
           OR NEW.attempt_count <> OLD.attempt_count
           OR NEW.recovery_query_baseline <> OLD.query_attempt_count
           OR NEW.recovery_submit_baseline <> OLD.submit_attempt_count
           OR NEW.last_result_code <> OLD.last_result_code
           OR NOT EXISTS (
               SELECT 1 FROM plugin_idc_core_product_fulfillment_command command
               JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.intent_no=command.intent_no
               WHERE command.intent_no=OLD.intent_no AND command.action_code='recover_preparation'
                 AND command.recovery_task_version=OLD.version
                 AND command.recovery_query_count=OLD.query_attempt_count
                 AND command.recovery_submit_count=OLD.submit_attempt_count
                 AND command.resulting_version=intent.version+1
                 AND intent.status_code='manual_pending' AND intent.entitlement_owner_id='idc-resource'
           ) THEN RAISE EXCEPTION 'fulfillment recovery requires immutable pre-submission evidence'; END IF;
        RETURN NEW;
    END IF;
    IF NEW.recovery_count <> OLD.recovery_count
       OR NEW.recovery_query_baseline <> OLD.recovery_query_baseline
       OR NEW.recovery_submit_baseline <> OLD.recovery_submit_baseline THEN
        RAISE EXCEPTION 'fulfillment recovery accounting is immutable outside recovery';
    END IF;
	IF OLD."status_code" IN ('succeeded','failed','manual_takeover','cancellation_attention','canceled') OR
	   (OLD."status_code"='result_unknown' AND NEW."status_code"<>'manual_takeover') THEN
		RAISE EXCEPTION 'terminal product fulfillment task evidence is immutable';
	END IF;
	IF NEW."status_code" IS NOT DISTINCT FROM OLD."status_code" AND NOT (
		OLD."status_code" IN ('querying','submitting') AND
		((NOT OLD."takeover_requested" AND NEW."takeover_requested") OR (NOT OLD."cancel_requested" AND NEW."cancel_requested"))
	) THEN RAISE EXCEPTION 'product fulfillment task update must advance its state'; END IF;
    IF NEW."status_code" IS DISTINCT FROM OLD."status_code" AND NOT (
        (OLD."status_code" IN ('pending_query','retry_wait','accepted','running') AND NEW."status_code"='querying') OR
        (OLD."status_code"='pending_submit' AND NEW."status_code"='submitting') OR
        (OLD."status_code" IN ('querying','submitting') AND NEW."status_code" IN ('pending_submit','retry_wait','accepted','running','succeeded','failed','result_unknown','manual_takeover','cancellation_attention','canceled')) OR
        (OLD."status_code" IN ('pending_query','pending_submit','retry_wait','accepted','running','result_unknown') AND NEW."status_code" IN ('manual_takeover','cancellation_attention','canceled'))
    ) THEN RAISE EXCEPTION 'product fulfillment task status transition is invalid'; END IF;
    IF (NEW."status_code" IN ('querying','submitting')) <> (NEW."lease_token" <> '' AND NEW."lease_expires_at" IS NOT NULL) THEN
        RAISE EXCEPTION 'product fulfillment task lease does not match its state';
    END IF;
	IF NEW."status_code" IN ('querying','submitting') AND NEW."lease_expires_at" <= CURRENT_TIMESTAMP THEN
		RAISE EXCEPTION 'product fulfillment task lease must expire in the future';
	END IF;
    IF (NEW."status_code"='pending_query' AND NEW."phase_code" <> 'pre_submit_query') OR
	   (NEW."status_code" IN ('pending_submit','submitting') AND NEW."phase_code" <> 'submit_allowed') OR
       (NEW."status_code" IN ('accepted','running','succeeded') AND NEW."phase_code" <> 'post_submit_query') OR
	   (NEW."status_code"='retry_wait' AND NEW."phase_code" NOT IN ('pre_submit_query','post_submit_query')) OR
	   (NEW."status_code" IN ('result_unknown','manual_takeover','cancellation_attention','canceled') AND NEW."phase_code" <> 'frozen_unknown') OR
	   (NEW."status_code"='querying' AND NEW."phase_code" NOT IN ('pre_submit_query','post_submit_query')) OR
	   (NEW."status_code"='failed' AND NEW."phase_code" NOT IN ('pre_submit_query','post_submit_query')) THEN
        RAISE EXCEPTION 'product fulfillment task phase does not match its state';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_fulfillment_intent()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'product fulfillment intent evidence is immutable'; END IF;
    IF TG_OP = 'INSERT' THEN
        IF NEW."initial_status_code" IS DISTINCT FROM NEW."status_code" OR
           NEW."approved_by" IS NOT NULL OR NEW."approved_at" IS NOT NULL OR NOT (
            (NEW."strategy_code"='manual' AND NEW."status_code"='manual_pending') OR
            (NEW."strategy_code"='approval_then_plugin' AND NEW."status_code" IN ('review_pending','manual_pending')) OR
            (NEW."strategy_code"='payment_confirmed_plugin' AND NEW."status_code" IN ('queued','gate_pending','manual_pending'))
        ) THEN RAISE EXCEPTION 'initial product fulfillment intent state is invalid'; END IF;
        IF NOT EXISTS(
            SELECT 1 FROM plugin_idc_core_commerce_order_item item
            JOIN plugin_idc_core_commerce_order_item_quantity_snapshot "quantity"
              ON "quantity"."tenant_id"=item."tenant_id" AND "quantity"."order_item_no"=item."order_item_no"
            WHERE item."tenant_id"=NEW."tenant_id" AND item."order_item_no"=NEW."order_item_no"
              AND item."order_no"=NEW."order_no" AND item."product_no"=NEW."product_no"
              AND item."version_no"=NEW."version_no" AND "quantity"."quantity"=NEW."quantity"
        ) THEN RAISE EXCEPTION 'product fulfillment order item evidence is invalid'; END IF;
        RETURN NEW;
    END IF;
    IF NEW."tenant_id" IS DISTINCT FROM OLD."tenant_id" OR NEW."intent_no" IS DISTINCT FROM OLD."intent_no"
       OR NEW."settlement_no" IS DISTINCT FROM OLD."settlement_no" OR NEW."order_no" IS DISTINCT FROM OLD."order_no"
       OR NEW."order_item_no" IS DISTINCT FROM OLD."order_item_no" OR NEW."customer_no" IS DISTINCT FROM OLD."customer_no"
       OR NEW."invoice_no" IS DISTINCT FROM OLD."invoice_no" OR NEW."currency_code" IS DISTINCT FROM OLD."currency_code"
       OR NEW."product_no" IS DISTINCT FROM OLD."product_no" OR NEW."version_no" IS DISTINCT FROM OLD."version_no"
       OR NEW."product_name_zh_cn" IS DISTINCT FROM OLD."product_name_zh_cn"
       OR NEW."product_name_en_us" IS DISTINCT FROM OLD."product_name_en_us"
       OR NEW."quantity" IS DISTINCT FROM OLD."quantity" OR NEW."unit_sequence" IS DISTINCT FROM OLD."unit_sequence"
       OR NEW."strategy_code" IS DISTINCT FROM OLD."strategy_code" OR NEW."granularity_code" IS DISTINCT FROM OLD."granularity_code"
       OR NEW."initial_status_code" IS DISTINCT FROM OLD."initial_status_code"
       OR NEW."configuration_snapshot_no" IS DISTINCT FROM OLD."configuration_snapshot_no"
       OR NEW."interface_instance_no" IS DISTINCT FROM OLD."interface_instance_no"
       OR NEW."interface_configuration_revision" IS DISTINCT FROM OLD."interface_configuration_revision"
       OR NEW."plugin_id" IS DISTINCT FROM OLD."plugin_id" OR NEW."plugin_release_version" IS DISTINCT FROM OLD."plugin_release_version"
       OR NEW."contract_version" IS DISTINCT FROM OLD."contract_version" OR NEW."release_digest" IS DISTINCT FROM OLD."release_digest"
       OR NEW."declaration_digest" IS DISTINCT FROM OLD."declaration_digest" OR NEW."provision_method" IS DISTINCT FROM OLD."provision_method"
       OR NEW."query_method" IS DISTINCT FROM OLD."query_method" OR NEW."policy_digest" IS DISTINCT FROM OLD."policy_digest"
       OR NEW."maximum_attempts" IS DISTINCT FROM OLD."maximum_attempts"
       OR NEW."retry_base_seconds" IS DISTINCT FROM OLD."retry_base_seconds"
       OR NEW."evidence_digest" IS DISTINCT FROM OLD."evidence_digest" OR NEW."financially_settled_at" IS DISTINCT FROM OLD."financially_settled_at"
       OR NEW."created_at" IS DISTINCT FROM OLD."created_at" THEN
        RAISE EXCEPTION 'product fulfillment intent frozen evidence is immutable';
    END IF;
    IF OLD."approved_by" IS NOT NULL AND
       (NEW."approved_by" IS DISTINCT FROM OLD."approved_by" OR NEW."approved_at" IS DISTINCT FROM OLD."approved_at") THEN
        RAISE EXCEPTION 'product fulfillment approval evidence is immutable';
    END IF;
    IF NEW."version" <> OLD."version" + 1 OR NEW."updated_at" < OLD."updated_at" THEN
        RAISE EXCEPTION 'product fulfillment intent version transition is invalid';
    END IF;
    IF NEW."approved_by" IS DISTINCT FROM OLD."approved_by" OR NEW."approved_at" IS DISTINCT FROM OLD."approved_at" THEN
        IF OLD."strategy_code" <> 'approval_then_plugin' OR OLD."status_code" <> 'review_pending'
           OR NEW."status_code" <> 'queued' OR OLD."approved_by" IS NOT NULL
           OR NEW."approved_by" IS NULL OR NEW."approved_by" <= 0 OR NEW."approved_at" IS NULL THEN
            RAISE EXCEPTION 'product fulfillment approval transition is invalid';
        END IF;
    END IF;
    IF NEW."status_code" IS DISTINCT FROM OLD."status_code" AND NOT (
        (OLD."status_code" = 'review_pending' AND NEW."status_code" = 'queued'
         AND OLD."strategy_code" = 'approval_then_plugin' AND NEW."approved_by" IS NOT NULL) OR
        (OLD."status_code" = 'review_pending' AND NEW."status_code" = 'manual_pending'
         AND NEW."approved_by" IS NULL) OR
        (OLD."status_code" = 'gate_pending' AND NEW."status_code" = 'queued'
         AND OLD."strategy_code" = 'payment_confirmed_plugin' AND NEW."approved_by" IS NULL) OR
        (OLD."status_code" = 'gate_pending' AND NEW."status_code" = 'manual_pending'
         AND OLD."strategy_code" = 'payment_confirmed_plugin' AND NEW."approved_by" IS NULL) OR
        (OLD."status_code" = 'queued' AND NEW."status_code" IN ('processing','manual_pending')) OR
        (OLD."status_code" = 'processing' AND NEW."status_code" IN ('completed','incomplete','manual_pending')) OR
        (OLD."status_code" = 'manual_pending' AND NEW."status_code" IN ('completed','incomplete')) OR
        (OLD.status_code='manual_pending' AND NEW.status_code='queued'
         AND OLD.entitlement_owner_id='idc-resource'
         AND EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_command command
                    JOIN plugin_idc_core_product_fulfillment_task task ON task.intent_no=command.intent_no
                    WHERE command.intent_no=OLD.intent_no AND command.action_code='recover_preparation'
                      AND command.resulting_version=NEW.version AND task.status_code='pending_query'
                      AND task.version=command.recovery_task_version+1
                      AND task.recovery_query_baseline=command.recovery_query_count
                      AND task.recovery_submit_baseline=command.recovery_submit_count)) OR
		(OLD."status_code" IN ('manual_pending','review_pending','gate_pending','queued','processing') AND NEW."status_code"='canceled'
		 AND EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_system_transition transition
		            WHERE transition."intent_no"=OLD."intent_no" AND transition."event_code"='cancelled'))
    ) THEN
        RAISE EXCEPTION 'product fulfillment intent status transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

ALTER TABLE plugin_idc_core_product_fulfillment_reason_option DROP CONSTRAINT IF EXISTS ck_idc_product_fulfillment_reason_action;
ALTER TABLE plugin_idc_core_product_fulfillment_reason_option ADD CONSTRAINT ck_idc_product_fulfillment_reason_action CHECK (
 action_code IN ('review_approve','review_reject','takeover','manual_conclude_completed','manual_conclude_unable','recover_preparation')
);
INSERT INTO plugin_idc_core_product_fulfillment_reason_option(tenant_id,action_code,reason_code,label_zh_cn,label_en_us,sort_order,status_code)
 VALUES(0,'recover_preparation','preparation_resolved','开通准备问题已处理','Provisioning prerequisites repaired',10,'active')
 ON CONFLICT(action_code,reason_code) DO NOTHING;
