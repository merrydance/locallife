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
const reservation_1 = require("../../../api/reservation");
const reservation_2 = __importDefault(require("../../../adapters/reservation"));
Page({
    data: {
        currentTab: 'all',
        tabs: [
            { value: 'all', label: '全部' },
            { value: 'pending', label: '待确认' },
            { value: 'confirmed', label: '已确认' },
            { value: 'completed', label: '已完成' },
            { value: 'cancelled', label: '已取消' }
        ],
        reservations: [],
        page: 1,
        pageSize: 10,
        hasMore: true,
        loading: false,
        refreshing: false
    },
    onLoad() {
        this.loadReservations(true);
    },
    onShow() {
        // Refresh list when returning from detail or create page
        if (this.data.reservations.length > 0) {
            this.loadReservations(true);
        }
    },
    onPullDownRefresh() {
        this.setData({ refreshing: true });
        this.loadReservations(true).then(() => {
            this.setData({ refreshing: false });
            wx.stopPullDownRefresh();
        });
    },
    onReachBottom() {
        if (this.data.hasMore && !this.data.loading) {
            this.loadReservations(false);
        }
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
                const status = this.data.currentTab === 'all' ? undefined : this.data.currentTab;
                const res = yield reservation_1.ReservationService.getReservations({
                    page_id: page,
                    page_size: this.data.pageSize,
                    status
                });
                const formattedReservations = res.reservations.map(r => (Object.assign(Object.assign({}, r), { _statusText: reservation_2.default.formatStatus(r.status), _statusTheme: reservation_2.default.getStatusTheme(r.status), _timeText: reservation_2.default.formatDateTime(r.reservation_time) })));
                this.setData({
                    reservations: reset ? formattedReservations : [...this.data.reservations, ...formattedReservations],
                    page: page + 1,
                    hasMore: formattedReservations.length === this.data.pageSize,
                    loading: false
                });
            }
            catch (error) {
                console.error(error);
                this.setData({ loading: false });
                wx.showToast({ title: '加载失败', icon: 'none' });
            }
        });
    },
    onToDetail(e) {
        const id = e.currentTarget.dataset.id;
        wx.navigateTo({
            url: `/pages/reservation/detail/index?id=${id}`
        });
    },
    onToCreate() {
        // For demo, list 10 dummy merchants or assume global merchant selection
        // In real app, this might go to merchant list or take a specific merchant ID
        wx.navigateTo({
            url: '/pages/merchant/list/index?action=reservation' // Assuming this exists or just go to home
        });
    }
});
