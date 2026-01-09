"use strict";
/**
 * 事件总线 - 用于跨组件/页面通信
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.Events = exports.eventBus = void 0;
class EventBus {
    constructor() {
        this.events = new Map();
    }
    /**
       * 订阅事件
       */
    on(event, handler) {
        if (!this.events.has(event)) {
            this.events.set(event, []);
        }
        this.events.get(event).push(handler);
    }
    /**
       * 订阅一次性事件
       */
    once(event, handler) {
        const onceHandler = (data) => {
            handler(data);
            this.off(event, onceHandler);
        };
        this.on(event, onceHandler);
    }
    /**
       * 发布事件
       */
    emit(event, data) {
        const handlers = this.events.get(event);
        if (handlers) {
            handlers.forEach((handler) => {
                try {
                    handler(data);
                }
                catch (error) {
                    console.error(`事件处理器执行失败: ${event}`, error);
                }
            });
        }
    }
    /**
       * 取消订阅
       */
    off(event, handler) {
        if (!handler) {
            // 取消所有订阅
            this.events.delete(event);
            return;
        }
        const handlers = this.events.get(event);
        if (handlers) {
            const index = handlers.indexOf(handler);
            if (index > -1) {
                handlers.splice(index, 1);
            }
            // 如果没有处理器了,删除事件
            if (handlers.length === 0) {
                this.events.delete(event);
            }
        }
    }
    /**
       * 清空所有事件
       */
    clear() {
        this.events.clear();
    }
    /**
       * 获取事件列表
       */
    getEvents() {
        return Array.from(this.events.keys());
    }
    /**
       * 获取事件的订阅数量
       */
    getListenerCount(event) {
        var _a;
        return ((_a = this.events.get(event)) === null || _a === void 0 ? void 0 : _a.length) || 0;
    }
}
// 导出单例
exports.eventBus = new EventBus();
// 预定义的事件名称
exports.Events = {
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
};
