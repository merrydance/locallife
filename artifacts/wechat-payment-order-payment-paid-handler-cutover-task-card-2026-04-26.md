# TASK-PAY-006A 订单支付成功 handler cutover 任务卡

日期：2026-04-26

## 1. 目标

把订单支付成功从总支付处理器迁到 `order_domain` owner handler。支付 callback、query replay、combine paid replay 命中同一 paid fact/application 入口，再由订单域单源推进订单激活与支付后续动作。

本段只处理 `business_type=order` 的 payment paid 主链，不处理 reservation、claim recovery、rider deposit 或分账结果收口。

## 2. 范围

- order partner JSAPI paid callback/query 统一记录 payment fact。
- combine order replay 到子单 paid 时也走同一 `order_domain` application。
- `order_domain` application 负责订单支付成功后的业务推进，保证重复 paid fact 幂等。
- 旧 `ProcessTaskPaymentSuccess` 中 order 分支降为显式跳过或保护，不再拥有生产推进权。
- 支付成功后的分账触发仍由订单域 owner 明确调用，不回流总支付处理器。

## 3. 不在本段处理

- 不迁移 reservation payment paid。
- 不迁移 order/refund terminal fact。
- 不处理 timeout 前补查策略统一化。
- 不处理 claim recovery、rider deposit、profit sharing receiver lifecycle。

## 4. 验收

- order paid callback 重复投递不重复激活订单。
- query replay 命中 paid 时与 callback 命中 paid 走同一 application 入口。
- combine paid replay 到子单时不再额外依赖总支付处理器做业务推进。
- 生产代码中 order payment success 终态推进只剩 `order_domain` owner。

## 5. 验证

- `go -C /home/sam/locallife/locallife test ./logic ./api ./worker -run 'TestHandle(Ecommerce|Partner|Combine)Payment.*Order|TestProcessTaskPaymentSuccess.*Order|TestPaymentFactServiceApplyExternalPaymentFactApplication.*Order' -count=1`

## 6. Review 结论

风险等级：G3。原因是本段触及订单 paid 幂等、支付成功后业务推进、combine replay 与后续分账触发链路。

完成标志不是“能写 paid fact”，而是 order 支付成功的生产推进权已从总支付处理器切到 `order_domain` owner handler。