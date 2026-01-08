/**
 * 会员管理页面
 * 使用新的 /merchants/:id/members API 展示会员余额和交易记录
 */

import { request } from '@/utils/request'
import { formatPriceNoSymbol } from '@/utils/util'

// 会员响应类型
interface MemberResponse {
    user_id: number
    full_name: string
    phone: string
    avatar_url: string
    membership_id: number
    balance: number
    total_recharged: number
    total_consumed: number
    created_at: string
}

// 交易记录类型
interface TransactionResponse {
    id: number
    membership_id: number
    type: string
    amount: number
    balance_after: number
    related_order_id?: number
    notes?: string
    created_at: string
}

// 会员详情响应类型
interface MemberDetailResponse extends MemberResponse {
    transactions: TransactionResponse[]
}

// 会员管理服务
const MemberService = {
    // 获取会员列表
    async listMembers(merchantId: number, pageId: number, pageSize: number): Promise<MemberResponse[]> {
        return request<MemberResponse[]>({
            url: `/v1/merchants/${merchantId}/members`,
            method: 'GET',
            data: { page_id: pageId, page_size: pageSize }
        })
    },

    // 获取会员详情
    async getMemberDetail(merchantId: number, userId: number): Promise<MemberDetailResponse> {
        return request<MemberDetailResponse>({
            url: `/v1/merchants/${merchantId}/members/${userId}`,
            method: 'GET'
        })
    },

    // 调整余额
    async adjustBalance(merchantId: number, userId: number, amount: number, notes: string): Promise<MemberResponse> {
        return request<MemberResponse>({
            url: `/v1/merchants/${merchantId}/members/${userId}/balance`,
            method: 'POST',
            data: { amount, notes }
        })
    }
}

