-- Menu hierarchy is shared configuration; customer templates choose its presentation.
ALTER TABLE plugin_idc_core_product_directory
 ADD COLUMN IF NOT EXISTS parent_directory_no VARCHAR(40) NOT NULL DEFAULT '';
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_idc_core_directory_parent_self' AND conrelid='plugin_idc_core_product_directory'::regclass) THEN
  ALTER TABLE plugin_idc_core_product_directory ADD CONSTRAINT ck_idc_core_directory_parent_self CHECK (parent_directory_no<>directory_no);
 END IF;
END $$;
CREATE INDEX IF NOT EXISTS idx_idc_core_product_directory_parent
 ON plugin_idc_core_product_directory(tenant_id,parent_directory_no,sort_order,id);
COMMENT ON COLUMN plugin_idc_core_product_directory.parent_directory_no IS 'Parent menu reference; empty means top level; owner validates a maximum of two levels';
