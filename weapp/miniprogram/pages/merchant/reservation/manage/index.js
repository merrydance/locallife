"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const reservation_1 = require("../../../../api/reservation");
const reservation_2 = __importDefault(require("../../../../adapters/reservation"));
Page({
    data: {
        currentTab: 'pending',
        tabs: [
            { value: 'pending', label: '待确认', count: 0 },
            { value: 'confirmed', label: '已确认', count: 0 },
            { value: 'history', label: '历史记录' }
        ],
        reservations: [],
        page: 1,
        pageSize: 10,
        hasMore: true,
        loading: false,
        // Action Dialogs
        showRejectDialog: false,
        selectedId: 0,
        rejectReason: ''
    },
    onLoad() {
        this.loadReservations(true);
    },
    onPullDownRefresh() {
        this.loadReservations(true).then(() => {
            wx.stopPullDownRefresh();
        });
    },
    onTabChange(e) {
        this.setData({
            currentTab: e.detail.value,
            reservations: [],
            page: 1,
            hasMore: true
        });
        this.loadReservations(true);
    },
    loadReservations(reset) {
        return __awaiter(this, void 0, void 0, function* () {
            if (this.data.loading && !reset)
                return;
            this.setData({ loading: true });
            try {
                const page = reset ? 1 : this.data.page;
                // Map tab to API status param
                let status;
                if (this.data.currentTab === 'pending')
                    status = 'pending';
                if (this.data.currentTab === 'confirmed')
                    status = 'confirmed';
                // History covers completed, cancelled, no_show. API needs adjustment if it doesn't support multiple statuses.
                // For now, let's assume 'history' fetches all others or we filter client-side if API is limited.
                // Simplified: if history, we don't pass status (fetch all) and filter? No, standard is backend support.
                // Let's assume the API `getReservations` handles single status. For history, we might strictly check 'completed'.
                // Or just 'completed' for now.
                if (this.data.currentTab === 'history')
                    status = 'completed';
                const res = yield reservation_1.ReservationService.getReservations({
                    page_id: page,
                    page_size: this.data.pageSize,
                    status
                });
                const formatted = res.reservations.map(r => (Object.assign(Object.assign({}, r), { _statusText: reservation_2.default.formatStatus(r.status), _statusTheme: reservation_2.default.getStatusTheme(r.status), _timeText: reservation_2.default.formatDateTime(r.reservation_time) })));
                this.setData({
                    reservations: reset ? formatted : [...this.data.reservations, ...formatted],
                    page: page + 1,
                    hasMore: formatted.length === this.data.pageSize,
                    loading: false
                });
            }
            catch (error) {
                console.error(error);
                this.setData({ loading: false });
            }
        });
    },
    // Actions
    onConfirm(e) {
        return __awaiter(this, void 0, void 0, function* () {
            const id = e.currentTarget.dataset.id;
            try {
                wx.showLoading({ title: '处理中' });
                yield reservation_1.ReservationService.confirmReservation(id);
                wx.showToast({ title: '已确认', icon: 'success' });
                this.loadReservations(true);
            }
            catch (error) {
                wx.showToast({ title: error.message || '操作失败', icon: 'none' });
            }
            finally {
                wx.hideLoading();
            }
        });
    },
    onReject(e) {
        const id = e.currentTarget.dataset.id;
        this.setData({
            showRejectDialog: true,
            selectedId: id,
            rejectReason: ''
        });
    },
    closeRejectDialog() {
        this.setData({ showRejectDialog: false });
    },
    onRejectInput(e) {
        this.setData({ rejectReason: e.detail.value });
    },
    confirmReject() {
        return __awaiter(this, void 0, void 0, function* () {
            if (!this.data.rejectReason) {
                wx.showToast({ title: '请输入拒绝原因', icon: 'none' });
                return;
            }
            try {
                wx.showLoading({ title: '处理中' });
                yield reservation_1.ReservationService.rejectReservation(this.data.selectedId, this.data.rejectReason);
                wx.showToast({ title: '已拒绝', icon: 'success' });
                this.closeRejectDialog();
                this.loadReservations(true);
            }
            catch (error) {
                wx.showToast({ title: error.message || '操作失败', icon: 'none' });
            }
            finally {
                wx.hideLoading();
            }
        });
    },
    onCallUser(e) {
        const phone = e.currentTarget.dataset.phone;
        wx.makePhoneCall({ phoneNumber: phone });
    }
});
