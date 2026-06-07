Component({
  options: {
    styleIsolation: 'apply-shared'
  },

  properties: {
    selected: {
      type: String,
      value: 'wechat_pay'
    },
    methods: {
      type: Array,
      value: []
    },
    balanceInsufficient: {
      type: Boolean,
      value: false
    }
  },

  methods: {
    onPaymentMethodChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
      this.triggerEvent('change', { value: e.detail.value })
    }
  }
})
