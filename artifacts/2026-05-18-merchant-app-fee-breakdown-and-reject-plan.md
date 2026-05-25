# Merchant App 费用清单与拒单入口实施计划

> **给续跑代理的硬约束：** 本计划只允许手工修改 `merchant_app/` 与本计划文档。可以合并 `origin/main`，因此 `locallife/`、`weapp/`、`web/` 等目录可能因 merge 产生变更，但不得手工编辑后端代码、不得“降级处理”跳过主线契约、不得删除或重写用户已有改动。

## 目标

1. 将主线后端已提供的商户订单 `fee_breakdown` 接入 Flutter 商户 App。
2. 商户端订单金额展示从“订单总额”改为可经营判断的费用清单：
   - 餐费：`fee_breakdown.food_payable_amount`，无费用清单时才回退到旧 `total_amount`。
   - 平台服务费：`fee_breakdown.platform_service_fee_amount`。
   - 支付通道费：`fee_breakdown.payment_channel_fee_amount`。
   - 预计到账：`fee_breakdown.merchant_receivable_amount`。
   - 代取费另列：`fee_breakdown.delivery_payable_amount`，不混入商户餐费。
3. 新订单接单提醒页保留 `稍后处理`，并新增可确认的 `拒单` 次级危险动作。`稍后处理` 只收起提醒，不代表拒单。

## 当前状态与恢复锚点

- 工作目录：`/home/sam/ll-merchant-app`
- 当前分支：`feature/merchant-android-app`
- 远端主线：`origin/main`
- 当前本地改动已临时保存：
  - `stash@{0}`，message: `codex-before-origin-main-merge-2026-05-18`
- 执行顺序：
  1. 确认 `git status --short --branch` 干净。
  2. `git merge origin/main`。
  3. `git stash pop stash@{0}` 恢复本地商户 App 改动。
  4. 只解决 merge/stash 冲突，不做后端手工修补。

## 后端契约来源

主线 `origin/main` 已新增商户订单费用清单：

- `locallife/api/order.go`
  - `orderResponse.FeeBreakdown *merchantOrderFeeBreakdownResponse json:"fee_breakdown,omitempty"`
- `locallife/api/order_response.go`
  - `food_amount`
  - `merchant_discount_amount`
  - `voucher_discount_amount`
  - `food_payable_amount`
  - `delivery_fee_amount`
  - `delivery_fee_discount_amount`
  - `delivery_payable_amount`
  - `customer_payable_amount`
  - `platform_service_fee_amount`
  - `payment_channel_fee_amount`
  - `merchant_receivable_amount`

不要改这些后端字段名。Flutter 只消费和展示。

## 允许修改文件

- `merchant_app/lib/models/order.dart`
- `merchant_app/lib/models/push_message.dart`
- `merchant_app/lib/features/order/order_provider.dart`
- `merchant_app/lib/features/order/order_list_page.dart`
- `merchant_app/lib/features/order/order_detail_page.dart`
- `merchant_app/lib/features/order/order_alert_page.dart`
- `merchant_app/test/order_provider_test.dart`
- `merchant_app/test/order_alert_coordinator_test.dart`
- 必要时新增 `merchant_app/test/order_model_test.dart`
- 本文档

除非验证发现商户 App 内部编译错误必须修复，否则不要扩大修改范围。

## 设计边界

- 状态 owner：`OrderNotifier` 继续负责 API 拉取、接单、拒单、单订单动作 single-flight 和本地订单列表更新。
- 数据模型：在 `OrderModel` 中新增可空 `OrderFeeBreakdown? feeBreakdown`；金额仍使用元为 UI 单位，解析时从后端分转换为元。
- 展示策略：
  - 列表卡片右下主金额显示 `餐费: ¥xx.xx`。
  - 详情状态卡主金额显示 `餐费`，不再称旧总额为商户收入。
  - 详情商品清单底部显示餐费小计与代取费；平台服务费、支付通道费、预计到账放在单独费用卡片或状态卡费用区。
  - 提醒页优先显示餐费；如推送尚未携带费用清单，则通过详情补水后展示。
- 拒单策略：
  - 订单详情已有拒单能力，保留。
  - 新订单提醒页新增 `拒单`，点击后必须确认并填写原因，原因至少 2 个字。
  - 拒单成功后刷新订单列表、toast `已拒绝订单`、关闭提醒。
  - 拒单失败不关闭提醒，toast 使用 `orderProvider.error ?? '拒单失败，请稍后再试'`。

## 任务卡片

### Task 1: 同步主线并恢复本地改动

**文件：**
- Merge 可能触碰全仓库文件，但手工冲突解决只允许在 `merchant_app/` 和本文档内进行。

