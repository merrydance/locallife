Component({
  options: {
    styleIsolation: 'apply-shared'
  },
  properties: {
    value: {
      type: String,
      value: ''
    },
    label: {
      type: String,
      value: '订单备注'
    },
    placeholder: {
      type: String,
      value: '口味要求，点此输入...'
    },
    maxlength: {
      type: Number,
      value: 100
    },
    quickRemarks: {
      type: Array,
      value: ['少辣', '不要葱', '不要香菜', '多加饭']
    }
  },

  methods: {
    onInputChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
      const value = e.detail?.value || ''
      this.triggerEvent('change', { value })
    },

    onQuickRemarkTap(e: WechatMiniprogram.TouchEvent) {
      const item = (e.currentTarget.dataset as { item?: string }).item || ''
      let { value } = this.properties
      if (!item) {
        return
      }
      if (value) {
        if (!value.includes(item)) {
          value = `${value}, ${item}`
        }
      } else {
        value = item
      }
      this.triggerEvent('change', { value })
    }
  }
})
