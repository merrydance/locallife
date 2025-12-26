/**
 * 事件总线 - 用于跨组件/页面通信
 */

type EventHandler = (data?: unknown) => void

class EventBus {
  private events: Map<string, EventHandler[]> = new Map()

  /**
     * 订阅事件
     */
  on(event: string, handler: EventHandler): void {
    if (!this.events.has(event)) {
      this.events.set(event, [])
    }
    this.events.get(event)!.push(handler)
  }

  /**
     * 订阅一次性事件
     */
  once(event: string, handler: EventHandler): void {
    const onceHandler: EventHandler = (data) => {
      handler(data)
      this.off(event, onceHandler)
    }
    this.on(event, onceHandler)
  }

  /**
     * 发布事件
     */
  emit(event: string, data?: unknown): void {
    const handlers = this.events.get(event)
    if (handlers) {
      handlers.forEach((handler) => {
        try {
          handler(data)
        } catch (error) {
          console.error(`事件处理器执行失败: ${event}`, error)
        }
      })
    }
  }

  /**
     * 取消订阅
     */
  off(event: string, handler?: EventHandler): void {
    if (!handler) {
      // 取消所有订阅
      this.events.delete(event)
      return
    }

    const handlers = this.events.get(event)
    if (handlers) {
      const index = handlers.indexOf(handler)
      if (index > -1) {
        handlers.splice(index, 1)
      }

      // 如果没有处理器了,删除事件
      if (handlers.length === 0) {
        this.events.delete(event)
      }
    }
  }

  /**
     * 清空所有事件
     */
  clear(): void {
    this.events.clear()
  }

  /**
     * 获取事件列表
     */
  getEvents(): string[] {
    return Array.from(this.events.keys())
  }

  /**
     * 获取事件的订阅数量
     */
  getListenerCount(event: string): number {
    return this.events.get(event)?.length || 0
  }
}

// 导出单例
export const eventBus = new EventBus()

// 预定义的事件名称
export const Events = {
  // 购物车相关
  CART_UPDATED: 'cart:updated',
  CART_CLEARED: 'cart:cleared',

  // 用户相关
  USER_LOGIN: 'user:login',
  USER_LOGOUT: 'user:logout',
  USER_INFO_UPDATED: 'user:info_updated',

  // 位置相关
  LOCATION_UPDATED: 'location:updated',

  // 订单相关
  ORDER_CREATED: 'order:created',
  ORDER_UPDATED: 'order:updated',
  ORDER_CANCELLED: 'order:cancelled',

  // 页面相关
  PAGE_REFRESH: 'page:refresh'
} as const
