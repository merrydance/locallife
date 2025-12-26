"use strict";
/**
 * 性能优化工具
 * 提供性能监控、优化和分析功能
 */
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.MemoryOptimizer = exports.RequestQueue = exports.CacheManager = exports.ImageLazyLoader = exports.PerformanceMonitor = void 0;
exports.debounce = debounce;
exports.throttle = throttle;
exports.trackPagePerformance = trackPagePerformance;
/**
 * 性能监控器
 */
class PerformanceMonitor {
    /**
     * 标记性能时间点
     */
    static mark(name) {
        this.marks.set(name, Date.now());
    }
    /**
     * 测量两个时间点之间的耗时
     */
    static measure(name, startMark, endMark) {
        const startTime = this.marks.get(startMark);
        const endTime = endMark ? this.marks.get(endMark) : Date.now();
        if (!startTime) {
            console.warn(`Performance mark "${startMark}" not found`);
            return 0;
        }
        const duration = (endTime || Date.now()) - startTime;
        this.measures.set(name, duration);
        return duration;
    }
    /**
     * 获取测量结果
     */
    static getMeasure(name) {
        return this.measures.get(name);
    }
    /**
     * 清除所有标记和测量
     */
    static clear() {
        this.marks.clear();
        this.measures.clear();
    }
    /**
     * 记录性能日志
     */
    static log(name, duration) {
        if (duration > 1000) {
            console.warn(`⚠️ Performance: ${name} took ${duration}ms (slow)`);
        }
        else if (duration > 500) {
            console.log(`⏱️ Performance: ${name} took ${duration}ms`);
        }
        else {
            console.log(`✅ Performance: ${name} took ${duration}ms (fast)`);
        }
    }
}
exports.PerformanceMonitor = PerformanceMonitor;
PerformanceMonitor.marks = new Map();
PerformanceMonitor.measures = new Map();
/**
 * 防抖函数
 */
function debounce(func, wait) {
    let timeout = null;
    return function (...args) {
        const context = this;
        if (timeout !== null) {
            clearTimeout(timeout);
        }
        timeout = setTimeout(() => {
            func.apply(context, args);
            timeout = null;
        }, wait);
    };
}
/**
 * 节流函数
 */
function throttle(func, wait) {
    let timeout = null;
    let previous = 0;
    return function (...args) {
        const now = Date.now();
        const context = this;
        if (!previous)
            previous = now;
        const remaining = wait - (now - previous);
        if (remaining <= 0 || remaining > wait) {
            if (timeout !== null) {
                clearTimeout(timeout);
                timeout = null;
            }
            previous = now;
            func.apply(context, args);
        }
        else if (!timeout) {
            timeout = setTimeout(() => {
                previous = Date.now();
                timeout = null;
                func.apply(context, args);
            }, remaining);
        }
    };
}
/**
 * 图片懒加载
 */
class ImageLazyLoader {
    /**
     * 初始化懒加载
     */
    static init() {
        // 微信小程序不支持IntersectionObserver，使用替代方案
        console.log('Image lazy loading initialized');
    }
    /**
     * 添加图片到懒加载队列
     */
    static add(imageUrl) {
        this.images.add(imageUrl);
    }
    /**
     * 预加载图片
     */
    static preload(imageUrl) {
        return new Promise((resolve, reject) => {
            wx.getImageInfo({
                src: imageUrl,
                success: () => resolve(),
                fail: reject
            });
        });
    }
    /**
     * 批量预加载图片
     */
    static preloadBatch(imageUrls) {
        return __awaiter(this, void 0, void 0, function* () {
            const promises = imageUrls.map(url => this.preload(url));
            yield Promise.all(promises);
        });
    }
}
exports.ImageLazyLoader = ImageLazyLoader;
ImageLazyLoader.observer = null;
ImageLazyLoader.images = new Set();
/**
 * 数据缓存管理
 */
class CacheManager {
    /**
     * 设置缓存
     */
    static set(key, data, ttl = 5 * 60 * 1000) {
        this.cache.set(key, {
            data,
            timestamp: Date.now(),
            ttl
        });
    }
    /**
     * 获取缓存
     */
    static get(key) {
        const item = this.cache.get(key);
        if (!item) {
            return null;
        }
        const now = Date.now();
        if (now - item.timestamp > item.ttl) {
            this.cache.delete(key);
            return null;
        }
        return item.data;
    }
    /**
     * 删除缓存
     */
    static delete(key) {
        this.cache.delete(key);
    }
    /**
     * 清除所有缓存
     */
    static clear() {
        this.cache.clear();
    }
    /**
     * 清除过期缓存
     */
    static clearExpired() {
        const now = Date.now();
        for (const [key, item] of this.cache.entries()) {
            if (now - item.timestamp > item.ttl) {
                this.cache.delete(key);
            }
        }
    }
}
exports.CacheManager = CacheManager;
CacheManager.cache = new Map();
/**
 * 请求队列管理
 */
class RequestQueue {
    /**
     * 添加请求到队列
     */
    static add(request) {
        return new Promise((resolve, reject) => {
            const task = () => __awaiter(this, void 0, void 0, function* () {
                try {
                    this.running++;
                    const result = yield request();
                    resolve(result);
                }
                catch (error) {
                    reject(error);
                }
                finally {
                    this.running--;
                    this.processQueue();
                }
            });
            this.queue.push(task);
            this.processQueue();
        });
    }
    /**
     * 处理队列
     */
    static processQueue() {
        while (this.running < this.maxConcurrent && this.queue.length > 0) {
            const task = this.queue.shift();
            if (task) {
                task();
            }
        }
    }
    /**
     * 清空队列
     */
    static clear() {
        this.queue = [];
    }
}
exports.RequestQueue = RequestQueue;
RequestQueue.queue = [];
RequestQueue.running = 0;
RequestQueue.maxConcurrent = 5;
/**
 * 页面性能追踪装饰器
 */
function trackPagePerformance(pageName) {
    return function (target) {
        const originalOnLoad = target.onLoad;
        const originalOnReady = target.onReady;
        target.onLoad = function (...args) {
            PerformanceMonitor.mark(`${pageName}-load-start`);
            if (originalOnLoad) {
                originalOnLoad.apply(this, args);
            }
        };
        target.onReady = function (...args) {
            PerformanceMonitor.mark(`${pageName}-ready`);
            const duration = PerformanceMonitor.measure(`${pageName}-load`, `${pageName}-load-start`, `${pageName}-ready`);
            PerformanceMonitor.log(`${pageName} load`, duration);
            if (originalOnReady) {
                originalOnReady.apply(this, args);
            }
        };
        return target;
    };
}
/**
 * 内存优化工具
 */
class MemoryOptimizer {
    /**
     * 清理未使用的数据
     */
    static cleanup() {
        // 清理过期缓存
        CacheManager.clearExpired();
        // 清理性能监控数据
        PerformanceMonitor.clear();
        console.log('Memory cleanup completed');
    }
    /**
     * 获取内存使用情况（微信小程序）
     */
    static getMemoryInfo() {
        return new Promise((resolve) => {
            if (wx.getPerformance) {
                const performance = wx.getPerformance();
                const memory = performance.getEntries();
                resolve(memory);
            }
            else {
                resolve(null);
            }
        });
    }
}
exports.MemoryOptimizer = MemoryOptimizer;
