# Operator 页面字段与必要性审查（第一轮）

> 目标：逐页确认“字段与后端契约对齐、字段展示正常且必要、文案可用于生产”。

## 审查结论汇总

| 页面 | 后端字段对齐 | 字段必要性 | 文案生产化 | 处理状态 |
|---|---|---|---|---|
| `operator/rules` | ✅ | ✅（移除规则 key 的直接展示） | ✅（移除“与小程序一致”） | ✅ 已完成 |
| `operator/appeals` | ✅（待逐字段映射复核） | ⚠️（详情区字段密度高） | ⚠️（存在 pending/approved/rejected 直出） | ⏳ 待改造 |
| `operator/safety` | ✅（待逐字段映射复核） | ⚠️（提交与列表可再分层） | ⚠️（存在 low/medium/high/critical 直出） | ⏳ 待改造 |
| `operator/finance` | ✅ | ✅ | ✅ | ✅ 已完成（Tab化） |
| `operator/applyment` | ✅ | ✅ | ✅ | ✅ 已完成 |

## `operator/rules` 本次修改点

1. 页面结构由三卡片改为两层：
   - 筛选与分类（区域选择 + Tabs）
   - 规则列表（展示与编辑）
2. 删除业务无关字段展示：
   - 不再向运营用户显示 `rule.key`。
3. 文案生产化：
   - 删除“与小程序一致”表述，改为业务导向描述。

## 下一轮优先页面

1. `operator/appeals`
   - 把状态枚举统一映射中文。
   - 按“筛选/列表/详情操作”做Tab或分区，降低单屏复杂度。
2. `operator/safety`
   - 等级枚举映射中文。
   - 提交事件与事件处置分区，避免操作混叠。
3. `operator/realtime` / `operator/regions`
   - 检查是否存在技术字段直接展示给运营角色。
