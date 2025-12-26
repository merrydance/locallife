/**
 * 响应式适配工具
 * 提供多设备屏幕尺寸检测和适配方案
 * 
 * 参考：https://mp.weixin.qq.com/s/3w1aZf86x2Im8jCy-CADBw
 */

export type DeviceType = 'mobile' | 'tablet' | 'desktop' | 'pc-full'

/** 平台类型（基于 wx.getDeviceInfo().platform） */
export type PlatformType = 'android' | 'ios' | 'ohos' | 'windows' | 'mac' | 'ohos_pc' | 'devtools' | string

export interface DeviceInfo {
  type: DeviceType
  screenWidth: number
  screenHeight: number
  pixelRatio: number
  windowWidth: number
  windowHeight: number
}

/** 平台信息（用于跨平台适配） */
export interface PlatformInfo {
  type: PlatformType       // platform 原始值
  isAndroid: boolean       // Android 手机
  isIos: boolean           // iOS 设备
  isOhos: boolean          // 鸿蒙 Next 手机
  isPc: boolean            // PC 端（Windows/Mac/鸿蒙PC）
  isDevtools: boolean      // 开发者工具
  isMobileDevice: boolean  // 是否为手机设备（Android/iOS/鸿蒙手机）
}

/**
 * 设备类型断点
 */
export const BREAKPOINTS = {
  mobile: 750,    // < 750px 为手机
  tablet: 1280,   // 750-1280px 为平板
  desktop: 1600,  // 1280-1600px 为桌面窗口
  pcFull: 1600    // >= 1600px 为桌面全屏
}

export function getDeviceInfo(): DeviceInfo {
  const windowInfo = wx.getWindowInfo()
  const windowWidth = windowInfo.windowWidth
  let type: DeviceType = 'mobile'
  if (windowWidth >= 1600) {
    type = 'pc-full'
  } else if (windowWidth >= 1280) {
    type = 'desktop'
  } else if (windowWidth >= 750) {
    type = 'tablet'
  }

  return {
    type,
    screenWidth: windowInfo.screenWidth,
    screenHeight: windowInfo.screenHeight,
    pixelRatio: windowInfo.pixelRatio,
    windowWidth: windowInfo.windowWidth,
    windowHeight: windowInfo.windowHeight
  }
}

/**
 * 获取稳定的系统Bar高度 (防止大屏下过度放大)
 */
export function getStableBarHeights() {
  const windowInfo = wx.getWindowInfo()
  const deviceInfo = wx.getDeviceInfo()
  const isLarge = isLargeScreen()

  // 状态栏高度
  let statusBarHeight = windowInfo.statusBarHeight

  // 导航栏内容区高度 (标准为44px)
  let navBarContentHeight = 44

  // 如果是大屏且是PC，锁定高度为 64px
  if (isLarge) {
    const platform = deviceInfo.platform
    if (platform === 'windows' || platform === 'mac' || platform === 'devtools') {
      // PC端通常只需要固定的导航栏，且不需要状态栏
      statusBarHeight = 0
      navBarContentHeight = 64
    }
  }

  return {
    statusBarHeight,
    navBarContentHeight,
    navBarHeight: statusBarHeight + navBarContentHeight
  }
}

/**
 * 判断是否为手机
 */
export function isMobile(): boolean {
  return getDeviceInfo().type === 'mobile'
}

/**
 * 判断是否为平板
 */
export function isTablet(): boolean {
  return getDeviceInfo().type === 'tablet'
}

/**
 * 判断是否为桌面
 */
export function isDesktop(): boolean {
  return getDeviceInfo().type === 'desktop'
}

/**
 * 根据设备类型返回不同的值
 */
export function responsive<T>(values: {
  mobile: T
  tablet?: T
  desktop?: T
  pcFull?: T
}): T {
  const deviceType = getDeviceInfo().type

  if (deviceType === 'pc-full' && values.pcFull !== undefined) {
    return values.pcFull
  }

  if (deviceType === 'desktop' && values.desktop !== undefined) {
    return values.desktop
  }

  if (deviceType === 'tablet' && values.tablet !== undefined) {
    return values.tablet
  }

  return values.mobile
}

