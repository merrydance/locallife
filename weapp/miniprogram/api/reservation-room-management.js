"use strict";
/**
 * 预定和包间管理接口重构 (Task 2.7)
 * 基于swagger.json完全重构，移除所有没有后端支持的旧功能
 * 包含：预定管理、包间管理、预定操作
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
exports.roomManagementService = exports.reservationManagementService = exports.ReservationRoomAdapter = exports.RoomManagementService = exports.ReservationManagementService = void 0;
exports.getTodayReservations = getTodayReservations;
exports.getPendingReservations = getPendingReservations;
exports.getReservationOverview = getReservationOverview;
exports.batchConfirmReservations = batchConfirmReservations;
exports.batchMarkNoShow = batchMarkNoShow;
exports.calculateReservationMetrics = calculateReservationMetrics;
exports.getReservationStatusText = getReservationStatusText;
exports.getReservationStatusColor = getReservationStatusColor;
exports.canConfirmReservation = canConfirmReservation;
exports.canMarkNoShow = canMarkNoShow;
exports.formatReservationDateTime = formatReservationDateTime;
exports.calculateRoomUtilization = calculateRoomUtilization;
const request_1 = require("../utils/request");
// ==================== 预定管理服务类 ====================
/**
 * 预定管理服务
 * 提供商户端预定管理功能，包括预定列表、统计、操作等
 */
class ReservationManagementService {
    /**
     * 获取商户预定列表
     * @param params 查询参数
     */
    getMerchantReservations(params) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/reservations/merchant',
                method: 'GET',
                data: params
            });
        });
    }
    /**
     * 获取预定统计
     */
    getReservationStats() {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: '/v1/reservations/merchant/stats',
                method: 'GET'
            });
        });
    }
    /**
     * 确认预定
     * @param reservationId 预定ID
     */
    confirmReservation(reservationId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/reservations/${reservationId}/confirm`,
                method: 'POST'
            });
        });
    }
    /**
     * 标记未到店
     * @param reservationId 预定ID
     */
    markNoShow(reservationId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/reservations/${reservationId}/no-show`,
                method: 'POST'
            });
        });
    }
}
exports.ReservationManagementService = ReservationManagementService;
// ==================== 包间管理服务类 ====================
/**
 * 包间管理服务
 * 提供包间查询和管理功能
 */
class RoomManagementService {
    /**
     * 获取商户可用包间列表
     * @param merchantId 商户ID
     */
    getAvailableRooms(merchantId) {
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
    getAllRooms(merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            return (0, request_1.request)({
                url: `/v1/merchants/${merchantId}/rooms/all`,
                method: 'GET'
            });
        });
    }
}
exports.RoomManagementService = RoomManagementService;
// ==================== 数据适配器 ====================
/**
 * 预定和包间管理数据适配器
 * 处理前端数据格式与后端API数据格式的转换
 */
class ReservationRoomAdapter {
    /**
     * 适配预定响应数据
     */
    static adaptReservationResponse(data) {
        return {
            id: data.id,
            userId: data.user_id,
            merchantId: data.merchant_id,
            tableId: data.table_id,
            tableNo: data.table_no,
            tableType: data.table_type,
            contactName: data.contact_name,
            contactPhone: data.contact_phone,
            guestCount: data.guest_count,
            reservationDate: data.reservation_date,
            reservationTime: data.reservation_time,
            paymentMode: data.payment_mode,
            depositAmount: data.deposit_amount,
            prepaidAmount: data.prepaid_amount,
            status: data.status,
            notes: data.notes,
            paymentDeadline: data.payment_deadline,
            refundDeadline: data.refund_deadline,
            cancelReason: data.cancel_reason,
            createdAt: data.created_at,
            updatedAt: data.updated_at,
            paidAt: data.paid_at,
            confirmedAt: data.confirmed_at,
            completedAt: data.completed_at,
            cancelledAt: data.cancelled_at
        };
    }
    /**
     * 适配包间列表项响应数据
     */
    static adaptRoomListItemResponse(data) {
        return {
            id: data.id,
            merchantId: data.merchant_id,
            roomNo: data.room_no,
            capacity: data.capacity,
            minimumSpend: data.minimum_spend,
            description: data.description,
            status: data.status,
            primaryImage: data.primary_image,
            monthlyReservations: data.monthly_reservations
        };
    }
    /**
     * 适配预定统计响应数据
     */
    static adaptReservationStatsResponse(data) {
        const totalCount = data.pending_count + data.paid_count + data.confirmed_count +
            data.completed_count + data.cancelled_count + data.expired_count + data.no_show_count;
        return {
            pendingCount: data.pending_count,
            paidCount: data.paid_count,
            confirmedCount: data.confirmed_count,
            completedCount: data.completed_count,
            cancelledCount: data.cancelled_count,
            expiredCount: data.expired_count,
            noShowCount: data.no_show_count,
            totalCount
        };
    }
}
exports.ReservationRoomAdapter = ReservationRoomAdapter;
// ==================== 导出服务实例 ====================
exports.reservationManagementService = new ReservationManagementService();
exports.roomManagementService = new RoomManagementService();
// ==================== 便捷函数 ====================
/**
 * 获取今日预定列表
 */
function getTodayReservations() {
    return __awaiter(this, void 0, void 0, function* () {
        const today = new Date().toISOString().split('T')[0];
        const result = yield exports.reservationManagementService.getMerchantReservations({
            date: today,
            page_id: 1,
            page_size: 50
        });
        return result.reservations;
    });
}
/**
 * 获取待处理预定列表
 */
function getPendingReservations() {
    return __awaiter(this, void 0, void 0, function* () {
        const result = yield exports.reservationManagementService.getMerchantReservations({
            status: 'paid',
            page_id: 1,
            page_size: 50
        });
        return result.reservations;
    });
}
/**
 * 获取预定概览数据
 */
