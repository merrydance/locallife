# Phase1 规则存储与版本化（草案）

## 目标
- 规则数据结构化存储，支持版本化与灰度发布。

## 表结构（草案）
- rules：规则主体（name/category/status/current_version）
- rule_versions：规则版本（scope/condition/action/priority/status/gray_config）
- rule_audits：规则变更审计（actor/action/detail）

## 版本策略
- 每次修改生成新版本
- 发布时更新 rules.current_version_id
- 禁用规则不删除历史版本

## 备注
- 本设计为 Phase1 草案，具体字段以迁移文件为准。
