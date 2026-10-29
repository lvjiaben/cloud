-- Payment contexts are recovery projections, not identity authorities. Every
-- prepare/resume still re-reads the verified supplier or staff-owned projection.
-- Staff verification deliberately has no fabricated supplier case number.
ALTER TABLE plugin_idc_core_commerce_payment_context
 DROP CONSTRAINT IF EXISTS ck_idc_com_payment_context_ready;
ALTER TABLE plugin_idc_core_commerce_payment_context
 ADD CONSTRAINT ck_idc_com_payment_context_ready CHECK (
  (status_code = 'ready' AND reason_code = 'payment_ready'
   AND identity_status_code = 'verified' AND identity_version > 0
   AND ready_at IS NOT NULL)
  OR (status_code <> 'ready' AND ready_at IS NULL)
 );