function getReservationOverview() {
    return __awaiter(this, void 0, void 0, function* () {
        const [stats, todayReservations, pendingReservations] = yield Promise.all([
            exports.reservationManagementService.getReservationStats(),
            getTodayReservations(),
            getPendingReservations()
        ]);
        return {
            stats,
            todayReservations,
            pendingReservations
        };
    });
}
/**
 * 批量确认预定
 * @param reservationIds 预定ID列表
 */
function batchConfirmReservations(reservationIds) {
    return __awaiter(this, void 0, void 0, function* () {
        const promises = reservationIds.map((reservationId) => __awaiter(this, void 0, void 0, function* () {
            try {
                const reservation = yield exports.reservationManagementService.confirmReservation(reservationId);
                return { reservationId, success: true, message: '确认成功', reservation };
            }
            catch (error) {
                return {
                    reservationId,
                    success: false,
                    message: (error === null || error === void 0 ? void 0 : error.message) || '确认失败'
                };
            }
        }));
        return Promise.all(promises);
    });
}
/**
 * 批量标记未到店
 * @param reservationIds 预定ID列表
 */
function batchMarkNoShow(reservationIds) {
    return __awaiter(this, void 0, void 0, function* () {
        const promises = reservationIds.map((reservationId) => __awaiter(this, void 0, void 0, function* () {
            try {
                const reservation = yield exports.reservationManagementService.markNoShow(reservationId);
                return { reservationId, success: true, message: '标记成功', reservation };
            }
            catch (error) {
                return {
                    reservationId,
                    success: false,
                    message: (error === null || error === void 0 ? void 0 : error.message) || '标记失败'
                };
            }
        }));
        return Promise.all(promises);
    });
}
/**
 * 计算预定统计指标
 * @param stats 预定统计数据
 */
function calculateReservationMetrics(stats) {
    const totalReservations = stats.pending_count + stats.paid_count + stats.confirmed_count +
        stats.completed_count + stats.cancelled_count + stats.expired_count + stats.no_show_count;
    if (totalReservations === 0) {
        return {
            totalReservations: 0,
            completionRate: 0,
            noShowRate: 0,
            cancellationRate: 0,
            confirmationRate: 0
        };
    }
    return {
        totalReservations,
        // 完成率 = 已完成 / 总数
        completionRate: (stats.completed_count / totalReservations) * 100,
        // 未到店率 = 未到店 / 总数
        noShowRate: (stats.no_show_count / totalReservations) * 100,
        // 取消率 = (已取消 + 已过期) / 总数
        cancellationRate: ((stats.cancelled_count + stats.expired_count) / totalReservations) * 100,
        // 确认率 = (已确认 + 已完成) / (已支付 + 已确认 + 已完成 + 未到店)
        confirmationRate: ((stats.confirmed_count + stats.completed_count) /
            (stats.paid_count + stats.confirmed_count + stats.completed_count + stats.no_show_count)) * 100
    };
}
/**
 * 获取预定状态显示文本
 * @param status 预定状态
 */
function getReservationStatusText(status) {
    const statusMap = {
        pending: '待支付',
        paid: '已支付',
        confirmed: '已确认',
        completed: '已完成',
        cancelled: '已取消',
        expired: '已过期',
        no_show: '未到店'
    };
    return statusMap[status] || status;
}
/**
 * 获取预定状态颜色
 * @param status 预定状态
 */
function getReservationStatusColor(status) {
    const colorMap = {
        pending: '#f39c12', // 橙色
        paid: '#3498db', // 蓝色
        confirmed: '#2ecc71', // 绿色
        completed: '#27ae60', // 深绿色
        cancelled: '#95a5a6', // 灰色
        expired: '#e74c3c', // 红色
        no_show: '#e67e22' // 深橙色
    };
    return colorMap[status] || '#95a5a6';
}
/**
 * 检查预定是否可以确认
 * @param reservation 预定信息
 */
function canConfirmReservation(reservation) {
    return reservation.status === 'paid';
}
/**
 * 检查预定是否可以标记未到店
 * @param reservation 预定信息
 */
function canMarkNoShow(reservation) {
    return reservation.status === 'paid' || reservation.status === 'confirmed';
}
/**
 * 格式化预定时间显示
 * @param date 预定日期
 * @param time 预定时间
 */
function formatReservationDateTime(date, time) {
    const reservationDate = new Date(`${date}T${time}`);
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const tomorrow = new Date(today.getTime() + 24 * 60 * 60 * 1000);
    const reservationDay = new Date(reservationDate.getFullYear(), reservationDate.getMonth(), reservationDate.getDate());
    let dateText = '';
    if (reservationDay.getTime() === today.getTime()) {
        dateText = '今天';
    }
    else if (reservationDay.getTime() === tomorrow.getTime()) {
        dateText = '明天';
    }
    else {
        dateText = `${reservationDate.getMonth() + 1}月${reservationDate.getDate()}日`;
    }
    const timeText = reservationDate.toLocaleTimeString('zh-CN', {
        hour: '2-digit',
        minute: '2-digit',
        hour12: false
    });
    return `${dateText} ${timeText}`;
}
/**
 * 计算包间利用率
 * @param rooms 包间列表
 */
function calculateRoomUtilization(rooms) {
    const totalRooms = rooms.length;
    const availableRooms = rooms.filter(room => room.status === 'available').length;
    const occupiedRooms = rooms.filter(room => room.status === 'occupied' || room.status === 'reserved').length;
    const utilizationRate = totalRooms > 0 ? (occupiedRooms / totalRooms) * 100 : 0;
    return {
        totalRooms,
        availableRooms,
        occupiedRooms,
        utilizationRate
    };
}
