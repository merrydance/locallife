# 移除千人千面推荐 + 搜索推荐区域过滤修复计划（2026-01-19）

## 目标
1) 完全移除“千人千面推荐”相关代码/逻辑与对外接口。
2) 修复“搜索排序推荐列表缺少显式区域过滤，跨区商户可能被远距离高销量穿透”的问题。

## 关联代码清单（已审查）
### 千人千面推荐（需移除）
- API 入口与逻辑：
  - [api/recommendation.go](api/recommendation.go)
  - [api/server.go](api/server.go)
- 算法实现：
  - [algorithm/personalized_recommender.go](algorithm/personalized_recommender.go)
- SQL 查询与模型：
  - [db/query/recommendation.sql](db/query/recommendation.sql)
  - [db/sqlc/recommendation.sql.go](db/sqlc/recommendation.sql.go)
  - [db/sqlc/querier.go](db/sqlc/querier.go)（接口暴露）
  - [db/mock/store.go](db/mock/store.go)（Mock Store）
  - [db/sqlc/recommendation_test.go](db/sqlc/recommendation_test.go)
- 测试与权限：
  - [api/recommendation_test.go](api/recommendation_test.go)
  - [api/main_test.go](api/main_test.go)
  - [api/casbin_enforcer_test.go](api/casbin_enforcer_test.go)
  - [casbin/policy.csv](casbin/policy.csv)
- 文档与对外契约：
  - [docs/swagger.yaml](docs/swagger.yaml)
  - [docs/swagger.json](docs/swagger.json)
  - [docs/docs.go](docs/docs.go)
  - [doc/api/recommendation_engine.md](doc/api/recommendation_engine.md)
  - [doc/api/flows/regional_oversight.md](doc/api/flows/regional_oversight.md)
- 小程序端接口封装：
  - [weapp/miniprogram/api/search-recommendation.ts](weapp/miniprogram/api/search-recommendation.ts)
  - [weapp/miniprogram/api/search-recommendation.js](weapp/miniprogram/api/search-recommendation.js)
  - [weapp/miniprogram/api/search.js](weapp/miniprogram/api/search.js)
- 数据库表（推荐相关）：
  - [db/migration/000026_add_recommendations.up.sql](db/migration/000026_add_recommendations.up.sql)
  - [db/migration/000026_add_recommendations.down.sql](db/migration/000026_add_recommendations.down.sql)

### 搜索排序推荐（需修复区域过滤）
- API：
  - [api/search.go](api/search.go)
- SQL：
  - [db/query/dish.sql](db/query/dish.sql)
  - [db/query/merchant.sql](db/query/merchant.sql)
  - [db/query/combo.sql](db/query/combo.sql)

## 执行计划（完成一项勾选一项）
1. [x] 移除千人千面推荐 API/算法与路由
  - 删除推荐相关 handler 与路由
  - 禁用个性化算法实现（避免参与构建）

2. [x] 移除推荐相关 SQLC 查询与测试
  - 清空 recommendation.sql 查询
  - 通过 sqlc regenerate 同步生成代码（避免手改生成文件）

3. [x] 移除权限、文档与小程序端推荐接口
  - 删除 Casbin 权限与 API 文档中的推荐入口
  - 将小程序推荐 API 替换为搜索排序列表

4. [x] 调整迁移：移除推荐相关表的创建/下线迁移
  - 删除 000100_remove_recommendations.*
  - 000026_add_recommendations.* 置空保留

5. [x] 修复搜索排序推荐的区域过滤
   - SearchDishesGlobal / SearchMerchants / SearchCombosGlobal 增加 region_id 过滤
   - API 增加 region_id 可选参数并透传

## 备注
- 搜索排序推荐仍保留，作为区域平台“推荐列表”主实现。
- 千人千面推荐相关接口移除后，将不再对外暴露 /v1/recommendations/* 与 /v1/behaviors/track。