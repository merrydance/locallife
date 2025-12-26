-- 为骑手身份证号添加唯一约束
-- 业务规则：一个身份证只能注册一个骑手账号
CREATE UNIQUE INDEX riders_id_card_no_idx ON riders(id_card_no);
