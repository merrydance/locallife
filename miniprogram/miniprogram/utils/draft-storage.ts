/**
 * 草稿存储工具
 * 用于保存和加载表单草稿，防止用户数据丢失
 */

import { logger } from './logger'

export const DraftStorage = {
  /**
     * 保存草稿
     * @param key 存储键名
     * @param data 要保存的数据
     */
  save(key: string, data: Record<string, unknown>) {
    try {
      wx.setStorageSync(key, {
        data,
        timestamp: Date.now()
      })
      logger.debug('草稿已保存', { key }, 'DraftStorage.save')
    } catch (e) {
      logger.error('保存草稿失败', e, 'DraftStorage.save')
    }
  },

  /**
     * 加载草稿
     * @param key 存储键名
     * @returns 草稿数据或 null
     */
  load<T = Record<string, unknown>>(key: string): T | null {
    try {
      const draft = wx.getStorageSync(key)
      if (draft && draft.data) {
        // 可选：检查过期时间，例如 7 天
        const sevenDays = 7 * 24 * 60 * 60 * 1000
        if (Date.now() - draft.timestamp > sevenDays) {
          wx.removeStorageSync(key)
          logger.debug('草稿已过期', { key }, 'DraftStorage.load')
          return null
        }
        logger.debug('草稿已加载', { key }, 'DraftStorage.load')
        return draft.data
      }
    } catch (e) {
      logger.error('加载草稿失败', e, 'DraftStorage.load')
    }
    return null
  },

  /**
     * 清除草稿
     * @param key 存储键名
     */
  clear(key: string) {
    try {
      wx.removeStorageSync(key)
      logger.debug('草稿已清除', { key }, 'DraftStorage.clear')
    } catch (e) {
      logger.error('清除草稿失败', e, 'DraftStorage.clear')
    }
  }
}
