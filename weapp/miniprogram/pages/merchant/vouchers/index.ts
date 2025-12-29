/**
 * 代金券管理页面
 * 功能：代金券的创建、编辑、启用/停用、删除
 * 遵循 PC-SaaS 布局规范
 */

import { logger } from '@/utils/logger'
import {
    VoucherManagementService,
    MarketingAdapter,
    type VoucherResponse,
    type CreateVoucherRequest,
    type UpdateVoucherRequest
} from '@/api/marketing-management'

interface VoucherWithStatus extends VoucherResponse {
    statusText: string
    statusClass: string
}

interface CalendarDay {
    day: number
    date: string
    disabled: boolean
    selected: boolean
    today: boolean
    currentMonth: boolean
}

Page({
    data: {
        // 布局状态
        sidebarCollapsed: false,
        loading: true,

        // 代金券列表
        vouchers: [] as VoucherWithStatus[],

        // 弹窗状态
        showModal: false,
        editingVoucher: null as VoucherResponse | null,
        form: {
            name: '',
            code: '',
            amount: '',
            min_order_amount: '',
            total_quantity: '',
            valid_from: '',
            valid_until: '',
            description: '',
            allowed_order_types: [] as string[]
        },

        // 订单类型选项
        orderTypeOptions: [
            { value: 'dine_in', label: '堂食' },
            { value: 'takeout', label: '外卖' },
            { value: 'reservation', label: '预订' }
        ],

        // 商户ID
        merchantId: 0,

        // 日历选择器状态
        showCalendar: false,
        calendarField: '' as string,
        calendarYear: 2024,
        calendarMonth: 1,
        calendarDays: [] as CalendarDay[]
    },

    async onLoad() {
        await this.initData()
    },

    async initData() {
        const app = getApp<IAppOption>()
        const merchantId = app.globalData.merchantId

        if (merchantId) {
            this.setData({ merchantId: Number(merchantId) })
            await this.loadVouchers()
        } else {
            app.userInfoReadyCallback = async () => {
                if (app.globalData.merchantId) {
                    this.setData({ merchantId: Number(app.globalData.merchantId) })
                    await this.loadVouchers()
                }
            }
        }
    },

    onSidebarCollapse(e: WechatMiniprogram.CustomEvent) {
        this.setData({ sidebarCollapsed: e.detail.collapsed })
    },

    // ==================== 数据加载 ====================

    async loadVouchers() {
        const { merchantId } = this.data
        if (!merchantId) return

        this.setData({ loading: true })
        try {
            const vouchers = await VoucherManagementService.getVoucherList(merchantId, {
                page_id: 1,
                page_size: 50
            })

            // 添加状态信息
            const vouchersWithStatus: VoucherWithStatus[] = vouchers.map(v => ({
                ...v,
                statusText: MarketingAdapter.getVoucherStatusText(v),
                statusClass: this.getStatusClass(v)
            }))

            this.setData({ vouchers: vouchersWithStatus })
        } catch (error) {
            logger.error('加载代金券失败', error, 'vouchers')
            wx.showToast({ title: '加载失败', icon: 'error' })
        } finally {
            this.setData({ loading: false })
        }
    },

    getStatusClass(v: VoucherResponse): string {
        if (!v.is_active) return 'inactive'
        if (MarketingAdapter.isVoucherExpired(v)) return 'expired'
        if (MarketingAdapter.isVoucherSoldOut(v)) return 'soldout'
        return 'active'
    },

    // ==================== 代金券操作 ====================

    onAddVoucher() {
        const today = new Date()
        const nextMonth = new Date()
        nextMonth.setMonth(nextMonth.getMonth() + 1)

        this.setData({
            showModal: true,
            editingVoucher: null,
            form: {
                name: '',
                code: '',
                amount: '',
                min_order_amount: '0',
                total_quantity: '100',
                valid_from: this.formatDate(today),
                valid_until: this.formatDate(nextMonth),
                description: '',
                allowed_order_types: ['dine_in', 'takeout', 'reservation']
            }
        })
    },

    onEditVoucher(e: WechatMiniprogram.TouchEvent) {
        const voucher = e.currentTarget.dataset.voucher as VoucherResponse
        this.setData({
            showModal: true,
            editingVoucher: voucher,
            form: {
                name: voucher.name,
                code: voucher.code,
                amount: String(voucher.amount / 100),
                min_order_amount: String(voucher.min_order_amount / 100),
                total_quantity: String(voucher.total_quantity),
                valid_from: voucher.valid_from.slice(0, 10),
                valid_until: voucher.valid_until.slice(0, 10),
                description: voucher.description || '',
                allowed_order_types: [...voucher.allowed_order_types]
            }
        })
    },

    onCloseModal() {
        this.setData({ showModal: false, editingVoucher: null })
    },

    onModalContentTap() {
        // 阻止冒泡
    },

    onFormInput(e: WechatMiniprogram.Input) {
        const field = e.currentTarget.dataset.field as string
        this.setData({ [`form.${field}`]: e.detail.value })
    },

    onOrderTypeToggle(e: WechatMiniprogram.TouchEvent) {
        const type = e.currentTarget.dataset.type as string
        const types = [...this.data.form.allowed_order_types]
        const index = types.indexOf(type)
        if (index > -1) {
            types.splice(index, 1)
        } else {
            types.push(type)
        }
        this.setData({ 'form.allowed_order_types': types })
    },

    async onSaveVoucher() {
        const { merchantId, editingVoucher, form } = this.data

        // 验证
        if (!form.name.trim()) {
            wx.showToast({ title: '请输入代金券名称', icon: 'none' })
            return
        }
        const amount = parseFloat(form.amount)
        if (isNaN(amount) || amount <= 0) {
            wx.showToast({ title: '请输入有效的优惠金额', icon: 'none' })
            return
        }
        const quantity = parseInt(form.total_quantity, 10)
        if (isNaN(quantity) || quantity <= 0) {
            wx.showToast({ title: '请输入有效的发行数量', icon: 'none' })
            return
        }
        if (!form.valid_from || !form.valid_until) {
            wx.showToast({ title: '请选择有效期', icon: 'none' })
            return
        }
        if (form.valid_until < form.valid_from) {
            wx.showToast({ title: '结束日期应晚于开始日期', icon: 'none' })
            return
        }
        if (form.allowed_order_types.length === 0) {
            wx.showToast({ title: '请至少选择一个适用场景', icon: 'none' })
            return
        }

        wx.showLoading({ title: '保存中...' })

        try {
            const minAmount = parseFloat(form.min_order_amount) || 0

            if (editingVoucher) {
                const request: UpdateVoucherRequest = {
                    name: form.name,
                    description: form.description || undefined,
                    total_quantity: quantity,
                    valid_from: form.valid_from + 'T00:00:00Z',
                    valid_until: form.valid_until + 'T23:59:59Z',
                    allowed_order_types: form.allowed_order_types
                }
                await VoucherManagementService.updateVoucher(merchantId, editingVoucher.id, request)
            } else {
                const request: CreateVoucherRequest = {
                    name: form.name,
                    amount: Math.round(amount * 100),
                    min_order_amount: Math.round(minAmount * 100),
                    total_quantity: quantity,
                    valid_from: form.valid_from + 'T00:00:00Z',
                    valid_until: form.valid_until + 'T23:59:59Z',
                    description: form.description || undefined,
                    allowed_order_types: form.allowed_order_types
                }
                await VoucherManagementService.createVoucher(merchantId, request)
            }

            wx.hideLoading()
            wx.showToast({ title: '保存成功', icon: 'success' })
            this.setData({ showModal: false })
            await this.loadVouchers()
        } catch (error) {
            wx.hideLoading()
            logger.error('保存代金券失败', error, 'vouchers')
            wx.showToast({ title: '保存失败', icon: 'error' })
        }
    },

    async onDeleteVoucher(e: WechatMiniprogram.TouchEvent) {
        const voucher = e.currentTarget.dataset.voucher as VoucherResponse

        wx.showModal({
            title: '确认删除',
            content: `确定删除代金券"${voucher.name}"？已领取但未使用的券将失效。`,
            success: async (res) => {
                if (res.confirm) {
                    wx.showLoading({ title: '删除中...' })
                    try {
                        await VoucherManagementService.deleteVoucher(this.data.merchantId, voucher.id)
                        wx.hideLoading()
                        wx.showToast({ title: '已删除', icon: 'success' })
                        await this.loadVouchers()
                    } catch (error) {
                        wx.hideLoading()
                        logger.error('删除代金券失败', error, 'vouchers')
                        wx.showToast({ title: '删除失败，可能有未使用的券', icon: 'none' })
                    }
                }
            }
        })
    },

    async onToggleVoucherStatus(e: WechatMiniprogram.TouchEvent) {
        const voucher = e.currentTarget.dataset.voucher as VoucherResponse
        const newStatus = !voucher.is_active

        try {
            await VoucherManagementService.updateVoucher(
                this.data.merchantId,
                voucher.id,
                { is_active: newStatus }
            )
            wx.showToast({ title: newStatus ? '已启用' : '已停用', icon: 'success' })
            await this.loadVouchers()
        } catch (error) {
            logger.error('更新代金券状态失败', error, 'vouchers')
            wx.showToast({ title: '操作失败', icon: 'error' })
        }
    },

    // ==================== 日历选择器 ====================

    onOpenCalendar(e: WechatMiniprogram.TouchEvent) {
        const field = e.currentTarget.dataset.field as string
        const currentValue = this.data.form[field as keyof typeof this.data.form] as string

        let year: number, month: number
        if (currentValue) {
            const parts = currentValue.split('-')
            year = parseInt(parts[0], 10)
            month = parseInt(parts[1], 10)
        } else {
            const now = new Date()
            year = now.getFullYear()
            month = now.getMonth() + 1
        }

        this.setData({
            showCalendar: true,
            calendarField: field,
            calendarYear: year,
            calendarMonth: month
        })
        this.generateCalendarDays()
    },

    onCloseCalendar() {
        this.setData({ showCalendar: false })
    },

    onCalendarContentTap() {
        // 阻止冒泡
    },

    onPrevMonth() {
        let { calendarYear, calendarMonth } = this.data
        calendarMonth--
        if (calendarMonth < 1) {
            calendarMonth = 12
            calendarYear--
        }
        this.setData({ calendarYear, calendarMonth })
        this.generateCalendarDays()
    },

    onNextMonth() {
        let { calendarYear, calendarMonth } = this.data
        calendarMonth++
        if (calendarMonth > 12) {
            calendarMonth = 1
            calendarYear++
        }
        this.setData({ calendarYear, calendarMonth })
        this.generateCalendarDays()
    },

    generateCalendarDays() {
        const { calendarYear, calendarMonth, calendarField, form } = this.data
        const selectedValue = form[calendarField as keyof typeof form] as string
        const today = this.formatDate(new Date())

        const firstDay = new Date(calendarYear, calendarMonth - 1, 1)
        const lastDay = new Date(calendarYear, calendarMonth, 0)
        const startWeekday = firstDay.getDay()
        const daysInMonth = lastDay.getDate()

        const days: CalendarDay[] = []
        const pad = (n: number) => ('0' + n).slice(-2)

        // 上月填充
        const prevMonth = new Date(calendarYear, calendarMonth - 1, 0)
        const prevDays = prevMonth.getDate()
        for (let i = startWeekday - 1; i >= 0; i--) {
            const day = prevDays - i
            const m = calendarMonth === 1 ? 12 : calendarMonth - 1
            const y = calendarMonth === 1 ? calendarYear - 1 : calendarYear
            const date = `${y}-${pad(m)}-${pad(day)}`
            days.push({ day, date, disabled: false, selected: date === selectedValue, today: date === today, currentMonth: false })
        }

        // 当月
        for (let day = 1; day <= daysInMonth; day++) {
            const date = `${calendarYear}-${pad(calendarMonth)}-${pad(day)}`
            days.push({ day, date, disabled: false, selected: date === selectedValue, today: date === today, currentMonth: true })
        }

        // 下月填充
        const remaining = 42 - days.length
        for (let day = 1; day <= remaining; day++) {
            const m = calendarMonth === 12 ? 1 : calendarMonth + 1
            const y = calendarMonth === 12 ? calendarYear + 1 : calendarYear
            const date = `${y}-${pad(m)}-${pad(day)}`
            days.push({ day, date, disabled: false, selected: date === selectedValue, today: date === today, currentMonth: false })
        }

        this.setData({ calendarDays: days })
    },

    onSelectCalendarDay(e: WechatMiniprogram.TouchEvent) {
        const date = e.currentTarget.dataset.date as string
        const field = this.data.calendarField
        this.setData({
            [`form.${field}`]: date,
            showCalendar: false
        })
    },

    onSelectToday() {
        const today = this.formatDate(new Date())
        const field = this.data.calendarField
        this.setData({
            [`form.${field}`]: today,
            showCalendar: false
        })
    },

    // ==================== 工具方法 ====================

    formatDate(date: Date): string {
        const y = date.getFullYear()
        const m = ('0' + (date.getMonth() + 1)).slice(-2)
        const d = ('0' + date.getDate()).slice(-2)
        return `${y}-${m}-${d}`
    }
})
