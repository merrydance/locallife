/**
 * 环境配置
 */

// 微信小程序全局配置类型声明
declare const __wxConfig: {
  envVersion: 'develop' | 'trial' | 'release'
} | undefined

// 检测环境
export const ENV = {
  // 开发环境: develop, 体验版: trial, 正式版: release
  version: typeof __wxConfig !== 'undefined' ? __wxConfig.envVersion : 'release',

  // 是否为开发环境
  isDev: typeof __wxConfig !== 'undefined' && __wxConfig.envVersion === 'develop',

  // 是否为生产环境
  isProd: typeof __wxConfig === 'undefined' || __wxConfig.envVersion === 'release'
}

// API配置
export const API_CONFIG = {
  // 生产环境API地址 (移除/api段，后端basePath为/v1)
  PROD_BASE_URL: 'https://llapi.merrydance.cn',

  // 开发环境API地址(可配置)
  DEV_BASE_URL: 'https://llapi.merrydance.cn',

  // 获取当前环境的API地址
  get BASE_URL() {
    return ENV.isDev ? this.DEV_BASE_URL : this.PROD_BASE_URL
  },

  // 请求超时时间(毫秒)
  TIMEOUT: 30000,

  // 最大重试次数
  MAX_RETRY: 3,

  // 重试延迟(毫秒)
  RETRY_DELAY: 1000
}

// 错误提示配置
export const ERROR_CONFIG = {
  // 是否显示详细错误信息
  SHOW_DETAIL: ENV.isDev,

  // Toast持续时间
  TOAST_DURATION: 2500,

  // 网络错误提示
  NETWORK_ERROR_MESSAGES: {
    TIMEOUT: '请求超时,请检查网络连接',
    NO_NETWORK: '网络不可用,请检查网络设置',
    SERVER_ERROR: '服务暂时不可用,请稍后重试',
    BACKEND_DOWN: '后端服务未启动 - 请联系管理员',
    NGINX_ERROR: '服务网关错误 - 请检查Nginx配置'
  }
}

// 日志配置
export const LOG_CONFIG = {
  // 是否启用控制台日志
  CONSOLE_ENABLED: true,

  // 是否上报远程日志
  REMOTE_ENABLED: ENV.isProd,

  // 日志级别: debug, info, warn, error
  LEVEL: ENV.isDev ? 'debug' : 'info'
}

// 缓存配置
export const CACHE_CONFIG = {
  // 默认缓存时间(毫秒)
  DEFAULT_TTL: 5 * 60 * 1000, // 5分钟

  // API缓存时间
  API_CACHE_TTL: {
    DISH_LIST: 10 * 60 * 1000,      // 菜品列表: 10分钟
    MERCHANT_INFO: 30 * 60 * 1000,  // 商户信息: 30分钟
    USER_INFO: 60 * 60 * 1000       // 用户信息: 1小时
  }
}

// 调试配置
export const DEBUG_CONFIG = {
  // 是否启用Mock数据
  MOCK_ENABLED: false,

  // 是否显示性能监控
  PERFORMANCE_MONITOR: ENV.isDev,

  // 是否显示网络请求日志
  LOG_NETWORK: ENV.isDev,

  // 是否启用调试面板
  DEBUG_PANEL: ENV.isDev
}

export default {
  ENV,
  API_CONFIG,
  ERROR_CONFIG,
  LOG_CONFIG,
  CACHE_CONFIG,
  DEBUG_CONFIG
}
