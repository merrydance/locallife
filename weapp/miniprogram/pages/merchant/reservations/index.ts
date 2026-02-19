import dayjs from 'dayjs'
import { getStableBarHeights } from '../../../utils/responsive'
import {
  MerchantReservationDishSummaryItem,
  ReservationResponse,
  ReservationService
} from '../../../api/reservation'
import { logger } from '../../../utils/logger'

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    date: '',
    reservations: [] as ReservationResponse[],
    dishSummary: [] as MerchantReservationDishSummaryItem[],
    summary: {
      reservationCount: 0,
      tableCount: 0,
      dishKinds: 0,
      dishTotalQuantity: 0
    }
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    const today = dayjs().format('YYYY-MM-DD')
    this.setData({ navBarHeight, date: today })
    this.loadData(today)
  },

  async loadData(date: string) {
    if (this.data.loading) return

    this.setData({ loading: true })
    try {
      const [todayReservations, dishSummaryRes] = await Promise.all([
        ReservationService.getTodayReservations(),
        ReservationService.getMerchantReservationDishes(date)
      ])

      const reservations = todayReservations.reservations || []
      const dishSummary = dishSummaryRes.items || []
      const tableCount = new Set(
        reservations
          .map((reservation) => reservation.table_no)
          .filter((tableNo): tableNo is string => !!tableNo)
      ).size
      const dishTotalQuantity = dishSummary.reduce((sum, item) => sum + (item.total_quantity || 0), 0)

      this.setData({
        reservations,
        dishSummary,
        summary: {
          reservationCount: reservations.length,
          tableCount,
          dishKinds: dishSummary.length,
          dishTotalQuantity
        }
      })
    } catch (err) {
      logger.error('Load reservation summary failed', err)
      wx.showToast({ title: '加载预订数据失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onPullDownRefresh() {
    this.loadData(this.data.date)
  },

  onDateChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const date = e.detail.value
    this.setData({ date })
    this.loadData(date)
  },

  onGoOrderList() {
    wx.navigateTo({ url: '/pages/merchant/orders/list/index?status=paid&order_type=reservation' })
  }
})