- [x] Step 1: 确认当前目录干净

```bash
git status --short --branch
```

期望：只有分支信息，无未提交文件。

- [x] Step 2: 合并主线

```bash
git merge origin/main
```

期望：merge 成功；若冲突，记录冲突文件。不要手工编辑 `locallife/`。

- [x] Step 3: 恢复本地改动

```bash
git stash pop stash@{0}
```

期望：恢复 `codex-before-origin-main-merge-2026-05-18` 的商户 App 改动；若冲突，仅解决商户 App 相关冲突。

- [x] Step 4: 检查后端没有手工编辑

```bash
git diff --name-only -- locallife web weapp | sed -n '1,120p'
```

期望：这些目录如有输出，只能来自 `origin/main` merge，不应出现手工冲突修补痕迹。

### Task 2: TDD 接入 `OrderFeeBreakdown`

**文件：**
- Modify: `merchant_app/lib/models/order.dart`
- Modify: `merchant_app/lib/models/push_message.dart`
- Modify: `merchant_app/lib/features/order/order_provider.dart`
- Test: `merchant_app/test/order_alert_coordinator_test.dart`
- Test: `merchant_app/test/order_provider_test.dart`

- [x] Step 1: 写失败测试：`OrderModel.fromJson` 解析 `fee_breakdown`

在 `merchant_app/test/order_alert_coordinator_test.dart` 的 `OrderModel.fromJson` group 增加：

```dart
test('parses merchant fee breakdown from backend cents', () {
  final order = OrderModel.fromJson({
    'id': 501,
    'order_no': 'ORD501',
    'total_amount': 10300,
    'status': 'paid',
    'created_at': '2026-04-12T08:00:00Z',
    'fee_breakdown': {
      'food_amount': 10000,
      'merchant_discount_amount': 300,
      'voucher_discount_amount': 200,
      'food_payable_amount': 9500,
      'delivery_fee_amount': 800,
      'delivery_fee_discount_amount': 0,
      'delivery_payable_amount': 800,
      'customer_payable_amount': 10300,
      'platform_service_fee_amount': 475,
      'payment_channel_fee_amount': 57,
      'merchant_receivable_amount': 8968,
    },
  });

  expect(order.amount, 103.0);
  expect(order.feeBreakdown, isNotNull);
  expect(order.feeBreakdown!.foodPayableAmount, 95.0);
  expect(order.feeBreakdown!.deliveryPayableAmount, 8.0);
  expect(order.feeBreakdown!.platformServiceFeeAmount, 4.75);
  expect(order.feeBreakdown!.paymentChannelFeeAmount, 0.57);
  expect(order.feeBreakdown!.merchantReceivableAmount, 89.68);
});
```

- [x] Step 2: 运行失败测试

```bash
cd merchant_app
flutter test test/order_alert_coordinator_test.dart
```

期望：失败原因是 `OrderModel` 没有 `feeBreakdown` 或字段不存在。

- [x] Step 3: 最小实现模型

在 `merchant_app/lib/models/order.dart` 增加：

```dart
class OrderFeeBreakdown {
  final double foodAmount;
  final double merchantDiscountAmount;
  final double voucherDiscountAmount;
  final double foodPayableAmount;
  final double deliveryFeeAmount;
  final double deliveryFeeDiscountAmount;
  final double deliveryPayableAmount;
  final double customerPayableAmount;
  final double platformServiceFeeAmount;
  final double paymentChannelFeeAmount;
  final double merchantReceivableAmount;

  const OrderFeeBreakdown({
    required this.foodAmount,
    required this.merchantDiscountAmount,
    required this.voucherDiscountAmount,
    required this.foodPayableAmount,
    required this.deliveryFeeAmount,
    required this.deliveryFeeDiscountAmount,
    required this.deliveryPayableAmount,
    required this.customerPayableAmount,
    required this.platformServiceFeeAmount,
    required this.paymentChannelFeeAmount,
    required this.merchantReceivableAmount,
  });

  factory OrderFeeBreakdown.fromJson(Map<String, dynamic> json) {
    return OrderFeeBreakdown(
      foodAmount: _moneyYuan(json, centsKeys: const ['food_amount']),
      merchantDiscountAmount: _moneyYuan(json, centsKeys: const ['merchant_discount_amount']),
      voucherDiscountAmount: _moneyYuan(json, centsKeys: const ['voucher_discount_amount']),
      foodPayableAmount: _moneyYuan(json, centsKeys: const ['food_payable_amount']),
      deliveryFeeAmount: _moneyYuan(json, centsKeys: const ['delivery_fee_amount']),
      deliveryFeeDiscountAmount: _moneyYuan(json, centsKeys: const ['delivery_fee_discount_amount']),
      deliveryPayableAmount: _moneyYuan(json, centsKeys: const ['delivery_payable_amount']),
      customerPayableAmount: _moneyYuan(json, centsKeys: const ['customer_payable_amount']),
      platformServiceFeeAmount: _moneyYuan(json, centsKeys: const ['platform_service_fee_amount']),
      paymentChannelFeeAmount: _moneyYuan(json, centsKeys: const ['payment_channel_fee_amount']),
      merchantReceivableAmount: _moneyYuan(json, centsKeys: const ['merchant_receivable_amount']),
    );
  }
}
```

