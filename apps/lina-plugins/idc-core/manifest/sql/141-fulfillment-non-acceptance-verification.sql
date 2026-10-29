-- Immutable staff verification facts retain the previous unknown task version.
CREATE TABLE IF NOT EXISTS plugin_idc_core_fulfillment_non_acceptance_verification (
 tenant_id BIGINT NOT NULL DEFAULT 0 CHECK(tenant_id=0),
 provider_task_no VARCHAR(64) PRIMARY KEY,
 intent_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_product_fulfillment_intent(intent_no),
 task_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_product_fulfillment_task(task_no),
 previous_task_version BIGINT NOT NULL CHECK(previous_task_version>0),
 resulting_version BIGINT NOT NULL CHECK(resulting_version>0),
 idempotency_key VARCHAR(128) NOT NULL UNIQUE,
 request_digest CHAR(64) NOT NULL,
 actor_user_id BIGINT NOT NULL CHECK(actor_user_id>0),
 safe_note VARCHAR(512) NOT NULL CHECK(char_length(safe_note) BETWEEN 10 AND 512),
 verified_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_idc_core_non_acceptance_intent
 ON plugin_idc_core_fulfillment_non_acceptance_verification(tenant_id,intent_no,verified_at DESC);
CREATE OR REPLACE FUNCTION plugin_idc_core_guard_non_acceptance_verification()
RETURNS TRIGGER AS $$ BEGIN
 RAISE EXCEPTION 'operator non-acceptance verification is immutable';
END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS guard_non_acceptance_verification ON plugin_idc_core_fulfillment_non_acceptance_verification;
CREATE TRIGGER guard_non_acceptance_verification BEFORE UPDATE OR DELETE
 ON plugin_idc_core_fulfillment_non_acceptance_verification FOR EACH ROW
 EXECUTE FUNCTION plugin_idc_core_guard_non_acceptance_verification();

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
    IF OLD.status_code='result_unknown' AND NEW.status_code='failed' THEN
      IF OLD.lease_token<>'' OR OLD.takeover_requested OR OLD.cancel_requested
       OR NEW.phase_code<>'post_submit_query' OR NEW.last_result_code<>'operator_non_acceptance_verified'
       OR NEW.external_reference IS DISTINCT FROM OLD.external_reference OR OLD.external_reference=''
       OR NEW.attempt_count<>OLD.attempt_count OR NEW.query_attempt_count<>OLD.query_attempt_count
       OR NEW.submit_attempt_count<>OLD.submit_attempt_count OR NEW.recovery_count<>OLD.recovery_count
       OR NEW.recovery_query_baseline<>OLD.recovery_query_baseline OR NEW.recovery_submit_baseline<>OLD.recovery_submit_baseline
       OR NEW.lease_token<>'' OR NEW.lease_expires_at IS NOT NULL
       OR NOT EXISTS(SELECT 1 FROM plugin_idc_core_fulfillment_non_acceptance_verification fact
          JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.intent_no=fact.intent_no
          WHERE fact.tenant_id=OLD.tenant_id AND fact.task_no=OLD.task_no AND fact.provider_task_no=OLD.external_reference
           AND fact.previous_task_version=OLD.version AND fact.resulting_version=intent.version+1
           AND intent.status_code='manual_pending' AND intent.entitlement_owner_id='idc-resource')
      THEN RAISE EXCEPTION 'unknown outcome requires independent operator verification'; END IF;
      RETURN NEW;
    END IF;
    IF NEW.recovery_count = OLD.recovery_count + 1 THEN
        IF OLD.status_code <> 'failed' OR NOT (
           (OLD.phase_code='pre_submit_query' AND OLD.external_reference='') OR
           (OLD.phase_code='post_submit_query' AND OLD.external_reference<>'' AND EXISTS(
             SELECT 1 FROM plugin_idc_core_product_fulfillment_attempt attempt
             WHERE attempt.tenant_id=OLD.tenant_id AND attempt.task_no=OLD.task_no
               AND attempt.attempt_no=OLD.attempt_count AND attempt.result_status_code='failed'
               AND attempt.finished_at IS NOT NULL
               AND attempt.safe_result_evidence->>'recoverableRejection'='true')) OR
           (OLD.phase_code='post_submit_query' AND OLD.last_result_code='operator_non_acceptance_verified' AND EXISTS(SELECT 1 FROM plugin_idc_core_fulfillment_non_acceptance_verification fact WHERE fact.task_no=OLD.task_no AND fact.provider_task_no=OLD.external_reference)))

           OR NEW.status_code <> 'pending_query' OR NEW.phase_code <> 'pre_submit_query'
           OR NEW.external_reference IS DISTINCT FROM OLD.external_reference
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
                 AND ((intent.status_code='manual_pending' AND OLD.phase_code='pre_submit_query') OR (intent.status_code='incomplete' AND OLD.phase_code='post_submit_query')) AND intent.entitlement_owner_id='idc-resource'
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


CREATE OR REPLACE FUNCTION plugin_idc_core_validate_fulfillment_terminal(target_intent_no VARCHAR)
RETURNS VOID AS $$
DECLARE
    intent_status VARCHAR(32);
    intent_strategy VARCHAR(32);
    conclusion_outcome VARCHAR(16);
    task_status VARCHAR(32);
    terminal_attempt_count INT;
BEGIN
    SELECT "status_code","strategy_code" INTO intent_status,intent_strategy
    FROM plugin_idc_core_product_fulfillment_intent
    WHERE "intent_no"=target_intent_no;
    IF NOT FOUND THEN RETURN; END IF;

    SELECT "outcome_code" INTO conclusion_outcome
    FROM plugin_idc_core_product_fulfillment_manual_conclusion
    WHERE "intent_no"=target_intent_no;

    IF intent_status NOT IN ('completed','incomplete') THEN
        IF conclusion_outcome IS NOT NULL THEN
            RAISE EXCEPTION 'manual conclusion requires a terminal fulfillment intent';
        END IF;
        RETURN;
    END IF;

    IF conclusion_outcome IS NOT NULL THEN
        IF (intent_status = 'completed' AND conclusion_outcome <> 'completed') OR
           (intent_status = 'incomplete' AND conclusion_outcome <> 'unable') THEN
            RAISE EXCEPTION 'manual conclusion does not match fulfillment intent status';
        END IF;
        RETURN;
    END IF;

    IF intent_strategy = 'manual' THEN
        RAISE EXCEPTION 'manual fulfillment terminal status requires a conclusion';
    END IF;
    SELECT "status_code" INTO task_status
    FROM plugin_idc_core_product_fulfillment_task
    WHERE "intent_no"=target_intent_no;
    IF (intent_status = 'completed' AND task_status <> 'succeeded') OR
       (intent_status = 'incomplete' AND task_status <> 'failed') OR task_status IS NULL THEN
        RAISE EXCEPTION 'automatic fulfillment terminal status requires a matching task result';
    END IF;
    IF intent_status='incomplete' AND EXISTS(
      SELECT 1 FROM plugin_idc_core_fulfillment_non_acceptance_verification fact
      JOIN plugin_idc_core_product_fulfillment_task task ON task.task_no=fact.task_no
      JOIN plugin_idc_core_product_fulfillment_intent intent ON intent.intent_no=fact.intent_no
      WHERE fact.intent_no=target_intent_no AND task.status_code='failed'
       AND task.last_result_code='operator_non_acceptance_verified'
       AND fact.provider_task_no=task.external_reference
       AND task.version=fact.previous_task_version+1 AND fact.resulting_version=intent.version
    ) THEN RETURN; END IF;
    SELECT count(*) INTO terminal_attempt_count
    FROM plugin_idc_core_product_fulfillment_attempt attempt
    JOIN plugin_idc_core_product_fulfillment_task task ON task."task_no"=attempt."task_no"
    WHERE task."intent_no"=target_intent_no AND attempt."finished_at" IS NOT NULL
      AND ((intent_status='completed' AND attempt."result_status_code"='succeeded') OR
           (intent_status='incomplete' AND attempt."result_status_code"='failed'));
    IF terminal_attempt_count < 1 THEN
        RAISE EXCEPTION 'automatic fulfillment terminal status requires immutable attempt evidence';
    END IF;
END;
$$ LANGUAGE plpgsql;
