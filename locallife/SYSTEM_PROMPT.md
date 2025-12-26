# Go HTTP Scaffold 开发指南

## 架构概述

本项目采用 **HTTP-Only 两层架构**：`Handler → Store → Database`

- **Handler层**: HTTP请求/响应、参数验证、权限检查
- **Store层**: 数据访问 + 事务编排（通过接口暴露，便于mock测试）
- 无独立Service层，简单业务逻辑在Handler，复杂事务在Store的Tx方法

---

## I. 核心设计原则

### 1. 类型安全
- 每个操作使用专用的参数和结果结构体，不用 `map[string]interface{}`
- 响应结构体与数据库模型分离，避免暴露敏感字段
- pgtype字段转换为指针类型 `*string`，使用 `omitempty`

### 2. 错误处理
```go
if err != nil {
    if errors.Is(err, pgx.ErrNoRows) {
        ctx.JSON(http.StatusNotFound, errorResponse(err))
        return
    }
    ctx.JSON(http.StatusInternalServerError, errorResponse(err))
    return
}
```

**HTTP状态码映射**:
- 400: 参数错误 | 401: 认证失败 | 403: 权限不足 | 404: 资源不存在 | 409: 业务冲突 | 500: 服务端错误

### 3. 依赖注入
- Server字段使用接口类型 `store db.Store`
- 构造函数接收所有依赖
- 测试时注入mock实现

---

## II. 数据建模规范

### 命名规范
- **表名**: 复数小写 `users`, `orders`
- **字段名**: snake_case `created_at`, `user_id`
- **主键**: `id bigserial PRIMARY KEY`
- **外键**: `{表名单数}_id` 格式
- **时间戳**: 统一使用 `timestamptz`

### 字段类型
| 用途 | PostgreSQL类型 | Go类型 |
|------|---------------|--------|
| 主键ID | bigserial | int64 |
| 金额 | bigint (分) | int64 |
| 状态枚举 | varchar(20) | string |
| JSON数据 | jsonb | json.RawMessage |
| 时间 | timestamptz | time.Time |
| 可选文本 | varchar NULL | pgtype.Text |

### 必备字段
```sql
CREATE TABLE xxx (
    id bigserial PRIMARY KEY,
    -- 业务字段...
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
```

---

## III. 数据库交互规范

### SQLC 查询规范
```sql
-- name: GetUser :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users ORDER BY id LIMIT $1 OFFSET $2;

-- name: CreateUser :one
INSERT INTO users (username, email) VALUES ($1, $2) RETURNING *;

-- name: UpdateUser :one
UPDATE users SET username = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
```

### 可选参数处理
```sql
-- name: UpdateUserOptional :one
UPDATE users SET
    username = COALESCE(sqlc.narg(username), username),
    email = COALESCE(sqlc.narg(email), email),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
```

### 事务模式
```go
func (store *SQLStore) TransferTx(ctx context.Context, arg TransferTxParams) (TransferTxResult, error) {
    var result TransferTxResult
    err := store.execTx(ctx, func(q *Queries) error {
        // 在事务中执行多个操作
        var err error
        result.FromAccount, err = q.AddAccountBalance(ctx, ...)
        if err != nil { return err }
        result.ToAccount, err = q.AddAccountBalance(ctx, ...)
        return err
    })
    return result, err
}
```

---

## IV. API 开发规范

### Handler 结构
```go
type createUserRequest struct {
    Username string `json:"username" binding:"required,min=3,max=50"`
    Email    string `json:"email" binding:"required,email"`
}

func (server *Server) createUser(ctx *gin.Context) {
    var req createUserRequest
    if err := ctx.ShouldBindJSON(&req); err != nil {
        ctx.JSON(http.StatusBadRequest, errorResponse(err))
        return
    }
    
    user, err := server.store.CreateUser(ctx, db.CreateUserParams{...})
    if err != nil {
        // 错误处理...
    }
    
    ctx.JSON(http.StatusOK, newUserResponse(user))
}
```

### 认证中间件
```go
func (server *Server) authMiddleware() gin.HandlerFunc {
    return func(ctx *gin.Context) {
        authHeader := ctx.GetHeader("Authorization")
        // Bearer Token 验证...
        payload, err := server.tokenMaker.VerifyToken(accessToken)
        ctx.Set(authorizationPayloadKey, payload)
        ctx.Next()
    }
}
```

### 权限检查
```go
authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
if resource.OwnerID != authPayload.UserID {
    ctx.JSON(http.StatusForbidden, errorResponse(errors.New("forbidden")))
    return
}
```

---

## V. 测试规范

### API层测试 (使用mock)
```go
func TestCreateUser(t *testing.T) {
    ctrl := gomock.NewController(t)
    mockStore := mockdb.NewMockStore(ctrl)
    
    mockStore.EXPECT().
        CreateUser(gomock.Any(), gomock.Any()).
        Return(db.User{ID: 1}, nil)
    
    server := newTestServer(t, mockStore)
    // 执行HTTP请求...
}
```

### Store层测试 (使用真实数据库)
```go
func TestCreateUser(t *testing.T) {
    user, err := testStore.CreateUser(context.Background(), CreateUserParams{...})
    require.NoError(t, err)
    require.NotEmpty(t, user.ID)
}
```

---

## VI. 异步任务 (Asynq)

### 任务分发
```go
func (distributor *RedisTaskDistributor) DistributeTaskSendNotification(
    ctx context.Context,
    payload *PayloadSendNotification,
    opts ...asynq.Option,
) error {
    jsonPayload, _ := json.Marshal(payload)
    task := asynq.NewTask(TaskSendNotification, jsonPayload, opts...)
    _, err := distributor.client.EnqueueContext(ctx, task)
    return err
}
```

### 任务处理
```go
func (processor *RedisTaskProcessor) ProcessTaskSendNotification(
    ctx context.Context,
    task *asynq.Task,
) error {
    var payload PayloadSendNotification
    json.Unmarshal(task.Payload(), &payload)
    // 处理逻辑...
    return nil
}
```

---

## VII. 严格禁止

### 🚫 禁止的实现方式
1. **硬编码业务数据** - 不要写死ID、金额、状态值
2. **空实现/TODO** - 不允许 `return nil` 或 `// TODO: implement`
3. **全局变量** - 使用依赖注入
4. **忽略错误** - 必须处理所有 `err != nil`
5. **直接返回DB模型** - 使用响应结构体
6. **裸SQL字符串** - 使用SQLC生成的类型安全方法
7. **在Handler中直接操作数据库** - 必须通过Store接口

### ✅ 必须遵守
1. 所有公开字段添加JSON tag和验证tag
2. 可选字段使用 `pgtype` 或指针类型
3. 金额使用分为单位的int64
4. 时间使用UTC的timestamptz
5. 每个模块有对应的单元测试
6. 数据库变更通过migration管理

---

## VIII. 实现检查清单

```
新增API时:
☐ 定义请求/响应结构体
☐ 添加参数验证tag
☐ 实现Handler函数
☐ 注册路由
☐ 编写单元测试
☐ 更新Swagger注释

新增数据库表时:
☐ 在db.dbml中设计
☐ 创建migration文件
☐ 编写SQLC查询
☐ 运行sqlc generate
☐ 更新Store接口
☐ 重新生成mock

部署前:
☐ 环境变量已配置
☐ 数据库迁移已执行
☐ 所有测试通过
☐ 日志级别为INFO
☐ 敏感信息不泄露
```
