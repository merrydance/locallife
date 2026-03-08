import Navigation from '../../utils/navigation'
import { updateUserInfo, getUserInfo, getWebLoginSessionStatus, confirmWebLoginSession } from '../../api/auth'
import { bindMerchant } from '../../api/personal'
import { logger } from '../../utils/logger'
import { UploadService } from '../../api/upload'
import { getStableBarHeights } from '../../utils/responsive'
import { notificationService } from '../../api/notification'

const app = getApp<IAppOption>()
let _refreshUserInfoPromise: Promise<void> | null = null
let _lastRefreshUserInfoAt = 0
let _fetchUnreadPromise: Promise<void> | null = null
let _lastFetchUnreadAt = 0

interface MessageError {
  userMessage?: string
  message?: string
}

interface ScanCodeRawPayload {
  path?: string
  result?: string
  rawData?: string
  scene?: string
  query?: Record<string, unknown>
}

function toFriendlyMessage(error: unknown, fallback: string) {
  const err = error as MessageError
  const raw = err?.userMessage || err?.message || ''
  const text = String(raw || '').trim()
  if (!text) return fallback
  if (/[\u4e00-\u9fa5]/.test(text)) return text
  const lower = text.toLowerCase()
  if (lower.includes('sig') && lower.includes('required')) {
    return '二维码签名缺失，请刷新二维码后重试'
  }
  if ((lower.includes('ts') || lower.includes('timestamp')) && lower.includes('required')) {
    return '二维码时间戳缺失，请刷新二维码后重试'
  }
  if (lower.includes('signature') || lower.includes('sig') || lower.includes('mismatch')) {
    return '二维码校验失败，请刷新二维码后重试'
  }
  if (lower.includes('expired')) {
    return '二维码已过期，请刷新二维码'
  }
  if (lower.includes('session not found') || lower.includes('not found')) {
    return '登录码不存在或已失效，请刷新二维码'
  }
  if (lower.includes('already consumed')) {
    return '该登录码已被使用，请刷新二维码'
  }
  if (lower.includes('not confirmed')) {
    return '请先在小程序确认登录'
  }
  if (lower.includes('merchant account') || lower.includes('merchant')) {
    return '当前账号暂无商户权限'
  }
  if (lower.includes('too many') || lower.includes('429')) {
    return '操作太频繁，请稍后再试'
  }
  if (lower.includes('network') || lower.includes('timeout')) {
    return '网络异常，请稍后重试'
  }
  return fallback
}

