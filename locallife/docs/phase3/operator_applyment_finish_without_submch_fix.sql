-- 目的：修复“显示开户成功(finish/active)但未落库 sub_mch_id”的历史脏数据
-- 适用范围：operator 进件数据
-- 执行建议：先在测试库演练；生产库务必在低峰期执行，并保留事务回滚窗口

BEGIN;

-- =============================================================
-- 1) 只读检查：operator 进件记录中，status=finish 但 sub_mch_id 为空
-- =============================================================
SELECT COUNT(*) AS bad_applyment_count
FROM ecommerce_applyments ea
WHERE ea.subject_type = 'operator'
  AND ea.status = 'finish'
  AND (ea.sub_mch_id IS NULL OR ea.sub_mch_id = '');

SELECT
  ea.id,
  ea.subject_id AS operator_id,
  ea.out_request_no,
  ea.applyment_id,
  ea.status,
  ea.sub_mch_id,
  ea.created_at,
  ea.updated_at
FROM ecommerce_applyments ea
WHERE ea.subject_type = 'operator'
  AND ea.status = 'finish'
  AND (ea.sub_mch_id IS NULL OR ea.sub_mch_id = '')
ORDER BY ea.updated_at DESC
LIMIT 100;

-- =============================================================
-- 2) 只读检查：operator 主表中，status=active 但 sub_mch_id 为空
-- =============================================================
SELECT COUNT(*) AS bad_operator_count
FROM operators o
WHERE o.status = 'active'
  AND (o.sub_mch_id IS NULL OR o.sub_mch_id = '');

SELECT
  o.id,
  o.user_id,
  o.name,
  o.status,
  o.sub_mch_id,
  o.updated_at
FROM operators o
WHERE o.status = 'active'
  AND (o.sub_mch_id IS NULL OR o.sub_mch_id = '')
ORDER BY o.updated_at DESC
LIMIT 100;

-- =============================================================
-- 3) 备份快照（临时表）
-- =============================================================
CREATE TEMP TABLE tmp_bad_operator_applyments AS
SELECT *
FROM ecommerce_applyments ea
WHERE ea.subject_type = 'operator'
  AND ea.status = 'finish'
  AND (ea.sub_mch_id IS NULL OR ea.sub_mch_id = '');

CREATE TEMP TABLE tmp_bad_operators AS
SELECT *
FROM operators o
WHERE o.status = 'active'
  AND (o.sub_mch_id IS NULL OR o.sub_mch_id = '');

-- =============================================================
-- 4) 修复：
--    A. 进件记录：finish -> submitted（无 sub_mch_id 不应视为成功）
--    B. 运营商：active -> pending_bindbank（无 sub_mch_id 不应可经营）
-- =============================================================
UPDATE ecommerce_applyments ea
SET status = 'submitted',
    updated_at = NOW()
WHERE ea.subject_type = 'operator'
  AND ea.status = 'finish'
  AND (ea.sub_mch_id IS NULL OR ea.sub_mch_id = '');

UPDATE operators o
SET status = 'pending_bindbank',
    updated_at = NOW()
WHERE o.status = 'active'
  AND (o.sub_mch_id IS NULL OR o.sub_mch_id = '');

-- =============================================================
-- 5) 复核
-- =============================================================
SELECT COUNT(*) AS remain_bad_applyment_count
FROM ecommerce_applyments ea
WHERE ea.subject_type = 'operator'
  AND ea.status = 'finish'
  AND (ea.sub_mch_id IS NULL OR ea.sub_mch_id = '');

SELECT COUNT(*) AS remain_bad_operator_count
FROM operators o
WHERE o.status = 'active'
  AND (o.sub_mch_id IS NULL OR o.sub_mch_id = '');

-- 复核通过后执行 COMMIT；若不符合预期则 ROLLBACK
-- COMMIT;
-- ROLLBACK;
