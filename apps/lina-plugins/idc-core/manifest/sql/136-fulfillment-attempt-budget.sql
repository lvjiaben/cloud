-- Keep retry accounting monotonic and consistent with the bounded worker policy.
-- Replaying this migration after audited recovery must retain cumulative history.
ALTER TABLE plugin_idc_core_product_fulfillment_task DROP CONSTRAINT IF EXISTS ck_idc_product_fulfillment_task_retry;
DO $budget$
BEGIN
 IF EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='plugin_idc_core_product_fulfillment_task' AND column_name='recovery_query_baseline') THEN
  ALTER TABLE plugin_idc_core_product_fulfillment_task ADD CONSTRAINT ck_idc_product_fulfillment_task_retry CHECK (
   maximum_attempts=10 AND retry_base_seconds=30 AND recovery_count>=0
   AND recovery_query_baseline BETWEEN 0 AND query_attempt_count
   AND recovery_submit_baseline BETWEEN 0 AND submit_attempt_count
   AND query_attempt_count-recovery_query_baseline BETWEEN 0 AND 10
   AND submit_attempt_count-recovery_submit_baseline BETWEEN 0 AND 10
   AND attempt_count=query_attempt_count+submit_attempt_count
  );
 ELSE
  ALTER TABLE plugin_idc_core_product_fulfillment_task ADD CONSTRAINT ck_idc_product_fulfillment_task_retry CHECK (
   maximum_attempts=10 AND retry_base_seconds=30
   AND query_attempt_count BETWEEN 0 AND 10 AND submit_attempt_count BETWEEN 0 AND 10
   AND attempt_count=query_attempt_count+submit_attempt_count
  );
 END IF;
END $budget$;
