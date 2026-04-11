import { formatPriceNoSymbol } from '../../utils/util'

Component({
  properties: {
    totalPrice: {
      type: Number,
      value: 0
    },
    totalPriceDisplay: {
      type: String,
      value: ''
    },
    totalCount: {
      type: Number,
      value: 0
    },
    deliveryFee: {
      type: Number,
      value: 0
    },
    deliveryFeeDisplay: {
      type: String,
      value: ''
    },
    alwaysShow: {
      type: Boolean,
      value: false
    },
    hideFee: {
      type: Boolean,
      value: false  // 堂食/包间场景不显示配送费提示
    },
    dockBottom: {
      type: Boolean,
      value: false  // 贴底模式，用于餐厅详情页
    }
  },

  observers: {
    'totalPrice, totalPriceDisplay' (price: number, display: string) {
      // 如果传入了格式化好的价格则使用，否则自己格式化
      if (!display && price >= 0) {
        this.setData({ computedPriceDisplay: formatPriceNoSymbol(price) })
      } else {
        this.setData({ computedPriceDisplay: display })
      }
    },
    'deliveryFee, deliveryFeeDisplay' (fee: number, display: string) {
      if (!display && fee > 0) {
        this.setData({ computedDeliveryDisplay: formatPriceNoSymbol(fee) })
      } else {
        this.setData({ computedDeliveryDisplay: display })
      }
    },
    'totalCount'(count: number) {
      if (count > 0) {
        this.setData({ isBouncing: true })
        const that = this as unknown as { _bounceTimer?: ReturnType<typeof setTimeout> }
        if (that._bounceTimer) clearTimeout(that._bounceTimer)
        that._bounceTimer = setTimeout(() => {
          this.setData({ isBouncing: false })
        }, 300)
      }
    }
  },

  data: {
    computedPriceDisplay: '0.00',
    computedDeliveryDisplay: '',
    isBouncing: false
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
