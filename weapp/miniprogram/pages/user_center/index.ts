import Navigation from '../../utils/navigation'
import { logger } from '../../utils/logger'
import { getStableBarHeights } from '../../utils/responsive'
import { invalidateConsoleAccessUserInfoCache, resolveConsoleWorkbenchesFromProfile } from '../../utils/console-access'
import {
  bindMerchantInviteCode,
  confirmWebLoginSessionCode,
  fetchUnreadNotificationCount,
  fetchUserProfile,
  fetchWebLoginSessionStatus,
  type UserWorkbenchProfile,
  updateUserProfile,
  uploadAvatarImage
} from '../../services/user-profile'
import {
  consumeForceRefreshFlag,
  extractCode,
  extractRawPayload,
  extractWebLoginMeta,
  getErrorStatusCode,
  isUsableWebLoginSession,
  normalizeRoles,
  pickPrimaryRole,
  ROLE_LABEL_MAP,
  toFriendlyMessage,
  type WebLoginMeta
} from '../../utils/user-center-auth'

const app = getApp<IAppOption>()
let _refreshUserInfoPromise: Promise<void> | null = null
let _pendingForcedRefresh = false
let _lastRefreshUserInfoAt = 0
let _fetchUnreadPromise: Promise<void> | null = null
let _lastFetchUnreadAt = 0
const USER_CENTER_FORCE_REFRESH_FLAG = 'user_center_force_refresh_after_bind_merchant'


