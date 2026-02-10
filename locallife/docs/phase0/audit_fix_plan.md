# 审计问题修复方案

> 生成时间: 2026-02-09 状态: **部分实施完成** 最后更新: 2026-02-09

## 已完成修复列表

| 编号   | 问题                                           | 状态    |
| ------ | ---------------------------------------------- | ------- |
| P1-007 | `adjustMemberBalance` 余额变动与流水记录非原子 | ✅ 完成 |
| P1-019 | `urgeOrder` 缺失速率限制                       | ✅ 完成 |
| P1-028 | `CancelOrderTx` 库存恢复未按 dish_id 排序      | ✅ 完成 |
| P1-003 | `grabOrder` 缺失物理距离校验                   | ✅ 完成 |
| P1-005 | `confirmDelivery` 缺失地理围栏校验             | ✅ 完成 |

## 目录

1. [高优先级 - 资金安全](#高优先级---资金安全)
2. [中优先级 - 数据一致性](#中优先级---数据一致性)
3. [中优先级 - 安全防护](#中优先级---安全防护)
4. [低优先级 - 敏感信息](#低优先级---敏感信息)

---

## 高优先级 - 资金安全

### P1-007: `adjustMemberBalance` 余额变动与流水记录非原子

**问题**: `api/membership.go` 中 `IncrementMembershipBalance` 和
`CreateMembershipTransaction` 分开调用，若流水创建失败，余额已变更但无记录。

**修复方案**:

1. 创建新事务函数 `db/sqlc/tx_membership.go`:

```go
// AdjustMemberBalanceTxParams 调整余额事务参数
type AdjustMemberBalanceTxParams struct {
    MembershipID int64
    Amount       int64  // 正数增加，负数减少
    Type         string // recharge/consume/adjust/refund
    Notes        string
    RelatedOrderID pgtype.Int8
}

// AdjustMemberBalanceTxResult 调整余额事务结果
type AdjustMemberBalanceTxResult struct {
    Membership  MerchantMembership
    Transaction MembershipTransaction
}

func (store *SQLStore) AdjustMemberBalanceTx(ctx context.Context, arg AdjustMemberBalanceTxParams) (AdjustMemberBalanceTxResult, error) {
    var result AdjustMemberBalanceTxResult

    err := store.execTx(ctx, func(q *Queries) error {
        // 1. 锁定会员记录
        membership, err := q.GetMembershipForUpdate(ctx, arg.MembershipID)
        if err != nil {
            return fmt.Errorf("get membership for update: %w", err)
        }

        // 2. 计算新余额
        newBalance := membership.Balance + arg.Amount
        if newBalance < 0 {
            return fmt.Errorf("余额不足")
        }

        // 3. 更新余额
        var totalRecharged, totalConsumed int64
        if arg.Amount > 0 {
            totalRecharged = membership.TotalRecharged + arg.Amount
            totalConsumed = membership.TotalConsumed
        } else {
            totalRecharged = membership.TotalRecharged
            totalConsumed = membership.TotalConsumed + (-arg.Amount)
        }

        result.Membership, err = q.UpdateMembershipBalance(ctx, UpdateMembershipBalanceParams{
            ID:             arg.MembershipID,
            Balance:        newBalance,
            TotalRecharged: totalRecharged,
            TotalConsumed:  totalConsumed,
        })
        if err != nil {
            return fmt.Errorf("update membership balance: %w", err)
        }

        // 4. 创建流水记录
        result.Transaction, err = q.CreateMembershipTransaction(ctx, CreateMembershipTransactionParams{
            MembershipID:   arg.MembershipID,
            Type:           arg.Type,
            Amount:         arg.Amount,
            BalanceAfter:   newBalance,
            RelatedOrderID: arg.RelatedOrderID,
            Notes:          pgtype.Text{String: arg.Notes, Valid: arg.Notes != ""},
        })
        if err != nil {
            return fmt.Errorf("create membership transaction: %w", err)
        }

        return nil
    })

    return result, err
}
```

2. 修改 `api/membership.go` 中 `adjustMemberBalance` 调用新事务

**影响评估**: 无业务影响，仅增强原子性

---

### P1-020: `replaceOrder` 跨表操作非原子

**问题**: `api/order.go` 中创建新订单和标记旧订单为替换状态分开执行。

**修复方案**:

1. 创建 `db/sqlc/tx_order_replace.go`:

```go
type ReplaceOrderTxParams struct {
    OldOrderID   int64
    NewOrderArgs CreateOrderTxParams
    OperatorID   int64
    OperatorType string
}

type ReplaceOrderTxResult struct {
    OldOrder Order
    NewOrder Order
}

func (store *SQLStore) ReplaceOrderTx(ctx context.Context, arg ReplaceOrderTxParams) (ReplaceOrderTxResult, error) {
    var result ReplaceOrderTxResult

    err := store.execTx(ctx, func(q *Queries) error {
        // 1. 锁定并验证旧订单
        oldOrder, err := q.GetOrderForUpdate(ctx, arg.OldOrderID)
        if err != nil {
            return fmt.Errorf("get old order: %w", err)
        }
        if oldOrder.Status != OrderStatusPaid {
            return fmt.Errorf("只有已支付订单可以改单")
        }

        // 2. 创建新订单 (复用 CreateOrderTx 逻辑)
        // ... 创建新订单逻辑 ...

        // 3. 标记旧订单为已替换
        result.OldOrder, err = q.MarkOrderReplaced(ctx, MarkOrderReplacedParams{
            ID:            arg.OldOrderID,
            ReplacedByID:  pgtype.Int8{Int64: result.NewOrder.ID, Valid: true},
        })
        if err != nil {
            return fmt.Errorf("mark order replaced: %w", err)
        }

        return nil
    })

    return result, err
}
```

**影响评估**: 需要重构 `replaceOrder` API，但不影响现有订单

---

### P1-019: `urgeOrder` 缺失速率限制

**问题**: 催单接口无频率限制，可被恶意刷接口。

**修复方案**:

1. 添加 Redis 速率限制器 `util/rate_limiter.go`:

```go
const (
    UrgeOrderRateLimitKey    = "urge_order:user:%d:order:%d"
    UrgeOrderRateLimitWindow = 5 * time.Minute
    UrgeOrderRateLimitMax    = 3
)

func (server *Server) checkUrgeOrderRateLimit(ctx context.Context, userID, orderID int64) error {
    key := fmt.Sprintf(UrgeOrderRateLimitKey, userID, orderID)
    
    count, err := server.redisClient.Incr(ctx, key).Result()
    if err != nil {
        return err
    }
    
    if count == 1 {
        server.redisClient.Expire(ctx, key, UrgeOrderRateLimitWindow)
    }
    
    if count > UrgeOrderRateLimitMax {
        return errors.New("催单过于频繁，请5分钟后再试")
    }
    
    return nil
}
```

2. 在 `urgeOrder` 开头添加检查:

```go
func (server *Server) urgeOrder(ctx *gin.Context) {
    // ... 参数解析 ...

    // 速率限制检查
    if err := server.checkUrgeOrderRateLimit(ctx, authPayload.UserID, req.OrderID); err != nil {
        ctx.JSON(http.StatusTooManyRequests, errorResponse(err))
        return
    }
    
    // ... 原有逻辑 ...
}
```

**影响评估**: 正常用户每5分钟最多3次催单，不影响正常使用

---

## 中优先级 - 数据一致性

### P1-028: `CancelOrderTx` 库存恢复未按 dish_id 排序

**问题**: 循环恢复库存时未排序，高并发可能死锁。

**修复方案**:

修改 `db/sqlc/tx_order_status.go`:

```go
func (store *SQLStore) CancelOrderTx(ctx context.Context, arg CancelOrderTxParams) (CancelOrderTxResult, error) {
    // ...
    
    if arg.OldStatus == OrderStatusPaid {
        orderItems, err = q.ListOrderItemsByOrder(ctx, arg.OrderID)
        if err != nil {
            return fmt.Errorf("list order items: %w", err)
        }

        // 关键修复：按 dish_id 排序，避免死锁
        sort.Slice(orderItems, func(i, j int) bool {
            return orderItems[i].DishID.Int64 < orderItems[j].DishID.Int64
        })

        for _, item := range orderItems {
            // ... 库存恢复逻辑 ...
        }
    }
    // ...
}
```

**影响评估**: 纯防御性编程，无业务影响

---

### P1-009: `createCombinedPaymentOrder` 子单操作非原子

**问题**: 多个子支付订单创建分开执行，部分失败会导致数据不一致。

**修复方案**:

创建 `db/sqlc/tx_combined_payment.go`:

```go
type CreateCombinedPaymentTxParams struct {
    UserID      int64
    OrderInfos  []CombinedOrderInfo
    ExpiresAt   time.Time
}

func (store *SQLStore) CreateCombinedPaymentTx(ctx context.Context, arg CreateCombinedPaymentTxParams) (CombinedPaymentTxResult, error) {
    var result CombinedPaymentTxResult

    err := store.execTx(ctx, func(q *Queries) error {
        // 1. 创建合单支付主记录
        combineOutTradeNo := generateOutTradeNoWithPrefix("CP")
        combinedPayment, err := q.CreateCombinedPaymentOrder(ctx, ...)
        if err != nil {
            return err
        }

        // 2. 批量创建子订单 (全部在同一事务)
        for _, info := range arg.OrderInfos {
            outTradeNo := generateOutTradeNoWithPrefix("C")
            
            paymentOrder, err := q.CreatePaymentOrder(ctx, ...)
            if err != nil {
                return err
            }

            _, err = q.CreateCombinedPaymentSubOrder(ctx, ...)
            if err != nil {
                return err
            }
        }

        return nil
    })

    return result, err
}
```

**影响评估**: 需要重构 API 调用，但保证了合单支付的原子性

---

## 中优先级 - 安全防护

### P1-003: `grabOrder` 缺失物理距离校验

**问题**: 骑手可从任意位置抢单，无距离限制。

**修复方案**:

1. 定义常量 `util/constants.go`:

```go
const (
    MaxGrabOrderDistanceMeters = 5000  // 最大抢单距离 5km
)
```

2. 修改 `api/delivery.go` 中 `grabOrder`:

```go
func (server *Server) grabOrder(ctx *gin.Context) {
    // ... 获取骑手位置 ...
    
    // 计算骑手与商户的距离
    merchantLat, _ := merchant.Latitude.Float64Value()
    merchantLng, _ := merchant.Longitude.Float64Value()
    
    distance := util.HaversineDistance(
        req.RiderLatitude, req.RiderLongitude,
        merchantLat.Float64, merchantLng.Float64,
    )
    
    if distance > util.MaxGrabOrderDistanceMeters {
        ctx.JSON(http.StatusBadRequest, errorResponse(
            fmt.Errorf("距离商户%.1fkm，超出抢单范围(%.1fkm)", 
                distance/1000, util.MaxGrabOrderDistanceMeters/1000),
        ))
        return
    }
    
    // ... 继续抢单逻辑 ...
}
```

**影响评估**: 新增必填参数 `rider_latitude`、`rider_longitude`，需客户端配合

---

### P1-005: `confirmDelivery` 缺失地理围栏校验

**问题**: 骑手可在任意位置确认送达，无位置验证。

**修复方案**:

1. 定义常量:

```go
const (
    DeliveryConfirmRadiusMeters = 200  // 确认送达半径 200m
)
```

2. 修改 `api/delivery.go` 中 `confirmDelivery`:

```go
type confirmDeliveryRequest struct {
    ID            int64   `json:"id" binding:"required"`
    RiderLatitude float64 `json:"rider_latitude" binding:"required"`
    RiderLongitude float64 `json:"rider_longitude" binding:"required"`
}

func (server *Server) confirmDelivery(ctx *gin.Context) {
    // ... 获取配送单 ...
    
    // 地理围栏校验
    deliveryLat, _ := delivery.DeliveryLatitude.Float64Value()
    deliveryLng, _ := delivery.DeliveryLongitude.Float64Value()
    
    distance := util.HaversineDistance(
        req.RiderLatitude, req.RiderLongitude,
        deliveryLat.Float64, deliveryLng.Float64,
    )
    
    if distance > util.DeliveryConfirmRadiusMeters {
        // 记录异常但不阻断（业务需求：可能用户下楼取餐）
        log.Warnf("骑手确认送达时距离收货点%.0fm，超出围栏范围", distance)
        
        // 可选：标记为异常配送，后续风控审查
        _ = server.store.MarkDeliveryLocationAbnormal(ctx, delivery.ID)
    }
    
    // ... 继续确认逻辑 ...
}
```

**影响评估**: 新增必填参数，记录异常但不阻断业务

---

### P1-012: `renewAccessToken` 无会话锁

**问题**: 并发刷新可能导致多个有效token。

**修复方案**:

1. 新增 SQL `db/query/session.sql`:

```sql
-- name: GetSessionByRefreshTokenForUpdate :one
SELECT * FROM sessions
WHERE refresh_token = $1
LIMIT 1
FOR UPDATE;
```

2. 创建事务 `db/sqlc/tx_session.go`:

```go
type RefreshTokenTxParams struct {
    RefreshToken string
    UserAgent    string
    ClientIP     string
}

func (store *SQLStore) RefreshTokenTx(ctx context.Context, arg RefreshTokenTxParams) (RefreshTokenTxResult, error) {
    var result RefreshTokenTxResult

    err := store.execTx(ctx, func(q *Queries) error {
        // 1. 加锁获取会话
        session, err := q.GetSessionByRefreshTokenForUpdate(ctx, arg.RefreshToken)
        if err != nil {
            return fmt.Errorf("session not found: %w", err)
        }

        // 2. 验证会话有效性
        if session.IsBlocked {
            return errors.New("session is blocked")
        }
        if time.Now().After(session.ExpiresAt) {
            return errors.New("session expired")
        }

        // 3. 生成新 token
        newAccessToken, accessPayload, err := maker.CreateToken(...)
        newRefreshToken, refreshPayload, err := maker.CreateToken(...)

        // 4. 更新会话（原子替换 refresh_token）
        result.Session, err = q.UpdateSession(ctx, UpdateSessionParams{
            ID:           session.ID,
            RefreshToken: newRefreshToken,
            ExpiresAt:    refreshPayload.ExpiredAt,
            UserAgent:    arg.UserAgent,
            ClientIP:     arg.ClientIP,
        })

        result.AccessToken = newAccessToken
        result.RefreshToken = newRefreshToken
        return nil
    })

    return result, err
}
```

**影响评估**: 需要重新生成 sqlc 代码，API 逻辑重构

---

### P1-029: 预订取消时状态变更与库存释放非原子

**问题**: `cancelReservation` 中状态更新和 `ReleaseReservationInventoryTx`
分开执行。

**修复方案**:

合并为单一事务 `db/sqlc/tx_reservation.go`:

```go
func (store *SQLStore) CancelReservationTx(ctx context.Context, arg CancelReservationTxParams) error {
    return store.execTx(ctx, func(q *Queries) error {
        // 1. 锁定并更新预订状态
        reservation, err := q.GetTableReservationForUpdate(ctx, arg.ReservationID)
        if err != nil {
            return err
        }

        _, err = q.UpdateReservationStatus(ctx, UpdateReservationStatusParams{
            ID:     arg.ReservationID,
            Status: "cancelled",
        })
        if err != nil {
            return err
        }

        // 2. 释放库存（同一事务内）
        reservationItems, err := q.ListReservationDishes(ctx, arg.ReservationID)
        if err != nil {
            return err
        }

        for _, item := range reservationItems {
            _, err = q.DecrementReservedInventory(ctx, DecrementReservedInventoryParams{
                DishID:   item.DishID,
                Date:     reservation.ReservationDate,
                Quantity: item.Quantity,
            })
            if err != nil {
                return err
            }
        }

        return nil
    })
}
```

**影响评估**: 保证预订取消的原子性

---

## 低优先级 - 敏感信息

### P2-001 & P2-002: 敏感信息明文存储

**问题**:

- `merchant_applications.legal_person_id_number` 明文存储身份证号
- `rider_applications.id_card_ocr` 明文存储 OCR 结果

**修复方案**:

1. 创建加密工具 `util/crypto.go`:

```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "io"
)

var sensitiveDataKey []byte // 从环境变量加载

func init() {
    key := os.Getenv("SENSITIVE_DATA_KEY")
    if len(key) != 32 {
        panic("SENSITIVE_DATA_KEY must be 32 bytes")
    }
    sensitiveDataKey = []byte(key)
}

// EncryptSensitiveData AES-GCM 加密敏感数据
func EncryptSensitiveData(plaintext string) (string, error) {
    block, err := aes.NewCipher(sensitiveDataKey)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }

    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptSensitiveData AES-GCM 解密敏感数据
func DecryptSensitiveData(encrypted string) (string, error) {
    data, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        return "", err
    }

    block, err := aes.NewCipher(sensitiveDataKey)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize {
        return "", errors.New("ciphertext too short")
    }

    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    return string(plaintext), err
}
```

2. 数据迁移脚本 `db/migration/encrypt_sensitive_data.sql`:

```sql
-- 添加加密字段（保留原字段用于迁移）
ALTER TABLE merchant_applications 
    ADD COLUMN legal_person_id_number_encrypted TEXT;

ALTER TABLE rider_applications 
    ADD COLUMN id_card_ocr_encrypted TEXT;

-- 迁移完成后删除原字段
-- ALTER TABLE merchant_applications DROP COLUMN legal_person_id_number;
-- ALTER TABLE rider_applications DROP COLUMN id_card_ocr;
```

3. 修改存储逻辑:

```go
// 存储时加密
encryptedID, _ := util.EncryptSensitiveData(legalPersonIDNumber)
_, err := store.CreateMerchantApplication(ctx, CreateMerchantApplicationParams{
    LegalPersonIDNumberEncrypted: encryptedID,
    // ...
})

// 读取时解密
decryptedID, _ := util.DecryptSensitiveData(app.LegalPersonIDNumberEncrypted)
```

**影响评估**: 需要数据迁移，历史数据需加密处理

---

## 业务逻辑问题

### P1-008: 赠额与本金拆分逻辑不完整

**问题**: `MerchantMembership` 只有 `Balance`，无法区分本金和赠额。

**修复方案**:

1. 数据库迁移:

```sql
ALTER TABLE merchant_memberships 
    ADD COLUMN principal_balance BIGINT DEFAULT 0,
    ADD COLUMN gift_balance BIGINT DEFAULT 0;

-- 迁移现有数据（全部视为本金）
UPDATE merchant_memberships 
SET principal_balance = balance, gift_balance = 0;
```

2. 修改扣款逻辑 `db/sqlc/tx_membership.go`:

```go
// DeductMembershipBalanceTx 按场景扣款
func (store *SQLStore) DeductMembershipBalanceTx(ctx context.Context, arg DeductMembershipBalanceTxParams) error {
    return store.execTx(ctx, func(q *Queries) error {
        membership, err := q.GetMembershipForUpdate(ctx, arg.MembershipID)
        if err != nil {
            return err
        }

        // 获取使用场景配置
        settings, _ := q.GetMembershipSettings(ctx, membership.MerchantID)
        
        amountToDeduct := arg.Amount
        var deductFromPrincipal, deductFromGift int64

        // 根据场景决定是否可用赠额
        if slices.Contains(settings.BonusUsableScenes, arg.Scene) {
            // 优先使用赠额
            if membership.GiftBalance >= amountToDeduct {
                deductFromGift = amountToDeduct
                deductFromPrincipal = 0
            } else {
                deductFromGift = membership.GiftBalance
                deductFromPrincipal = amountToDeduct - deductFromGift
            }
        } else {
            // 只能用本金
            deductFromPrincipal = amountToDeduct
        }

        if membership.PrincipalBalance < deductFromPrincipal {
            return errors.New("余额不足")
        }

        _, err = q.UpdateMembershipBalanceSplit(ctx, UpdateMembershipBalanceSplitParams{
            ID:               membership.ID,
            PrincipalBalance: membership.PrincipalBalance - deductFromPrincipal,
            GiftBalance:      membership.GiftBalance - deductFromGift,
        })
        return err
    })
}
```

**影响评估**: 需要数据迁移，修改充值和消费逻辑

---

### P1-010: 申诉有效期窗口未实现

**问题**: 申诉没有时间窗口限制。

**修复方案**:

1. 添加常量:

```go
const (
    AppealWindowDays = 7  // 申诉有效期 7 天
)
```

2. 修改 `api/appeal.go`:

```go
func (server *Server) createMerchantAppeal(ctx *gin.Context) {
    // ... 获取索赔记录 ...

    // 检查申诉窗口期
    appealDeadline := claimRecovery.CreatedAt.Add(time.Duration(AppealWindowDays) * 24 * time.Hour)
    if time.Now().After(appealDeadline) {
        ctx.JSON(http.StatusBadRequest, errorResponse(
            fmt.Errorf("申诉窗口期已过（%d天内可申诉）", AppealWindowDays),
        ))
        return
    }

    // ... 继续申诉逻辑 ...
}
```

---

### P1-023: 签到窗口硬编码

**问题**: 30 分钟签到窗口为魔法数字。

**修复方案**:

```go
// util/constants.go
const (
    ReservationCheckInEarlyMinutes = 30  // 可提前签到时间
    ReservationCheckInLateMinutes  = 30  // 迟到容忍时间
)

// api/table_reservation.go
func (server *Server) checkInReservation(ctx *gin.Context) {
    // ...
    
    now := time.Now()
    earlyLimit := reservation.ReservationTime.Add(-time.Duration(util.ReservationCheckInEarlyMinutes) * time.Minute)
    lateLimit := reservation.ReservationTime.Add(time.Duration(util.ReservationCheckInLateMinutes) * time.Minute)
    
    if now.Before(earlyLimit) {
        ctx.JSON(http.StatusBadRequest, errorResponse(
            fmt.Errorf("尚未到签到时间，请在预订时间前%d分钟内签到", util.ReservationCheckInEarlyMinutes),
        ))
        return
    }
    
    if now.After(lateLimit) {
        ctx.JSON(http.StatusBadRequest, errorResponse(
            fmt.Errorf("已超过签到时间%d分钟", util.ReservationCheckInLateMinutes),
        ))
        return
    }
    
    // ...
}
```

---

## 系统架构问题

### P1-043: Scheduler 无分布式锁

**问题**: 多实例部署时定时任务会重复执行。

**修复方案**:

1. 使用 Redis 分布式锁 `scheduler/lock.go`:

```go
import "github.com/go-redsync/redsync/v4"

type DistributedScheduler struct {
    rs        *redsync.Redsync
    scheduler *Scheduler
}

func (ds *DistributedScheduler) RunWithLock(taskName string, fn func() error) error {
    mutex := ds.rs.NewMutex(
        fmt.Sprintf("scheduler:lock:%s", taskName),
        redsync.WithExpiry(5*time.Minute),
        redsync.WithTries(1),  // 不重试，抢不到就放弃
    )

    if err := mutex.Lock(); err != nil {
        // 其他实例已获取锁，跳过
        return nil
    }
    defer mutex.Unlock()

    return fn()
}
```

2. 修改 `scheduler/manager.go`:

```go
func (m *Manager) scheduleDataCleanup() {
    m.cron.AddFunc("@every 10m", func() {
        _ = m.distributedScheduler.RunWithLock("data_cleanup", func() error {
            return m.cleanupStaleDeliveries(context.Background())
        })
    })
}
```

---

## 实施优先级

| 优先级 | 问题编号       | 预估工时 | 影响范围   |
| ------ | -------------- | -------- | ---------- |
| P0     | P1-007, P1-020 | 2天      | 资金安全   |
| P0     | P1-019         | 0.5天    | 安全防护   |
| P1     | P1-028, P1-009 | 2天      | 数据一致性 |
| P1     | P1-003, P1-005 | 1天      | 业务风控   |
| P1     | P1-012, P1-029 | 1天      | 并发安全   |
| P2     | P1-008         | 3天      | 业务功能   |
| P2     | P1-010, P1-023 | 0.5天    | 业务逻辑   |
| P2     | P2-001, P2-002 | 2天      | 合规要求   |
| P3     | P1-043         | 1天      | 架构优化   |

**总预估工时**: 13 天

---

## 实施步骤

### 第一阶段（1周）- 资金安全

1. 实现 `AdjustMemberBalanceTx`
2. 实现 `ReplaceOrderTx`
3. 添加催单速率限制
4. 库存恢复排序修复

### 第二阶段（1周）- 安全防护

1. 抢单距离校验
2. 送达地理围栏
3. Token 刷新事务化
4. 预订取消事务化
5. 合单支付事务化

### 第三阶段（1周）- 合规与优化

1. 敏感数据加密迁移
2. 会员余额拆分
3. 申诉/签到窗口常量化
4. 分布式调度锁

---

## 测试要求

每个修复需要：

1. 单元测试覆盖正常和异常路径
2. 并发测试验证竞态条件
3. 集成测试验证端到端流程
4. 线上灰度发布
