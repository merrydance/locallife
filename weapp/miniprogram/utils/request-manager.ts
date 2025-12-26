/**
 * 请求任务管理器
 * 管理所有进行中的请求,支持取消、防抖等功能
 */

import { logger } from './logger'

interface RequestTask {
    id: string
    task: WechatMiniprogram.RequestTask | WechatMiniprogram.UploadTask
    timestamp: number
    context?: string
}

class RequestManager {
  private static instance: RequestManager
  private tasks: Map<string, RequestTask> = new Map()
  private debounceTimers: Map<string, number> = new Map()

  private constructor() {}

  static getInstance(): RequestManager {
    if (!RequestManager.instance) {
      RequestManager.instance = new RequestManager()
    }
    return RequestManager.instance
  }

  /**
     * 注册一个请求任务
     */
  register(id: string, task: WechatMiniprogram.RequestTask | WechatMiniprogram.UploadTask, context?: string): void {
    // 如果已有同ID的请求,取消它
    if (this.tasks.has(id)) {
      this.cancel(id)
    }

    this.tasks.set(id, {
      id,
      task,
      timestamp: Date.now(),
      context
    })

    logger.debug(`请求已注册: ${id}`, { context }, 'RequestManager')
  }

  /**
     * 请求完成后注销
     */
  unregister(id: string): void {
    if (this.tasks.has(id)) {
      this.tasks.delete(id)
      logger.debug(`请求已注销: ${id}`, undefined, 'RequestManager')
    }
  }

  /**
     * 取消指定请求
     */
  cancel(id: string): void {
    const task = this.tasks.get(id)
    if (task) {
      try {
        task.task.abort()
        logger.info(`请求已取消: ${id}`, { context: task.context }, 'RequestManager')
      } catch (error) {
        logger.warn(`取消请求失败: ${id}`, error, 'RequestManager')
      } finally {
        this.tasks.delete(id)
      }
    }
  }

  /**
     * 取消指定上下文的所有请求
     */
  cancelByContext(context: string): void {
    const tasksToCancel = Array.from(this.tasks.values()).filter((t) => t.context === context)
        
    if (tasksToCancel.length > 0) {
      logger.info(`取消上下文的所有请求: ${context}`, { count: tasksToCancel.length }, 'RequestManager')
      tasksToCancel.forEach((task) => this.cancel(task.id))
    }
  }

  /**
     * 取消所有进行中的请求
     */
  cancelAll(): void {
    const count = this.tasks.size
    if (count > 0) {
      logger.info(`取消所有请求`, { count }, 'RequestManager')
      Array.from(this.tasks.keys()).forEach((id) => this.cancel(id))
    }
  }

  /**
     * 防抖执行函数
     * @param key 防抖键
     * @param fn 要执行的函数
     * @param delay 延迟时间(毫秒)
     */
  debounce<T extends (...args: never[]) => unknown>(
    key: string,
    fn: T,
    delay: number = 300
  ): (...args: Parameters<T>) => void {
    return (...args: Parameters<T>) => {
      // 清除之前的定时器
      const existingTimer = this.debounceTimers.get(key)
      if (existingTimer) {
        clearTimeout(existingTimer)
      }

      // 设置新的定时器
      const timer = setTimeout(() => {
        fn(...args)
        this.debounceTimers.delete(key)
      }, delay) as unknown as number

      this.debounceTimers.set(key, timer)
    }
  }

  /**
     * 节流执行函数
     * @param key 节流键
     * @param fn 要执行的函数
     * @param delay 节流时间(毫秒)
     */
  throttle<T extends (...args: never[]) => unknown>(
    key: string,
    fn: T,
    delay: number = 300
  ): (...args: Parameters<T>) => void {
    let lastExecTime = 0

    return (...args: Parameters<T>) => {
      const now = Date.now()
            
      if (now - lastExecTime >= delay) {
        fn(...args)
        lastExecTime = now
      }
    }
  }

  /**
     * 获取所有进行中的请求数量
     */
  getPendingCount(): number {
    return this.tasks.size
  }

  /**
     * 清理超时的请求(超过30秒)
     */
  cleanupStaleRequests(): void {
    const now = Date.now()
    const staleTimeout = 30000 // 30秒

    Array.from(this.tasks.entries()).forEach(([id, task]) => {
      if (now - task.timestamp > staleTimeout) {
        logger.warn(`清理超时请求: ${id}`, { age: now - task.timestamp }, 'RequestManager')
        this.cancel(id)
      }
    })
  }
}

export const requestManager = RequestManager.getInstance()

// 定期清理超时请求
setInterval(() => {
  requestManager.cleanupStaleRequests()
}, 60000) // 每分钟检查一次