/**
 * 计算响应式列数
 */
export function getResponsiveColumns(options: {
  mobile: number
  tablet?: number
  desktop?: number
}): number {
  return responsive({
    mobile: options.mobile,
    tablet: options.tablet || options.mobile * 2,
    desktop: options.desktop || options.mobile * 3
  })
}

/**
 * 计算响应式间距
 */
export function getResponsiveSpacing(baseSpacing: number): number {
  return responsive({
    mobile: baseSpacing,
    tablet: baseSpacing * 1.5,
    desktop: baseSpacing * 2
  })
}

/**
 * 获取平台信息（用于跨平台适配）
 * 
 * platform 可能的值：
 * - 手机：android, ios, ohos (鸿蒙 Next)
 * - 电脑：windows, mac, ohos_pc (鸿蒙 PC)
 * - 开发工具：devtools
 */
export function getPlatformInfo(): PlatformInfo {
  const info = wx.getDeviceInfo()
  const platform = info.platform as PlatformType

  return {
    type: platform,
    isAndroid: platform === 'android',
    isIos: platform === 'ios',
    isOhos: platform === 'ohos',
    isPc: platform === 'windows' || platform === 'mac' || platform === 'ohos_pc',
    isDevtools: platform === 'devtools',
    isMobileDevice: platform === 'android' || platform === 'ios' || platform === 'ohos'
  }
}

/**
 * 判断是否为 Android 设备
 */
export function isAndroid(): boolean {
  return getPlatformInfo().isAndroid
}

/**
 * 判断是否为 iOS 设备
 */
export function isIOS(): boolean {
  return getPlatformInfo().isIos
}

/**
 * 判断是否为鸿蒙 Next 设备
 */
export function isHarmonyOS(): boolean {
  return getPlatformInfo().isOhos
}

/**
 * 判断是否为 PC 端（Windows/Mac/鸿蒙PC）
 */
export function isPCPlatform(): boolean {
  return getPlatformInfo().isPc
}

/**
 * 判断是否为手机设备（Android/iOS/鸿蒙手机）
 */
export function isMobileDevice(): boolean {
  return getPlatformInfo().isMobileDevice
}

/**
 * 根据平台类型返回不同的值
 * 用于处理不同平台的差异化逻辑
 */
export function platformResponsive<T>(values: {
  android?: T
  ios?: T
  ohos?: T      // 鸿蒙手机
  pc?: T        // PC 端统一处理
  default: T    // 默认值
}): T {
  const platform = getPlatformInfo()

  if (platform.isAndroid && values.android !== undefined) {
    return values.android
  }
  if (platform.isIos && values.ios !== undefined) {
    return values.ios
  }
  if (platform.isOhos && values.ohos !== undefined) {
    return values.ohos
  }
  if (platform.isPc && values.pc !== undefined) {
    return values.pc
  }

  return values.default
}

/**
 * 获取全局布局初始化数据
 * 用于 Page.data 或 Component.data
 */
export function getGlobalLayoutData() {
  const deviceInfo = getDeviceInfo()
  const barHeights = getStableBarHeights()
  return {
    isLargeScreen: deviceInfo.type !== 'mobile',
    deviceType: deviceInfo.type,
    navBarHeight: barHeights.navBarHeight,
    statusBarHeight: barHeights.statusBarHeight
  }
}

/**
 * 响应式 Behavior (推荐使用)
 * 自动注入 isLargeScreen, navBarHeight 等数据到页面
 */
export const responsiveBehavior = Behavior({
  data: getGlobalLayoutData(),
  lifetimes: {
    attached() {
      // 可以在此处监听窗口大小变化（如果需要实时响应）
      // if (wx.onWindowResize) { ... }
    }
  }
})

export function isLargeScreen(): boolean {
  const deviceType = getDeviceInfo().type
  return deviceType === 'tablet' || deviceType === 'desktop' || deviceType === 'pc-full'
}
