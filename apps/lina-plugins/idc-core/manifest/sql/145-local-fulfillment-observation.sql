-- Query counters are immutable audit facts, not a duplicated polling policy.
-- The worker owns bounded query windows (10 remote or 60 local observations).
-- Keep monotonic counters, frozen policy units and the hard write-attempt cap.
ALTER TABLE plugin_idc_core_product_fulfillment_task
 DROP CONSTRAINT IF EXISTS ck_idc_product_fulfillment_task_retry;
ALTER TABLE plugin_idc_core_product_fulfillment_task
 ADD CONSTRAINT ck_idc_product_fulfillment_task_retry CHECK (
 maximum_attempts=10 AND retry_base_seconds=30 AND recovery_count>=0
 AND recovery_query_baseline BETWEEN 0 AND query_attempt_count
 AND recovery_submit_baseline BETWEEN 0 AND submit_attempt_count
 AND submit_attempt_count-recovery_submit_baseline BETWEEN 0 AND 10
 AND attempt_count=query_attempt_count+submit_attempt_count
);
