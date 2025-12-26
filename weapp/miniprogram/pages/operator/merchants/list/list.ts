/**
 * 运营商商户管理列表页
 * 提供商户列表查看、搜索、筛选、暂停/恢复等功能
 */

import {
    operatorMerchantManagementService,
    formatMerchantStatus,
    type OperatorMerchantItem,
    type MerchantQueryParams,
    type MerchantStatus
} from '@/api/operator-merchant-management'

Page({
    data: {
        loading: false,
        loadingMore: false,
        refreshing: false,

        // 商户列表
        merchants: [] as OperatorMerchantItem[],

        // 分页
        page: 1,
        limit: 20,
        total: 0,
        hasMore: true,

        // 搜索和筛选
        searchKeyword: '',
        statusFilter: '' as MerchantStatus | '',

        // 对话框
        suspendDialogVisible: false,
        resumeDialogVisible: false,
        selectedMerchant: { id: 0, name: '' },
        suspendReason: '',

        // 防抖定时器
        searchTimer: null as number | null
    },

    onLoad() {
        this.loadMerchants()
    },

    onPullDownRefresh() {
        this.setData({ refreshing: true, page: 1 })
        this.loadMerchants(true).finally(() => {
            this.setData({ refreshing: false })
            wx.stopPullDownRefresh()
        })
    },

    /**
     * 加载商户列表
     */
    async loadMerchants(refresh: boolean = false) {
        if (this.data.loading || this.data.loadingMore) return

        try {
            if (refresh) {
                this.setData({ loading: true, page: 1 })
            } else {
                this.setData({ loadingMore: true })
            }

            const params: MerchantQueryParams = {
                page: this.data.page,
                limit: this.data.limit,
                keyword: this.data.searchKeyword || undefined,
                status: this.data.statusFilter || undefined,
                sort_by: 'created_at',
                sort_order: 'desc'
            }

            const result = await operatorMerchantManagementService.getMerchantList(params)

            const merchants = refresh ? result.merchants : [...this.data.merchants, ...result.merchants]

            this.setData({
                merchants,
                total: result.total,
                hasMore: result.has_more,
                page: this.data.page + 1
            })
        } catch (error) {
            console.error('加载商户列表失败:', error)
            wx.showToast({
                title: '加载失败',
                icon: 'none'
            })
        } finally {
            this.setData({
                loading: false,
                loadingMore: false
            })
        }
    },

    /**
     * 加载更多
     */
    onLoadMore() {
        if (this.data.hasMore && !this.data.loading && !this.data.loadingMore) {
            this.loadMerchants()
        }
    },

    /**
     * 搜索变化
     */
    onSearchChange(e: any) {
        const keyword = e.detail.value
        this.setData({ searchKeyword: keyword })

        // 防抖搜索
        if (this.data.searchTimer) {
            clearTimeout(this.data.searchTimer)
        }

        const timer = setTimeout(() => {
            this.setData({ page: 1 })
            this.loadMerchants(true)
        }, 500)

        this.setData({ searchTimer: timer as any })
    },

    /**
     * 清空搜索
     */
    onSearchClear() {
        this.setData({ searchKeyword: '', page: 1 })
        this.loadMerchants(true)
    },

    /**
     * 状态筛选变化
     */
    onStatusFilterChange(e: any) {
        this.setData({
            statusFilter: e.detail.value,
            page: 1
        })
        this.loadMerchants(true)
    },

    /**
     * 点击商户卡片
     */
    onMerchantTap(e: any) {
        const { id } = e.currentTarget.dataset
        wx.navigateTo({
            url: `/pages/operator/merchants/detail/detail?id=${id}`
        })
    },

    /**
     * 暂停商户
     */
    onSuspendTap(e: any) {
        const { id, name } = e.currentTarget.dataset
        this.setData({
            selectedMerchant: { id, name },
            suspendDialogVisible: true,
            suspendReason: ''
        })
    },

    /**
     * 确认暂停
     */
    async onSuspendConfirm() {
        const { selectedMerchant, suspendReason } = this.data

        if (!suspendReason.trim()) {
            wx.showToast({
                title: '请输入暂停原因',
                icon: 'none'
            })
            return
        }

        try {
            wx.showLoading({ title: '处理中...' })

            await operatorMerchantManagementService.suspendMerchant(selectedMerchant.id, {
                reason: suspendReason
            })

            wx.showToast({
                title: '暂停成功',
                icon: 'success'
            })

            this.setData({
                suspendDialogVisible: false,
                page: 1
            })
            this.loadMerchants(true)
        } catch (error) {
            console.error('暂停商户失败:', error)
            wx.showToast({
                title: '操作失败',
                icon: 'none'
            })
        } finally {
            wx.hideLoading()
        }
    },

    /**
     * 取消暂停
     */
    onSuspendCancel() {
        this.setData({ suspendDialogVisible: false })
    },

    /**
     * 恢复商户
     */
    onResumeTap(e: any) {
        const { id, name } = e.currentTarget.dataset
        this.setData({
            selectedMerchant: { id, name },
            resumeDialogVisible: true
        })
    },

    /**
     * 确认恢复
     */
    async onResumeConfirm() {
        const { selectedMerchant } = this.data

        try {
            wx.showLoading({ title: '处理中...' })

            await operatorMerchantManagementService.resumeMerchant(selectedMerchant.id, {
                reason: '运营商恢复'
            })

            wx.showToast({
                title: '恢复成功',
                icon: 'success'
            })

            this.setData({
                resumeDialogVisible: false,
                page: 1
            })
            this.loadMerchants(true)
        } catch (error) {
            console.error('恢复商户失败:', error)
            wx.showToast({
                title: '操作失败',
                icon: 'none'
            })
        } finally {
            wx.hideLoading()
        }
    },

    /**
     * 取消恢复
     */
    onResumeCancel() {
        this.setData({ resumeDialogVisible: false })
    },

    /**
     * 阻止事件冒泡
     */
    stopPropagation() {
        // 阻止事件冒泡
    },

    /**
     * 格式化商户状态
     */
    formatStatus(status: MerchantStatus): string {
        return formatMerchantStatus(status)
    },

    /**
     * 格式化金额
     */
    formatAmount(amount: number): string {
        return `¥${(amount / 100).toFixed(2)}`
    }
})
