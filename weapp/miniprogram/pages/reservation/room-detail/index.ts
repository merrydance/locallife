import { getRoomDetail, Room } from './_main_shared/api/reservation'
import { checkRoomAvailability, RoomAvailabilityResponse, TimeSlot } from './_main_shared/api/room'
import { formatPriceNoSymbol } from '@/utils/util'
import { settleAll } from '../../../utils/promise'
import { getErrorUserMessage } from '../../../utils/user-facing'

const ROOM_AVAILABILITY_BATCH_SIZE = 3

const getErrorMessage = getErrorUserMessage

interface DayAvailability {
  date: string
  dayLabel: string
  dateNum: string
  lunchAvailable: boolean
  dinnerAvailable: boolean
  lunchSlots: TimeSlot[]
  dinnerSlots: TimeSlot[]
}

type RoomView = Room & {
  minSpendDisplay: string
  depositDisplay: string
}

Page({
  data: {
    roomId: '',
    room: null as RoomView | null,
    navBarHeight: 88,
    loading: false,
    isError: false,
    errorMessage: '',
    // 可用日期列表（未来7天）
    calendarDays: [] as DayAvailability[],
    currentMonth: '',
    loadingDates: false,
    selectedDate: '',
    selectedType: '' as 'lunch' | 'dinner' | ''
  },

  onLoad(options: { id?: string }) {
    if (options.id) {
      this.setData({ roomId: options.id })
      this.loadRoomDetail(options.id)
    }
  },

  onNavHeight(e: WechatMiniprogram.CustomEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onRetry() {
    this.loadRoomDetail(this.data.roomId)
  },

  async loadRoomDetail(id: string) {
    this.setData({ loading: true, isError: false })

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
    } catch (error: unknown) {
      console.error(error)
      this.setData({ 
        loading: false,
        isError: true,
        errorMessage: getErrorMessage(error, '加载详情失败')
      })
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

    // 从今天起加载7天（含当天）
    for (let i = 0; i < 7; i++) {
      const date = new Date(today)
      date.setDate(today.getDate() + i)
      // 格式化日期为 YYYY-MM-DD（后端需要）
      const pad = (n: number) => n < 10 ? '0' + n : String(n)
      const dateStr = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`

      calendarDays.push({
        date: dateStr,
        dayLabel: i === 0 ? '今天' : '周' + weekDays[date.getDay()],
        dateNum: String(date.getDate()),
        lunchAvailable: false,
        dinnerAvailable: false,
        lunchSlots: [],
        dinnerSlots: []
      })
    }

    this.setData({ calendarDays })

    // 并行加载所有日期的可用时段
    try {
      const results: RoomAvailabilityResponse[] = []

      for (let index = 0; index < calendarDays.length; index += ROOM_AVAILABILITY_BATCH_SIZE) {
        const batch = calendarDays.slice(index, index + ROOM_AVAILABILITY_BATCH_SIZE)
        const batchResults = await settleAll(
          batch.map((day) => checkRoomAvailability(roomId, { date: day.date }))
        )

        batchResults.forEach((result, batchIndex) => {
          if (result.status === 'fulfilled') {
            results.push(result.value)
            return
          }

          console.error(`加载日期 ${batch[batchIndex]?.date || ''} 可用性失败:`, result.reason)
          results.push({ time_slots: [] } as RoomAvailabilityResponse)
        })
      }

      const updatedDays: DayAvailability[] = calendarDays.map((day, i) => {
        const slots = results[i].time_slots || []
        
        // 使用后端返回的 period 进行分类
        const lunchSlots = slots.filter((s) => s.period === 'lunch')
        const dinnerSlots = slots.filter((s) => s.period === 'dinner')
        const otherSlots = slots.filter((s) => s.period === 'other')

        // 将 other 合并到 dinner 中
        const effectiveDinnerSlots = [...dinnerSlots, ...otherSlots]

        return {
          ...day,
          lunchSlots,
          dinnerSlots: effectiveDinnerSlots,
          lunchAvailable: lunchSlots.some((s) => s.available),
          dinnerAvailable: effectiveDinnerSlots.some((s) => s.available)
        }
      })

      this.setData({
        calendarDays: updatedDays,
        loadingDates: false
      })
    } catch (error) {
      console.error('加载日历数据失败:', error)
      this.setData({ loadingDates: false })
      wx.showToast({ title: '加载排期失败，请重试', icon: 'none' })
    }
  },

  onCellTap(e: WechatMiniprogram.CustomEvent) {
    const { date, type, available } = e.currentTarget.dataset as { date?: string, type?: 'lunch' | 'dinner', available?: boolean }
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
      // 查找该类别的第一个可用时段作为默认时间
      let time = ''
      const day = this.data.calendarDays.find((d) => d.date === selectedDate)
      
      if (day && selectedType) {
        const targetSlots = selectedType === 'lunch' ? day.lunchSlots : day.dinnerSlots
        const firstAvailable = targetSlots.find((s) => s.available)
        
        if (firstAvailable) {
          time = firstAvailable.time
        } else {
          // 兜底逻辑
          time = selectedType === 'lunch' ? '12:00' : '18:00'
        }
      }

      const url = `/pages/reservation/confirm/index?roomId=${room.id}&merchantId=${room.merchant_id}&roomName=${encodeURIComponent(room.name)}&capacity=${room.capacity}&deposit=${room.deposit}&date=${selectedDate}&time=${time}`
      wx.navigateTo({ url })
    }
  },

  onShareAppMessage(): WechatMiniprogram.Page.ICustomShareContent {
    const { room, roomId } = this.data
    return {
      title: room ? `${room.name} — 一起来预订包间！` : '发现一个好包间，快来看看！',
      path: `/pages/reservation/room-detail/index?id=${roomId}`,
      imageUrl: room?.images?.[0] || ''
    }
  },

  onShareTimeline(): WechatMiniprogram.Page.ICustomTimelineContent {
    const { room, roomId } = this.data
    return {
      title: room ? `${room.name} — 包间推荐` : '包间推荐',
      query: `id=${roomId}`,
      imageUrl: room?.images?.[0] || ''
    }
  }
})
