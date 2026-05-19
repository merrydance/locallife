# 后端越权与幂等修复任务索引

日期：2026-05-19
范围：仅 `locallife/` 后端。
风险等级：G3。原因：涉及 OCR 私有媒体、证件/申请单归属、运营商对象级授权、补差资金流、外部副作用幂等和重复提交。

这是一组可独立接手的任务卡，不是流水账。每张卡都必须能在失去上下文后单独执行：先读卡，再读卡里点名的源文件，再改代码，再补测试，再跑验证。

任务卡列表：

1. [TASK-OCR-AUTHZ-001](./backend-ocr-authz-task-card-2026-05-19.md)
2. [TASK-SUBSIDY-AUTHZ-001](./backend-subsidy-authz-task-card-2026-05-19.md)（deferred：微信平台收付通支付链路当前停用）
3. [TASK-OCR-IDEMPOTENCY-001](./backend-ocr-idempotency-task-card-2026-05-19.md)
4. [TASK-SUBSIDY-IDEMPOTENCY-001](./backend-subsidy-idempotency-task-card-2026-05-19.md)（deferred：微信平台收付通支付链路当前停用）

当前执行状态：

- 活动任务：OCR 授权、OCR 幂等。
- 暂缓任务：补差授权、补差幂等。微信平台收付通支付链路恢复前，不推进实现。

全局不变式：

- 不得把 `media_asset_id`、`merchant_id`、`user_id` 当作授权来源本身。
- 不得把任意客户端 `idempotency_key` 当作跨用户复用键。
- 不得在没有持久化 claim、条件更新、唯一约束或事务边界的前提下发起外部副作用。
- 对外返回的拒绝/冲突文案必须稳定、中文、可理解，不得透出 raw provider、SQL、driver 或堆栈细节。
- 任何新增 SQL / migration / sqlc 变更都必须同步跑 `make sqlc` 和 `make check-generated`。

单卡执行规则：

- 一次只做一张卡。
- 卡内如果发现需要 SQL / migration / sqlc 改动，先把这张卡闭合，再继续同卡的 API 改动，不要顺手把下一张卡也带上。
- 如果卡片里的前提和代码现状不一致，先修正卡片，不要靠记忆补。
