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
import { tableManagementService } from '@/api/table-device-management'

interface TableOption {
  id: number
  table_no: string
  table_type: string
  capacity: number
  todayReservations?: ReservationResponse[] // 当日预订
}

interface CalendarDay {
  day: number
  month: number
  year: number
  fullDate: string // YYYY-MM-DD
  isToday: boolean
  hasReservations: boolean
  disabled: boolean
}

Page({
  data: {
    // 布局
    isLargeScreen: false,
    sidebarCollapsed: false,
    navBarHeight: 88,
    merchantName: '',

    // 日历数据
    currentYear: 2023,
    currentMonth: 1,
    daysInMonth: [] as CalendarDay[],
    emptyDays: [] as number[], // 月初空白天数
    selectedDate: '', // YYYY-MM-DD
    selectedDay: 1,
    selectedMonth: 1,
    selectedDateDisplay: '', // 例如 "12月28日"

    // 业务数据
    loading: false,
    reservations: [] as ReservationResponse[],
    stats: null as ReservationStats | null,
    tables: [] as TableOption[],
    tableViews: [] as TableOption[], // 整合了预订信息的桌台视图数据

    // 创建预订弹窗
    showCreateModal: false,
    createForm: {
      table_id: 0,
      date: '',
      time: '12:00',
      guest_count: 2,
      contact_name: '',
      contact_phone: '',
      source: 'phone' as 'phone' | 'walkin' | 'merchant',
      notes: ''
    },
    selectedTableName: '', // 已选桌台名称
    timeOptions: [] as { value: string, disabled: boolean }[],

    // 详情弹窗
    showDetailModal: false,
    selectedReservation: null as ReservationResponse | null
  },

  async onLoad() {
    this.setData({ isLargeScreen: isLargeScreen() })
    this.initCalendar() // 初始化为今天
    await this.loadTables() // 先加载桌台
    this.refreshData() // 再加载预订数据
  },

  onShow() {
    // 从其他页面返回时刷新数据（如取消预订后返回）
    if (this.data.selectedDate) {
      this.refreshData()
    }
  },

  onSidebarCollapse(e: WechatMiniprogram.CustomEvent) {
    this.setData({ sidebarCollapsed: e.detail.collapsed })
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  // ==================== 日历核心逻辑 ====================

  initCalendar(dateStr?: string) {
    const today = new Date()
    const targetDate = dateStr ? new Date(dateStr) : today
    const year = targetDate.getFullYear()
    const month = targetDate.getMonth() + 1
    const day = targetDate.getDate()

    // 格式化当前选中日期
    const fullDate = `${year}-${('0' + month).slice(-2)}-${('0' + day).slice(-2)}`

    // 生成当月数据
    this.generateMonthDays(year, month)

    this.setData({
      currentYear: year,
      currentMonth: month,
      selectedDate: fullDate,
      selectedDay: day,
      selectedMonth: month,
      selectedDateDisplay: `${month}月${day}日`
    })
  },

  generateMonthDays(year: number, month: number) {
    // 获取当月1号是星期几
    const firstDay = new Date(year, month - 1, 1).getDay()
    // 获取当月总天数
    const lastDay = new Date(year, month, 0).getDate()

    // 空白天占位
    const emptyDays = Array(firstDay).fill(0)

    const days: CalendarDay[] = []
    const today = new Date()
    today.setHours(0, 0, 0, 0)
    const todayTime = today.getTime()

    for (let i = 1; i <= lastDay; i++) {
      const fullDate = `${year}-${('0' + month).slice(-2)}-${('0' + i).slice(-2)}`
      const dateTime = new Date(year, month - 1, i).getTime()

      days.push({
        day: i,
        month: month,
        year: year,
        fullDate: fullDate,
        isToday: dateTime === todayTime,
        hasReservations: false,
        disabled: dateTime < todayTime
      })
    }

    this.setData({
      daysInMonth: days,
      emptyDays: emptyDays
    })
  },

  onDateSelect(e: WechatMiniprogram.TouchEvent) {
    const { date, disabled } = e.currentTarget.dataset
    if (disabled) return
    if (date === this.data.selectedDate) return

    const d = new Date(date)
    this.setData({
      selectedDate: date,
      selectedDay: d.getDate(),
      selectedMonth: d.getMonth() + 1,
      selectedDateDisplay: `${d.getMonth() + 1}月${d.getDate()}日`
    })
    this.refreshData()
  },

  onSelectToday() {
    this.initCalendar()
    this.refreshData()
  },

  onPrevMonth() {
    let { currentYear, currentMonth } = this.data
    currentMonth -= 1
    if (currentMonth < 1) {
      currentMonth = 12
      currentYear -= 1
    }
    this.setData({ currentYear, currentMonth })
    this.generateMonthDays(currentYear, currentMonth)
  },

  onNextMonth() {
    let { currentYear, currentMonth } = this.data
    currentMonth += 1
    if (currentMonth > 12) {
      currentMonth = 1
      currentYear += 1
    }
    this.setData({ currentYear, currentMonth })
    this.generateMonthDays(currentYear, currentMonth)
  },

  // ==================== 数据加载与整合 ====================

  async refreshData() {
    // 并行加载统计和预订列表
    this.loadStats()
    this.loadReservations()
  },

  async loadStats() {
    try {
      // 注意：这里可能需要传日期参数，如果后端支持的话。暂时先不传。
      const stats = await ReservationService.getReservationStats()
      this.setData({ stats })
    } catch (error) {
      logger.error('加载预订统计失败', error, 'reservations')
    }
  },

  async loadReservations() {
    const { selectedDate } = this.data
    this.setData({ loading: true })

    try {
      const params: any = {
        page_id: 1,
        page_size: 100, // 尽量拉取当天所有
        date: selectedDate
      }
      const result = await ReservationService.getMerchantReservations(params)
      const reservations = result.reservations || []

      this.setData({ reservations })
      this.mergeReservationsToTables(reservations)

    } catch (error) {
      logger.error('加载预订列表失败', error, 'reservations')
      wx.showToast({ title: '加载失败', icon: 'error' })
    } finally {
      this.setData({ loading: false })
    }
  },

  async loadTables() {
    try {
      // 获取所有类型的桌台
      const response = await tableManagementService.listTables()
      const tables = response.tables || []
      this.setData({ tables })
      // 如果已经有预订数据，则进行合并；否则只显示桌台
      this.mergeReservationsToTables(this.data.reservations)
    } catch (error) {
      logger.error('加载桌台列表失败', error, 'reservations')
    }
  },

  mergeReservationsToTables(reservations: ReservationResponse[]) {
    const { tables } = this.data
    if (!tables || tables.length === 0) return

    const tableViews = tables.map(table => {
      // 过滤出该桌台的当日预订
      const tableReservations = reservations.filter(r =>
        r.table_id === table.id &&
        // 排除已取消和未到店（看需求，是否要显示已取消但在时间轴上占位的？通常不显示）
        ['confirmed', 'checked_in', 'pending', 'paid', 'completed'].includes(r.status)
      )

      // 按时间排序
      tableReservations.sort((a, b) => a.reservation_time.localeCompare(b.reservation_time))

      return {
        ...table,
        todayReservations: tableReservations
      }
    })

    this.setData({ tableViews })
  },

  // ==================== 创建预订交互 ====================

  generateTimeOptions(date: string) {
    const times = ['11:00', '12:00', '13:00', '17:00', '18:00', '19:00']
    const now = new Date()
    const today = new Date()
    today.setHours(0, 0, 0, 0)
    const selected = new Date(date)
    selected.setHours(0, 0, 0, 0)

    const isToday = selected.getTime() === today.getTime()
    const currentHour = now.getHours()
    const currentMinute = now.getMinutes()

    const options = times.map(t => {
      let disabled = false
      if (selected.getTime() < today.getTime()) {
        disabled = true // 过去日期全禁
      } else if (isToday) {
        const [h, m] = t.split(':').map(Number)
        if (h < currentHour || (h === currentHour && m < currentMinute)) {
          disabled = true
        }
      }
      return { value: t, disabled }
    })
    this.setData({ timeOptions: options })
  },

  onKeyTableTap(e: WechatMiniprogram.TouchEvent) {
    const id = e.currentTarget.dataset.id as number
    const table = this.data.tables.find(t => t.id === id)

    if (!table) return

    this.generateTimeOptions(this.data.selectedDate)

    this.setData({
      showCreateModal: true,
      selectedTableName: table.table_no,
      createForm: {
        table_id: table.id,
        date: this.data.selectedDate, // 自动填充选中日期
        time: '12:00', // 默认时间
        guest_count: table.capacity || 2,
        contact_name: '',
        contact_phone: '',
        source: 'phone',
        notes: ''
      }
    })
  },

  onTimeSelect(e: WechatMiniprogram.TouchEvent) {
    const { time, disabled } = e.currentTarget.dataset
    if (disabled) return
    this.setData({ 'createForm.time': time })
  },

  onGuestCountInc() {
    this.setData({ 'createForm.guest_count': this.data.createForm.guest_count + 1 })
  },

  onGuestCountDec() {
    const current = this.data.createForm.guest_count
    if (current > 1) {
      this.setData({ 'createForm.guest_count': current - 1 })
    }
  },

  onSourceTap(e: WechatMiniprogram.TouchEvent) {
    const source = e.currentTarget.dataset.source as 'phone' | 'walkin' | 'merchant'
    this.setData({ 'createForm.source': source })
  },

  onCloseCreateModal() {
    this.setData({ showCreateModal: false })
  },

  onModalContentTap() {
    // 阻止冒泡
  },

  onFormInput(e: WechatMiniprogram.Input) {
    const field = e.currentTarget.dataset.field as string
    const value = e.detail.value
    this.setData({
      [`createForm.${field}`]: value
    })
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
      this.refreshData() // 刷新数据
    } catch (error) {
      wx.hideLoading()
      logger.error('创建预订失败', error, 'reservations')
      wx.showToast({ title: '创建失败', icon: 'error' })
    }
  },

  // ==================== 自定义详情查看 ====================
  // 为了防止点击卡片触发 "新建"，在详情点击时使用 catchtap
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

  // 详情页操作
  async onCheckIn(e: WechatMiniprogram.TouchEvent) {
    const id = e.currentTarget.dataset.id
    try {
      await ReservationService.checkIn(id)
      wx.showToast({ title: '签到成功' })
      this.setData({ showDetailModal: false })
      this.refreshData()
    } catch (error) {
      wx.showToast({ title: '操作失败', icon: 'none' })
    }
  },

  async onCancelReservation(e: WechatMiniprogram.TouchEvent) {
    const id = e.currentTarget.dataset.id
    wx.showModal({
      title: '提示',
      content: '确定要取消该预订吗？',
      success: async (res) => {
        if (res.confirm) {
          wx.showLoading({ title: '取消中' })
          try {
            await ReservationService.cancelReservation(id, '商户主动取消')
            wx.hideLoading()
            wx.showToast({ title: '已取消' })
            this.setData({ showDetailModal: false })
            this.refreshData()
          } catch (error) {
            wx.hideLoading()
            logger.error('取消预订失败', error, 'reservations')
            wx.showToast({ title: '取消失败', icon: 'none' })
          }
        }
      }
    })
  },

  // ... 其他辅助函数 ...
})
