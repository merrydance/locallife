import { getRoomDetail, Room } from '../../../api/reservation'
import { checkRoomAvailability, RoomAvailabilityResponse } from '../../../api/room'
import { formatTime, formatPriceNoSymbol } from '@/utils/util'

interface DayAvailability {
  date: string
  dayLabel: string    // "周一" "周二" 等
  dateNum: string     // "6" "7" 等
  lunchAvailable: boolean    // 午餐时段是否可订
  dinnerAvailable: boolean   // 晚餐时段是否可订
}

Page({
  data: {
    roomId: '',
    room: null as Room | null,
    navBarHeight: 88,
    loading: false,
    // 可用日期列表（未来7天）
    calendarDays: [] as DayAvailability[],
    currentMonth: '',
    loadingDates: false,
    selectedDate: '',
    selectedType: '' as 'lunch' | 'dinner' | ''
  },

  onLoad(options: any) {
    if (options.id) {
      this.setData({ roomId: options.id })
      this.loadRoomDetail(options.id)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  async loadRoomDetail(id: string) {
    this.setData({ loading: true })

    try {
      const room = await getRoomDetail(id)
      // 预处理价格
      const processedRoom = {
        ...room,
        minSpendDisplay: formatPriceNoSymbol(room.min_spend || 0),
        depositDisplay: formatPriceNoSymbol(room.deposit || 0)
      }
      this.setData({
        room: processedRoom,
        loading: false
      })
      // 加载可用日期
      this.loadCalendarData(parseInt(id))
    } catch (error) {
      wx.showToast({ title: '加载失败', icon: 'error' })
      this.setData({ loading: false })
    }
  },

  // 加载日历数据（未来7天）
  async loadCalendarData(roomId: number) {
    this.setData({ loadingDates: true })

    const calendarDays: DayAvailability[] = []
    const today = new Date()
    const weekDays = ['日', '一', '二', '三', '四', '五', '六']

    // 设置当前月份显示
    const monthNames = ['一月', '二月', '三月', '四月', '五月', '六月',
      '七月', '八月', '九月', '十月', '十一月', '十二月']
    this.setData({ currentMonth: monthNames[today.getMonth()] })

    // 从明天开始加载7天
    for (let i = 1; i <= 7; i++) {
      const date = new Date(today)
      date.setDate(today.getDate() + i)
      // 格式化日期为 YYYY-MM-DD（后端需要）
      const pad = (n: number) => n < 10 ? '0' + n : String(n)
      const dateStr = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`

      calendarDays.push({
        date: dateStr,
        dayLabel: '周' + weekDays[date.getDay()],
        dateNum: String(date.getDate()),
        lunchAvailable: false,
        dinnerAvailable: false
      })
    }

    this.setData({ calendarDays })

    // 并行加载所有日期的可用时段
    try {
      const promises = calendarDays.map(d =>
        checkRoomAvailability(roomId, { date: d.date })
          .catch(() => ({ time_slots: [] } as RoomAvailabilityResponse))
      )
      const results = await Promise.all(promises)

      const updatedDays = calendarDays.map((day, i) => {
        const slots = results[i].time_slots || []
        // 午餐时段: 11:00-14:00
        const lunchSlots = slots.filter(s => {
          const hour = parseInt(s.time.split(':')[0])
          return hour >= 11 && hour < 14
        })
        // 晚餐时段: 17:00-21:00
        const dinnerSlots = slots.filter(s => {
          const hour = parseInt(s.time.split(':')[0])
          return hour >= 17 && hour <= 21
        })

        return {
          ...day,
          lunchAvailable: lunchSlots.some(s => s.available),
          dinnerAvailable: dinnerSlots.some(s => s.available)
        }
      })

      this.setData({
        calendarDays: updatedDays,
        loadingDates: false
      })
    } catch (error) {
      console.error('加载日历数据失败:', error)
      this.setData({ loadingDates: false })
    }
  },

  onCellTap(e: any) {
    const { date, type, available } = e.currentTarget.dataset
    if (!available) {
      wx.showToast({ title: '时段已满', icon: 'none' })
      return
    }

    this.setData({
      selectedDate: date,
      selectedType: type
    })

    // 直接跳转
    this.onBook()
  },

  onBook() {
    const { room, selectedDate, selectedType } = this.data
    if (room) {
      // 默认时间
      let time = ''
      if (selectedType === 'lunch') time = '12:00'
      if (selectedType === 'dinner') time = '18:00'

      const url = `/pages/reservation/confirm/index?roomId=${room.id}&merchantId=${room.merchant_id}&roomName=${encodeURIComponent(room.name)}&capacity=${room.capacity}&deposit=${room.deposit}&date=${selectedDate}&time=${time}`
      wx.navigateTo({ url })
    }
  }
})
