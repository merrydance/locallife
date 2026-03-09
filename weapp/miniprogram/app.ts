import { tracker, EventType } from './utils/tracker'
import { wechatLogin, getUserInfo, getDeviceId } from './api/auth'
import { setToken } from './utils/auth'
import { logger } from './utils/logger'
import { ErrorHandler } from './utils/error-handler'
import { networkMonitor } from './utils/network-monitor'
import { themeManager } from './utils/theme'
import { locationService } from './utils/location'

App<IAppOption>({
  globalData: {
    userInfo: null,
    // location as object to store name and optional details
    location: { name: '' } as { name?: string, address?: string },
    latitude: null,
    longitude: null,
    userRole: 'guest',
    userId: undefined as number | undefined,
    merchantId: undefined,
    merchantName: '',
    // 多店铺切换支持
    currentMerchantId: undefined as number | undefined,
    merchantInfo: undefined as {
      id: number
      name: string
      logo_url?: string
      is_open: boolean
      status: string
    } | undefined,
    // (内部使用) 上次定位上下文
    _lastLocationContext: null as {
      lat: number
      lng: number
      time: number
      name: string
      address?: string
    } | null
  },

  onLaunch() {
    logger.info('🚀 小程序启动', undefined, 'App.onLaunch')

    // 检查小程序版本更新
    if (wx.canIUse('getUpdateManager')) {
      const updateManager = wx.getUpdateManager()
      updateManager.onUpdateReady(() => {
        wx.showModal({
          title: '更新提示',
          content: '新版本已准备好，重启后生效，是否立即重启？',
          confirmText: '立即重启',
          cancelText: '稍后',
          success(res) {
            if (res.confirm) {
              updateManager.applyUpdate()
            }
          }
        })
      })
      updateManager.onUpdateFailed(() => {
        logger.warn('小程序新版本下载失败', undefined, 'App.onLaunch')
      })
    }

    // 预初始化全局布局数据 (Phase 1)
    const { getGlobalLayoutData } = require('./utils/responsive')
    const layout = getGlobalLayoutData()
    const { globalStore } = require('./utils/global-store')
    globalStore.set('navBarHeight', layout.navBarHeight)
    globalStore.set('isLargeScreen', layout.isLargeScreen)

    // 恢复上次已知位置，避免首屏等待 GPS
    // 用 setTimeout(0) 推迟到首帧渲染后再读 Storage，不阻塞 onLoad
    setTimeout(() => {
      const LOCATION_CACHE_MAX_AGE = 24 * 60 * 60 * 1000
      try {
        const lastKnown = wx.getStorageSync('last_known_location') as {
          lat: number; lng: number; name: string; address?: string; time: number
        } | null
        if (lastKnown?.lat && lastKnown?.lng && (Date.now() - lastKnown.time) < LOCATION_CACHE_MAX_AGE) {
          // 只在 globalData 还没被真实 GPS 覆盖的情况下才应用缓存
          if (!this.globalData.latitude || !this.globalData.longitude) {
            this.globalData.latitude = lastKnown.lat
            this.globalData.longitude = lastKnown.lng
            this.globalData.location = { name: lastKnown.name, address: lastKnown.address }
            this.globalData._lastLocationContext = {
              lat: lastKnown.lat,
              lng: lastKnown.lng,
              time: lastKnown.time,
              name: lastKnown.name,
              address: lastKnown.address
            }
            // 通知 globalStore，使地址栏等订阅者立即收到位置
            const { globalStore } = require('./utils/global-store')
            globalStore.updateLocation(lastKnown.lat, lastKnown.lng, lastKnown.name, lastKnown.address)
            logger.info('已从缓存恢复位置', { name: lastKnown.name }, 'App.onLaunch')
          }
        }
      } catch (_e) { /* storage 读取失败不影响主流程 */ }
    }, 0)

    // 清除旧的 API 缓存（响应格式已更新为统一信封格式）
    this.clearApiCache()

    // 初始化主题
    themeManager // 引用主题管理器触发初始化
    logger.debug('主题管理器已初始化', { isDark: themeManager.isDark() }, 'App.onLaunch')

    // 初始化网络监控
    networkMonitor.subscribe((state) => {
      logger.info('网络状态更新', state, 'App.networkMonitor')
    })

    // 始终执行真实登录流程（移除 Demo 模式判断）
    // 并行执行：登录 + 获取坐标
    logger.info('📍 开始并行执行：登录 + 获取坐标', undefined, 'App.onLaunch')
    this.silentLogin()
    this.getLocationCoordinates() // 先获取坐标，不等待 token

    // Step 3: Track App Open
    tracker.log(EventType.APP_OPEN)
  },

  onShow() {
    // 桌面微信长时间后台后，网络状态可能滞后；前台恢复时主动刷新
    networkMonitor.refreshStatus(true).catch(() => {
      // 忽略刷新失败，不影响主流程
    })
  },

  /**
     * 全局错误捕获 - 捕获未处理的同步错误
     */
  onError(error: string) {
    logger.error('全局错误捕获', { error }, 'App.onError')
    ErrorHandler.handle(error, 'App.onError')

    // 上报到监控平台
    this.reportErrorToMonitor(error, 'onError')
  },

  /**
     * 全局Promise拒绝捕获 - 捕获未处理的Promise错误
     */
  onUnhandledRejection(res: { reason?: unknown, promise?: unknown }) {
    // 后端服务不可用时使用简洁日志
    const reason = res.reason
    const reasonMessage =
      reason && typeof reason === 'object' && 'message' in reason
        ? (reason as { message?: string }).message
        : undefined
    const reasonStr = String(reasonMessage || reason)
    const isBackendError = reasonStr.includes('502') || reasonStr.includes('503') || reasonStr.includes('504')

    if (isBackendError) {
      logger.warn('[后端服务不可用] Promise rejected', { reason: reasonStr }, 'App.onUnhandledRejection')
    } else {
      logger.error('未处理的Promise拒绝', {
        reason: res.reason,
        promise: res.promise
      }, 'App.onUnhandledRejection')
      ErrorHandler.handle(res.reason, 'App.onUnhandledRejection')
    }

    // 后端不可用时不上报
    if (!isBackendError) {
      this.reportErrorToMonitor(res.reason, 'onUnhandledRejection')
    }
  },

  /**
     * 页面未找到捕获
     */
  onPageNotFound(res: {
    path?: string
    query?: Record<string, unknown>
    isEntryPage?: boolean
  }) {
    logger.warn('页面未找到', {
      path: res.path,
      query: res.query,
      isEntryPage: res.isEntryPage
    }, 'App.onPageNotFound')

    // 重定向到首页
    wx.switchTab({
      url: '/pages/takeout/index',
      fail: () => {
        wx.reLaunch({ url: '/pages/takeout/index' })
      }
    })
  },

  /**
     * 上报错误到监控平台
     */
  reportErrorToMonitor(error: unknown, type: string) {
    try {
      // 使用微信小程序实时日志
      const realtimeLog = wx.getRealtimeLogManager ? wx.getRealtimeLogManager() : null
      if (realtimeLog) {
        realtimeLog.error(`[${type}]`, error)
      }

      // TODO: 接入第三方监控平台(如腾讯云CLS、Sentry等)
      // wx.request({
      //     url: 'https://your-monitor-platform.com/api/error',
      //     method: 'POST',
      //     data: {
      //         type,
      //         error: String(error),
      //         timestamp: Date.now(),
      //         appVersion: __wxConfig?.envVersion || 'unknown'
      //     }
      // })
    } catch (e) {
      // 上报失败也不能影响主流程
      console.error('Error reporting failed:', e)
    }
  },

  silentLogin(attempt = 0) {
    const MAX_LOGIN_RETRIES = 3
    const RETRY_DELAYS = [0, 2000, 4000] // 首次无延迟，第2次等2秒，第3次等4秒

    if (attempt === 0) {
      logger.info('开始静默登录流程', undefined, 'App.silentLogin')

      const { getToken, isTokenNearExpiry, getRefreshToken, clearToken } = require('./utils/auth')
      const existingToken = getToken() as string

      // 快速路径：access token 仍有效，直接拉取用户信息，无需 wx.login
      if (existingToken && !isTokenNearExpiry(0)) {
        logger.info('Token 仍有效，跳过 wx.login', undefined, 'App.silentLogin')
        getUserInfo()
          .then((user) => {
            this._applyUserInfo(user)
            logger.info('✅ 静默登录成功 (复用 token)', { userId: user.id }, 'App.silentLogin')
            if (this.globalData.latitude && this.globalData.longitude) {
              this.reverseGeocodeWhenReady()
            }
          })
          .catch((_err: unknown) => {
            logger.warn('复用 token 拉取用户信息失败，降级到完整登录', _err, 'App.silentLogin')
            clearToken()
            this._doWxLogin(0, MAX_LOGIN_RETRIES, RETRY_DELAYS)
          })
        return
      }

      // 中速路径：access token 已过期但 refresh_token 仍有效 → 静默续期
      const refreshToken = getRefreshToken() as string
      if (refreshToken) {
        logger.info('Token 已过期，尝试 refresh_token 静默续期', undefined, 'App.silentLogin')
        this._refreshThenLoadUser(refreshToken, MAX_LOGIN_RETRIES, RETRY_DELAYS)
        return
      }

      // 慢速路径：无任何可用凭证，走完整 wx.login
      logger.debug('无有效 token，开始完整 wx.login 流程', undefined, 'App.silentLogin')
      clearToken()
    } else {
      logger.info(`静默登录重试 (${attempt}/${MAX_LOGIN_RETRIES})`, undefined, 'App.silentLogin')
    }

    this._doWxLogin(attempt, MAX_LOGIN_RETRIES, RETRY_DELAYS)
  },

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  _applyUserInfo(user: any) {
    this.globalData.userId = user.id
    this.globalData.userInfo = {
      nickName: user.full_name || `User ${String(user.id).slice(-4)}`,
      avatarUrl: user.avatar_url || 'https://tdesign.gtimg.com/mobile/demos/avatar1.png',
      gender: 0,
      country: '',
      province: '',
      city: '',
      language: 'zh_CN'
    }
    if (user.roles.includes('MERCHANT')) {
      this.globalData.userRole = 'merchant'
    } else if (user.roles.includes('RIDER')) {
      this.globalData.userRole = 'rider'
    } else if (user.roles.includes('OPERATOR')) {
      this.globalData.userRole = 'operator'
    } else if (user.roles.includes('CUSTOMER')) {
      this.globalData.userRole = 'customer'
    }
  },

  /** 使用 refresh_token 静默续期，成功后拉取用户信息；失败则降级到 wx.login */
  _refreshThenLoadUser(refreshToken: string, max: number, delays: number[]) {
    const { API_BASE } = require('./utils/request')
    const { clearToken } = require('./utils/auth')

    wx.request({
      url: `${API_BASE}/v1/auth/refresh`,
      method: 'POST',
      data: { refresh_token: refreshToken },
      header: { 'Content-Type': 'application/json', 'X-Response-Envelope': '1' },
      timeout: 10000,
      success: (res) => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const body = res.data as any
        if (res.statusCode === 200 && body?.code === 0 && body?.data?.access_token) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          const d = body.data as any
          const expiresAt = d.access_token_expires_at
            ? new Date(d.access_token_expires_at).getTime()
            : undefined
          setToken(d.access_token, expiresAt, d.refresh_token)
          logger.info('refresh_token 续期成功，拉取用户信息', undefined, 'App._refreshThenLoadUser')
          getUserInfo()
            .then((user) => {
              this._applyUserInfo(user)
              logger.info('✅ 静默登录成功 (refresh_token)', { userId: user.id }, 'App._refreshThenLoadUser')
              if (this.globalData.latitude && this.globalData.longitude) {
                this.reverseGeocodeWhenReady()
              }
            })
            .catch((_err: unknown) => {
              logger.warn('refresh_token 续期后 getUserInfo 失败，降级到 wx.login', _err, 'App._refreshThenLoadUser')
              clearToken()
              this._doWxLogin(0, max, delays)
            })
        } else {
          logger.warn('refresh_token 已失效，降级到 wx.login', { statusCode: res.statusCode, code: body?.code }, 'App._refreshThenLoadUser')
          clearToken()
          this._doWxLogin(0, max, delays)
        }
      },
      fail: (err) => {
        logger.warn('refresh_token 请求失败，降级到 wx.login', err, 'App._refreshThenLoadUser')
        clearToken()
        this._doWxLogin(0, max, delays)
      }
    })
  },

  /** 完整的 wx.login → 后端 wechatLogin 流程（慢速路径及重试） */
  _doWxLogin(attempt: number, max: number, delays: number[]) {
    wx.login({
      success: async (res) => {
        if (res.code) {
          try {
            logger.debug('微信登录成功,code获取成功', { code: res.code.substring(0, 10) + '...' }, '_doWxLogin')

            const deviceId = getDeviceId()
            logger.debug('设备ID已生成', { deviceId: deviceId.substring(0, 15) + '...' }, '_doWxLogin')

            logger.debug('开始调用后端登录接口', undefined, '_doWxLogin')
            const loginData = await wechatLogin({
              code: res.code,
              device_id: deviceId,
              device_type: 'miniprogram'
            })

            const expiresAt = loginData.access_token_expires_at
              ? new Date(loginData.access_token_expires_at).getTime()
              : undefined
            setToken(loginData.access_token, expiresAt, loginData.refresh_token)

            const { getToken } = require('./utils/auth')
            const savedToken = getToken()
            if (!savedToken) {
              logger.error('Token保存失败！getToken()返回空', undefined, '_doWxLogin')
            }

            this._applyUserInfo(loginData.user)
            logger.info('✅ 静默登录完全成功 (wx.login)', {
              userId: loginData.user.id,
              userRole: this.globalData.userRole,
              hasToken: !!savedToken
            }, '_doWxLogin')

            if (this.globalData.latitude && this.globalData.longitude) {
              this.reverseGeocodeWhenReady()
            }

          } catch (error) {
            const appError = error as { message?: string }
            const errorMsg = appError.message || ''

            const isRetryable = errorMsg.includes('502') ||
              errorMsg.includes('503') ||
              errorMsg.includes('504') ||
              errorMsg.includes('timeout') ||
              errorMsg.includes('request:fail') ||
              errorMsg.includes('网络请求失败')

            if (isRetryable && attempt < max - 1) {
              const delay = delays[attempt + 1] || 4000
              logger.warn(`静默登录失败,${delay / 1000}秒后重试 (${attempt + 1}/${max})`, { error: errorMsg }, 'App._doWxLogin')
              setTimeout(() => this._doWxLogin(attempt + 1, max, delays), delay)
            } else {
              logger.warn('静默登录最终失败,用户可继续浏览', {
                error: errorMsg,
                attempts: attempt + 1
              }, 'App._doWxLogin')
            }
          }
        } else {
          logger.error('wx.login成功但未返回code', res, '_doWxLogin')
        }
      },
      fail: (err) => {
        logger.error('wx.login调用失败', err, 'App._doWxLogin')
      }
    })
  },

  isDemoMode(): boolean {
    const accountInfo = wx.getAccountInfoSync ? wx.getAccountInfoSync() : undefined
    const envVersion = accountInfo?.miniProgram?.envVersion
    return envVersion === 'develop' || envVersion === 'trial'
  },

  bootstrapDemoUser() {
    this.globalData.userInfo = {
      nickName: '演示账号',
      avatarUrl: 'https://tdesign.gtimg.com/mobile/demos/avatar1.png',
      gender: 0,
      country: 'CN',
      province: 'Beijing',
      city: 'Chaoyang',
      language: 'zh_CN'
    }
    this.globalData.userRole = 'customer'
    logger.warn('Demo mode: backend requests skipped,使用 mock 用户数据', undefined, 'App.bootstrapDemoUser')
  },

  /**
   * 获取位置坐标（不需要 token，本地调用）
   * 获取成功后，等待 token 准备好再调用逆地理编码
   * 注意：即使 globalData 中已有缓存坐标（来自 Storage 恢复），也始终刷新 GPS，
   * 成功后覆盖旧坐标，保证跨城场景下数据最终一致。
   */
  getLocationCoordinates() {
    logger.info('📍 开始获取位置坐标', undefined, 'getLocationCoordinates')

    // 获取当前位置坐标（本地调用，不需要网络请求）
    logger.debug('调用 wx.getLocation', undefined, 'getLocationCoordinates')
    wx.getLocation({
      type: 'gcj02', // 返回国测局坐标，适用于国内地图
      altitude: false, // 不需要高度信息
      success: (res) => {
        // 保存坐标到全局变量
        this.globalData.latitude = res.latitude
        this.globalData.longitude = res.longitude

        logger.info('✅ 坐标获取成功', {
          latitude: res.latitude,
          longitude: res.longitude
        }, 'getLocationCoordinates')

        // 等待 token 准备好后，调用逆地理编码
        this.reverseGeocodeWhenReady()
      },
      fail: (err) => {
        logger.error('❌ 坐标获取失败', err, 'getLocationCoordinates')

        // 设置 "定位失败" 文本
        this.globalData.location = { name: '定位失败' }

        // 同步到 globalStore
        const { globalStore } = require('./utils/global-store')
        globalStore.set('location', { name: '定位失败' })

        // 检查是否是权限问题
        if (err.errMsg && err.errMsg.includes('auth deny')) {
          logger.warn('⚠️ 位置权限被拒绝', undefined, 'getLocationCoordinates')

          // 提示用户授权（不阻塞，用户可以稍后在页面中手动选择）
          setTimeout(() => {
            wx.showModal({
              title: '需要位置权限',
              content: '本地生活服务需要获取您的位置信息，请允许位置权限',
              confirmText: '去设置',
              cancelText: '稍后',
              success: (res) => {
                if (res.confirm) {
                  wx.openSetting({
                    success: (settingRes) => {
                      if (settingRes.authSetting['scope.userLocation']) {
                        // 用户开启了权限，重新获取位置
                        logger.info('用户已开启位置权限，重新获取', undefined, 'getLocationCoordinates')
                        this.getLocationCoordinates()
                      }
                    }
                  })
                }
              }
            })
          }, 1000) // 延迟1秒，避免和其他弹窗冲突
        } else {
          // 其他错误（如网络问题、系统问题）
          logger.warn('⚠️ 位置获取失败（非权限问题）', err, 'getLocationCoordinates')
        }
      }
    })
  },

  /**
   * 等待 token 准备好后，调用逆地理编码
   */
  async reverseGeocodeWhenReady(retryCount = 0) {
    const MAX_RETRIES = 20 // 最多等待 10 秒
    const RETRY_INTERVAL = 500

    // 引入距离计算工具
    const { haversineDistance } = require('./utils/geo')

    // 检查是否已有缓存的坐标
    if (!this.globalData.latitude || !this.globalData.longitude) {
      logger.warn('坐标不存在，无法进行逆地理编码', undefined, 'reverseGeocodeWhenReady')
      return
    }

    // === 新增：位置更新优化策略 ===
    const lastLoc = this.globalData._lastLocationContext || { lat: 0, lng: 0, time: 0, name: '' }
    const now = Date.now()
    const TIME_THRESHOLD = 5 * 60 * 1000 // 5分钟
    const DISTANCE_THRESHOLD_KM = 0.05 // 50米

    const distance = haversineDistance(
      lastLoc.lat,
      lastLoc.lng,
      this.globalData.latitude,
      this.globalData.longitude
    )

    const isRecent = (now - lastLoc.time) < TIME_THRESHOLD
    const isSmallMove = distance < DISTANCE_THRESHOLD_KM
    const hasCachedName = !!lastLoc.name

    if (isRecent && isSmallMove && hasCachedName) {
      logger.info('📍 移动距离过小且时间较短，使用缓存位置名称', {
        distance: `${(distance * 1000).toFixed(1)}m`,
        cachedName: lastLoc.name
      }, 'reverseGeocodeWhenReady')

      // 复用上次的名称，但更新坐标
      this.globalData.location = {
        name: lastLoc.name,
        address: lastLoc.address || lastLoc.name
      }
      // 刷新缓存时间戳（坐标小幅移动，延长有效期）
      const _refreshedCtx = { lat: this.globalData.latitude, lng: this.globalData.longitude, time: now, name: lastLoc.name, address: lastLoc.address }
      this.globalData._lastLocationContext = _refreshedCtx
      try { wx.setStorageSync('last_known_location', _refreshedCtx) } catch (_e) { /* ignore */ }

      // 同步到 globalStore
      const { globalStore } = require('./utils/global-store')
      globalStore.updateLocation(
        this.globalData.latitude,
        this.globalData.longitude,
        lastLoc.name,
        lastLoc.address || lastLoc.name
      )
      return
    }
    // ============================

    // 检查 token 是否准备好
    const { getToken } = require('./utils/auth')
    const token = getToken()

    if (!token) {
      if (retryCount >= MAX_RETRIES) {
        logger.warn('⏰ Token 等待超时，逆地理编码失败', { retryCount }, 'reverseGeocodeWhenReady')
        this.globalData.location = { name: '定位失败' }
        return
      }

      // Token 未准备好，等待后重试
      if (retryCount === 0) {
        logger.debug('等待 token 准备好以进行逆地理编码...', undefined, 'reverseGeocodeWhenReady')
      }

      setTimeout(() => {
        this.reverseGeocodeWhenReady(retryCount + 1)
      }, RETRY_INTERVAL)
      return
    }

    // Token 已准备好，调用逆地理编码
    try {
      logger.debug('开始调用逆地理编码', {
        latitude: this.globalData.latitude,
        longitude: this.globalData.longitude,
        waitedTime: `${(retryCount * RETRY_INTERVAL) / 1000}秒`
      }, 'reverseGeocodeWhenReady')

      const locationInfo = await locationService.reverseGeocode(
        this.globalData.latitude,
        this.globalData.longitude
      )

      // 缓存位置信息到 globalData
      const fullAddress = locationInfo.formatted_address || locationInfo.address
      const locationName = locationInfo.street || locationInfo.district || fullAddress || '当前位置'
      this.globalData.location = {
        name: locationName,
        address: fullAddress
      }

      // === 更新缓存上下文 ===
      const _locCtx = {
        lat: this.globalData.latitude,
        lng: this.globalData.longitude,
        time: Date.now(),
        name: locationName,
        address: fullAddress
      }
      this.globalData._lastLocationContext = _locCtx
      // 持久化到 storage，供下次启动时立即恢复（避免等待 GPS）
      try { wx.setStorageSync('last_known_location', _locCtx) } catch (_e) { /* ignore */ }
      // ====================

      // 同步到 globalStore（导航栏等组件使用）
      const { globalStore } = require('./utils/global-store')
      globalStore.updateLocation(
        this.globalData.latitude!,
        this.globalData.longitude!,
        locationName,
        fullAddress
      )

      logger.info('✅ 逆地理编码成功，位置已缓存', {
        name: locationName,
        address: fullAddress,
        syncedToGlobalStore: true
      }, 'reverseGeocodeWhenReady')
    } catch (err) {
      // 逆地理编码失败
      this.globalData.location = {
        name: '定位失败',
        address: `${this.globalData.latitude.toFixed(6)}, ${this.globalData.longitude.toFixed(6)}`
      }

      // 同步到 globalStore
      const { globalStore } = require('./utils/global-store')
      globalStore.updateLocation(
        this.globalData.latitude!,
        this.globalData.longitude!,
        '定位失败',
        this.globalData.location.address
      )

      logger.warn('❌ 逆地理编码失败', err, 'reverseGeocodeWhenReady')
    }
  },

  /**
   * 获取位置（兼容旧代码，Demo 模式使用）
   */
  getLocation() {
    this.getLocationCoordinates()
  },

  /**
   * 清除 API 缓存（响应格式更新时需要清除旧缓存）
   */
  clearApiCache() {
    try {
      // 获取所有存储的 key
      const res = wx.getStorageInfoSync()
      const keysToRemove = res.keys.filter((key) => key.startsWith('api_'))

      keysToRemove.forEach((key) => {
        try {
          wx.removeStorageSync(key)
        } catch (e) {
          // 忽略单个 key 删除失败
        }
      })

      if (keysToRemove.length > 0) {
        logger.info('已清除 API 缓存', { count: keysToRemove.length }, 'clearApiCache')
      }
    } catch (e) {
      // 忽略缓存清除失败
      logger.warn('清除 API 缓存失败', e, 'clearApiCache')
    }
  }


})
