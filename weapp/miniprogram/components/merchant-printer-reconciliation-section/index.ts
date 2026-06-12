Component({
  properties: {
    jobs: {
      type: Array,
      value: []
    },
    errorMessage: {
      type: String,
      value: ''
    },
    loading: {
      type: Boolean,
      value: false
    },
    retryingJobId: {
      type: Number,
      value: 0
    }
  },
  methods: {
    onRefreshTap() {
      this.triggerEvent('refresh')
    },

    onRetryTap(e: WechatMiniprogram.TouchEvent) {
      const { id } = e.currentTarget.dataset as { id?: number }
      this.triggerEvent('retry', { id })
    }
  }
})
