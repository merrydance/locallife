# 小程序提示系统规范

## 目标

小程序提示系统需要同时满足两个要求：

1. 同一件事只提示一次，避免弹窗、Toast、页内错误态同时抢占用户注意力。
2. 所有后端错误都必须转换成面向最终用户的业务文案，不能直接展示后端原文、英语错误、网关错误、数据库错误或调试信息。

## 产品与设计验收摘要

这一节用于非研发角色快速验收，不需要通读整份规范。

### 验收结论标准

- 通过：同一动作只有一个主提示，且提示方式与页面结构一致。
- 不通过：同一结果被 Toast、弹窗、页内状态重复表达两次或以上。
- 不通过：用户能看到后端原文、英语报错、技术字段名或调试信息。

### 重点看什么

- 首屏失败是否用页内错误态承接，而不是只弹 Toast。
- 成功后如果页面已经跳转、返回、刷新或出现明显状态变化，是否还在额外弹 success Toast。
- 需要确认、解释原因、引导下一步的场景，是否用了 Modal 而不是 Toast。
- 所有失败提示是否都是中文业务文案，且用户能理解下一步怎么做。

### 必过场景

- 下单、支付、取消、申请、上传、登录、授权这类核心动作，不能出现双重提示。
- 列表页、详情页、工作台、账户页在刷新失败时，页面内必须有可见错误态或重试入口。
- 后端错误、网关错误、英语错误、数据库错误不能直接展示给用户。
- 复制、加购、呼叫服务员、测试打印这类轻动作，可以保留单次短 Toast，但不能叠加别的主提示。

### 当前可以接受的保留项

- 停留在当前页继续操作的加购成功提示。
- 复制成功、网页登录确认、呼叫服务员、测试打印发送成功。
- 领券完成、证照识别完成后这类需要给用户一个短确认、但不需要长解释的提示。

### 当前仍可优化但不阻塞验收的边界项

- 扫码换桌成功后仍有短 Toast，再进入点餐页；这一项可以继续收，但不影响本轮整体验收。

### 验收方式

- 按真实任务流走查，而不是只看单个页面。
- 每条任务流至少检查一次：成功、失败、弱网或刷新后的表现。
- 如果页面状态本身已经说明结果，应优先接受“无 success Toast”的设计，不把“没弹 Toast”当成缺失。

## 统一原则

### 一事一次提示

- 同一用户动作只允许出现一个主提示。
- 成功提示优先用短 Toast，不再叠加二次 Toast 或确认弹窗。
- 失败提示优先判断是否已经有页内错误态，如果页面主体已经展示失败状态，就不要再追加 Toast。
- 需要用户确认、二次选择、授权跳转、后续操作指引时，使用 Modal，不使用 Toast。
- Modal 打开前应视为当前流程的唯一主提示，不应再并发弹出相同含义的 Toast。
- 高频重复动作允许重复成功，但同一时刻只展示一次同类 Toast，避免“连点三次弹三次”。

### 提示通道选择

- Toast：轻量结果反馈，适合成功、轻校验、短时失败提示，文案控制在一行内。
- Modal：需要确认、需要解释原因、需要引导下一步、需要跳设置页时使用。
- 页内错误态：首屏加载失败、列表加载失败、核心数据缺失、弱网重试场景，必须在页面结构内展示可恢复状态，Toast 只能作为补充，不能代替页内状态。
- Loading：只表达处理中，不携带业务结论。处理完成后只能落一个结果提示。

### 成功提示判定表