Page({
  data: {
    userInfo: {
      nickName: '微信用户',
      avatarUrl: ''
    },
    userRoles: [] as Array<{ key: string, label: string }>,
    workbenches: [] as Array<{ id: string, name: string, path: string, icon: string }>,
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
    // windowHeight 已扣除原生 tabBar，只需扣除自定义导航栏
    const scrollViewHeight = windowInfo.windowHeight - navBarHeight

    this.setData({ navBarHeight, scrollViewHeight })
    this.initUserInfo()
  },

  onShow() {
    // Refresh role in case it changed
    if (app.globalData.userInfo) {
      this.updateUser(app.globalData.userInfo, app.globalData.userRole)
    }
    // Always try to fetch fresh data on show to ensure persistence check
    this.refreshUserInfo()
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
    roles: string[] | string
  ) {
    const roleList = Array.isArray(roles) ? roles : [roles]
    const roleMap: Record<string, string> = {
      'merchant': '商家',
      'merchant_boss': '店长',
      'merchant_staff': '店员',
      'rider': '骑手',
      'customer': '顾客',
      'operator': '运营',
      'admin': '管理员',
      'MERCHANT': '商家',
      'MERCHANT_BOSS': '店长',
      'MERCHANT_STAFF': '店员',
      'RIDER': '骑手',
      'CUSTOMER': '顾客',
      'OPERATOR': '运营',
      'ADMIN': '管理员'
    }

    const nonCustomerRoleList = roleList.filter((r) => {
      const normalized = String(r || '').toLowerCase()
      return normalized !== 'customer'
    })

    const userRoles = nonCustomerRoleList.map((r) => ({ key: r, label: roleMap[r] || r }))

    this.setData({
      userInfo: {
        nickName: info.nickName || info.full_name || info.nickname || '微信用户',
        avatarUrl: info.avatarUrl || info.avatar_url || info.avatar || ''
      },
      userRoles
    })

    this.loadWorkbenches(roleList)
  },

  async initUserInfo() {
    if (app.globalData.userInfo) {
      // Use cached role as fallback
      this.updateUser(app.globalData.userInfo, app.globalData.userRole)
    }
    await this.refreshUserInfo()
  },

  async refreshUserInfo() {
    const now = Date.now()
    if (_refreshUserInfoPromise) {
      return _refreshUserInfoPromise
    }
    if (now - _lastRefreshUserInfoAt < 1000) {
      return
    }

    _refreshUserInfoPromise = (async () => {
      if (this.data.loading) return

      this.setData({ loading: true, error: null })
      try {
        const user = await getUserInfo()
        if (user) {
          logger.debug('Refreshed User Info from Backend', user)

          // Recover avatar from local storage if backend returns empty
          const localAvatar = wx.getStorageSync('user_avatar')
          const finalAvatar = user.avatar_url || localAvatar || ''
          console.log('[UserCenter] Refresh Info - Avatar:', finalAvatar, 'User:', user)

          // Update Global Data
          app.globalData.userInfo = {
            nickName: user.full_name || '微信用户',
            avatarUrl: finalAvatar
          } as WechatMiniprogram.UserInfo

          // Update Local Data
          this.updateUser(app.globalData.userInfo, user.roles || [])
        }
        this.setData({ initialLoading: false, loading: false })
      } catch (err) {
        logger.error('Failed to refresh user info', err)
        this.setData({
          error: '加载用户信息失败',
          loading: false,
          initialLoading: false
        })
      }
    })().finally(() => {
      _refreshUserInfoPromise = null
      _lastRefreshUserInfoAt = Date.now()
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
    wx.navigateTo({ url: '/pages/orders/list/index?order_type=takeout' })
  },

  onReservationOrders() {
	wx.navigateTo({ url: '/pages/user_center/reservations/index' })
  },

  onDineInOrders() {
    wx.navigateTo({ url: '/pages/orders/list/index?order_type=dine_in' })
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
    wx.navigateTo({ url: '/pages/user_center/reservations/index' })
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
    if (now - _lastFetchUnreadAt < 1000) {
      return
    }

    _fetchUnreadPromise = (async () => {
      try {
        const res = await notificationService.getUnreadCount()
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

  loadWorkbenches(roles: string[]) {
    const workbenches = []

    // 商家入口：仅商户相关角色
    if (roles.some((r) => ['merchant', 'merchant_boss', 'merchant_staff'].includes(r))) {
      workbenches.push({
        id: 'merchant',
        name: '商户中心',
        icon: '/assets/icons/store.svg',
        path: '/pages/merchant/dashboard/index'
      })
    }

    // 骑手入口：支持骑手，或者运营人员
    if (roles.includes('rider') || roles.includes('operator')) {
      workbenches.push({
        id: 'rider',
        name: '骑手配送',
        icon: '/assets/icons/rider.svg',
        path: '/pages/rider/dashboard/index'
      })
    }

    // 运营入口：独立显示
    if (roles.includes('operator')) {
      workbenches.push({
        id: 'operator',
        name: '运营管理中心',
        icon: '/assets/icons/bill-list.svg',
        path: '/pages/operator/dashboard/index'
      })
    }

    // Admin Entrance
    if (roles.includes('admin')) {
      workbenches.push({
        id: 'admin',
        name: '平台管理中心',
        icon: '/assets/icons/platform.svg',
        path: '/pages/platform/dashboard/dashboard'
      })
    }

    this.setData({ workbenches })
  },

  onWorkbenchTap(e: WechatMiniprogram.TouchEvent) {
    const { path } = e.currentTarget.dataset
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
    const payload = this.extractRawPayload(res)
    const raw = payload.raw
    const webLoginMeta = this.extractWebLoginMeta(raw)
    const code = webLoginMeta.code || this.extractCode(payload.codeCandidate)

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

  async handleCodeCandidate(raw: string, code: string, webLoginMeta?: { code?: string, sig?: string, ts?: number }) {
    const isWebLoginHint = raw.includes('web-login') || raw.includes('/merchant/login') || raw.includes('sig=') || raw.includes('ts=')
    const isInviteHint = raw.includes('invite-merchant') || raw.includes('bind-merchant')

    // 优先识别 Web 登录码
    if (isWebLoginHint) {
      try {
        const loginCode = webLoginMeta?.code || code
        const session = await getWebLoginSessionStatus(loginCode)
        if (session?.code) {
          this.confirmWebLogin(loginCode, webLoginMeta?.sig, webLoginMeta?.ts)
          return
        }
      } catch (error) {
        logger.warn('Scan not web login', error, 'UserCenter.scan')
      }
    }

    // 识别为入职码
    if (isInviteHint || (!isWebLoginHint && raw.includes('code='))) {
      this.confirmInviteCode(code)
      return
    }

    // 无法识别为登录码时，按入职码处理
    this.confirmInviteCode(code)
  },

  extractCode(raw: string) {
    if (!raw) return ''
    const decoded = decodeURIComponent(raw)
    const match = decoded.match(/code=([^&]+)/)
    if (match) return match[1]
    const webLoginMatch = decoded.match(/web-login:([0-9a-fA-F]{32})/)
    if (webLoginMatch) return webLoginMatch[1]
    const inviteMatch = decoded.match(/invite-merchant:([A-Za-z0-9_-]+)/)
    if (inviteMatch) return inviteMatch[1]
    const hexMatch = decoded.match(/[0-9a-fA-F]{32}/)
    if (hexMatch) return hexMatch[0]
    return decoded
  },

  extractWebLoginMeta(raw: string) {
    if (!raw) return { code: '', sig: '', ts: undefined }
    const decoded = decodeURIComponent(raw)
    const queryCodeMatch = decoded.match(/code=([^&]+)/)
    const webLoginMatch = decoded.match(/web-login:([0-9a-fA-F]{32})/)
    const code = queryCodeMatch ? queryCodeMatch[1] : webLoginMatch ? webLoginMatch[1] : ''
    if (!code) return { code: '', sig: '', ts: undefined }
    const sigMatch = decoded.match(/sig=([0-9a-fA-F]+)/)
    const tsMatch = decoded.match(/ts=(\d+)/)
    return {
      code,
      sig: sigMatch ? sigMatch[1] : '',
      ts: tsMatch ? Number(tsMatch[1]) : undefined
    }
  },

  extractRawPayload(res: WechatMiniprogram.ScanCodeSuccessCallbackResult) {
    const rawRes = res as unknown as ScanCodeRawPayload
    const path = rawRes.path || ''
    const result = rawRes.result || ''
    const rawData = rawRes.rawData || ''
    const scene = rawRes.scene || ''
    const query = rawRes.query || {}
    const codeFromQuery = typeof query.code === 'string' ? query.code : ''
    const candidate = [path, result, rawData, scene, codeFromQuery].find((val) => !!val) || ''
    return {
      raw: String(candidate),
      codeCandidate: String(codeFromQuery || candidate || '')
    }
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
        const code = this.extractCode(raw)
        if (!code) {
          wx.showToast({ title: '内容无效', icon: 'none' })
          return
        }
        await this.handleCodeCandidate(raw, code)
      }
    })
  },

  confirmInviteCode(code: string) {
    wx.showModal({
      title: '员工入职',
      content: '检测到员工入职码，是否确认加入商户？',
      confirmText: '确认入职',
      cancelText: '取消',
      success: (modal) => {
        if (!modal.confirm) return
        wx.showLoading({ title: '处理中...' })
        bindMerchant(code)
          .then(() => {
            wx.showToast({ title: '加入成功', icon: 'success' })
          })
          .catch((error: unknown) => {
            const message = toFriendlyMessage(error, '加入失败，请稍后重试')
            wx.showToast({ title: message, icon: 'none' })
          })
          .finally(() => {
            wx.hideLoading()
          })
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
          await confirmWebLoginSession(code, sig, ts)
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

    // Optimistic Update
    this.setData({
      'userInfo.avatarUrl': avatarUrl
    })

    wx.showLoading({ title: '上传中...' })

    try {
      // 1. Upload to Server
      const imageUrl = await UploadService.uploadImage(avatarUrl, 'avatar')
      const remoteUrl = imageUrl

      // 2. Persist locally with remote URL
      wx.setStorageSync('user_avatar', remoteUrl)

      // 3. Update Global Data
      app.globalData.userInfo = {
        ...(app.globalData.userInfo || {}),
        avatarUrl: remoteUrl
      } as WechatMiniprogram.UserInfo

      this.setData({
        'userInfo.avatarUrl': remoteUrl
      })

      // 4. Update Backend Profile
      await updateUserInfo({ avatar_url: remoteUrl })

    } catch (error) {
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
      await updateUserInfo({ full_name: nickName })
    } catch (error) {
      console.error('Failed to update nickname on backend', error)
    }
  }
})
