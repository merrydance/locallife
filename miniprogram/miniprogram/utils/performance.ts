/**
 * 性能优化工具
 * 提供性能监控、优化和分析功能
 */

/**
 * 性能监控器
 */
export class PerformanceMonitor {
    private static marks: Map<string, number> = new Map()
    private static measures: Map<string, number> = new Map()

    /**
     * 标记性能时间点
     */
    static mark(name: string): void {
        this.marks.set(name, Date.now())
    }

    /**
     * 测量两个时间点之间的耗时
     */
    static measure(name: string, startMark: string, endMark?: string): number {
        const startTime = this.marks.get(startMark)
        const endTime = endMark ? this.marks.get(endMark) : Date.now()

        if (!startTime) {
            console.warn(`Performance mark "${startMark}" not found`)
            return 0
        }

        const duration = (endTime || Date.now()) - startTime
        this.measures.set(name, duration)

        return duration
    }

    /**
     * 获取测量结果
     */
    static getMeasure(name: string): number | undefined {
        return this.measures.get(name)
    }

    /**
     * 清除所有标记和测量
     */
    static clear(): void {
        this.marks.clear()
        this.measures.clear()
    }

    /**
     * 记录性能日志
     */
    static log(name: string, duration: number): void {
        if (duration > 1000) {
            console.warn(`⚠️ Performance: ${name} took ${duration}ms (slow)`)
        } else if (duration > 500) {
            console.log(`⏱️ Performance: ${name} took ${duration}ms`)
        } else {
            console.log(`✅ Performance: ${name} took ${duration}ms (fast)`)
        }
    }
}

/**
 * 防抖函数
 */
export function debounce<T extends (...args: any[]) => any>(
    func: T,
    wait: number
): (...args: Parameters<T>) => void {
    let timeout: number | null = null

    return function (this: any, ...args: Parameters<T>) {
        const context = this

        if (timeout !== null) {
            clearTimeout(timeout)
        }

        timeout = setTimeout(() => {
            func.apply(context, args)
            timeout = null
        }, wait) as any
    }
}

/**
 * 节流函数
 */
export function throttle<T extends (...args: any[]) => any>(
    func: T,
    wait: number
): (...args: Parameters<T>) => void {
    let timeout: number | null = null
    let previous = 0

    return function (this: any, ...args: Parameters<T>) {
        const now = Date.now()
        const context = this

        if (!previous) previous = now

        const remaining = wait - (now - previous)

        if (remaining <= 0 || remaining > wait) {
            if (timeout !== null) {
                clearTimeout(timeout)
                timeout = null
            }
            previous = now
            func.apply(context, args)
        } else if (!timeout) {
            timeout = setTimeout(() => {
                previous = Date.now()
                timeout = null
                func.apply(context, args)
            }, remaining) as any
        }
    }
}

/**
 * 图片懒加载
 */
export class ImageLazyLoader {
    private static observer: IntersectionObserver | null = null
    private static images: Set<string> = new Set()

    /**
     * 初始化懒加载
     */
    static init(): void {
        // 微信小程序不支持IntersectionObserver，使用替代方案
        console.log('Image lazy loading initialized')
    }

    /**
     * 添加图片到懒加载队列
     */
    static add(imageUrl: string): void {
        this.images.add(imageUrl)
    }

    /**
     * 预加载图片
     */
    static preload(imageUrl: string): Promise<void> {
        return new Promise((resolve, reject) => {
            wx.getImageInfo({
                src: imageUrl,
                success: () => resolve(),
                fail: reject
            })
        })
    }

    /**
     * 批量预加载图片
     */
    static async preloadBatch(imageUrls: string[]): Promise<void> {
        const promises = imageUrls.map(url => this.preload(url))
        await Promise.all(promises)
    }
}

/**
 * 数据缓存管理
 */
export class CacheManager {
    private static cache: Map<string, { data: any; timestamp: number; ttl: number }> = new Map()

    /**
     * 设置缓存
     */
    static set(key: string, data: any, ttl: number = 5 * 60 * 1000): void {
        this.cache.set(key, {
            data,
            timestamp: Date.now(),
            ttl
        })
    }

    /**
     * 获取缓存
     */
    static get<T = any>(key: string): T | null {
        const item = this.cache.get(key)

        if (!item) {
            return null
        }

        const now = Date.now()
        if (now - item.timestamp > item.ttl) {
            this.cache.delete(key)
            return null
        }

        return item.data as T
    }

    /**
     * 删除缓存
     */
    static delete(key: string): void {
        this.cache.delete(key)
    }

    /**
     * 清除所有缓存
     */
    static clear(): void {
        this.cache.clear()
    }

    /**
     * 清除过期缓存
     */
    static clearExpired(): void {
        const now = Date.now()
        for (const [key, item] of this.cache.entries()) {
            if (now - item.timestamp > item.ttl) {
                this.cache.delete(key)
            }
        }
    }
}

/**
 * 请求队列管理
 */
export class RequestQueue {
    private static queue: Array<() => Promise<any>> = []
    private static running = 0
    private static maxConcurrent = 5

    /**
     * 添加请求到队列
     */
    static add<T>(request: () => Promise<T>): Promise<T> {
        return new Promise((resolve, reject) => {
            const task = async () => {
                try {
                    this.running++
                    const result = await request()
                    resolve(result)
                } catch (error) {
                    reject(error)
                } finally {
                    this.running--
                    this.processQueue()
                }
            }

            this.queue.push(task)
            this.processQueue()
        })
    }

    /**
     * 处理队列
     */
    private static processQueue(): void {
        while (this.running < this.maxConcurrent && this.queue.length > 0) {
            const task = this.queue.shift()
            if (task) {
                task()
            }
        }
    }

    /**
     * 清空队列
     */
    static clear(): void {
        this.queue = []
    }
}

/**
 * 页面性能追踪装饰器
 */
export function trackPagePerformance(pageName: string) {
    return function (target: any) {
        const originalOnLoad = target.onLoad
        const originalOnReady = target.onReady

        target.onLoad = function (...args: any[]) {
            PerformanceMonitor.mark(`${pageName}-load-start`)
            if (originalOnLoad) {
                originalOnLoad.apply(this, args)
            }
        }

        target.onReady = function (...args: any[]) {
            PerformanceMonitor.mark(`${pageName}-ready`)
            const duration = PerformanceMonitor.measure(
                `${pageName}-load`,
                `${pageName}-load-start`,
                `${pageName}-ready`
            )
            PerformanceMonitor.log(`${pageName} load`, duration)

            if (originalOnReady) {
                originalOnReady.apply(this, args)
            }
        }

        return target
    }
}

/**
 * 内存优化工具
 */
export class MemoryOptimizer {
    /**
     * 清理未使用的数据
     */
    static cleanup(): void {
        // 清理过期缓存
        CacheManager.clearExpired()

        // 清理性能监控数据
        PerformanceMonitor.clear()

        console.log('Memory cleanup completed')
    }

    /**
     * 获取内存使用情况（微信小程序）
     */
    static getMemoryInfo(): Promise<any> {
        return new Promise((resolve) => {
            if (wx.getPerformance) {
                const performance = wx.getPerformance()
                const memory = performance.getEntries()
                resolve(memory)
            } else {
                resolve(null)
            }
        })
    }
}
