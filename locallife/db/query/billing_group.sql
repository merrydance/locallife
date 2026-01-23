-- Billing groups

-- name: CreateBillingGroup :one
INSERT INTO billing_groups (
  dining_session_id,
  status,
  is_default,
  total_amount,
  paid_amount
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetDefaultBillingGroupBySession :one
SELECT * FROM billing_groups
WHERE dining_session_id = $1
  AND is_default = true
LIMIT 1;

-- name: GetBillingGroup :one
SELECT * FROM billing_groups
WHERE id = $1
LIMIT 1;

-- name: ListBillingGroupsBySession :many
SELECT * FROM billing_groups
WHERE dining_session_id = $1
ORDER BY id ASC;

-- Billing group members

-- name: CreateBillingGroupMember :one
INSERT INTO billing_group_members (
  billing_group_id,
  user_id,
  role
) VALUES ($1, $2, $3)
RETURNING *;

-- name: GetActiveBillingGroupMember :one
SELECT * FROM billing_group_members
WHERE billing_group_id = $1
  AND user_id = $2
  AND left_at IS NULL
LIMIT 1;

-- Billing group orders

-- name: CreateBillingGroupOrder :one
INSERT INTO billing_group_orders (
  billing_group_id,
  order_id,
  amount,
  status
) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListBillingGroupOrdersByGroup :many
SELECT * FROM billing_group_orders
WHERE billing_group_id = $1
ORDER BY id ASC;

-- name: UpdateBillingGroupStatus :one
UPDATE billing_groups
SET status = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;
