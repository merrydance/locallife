/**
 * KDS 厨房显示系统 - 全屏沉浸式界面
 * 通过 WebSocket 实时接收订单推送，支持三栏看板布局
 */

import {
    KitchenDisplayService,
    type KitchenOrdersResponse,
    type KitchenOrderResponse,
    type KitchenStats
} from '@/api/order-management'
import { RealtimeUtils, WebSocketUtils } from '@/api/websocket-realtime'
import { logger } from '@/utils/logger'

// 订单类型映射
const ORDER_TYPE_MAP: Record<string, string> = {
    'takeout': '外卖',
    'dine_in': '堂食',
    'takeaway': '自取'
}

// 格式化时间 HH:mm
function formatTime(dateStr: string): string {
    const d = new Date(dateStr)
    return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

Page({
    data: {
        // 订单数据
        newOrders: [] as KitchenOrderResponse[],
        preparingOrders: [] as KitchenOrderResponse[],
        readyOrders: [] as KitchenOrderResponse[],

        // 统计
        stats: {
            new_count: 0,
            preparing_count: 0,
            ready_count: 0,
            completed_today_count: 0,
            avg_prepare_time: 15
        } as KitchenStats,

        // 状态
        loading: true,
        connected: false,
        currentTime: '',

        // 设置
        autoRefresh: true,
        voiceEnabled: true,
        refreshInterval: 10000,

        // 弹窗
        showOrderDetail: false,
        selectedOrder: null as KitchenOrderResponse | null
    },

    _refreshTimer: null as any,
    _clockTimer: null as any,
    _audioCtx: null as WechatMiniprogram.InnerAudioContext | null,

    onLoad() {
        this.initClock()
        this.loadOrders()
        this.connectWebSocket()
    },

    onShow() {
        // WebSocket 连接后使用实时推送，仅作为备用的轮询
        if (!this.data.connected && this.data.autoRefresh) {
            this.startAutoRefresh()
        }
    },

    onHide() {
        this.stopAutoRefresh()
    },

    onUnload() {
        this.stopAutoRefresh()
        this.stopClock()
        this.disconnectWebSocket()
        if (this._audioCtx) {
            this._audioCtx.destroy()
        }
    },

    // 初始化时钟
    initClock() {
        this.updateClock()
        this._clockTimer = setInterval(() => this.updateClock(), 1000)
    },

    updateClock() {
        const now = new Date()
        const h = String(now.getHours()).padStart(2, '0')
        const m = String(now.getMinutes()).padStart(2, '0')
        const s = String(now.getSeconds()).padStart(2, '0')
        this.setData({ currentTime: `${h}:${m}:${s}` })
    },

    stopClock() {
        if (this._clockTimer) {
            clearInterval(this._clockTimer)
            this._clockTimer = null
        }
    },

    // 加载订单
    async loadOrders() {
        try {
            if (!this.data.loading) {
                // 静默刷新，不显示 loading
            } else {
                this.setData({ loading: true })
            }

            const result = await KitchenDisplayService.getKitchenOrders()

            // 处理订单数据，添加显示用字段
            const processOrders = (orders: KitchenOrderResponse[]) => {
                return (orders || []).map(order => ({
                    ...order,
                    order_type_text: ORDER_TYPE_MAP[order.order_type] || order.order_type,
                    created_time: formatTime(order.created_at),
                    paid_time: order.paid_at ? formatTime(order.paid_at) : ''
                }))
            }

            const prevNewCount = this.data.stats.new_count || 0
            const newCount = result.new_orders?.length || 0

            this.setData({
                newOrders: processOrders(result.new_orders),
                preparingOrders: processOrders(result.preparing_orders),
                readyOrders: processOrders(result.ready_orders),
                stats: result.stats || this.data.stats,
                loading: false
            })

            // 检查新订单提醒
            if (newCount > prevNewCount && this.data.voiceEnabled) {
                this.playAlert()
            }

        } catch (error) {
            logger.error('加载订单失败', error, 'KDS')
            this.setData({ loading: false })
            wx.showToast({ title: '加载失败', icon: 'none' })
        }
    },

    // WebSocket 连接
    async connectWebSocket() {
        try {
            // 获取用户和商户信息
            const app = getApp()
            const userId = app.globalData?.userId
            const merchantId = app.globalData?.merchantId

            if (!userId || !merchantId) {
                logger.warn('无法获取用户/商户ID，使用轮询模式', null, 'KDS')
                this.startAutoRefresh()
                return
            }

            // 使用现有的 WebSocket 服务初始化商户连接
            await RealtimeUtils.initializeForMerchant(userId, merchantId, {
                onOpen: () => {
                    logger.info('KDS WebSocket 已连接', null, 'KDS')
                    this.setData({ connected: true })
                    // 连接成功后停止轮询，使用实时推送
                    this.stopAutoRefresh()
                },
                onOrderUpdate: (orderData: any) => {
                    logger.info('收到订单更新', orderData, 'KDS')
                    // 收到订单更新时刷新订单列表
                    this.loadOrders()
                },
                onNotification: (notification: any) => {
                    // 处理新订单通知
                    if (notification.type === 'new_order') {
                        logger.info('收到新订单通知', notification, 'KDS')
                        this.loadOrders()
                        if (this.data.voiceEnabled) {
                            this.playAlert()
                        }
                    }
                },
                onMessage: (message: any) => {
                    // 处理所有消息，检查是否是订单相关
                    if (message.type === 'order_update' || message.type === 'new_order') {
                        this.loadOrders()
                    }
                }
            })

            this.setData({ connected: WebSocketUtils.isConnected() })

        } catch (error) {
            logger.error('WebSocket 连接失败，使用轮询模式', error, 'KDS')
            this.setData({ connected: false })
            this.startAutoRefresh()
        }
    },

    // 断开 WebSocket
    disconnectWebSocket() {
        // 注意：不调用 closeAll，因为其他页面可能也在使用
        // WebSocket 连接由全局管理
    },

    // 自动刷新
    startAutoRefresh() {
        if (!this.data.autoRefresh) return
        this.stopAutoRefresh()
        this._refreshTimer = setInterval(() => {
            this.loadOrders()
        }, this.data.refreshInterval)
    },

    stopAutoRefresh() {
        if (this._refreshTimer) {
            clearInterval(this._refreshTimer)
            this._refreshTimer = null
        }
    },

    // 手动刷新
    onRefresh() {
        this.loadOrders()
    },

    // 切换自动刷新
    onToggleAutoRefresh() {
        const autoRefresh = !this.data.autoRefresh
        this.setData({ autoRefresh })
        if (autoRefresh) {
            this.startAutoRefresh()
        } else {
            this.stopAutoRefresh()
        }
        wx.showToast({
            title: autoRefresh ? '自动刷新已开' : '自动刷新已关',
            icon: 'none'
        })
    },

    // 切换语音
    onToggleVoice() {
        const voiceEnabled = !this.data.voiceEnabled
        this.setData({ voiceEnabled })
        wx.showToast({
            title: voiceEnabled ? '语音已开' : '语音已关',
            icon: 'none'
        })
    },

    // 新订单提醒
    checkNewOrderAlert(newCount: number) {
        const prevCount = this.data.stats.new_count || 0
        if (newCount > prevCount && this.data.voiceEnabled) {
            this.playAlert()
        }
    },

    playAlert() {
        // 播放提示音
        if (!this._audioCtx) {
            this._audioCtx = wx.createInnerAudioContext()
            this._audioCtx.src = '/assets/audio/new_order.mp3'
        }
        this._audioCtx.play()
    },

    // 开始制作
    async onStartPreparing(e: WechatMiniprogram.TouchEvent) {
        const orderId = e.currentTarget.dataset.id as number
        if (!orderId) return

        try {
            wx.showLoading({ title: '处理中' })
            await KitchenDisplayService.startPreparing(orderId)
            wx.hideLoading()
            wx.showToast({ title: '已开始制作', icon: 'success' })
            this.loadOrders()
        } catch (error) {
            wx.hideLoading()
            logger.error('开始制作失败', error, 'KDS')
            wx.showToast({ title: '操作失败', icon: 'none' })
        }
    },

    // 制作完成
    async onMarkReady(e: WechatMiniprogram.TouchEvent) {
        const orderId = e.currentTarget.dataset.id as number
        if (!orderId) return

        try {
            wx.showLoading({ title: '处理中' })
            await KitchenDisplayService.markKitchenOrderReady(orderId)
            wx.hideLoading()
            wx.showToast({ title: '已完成', icon: 'success' })
            this.loadOrders()
        } catch (error) {
            wx.hideLoading()
            logger.error('标记完成失败', error, 'KDS')
            wx.showToast({ title: '操作失败', icon: 'none' })
        }
    },

    // 查看订单详情
    onViewOrder(e: WechatMiniprogram.TouchEvent) {
        const order = e.currentTarget.dataset.order as KitchenOrderResponse
        if (order) {
            this.setData({
                selectedOrder: order,
                showOrderDetail: true
            })
        }
    },

    // 关闭详情
    onCloseDetail() {
        this.setData({
            showOrderDetail: false,
            selectedOrder: null
        })
    },

    // 阻止冒泡
    stopPropagation() { },

    // 退出 KDS
    onExit() {
        wx.showModal({
            title: '退出确认',
            content: '确定退出厨房显示系统吗？',
            success: (res) => {
                if (res.confirm) {
                    wx.navigateBack({
                        fail: () => {
                            wx.redirectTo({ url: '/pages/merchant/dashboard/index' })
                        }
                    })
                }
            }
        })
    }
})
