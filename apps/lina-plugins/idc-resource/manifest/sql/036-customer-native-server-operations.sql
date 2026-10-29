-- Purpose: Bounded native customer operation lookup and resource task exclusion.
-- 用途：为原生服务器客户操作及互斥检查提供有界索引，不新增服务器业务真相。
CREATE INDEX IF NOT EXISTS idx_idc_resource_customer_operation_reference
    ON plugin_idc_resource_task (tenant_id,local_resource_no,(owner_metadata->>'operationNo'))
    WHERE owner_metadata->>'operationNo' LIKE 'nso\_%' ESCAPE '\';
CREATE INDEX IF NOT EXISTS idx_idc_resource_unfinished_resource_task
    ON plugin_idc_resource_task (tenant_id,local_resource_no)
    WHERE status_code NOT IN ('succeeded','failed','canceled');
