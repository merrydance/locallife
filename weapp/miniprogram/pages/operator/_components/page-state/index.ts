Component({
  properties: {
    type: {
      type: String,
      value: 'empty'
    },
    description: {
      type: String,
      value: ''
    },
    loadingText: {
      type: String,
      value: '加载中'
    },
    actionText: {
      type: String,
      value: ''
    },
    icon: {
      type: String,
      value: ''
    }
  },

  methods: {
    onAction() {
      this.triggerEvent('action')
    }
  }
})