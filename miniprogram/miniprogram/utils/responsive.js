"use strict";
/**
 * 响应式适配工具
 * 提供多设备屏幕尺寸检测和适配方案
 *
 * 参考：https://mp.weixin.qq.com/s/3w1aZf86x2Im8jCy-CADBw
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.responsiveBehavior = exports.BREAKPOINTS = void 0;
exports.getDeviceInfo = getDeviceInfo;
exports.getStableBarHeights = getStableBarHeights;
exports.isMobile = isMobile;
exports.isTablet = isTablet;
exports.isDesktop = isDesktop;
exports.responsive = responsive;
exports.getResponsiveColumns = getResponsiveColumns;
exports.getResponsiveSpacing = getResponsiveSpacing;
exports.getPlatformInfo = getPlatformInfo;
exports.isAndroid = isAndroid;
exports.isIOS = isIOS;
exports.isHarmonyOS = isHarmonyOS;
exports.isPCPlatform = isPCPlatform;
exports.isMobileDevice = isMobileDevice;
exports.platformResponsive = platformResponsive;
exports.getGlobalLayoutData = getGlobalLayoutData;
exports.isLargeScreen = isLargeScreen;
/**
 * 设备类型断点
 */
exports.BREAKPOINTS = {
    mobile: 750, // < 750px 为手机
    tablet: 1280, // 750-1280px 为平板
    desktop: 1600, // 1280-1600px 为桌面窗口
    pcFull: 1600 // >= 1600px 为桌面全屏
};
function getDeviceInfo() {
    const windowInfo = wx.getWindowInfo();
    const windowWidth = windowInfo.windowWidth;
    let type = 'mobile';
    if (windowWidth >= 1600) {
        type = 'pc-full';
    }
    else if (windowWidth >= 1280) {
        type = 'desktop';
    }
    else if (windowWidth >= 750) {
        type = 'tablet';
    }
    return {
        type,
        screenWidth: windowInfo.screenWidth,
        screenHeight: windowInfo.screenHeight,
        pixelRatio: windowInfo.pixelRatio,
        windowWidth: windowInfo.windowWidth,
        windowHeight: windowInfo.windowHeight
    };
}
/**
 * 获取稳定的系统Bar高度 (防止大屏下过度放大)
 */
function getStableBarHeights() {
    const windowInfo = wx.getWindowInfo();
    const deviceInfo = wx.getDeviceInfo();
    const isLarge = isLargeScreen();
    // 状态栏高度
    let statusBarHeight = windowInfo.statusBarHeight;
    // 导航栏内容区高度 (标准为44px)
    let navBarContentHeight = 44;
    // 如果是大屏且是PC，锁定高度为 64px
    if (isLarge) {
        const platform = deviceInfo.platform;
        if (platform === 'windows' || platform === 'mac' || platform === 'devtools') {
            // PC端通常只需要固定的导航栏，且不需要状态栏
            statusBarHeight = 0;
            navBarContentHeight = 64;
        }
    }
    return {
        statusBarHeight,
        navBarContentHeight,
        navBarHeight: statusBarHeight + navBarContentHeight
    };
}
/**
 * 判断是否为手机
 */
function isMobile() {
    return getDeviceInfo().type === 'mobile';
}
/**
 * 判断是否为平板
 */
function isTablet() {
    return getDeviceInfo().type === 'tablet';
}
/**
 * 判断是否为桌面
 */
function isDesktop() {
    return getDeviceInfo().type === 'desktop';
}
/**
 * 根据设备类型返回不同的值
 */
function responsive(values) {
    const deviceType = getDeviceInfo().type;
    if (deviceType === 'pc-full' && values.pcFull !== undefined) {
        return values.pcFull;
    }
    if (deviceType === 'desktop' && values.desktop !== undefined) {
        return values.desktop;
    }
    if (deviceType === 'tablet' && values.tablet !== undefined) {
        return values.tablet;
    }
    return values.mobile;
}
/**
 * 计算响应式列数
 */
function getResponsiveColumns(options) {
    return responsive({
        mobile: options.mobile,
        tablet: options.tablet || options.mobile * 2,
        desktop: options.desktop || options.mobile * 3
    });
}
/**
 * 计算响应式间距
 */
function getResponsiveSpacing(baseSpacing) {
    return responsive({
        mobile: baseSpacing,
        tablet: baseSpacing * 1.5,
        desktop: baseSpacing * 2
    });
}
/**
 * 获取平台信息（用于跨平台适配）
 *
 * platform 可能的值：
 * - 手机：android, ios, ohos (鸿蒙 Next)
 * - 电脑：windows, mac, ohos_pc (鸿蒙 PC)
 * - 开发工具：devtools
 */
function getPlatformInfo() {
    const info = wx.getDeviceInfo();
    const platform = info.platform;
    return {
        type: platform,
        isAndroid: platform === 'android',
        isIos: platform === 'ios',
        isOhos: platform === 'ohos',
        isPc: platform === 'windows' || platform === 'mac' || platform === 'ohos_pc',
        isDevtools: platform === 'devtools',
        isMobileDevice: platform === 'android' || platform === 'ios' || platform === 'ohos'
    };
}
/**
 * 判断是否为 Android 设备
 */
function isAndroid() {
    return getPlatformInfo().isAndroid;
}
/**
 * 判断是否为 iOS 设备
 */
function isIOS() {
    return getPlatformInfo().isIos;
}
/**
 * 判断是否为鸿蒙 Next 设备
 */
function isHarmonyOS() {
    return getPlatformInfo().isOhos;
}
/**
 * 判断是否为 PC 端（Windows/Mac/鸿蒙PC）
 */
function isPCPlatform() {
    return getPlatformInfo().isPc;
}
/**
 * 判断是否为手机设备（Android/iOS/鸿蒙手机）
 */
function isMobileDevice() {
    return getPlatformInfo().isMobileDevice;
}
/**
 * 根据平台类型返回不同的值
 * 用于处理不同平台的差异化逻辑
 */
function platformResponsive(values) {
    const platform = getPlatformInfo();
    if (platform.isAndroid && values.android !== undefined) {
        return values.android;
    }
    if (platform.isIos && values.ios !== undefined) {
        return values.ios;
    }
    if (platform.isOhos && values.ohos !== undefined) {
        return values.ohos;
    }
    if (platform.isPc && values.pc !== undefined) {
        return values.pc;
    }
    return values.default;
}
/**
 * 获取全局布局初始化数据
 * 用于 Page.data 或 Component.data
 */
function getGlobalLayoutData() {
    const deviceInfo = getDeviceInfo();
    const barHeights = getStableBarHeights();
    return {
        isLargeScreen: deviceInfo.type !== 'mobile',
        deviceType: deviceInfo.type,
        navBarHeight: barHeights.navBarHeight,
        statusBarHeight: barHeights.statusBarHeight
    };
}
/**
 * 响应式 Behavior (推荐使用)
 * 自动注入 isLargeScreen, navBarHeight 等数据到页面
 */
exports.responsiveBehavior = Behavior({
    data: getGlobalLayoutData(),
    lifetimes: {
        attached() {
            // 可以在此处监听窗口大小变化（如果需要实时响应）
            // if (wx.onWindowResize) { ... }
        }
    }
});
function isLargeScreen() {
    const deviceType = getDeviceInfo().type;
    return deviceType === 'tablet' || deviceType === 'desktop' || deviceType === 'pc-full';
}
