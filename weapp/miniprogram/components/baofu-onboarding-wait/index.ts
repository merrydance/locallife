export type BaofuOnboardingWaitState =
  | 'submitting'
  | 'payment_confirming'
  | 'opening_processing'
  | 'pending_confirmation'
  | 'ready'
  | 'failed'
  | 'error'

Component({
  properties: {
    state: {
      type: String,
      value: 'opening_processing'
    },
    title: {
      type: String,
      value: '开户状态同步中'
    },
    description: {
      type: String,
      value: '请稍候，结果会以后端开户状态为准。'
    },
    progressText: {
      type: String,
      value: ''
    },
    theme: {
      type: String,
      value: 'warning'
    },
    primaryActionText: {
      type: String,
      value: ''
    },
    loading: {
      type: Boolean,
      value: false
    }
  },

  data: {},

  methods: {
    onPrimary() {
      if (this.properties.loading) {
        return
      }
      this.triggerEvent('primary')
    }
  }
})
