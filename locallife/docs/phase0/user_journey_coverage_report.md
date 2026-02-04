# 文档端点覆盖审计报告

- doc: `docs/phase0/user_journey_mermaid.md`
- api: `api`（扫描 @Router 注解 + api/server.go 路由注册）
- doc endpoints: 65
- implemented paths: 531

## 结论

- ✅ 文档引用的所有 `/v1/...` 端点在代码中都能找到对应实现（`@Router` 注解或 `api/server.go` 路由注册）。

## 备注

- 这是**文档 → 代码**的单向约束：确保文档里提到的端点不会‘空跑’。
- 若需要反向约束（代码新增端点必须补文档），可以在后续版本加白名单/标签体系。
