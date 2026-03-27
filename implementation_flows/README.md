# 真实实现流程拆分文档

本目录只基于生产代码实现梳理流程，不把现存文档、注释、评审记录当作事实来源。

使用方法：

1. 先看总览索引，再进入具体流程文档。
2. 每份文档都列出它实际依据的实现文件。
3. 如果代码继续变化，应以这些实现文件为准更新文档，而不是反向依赖旧文档。

当前拆分结果：

- trading-and-fulfillment.md：交易创建、堂食/预订耦合、骑手履约。
- payment-settlement-and-recovery.md：支付创建、回调、支付后置、退款、分账、进件结果。
- claims-appeals-and-recovery.md：索赔、申诉、追偿支付、追偿核销。
- onboarding-media-and-ocr.md：商户/运营商/骑手申请、媒资上传、统一 OCR 作业。
- abnormal-order-main-adjudicator-redesign.md：异常订单全自动主判重构方案，定义新的评分模型、责任矩阵、平台兜底和切换路径。
- abnormal-order-main-adjudicator-rules-matrix.md：异常订单最终主判规则表，冻结 claim 类型、责任域、平台兜底和恶意限制的正式裁决契约。
- abnormal-order-main-adjudicator-data-model-v2.md：异常订单主判数据模型 V2，定义主判落库主表、结构化快照、画像净值账本和迁移顺序。
- abnormal-order-main-adjudicator-master-plan.md：异常订单主判落地总计划，定义全局阶段、依赖顺序、退出条件和后续 phase delivery map 的使用方式。

配套任务卡：

- [abnormal_order_task_cards_20260327/README.md](abnormal_order_task_cards_20260327/README.md)：异常订单主判开发任务卡索引，按阶段拆成可勾选交付项。