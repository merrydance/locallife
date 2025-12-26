"use strict";
/**
 * 我的预订页面
 * 显示用户的所有预订记录
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
const reservation_1 = require("../../../api/reservation");
const logger_1 = require("../../../utils/logger");
// 状态筛选选项
const STATUS_TABS = [
    { label: '全部', value: '' },
    { label: '待支付', value: 'pending' },
    { label: '已确认', value: 'confirmed' },
    { label: '已完成', value: 'completed' },
    { label: '已取消', value: 'cancelled' }
];
// 取消原因选项
const CANCEL_REASONS = [
    '行程有变',
    '预订错误',
    '找到更好的选择',
    '其他原因'
];
Page({
    data: {
        reservations: [],
        navBarHeight: 88,
        loading: false,
        page: 1,
        pageSize: 10,
        hasMore: true,
        statusTabs: STATUS_TABS,
        currentStatus: ''
    },
    onLoad() {
        this.loadReservations(true);
    },
    onShow() {
        if (this.data.reservations.length > 0) {
            this.loadReservations(true);
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    onReachBottom() {
        if (this.data.hasMore && !this.data.loading) {
            this.setData({ page: this.data.page + 1 });
            this.loadReservations(false);
        }
    },
    loadReservations() {
        return __awaiter(this, arguments, void 0, function* (reset = false) {
            if (this.data.loading)
                return;
            this.setData({ loading: true });
            if (reset) {
                this.setData({ page: 1, reservations: [], hasMore: true });
            }
            try {
                const { currentStatus, page, pageSize } = this.data;
                const params = { page_id: page, page_size: pageSize };
                if (currentStatus) {
                    params.status = currentStatus;
                }
                const response = yield reservation_1.ReservationService.getReservations(params);
                const result = response.reservations;
                // 处理显示字段
                const processedReservations = result.map(r => this.processReservation(r));
                const reservations = reset ? processedReservations : [...this.data.reservations, ...processedReservations];
                this.setData({
                    reservations,
                    loading: false,
                    hasMore: result.length === pageSize
                });
            }
            catch (error) {
                logger_1.logger.error('加载预订列表失败', error, 'reservations.loadReservations');
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    processReservation(r) {
        return Object.assign(Object.assign({}, r), { _statusText: this.getStatusText(r.status || ''), _statusClass: r.status || '', _canCancel: ['pending', 'paid', 'confirmed'].includes(r.status || ''), _dateTimeDisplay: r.reservation_time, _depositDisplay: r.deposit_amount ? `¥${(r.deposit_amount / 100).toFixed(2)}` : '' });
    },
    getStatusText(status) {
        const statusMap = {
            'pending': '待支付',
            'paid': '已支付',
            'confirmed': '已确认',
            'completed': '已完成',
            'cancelled': '已取消',
            'no_show': '未到店'
        };
        return statusMap[status] || status;
    },
    onStatusChange(e) {
        const status = e.detail.value || '';
        if (status === this.data.currentStatus)
            return;
        this.setData({ currentStatus: status });
        this.loadReservations(true);
    },
    onViewDetail(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({
            url: `/pages/user_center/reservations/detail/index?id=${id}`
        });
    },
    onCancelReservation(e) {
        const { id } = e.currentTarget.dataset;
        if (!id)
            return;
        wx.showActionSheet({
            itemList: CANCEL_REASONS,
            success: (res) => __awaiter(this, void 0, void 0, function* () {
                const reason = CANCEL_REASONS[res.tapIndex];
                yield this.doCancelReservation(Number(id), reason);
            })
        });
    },
    doCancelReservation(reservationId, reason) {
        return __awaiter(this, void 0, void 0, function* () {
            wx.showLoading({ title: '取消中...' });
            try {
                yield reservation_1.ReservationService.cancelReservation(reservationId, reason);
                wx.hideLoading();
                wx.showToast({ title: '已取消', icon: 'success' });
                setTimeout(() => this.loadReservations(true), 1500);
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('取消预订失败', error, 'reservations.doCancelReservation');
                wx.showToast({ title: '取消失败', icon: 'error' });
            }
        });
    }
});
