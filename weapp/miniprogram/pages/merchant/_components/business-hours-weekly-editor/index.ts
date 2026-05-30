type TimeField = 'open_time' | 'close_time'

type PickerValueTuple = [string, string]

interface PickerConfirmDetail {
  value?: Array<string | number> | null
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

function buildTwoDigit(value: number): string {
  return String(value).padStart(2, '0')
}

function buildHourOptions() {
  return Array.from({ length: 24 }, (_, index) => {
    const value = buildTwoDigit(index)
    return { label: value, value }
  })
}

function buildMinuteOptions() {
  return Array.from({ length: 60 }, (_, index) => {
    const value = buildTwoDigit(index)
    return { label: value, value }
  })
}

function splitTimeValue(value: string | undefined): PickerValueTuple {
  const normalized = normalizeTimeValue(value)
  return [normalized.slice(0, 2), normalized.slice(3, 5)]
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
    hourOptions: buildHourOptions(),
    minuteOptions: buildMinuteOptions(),
    pickerPopupProps: {
      preventScrollThrough: true,
      overlayProps: {
        preventScrollThrough: true
      }
    },
    pickerVisible: false,
    pickerTitle: '选择时间',
    pickerValue: ['09', '00'] as PickerValueTuple,
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
        pickerValue: splitTimeValue(value),
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

    onPickerVisibleChange(e: WechatMiniprogram.CustomEvent<{ visible?: boolean }>) {
      const visible = !!e.detail?.visible
      this.setData({
        pickerVisible: visible,
        timeEditContext: visible ? this.data.timeEditContext : null
      })
      this.triggerEvent('pickervisibilitychange', { visible })
    },

    onPickerConfirm(e: WechatMiniprogram.CustomEvent<PickerConfirmDetail>) {
      const context = this.data.timeEditContext
      if (!context) {
        this.onPickerCancel()
        return
      }

      const values = Array.isArray(e.detail?.value) ? e.detail.value : []
      const hour = String(values[0] || '09').padStart(2, '0')
      const minute = String(values[1] || '00').padStart(2, '0')

      this.triggerEvent('changetime', {
        day: context.day,
        slotIndex: context.slotIndex,
        field: context.field,
        value: `${hour}:${minute}`
      })

      this.setData({
        pickerVisible: false,
        timeEditContext: null
      })
      this.triggerEvent('pickervisibilitychange', { visible: false })
    }
  }
})
