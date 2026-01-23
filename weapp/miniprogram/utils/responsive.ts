/**
 * 响应式适配工具 (已简化为仅支持移动端)
 * 移除大屏及 PC 端适配逻辑
 */

export type DeviceType = 'mobile'

export interface DeviceInfo {
  type: DeviceType
  screenWidth: number
  screenHeight: number
  pixelRatio: number
  windowWidth: number
  windowHeight: number
}

export function getDeviceInfo(): DeviceInfo {
  const windowInfo = wx.getWindowInfo()
  return {
    type: 'mobile',
    screenWidth: windowInfo.screenWidth,
    screenHeight: windowInfo.screenHeight,
    pixelRatio: windowInfo.pixelRatio,
    windowWidth: windowInfo.windowWidth,
    windowHeight: windowInfo.windowHeight
  }
}

/**
 * 获取系统 Bar 高度
 */
export function getStableBarHeights() {
  const windowInfo = wx.getWindowInfo()
  const statusBarHeight = windowInfo.statusBarHeight || 20
  const navBarContentHeight = 44

  return {
    statusBarHeight,
    navBarContentHeight,
    navBarHeight: statusBarHeight + navBarContentHeight
  }
}

export function isMobile(): boolean { return true }
export function isTablet(): boolean { return false }
export function isDesktop(): boolean { return false }
export function isLargeScreen(): boolean { return false }

export function responsive<T>(values: { mobile: T }): T {
  return values.mobile
}

export function getPlatformInfo() {
  const info = wx.getDeviceInfo()
  return {
    type: info.platform,
    isPc: false,
    isMobileDevice: true
  }
}

export function getGlobalLayoutData() {
  const barHeights = getStableBarHeights()
  return {
    isLargeScreen: false,
    deviceType: 'mobile',
    navBarHeight: barHeights.navBarHeight,
    statusBarHeight: barHeights.statusBarHeight
  }
}

export const responsiveBehavior = Behavior({
  data: getGlobalLayoutData()
})
