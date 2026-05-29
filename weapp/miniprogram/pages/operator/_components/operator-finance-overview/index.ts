Component({
  properties: {
    regions: {
      type: Array,
      value: []
    },
    selectedRegionIdx: {
      type: Number,
      value: 0
    },
    totalIncomeDisplay: {
      type: String,
      value: '0.00'
    },
    todayIncomeDisplay: {
      type: String,
      value: '0.00'
    },
    currentMonthIncomeDisplay: {
      type: String,
      value: '0.00'
    }
  },

  methods: {
    onRegionTap() {
      this.triggerEvent('regiontap')
    },

    onFinanceTap() {
      this.triggerEvent('financetap')
    }
  }
})