同时在 `OrderModel` 增加 `final OrderFeeBreakdown? feeBreakdown;`，构造函数、`fromJson`、`OrderNotifier` fallback 更新和 `_mergeOrder` 都要保留/传递该字段。

- [x] Step 4: 写失败测试：PushMessage 保留费用清单

在 `merchant_app/test/order_alert_coordinator_test.dart` 的 `PushMessage.fromJson` group 增加：

```dart
test('carries fee breakdown through push hydration', () {
  final message = PushMessage.fromJson({
    'message_id': 'merchant:new_order:501',
    'order_id': 501,
    'order_no': 'ORD501',
    'amount': 10300,
    'shop_name': '测试门店',
    'fee_breakdown': {
      'food_payable_amount': 9500,
      'delivery_payable_amount': 800,
      'platform_service_fee_amount': 475,
      'payment_channel_fee_amount': 57,
      'merchant_receivable_amount': 8968,
    },
  });

  expect(message.feeBreakdown, isNotNull);
  expect(message.feeBreakdown!.foodPayableAmount, 95.0);

  final hydrated = message.withOrderSnapshot(
    OrderModel.fromJson({
      'id': 501,
      'order_no': 'ORD501',
      'total_amount': 10300,
      'fee_breakdown': {
        'food_payable_amount': 9600,
        'delivery_payable_amount': 700,
        'platform_service_fee_amount': 480,
        'payment_channel_fee_amount': 58,
        'merchant_receivable_amount': 9062,
      },
    }),
  );

  expect(hydrated.feeBreakdown!.foodPayableAmount, 96.0);
  expect(hydrated.feeBreakdown!.deliveryPayableAmount, 7.0);
});
```

- [x] Step 5: 实现 PushMessage 字段传递

在 `merchant_app/lib/models/push_message.dart` 增加 `OrderFeeBreakdown? feeBreakdown`，并在 `fromJson`、`fromOrder`、`withOrderNumber`、`withOrderSnapshot`、`toJson` 中传递。`toJson` 的费用字段必须输出后端同名 snake_case，单位可继续用元仅供本地通知反序列化；不要发回后端。

- [x] Step 6: 跑模型测试

```bash
cd merchant_app
flutter test test/order_alert_coordinator_test.dart test/order_provider_test.dart
```

期望：两个测试文件通过。

### Task 3: 费用展示接入列表与详情

**文件：**
- Modify: `merchant_app/lib/features/order/order_list_page.dart`
- Modify: `merchant_app/lib/features/order/order_detail_page.dart`

- [x] Step 1: 增加 UI 辅助 getter

在 `OrderModel` 增加只读 getter：

```dart
double get merchantFoodDisplayAmount =>
    feeBreakdown?.foodPayableAmount ?? amount;

double get deliveryFeeDisplayAmount =>
    feeBreakdown?.deliveryPayableAmount ?? 0.0;

bool get hasFeeBreakdown => feeBreakdown != null;
```

- [x] Step 2: 列表金额改为餐费

在 `order_list_page.dart` 将右下角：

```dart
'总计: ¥${order.amount.toStringAsFixed(2)}'
```

改为：

```dart
'餐费: ¥${order.merchantFoodDisplayAmount.toStringAsFixed(2)}'
```

如果 `order.deliveryFeeDisplayAmount > 0`，在左侧商品数下方增加一行 `代取费另列 ¥x.xx`。

- [x] Step 3: 详情页状态卡改为餐费

在 `order_detail_page.dart` 把 `订单总额` 标签改为 `餐费`，金额用 `merchantFoodDisplayAmount`。商品清单底部 `总计` 改成 `餐费小计`。

- [x] Step 4: 详情页新增费用清单区

在商品清单后、备注前插入 `_buildFeeBreakdownCard(context, currentOrder)`。内容：

```text
费用清单
餐费 ¥xx.xx
平台服务费 -¥xx.xx
支付通道费 -¥xx.xx
预计到账 ¥xx.xx
代取费 ¥xx.xx
```

