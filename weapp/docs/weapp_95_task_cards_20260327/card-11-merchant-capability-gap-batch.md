# CARD-11 商户侧能力缺口补齐批次

状态：进行中（KDS、经营统计、店铺资料、会员设置切片代码完成，待人工回归）

优先级：P1

所属阶段：Phase 3

## 问题目标

按现有矩阵文档补齐商户侧缺口能力，把商户端提升到完整经营工作台水平。

## 影响范围

- [weapp/docs/merchant/MERCHANT_BACKEND_ALIGNMENT_MATRIX_2026-03-26.md](weapp/docs/merchant/MERCHANT_BACKEND_ALIGNMENT_MATRIX_2026-03-26.md)
- [weapp/docs/merchant/MERCHANT_PAGE_GAP_AND_IMPLEMENTATION_CHECKLIST_2026-03-26.md](weapp/docs/merchant/MERCHANT_PAGE_GAP_AND_IMPLEMENTATION_CHECKLIST_2026-03-26.md)
- `weapp/miniprogram/pages/merchant/**`

## 任务内容

- [ ] 补齐 KDS、投诉、员工、统计、设置域缺口页。
- [ ] 重构 config 为真正的设置导航页。
- [ ] 收口 finance、claims、appeals、reservations 的状态与动作闭环。

## 完成定义

- [ ] 商户侧主要后端能力都有明确页面落点。
- [ ] 高风险业务动作全部可用且状态可信。

## 验证要求

- [ ] 对照矩阵做一轮能力反查。
- [ ] 执行 merchant 相关最小校验与人工回归。

## 完成记录

- [x] 首个切片完成：新增 `merchant/kitchen` 后厨看板页，并接入工作台与配置中心入口
- [x] 已接通后厨订单列表、开始制作、标记出餐三条主动作
- [x] 已补齐页面级 loading、error、empty、retry 与动作中状态
- [x] 第二个切片完成：新增 `merchant/stats` 经营统计页，并接入工作台与配置中心入口
- [x] 已接通经营概览、订单统计、热销菜、预订池概览四组真实数据
- [x] 第三个切片完成：新增 `merchant/settings/profile` 店铺资料页，并接通商户资料读取、校验、保存与配置中心入口
- [x] 已补齐店铺资料编辑态、保存中、保存成功失败反馈，以及到门店图片/经营分类/会员设置的导航
- [x] 第四个切片完成：新增 `merchant/settings/membership` 会员设置页，并接通会员叠加规则、抵扣上限、适用场景读取与保存
- [x] 已补齐会员设置 loading、error、dirty、retry、保存中状态
- [ ] KDS 人工回归完成
- [ ] 经营统计人工回归完成
- [ ] 店铺资料人工回归完成
- [ ] 会员设置人工回归完成
- [ ] 投诉、员工、统计、设置域缺口页完成
- [ ] 缺口页完成
- [ ] 反查完成
- [ ] 回归完成