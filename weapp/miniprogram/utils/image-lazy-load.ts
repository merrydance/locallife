/**
 * 图片懒加载增强工具
 * 提供预加载、占位图、失败重试等功能
 */

import { logger } from './logger'

// 占位图 (Data URL - 1x1 透明 GIF)
export const PLACEHOLDER_IMAGE = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7'

// 图片加载状态
export enum ImageLoadState {
  IDLE = 'idle',
  LOADING = 'loading',
  LOADED = 'loaded',
  ERROR = 'error'
}

interface ImageCacheItem {
  state: ImageLoadState
  timestamp: number
  retryCount: number
}

class ImageLazyLoader {
  private cache: Map<string, ImageCacheItem> = new Map()
  private readonly MAX_RETRY = 3
  private readonly CACHE_TTL = 30 * 60 * 1000 // 30分钟

  /**
   * 预加载图片
   * @param urls 图片 URL 数组
   * @param priority 是否高优先级（立即加载）
   */
  preload(urls: string[], priority: boolean = false): void {
    if (!Array.isArray(urls) || urls.length === 0) return

    urls.forEach((url, index) => {
      if (!url || this.isLoaded(url)) return

      const delay = priority ? 0 : index * 100 // 高优先级立即加载，否则错开加载

      setTimeout(() => {
        this.loadImage(url)
      }, delay)
    })
  }

  /**
   * 加载单张图片
   */
  private loadImage(url: string): Promise<void> {
    return new Promise((resolve, reject) => {
      const cacheItem = this.cache.get(url)

      // 如果已在加载中或已加载，跳过
      if (cacheItem?.state === ImageLoadState.LOADING || cacheItem?.state === ImageLoadState.LOADED) {
        resolve()
        return
      }

      // 超过重试次数，跳过
      if (cacheItem && cacheItem.retryCount >= this.MAX_RETRY) {
        reject(new Error('Max retry exceeded'))
        return
      }

      // 更新状态为加载中
      this.cache.set(url, {
        state: ImageLoadState.LOADING,
        timestamp: Date.now(),
        retryCount: (cacheItem?.retryCount || 0)
      })

      // 使用 wx.downloadFile 预加载图片
      wx.downloadFile({
        url,
        success: (res) => {
          if (res.statusCode === 200) {
            this.cache.set(url, {
              state: ImageLoadState.LOADED,
              timestamp: Date.now(),
              retryCount: 0
            })
            logger.debug(`图片预加载成功: ${url}`, undefined, 'ImageLazyLoader')
            resolve()
          } else {
            this.handleLoadError(url)
            reject(new Error(`HTTP ${res.statusCode}`))
          }
        },
        fail: (err) => {
          this.handleLoadError(url)
          logger.warn(`图片预加载失败: ${url}`, err, 'ImageLazyLoader')
          reject(err)
        }
      })
    })
  }

  /**
   * 处理加载失败
   */
  private handleLoadError(url: string): void {
    const cacheItem = this.cache.get(url)
    if (cacheItem) {
      this.cache.set(url, {
        state: ImageLoadState.ERROR,
        timestamp: Date.now(),
        retryCount: cacheItem.retryCount + 1
      })
    }
  }

  /**
   * 检查图片是否已加载
   */
  isLoaded(url: string): boolean {
    const cacheItem = this.cache.get(url)
    if (!cacheItem) return false

    // 检查是否过期
    if (Date.now() - cacheItem.timestamp > this.CACHE_TTL) {
      this.cache.delete(url)
      return false
    }

    return cacheItem.state === ImageLoadState.LOADED
  }

  /**
   * 获取图片状态
   */
  getState(url: string): ImageLoadState {
    return this.cache.get(url)?.state || ImageLoadState.IDLE
  }

  /**
   * 清理过期缓存
   */
  cleanup(): void {
    const now = Date.now()
    let cleanedCount = 0

    this.cache.forEach((item, url) => {
      if (now - item.timestamp > this.CACHE_TTL) {
        this.cache.delete(url)
        cleanedCount++
      }
    })

    if (cleanedCount > 0) {
      logger.debug(`清理了 ${cleanedCount} 个过期图片缓存`, undefined, 'ImageLazyLoader')
    }
  }

  /**
   * 获取缓存统计
   */
  getStats() {
    let loaded = 0
    let loading = 0
    let error = 0

    this.cache.forEach((item) => {
      switch (item.state) {
        case ImageLoadState.LOADED:
          loaded++
          break
        case ImageLoadState.LOADING:
          loading++
          break
        case ImageLoadState.ERROR:
          error++
          break
      }
    })

    return {
      total: this.cache.size,
      loaded,
      loading,
      error,
      hitRate: this.cache.size > 0 ? (loaded / this.cache.size * 100).toFixed(1) + '%' : '0%'
    }
  }

  /**
   * 清空缓存
   */
  clear(): void {
    this.cache.clear()
    logger.debug('图片缓存已清空', undefined, 'ImageLazyLoader')
  }
}

// 导出单例
export const imageLazyLoader = new ImageLazyLoader()

// 定期清理（每10分钟）
setInterval(() => {
  imageLazyLoader.cleanup()
}, 10 * 60 * 1000)
