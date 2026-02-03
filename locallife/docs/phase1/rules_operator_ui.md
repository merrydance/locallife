# Phase1 运营商最小规则配置 UI（草案）

## 目标
提供可用的最小规则配置界面，支持运营人员进行规则创建、发布、灰度、回滚与命中查看，降低手动 SQL 风险。

## 使用角色
- 运营商（operator）：规则配置与灰度发布
- 平台管理员（admin）：全局规则管理与审计

> 现阶段后端接口为平台规则接口（/v1/platform/rules），若运营商角色需要直接使用，需在网关层或新增 operator 代理接口。

## 页面与功能

### 1) 规则列表页
- 列表字段：规则ID、名称、分类、状态、当前版本、更新时间
- 操作：查看详情、创建规则、禁用规则
- 过滤：分类、状态、关键词
- 分页：limit/offset

### 2) 规则详情页
- 展示：规则基础信息 + 版本列表
- 版本列表字段：version、status、priority、effective_at、expires_at
- 操作：创建版本、发布、回滚、禁用

### 3) 规则版本编辑页（创建）
- 表单字段：
  - 基础：version、priority、status
  - scope：order_type / domain / region_id / merchant_id / user_id
  - condition：业务条件（JSON）
  - action：动作（JSON）
  - gray_config：region_id / merchant_id / user_id
  - effective_at / expires_at
- 校验：
  - status 仅支持 draft/published
  - JSON 合法性校验

### 4) 命中记录页（只读）
- 过滤：rule_id / rule_version_id / action / 决策结果 / 时间范围
- 展示：输入、输出、命中规则与版本、决策结果

## 主要交互流程

### 创建与发布
1. 创建规则（draft）
2. 创建规则版本（draft/published）
3. 发布规则（设置 current_version_id 并激活）
4. 观察命中与指标

### 灰度发布
1. 在版本中设置 gray_config
2. 小流量灰度
3. 扩大灰度

### 回滚/禁用
- 回滚到指定版本或自动选择上一发布版本
- 禁用清空 current_version_id

## API 映射
- 规则列表：GET /v1/platform/rules?limit=&offset=
- 规则详情：GET /v1/platform/rules/{id}
- 创建规则：POST /v1/platform/rules
- 创建版本：POST /v1/platform/rules/{id}/versions
- 发布：POST /v1/platform/rules/{id}/publish
- 回滚：POST /v1/platform/rules/{id}/rollback
- 禁用：POST /v1/platform/rules/{id}/disable
- 命中记录：GET /v1/platform/rules/hits

## 数据安全与审计
- 所有写操作必须记录 rule_audits
- 所有修改动作必须记录操作人（actor_id/role）
- 禁止直接修改历史版本的 JSON

## 后续增强
- 可视化条件/动作构建器
- 灰度百分比配置
- 审批流与变更记录对比
