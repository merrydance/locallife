"use strict";
/**
 * 网络状态监控器
 * 监听网络变化、提供离线提示、重试机制
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
exports.networkMonitor = void 0;
const logger_1 = require("./logger");
class NetworkMonitor {
    constructor() {
        this.networkState = {
            isConnected: true,
            networkType: 'unknown',
            isOfflineMode: false
        };
        this.listeners = new Set();
        this.offlineToastShown = false;
        this.init();
    }
    static getInstance() {
        if (!NetworkMonitor.instance) {
            NetworkMonitor.instance = new NetworkMonitor();
        }
        return NetworkMonitor.instance;
    }
    /**
       * 初始化网络监控
       */
    init() {
        // 获取初始网络状态
        this.checkNetworkStatus();
        // 监听网络状态变化
        wx.onNetworkStatusChange((res) => {
            const wasConnected = this.networkState.isConnected;
            this.networkState = {
                isConnected: res.isConnected,
                networkType: res.networkType,
                isOfflineMode: !res.isConnected
            };
            logger_1.logger.info('网络状态变化', this.networkState, 'NetworkMonitor');
            // 通知所有监听者
            this.notifyListeners();
            // 从离线恢复到在线
            if (!wasConnected && res.isConnected) {
                this.onNetworkRestore();
            }
            // 从在线变为离线
            if (wasConnected && !res.isConnected) {
                this.onNetworkLost();
            }
        });
        logger_1.logger.info('网络监控已启动', this.networkState, 'NetworkMonitor');
    }
    /**
       * 检查当前网络状态
       */
    checkNetworkStatus() {
        wx.getNetworkType({
            success: (res) => {
                const networkType = res.networkType;
                this.networkState = {
                    isConnected: networkType !== 'none',
                    networkType,
                    isOfflineMode: networkType === 'none'
                };
            },
            fail: () => {
                logger_1.logger.warn('获取网络状态失败', undefined, 'NetworkMonitor');
            }
        });
    }
    /**
       * 网络恢复处理
       */
    onNetworkRestore() {
        this.offlineToastShown = false;
        wx.showToast({
            title: '网络已恢复',
            icon: 'success',
            duration: 2000
        });
        logger_1.logger.info('网络已恢复', undefined, 'NetworkMonitor');
        // 可以在这里触发数据重新加载
        // eventBus.emit('network:restored')
    }
    /**
       * 网络断开处理
       */
    onNetworkLost() {
        if (!this.offlineToastShown) {
            wx.showToast({
                title: '网络已断开',
                icon: 'none',
                duration: 3000
            });
            this.offlineToastShown = true;
        }
        logger_1.logger.warn('网络已断开', undefined, 'NetworkMonitor');
        // eventBus.emit('network:lost')
    }
    /**
       * 订阅网络状态变化
       */
    subscribe(listener) {
        this.listeners.add(listener);
        // 立即通知当前状态
        listener(this.networkState);
        // 返回取消订阅函数
        return () => {
            this.listeners.delete(listener);
        };
    }
    /**
       * 通知所有监听者
       */
    notifyListeners() {
        this.listeners.forEach((listener) => {
            try {
                listener(this.networkState);
            }
            catch (error) {
                logger_1.logger.error('网络状态监听器执行失败', error, 'NetworkMonitor');
            }
        });
    }
    /**
       * 获取当前网络状态
       */
    getState() {
        return Object.assign({}, this.networkState);
    }
    /**
       * 是否在线
       */
    isOnline() {
        return this.networkState.isConnected;
    }
    /**
       * 是否是良好的网络(WiFi或4G/5G)
       */
    isGoodNetwork() {
        const { networkType, isConnected } = this.networkState;
        return isConnected && ['wifi', '4g', '5g'].includes(networkType);
    }
    /**
       * 显示离线提示
       */
    showOfflineHint(message = '当前网络不可用') {
        wx.showModal({
            title: '网络异常',
            content: message,
            showCancel: false,
            confirmText: '我知道了'
        });
    }
    /**
       * 检查网络并执行操作
       */
    checkAndExecute(fn_1) {
        return __awaiter(this, arguments, void 0, function* (fn, options = {}) {
            if (!this.isOnline()) {
                this.showOfflineHint(options.offlineMessage);
                throw new Error('Network offline');
            }
            if (options.requireGoodNetwork && !this.isGoodNetwork()) {
                const proceed = yield new Promise((resolve) => {
                    wx.showModal({
                        title: '网络较差',
                        content: '当前网络环境不佳,是否继续?',
                        success: (res) => resolve(res.confirm),
                        fail: () => resolve(false)
                    });
                });
                if (!proceed) {
                    throw new Error('User cancelled due to poor network');
                }
            }
            return fn();
        });
    }
}
exports.networkMonitor = NetworkMonitor.getInstance();
