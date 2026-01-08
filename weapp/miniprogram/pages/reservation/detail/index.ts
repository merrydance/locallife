import { ReservationService, ReservationResponse, ReservationStatus } from '../../../api/reservation';
import ReservationAdapter from '../../../adapters/reservation';

Page({
    data: {
        id: 0,
        reservation: null as (ReservationResponse & { _statusText: string, _statusTheme: string, _timeText: string }) | null,
        loading: true,
        showCancelDialog: false,
        cancelReason: '',
        cancelReasons: ['行程改变', '订错了', '不想去了', '其他原因']
    },

    onLoad(options: any) {
        if (options.id) {
            this.setData({ id: parseInt(options.id) });
            this.loadDetail();
        }
    },

    async loadDetail() {
        this.setData({ loading: true });
        try {
            const res = await ReservationService.getReservationDetail(this.data.id);

            const formatted = {
                ...res,
                _statusText: ReservationAdapter.formatStatus(res.status),
                _statusTheme: ReservationAdapter.getStatusTheme(res.status),
                _timeText: ReservationAdapter.formatFullDateTime(res.reservation_time)
            };

            this.setData({ reservation: formatted, loading: false });
        } catch (error) {
            console.error(error);
            wx.showToast({ title: '加载失败', icon: 'none' });
            this.setData({ loading: false });
        }
    },

    onCancel() {
        this.setData({ showCancelDialog: true });
    },

    closeCancelDialog() {
        this.setData({ showCancelDialog: false });
    },

    onReasonChange(e: any) {
        this.setData({ cancelReason: e.detail.value });
    },

    async confirmCancel() {
        if (!this.data.cancelReason) {
            wx.showToast({ title: '请选择取消原因', icon: 'none' });
            return;
        }

        try {
            wx.showLoading({ title: '提交中...' });
            await ReservationService.cancelReservation(this.data.id, this.data.cancelReason);
            wx.showToast({ title: '已取消', icon: 'success' });
            this.closeCancelDialog();
            this.loadDetail();
        } catch (error: any) {
            wx.showToast({ title: error.message || '取消失败', icon: 'none' });
        } finally {
            wx.hideLoading();
        }
    },

    onCallMerchant() {
        // Placeholder for calling merchant
        wx.makePhoneCall({ phoneNumber: '13800000000' });
    },

    /**
     * 跳转到点菜页面（定金模式顾客到店后点菜）
     */
    onGoToOrder() {
        const { reservation } = this.data;
        if (!reservation) return;

        wx.navigateTo({
            url: `/pages/dine-in/menu/menu?reservation_id=${reservation.id}&merchant_id=${reservation.merchant_id}`
        });
    }
});
