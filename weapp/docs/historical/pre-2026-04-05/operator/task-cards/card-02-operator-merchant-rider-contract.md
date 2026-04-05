# CARD-02 运营商户与骑手主链合同修复

状态：进行中

优先级：P0

所属阶段：Phase 1

## 问题目标

把商户与骑手链路补到列表可信、详情完整、动作可回流的可运营状态。

## 影响范围

1. weapp/miniprogram/pages/operator/merchants/index
2. weapp/miniprogram/pages/operator/merchants/detail/index
3. weapp/miniprogram/pages/operator/riders/index
4. weapp/miniprogram/pages/operator/riders/detail/index
5. weapp/miniprogram/api/operator-merchant-management.ts
6. weapp/miniprogram/api/operator-rider-management.ts

## 任务内容

- [x] merchants/index 接 total 或 has_more，不再通过条数猜测 hasMore。
- [x] merchants/index 补齐搜索、筛选、暂停、恢复能力，或明确统一入口。
- [x] merchants/detail/index 去掉 as unknown as，完全对齐 OperatorMerchantDetailResponse 或真实后端新 DTO。
- [x] riders/index 改为统一使用 operatorRiderManagementService。
- [x] riders/index 和 riders/detail/index 补齐暂停、恢复等后端已支持动作。
- [x] riders/detail/index 对齐 OperatorRiderDetailResponse，不再直接 request 自定义字段模型。

## 完成定义

- [ ] 商户、骑手列表的分页、筛选、状态与后端一致。
- [ ] 商户、骑手详情页不再存在 DTO 强转或服务层绕过。
- [ ] 关键动作完成后具备真实刷新回流。

## 验证要求

- [x] 商户列表和骑手列表的编辑器诊断通过。
- [x] 运行 npm run quality:check。
- [ ] 人工回归搜索、筛选、翻页、详情进入、暂停、恢复主链。

## 完成记录

- [x] 商户主链完成
- [x] 骑手主链完成
- [x] 动作回流完成