- 保留成功 Toast：用户停留在当前页，且页面局部变化不够显眼时。例如“已加入购物车”“已复制邀请码”“已呼叫服务员”“测试命令已发送”。
- 保留成功 Toast：动作是瞬时的、没有稳定结果页，也没有显式状态区承接。例如复制、轻量提交、单次触发类操作。
- 去掉成功 Toast：动作完成后立即跳转、返回、切 tab、进入成功页。目标页本身就是结果承接，不再先弹 Toast。
- 去掉成功 Toast：动作完成后当前页会立刻 reload，且列表状态、详情状态、金额、标签、按钮状态已经能明确表达结果。
- 去掉成功 Toast：页面已经存在显式状态条、横幅、步骤条或实时状态区，并且这些元素会在动作后立即更新。
- 去掉成功 Toast：充值、提现、审核、同步等动作完成后，当前页的余额、账单、状态卡、步骤条会持续承接结果时，优先使用页内提示条或结果区，不再额外弹成功 Toast。
- 只保留一个成功提示：同一动作如果存在“正常成功”和“对账成功/已同步成功”两条成功分支，必须二选一，不要连续提示两次成功。

### 成功提示反例

- 先 Toast “支付成功”，再进入支付成功页。
- 先 Toast “已取消”，再重新加载详情页或列表页让状态变成“已取消”。
- 先 Toast “定位已刷新”，而页面上已经同步更新“定位运行中”“最近更新时间”“待补发状态”。
- 先 Toast “审核通过”，随后立刻刷新列表，列表状态已变成“已通过”。
- 先 Toast “提现申请已提交”，随后余额卡、冻结金额和账单列表立刻开始刷新。

### 成功提示正例

- 复制成功：页面通常不会出现新的结构状态，保留短 Toast。
- 单次加入购物车且仍停留在当前商品页：保留短 Toast。
- 提交后仍停留在当前页，但页面没有单独成功区承接结果：可保留一个短 Toast。
- 需要提醒用户外部动作已经发出，但最终状态还要稍后同步：可保留类似“支付已提交”“回复已提交，待同步”的单次 Toast。

### 当前保留清单

以下 success Toast 在当前版本中属于明确保留项，不作为继续清理的优先目标：

- 当前页轻量加购反馈：
	- weapp/miniprogram/pages/dine-in/menu/menu.ts
	- weapp/miniprogram/pages/dining/index.ts
	- weapp/miniprogram/pages/takeout/index.ts
	- weapp/miniprogram/pages/takeout/restaurant-detail/index.ts
	- weapp/miniprogram/pages/takeout/dish-detail/index.ts
	- weapp/miniprogram/pages/takeout/combo-detail/index.ts
	- 原因：用户停留在当前页继续点单，保留短 Toast 能提供即时确认。
	- 约束：若进入“立即购买”分支，应继续通过 silentSuccess 等方式关闭 success Toast，避免和跳转购物车重复。

- 当前页轻动作反馈：
	- weapp/miniprogram/pages/dine-in/menu/menu.ts 的“已呼叫服务员”
	- weapp/miniprogram/pages/dining/index.ts 的“已呼叫服务员”
	- weapp/miniprogram/pages/merchant/printers/index.ts 的“测试命令已发送”
	- 原因：动作已发出，但页面没有稳定结果页承接，保留一次短 Toast 更清晰。

- 复制与外部动作确认：
	- weapp/miniprogram/pages/merchant/staff/index.ts 的“邀请码已复制”
	- weapp/miniprogram/pages/merchant/settings/applyment/index.ts 的“签约链接已复制”
	- weapp/miniprogram/pages/user/bind-merchant/index.ts 的“已确认登录”
	- weapp/miniprogram/pages/user_center/index.ts 的“已确认登录”
	- 原因：复制和网页登录确认缺少页内结构承接，单次 Toast 足够且不构成双提示。

- 轻业务完成后的即时确认：
	- weapp/miniprogram/pages/reservation/confirm/index.ts 的“领券完成”
	- weapp/miniprogram/pages/takeout/order-confirm/index.ts 的“领券完成”
	- weapp/miniprogram/pages/merchant/settings/application/index.ts 的“识别完成，请确认回填信息”
	- 原因：虽然页面金额或表单会更新，但用户仍需要一个短确认，告诉自己“领券已成功”“识别结果已回填并等待确认”。

