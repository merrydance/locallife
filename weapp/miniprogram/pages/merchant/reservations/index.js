"use strict";
/**
 * 商户预约管理页面
 * 使用真实后端API
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
const responsive_1 = require("../../../utils/responsive");
const reservation_1 = require("../../../api/reservation");
const logger_1 = require("../../../utils/logger");
Page({
    data: {
        activeTab: 'pending',
        reservations: [],
        isLargeScreen: false,
        navBarHeight: 88,
        loading: false
    },
    onLoad() {
        this.setData({ isLargeScreen: (0, responsive_1.isLargeScreen)() });
        this.loadReservations();
    },
    onShow() {
        // 返回时刷新
        if (this.data.reservations.length > 0) {
            this.loadReservations();
        }
    },
    onNavHeight(e) {
        this.setData({ navBarHeight: e.detail.navBarHeight });
    },
    loadReservations() {
        return __awaiter(this, void 0, void 0, function* () {
            this.setData({ loading: true });
            try {
                const { activeTab } = this.data;
                const result = yield (0, reservation_1.getMerchantReservations)({
                    page_id: 1,
                    page_size: 50,
                    status: activeTab
                });
                const reservations = (result || []).map((res) => ({
                    id: res.id,
                    contact_name: res.contact_name || '顾客',
                    contact_phone: res.contact_phone || '',
                    table_id: res.table_id,
                    table_no: res.table_no,
                    guest_count: res.guest_count,
                    reservation_date: res.reservation_date,
                    reservation_time: res.reservation_time,
                    deposit: res.prepaid_amount || res.deposit_amount || 0,
                    status: res.status,
                    notes: res.notes,
                    created_at: res.created_at
                }));
                this.setData({
                    reservations,
                    loading: false
                });
            }
            catch (error) {
                console.error('加载预约失败:', error);
                wx.showToast({ title: '加载失败', icon: 'error' });
                this.setData({ loading: false });
            }
        });
    },
    onTabChange(e) {
        this.setData({ activeTab: e.detail.value });
        this.loadReservations();
    },
    onViewDetail(e) {
        const { id } = e.currentTarget.dataset;
        wx.navigateTo({ url: `/pages/merchant/reservations/detail/index?id=${id}` });
    },
    // 确认预订
    onConfirmReservation(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id } = e.currentTarget.dataset;
            if (!id)
                return;
            wx.showModal({
                title: '确认预订',
                content: '确定要确认此预订吗？',
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        yield this.doConfirmReservation(Number(id));
                    }
                })
            });
        });
    },
    doConfirmReservation(reservationId) {
        return __awaiter(this, void 0, void 0, function* () {
            wx.showLoading({ title: '处理中...' });
            try {
                yield (0, reservation_1.confirmReservationByMerchant)(reservationId);
                wx.hideLoading();
                wx.showToast({ title: '已确认', icon: 'success' });
                this.loadReservations();
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('确认预订失败', error, 'merchant/reservations.doConfirmReservation');
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
        });
    },
    // 标记未到店
    onMarkNoShow(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id } = e.currentTarget.dataset;
            if (!id)
                return;
            wx.showModal({
                title: '标记未到店',
                content: '确定要标记此预订为未到店吗？定金将被没收。',
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        yield this.doMarkNoShow(Number(id));
                    }
                })
            });
        });
    },
    doMarkNoShow(reservationId) {
        return __awaiter(this, void 0, void 0, function* () {
            wx.showLoading({ title: '处理中...' });
            try {
                yield (0, reservation_1.markReservationNoShow)(reservationId);
                wx.hideLoading();
                wx.showToast({ title: '已标记', icon: 'success' });
                this.loadReservations();
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('标记未到店失败', error, 'merchant/reservations.doMarkNoShow');
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
        });
    },
    // 完成预订
    onCompleteReservation(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const { id } = e.currentTarget.dataset;
            if (!id)
                return;
            wx.showModal({
                title: '完成预订',
                content: '确定要完成此预订吗？',
                success: (res) => __awaiter(this, void 0, void 0, function* () {
                    if (res.confirm) {
                        yield this.doCompleteReservation(Number(id));
                    }
                })
            });
        });
    },
    doCompleteReservation(reservationId) {
        return __awaiter(this, void 0, void 0, function* () {
            wx.showLoading({ title: '处理中...' });
            try {
                yield (0, reservation_1.completeReservationByMerchant)(reservationId);
                wx.hideLoading();
                wx.showToast({ title: '已完成', icon: 'success' });
                this.loadReservations();
            }
            catch (error) {
                wx.hideLoading();
                logger_1.logger.error('完成预订失败', error, 'merchant/reservations.doCompleteReservation');
                wx.showToast({ title: '操作失败', icon: 'error' });
            }
        });
    }
});
