"use strict";
/**
 * 性能监控工具
 * 仅在开发环境下启用
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.performanceMonitor = void 0;
const logger_1 = require("./logger");
const cache_1 = require("./cache");
const image_lazy_load_1 = require("./image-lazy-load");
class PerformanceMonitor {
    constructor() {
        this.enabled = false;
        this.requestCount = 0;
        this.cacheHitCount = 0;
        this.updateInterval = null;
        this.UPDATE_FREQUENCY = 2000; // 2秒更新一次
        this.init();
    }
    /**
     * 初始化监控
     */
    init() {
        try {
            const accountInfo = wx.getAccountInfoSync();
            // 仅在开发环境启用
            this.enabled = accountInfo.miniProgram.envVersion === 'develop';
            if (this.enabled) {
                logger_1.logger.info('性能监控已启用', undefined, 'PerformanceMonitor');
                this.startMonitoring();
            }
        }
        catch (e) {
            logger_1.logger.warn('无法初始化性能监控', e, 'PerformanceMonitor');
        }
    }
    /**
     * 开始监控
     */
    startMonitoring() {
        if (!this.enabled)
            return;
        // 定期更新指标
        this.updateInterval = setInterval(() => {
            this.collectMetrics();
        }, this.UPDATE_FREQUENCY);
    }
    /**
     * 停止监控
     */
    stop() {
        if (this.updateInterval) {
            clearInterval(this.updateInterval);
            this.updateInterval = null;
        }
    }
    /**
     * 收集性能指标
     */
    collectMetrics() {
        const metrics = {
            memoryUsed: 0,
            memoryLimit: 0,
            memoryUsage: '0%',
            networkRequests: this.requestCount,
            cacheHits: this.cacheHitCount,
            cacheHitRate: this.calculateCacheHitRate(),
            imageLoaded: 0,
            imageTotal: 0,
            imageHitRate: '0%',
            currentPage: '',
            pageCount: 0,
            timestamp: new Date().toLocaleTimeString()
        };
        // 内存信息
        try {
            const performance = wx.getPerformance();
            if (performance && performance.memory) {
                metrics.memoryUsed = Math.round(performance.memory.jsHeapSizeUsed / 1024 / 1024);
                metrics.memoryLimit = Math.round(performance.memory.jsHeapSizeLimit / 1024 / 1024);
                metrics.memoryUsage = ((performance.memory.jsHeapSizeUsed / performance.memory.jsHeapSizeLimit) * 100).toFixed(1) + '%';
            }
        }
        catch (e) {
            // 某些环境可能不支持
        }
        // 缓存统计
        try {
            const cacheStats = cache_1.cache.getStats();
            metrics.cacheHits = cacheStats.memoryCount;
        }
        catch (e) {
            // ignore
        }
        // 图片统计
        try {
            const imageStats = image_lazy_load_1.imageLazyLoader.getStats();
            metrics.imageLoaded = imageStats.loaded;
            metrics.imageTotal = imageStats.total;
            metrics.imageHitRate = imageStats.hitRate;
        }
        catch (e) {
            // ignore
        }
        // 页面信息
        try {
            const pages = getCurrentPages();
            metrics.pageCount = pages.length;
            if (pages.length > 0) {
                const currentPage = pages[pages.length - 1];
                metrics.currentPage = currentPage.route || 'unknown';
            }
        }
        catch (e) {
            // ignore
        }
        return metrics;
    }
    /**
     * 计算缓存命中率
     */
    calculateCacheHitRate() {
        if (this.requestCount === 0)
            return '0%';
        return ((this.cacheHitCount / this.requestCount) * 100).toFixed(1) + '%';
    }
    /**
     * 记录网络请求
     */
    recordRequest(fromCache = false) {
        if (!this.enabled)
            return;
        this.requestCount++;
        if (fromCache) {
            this.cacheHitCount++;
        }
    }
    /**
     * 获取当前指标
     */
    getMetrics() {
        return this.collectMetrics();
    }
    /**
     * 获取详细报告
     */
    getReport() {
        const metrics = this.collectMetrics();
        return `
═══════════════════════════════════
📊 性能监控报告
═══════════════════════════════════

⏰ 时间: ${metrics.timestamp}
📄 当前页面: ${metrics.currentPage}
📚 页面栈深度: ${metrics.pageCount}

💾 内存使用:
   已用: ${metrics.memoryUsed} MB
   限制: ${metrics.memoryLimit} MB
   占比: ${metrics.memoryUsage}

🌐 网络请求:
   总数: ${metrics.networkRequests}
   缓存命中: ${metrics.cacheHits}
   命中率: ${metrics.cacheHitRate}

🖼️ 图片加载:
   已加载: ${metrics.imageLoaded}
   总数: ${metrics.imageTotal}
   成功率: ${metrics.imageHitRate}

═══════════════════════════════════
    `.trim();
    }
    /**
     * 打印报告到控制台
     */
    printReport() {
        if (!this.enabled)
            return;
        console.log(this.getReport());
    }
    /**
     * 检查是否启用
     */
    isEnabled() {
        return this.enabled;
    }
    /**
     * 显示悬浮窗（仅开发环境）
     */
    showFloatingWindow() {
        if (!this.enabled)
            return;
        // 在页面上创建悬浮窗
        const pages = getCurrentPages();
        if (pages.length === 0)
            return;
        const currentPage = pages[pages.length - 1];
        const metrics = this.collectMetrics();
        currentPage.setData({
            __performanceMetrics__: metrics,
            __showPerformanceMonitor__: true
        });
    }
    /**
     * 隐藏悬浮窗
     */
    hideFloatingWindow() {
        const pages = getCurrentPages();
        if (pages.length === 0)
            return;
        const currentPage = pages[pages.length - 1];
        currentPage.setData({
            __showPerformanceMonitor__: false
        });
    }
}
// 导出单例
exports.performanceMonitor = new PerformanceMonitor();
// 全局暴露（方便在控制台调试）
if (typeof global !== 'undefined') {
    global.perfMonitor = exports.performanceMonitor;
}
