import { ReservationItem, ReservationService, ReservationResponse, ReservationStatus } from '../../../api/reservation'
import { processPayment } from '../../../api/payment'
import ReservationAdapter from '../../../adapters/reservation'

type ReservationItemView = ReservationItem & {
	_displayName: string
	_unitPrice: string
	_totalPrice: string
	_image: string
}

type ReservationView = ReservationResponse & {
    _statusText: string
    _statusTheme: string
    _statusColor: string
    _timeText: string
    _createdAt: string
    _reservationDate: string
    _reservationTime: string
    _guestCount: string
    items?: ReservationItemView[]
}

Page({
    data: {
        id: 0,
        reservation: null as ReservationView | null,
        loading: true,
        navBarHeight: 88,
        showCancelDialog: false,
        cancelReason: '',
        cancelReasons: ['行程改变', '订错了', '不想去了', '其他原因'],
        showPayButton: false,
        showCancelButton: false
    },

    onLoad(options: any) {
        if (options.id) {
            this.setData({ id: parseInt(options.id) })
            this.loadDetail()
        }
    },

    onShow() {
        // refresh when returning from pay/menu/cart
        if (this.data.id) {
            this.loadDetail()
        }
    },

    onNavHeight(e: WechatMiniprogram.CustomEvent) {
        this.setData({ navBarHeight: e.detail.navBarHeight || 88 })
    },

    async loadDetail() {
        this.setData({ loading: true })
        try {
            const res = await ReservationService.getReservationDetail(this.data.id)

            const mappedItems = this.mapReservationItems(res.items || [])

            const formatted: ReservationView = {
                ...res,
                items: mappedItems,
                _statusText: ReservationAdapter.formatStatus(res.status),
                _statusTheme: ReservationAdapter.getStatusTheme(res.status),
                _statusColor: this.getStatusColor(res.status),
                _timeText: this.formatReservationDateTime(res.reservation_date, res.reservation_time),
                _createdAt: this.formatDateSafe(res.created_at as any),
                _reservationDate: res.reservation_date || '--',
                _reservationTime: res.reservation_time || '--',
                _guestCount: (res.guest_count || (res as any).party_size) ? `${res.guest_count || (res as any).party_size}人` : '--'
            }

            const showPayButton = res.status === 'pending'
            const showCancelButton = ['pending', 'paid', 'confirmed'].includes(res.status)

            this.setData({
                reservation: formatted,
                loading: false,
                showPayButton,
                showCancelButton
            })
        } catch (error) {
            console.error(error)
            wx.showToast({ title: '加载失败', icon: 'none' })
            this.setData({ loading: false })
        }
    },

    getStatusColor(status: ReservationStatus): string {
        switch (status) {
            case 'completed':
            case 'checked_in':
                return '#00A870'
            case 'cancelled':
            case 'expired':
            case 'no_show':
                return '#A0A4B3'
            case 'pending':
                return '#FF9F45'
            default:
                return '#1F7AEC'
        }
    },

    formatDateSafe(value?: string): string {
        if (!value) return '--'
        const d = new Date(value.replace(/-/g, '/'))
        if (Number.isNaN(d.getTime())) return value
        return ReservationAdapter.formatFullDateTime(value)
    },

    formatReservationDateTime(dateStr?: string, timeStr?: string): string {
        const datePart = (dateStr || '').trim()
        const timePart = (timeStr || '').trim()
        if (!datePart && !timePart) return '--'

        // Prefer combined parsing to avoid NaN when time lacks date
        if (datePart && timePart) {
            const combined = `${datePart} ${timePart}`.replace(/-/g, '/')
            const parsed = new Date(combined)
            if (!Number.isNaN(parsed.getTime())) {
                const y = parsed.getFullYear()
                const m = ('0' + (parsed.getMonth() + 1)).slice(-2)
                const d = ('0' + parsed.getDate()).slice(-2)
                const hh = ('0' + parsed.getHours()).slice(-2)
                const mm = ('0' + parsed.getMinutes()).slice(-2)
                return `${y}-${m}-${d} ${hh}:${mm}`
            }
        }

        // Fallback to whichever parts we have
        if (datePart && timePart) return `${datePart} ${timePart}`
        if (datePart) return datePart
        return timePart
    },

    mapReservationItems(items: ReservationItem[]): ReservationItemView[] {
        if (!items || items.length === 0) return []

        return items.map((item) => {
            const unitPrice = this.formatPrice(item.unit_price ?? item.price)
            const totalPrice = this.formatPrice(
                item.total_price ?? (item.unit_price ?? item.price ?? 0) * (item.quantity || 1)
            )
            return {
                ...item,
                _displayName: item.name || (item.type === 'combo' ? '套餐' : '菜品'),
                _unitPrice: unitPrice,
                _totalPrice: totalPrice,
                _image: this.normalizeImage(item.image_url)
            }
        })
    },

    formatPrice(amount?: number): string {
        if (amount === undefined || amount === null) return '--'
        return `¥${(amount / 100).toFixed(2)}`
    },

    normalizeImage(url?: string): string {
        if (!url) return ''
        if (url.startsWith('http')) return url
        if (url.startsWith('/')) return url
        return url
    },

    onCancel() {
        this.setData({ showCancelDialog: true })
    },

    closeCancelDialog() {
        this.setData({ showCancelDialog: false })
    },

    onReasonChange(e: any) {
        this.setData({ cancelReason: e.detail.value })
    },

    async confirmCancel() {
        if (!this.data.cancelReason) {
            wx.showToast({ title: '请选择取消原因', icon: 'none' })
            return
        }

        try {
            wx.showLoading({ title: '提交中...' })
            await ReservationService.cancelReservation(this.data.id, this.data.cancelReason)
            wx.showToast({ title: '已取消', icon: 'success' })
            this.closeCancelDialog()
            this.loadDetail()
        } catch (error: any) {
            wx.showToast({ title: error?.message || '取消失败', icon: 'none' })
        } finally {
            wx.hideLoading()
        }
    },

    onCallMerchant() {
        const phone = this.data.reservation?.merchant_phone || this.data.reservation?.contact_phone
        if (!phone) {
            wx.showToast({ title: '暂无电话', icon: 'none' })
            return
        }
        wx.makePhoneCall({ phoneNumber: phone })
    },

    async onPay() {
        if (!this.data.reservation) return
        try {
            wx.showLoading({ title: '拉起支付' })
            await processPayment(this.data.reservation.id, 'reservation' as any)
            wx.showToast({ title: '支付成功', icon: 'success' })
            this.loadDetail()
        } catch (error: any) {
            wx.showToast({ title: error?.message || '支付失败', icon: 'none' })
        } finally {
            wx.hideLoading()
        }
    }
});
