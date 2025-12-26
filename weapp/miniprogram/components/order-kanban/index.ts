Component({
  properties: {
    orders: {
      type: Array,
      value: []
    }
  },

  methods: {
    onAction(e: WechatMiniprogram.TouchEvent) {
      const { id, action } = e.currentTarget.dataset
      this.triggerEvent('action', { id, action })
    }
  }
})
