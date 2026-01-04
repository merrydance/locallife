Component({
  properties: {
    totalPrice: {
      type: Number,
      value: 0
    },
    totalCount: {
      type: Number,
      value: 0
    },
    deliveryFee: {
      type: Number,
      value: 0
    },
    alwaysShow: {
      type: Boolean,
      value: false
    },
    dockBottom: {
      type: Boolean,
      value: false  // 贴底模式，用于餐厅详情页
    }
  },

  methods: {
    onCheckout() {
      if (this.properties.totalCount > 0) {
        this.triggerEvent('checkout')
      }
    },

    onCartClick() {
      this.triggerEvent('toggle')
    }
  }
})
