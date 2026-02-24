import { tracker, EventType } from './utils/tracker'
import { wechatLogin, getDeviceId } from './api/auth'
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



    // 预初始化全局布局数据 (Phase 1)
    const { getGlobalLayoutData } = require('./utils/responsive')
    const layout = getGlobalLayoutData()
    const { globalStore } = require('./utils/global-store')
    globalStore.set('navBarHeight', layout.navBarHeight)
    globalStore.set('isLargeScreen', layout.isLargeScreen)

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

      // 清除旧 token，避免旧 token 导致的刷新循环
      const { clearToken } = require('./utils/auth')
      clearToken()
      logger.debug('已清除旧 token，准备重新登录', undefined, 'silentLogin')
    } else {
      logger.info(`静默登录重试 (${attempt}/${MAX_LOGIN_RETRIES})`, undefined, 'App.silentLogin')
    }

    wx.login({
      success: async (res) => {
        if (res.code) {
          try {
            logger.debug('微信登录成功,code获取成功', { code: res.code.substring(0, 10) + '...' }, 'silentLogin')

            // 获取设备ID
            const deviceId = getDeviceId()
            logger.debug('设备ID已生成', { deviceId: deviceId.substring(0, 15) + '...' }, 'silentLogin')

            // 调用登录接口
            logger.debug('开始调用后端登录接口', undefined, 'silentLogin')
            const loginData = await wechatLogin({
              code: res.code,
              device_id: deviceId,
              device_type: 'miniprogram'
            })

            logger.debug('后端登录接口调用成功', {
              userId: loginData.user.id,
              hasToken: !!loginData.access_token
            }, 'silentLogin')

            // 保存token
            const expiresAt = loginData.access_token_expires_at
              ? new Date(loginData.access_token_expires_at).getTime()
              : undefined
            setToken(
              loginData.access_token,
              expiresAt,
              loginData.refresh_token
            )

            logger.info('Token已保存', {
              tokenLength: loginData.access_token.length,
              expiresAt: loginData.access_token_expires_at
            }, 'silentLogin')

            // 验证token是否真的保存成功
            const { getToken } = require('./utils/auth')
            const savedToken = getToken()
            if (!savedToken) {
              logger.error('Token保存失败！getToken()返回空', undefined, 'silentLogin')
            } else {
              logger.debug('Token保存验证成功', { tokenLength: savedToken.length }, 'silentLogin')
            }

            // 保存用户信息
            const user = loginData.user
            this.globalData.userId = user.id
            this.globalData.userInfo = {
              nickName: user.full_name || `User ${user.id.toString().slice(-4)}`,
              avatarUrl: user.avatar_url || 'https://tdesign.gtimg.com/mobile/demos/avatar1.png',
              gender: 0,
              country: '',
              province: '',
              city: '',
              language: 'zh_CN'
            }

            // 确定用户角色
            if (user.roles.includes('MERCHANT')) {
              this.globalData.userRole = 'merchant'
              logger.info('用户角色: 商户', undefined, 'silentLogin')
            } else if (user.roles.includes('RIDER')) {
              this.globalData.userRole = 'rider'
              logger.info('用户角色: 骑手', undefined, 'silentLogin')
            } else if (user.roles.includes('OPERATOR')) {
              this.globalData.userRole = 'operator'
              logger.info('用户角色: 运营商', undefined, 'silentLogin')
            } else if (user.roles.includes('CUSTOMER')) {
              this.globalData.userRole = 'customer'
              logger.info('用户角色: 顾客', undefined, 'silentLogin')
            }

            logger.info('✅ 静默登录完全成功', {
              userId: user.id,
              userRole: this.globalData.userRole,
              hasToken: !!savedToken
            }, 'silentLogin')

            // 登录成功后，如果已有坐标，立即进行逆地理编码
            if (this.globalData.latitude && this.globalData.longitude) {
              logger.debug('登录成功，立即进行逆地理编码', undefined, 'silentLogin')
              this.reverseGeocodeWhenReady()
            }

          } catch (error) {
            // 静默登录失败：仅记录日志，不弹 Toast（页面层的 tryLoadData 会处理超时引导）
            const appError = error as { message?: string }
            const errorMsg = appError.message || ''

            // 判断是否可重试的错误（网络/后端不可用）
            const isRetryable = errorMsg.includes('502') ||
              errorMsg.includes('503') ||
              errorMsg.includes('504') ||
              errorMsg.includes('timeout') ||
              errorMsg.includes('request:fail') ||
              errorMsg.includes('网络请求失败')

            if (isRetryable && attempt < MAX_LOGIN_RETRIES - 1) {
              const delay = RETRY_DELAYS[attempt + 1] || 4000
              logger.warn(`静默登录失败,${delay / 1000}秒后重试 (${attempt + 1}/${MAX_LOGIN_RETRIES})`, { error: errorMsg }, 'App.silentLogin')
              setTimeout(() => this.silentLogin(attempt + 1), delay)
            } else {
              // 已达最大重试次数或不可重试的错误，仅记录日志
              logger.warn('静默登录最终失败,用户可继续浏览', {
                error: errorMsg,
                attempts: attempt + 1
              }, 'App.silentLogin')
            }
          }
        } else {
          logger.error('wx.login成功但未返回code', res, 'App.silentLogin')
        }
      },
      fail: (err) => {
        // wx.login 本身失败（极罕见），仅记录日志
        logger.error('wx.login调用失败', err, 'App.wx.login')
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
   */
  getLocationCoordinates() {
    logger.info('📍 开始获取位置坐标', undefined, 'getLocationCoordinates')

    // 检查是否已有缓存的坐标
    if (this.globalData.latitude && this.globalData.longitude) {
      logger.info('使用缓存的坐标', {
        latitude: this.globalData.latitude,
        longitude: this.globalData.longitude
      }, 'getLocationCoordinates')
      return
    }

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
      this.globalData._lastLocationContext = {
        lat: this.globalData.latitude,
        lng: this.globalData.longitude,
        time: Date.now(),
        name: locationName,
        address: fullAddress
      }
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
