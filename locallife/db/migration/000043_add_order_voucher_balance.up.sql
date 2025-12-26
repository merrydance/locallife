-- 订单表添加优惠券和会员余额支付字段
-- 用于支持：
-- 1. 用户下单时使用已领取的优惠券抵扣
-- 2. 用户使用会员储值余额支付（部分或全额）

-- 添加优惠券和余额支付字段
ALTER TABLE orders
    ADD COLUMN user_voucher_id BIGINT REFERENCES user_vouchers(id),
    ADD COLUMN voucher_amount BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN balance_paid BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN membership_id BIGINT REFERENCES merchant_memberships(id);

-- 索引：便于查询使用了优惠券/余额的订单
CREATE INDEX orders_user_voucher_id_idx ON orders(user_voucher_id) WHERE user_voucher_id IS NOT NULL;
CREATE INDEX orders_membership_id_idx ON orders(membership_id) WHERE membership_id IS NOT NULL;

-- 约束：优惠券抵扣金额和余额支付不能为负
ALTER TABLE orders
    ADD CONSTRAINT orders_voucher_amount_check CHECK (voucher_amount >= 0),
    ADD CONSTRAINT orders_balance_paid_check CHECK (balance_paid >= 0);

-- 注释
COMMENT ON COLUMN orders.user_voucher_id IS '使用的用户优惠券ID';
COMMENT ON COLUMN orders.voucher_amount IS '优惠券抵扣金额(分)';
COMMENT ON COLUMN orders.balance_paid IS '会员余额支付金额(分)';
COMMENT ON COLUMN orders.membership_id IS '使用的会员卡ID';
