-- Remove payment-owned routing settings when the payment plugin is removed.
DROP TABLE IF EXISTS plugin_idc_payment_public_address;