无 `feeBreakdown` 时显示：

```text
费用清单同步中，请稍后刷新订单。
```

不要把代取费纳入预计到账公式，直接展示后端字段。

- [x] Step 5: 跑静态检查

```bash
cd merchant_app
flutter analyze
```

期望：无新增错误。若出现已有主线/环境错误，记录完整输出并只修复本任务引入的问题。

### Task 4: 新订单提醒页增加拒单按钮

**文件：**
- Modify: `merchant_app/lib/features/order/order_alert_page.dart`

- [x] Step 1: 提醒页金额显示餐费

把主金额从 `message.amount` 改为：

```dart
final displayFoodAmount = message.feeBreakdown?.foodPayableAmount ?? message.amount;
```

主金额显示 `¥ displayFoodAmount`，下方小字有费用清单时显示 `代取费另列 ¥x.xx`。

- [x] Step 2: 三动作布局

按钮区改为上下两行：

```text
[立即接单]
[拒单] [稍后处理]
```

`立即接单` 保持主按钮。`拒单` 用次级按钮，文案不比主按钮更抢眼。`稍后处理` 只关闭页面。

- [x] Step 3: 增加拒单确认弹窗

在 `OrderAlertPage` 增加 `_showRejectReasonDialog`，逻辑与详情页一致：

```dart
Future<String?> _showRejectReasonDialog(BuildContext context) async { ... }
```

校验：

- 空：`请填写拒单原因`
- 少于 2 字：`拒单原因至少 2 个字`

- [x] Step 4: 接入拒单调用

点击拒单：

```dart
final reason = await _showRejectReasonDialog(context);
if (reason == null || reason.trim().isEmpty || !context.mounted) return;
final success = await ref.read(orderProvider.notifier).rejectOrder(message.orderId, reason: reason.trim());
if (!context.mounted) return;
ScaffoldMessenger.of(context).showSnackBar(
  SnackBar(content: Text(success ? '已拒绝订单' : ref.read(orderProvider).error ?? '拒单失败，请稍后再试')),
);
if (success) {
  await ref.read(orderProvider.notifier).fetchOrders();
  if (context.mounted) Navigator.pop(context);
}
```

- [x] Step 5: 跑目标测试

```bash
cd merchant_app
flutter test test/order_alert_coordinator_test.dart test/order_provider_test.dart
```

期望：通过。

### Task 5: 完整验证与交付

**文件：**
- 不新增文件。

- [x] Step 1: 运行 Flutter 单元测试

```bash
cd merchant_app
flutter test
```

期望：所有 Flutter 测试通过。

- [x] Step 2: 运行 Flutter analyze

```bash
cd merchant_app
flutter analyze
```

期望：无 issues。

- [x] Step 3: 检查改动范围

```bash
git status --short
git diff --name-only
```

期望：手工改动集中在 `merchant_app/` 和本计划文档；其他目录变更来自 `origin/main` merge。

- [ ] Step 4: 交付说明必须包含

- 已合并 `origin/main`。
- 后端未手工修改。
- 费用清单字段已在商户 App 解析和展示。
- 新订单提醒页已增加拒单入口；`稍后处理` 仍只是收起。
- 已运行的验证命令和结果。
- 未在真机验证的残余风险：Android 厂商、推送、保活、打印链路。

## 执行记录

- 2026-05-18: 已合并 `origin/main`，merge commit 为当前 `HEAD` 的父历史之一。
- 2026-05-18: 已恢复 `codex-before-origin-main-merge-2026-05-18` stash，本地商户 App 既有音频/轮询改动保留。
- 2026-05-18: 已按 TDD 增加 `fee_breakdown` 模型解析、PushMessage 传递和提醒页动作展示测试。
- 2026-05-18: 已根据只读 review 修复列表页连点风险和底部金额行小屏横向溢出风险。
- 2026-05-18: 验证通过：
  - `cd merchant_app && flutter analyze`
  - `cd merchant_app && flutter test`
- 2026-05-18: 未做 Android 真机验证；推送、保活、打印和厂商后台链路仍需真机回归。

## 防跑偏检查表

- [x] 没有手工编辑 `locallife/`。
- [x] 没有把 `total_amount` 继续作为商户应收餐费展示。
- [x] 没有把代取费混进商户预计到账。
- [x] 没有移除 `稍后处理`。
- [x] 拒单必须填写原因并经过确认。
- [x] 接单、拒单都受 `actionInFlightOrderIds` 禁用保护。
- [x] `PushMessage`、`OrderModel`、`OrderNotifier._mergeOrder` 都保留 `feeBreakdown`。
- [x] 验证失败时如实报告，不宣称完成。
