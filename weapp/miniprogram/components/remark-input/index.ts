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
    onInputChange(e: any) {
      const { value } = e.detail
      this.triggerEvent('change', { value })
    },

    onQuickRemarkTap(e: any) {
      const { item } = e.currentTarget.dataset
      let { value } = this.properties
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
