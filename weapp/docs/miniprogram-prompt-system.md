# 小程序提示系统规范

本文件是小程序运行时提示系统的补充文档，不再承担共享反馈标准或页面交互标准的正文职责。

当前长期权威文档为：

- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
- `.github/standards/weapp/INTERACTION_STANDARDS.md`
- `.github/standards/weapp/API_INTERACTION_CONTRACT.md`

本文件只补充以下内容：

- 小程序运行时提示守卫与错误对象约定
- 当前明确保留的 success Toast 例外项
- 页面接入工具与迁移路径
- 小程序端特有的文案和错误映射落点

当同一主题在本文件和长期标准里同时出现时，按以下顺序裁定：

- 提示是否该出现、该由页面状态还是结果区承接：看 `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md` 与 `.github/standards/weapp/INTERACTION_STANDARDS.md`
- 请求层错误映射职责、后端真值和异步结果 contract：看 `.github/standards/weapp/API_INTERACTION_CONTRACT.md`
- 本文件只负责运行时提示守卫、错误对象字段、页面接入工具、保留 success Toast 例外和当前实现细节

## 运行时验收关注点

- 页面状态本身已经承接结果时，不把“没有 success Toast”视为缺失。
- 本文列出的保留项只代表当前可接受例外，不代表未来新增页面可以默认照抄。

## 提示通道继承说明

- 提示通道选择继承共享前端反馈标准与小程序交互标准，本文件不重新定义一套新的页面级通道规则。
- 运行时层只额外约束两件事：同一结果不要被重复提示，以及已经由页面结构承接的结果不应再额外叠加 success Toast。

## 当前可保留的 success Toast 例外

以下 success Toast 在当前版本中属于明确保留项，不作为继续清理的优先目标。

### 当前页轻量加购反馈

- `weapp/miniprogram/pages/dine-in/menu/menu.ts`
- `weapp/miniprogram/pages/dining/index.ts`
- `weapp/miniprogram/pages/takeout/index.ts`
- `weapp/miniprogram/pages/takeout/restaurant-detail/index.ts`
- `weapp/miniprogram/pages/takeout/dish-detail/index.ts`
- `weapp/miniprogram/pages/takeout/combo-detail/index.ts`
- 原因：用户停留在当前页继续点单，保留短 Toast 能提供即时确认。
- 约束：若进入“立即购买”分支，应继续通过 silentSuccess 等方式关闭 success Toast，避免和跳转购物车重复。

### 当前页轻动作反馈

- `weapp/miniprogram/pages/dine-in/menu/menu.ts` 的“已呼叫服务员”
- `weapp/miniprogram/pages/dining/index.ts` 的“已呼叫服务员”
- `weapp/miniprogram/pages/merchant/printers/index.ts` 的“测试命令已发送”
- 原因：动作已发出，但页面没有稳定结果页承接，保留一次短 Toast 更清晰。

### 复制与外部动作确认

- `weapp/miniprogram/pages/merchant/staff/index.ts` 的“邀请码已复制”
- `weapp/miniprogram/pages/merchant/settings/applyment/index.ts` 的“签约链接已复制”
- `weapp/miniprogram/pages/user/bind-merchant/index.ts` 的“已确认登录”
- `weapp/miniprogram/pages/user_center/index.ts` 的“已确认登录”
- 原因：复制和网页登录确认缺少页内结构承接，单次 Toast 足够且不构成双提示。

## 待继续收口或逐页复审的边界项

- `weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts` 的“换桌成功”
- 当前行为：Toast 后立即跳转到堂食点餐页。
- 结论：这是仍可继续收口的边界项。如果后续还要压缩 success Toast，可优先把这里改成直接跳转，由目标页桌台信息承接结果。

- `weapp/miniprogram/pages/reservation/confirm/index.ts` 的“领券完成”
- `weapp/miniprogram/pages/takeout/order-confirm/index.ts` 的“领券完成”
- `weapp/miniprogram/pages/merchant/settings/application/index.ts` 的“识别完成，请确认回填信息”
- 当前行为：页面金额或表单会更新，同时再给一个 success Toast。
- 结论：这类提示不再视为长期稳定例外。只有当页面在短时间内无法让用户明确感知结果时，才可暂时保留；一旦结构结果已经足够明确，应优先删除 success Toast，交给金额变化、结果区或表单回填状态承接。

## 文案与错误映射补充规则

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

### 错误映射落点

- 请求层负责把后端返回归一成用户可展示的 userMessage；这是对 API contract 中“错误映射责任”的运行时落地。
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
- 不要为同一结果同时挂顶部横幅、页内横幅、Toast 和 Modal。
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
- 所有后端错误对用户展示的都是中文业务文案。
- 日志仍可看到 detailMessage，方便排查问题。