/**
 * 缓存管理器
 * 支持内存缓存和本地存储缓存
 */

import { logger } from './logger'

interface CacheItem<T> {
  data: T
  timestamp: number
  ttl: number  // 过期时间(毫秒)
}

export enum CacheStrategy {
  MEMORY = 'MEMORY',           // 仅内存缓存
  STORAGE = 'STORAGE',         // 仅本地存储
  MEMORY_FIRST = 'MEMORY_FIRST' // 优先内存,降级到本地存储
}

export class CacheManager {
  private memoryCache: Map<string, CacheItem<unknown>> = new Map()
  private readonly STORAGE_PREFIX = 'cache_'

  /**
     * 设置缓存
     */
  set<T>(
    key: string,
    data: T,
    ttl: number = 5 * 60 * 1000,  // 默认5分钟
    strategy: CacheStrategy = CacheStrategy.MEMORY
  ): void {
    const item: CacheItem<T> = {
      data,
      timestamp: Date.now(),
      ttl
    }

    try {
      if (strategy === CacheStrategy.MEMORY || strategy === CacheStrategy.MEMORY_FIRST) {
        this.memoryCache.set(key, item)
        logger.debug(`缓存已设置(内存): ${key}`, { ttl }, 'CacheManager')
      }

      if (strategy === CacheStrategy.STORAGE || strategy === CacheStrategy.MEMORY_FIRST) {
        wx.setStorageSync(this.STORAGE_PREFIX + key, item)
        logger.debug(`缓存已设置(存储): ${key}`, { ttl }, 'CacheManager')
      }
    } catch (error) {
      logger.error('设置缓存失败', error, 'CacheManager')
    }
  }

  /**
     * 获取缓存
     */
  get<T>(key: string, strategy: CacheStrategy = CacheStrategy.MEMORY): T | null {
    try {
      // 优先从内存获取
      if (strategy === CacheStrategy.MEMORY || strategy === CacheStrategy.MEMORY_FIRST) {
        const item = this.memoryCache.get(key)
        if (item && this.isValid(item)) {
          logger.debug(`缓存命中(内存): ${key}`, undefined, 'CacheManager')
          return item.data as T
        }
        if (item) {
          // 过期了,删除
          this.memoryCache.delete(key)
        }
      }

      // 从本地存储获取
      if (strategy === CacheStrategy.STORAGE || strategy === CacheStrategy.MEMORY_FIRST) {
        const item = wx.getStorageSync(this.STORAGE_PREFIX + key) as CacheItem<T>
        if (item && this.isValid(item)) {
          logger.debug(`缓存命中(存储): ${key}`, undefined, 'CacheManager')

          // 如果是MEMORY_FIRST策略,回填到内存
          if (strategy === CacheStrategy.MEMORY_FIRST) {
            this.memoryCache.set(key, item)
          }

          return item.data
        }
        if (item) {
          // 过期了,删除
          wx.removeStorageSync(this.STORAGE_PREFIX + key)
        }
      }

      logger.debug(`缓存未命中: ${key}`, undefined, 'CacheManager')
      return null
    } catch (error) {
      logger.error('获取缓存失败', error, 'CacheManager')
      return null
    }
  }

  /**
     * 检查缓存是否有效
     */
  private isValid<T>(item: CacheItem<T>): boolean {
    return Date.now() - item.timestamp < item.ttl
  }

  /**
     * 删除缓存
     */
  remove(key: string, strategy: CacheStrategy = CacheStrategy.MEMORY_FIRST): void {
    try {
      if (strategy === CacheStrategy.MEMORY || strategy === CacheStrategy.MEMORY_FIRST) {
        this.memoryCache.delete(key)
      }

      if (strategy === CacheStrategy.STORAGE || strategy === CacheStrategy.MEMORY_FIRST) {
        wx.removeStorageSync(this.STORAGE_PREFIX + key)
      }

      logger.debug(`缓存已删除: ${key}`, undefined, 'CacheManager')
    } catch (error) {
      logger.error('删除缓存失败', error, 'CacheManager')
    }
  }

  /**
     * 清空所有缓存
     */
  clear(strategy: CacheStrategy = CacheStrategy.MEMORY_FIRST): void {
    try {
      if (strategy === CacheStrategy.MEMORY || strategy === CacheStrategy.MEMORY_FIRST) {
        this.memoryCache.clear()
      }

      if (strategy === CacheStrategy.STORAGE || strategy === CacheStrategy.MEMORY_FIRST) {
        // 清空所有带前缀的存储
        const info = wx.getStorageInfoSync()
        info.keys.forEach((key) => {
          if (key.startsWith(this.STORAGE_PREFIX)) {
            wx.removeStorageSync(key)
          }
        })
      }

      logger.info('缓存已清空', { strategy }, 'CacheManager')
    } catch (error) {
      logger.error('清空缓存失败', error, 'CacheManager')
    }
  }

  /**
     * 获取缓存年龄（毫秒）
     */
  getAge(key: string, strategy: CacheStrategy = CacheStrategy.MEMORY): number | null {
    try {
      // 优先从内存获取
      if (strategy === CacheStrategy.MEMORY || strategy === CacheStrategy.MEMORY_FIRST) {
        const item = this.memoryCache.get(key)
        if (item) {
          return Date.now() - item.timestamp
        }
      }

      // 从本地存储获取
      if (strategy === CacheStrategy.STORAGE || strategy === CacheStrategy.MEMORY_FIRST) {
        const item = wx.getStorageSync(this.STORAGE_PREFIX + key) as CacheItem<unknown>
        if (item) {
          return Date.now() - item.timestamp
        }
      }

      return null
    } catch (error) {
      logger.error('获取缓存年龄失败', error, 'CacheManager')
      return null
    }
  }

  /**
     * 获取缓存统计信息
     */
  getStats(): {
    memoryCount: number
    storageCount: number
    memoryKeys: string[]
  } {
    try {
      const info = wx.getStorageInfoSync()
      const storageKeys = info.keys.filter((key) => key.startsWith(this.STORAGE_PREFIX))

      return {
        memoryCount: this.memoryCache.size,
        storageCount: storageKeys.length,
        memoryKeys: Array.from(this.memoryCache.keys())
      }
    } catch (error) {
      logger.error('获取缓存统计失败', error, 'CacheManager')
      return {
        memoryCount: this.memoryCache.size,
        storageCount: 0,
        memoryKeys: Array.from(this.memoryCache.keys())
      }
    }
  }
}

// 导出单例
export const cache = new CacheManager()

// 预定义的缓存键
export const CacheKeys = {
  // 用户相关
  USER_INFO: 'user_info',
  USER_LOCATION: 'user_location',

  // 商户相关
  MERCHANT_LIST: 'merchant_list',
  MERCHANT_DETAIL: 'merchant_detail_',  // + merchantId

  // 菜品相关
  DISH_CATEGORIES: 'dish_categories',
  DISH_FEED: 'dish_feed_',  // + categoryId

  // 订单相关
  ORDER_LIST: 'order_list'
} as const
