/**
 * 运费减免设置页面
 * 商户管理配送费优惠规则
 */

import { deliveryFeeService, DeliveryFeeAdapter } from '../../../api/delivery-fee'

const app = getApp<IAppOption>()

interface PromotionDisplay {
    id: number
    name: string
    min_order_amount: number
    min_order_amount_display: string
    discount_amount: number
    discount_display: string
    valid_from: string
    valid_until: string
    valid_period: string
    is_active: boolean
    typeText: string
}

Page({
    data: {
        loading: false,
        submitting: false,
        promotions: [] as PromotionDisplay[],
        showModal: false,
        formData: {
            name: '',
            min_order_amount: '',
            discount_amount: '',
            valid_from: '',
            valid_until: ''
        }
    },

    onLoad() {
        this.loadPromotions()
    },

    onShow() {
        if (this.data.promotions.length > 0) {
            this.loadPromotions()
        }
    },

    async loadPromotions() {
        this.setData({ loading: true })

        try {
            const merchantId = Number(app.globalData.merchantId)
            if (!merchantId) {
                wx.showToast({ title: '请先登录商户', icon: 'none' })
                this.setData({ loading: false })
                return
            }

            const result = await deliveryFeeService.getMerchantPromotions(merchantId)

            // 格式化显示数据
            const promotions: PromotionDisplay[] = (result || []).map((item: any) => ({
                id: item.id,
                name: item.name,
                min_order_amount: item.min_order_amount,
                min_order_amount_display: DeliveryFeeAdapter.formatFee(item.min_order_amount),
                discount_amount: item.discount_amount,
                discount_display: DeliveryFeeAdapter.formatFee(item.discount_amount),
                valid_from: item.valid_from?.split('T')[0] || '',
                valid_until: item.valid_until?.split('T')[0] || '',
                valid_period: `${item.valid_from?.split('T')[0] || ''} ~ ${item.valid_until?.split('T')[0] || ''}`,
                is_active: item.is_active,
                typeText: item.discount_amount === 0 ? '免运费' : '运费减免'
            }))

            this.setData({ promotions, loading: false })
        } catch (error) {
            console.error('加载运费减免规则失败:', error)
            wx.showToast({ title: '加载失败', icon: 'error' })
            this.setData({ loading: false })
        }
    },

    showAddModal() {
        // 默认日期：今天到一个月后
        const today = new Date()
        const nextMonth = new Date(today.getTime() + 30 * 24 * 60 * 60 * 1000)

        this.setData({
            showModal: true,
            formData: {
                name: '',
                min_order_amount: '',
                discount_amount: '0',
                valid_from: this.formatDate(today),
                valid_until: this.formatDate(nextMonth)
            }
        })
    },

    hideModal() {
        this.setData({ showModal: false })
    },

    formatDate(date: Date): string {
        const y = date.getFullYear()
        const m = date.getMonth() + 1
        const d = date.getDate()
        return `${y}-${m < 10 ? '0' + m : m}-${d < 10 ? '0' + d : d}`
    },

    onInputChange(e: any) {
        const field = e.currentTarget.dataset.field
        const value = e.detail.value
        this.setData({
            [`formData.${field}`]: value
        })
    },

    onDateChange(e: any) {
        const field = e.currentTarget.dataset.field
        const value = e.detail.value
        this.setData({
            [`formData.${field}`]: value
        })
    },

    async createPromotion() {
        const { formData } = this.data

        // 验证
        if (!formData.name.trim()) {
            wx.showToast({ title: '请输入规则名称', icon: 'none' })
            return
        }
        if (!formData.min_order_amount || Number(formData.min_order_amount) <= 0) {
            wx.showToast({ title: '请输入起订金额', icon: 'none' })
            return
        }
        if (!formData.valid_from || !formData.valid_until) {
            wx.showToast({ title: '请选择有效期', icon: 'none' })
            return
        }

        this.setData({ submitting: true })

        try {
            const merchantId = Number(app.globalData.merchantId)

            await deliveryFeeService.createMerchantPromotion(merchantId, {
                name: formData.name.trim(),
                promotion_type: Number(formData.discount_amount) === 0 ? 'free_shipping' : 'fixed_amount',
                min_order_amount: Math.round(Number(formData.min_order_amount) * 100), // 转分
                discount_value: Math.round(Number(formData.discount_amount) * 100), // 转分
                start_time: formData.valid_from + 'T00:00:00Z',
                end_time: formData.valid_until + 'T23:59:59Z',
                is_active: true
            })

            wx.showToast({ title: '创建成功', icon: 'success' })
            this.hideModal()
            this.loadPromotions()
        } catch (error: any) {
            console.error('创建失败:', error)
            wx.showToast({ title: error.message || '创建失败', icon: 'none' })
        } finally {
            this.setData({ submitting: false })
        }
    },

    async deletePromotion(e: any) {
        const promoId = e.currentTarget.dataset.id

        wx.showModal({
            title: '确认删除',
            content: '确定要删除这条运费减免规则吗？',
            success: async (res) => {
                if (res.confirm) {
                    try {
                        const merchantId = Number(app.globalData.merchantId)
                        await deliveryFeeService.deleteMerchantPromotion(merchantId, promoId)
                        wx.showToast({ title: '已删除', icon: 'success' })
                        this.loadPromotions()
                    } catch (error) {
                        console.error('删除失败:', error)
                        wx.showToast({ title: '删除失败', icon: 'error' })
                    }
                }
            }
        })
    }
})
