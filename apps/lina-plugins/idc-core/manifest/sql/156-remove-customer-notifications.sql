-- Remove destinations for the retired customer notification center.
-- Preserve unrelated navigation and promote any children before removing the node.
UPDATE plugin_idc_core_product_directory
SET parent_directory_no=''
WHERE parent_directory_no IN (SELECT directory_no FROM plugin_idc_core_product_directory
 WHERE navigation_type IN ('system','plugin')
   AND rtrim(split_part(split_part(target_url,chr(63),1),'#',1),'/') = '/account/notifications');

DELETE FROM plugin_idc_core_product_directory_membership
WHERE directory_no IN (SELECT directory_no FROM plugin_idc_core_product_directory
 WHERE navigation_type IN ('system','plugin')
   AND rtrim(split_part(split_part(target_url,chr(63),1),'#',1),'/') = '/account/notifications');

DELETE FROM plugin_idc_core_product_directory_visibility
WHERE directory_no IN (SELECT directory_no FROM plugin_idc_core_product_directory
 WHERE navigation_type IN ('system','plugin')
   AND rtrim(split_part(split_part(target_url,chr(63),1),'#',1),'/') = '/account/notifications');

DELETE FROM plugin_idc_core_product_directory
WHERE navigation_type IN ('system','plugin')
  AND rtrim(split_part(split_part(target_url,chr(63),1),'#',1),'/') = '/account/notifications';
