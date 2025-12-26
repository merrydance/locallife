-- ==========================================
-- 用户余额账户相关查询
-- ==========================================

-- name: GetUserBalance :one
-- 获取用户余额（不存在则返回nil）
SELECT * FROM user_balances
WHERE user_id = $1;

-- name: GetUserBalanceForUpdate :one
-- 获取用户余额（加锁，用于更新）
SELECT * FROM user_balances
WHERE user_id = $1
FOR UPDATE;

-- name: CreateUserBalance :one
-- 创建用户余额账户
INSERT INTO user_balances (user_id, balance, frozen_balance, total_income, total_expense, total_withdraw)
VALUES ($1, 0, 0, 0, 0, 0)
RETURNING *;

-- name: GetOrCreateUserBalance :one
-- 获取或创建用户余额账户（原子操作）
INSERT INTO user_balances (user_id, balance, frozen_balance, total_income, total_expense, total_withdraw)
VALUES ($1, 0, 0, 0, 0, 0)
ON CONFLICT (user_id) DO UPDATE SET updated_at = NOW()
RETURNING *;

-- name: AddUserBalance :one
-- 增加用户余额（入账）
UPDATE user_balances
SET balance = balance + $2,
    total_income = total_income + $2,
    updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: DeductUserBalance :one
-- 扣减用户余额（出账，余额不足会报错）
UPDATE user_balances
SET balance = balance - $2,
    total_expense = total_expense + $2,
    updated_at = NOW()
WHERE user_id = $1 AND balance >= $2
RETURNING *;

-- name: FreezeUserBalance :one
-- 冻结用户余额（提现申请时）
UPDATE user_balances
SET balance = balance - $2,
    frozen_balance = frozen_balance + $2,
    updated_at = NOW()
WHERE user_id = $1 AND balance >= $2
RETURNING *;

-- name: UnfreezeUserBalance :one
-- 解冻用户余额（提现失败时）
UPDATE user_balances
SET balance = balance + $2,
    frozen_balance = frozen_balance - $2,
    updated_at = NOW()
WHERE user_id = $1 AND frozen_balance >= $2
RETURNING *;

-- name: ConfirmUserWithdraw :one
-- 确认提现完成（冻结余额转为已提现）
UPDATE user_balances
SET frozen_balance = frozen_balance - $2,
    total_withdraw = total_withdraw + $2,
    updated_at = NOW()
WHERE user_id = $1 AND frozen_balance >= $2
RETURNING *;

-- ==========================================
-- 用户余额日志相关查询
-- ==========================================

-- name: CreateUserBalanceLog :one
-- 创建余额变动日志
INSERT INTO user_balance_logs (
    user_id, type, amount, balance_before, balance_after,
    related_type, related_id, source_type, source_id, remark
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: ListUserBalanceLogs :many
-- 获取用户余额变动日志
SELECT * FROM user_balance_logs
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListUserBalanceLogsByType :many
-- 按类型获取用户余额变动日志
SELECT * FROM user_balance_logs
WHERE user_id = $1 AND type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetUserBalanceLogByRelated :one
-- 根据关联信息获取日志（用于幂等检查）
SELECT * FROM user_balance_logs
WHERE related_type = $1 AND related_id = $2
LIMIT 1;

-- name: CountUserBalanceLogs :one
-- 统计用户余额变动日志数量
SELECT COUNT(*) FROM user_balance_logs
WHERE user_id = $1;