Page({
    data: {
        merchantId: 0,
        sidebarCollapsed: false,

        // 会员列表
        members: [] as MemberResponse[],
        loading: true,
        pageId: 1,
        pageSize: 20,
        hasMore: true,

        // 选中的会员
        selectedMember: null as MemberDetailResponse | null,
        showDetailModal: false,
        detailLoading: false,

        // 余额调整
        showAdjustModal: false,
        adjustForm: {
            amount: '',
            notes: '',
            type: 'add' as 'add' | 'deduct'
        },
        adjusting: false
    },

    onLoad() {
        this.initData()
    },

    async initData() {
        const app = getApp<IAppOption>()
        const merchantId = app.globalData.merchantId

        if (merchantId) {
            this.setData({ merchantId: Number(merchantId) })
            await this.loadMembers()
        } else {
            app.userInfoReadyCallback = async () => {
                if (app.globalData.merchantId) {
                    this.setData({ merchantId: Number(app.globalData.merchantId) })
                    await this.loadMembers()
                }
            }
        }
    },

    onSidebarCollapse(e: any) {
        this.setData({ sidebarCollapsed: e.detail.collapsed })
    },

    // 加载会员列表
    async loadMembers(loadMore = false) {
        const { merchantId, pageId, pageSize, members } = this.data

        if (!loadMore) {
            this.setData({ loading: true, pageId: 1, members: [] })
        }

        try {
            const result = await MemberService.listMembers(merchantId, loadMore ? pageId : 1, pageSize)
            // 预处理价格
            const processedMembers = result.map(m => ({
                ...m,
                balance_display: formatPriceNoSymbol(m.balance || 0),
                total_recharged_display: formatPriceNoSymbol(m.total_recharged || 0),
                total_consumed_display: formatPriceNoSymbol(m.total_consumed || 0),
                created_date: m.created_at ? m.created_at.slice(0, 10) : '-'
            }))
            this.setData({
                members: loadMore ? [...members, ...processedMembers] : processedMembers,
                hasMore: result.length === pageSize,
                pageId: loadMore ? pageId + 1 : 2,
                loading: false
            })
        } catch (error: any) {
            console.error('加载会员列表失败:', error)
            wx.showToast({ title: error.message || '加载失败', icon: 'none' })
            this.setData({ loading: false })
        }
    },

    // 加载更多
    onLoadMore() {
        if (this.data.hasMore && !this.data.loading) {
            this.loadMembers(true)
        }
    },

    // 查看会员详情
    async onViewMember(e: any) {
        const userId = e.currentTarget.dataset.userId
        const { merchantId } = this.data

        this.setData({ showDetailModal: true, detailLoading: true, selectedMember: null })

        try {
            const detail = await MemberService.getMemberDetail(merchantId, userId)
            // 预处理详情价格
            const processedDetail = {
                ...detail,
                balance_display: formatPriceNoSymbol(detail.balance || 0),
                total_recharged_display: formatPriceNoSymbol(detail.total_recharged || 0),
                total_consumed_display: formatPriceNoSymbol(detail.total_consumed || 0),
                transactions: (detail.transactions || []).map(tx => ({
                    ...tx,
                    amount_display: formatPriceNoSymbol(Math.abs(tx.amount || 0)),
                    amount_sign: tx.amount >= 0 ? '+' : '-',
                    created_date: tx.created_at ? tx.created_at.slice(0, 10) : '-',
                    type_display: this.formatTxType(tx.type)
                }))
            }
            this.setData({ selectedMember: processedDetail, detailLoading: false })
        } catch (error: any) {
            console.error('加载会员详情失败:', error)
            wx.showToast({ title: error.message || '加载失败', icon: 'none' })
            this.setData({ detailLoading: false })
        }
    },

    // 关闭详情弹窗
    onCloseDetail() {
        this.setData({ showDetailModal: false, selectedMember: null })
    },

    // 打开余额调整弹窗
    onOpenAdjust(e: any) {
        const userId = e.currentTarget.dataset.userId
        // 先关闭详情弹窗
        this.setData({
            showDetailModal: false,
            showAdjustModal: true,
            adjustForm: { amount: '', notes: '', type: 'add' }
        })
    },

    // 关闭余额调整弹窗
    onCloseAdjust() {
        this.setData({ showAdjustModal: false })
    },

    // 调整类型切换
    onAdjustTypeChange(e: any) {
        const type = e.currentTarget.dataset.type
        this.setData({ 'adjustForm.type': type })
    },

    // 输入金额
    onAmountInput(e: any) {
        this.setData({ 'adjustForm.amount': e.detail.value })
    },

    // 输入备注
    onNotesInput(e: any) {
        this.setData({ 'adjustForm.notes': e.detail.value })
    },

    // 提交余额调整
    async onSubmitAdjust() {
        const { merchantId, selectedMember, adjustForm } = this.data

        if (!selectedMember) return

        const amountYuan = parseFloat(adjustForm.amount)
        if (isNaN(amountYuan) || amountYuan <= 0) {
            wx.showToast({ title: '请输入有效金额', icon: 'none' })
            return
        }
        if (!adjustForm.notes.trim()) {
            wx.showToast({ title: '请输入调整原因', icon: 'none' })
            return
        }

        const amountFen = Math.round(amountYuan * 100)
        const finalAmount = adjustForm.type === 'add' ? amountFen : -amountFen

        this.setData({ adjusting: true })

        try {
            await MemberService.adjustBalance(merchantId, selectedMember.user_id, finalAmount, adjustForm.notes)
            wx.showToast({ title: '调整成功', icon: 'success' })
            this.setData({ showAdjustModal: false })
            this.loadMembers() // 刷新列表
        } catch (error: any) {
            console.error('余额调整失败:', error)
            wx.showToast({ title: error.message || '调整失败', icon: 'none' })
        } finally {
            this.setData({ adjusting: false })
        }
    },

    // 格式化金额 - 使用统一的 formatPriceNoSymbol
    formatAmount(fen: number): string {
        return formatPriceNoSymbol(fen)
    },

    // 格式化日期
    formatDate(dateStr: string): string {
        if (!dateStr) return '-'
        return dateStr.slice(0, 10)
    },

    // 格式化交易类型
    formatTxType(type: string): string {
        const map: Record<string, string> = {
            'recharge': '充值',
            'consume': '消费',
            'refund': '退款',
            'adjustment_credit': '余额增加',
            'adjustment_debit': '余额扣减'
        }
        return map[type] || type
    }
})
