"use strict";
/**
 * 包间相关API接口
 * 基于swagger.json中的包间浏览接口
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
exports.getRoom = exports.getRooms = void 0;
exports.getMerchantAvailableRooms = getMerchantAvailableRooms;
exports.getMerchantAllRooms = getMerchantAllRooms;
exports.getRoomDetail = getRoomDetail;
exports.checkRoomAvailability = checkRoomAvailability;
exports.getRoomsByCapacity = getRoomsByCapacity;
exports.getRoomsByPrice = getRoomsByPrice;
exports.getRoomsByType = getRoomsByType;
exports.getVIPRooms = getVIPRooms;
exports.getLuxuryRooms = getLuxuryRooms;
exports.checkMultipleRoomsAvailability = checkMultipleRoomsAvailability;
exports.getAvailableRoomsForTimeSlot = getAvailableRoomsForTimeSlot;
exports.calculateRoomCost = calculateRoomCost;
const request_1 = require("../utils/request");
// ==================== API接口函数 ====================
/**
 * 获取商户可用包间列表
 * @param merchantId 商户ID
 */
function getMerchantAvailableRooms(merchantId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/merchants/${merchantId}/rooms`,
            method: 'GET'
        });
    });
}
/**
 * 获取商户全部包间列表
 * @param merchantId 商户ID
 */
function getMerchantAllRooms(merchantId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/merchants/${merchantId}/rooms/all`,
            method: 'GET'
        });
    });
}
/**
 * 获取包间详情
 * @param roomId 包间ID
 */
function getRoomDetail(roomId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/rooms/${roomId}`,
            method: 'GET'
        });
    });
}
/**
 * 检查包间可用性
 * @param roomId 包间ID
 * @param params 检查参数
 */
function checkRoomAvailability(roomId, params) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/rooms/${roomId}/availability`,
            method: 'GET',
            data: params
        });
    });
}
// ==================== 便捷方法 ====================
/**
 * 根据容量筛选包间
 * @param merchantId 商户ID
 * @param minCapacity 最小容量
 * @param maxCapacity 最大容量
 */
function getRoomsByCapacity(merchantId, minCapacity, maxCapacity) {
    return __awaiter(this, void 0, void 0, function* () {
        const response = yield getMerchantAvailableRooms(merchantId);
        return response.rooms.filter(room => {
            if (maxCapacity) {
                return room.capacity >= minCapacity && room.capacity <= maxCapacity;
            }
            return room.capacity >= minCapacity;
        });
    });
}
/**
 * 根据价格筛选包间
 * @param merchantId 商户ID
 * @param maxHourlyRate 最大时租
 * @param maxMinimumSpend 最大最低消费
 */
function getRoomsByPrice(merchantId, maxHourlyRate, maxMinimumSpend) {
    return __awaiter(this, void 0, void 0, function* () {
        const response = yield getMerchantAvailableRooms(merchantId);
        return response.rooms.filter(room => {
            let match = true;
            if (maxHourlyRate && room.hourly_rate > maxHourlyRate) {
                match = false;
            }
            if (maxMinimumSpend && room.minimum_spend > maxMinimumSpend) {
                match = false;
            }
            return match;
        });
    });
}
/**
 * 根据包间类型筛选
 * @param merchantId 商户ID
 * @param roomType 包间类型
 */
function getRoomsByType(merchantId, roomType) {
    return __awaiter(this, void 0, void 0, function* () {
        const response = yield getMerchantAvailableRooms(merchantId);
        return response.rooms.filter(room => room.room_type === roomType);
    });
}
/**
 * 获取VIP包间
 * @param merchantId 商户ID
 */
function getVIPRooms(merchantId) {
    return __awaiter(this, void 0, void 0, function* () {
        return getRoomsByType(merchantId, 'vip');
    });
}
/**
 * 获取豪华包间
 * @param merchantId 商户ID
 */
function getLuxuryRooms(merchantId) {
    return __awaiter(this, void 0, void 0, function* () {
        return getRoomsByType(merchantId, 'luxury');
    });
}
/**
 * 检查多个包间的可用性
 * @param roomIds 包间ID列表
 * @param params 检查参数
 */
function checkMultipleRoomsAvailability(roomIds, params) {
    return __awaiter(this, void 0, void 0, function* () {
        const results = yield Promise.all(roomIds.map(roomId => checkRoomAvailability(roomId, params)));
        return results;
    });
}
/**
 * 获取指定时间段的可用包间
 * @param merchantId 商户ID
 * @param date 日期
 * @param startTime 开始时间
 * @param endTime 结束时间
 */
function getAvailableRoomsForTimeSlot(merchantId, date, startTime, endTime) {
    return __awaiter(this, void 0, void 0, function* () {
        const response = yield getMerchantAvailableRooms(merchantId);
        const availabilityChecks = yield Promise.all(response.rooms.map(room => checkRoomAvailability(room.id, { date, start_time: startTime, end_time: endTime })));
        return response.rooms.filter((_room, index) => {
            var _a, _b;
            const check = availabilityChecks[index];
            // 检查所有时间段是否都可用
            return (_b = (_a = check.time_slots) === null || _a === void 0 ? void 0 : _a.every(slot => slot.available)) !== null && _b !== void 0 ? _b : false;
        });
    });
}
/**
 * 计算包间费用
 * @param room 包间信息
 * @param hours 使用小时数
 */
function calculateRoomCost(room, hours) {
    const hourlyFee = room.hourly_rate * hours;
    const minimumSpend = room.minimum_spend;
    const totalCost = Math.max(hourlyFee, minimumSpend);
    return {
        hourlyFee,
        minimumSpend,
        totalCost
    };
}
// ==================== 兼容性别名 ====================
/** @deprecated 使用 getMerchantAvailableRooms 替代 */
exports.getRooms = getMerchantAvailableRooms;
/** @deprecated 使用 getRoomDetail 替代 */
exports.getRoom = getRoomDetail;
