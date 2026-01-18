ALTER TABLE merchants DROP COLUMN IF EXISTS brand_id;
ALTER TABLE merchants DROP COLUMN IF EXISTS group_id;

DROP TABLE IF EXISTS merchant_group_audit_logs;
DROP TABLE IF EXISTS brand_menu_templates;
DROP TABLE IF EXISTS group_menu_templates;
DROP TABLE IF EXISTS group_policies;
DROP TABLE IF EXISTS merchant_group_join_requests;
DROP TABLE IF EXISTS merchant_group_members;
DROP TABLE IF EXISTS merchant_brands;
DROP TABLE IF EXISTS merchant_groups;
DROP TABLE IF EXISTS merchant_group_applications;