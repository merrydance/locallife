/**
 * 主题管理器
 * 支持深色模式/浅色模式切换
 */

import { logger } from './logger'

export enum ThemeMode {
  LIGHT = 'light',
  DARK = 'dark',
  AUTO = 'auto' // 跟随系统
}

interface ThemeColors {
  // 品牌色
  brandPrimary: string
  brandSecondary: string
  brandAccent: string

  // 背景色
  bgPage: string
  bgContainer: string
  bgCard: string
  bgOverlay: string

  // 文字色
  textPrimary: string
  textSecondary: string
  textTertiary: string
  textInverse: string

  // 边框色
  borderBase: string
  borderLight: string

  // 状态色
  success: string
  warning: string
  error: string
  info: string
}

const LIGHT_THEME: ThemeColors = {
  brandPrimary: '#FF6B58',
  brandSecondary: '#00897B',
  brandAccent: '#FFC107',

  bgPage: '#FAFAFA',
  bgContainer: '#FFFFFF',
  bgCard: '#FFFFFF',
  bgOverlay: 'rgba(0, 0, 0, 0.5)',

  textPrimary: '#333333',
  textSecondary: '#666666',
  textTertiary: '#999999',
  textInverse: '#FFFFFF',

  borderBase: '#E5E5E5',
  borderLight: '#F0F0F0',

  success: '#00897B',
  warning: '#FFC107',
  error: '#FF5252',
  info: '#2196F3'
}

const DARK_THEME: ThemeColors = {
  brandPrimary: '#FF8A7A',
  brandSecondary: '#26A69A',
  brandAccent: '#FFD54F',

  bgPage: '#121212',
  bgContainer: '#1E1E1E',
  bgCard: '#2C2C2C',
  bgOverlay: 'rgba(0, 0, 0, 0.7)',

  textPrimary: '#E0E0E0',
  textSecondary: '#B0B0B0',
  textTertiary: '#808080',
  textInverse: '#121212',

  borderBase: '#3C3C3C',
  borderLight: '#2C2C2C',

  success: '#26A69A',
  warning: '#FFD54F',
  error: '#FF6B6B',
  info: '#64B5F6'
}

class ThemeManager {
  private currentMode: ThemeMode = ThemeMode.LIGHT
  private systemDarkMode: boolean = false
  private readonly STORAGE_KEY = 'theme_mode'

  constructor() {
    this.init()
  }

  /**
   * 初始化主题
   */
  private init(): void {
    // 读取系统深色模式状态
    try {
      const appBaseInfo = wx.getAppBaseInfo()
      this.systemDarkMode = appBaseInfo.theme === 'dark'
    } catch (e) {
      logger.warn('无法获取系统主题', e, 'ThemeManager')
    }

    // 读取用户偏好
    try {
      const savedMode = wx.getStorageSync(this.STORAGE_KEY) as ThemeMode
      const validModes = [ThemeMode.LIGHT, ThemeMode.DARK, ThemeMode.AUTO]
      if (savedMode && validModes.includes(savedMode)) {
        this.currentMode = savedMode
      }
    } catch (e) {
      logger.warn('无法读取主题偏好', e, 'ThemeManager')
    }

    // 监听系统主题变化
    wx.onThemeChange((res) => {
      this.systemDarkMode = res.theme === 'dark'
      if (this.currentMode === ThemeMode.AUTO) {
        this.applyTheme()
      }
    })

    // 应用主题
    this.applyTheme()
  }

  /**
   * 设置主题模式
   */
  setMode(mode: ThemeMode): void {
    this.currentMode = mode

    try {
      wx.setStorageSync(this.STORAGE_KEY, mode)
    } catch (e) {
      logger.error('保存主题偏好失败', e, 'ThemeManager')
    }

    this.applyTheme()
    logger.info(`主题切换为: ${mode}`, undefined, 'ThemeManager')
  }

  /**
   * 获取当前模式
   */
  getMode(): ThemeMode {
    return this.currentMode
  }

  /**
   * 判断当前是否为深色模式
   */
  isDark(): boolean {
    if (this.currentMode === ThemeMode.DARK) return true
    if (this.currentMode === ThemeMode.LIGHT) return false
    return this.systemDarkMode // AUTO 模式跟随系统
  }

  /**
   * 获取当前主题颜色
   */
  getColors(): ThemeColors {
    return this.isDark() ? DARK_THEME : LIGHT_THEME
  }

  /**
   * 应用主题到页面
   */
  private applyTheme(): void {
    const colors = this.getColors()
    const isDark = this.isDark()

    // 更新 CSS 变量
    const styleContent = this.generateCSSVariables(colors)

    // 尝试更新页面样式
    try {
      const pages = getCurrentPages()
      if (pages.length > 0) {
        const currentPage = pages[pages.length - 1]
        if (currentPage) {
          // 触发页面重新渲染
          currentPage.setData({
            __theme__: isDark ? 'dark' : 'light',
            __themeColors__: colors
          })
        }
      }
    } catch (e) {
      logger.warn('应用主题失败', e, 'ThemeManager')
    }

    // 在 app.wxss 中动态注入
    this.injectGlobalStyles(styleContent)

    logger.debug('主题已应用', { isDark, colors }, 'ThemeManager')
  }

  /**
   * 生成 CSS 变量
   */
  private generateCSSVariables(colors: ThemeColors): string {
    return `
      page {
        --brand-primary: ${colors.brandPrimary};
        --brand-secondary: ${colors.brandSecondary};
        --brand-accent: ${colors.brandAccent};
        
        --bg-page: ${colors.bgPage};
        --bg-container: ${colors.bgContainer};
        --bg-card: ${colors.bgCard};
        --bg-overlay: ${colors.bgOverlay};
        
        --text-primary: ${colors.textPrimary};
        --text-secondary: ${colors.textSecondary};
        --text-tertiary: ${colors.textTertiary};
        --text-inverse: ${colors.textInverse};
        
        --border-base: ${colors.borderBase};
        --border-light: ${colors.borderLight};
        
        --success: ${colors.success};
        --warning: ${colors.warning};
        --error: ${colors.error};
        --info: ${colors.info};
      }
    `
  }

  /**
   * 注入全局样式（仅在支持的环境）
   */
  private injectGlobalStyles(_styleContent: string): void {
    // 微信小程序不支持动态注入全局样式
    // 需要在 app.wxss 中预定义暗色模式样式
    // 这里仅作占位，实际主题切换通过 CSS 变量和 data 属性实现
  }

  /**
   * 切换主题（Light <-> Dark）
   */
  toggle(): void {
    const newMode = this.isDark() ? ThemeMode.LIGHT : ThemeMode.DARK
    this.setMode(newMode)
  }
}

// 导出单例
export const themeManager = new ThemeManager()
