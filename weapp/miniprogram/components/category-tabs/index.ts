Component({
  properties: {
    categories: {
      type: Array,
      value: []
    },
    activeId: {
      type: String,
      value: ''
    }
  },

  methods: {
    onTabClick(e: WechatMiniprogram.TouchEvent) {
      const { id } = e.currentTarget.dataset
      this.triggerEvent('change', { id })
    }
  }
})