### 待终审边界项

- weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts 的“换桌成功”
	- 当前行为：Toast 后立即跳转到堂食点餐页。
	- 结论：这是仍可继续收口的边界项。如果后续还要压缩 success Toast，可优先把这里改成直接跳转，由目标页桌台信息承接结果。

## 文案规则

### 用户文案要求

- 只告诉用户发生了什么、当前影响是什么、建议怎么处理。
- 先业务结果，后用户动作，例如“支付未完成，请重新确认支付结果”。
- 避免工程术语，例如 API、SQL、Gateway、Token、Exception、Nginx、No rows。
- 避免英语原文、错误码拼接、后端堆栈、原始字段名。
- 避免模糊空话，例如“系统异常”“未知错误”，除非已经补充明确处理建议。

### 推荐句式

- 网络类：网络开小差了，请检查网络后重试
- 服务类：服务暂时不可用，请稍后再试
- 登录类：登录状态已失效，请重新进入后再试
- 权限类：当前无权限执行该操作
- 频率类：请求太频繁，请稍后再试
- 资源失效类：图片已失效，请重新上传
- 状态冲突类：当前操作暂时无法完成，请刷新后再试

## 后端错误映射规则

### 必须做前端映射的错误

- 网关错误：502、503、504、Bad Gateway、Gateway Timeout、Service Unavailable
- 认证错误：401、token 失效、unauthorized
- 权限错误：403、forbidden、permission denied
- 频率限制：429、too many requests、rate limit
- 数据层错误：SQL、constraint、duplicate key、no rows in result set
- 资源缺失：not found、asset not found、upload session not found
- 第三方内部错误：微信、地图、OCR、多媒体审核返回的原始技术信息

### 映射落点

- 请求层负责把后端返回归一成用户可展示的 userMessage。
- 页面层优先读取 userMessage，不直接使用原始 message。
- 技术细节只保留在日志和监控里，不进入 Toast、Modal、页内文案。

## 当前实现约束

### 全局去重

- 已在全局安装提示守卫，对连续重复的 Toast 做时间窗去重。
- Modal 打开前会主动收起 Toast，减少同一事件的双重提示。

### 错误对象约定

- AppError.message 作为默认可展示文案。
- AppError.userMessage 作为显式用户文案。
- AppError.detailMessage 保留技术细节，仅用于日志、监控和错误判定。

## 页面接入规则

- 新页面获取错误文案统一使用 utils/user-facing.ts 的 getErrorUserMessage。
- 需要根据错误做逻辑分支时，统一使用 getErrorDebugMessage 或状态码，不直接依赖用户提示文案。
- 不要在页面里自行拼接后端原始 message 给用户。
- 不要在 catch 里同时 setData(error) 又立即 Toast 同一失败，二者只能保留一个主提示。
- 不要在成功后同时使用 Toast 和跳转/返回/刷新结果页承接同一结果。
- 不要在 success Toast 后再人为等待 500ms 到 1500ms 只为了让 Toast 显示一遍；如果结果页足够明确，应直接跳转或直接刷新。
- 如果按钮本身已有 loading，且操作完成后页面状态会立即刷新，优先只保留 loading + 刷新结果，不再叠加 success Toast。

## 迁移优先级

1. 请求层和公共服务层先完成映射，先堵住后端错误直出。
2. 首屏、支付、下单、登录、上传、授权等核心流程优先迁移到统一工具。
3. 剩余页面逐步替换本地 getErrorMessage 和直接的 error.message 展示逻辑。

## 验收清单

- 同一失败动作不会连续出现两个以上提示。
- 同一成功动作不会同时出现 Toast 和结果页/返回页状态双重承接。
- 长文案、确认型提示不再使用 Toast。
- 页面首屏失败有页内错误态和重试入口。
- 所有后端错误对用户展示的都是中文业务文案。
- 日志仍可看到 detailMessage，方便排查问题。