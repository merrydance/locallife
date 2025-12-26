"use strict";
/**
 * 全局状态管理器
 * 使用观察者模式管理全局状态,避免频繁操作globalData
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.globalStore = void 0;
const logger_1 = require("./logger");
class GlobalStore {
    constructor() {
        var _a, _b, _c;
        this.listeners = new Map();
        // 从app.globalData初始化状态
        const app = getApp();
        const loc = ((_a = app === null || app === void 0 ? void 0 : app.globalData) === null || _a === void 0 ? void 0 : _a.location) || { name: '' };
        this.state = {
            location: { name: loc.name || '', address: loc.address },
            latitude: ((_b = app === null || app === void 0 ? void 0 : app.globalData) === null || _b === void 0 ? void 0 : _b.latitude) || null,
            longitude: ((_c = app === null || app === void 0 ? void 0 : app.globalData) === null || _c === void 0 ? void 0 : _c.longitude) || null,
            navBarHeight: 88, // 默认值
            cart: {
                items: [],
                totalCount: 0,
                totalPrice: 0,
                totalPriceDisplay: '¥0.00'
            }
        };
        // 计算真实navBarHeight
        this.calculateNavBarHeight();
    }
    static getInstance() {
        if (!GlobalStore.instance) {
            GlobalStore.instance = new GlobalStore();
        }
        return GlobalStore.instance;
    }
    /**
       * 获取状态值
       */
    get(key) {
        return this.state[key];
    }
    /**
       * 设置状态值并通知监听者
       */
    set(key, value, silent = false) {
        const oldValue = this.state[key];
        // 浅比较,如果值没变则不触发更新
        if (JSON.stringify(oldValue) === JSON.stringify(value)) {
            return;
        }
        this.state[key] = value;
        // 同步到app.globalData
        this.syncToGlobalData(key, value);
        if (!silent) {
            logger_1.logger.debug(`GlobalStore更新: ${key}`, { oldValue, newValue: value }, 'GlobalStore');
            this.notify(key, value, oldValue);
        }
    }
    /**
       * 批量设置状态
       */
    setBatch(updates, silent = false) {
        const keys = Object.keys(updates);
        keys.forEach((key) => {
            const value = updates[key];
            if (value !== undefined) {
                this.set(key, value, silent);
            }
        });
    }
    /**
       * 订阅状态变化
       */
    subscribe(key, listener) {
        if (!this.listeners.has(key)) {
            this.listeners.set(key, new Set());
        }
        this.listeners.get(key).add(listener);
        // 返回取消订阅函数
        return () => {
            var _a;
            (_a = this.listeners.get(key)) === null || _a === void 0 ? void 0 : _a.delete(listener);
        };
    }
    /**
       * 通知监听者
       */
    notify(key, newValue, oldValue) {
        const listeners = this.listeners.get(key);
        logger_1.logger.debug(`[GlobalStore] notify 被调用: ${key}`, {
            listenersCount: (listeners === null || listeners === void 0 ? void 0 : listeners.size) || 0,
            newValue,
            oldValue
        }, 'GlobalStore.notify');
        if (listeners) {
            listeners.forEach((listener) => {
                try {
                    listener(newValue, oldValue);
                }
                catch (error) {
                    logger_1.logger.error(`GlobalStore监听器执行失败: ${key}`, error, 'GlobalStore');
                }
            });
        }
    }
    /**
       * 同步到app.globalData
       */
    syncToGlobalData(key, value) {
        try {
            const app = getApp();
            if (app === null || app === void 0 ? void 0 : app.globalData) {
                switch (key) {
                    case 'location':
                        app.globalData.location = value;
                        break;
                    case 'latitude':
                        app.globalData.latitude = value;
                        break;
                    case 'longitude':
                        app.globalData.longitude = value;
                        break;
                }
            }
        }
        catch (error) {
            logger_1.logger.error('同步到globalData失败', error, 'GlobalStore');
        }
    }
    /**
     * 计算并缓存导航栏高度
     */
    calculateNavBarHeight() {
        try {
            const { getStableBarHeights } = require('./responsive');
            const { navBarHeight } = getStableBarHeights();
            this.state.navBarHeight = navBarHeight;
            logger_1.logger.debug('导航栏高度已缓存(稳定版)', { navBarHeight }, 'GlobalStore');
        }
        catch (error) {
            logger_1.logger.error('计算导航栏高度失败', error, 'GlobalStore');
            this.state.navBarHeight = 88; // 使用默认值
        }
    }
    /**
       * 更新位置信息
       */
    updateLocation(latitude, longitude, name, address) {
        logger_1.logger.info('[GlobalStore] updateLocation 被调用', {
            latitude,
            longitude,
            name,
            address
        }, 'GlobalStore.updateLocation');
        this.setBatch({
            latitude,
            longitude,
            location: { name, address }
        });
        logger_1.logger.info('[GlobalStore] updateLocation 完成，当前状态', this.getState(), 'GlobalStore.updateLocation');
    }
    /**
       * 获取完整状态(用于调试)
       */
    getState() {
        return Object.assign({}, this.state);
    }
}
exports.globalStore = GlobalStore.getInstance();
