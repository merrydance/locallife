/**
 * 商户预订管理页面
 * PC-SaaS 风格，支持完整的预订管理流程
 */

import { isLargeScreen } from '@/utils/responsive'
import { logger } from '@/utils/logger'
import {
  ReservationService,
  type ReservationResponse,
  type ReservationStats,
  type MerchantCreateReservationRequest
} from '@/api/reservation'
import { getPrivateRooms } from '@/api/table-device-management'

interface TableOption {
  id: number
  table_no: string
  table_type: string
  capacity: number
}

Page({
  data: {
    // 布局
    isLargeScreen: false,
    navBarHeight: 88,

    // 状态
    loading: false,
    activeTab: 'today' as 'today' | 'all' | 'pending' | 'confirmed' | 'completed',

    // 数据
    reservations: [] as ReservationResponse[],
    stats: null as ReservationStats | null,
    tables: [] as TableOption[],

    // 筛选
    filterDate: '',
    filterStatus: '',

    // 创建预订弹窗
    showCreateModal: false,
    createForm: {
      table_id: 0,
      date: '',
      time: '',
      guest_count: 2,
      contact_name: '',
      contact_phone: '',
      source: 'phone' as 'phone' | 'walkin' | 'merchant',
      notes: ''
    },
    selectedTableName: '', // 已选桌台名称

    // 详情弹窗
    showDetailModal: false,
    selectedReservation: null as ReservationResponse | null
  },

  onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.loadStats()
    this.loadReservations()
    this.loadTables()
  },

  onShow() {
    if (this.data.reservations.length > 0) {
      this.loadReservations()
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  // ==================== 数据加载 ====================

  async loadStats() {
    try {
      const stats = await ReservationService.getReservationStats()
      this.setData({ stats })
    } catch (error) {
      logger.error('加载预订统计失败', error, 'reservations')
    }
  },

  async loadReservations() {
    const { activeTab, filterDate, filterStatus } = this.data
    this.setData({ loading: true })

    try {
      let reservations: ReservationResponse[] = []

      if (activeTab === 'today') {
        const result = await ReservationService.getTodayReservations()
        reservations = result.reservations || []
      } else {
        const params: any = { page_id: 1, page_size: 50 }
        if (filterDate) params.date = filterDate
        if (activeTab !== 'all') params.status = activeTab
        if (filterStatus) params.status = filterStatus

        const result = await ReservationService.getMerchantReservations(params)
        reservations = result.reservations || []
      }

      this.setData({ reservations, loading: false })
    } catch (error) {
      logger.error('加载预订列表失败', error, 'reservations')
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },



  async loadTables() {
    try {
      const tables = await getPrivateRooms()
      this.setData({ tables })
    } catch (error) {
      logger.error('加载桌台列表失败', error, 'reservations')
    }
  },

  // ==================== Tab 切换 ====================

  onTabChange(e: WechatMiniprogram.TouchEvent) {
    const tab = e.currentTarget.dataset.tab as string
    this.setData({ activeTab: tab as any })
    this.loadReservations()
  },

  // ==================== 筛选 ====================

  onDateChange(e: WechatMiniprogram.PickerChange) {
    this.setData({ filterDate: e.detail.value as string })
    this.loadReservations()
  },

  onClearDate() {
    this.setData({ filterDate: '' })
    this.loadReservations()
  },

  // ==================== 创建预订 ====================

  onShowCreateModal() {
    const today = new Date()
    const dateStr = today.toISOString().split('T')[0]
    const timeStr = '12:00'

    this.setData({
      showCreateModal: true,
      selectedTableName: '',
      createForm: {
        table_id: 0,
        date: dateStr,
        time: timeStr,
        guest_count: 2,
        contact_name: '',
        contact_phone: '',
        source: 'phone',
        notes: ''
      }
    })
  },

  onCloseCreateModal() {
    this.setData({ showCreateModal: false })
  },

  // 空函数，用于阻止事件冒泡
  onModalContentTap() {
    // 不执行任何操作，仅阻止冒泡
  },

  onFormInput(e: WechatMiniprogram.Input) {
    const field = e.currentTarget.dataset.field as string
    const value = e.detail.value
    this.setData({
      [`createForm.${field}`]: value
    })
  },

  onTableOptionTap(e: WechatMiniprogram.TouchEvent) {
    const table = e.currentTarget.dataset.table as TableOption
    if (table) {
      this.setData({
        'createForm.table_id': table.id,
        selectedTableName: table.table_no
      })
    }
  },

  onDateSelect(e: WechatMiniprogram.PickerChange) {
    this.setData({ 'createForm.date': e.detail.value as string })
  },

  onTimeSelect(e: WechatMiniprogram.PickerChange) {
    this.setData({ 'createForm.time': e.detail.value as string })
  },

  onSourceTap(e: WechatMiniprogram.TouchEvent) {
    const source = e.currentTarget.dataset.source as 'phone' | 'walkin' | 'merchant'
    this.setData({ 'createForm.source': source })
  },

  onGuestCountChange(e: WechatMiniprogram.Input) {
    this.setData({ 'createForm.guest_count': Number(e.detail.value) || 2 })
  },

  async onSubmitCreate() {
    const { createForm } = this.data

    if (!createForm.table_id) {
      wx.showToast({ title: '请选择包间', icon: 'none' })
      return
    }
    if (!createForm.contact_name) {
      wx.showToast({ title: '请输入联系人', icon: 'none' })
      return
    }
    if (!createForm.contact_phone) {
      wx.showToast({ title: '请输入联系电话', icon: 'none' })
      return
    }

    wx.showLoading({ title: '创建中...' })

    try {
      await ReservationService.merchantCreateReservation(createForm as MerchantCreateReservationRequest)
      wx.hideLoading()
      wx.showToast({ title: '创建成功', icon: 'success' })
      this.setData({ showCreateModal: false })
      this.loadReservations()
      this.loadStats()
    } catch (error) {
      wx.hideLoading()
      logger.error('创建预订失败', error, 'reservations')
      wx.showToast({ title: '创建失败', icon: 'error' })
    }
  },

  // ==================== 预订详情 ====================

  onViewDetail(e: WechatMiniprogram.TouchEvent) {
    const id = e.currentTarget.dataset.id as number
    const reservation = this.data.reservations.find(r => r.id === id)
    if (reservation) {
      this.setData({
        showDetailModal: true,
        selectedReservation: reservation
      })
    }
  },

  onCloseDetailModal() {
    this.setData({ showDetailModal: false, selectedReservation: null })
  },

  // ==================== 预订操作 ====================

  async onConfirmReservation(e: WechatMiniprogram.TouchEvent) {
    const id = e.currentTarget.dataset.id as number

    wx.showModal({
      title: '确认预订',
      content: '确定要确认此预订吗？',
      success: async (res) => {
        if (res.confirm) {
          wx.showLoading({ title: '处理中...' })
          try {
            await ReservationService.confirmReservation(id)
            wx.hideLoading()
            wx.showToast({ title: '已确认', icon: 'success' })
            this.loadReservations()
            this.loadStats()
          } catch (error) {
            wx.hideLoading()
            wx.showToast({ title: '操作失败', icon: 'error' })
          }
        }
      }
    })
  },

  async onCheckIn(e: WechatMiniprogram.TouchEvent) {
    const id = e.currentTarget.dataset.id as number

    wx.showModal({
      title: '到店签到',
      content: '确定顾客已到店吗？',
      success: async (res) => {
        if (res.confirm) {
          wx.showLoading({ title: '处理中...' })
          try {
            await ReservationService.checkIn(id)
            wx.hideLoading()
            wx.showToast({ title: '已签到', icon: 'success' })
            this.loadReservations()
            this.loadStats()
          } catch (error) {
            wx.hideLoading()
            wx.showToast({ title: '操作失败', icon: 'error' })
          }
        }
      }
    })
  },

  async onStartCooking(e: WechatMiniprogram.TouchEvent) {
    const id = e.currentTarget.dataset.id as number

    wx.showModal({
      title: '起菜通知',
      content: '确定通知厨房开始制作吗？',
      success: async (res) => {
        if (res.confirm) {
          wx.showLoading({ title: '处理中...' })
          try {
            await ReservationService.startCooking(id)
            wx.hideLoading()
            wx.showToast({ title: '已通知厨房', icon: 'success' })
            this.loadReservations()
          } catch (error) {
            wx.hideLoading()
            wx.showToast({ title: '操作失败', icon: 'error' })
          }
        }
      }
    })
  },

  async onCompleteReservation(e: WechatMiniprogram.TouchEvent) {
    const id = e.currentTarget.dataset.id as number

    wx.showModal({
      title: '完成预订',
      content: '确定顾客已离店，完成此预订吗？',
      success: async (res) => {
        if (res.confirm) {
          wx.showLoading({ title: '处理中...' })
          try {
            await ReservationService.completeReservation(id)
            wx.hideLoading()
            wx.showToast({ title: '已完成', icon: 'success' })
            this.loadReservations()
            this.loadStats()
          } catch (error) {
            wx.hideLoading()
            wx.showToast({ title: '操作失败', icon: 'error' })
          }
        }
      }
    })
  },

  async onMarkNoShow(e: WechatMiniprogram.TouchEvent) {
    const id = e.currentTarget.dataset.id as number

    wx.showModal({
      title: '标记未到店',
      content: '确定标记此预订为未到店吗？定金将被没收。',
      success: async (res) => {
        if (res.confirm) {
          wx.showLoading({ title: '处理中...' })
          try {
            await ReservationService.markNoShow(id)
            wx.hideLoading()
            wx.showToast({ title: '已标记', icon: 'success' })
            this.loadReservations()
            this.loadStats()
          } catch (error) {
            wx.hideLoading()
            wx.showToast({ title: '操作失败', icon: 'error' })
          }
        }
      }
    })
  },

  async onCancelReservation(e: WechatMiniprogram.TouchEvent) {
    const id = e.currentTarget.dataset.id as number

    wx.showModal({
      title: '取消预订',
      content: '确定要取消此预订吗？已支付的将自动退款。',
      success: async (res) => {
        if (res.confirm) {
          wx.showLoading({ title: '处理中...' })
          try {
            await ReservationService.cancelReservation(id, '商户取消')
            wx.hideLoading()
            wx.showToast({ title: '已取消', icon: 'success' })
            this.loadReservations()
            this.loadStats()
          } catch (error) {
            wx.hideLoading()
            wx.showToast({ title: '操作失败', icon: 'error' })
          }
        }
      }
    })
  },

  // ==================== 辅助方法 ====================

  getStatusText(status: string): string {
    const map: Record<string, string> = {
      pending: '待支付',
      paid: '已支付',
      confirmed: '已确认',
      checked_in: '已到店',
      completed: '已完成',
      cancelled: '已取消',
      expired: '已过期',
      no_show: '未到店'
    }
    return map[status] || status
  },

  getStatusClass(status: string): string {
    const map: Record<string, string> = {
      pending: 'status-warning',
      paid: 'status-info',
      confirmed: 'status-primary',
      checked_in: 'status-success',
      completed: 'status-default',
      cancelled: 'status-error',
      expired: 'status-default',
      no_show: 'status-error'
    }
    return map[status] || ''
  },

  getSourceText(source?: string): string {
    const map: Record<string, string> = {
      online: '线上预订',
      phone: '电话预订',
      walkin: '现场预订',
      merchant: '商户代订'
    }
    return source ? (map[source] || source) : ''
  },

  formatAmount(amount: number): string {
    return (amount / 100).toFixed(2)
  }
})
