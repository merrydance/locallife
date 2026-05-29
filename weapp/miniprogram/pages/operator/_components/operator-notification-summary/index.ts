Component({
  properties: {
    unreadCount: {
      type: Number,
      value: 0
    },
    latestTitle: {
      type: String,
      value: '暂无待接单提醒'
    },
    latestSummary: {
      type: String,
      value: '当前没有新的待接单提醒。'
    },
    latestCreatedAt: {
      type: String,
      value: ''
    }
  },

  methods: {
    onTap() {
      this.triggerEvent('tap')
    }
  }
})