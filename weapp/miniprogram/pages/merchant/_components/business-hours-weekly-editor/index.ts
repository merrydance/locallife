type TimeField = 'open_time' | 'close_time'

interface PickerConfirmDetail {
  value?: string | null
}

interface TimeEditContext {
  day: number
  slotIndex: number
  field: TimeField
}

function normalizeTimeValue(value: string | undefined): string {
  const matched = (value || '').match(/^(\d{1,2}):(\d{1,2})$/)
  if (!matched) return '09:00'
  return `${matched[1].padStart(2, '0')}:${matched[2].padStart(2, '0')}`
}

Component({
  options: {
    styleIsolation: 'apply-shared'
  },

  properties: {
    weeklyHours: {
      type: Array,
      value: []
    }
  },

  data: {
    pickerVisible: false,
    pickerTitle: '选择时间',
    pickerValue: '09:00',
    timeEditContext: null as TimeEditContext | null
  },

  methods: {
    onToggleOpenState(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
      const { day } = e.currentTarget.dataset as { day?: number }
      if (typeof day !== 'number') {
        return
      }

      this.triggerEvent('toggleclosed', {
        day,
        value: e.detail.value
      })
    },

    onAddAfter(e: WechatMiniprogram.TouchEvent) {
      const { day, slotIndex } = e.currentTarget.dataset as { day?: number, slotIndex?: number }
      if (typeof day !== 'number' || typeof slotIndex !== 'number') {
        return
      }

      this.triggerEvent('addslot', { day, slotIndex })
    },

    onRemoveAt(e: WechatMiniprogram.TouchEvent) {
      const { day, slotIndex } = e.currentTarget.dataset as { day?: number, slotIndex?: number }
      if (typeof day !== 'number' || typeof slotIndex !== 'number') {
        return
      }

      this.triggerEvent('removeslot', { day, slotIndex })
    },

    onOpenTimePicker(e: WechatMiniprogram.TouchEvent) {
      const { day, slotIndex, field, value } = e.currentTarget.dataset as {
        day?: number
        slotIndex?: number
        field?: TimeField
        value?: string
      }
      if (typeof day !== 'number' || typeof slotIndex !== 'number' || (field !== 'open_time' && field !== 'close_time')) {
        return
      }

      this.setData({
        pickerVisible: true,
        pickerTitle: field === 'open_time' ? '选择开始时间' : '选择结束时间',
        pickerValue: normalizeTimeValue(value),
        timeEditContext: {
          day,
          slotIndex,
          field
        }
      })
      this.triggerEvent('pickervisibilitychange', { visible: true })
    },

    onPickerCancel() {
      this.setData({
        pickerVisible: false,
        timeEditContext: null
      })
      this.triggerEvent('pickervisibilitychange', { visible: false })
    },

    onPickerClose() {
      this.setData({
        pickerVisible: false,
        timeEditContext: null
      })
      this.triggerEvent('pickervisibilitychange', { visible: false })
    },

    onPickerConfirm(e: WechatMiniprogram.CustomEvent<PickerConfirmDetail>) {
      const context = this.data.timeEditContext
      if (!context) {
        this.onPickerCancel()
        return
      }

      const value = normalizeTimeValue(e.detail?.value || undefined)

      this.triggerEvent('changetime', {
        day: context.day,
        slotIndex: context.slotIndex,
        field: context.field,
        value
      })

      this.setData({
        pickerVisible: false,
        timeEditContext: null
      })
      this.triggerEvent('pickervisibilitychange', { visible: false })
    }
  }
})
