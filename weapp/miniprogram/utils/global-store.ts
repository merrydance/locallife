/**
 * 全局状态管理器
 * 使用观察者模式管理全局状态,避免频繁操作globalData
 */

import { logger } from './logger'

type Listener<T> = (newValue: T, oldValue: T) => void

interface StoreState {
  location: {
    name: string
    address?: string
  }
  latitude: number | null
  longitude: number | null
  navBarHeight: number
  cart: {
    items: any[]
    totalCount: number
    totalPrice: number
    totalPriceDisplay: string
  }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type ListenerMap = Map<keyof StoreState, Set<Listener<any>>>

class GlobalStore {
  private static instance: GlobalStore
  private state: StoreState
  private listeners: ListenerMap = new Map()

  private constructor() {
    // 从app.globalData初始化状态
    const app = getApp<IAppOption>()
    const loc = app?.globalData?.location || { name: '' }
    this.state = {
      location: { name: loc.name || '', address: loc.address },
      latitude: app?.globalData?.latitude || null,
      longitude: app?.globalData?.longitude || null,
      navBarHeight: 88, // 默认值
      cart: {
        items: [],
        totalCount: 0,
        totalPrice: 0,
        totalPriceDisplay: '¥0.00'
      }
    }

    // 计算真实navBarHeight
    this.calculateNavBarHeight()
  }

  static getInstance(): GlobalStore {
    if (!GlobalStore.instance) {
      GlobalStore.instance = new GlobalStore()
    }
    return GlobalStore.instance
  }

  /**
     * 获取状态值
     */
  get<K extends keyof StoreState>(key: K): StoreState[K] {
    return this.state[key]
  }

  /**
     * 设置状态值并通知监听者
     */
  set<K extends keyof StoreState>(key: K, value: StoreState[K], silent: boolean = false): void {
    const oldValue = this.state[key]

    // 浅比较,如果值没变则不触发更新
    if (JSON.stringify(oldValue) === JSON.stringify(value)) {
      return
    }

    this.state[key] = value

    // 同步到app.globalData
    this.syncToGlobalData(key, value)

    if (!silent) {
      logger.debug(`GlobalStore更新: ${key}`, { oldValue, newValue: value }, 'GlobalStore')
      this.notify(key, value, oldValue)
    }
  }

  /**
     * 批量设置状态
     */
  setBatch(updates: Partial<StoreState>, silent: boolean = false): void {
    const keys = Object.keys(updates) as Array<keyof StoreState>
    keys.forEach(<K extends keyof StoreState>(key: K) => {
      const value = updates[key]
      if (value !== undefined) {
        this.set(key, value as StoreState[K], silent)
      }
    })
  }

  /**
     * 订阅状态变化
     */
  subscribe<K extends keyof StoreState>(key: K, listener: Listener<StoreState[K]>): () => void {
    if (!this.listeners.has(key)) {
      this.listeners.set(key, new Set())
    }
    this.listeners.get(key)!.add(listener)

    // 返回取消订阅函数
    return () => {
      this.listeners.get(key)?.delete(listener)
    }
  }

  /**
     * 通知监听者
     */
  private notify<K extends keyof StoreState>(key: K, newValue: StoreState[K], oldValue: StoreState[K]): void {
    const listeners = this.listeners.get(key)
    logger.debug(`[GlobalStore] notify 被调用: ${key}`, {
      listenersCount: listeners?.size || 0,
      newValue,
      oldValue
    }, 'GlobalStore.notify')

    if (listeners) {
      listeners.forEach((listener) => {
        try {
          listener(newValue, oldValue)
        } catch (error) {
          logger.error(`GlobalStore监听器执行失败: ${key}`, error, 'GlobalStore')
        }
      })
    }
  }

  /**
     * 同步到app.globalData
     */
  private syncToGlobalData<K extends keyof StoreState>(key: K, value: StoreState[K]): void {
    try {
      const app = getApp<IAppOption>()
      if (app?.globalData) {
        switch (key) {
          case 'location':
            app.globalData.location = value as typeof app.globalData.location
            break
          case 'latitude':
            app.globalData.latitude = value as typeof app.globalData.latitude
            break
          case 'longitude':
            app.globalData.longitude = value as typeof app.globalData.longitude
            break
        }
      }
    } catch (error) {
      logger.error('同步到globalData失败', error, 'GlobalStore')
    }
  }

  /**
   * 计算并缓存导航栏高度
   */
  private calculateNavBarHeight(): void {
    try {
      const { getStableBarHeights } = require('./responsive')
      const { navBarHeight } = getStableBarHeights()

      this.state.navBarHeight = navBarHeight
      logger.debug('导航栏高度已缓存(稳定版)', { navBarHeight }, 'GlobalStore')
    } catch (error) {
      logger.error('计算导航栏高度失败', error, 'GlobalStore')
      this.state.navBarHeight = 88 // 使用默认值
    }
  }

  /**
     * 更新位置信息
     */
  updateLocation(latitude: number, longitude: number, name: string, address?: string): void {
    logger.info('[GlobalStore] updateLocation 被调用', {
      latitude,
      longitude,
      name,
      address
    }, 'GlobalStore.updateLocation')

    this.setBatch({
      latitude,
      longitude,
      location: { name, address }
    })

    logger.info('[GlobalStore] updateLocation 完成，当前状态', this.getState(), 'GlobalStore.updateLocation')
  }

  /**
     * 获取完整状态(用于调试)
     */
  getState(): Readonly<StoreState> {
    return { ...this.state }
  }
}

export const globalStore = GlobalStore.getInstance()
