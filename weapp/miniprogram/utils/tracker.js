"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.tracker = exports.BehaviorTracker = exports.Role = exports.EventType = void 0;
const logger_1 = require("./logger");
var EventType;
(function (EventType) {
    EventType["APP_OPEN"] = "APP_OPEN";
    EventType["VIEW_DISH"] = "VIEW_DISH";
    EventType["ADD_CART"] = "ADD_CART";
    EventType["SUBMIT_ORDER"] = "SUBMIT_ORDER";
    EventType["PAY_ORDER"] = "PAY_ORDER";
})(EventType || (exports.EventType = EventType = {}));
var Role;
(function (Role) {
    Role["CUSTOMER"] = "customer";
    Role["MERCHANT"] = "merchant";
    Role["RIDER"] = "rider";
})(Role || (exports.Role = Role = {}));
class BehaviorTracker {
    constructor() { }
    static getInstance() {
        if (!BehaviorTracker.instance) {
            BehaviorTracker.instance = new BehaviorTracker();
        }
        return BehaviorTracker.instance;
    }
    /**
       * Log a user behavior event
       * @param eventType The type of event
       * @param targetID The ID of the target object (dish_id, order_id, etc.)
       * @param metaData Additional metadata
       */
    log(eventType, targetID = '', metaData = {}) {
        // Get user role from global data or storage
        // For MVP, we assume customer role mostly
        const role = Role.CUSTOMER;
        logger_1.logger.debug('用户行为追踪', { eventType, targetID, metaData, role }, 'BehaviorTracker');
        // Fire and forget - don't await
        // Backend endpoint /rank/behavior not available yet
        /*
            request({
                url: '/rank/behavior',
                method: 'POST',
                loading: false,
                data: {
                    role: role,
                    event_type: eventType,
                    target_id: targetID,
                    meta_data: metaData
                }
            }).catch(err => {
                // Silently fail for tracker events
                logger.warn('行为追踪上报失败', err, 'BehaviorTracker')
            })
            */
    }
}
exports.BehaviorTracker = BehaviorTracker;
exports.tracker = BehaviorTracker.getInstance();