Page({
  data: {
    userInfo: {
      nickName: '微信用户',
      avatarUrl: ''
    },
    actionNoticeMessage: '',
    userRoles: [] as Array<{ key: string, label: string }>,
    workbenches: [] as Array<{ id: string, name: string, path: string, icon: string, description?: string, disabled?: boolean, message?: string }>,
    registrationOptions: [
      { id: 'merchant', name: '餐厅入驻', desc: '开通商家账号', path: '/pages/register/merchant/index', icon: 'shop' },
      { id: 'rider', name: '骑手入驻', desc: '成为配送骑手', path: '/pages/register/rider/index', icon: 'undertake-delivery' },
      { id: 'operator', name: '运营商入驻', desc: '区域运营合作', path: '/pages/register/operator/index', icon: 'root-list' }
    ],
    navBarHeight: 88,
    scrollViewHeight: 600,
    loading: false,
    initialLoading: true,
    error: null as string | null,
    unreadCount: 0  // Gap 4: 消息未读数
  },

  onLoad() {
    // 计算导航栏高度和滚动区域高度
    const { navBarHeight } = getStableBarHeights()
    const windowInfo = wx.getWindowInfo()
    // 自定义导航栏为 fixed，顶部避让交给 shared page-shell 处理
    const scrollViewHeight = windowInfo.windowHeight

    this.setData({ navBarHeight, scrollViewHeight })
    this.initUserInfo()
  },

  onShow() {
    // Refresh role in case it changed
    if (app.globalData.userInfo) {
      // 优先使用缓存的完整角色数组，避免 onShow 用 userRole 单值（可能是 'guest'）清空工作台
      const cachedRoles = app.globalData.userRoles
      const roles = cachedRoles && cachedRoles.length > 0 ? cachedRoles : app.globalData.userRole
      this.updateUser(app.globalData.userInfo, roles, app.globalData.userWorkbenches)
    }
    const shouldForceRefresh = consumeForceRefreshFlag(USER_CENTER_FORCE_REFRESH_FLAG)
    // Always try to fetch fresh data on show to ensure persistence check
    this.refreshUserInfo(shouldForceRefresh, shouldForceRefresh)
    // Gap 4: 获取未读消息数
    this.fetchUnreadCount()
  },

  updateUser(
    info: Partial<WechatMiniprogram.UserInfo> & {
      full_name?: string
      nickname?: string
      avatar_url?: string
      avatar?: string
    },
    roles: string[] | string,
    workbenches?: UserWorkbenchProfile[]
  ) {
    const roleList = normalizeRoles(roles)

    const nonCustomerRoleList = roleList.filter((r) => {
      // 过滤掉纯消费者角色和默认游客占位值，不应显示为标签
      return r !== 'customer' && r !== 'guest'
    })

    const userRoles = nonCustomerRoleList.map((r) => ({ key: r, label: ROLE_LABEL_MAP[r] || r }))

    this.setData({
      userInfo: {
        nickName: info.nickName || info.full_name || info.nickname || '微信用户',
        avatarUrl: info.avatarUrl || info.avatar_url || info.avatar || ''
      },
      userRoles
    })

    this.loadWorkbenches(roleList, workbenches)
  },

  async initUserInfo() {
    if (app.globalData.userInfo) {
      const cachedRoles = app.globalData.userRoles
      const roles = cachedRoles && cachedRoles.length > 0 ? cachedRoles : app.globalData.userRole
      this.updateUser(app.globalData.userInfo, roles, app.globalData.userWorkbenches)
    }
    await this.refreshUserInfo()
  },

  async refreshUserInfo(force: boolean = false, suppressUiError: boolean = false) {
    const now = Date.now()
    if (_refreshUserInfoPromise) {
      if (force) {
        _pendingForcedRefresh = true
      }
      return _refreshUserInfoPromise
    }
    // 同一会话内最多每 60 秒请求一次，避免频繁 tab 切换触发大量请求
    if (!force && now - _lastRefreshUserInfoAt < 60000) {
      return
    }

    _refreshUserInfoPromise = (async () => {
      // 如果已有用户数据（非首次加载），不设置 loading 状态，避免 UI 闪烁
      const isInitial = this.data.initialLoading
      if (isInitial) {
        this.setData({ loading: true, error: null })
      }
      try {
        const user = await fetchUserProfile()
        if (user) {
          logger.debug('Refreshed User Info from Backend', user)
          invalidateConsoleAccessUserInfoCache()

          // Recover avatar from local storage if backend returns empty
          const localAvatar = wx.getStorageSync('user_avatar')
          const finalAvatar = user.avatar_url || localAvatar || ''
          console.log('[UserCenter] Refresh Info - Avatar:', finalAvatar, 'User:', user)

          // Update Global Data
          app.globalData.userInfo = {
            nickName: user.full_name || '微信用户',
            avatarUrl: finalAvatar
          } as WechatMiniprogram.UserInfo
          // 缓存完整角色列表，供 onShow 快速恢复工作台
          app.globalData.userRoles = normalizeRoles(user.roles || [])
          app.globalData.userWorkbenches = user.workbenches || []
          app.globalData.userRole = pickPrimaryRole(app.globalData.userRoles)

          // Update Local Data
          this.updateUser(app.globalData.userInfo, user.roles || [], user.workbenches)
        }
        this.setData({ initialLoading: false, loading: false })
        // 只有成功才更新时间戳：失败（如 token 尚未就绪）不应占用 60s 节流窗口，
        // 否则下次 onShow 进来会被跳过，导致角色信息长期停留在错误状态
        _lastRefreshUserInfoAt = Date.now()
      } catch (err) {
        logger.error('Failed to refresh user info', err)
        if (!suppressUiError) {
          this.setData({
            error: '加载用户信息失败',
            loading: false,
            initialLoading: false
          })
        }
        // 失败时不更新时间戳，下次 onShow 可以立即重试
      }
    })().finally(() => {
      _refreshUserInfoPromise = null
      if (_pendingForcedRefresh) {
        _pendingForcedRefresh = false
        void this.refreshUserInfo(true, true)
      }
    })

    return _refreshUserInfoPromise
  },

  onRetry() {
    this.refreshUserInfo()
  },

  // ==================== 导航方法 ====================

  onMyOrders() {
    Navigation.toOrderList()
  },

  onTakeoutOrders() {
    Navigation.toOrderList({ orderType: 'takeout' })
  },

  onReservationOrders() {
	Navigation.toUserReservations()
  },

  onDineInOrders() {
    Navigation.toOrderList({ orderType: 'dine_in' })
  },

  onAddresses() {
    Navigation.toAddressList()
  },

  onCoupons() {
    Navigation.toCoupons()
  },

  onFavorites() {
    Navigation.toFavorites()
  },

  onMembership() {
    Navigation.toMembership()
  },

  onMyReviews() {
    Navigation.toMyReviews()
  },

  onMyReservations() {
    Navigation.toUserReservations()
  },

  onWallet() {
    Navigation.toWallet()
  },

  onAgreements() {
    Navigation.toAgreementCenter()
  },


  // Gap 4: 获取未读消息数
  async fetchUnreadCount() {
    const now = Date.now()
    if (_fetchUnreadPromise) {
      return _fetchUnreadPromise
    }
    // 30 秒内不重复请求
    if (now - _lastFetchUnreadAt < 30000) {
      return
    }

    _fetchUnreadPromise = (async () => {
      try {
        const res = await fetchUnreadNotificationCount()
        this.setData({ unreadCount: res.count || 0 })
      } catch (err) {
        logger.warn('获取未读消息失败', err, 'UserCenter.fetchUnreadCount')
      }
    })().finally(() => {
      _fetchUnreadPromise = null
      _lastFetchUnreadAt = Date.now()
    })

    return _fetchUnreadPromise
  },

  // Gap 4: 跳转通知中心
  onNotifTap() {
    wx.navigateTo({ url: '/pages/notification/index' })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  loadWorkbenches(roles: string[], backendWorkbenches?: UserWorkbenchProfile[]) {
    this.setData({ workbenches: resolveConsoleWorkbenchesFromProfile(roles, backendWorkbenches) })
  },

  onWorkbenchTap(e: WechatMiniprogram.TouchEvent) {
    const { path, disabled, message } = e.currentTarget.dataset
    if (disabled) {
      wx.showModal({
        title: '暂未开通',
        content: message || '已加入商户，等待老板分配岗位后即可进入工作台。',
        showCancel: false,
        confirmText: '我知道了'
      })
      return
    }
    if (path) {
      wx.navigateTo({ url: path })
    }
  },

  onRegisterTap(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset
    // 已有运营商点击「运营商入驻」应跳转「申请更多区域」而非重新走注册流程
    if (id === 'operator' && app.globalData.userRole === 'operator') {
      wx.navigateTo({ url: '/pages/operator/region-expansion/index' })
      return
    }
    const pathMap: Record<string, string> = {
      merchant: '/pages/register/merchant/index',
      rider: '/pages/register/rider/index',
      operator: '/pages/register/operator/index'
    }
    const path = pathMap[id]
    if (path) {
      wx.navigateTo({ url: path })
    }
  },

  onContact() {
    wx.navigateTo({ url: '/pages/user_center/service_center/index' })
  },

  // 扫码入职 - 直接打开相机扫码
  onScanToJoin() {
    if (this.data.actionNoticeMessage) {
      this.setData({ actionNoticeMessage: '' })
    }
    wx.scanCode({
      onlyFromCamera: true,
      scanType: ['qrCode', 'wxCode'],
      success: (res) => {
        void this.handleScanResult(res)
      },
      fail: (err) => {
        logger.warn('Scan cancelled', err, 'UserCenter.scan')
      }
    })
  },

  async handleScanResult(res: WechatMiniprogram.ScanCodeSuccessCallbackResult) {
    const payload = extractRawPayload(res)
    const raw = payload.raw
    const webLoginMeta = extractWebLoginMeta(raw)
    const code = webLoginMeta.code || extractCode(payload.codeCandidate)

    if (!code) {
      logger.warn('Scan empty payload', res, 'UserCenter.scan')
      const system = wx.getSystemInfoSync()
      if (system.platform === 'devtools') {
        wx.showModal({
          title: '扫码结果为空',
          content: '开发者工具中扫码可能不会返回内容，请使用真机扫码或手动输入。',
          confirmText: '手动输入',
          showCancel: false,
          success: () => this.promptManualCode()
        })
        return
      }
      this.promptManualCode()
      return
    }

    await this.handleCodeCandidate(raw, code, webLoginMeta)
  },

  async handleCodeCandidate(raw: string, code: string, webLoginMeta?: WebLoginMeta) {
    const isWebLoginHint = raw.includes('web-login') || raw.includes('/merchant/login') || raw.includes('sig=') || raw.includes('ts=')
    const isInviteHint = raw.includes('invite-merchant') || raw.includes('bind-merchant')

    // 优先识别 Web 登录码
    if (isWebLoginHint) {
      const loginCode = webLoginMeta?.code || code
      try {
        const session = await fetchWebLoginSessionStatus(loginCode)
        if (isUsableWebLoginSession(session)) {
          this.confirmWebLogin(loginCode, webLoginMeta?.sig, webLoginMeta?.ts)
          return
        }
      } catch (error) {
        logger.warn('Web login session validation failed', error, 'UserCenter.scan')
        if (getErrorStatusCode(error) === 404) {
          this.showInvalidWebLoginCode()
        } else {
          this.showWebLoginStatusCheckFailed()
        }
        return
      }

      logger.warn('Web login session missing', { code: loginCode }, 'UserCenter.scan')
      this.showInvalidWebLoginCode()
      return
    }

    // 识别为入职码
    if (isInviteHint || (!isWebLoginHint && raw.includes('code='))) {
      this.confirmInviteCode(code)
      return
    }

    // 无法识别为登录码时，按入职码处理
    this.confirmInviteCode(code)
  },

  promptManualCode() {
    wx.showModal({
      title: '输入扫码内容',
      content: '未识别到二维码内容，请粘贴登录码或入职码',
      editable: true,
      placeholderText: 'web-login:xxxx 或 邀请码',
      confirmText: '继续',
      success: async (res) => {
        if (!res.confirm || !res.content) return
        const raw = String(res.content)
        const webLoginMeta = extractWebLoginMeta(raw)
        const code = webLoginMeta.code || extractCode(raw)
        if (!code) {
          wx.showToast({ title: '内容无效', icon: 'none' })
          return
        }
        await this.handleCodeCandidate(raw, code, webLoginMeta)
      }
    })
  },

  showInvalidWebLoginCode() {
    wx.showModal({
      title: '网页登录码无效',
      content: '当前网页登录码无效或已失效，请返回网页刷新二维码后重试。',
      showCancel: false,
      confirmText: '我知道了'
    })
  },

  showWebLoginStatusCheckFailed() {
    wx.showModal({
      title: '状态校验失败',
      content: '网页登录状态校验失败，请稍后重试。',
      showCancel: false,
      confirmText: '我知道了'
    })
  },

  confirmInviteCode(code: string) {
    wx.showModal({
      title: '员工入职',
      content: '检测到员工入职码，是否确认加入商户？',
      confirmText: '确认入职',
      cancelText: '取消',
      success: async (modal) => {
        if (!modal.confirm) return
        wx.showLoading({ title: '处理中...' })
        try {
          await bindMerchantInviteCode(code)
          await this.refreshUserInfo(true, true)
          this.setData({ actionNoticeMessage: '已加入商户，可从“商家及运营服务”进入对应工作台。' })
        } catch (error: unknown) {
          const message = toFriendlyMessage(error, '加入失败，请稍后重试')
          wx.showToast({ title: message, icon: 'none' })
        } finally {
          wx.hideLoading()
        }
      }
    })
  },

  confirmWebLogin(code: string, sig?: string, ts?: number) {
    wx.showModal({
      title: 'Web 登录确认',
      content: '检测到 Web 登录码，是否确认登录网页端？',
      confirmText: '确认登录',
      cancelText: '取消',
      success: async (modal) => {
        if (!modal.confirm) return
        if (!sig || !ts) {
          wx.showModal({
            title: '二维码无效',
            content: '当前二维码缺少校验信息，请在网页端刷新二维码后重试。',
            showCancel: false,
            confirmText: '我知道了'
          })
          return
        }
        wx.showLoading({ title: '确认中...' })
        try {
          await confirmWebLoginSessionCode(code, sig, ts)
          wx.showToast({ title: '已确认登录', icon: 'success' })
        } catch (error: unknown) {
          const message = toFriendlyMessage(error, '确认失败，请稍后重试')
          wx.showModal({
            title: '无法登录网页端',
            content: message,
            showCancel: false,
            confirmText: '我知道了'
          })
        } finally {
          wx.hideLoading()
        }
      }
    })
  },

  async onChooseAvatar(e: WechatMiniprogram.CustomEvent) {
    const { avatarUrl } = e.detail
    const previousAvatarUrl = this.data.userInfo.avatarUrl || ''

    // Optimistic Update
    this.setData({
      'userInfo.avatarUrl': avatarUrl
    })

    wx.showLoading({ title: '上传中...' })

    try {
      // 1. Upload to Server
      const { mediaId, displayUrl } = await uploadAvatarImage(avatarUrl)

      // 2. Update Backend Profile
      await updateUserProfile({ avatar_media_asset_id: mediaId })

      const resolvedAvatarUrl = displayUrl || avatarUrl

      // 3. Persist the latest usable avatar after backend profile update succeeds
      wx.setStorageSync('user_avatar', resolvedAvatarUrl)

      // 4. Update Global Data and current page state
      app.globalData.userInfo = {
        ...(app.globalData.userInfo || {}),
        avatarUrl: resolvedAvatarUrl
      } as WechatMiniprogram.UserInfo

      this.setData({
        'userInfo.avatarUrl': resolvedAvatarUrl
      })

      if (!displayUrl) {
        wx.showToast({ title: '头像已提交，审核通过后自动更新', icon: 'none' })
      }

    } catch (error) {
      this.setData({
        'userInfo.avatarUrl': previousAvatarUrl
      })

      app.globalData.userInfo = {
        ...(app.globalData.userInfo || {}),
        avatarUrl: previousAvatarUrl
      } as WechatMiniprogram.UserInfo

      console.error('Failed to update avatar on backend', error)
      wx.showToast({ title: '头像上传失败', icon: 'none' })
    } finally {
      wx.hideLoading()
    }
  },

  async onNicknameChange(e: WechatMiniprogram.CustomEvent) {
    const nickName = e.detail.value
    if (!nickName) return

    this.setData({
      'userInfo.nickName': nickName
    })

    // Update Global Data
    app.globalData.userInfo = {
      ...(app.globalData.userInfo || {}),
      nickName
    } as WechatMiniprogram.UserInfo

    // Call Backend API
    try {
      await updateUserProfile({ full_name: nickName })
    } catch (error) {
      console.error('Failed to update nickname on backend', error)
    }
  }
})
