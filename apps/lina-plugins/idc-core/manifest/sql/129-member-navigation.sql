-- Extend existing product menus without moving orders or service ownership.
ALTER TABLE plugin_idc_core_product_directory ADD COLUMN IF NOT EXISTS navigation_type VARCHAR(16) NOT NULL DEFAULT 'product';
ALTER TABLE plugin_idc_core_product_directory ADD COLUMN IF NOT EXISTS target_url VARCHAR(2048) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_core_product_directory ADD COLUMN IF NOT EXISTS icon VARCHAR(80) NOT NULL DEFAULT 'lucide:folder';
ALTER TABLE plugin_idc_core_product_directory DROP CONSTRAINT IF EXISTS ck_idc_core_member_navigation_type;
ALTER TABLE plugin_idc_core_product_directory ADD CONSTRAINT ck_idc_core_member_navigation_type CHECK(navigation_type IN ('product','system','custom','plugin'));

-- Preserve essential routes as editable navigation records when switching from
-- the built-in sidebar. Re-running never renames or re-enables an existing menu.
INSERT INTO plugin_idc_core_product_directory
(directory_no,name_zh_cn,name_en_us,sort_order,navigation_type,target_url,icon)
SELECT 'pdr_' || md5(defaults.target), defaults.zh, defaults.en, defaults.sort,
       'system', defaults.target, defaults.icon
FROM (VALUES
 ('总览','Overview',10,'/account','lucide:layout-dashboard'),
 ('购买商品','Shop',20,'/account/products/catalog','lucide:shopping-bag'),
 ('全部服务','All services',30,'/account/product-directories','lucide:server'),
 ('购物车','Cart',40,'/account/cart','lucide:shopping-cart'),
 ('订单','Orders',50,'/account/orders','lucide:file-text'),
 ('财务','Billing',60,'/account/billing','lucide:wallet'),
 ('工单','Support tickets',70,'/account/support','lucide:life-buoy'),
 ('账户资料','Profile',80,'/account/profile','lucide:user')
) AS defaults(zh,en,sort,target,icon)
WHERE NOT EXISTS (
 SELECT 1 FROM plugin_idc_core_product_directory existing
 WHERE existing.tenant_id=0 AND existing.navigation_type='system'
 AND existing.target_url=defaults.target
)
ON CONFLICT(directory_no) DO NOTHING;
