-- Billing groups to support shared ordering and split payments within a dining session
CREATE TABLE billing_groups (
    id BIGSERIAL PRIMARY KEY,
    dining_session_id BIGINT NOT NULL REFERENCES dining_sessions(id),
    status TEXT NOT NULL DEFAULT 'open',
    is_default BOOLEAN NOT NULL DEFAULT true,
    total_amount BIGINT NOT NULL DEFAULT 0,
    paid_amount BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    CONSTRAINT billing_groups_status_check CHECK (status IN ('open', 'partially_paid', 'paid', 'closed'))
);

-- One default billing group per dining session
CREATE UNIQUE INDEX billing_groups_default_idx ON billing_groups(dining_session_id) WHERE is_default = true;
-- Finder indexes
CREATE INDEX billing_groups_session_idx ON billing_groups(dining_session_id);
CREATE INDEX billing_groups_status_idx ON billing_groups(status);

-- Billing group members
CREATE TABLE billing_group_members (
    id BIGSERIAL PRIMARY KEY,
    billing_group_id BIGINT NOT NULL REFERENCES billing_groups(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    role TEXT NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    left_at TIMESTAMPTZ,
    CONSTRAINT billing_group_members_role_check CHECK (role IN ('owner', 'member'))
);

-- Only one active membership per user per billing group
CREATE UNIQUE INDEX billing_group_members_active_idx ON billing_group_members(billing_group_id, user_id) WHERE left_at IS NULL;
CREATE INDEX billing_group_members_group_idx ON billing_group_members(billing_group_id);
CREATE INDEX billing_group_members_user_idx ON billing_group_members(user_id);

-- Billing group orders
CREATE TABLE billing_group_orders (
    id BIGSERIAL PRIMARY KEY,
    billing_group_id BIGINT NOT NULL REFERENCES billing_groups(id),
    order_id BIGINT NOT NULL REFERENCES orders(id),
    amount BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'linked',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    CONSTRAINT billing_group_orders_status_check CHECK (status IN ('linked', 'paid', 'refunded'))
);

CREATE UNIQUE INDEX billing_group_orders_unique_idx ON billing_group_orders(billing_group_id, order_id);
CREATE INDEX billing_group_orders_group_idx ON billing_group_orders(billing_group_id);
CREATE INDEX billing_group_orders_order_idx ON billing_group_orders(order_id);
