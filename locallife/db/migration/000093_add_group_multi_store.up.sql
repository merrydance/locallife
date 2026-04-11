-- Group multi-store management models

CREATE TABLE merchant_group_applications (
    id BIGSERIAL PRIMARY KEY,
    applicant_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    group_name TEXT NOT NULL,
    contact_phone TEXT NOT NULL,
    license_number TEXT,
    license_image_url TEXT,
    address TEXT,
    region_id BIGINT REFERENCES regions(id),
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'submitted', 'approved', 'rejected')),
    reject_reason TEXT,
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    application_data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_group_applicant ON merchant_group_applications(applicant_user_id);
CREATE INDEX idx_group_app_status ON merchant_group_applications(status);

CREATE TABLE merchant_groups (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    contact_phone TEXT,
    license_number TEXT,
    license_image_url TEXT,
    address TEXT,
    region_id BIGINT REFERENCES regions(id),
    application_data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_groups_owner ON merchant_groups(owner_user_id);

CREATE TABLE merchant_brands (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES merchant_groups(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    logo_url TEXT,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_brands_group ON merchant_brands(group_id);

CREATE TABLE merchant_group_members (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES merchant_groups(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'finance', 'ops')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    invited_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE (group_id, user_id)
);

CREATE INDEX idx_group_members_group ON merchant_group_members(group_id);

CREATE TABLE merchant_group_join_requests (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES merchant_groups(id) ON DELETE CASCADE,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    applicant_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled')),
    reason TEXT,
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (group_id, merchant_id, status)
);

CREATE INDEX idx_group_join_group ON merchant_group_join_requests(group_id);

CREATE TABLE group_policies (
    group_id BIGINT PRIMARY KEY REFERENCES merchant_groups(id) ON DELETE CASCADE,
    pricing_mode TEXT NOT NULL DEFAULT 'store' CHECK (pricing_mode IN ('central', 'store')),
    menu_mode TEXT NOT NULL DEFAULT 'store' CHECK (menu_mode IN ('central', 'store')),
    inventory_mode TEXT NOT NULL DEFAULT 'store' CHECK (inventory_mode IN ('central', 'store')),
    promotion_mode TEXT NOT NULL DEFAULT 'store' CHECK (promotion_mode IN ('central', 'store'))
);

CREATE TABLE group_menu_templates (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES merchant_groups(id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    version INT NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE brand_menu_templates (
    id BIGSERIAL PRIMARY KEY,
    brand_id BIGINT NOT NULL REFERENCES merchant_brands(id) ON DELETE CASCADE,
    payload JSONB NOT NULL,
    version INT NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE merchant_group_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT REFERENCES merchant_groups(id) ON DELETE CASCADE,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id BIGINT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE merchants ADD COLUMN IF NOT EXISTS group_id BIGINT REFERENCES merchant_groups(id);
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS brand_id BIGINT REFERENCES merchant_brands(id);

CREATE INDEX IF NOT EXISTS idx_merchants_group_id ON merchants(group_id);
CREATE INDEX IF NOT EXISTS idx_merchants_brand_id ON merchants(brand_id);
